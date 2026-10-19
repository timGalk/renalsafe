package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/evidence"
	"renalsafe/core/internal/store"
	"renalsafe/core/internal/testkit"
	"strings"
	"sync"
	"testing"
)

var physician = store.Actor{ID: "fixture-physician", Role: "physician"}
var pharmacist = store.Actor{ID: "fixture-pharmacist", Role: "pharmacist"}

func approve(t *testing.T, db *store.Store, r store.StoredRule) store.StoredRule {
	t.Helper()
	out, e := db.Review(t.Context(), pharmacist, r.Rule.ID, "approve", store.ReviewInput{ExpectedDigest: r.Digest, Confirmed: true, Note: "SYNTHETIC SOFTWARE TEST ONLY"})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestTransactionalAssessmentReviewAndHistoricalReplay(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	pending := dbtest.Pending(t, db, d)
	a, e := db.Assess(t.Context(), physician, dbtest.CurrentRequest())
	if e != nil {
		t.Fatal(e)
	}
	if a.Assessment.Results[0].Status != "MANUAL_REVIEW_REQUIRED" || a.Assessment.Results[0].Action != "" {
		t.Fatal("pending rule leaked")
	}
	approved := approve(t, db, pending)
	if approved.Rule.Approval.Reviewer != pharmacist.ID {
		t.Fatal("review identity not bound")
	}
	a, e = db.Assess(t.Context(), physician, dbtest.CurrentRequest())
	if e != nil {
		t.Fatal(e)
	}
	if a.Assessment.Results[0].Action != "AVOID" {
		t.Fatal(a.Assessment.Results)
	}
	if _, e = db.Assessment(t.Context(), store.Actor{ID: "another", Role: "physician"}, a.ID); !errors.Is(e, store.ErrForbidden) {
		t.Fatal("assessment leaked", e)
	}
	replacement := dbtest.AddDocument(t, db, testkit.Context)
	if e = db.Supersede(t.Context(), pharmacist, d.ID, replacement.ID); e != nil {
		t.Fatal(e)
	}
	ok, e := db.Replay(t.Context(), physician, a.ID)
	if e != nil || !ok {
		t.Fatal("historical replay failed", e)
	}
	current, e := db.Assess(t.Context(), physician, dbtest.CurrentRequest())
	if e != nil || current.Assessment.Results[0].Action != "" {
		t.Fatal("superseded rule active", e)
	}
	audit, e := db.Audit(t.Context())
	if e != nil || len(audit) < 7 {
		t.Fatal(e, len(audit))
	}
}
func TestReviewAuthorizationStalenessAndOverlap(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	a := dbtest.Pending(t, db, d)
	b := dbtest.Pending(t, db, d)
	input := store.ReviewInput{ExpectedDigest: a.Digest, Confirmed: true, Note: "fixture"}
	if _, e := db.Review(t.Context(), physician, a.Rule.ID, "approve", input); !errors.Is(e, store.ErrForbidden) {
		t.Fatal(e)
	}
	input.ExpectedDigest = "stale"
	if _, e := db.Review(t.Context(), pharmacist, a.Rule.ID, "approve", input); !errors.Is(e, store.ErrConflict) {
		t.Fatal(e)
	}
	input.ExpectedDigest = a.Digest
	input.Confirmed = false
	if _, e := db.Review(t.Context(), pharmacist, a.Rule.ID, "approve", input); !errors.Is(e, store.ErrInvalid) {
		t.Fatal(e)
	}
	approve(t, db, a)
	if _, e := db.Review(t.Context(), pharmacist, b.Rule.ID, "approve", store.ReviewInput{ExpectedDigest: b.Digest, Confirmed: true, Note: "fixture"}); e == nil {
		t.Fatal("overlapping approval accepted")
	}
	// The database exclusion protects the invariant even if an application check is bypassed.
	raw := b.Rule
	raw.Status = "APPROVED"
	payload, _ := json.Marshal(raw)
	_, e := db.Pool.Exec(t.Context(), `UPDATE renal_rules SET status='APPROVED',payload=$2 WHERE id=$1`, raw.ID, payload)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "23P01" {
		t.Fatal("database overlap exclusion absent", e)
	}
}
func TestConcurrentApprovalsAndAuditOrdering(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	a, b := dbtest.Pending(t, db, d), dbtest.Pending(t, db, d)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, r := range []store.StoredRule{a, b} {
		wg.Add(1)
		go func(r store.StoredRule) {
			defer wg.Done()
			_, e := db.Review(context.Background(), pharmacist, r.Rule.ID, "approve", store.ReviewInput{ExpectedDigest: r.Digest, Confirmed: true, Note: "synthetic concurrent review"})
			results <- e
		}(r)
	}
	wg.Wait()
	close(results)
	count := 0
	for e := range results {
		if e == nil {
			count++
		}
	}
	if count != 1 {
		t.Fatal("expected one approval", count)
	}
	if _, e := db.Audit(t.Context()); e != nil {
		t.Fatal(e)
	}
}
func TestAuditFailureRollsBackAssessment(t *testing.T) {
	db := dbtest.Database(t)
	dbtest.AddDocument(t, db, testkit.Context)
	_, e := db.Pool.Exec(t.Context(), `CREATE FUNCTION fail_audit_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;CREATE TRIGGER fail_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_audit_test()`)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Assess(t.Context(), physician, dbtest.CurrentRequest()); e == nil {
		t.Fatal("unaudited success")
	}
	var count int
	if e = db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM assessments`).Scan(&count); e != nil || count != 0 {
		t.Fatal("partial commit", e, count)
	}
}
func TestImmutableSourceAndHistory(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	if _, e := db.Pool.Exec(t.Context(), `UPDATE documents SET text_content='tampered' WHERE id=$1`, d.ID); e == nil {
		t.Fatal("source mutated")
	}
	if _, e := db.Pool.Exec(t.Context(), `DELETE FROM audit_events`); e == nil {
		t.Fatal("audit deleted")
	}
}

type fakeEmbed struct{ fail bool }

func (fakeEmbed) Model() string { return "SYNTHETIC-EMBEDDINGS-v1" }
func (f fakeEmbed) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if f.fail {
		return nil, errors.New("offline")
	}
	out := [][]float64{}
	for _, text := range texts {
		v := make([]float64, evidence.Dimensions)
		v[0] = 1
		if strings.Contains(text, "threshold") {
			v[1] = 1
		}
		out = append(out, v)
	}
	return out, nil
}
func TestEvidenceIsolationHybridAndFallback(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	pending := dbtest.Pending(t, db, d)
	input := store.SearchInput{Context: testkit.Context, Query: "renal", Limit: 4}
	out, e := db.Search(t.Context(), physician, input, nil)
	if e != nil || len(out.Hits) == 0 || out.Mode != "lexical" || out.DegradedReason == "" || out.Status != "EVIDENCE_ONLY" {
		t.Fatal(e, out)
	}
	for _, h := range out.Hits {
		if h.Source.DocumentSHA256 != d.Source.DocumentSHA256 || h.Chunk.DocumentID != d.ID {
			t.Fatal("wrong provenance")
		}
		if dbtest.DocumentText[h.Chunk.Start:h.Chunk.End] != h.Chunk.Text {
			t.Fatal("source offsets inaccurate")
		}
	}
	if _, e = db.IndexEmbeddings(t.Context(), pharmacist, d.ID, fakeEmbed{}); e != nil {
		t.Fatal(e)
	}
	out, e = db.Search(t.Context(), physician, input, fakeEmbed{})
	if e != nil || out.Mode != "hybrid" {
		t.Fatal(e, out)
	}
	if out.Hits[0].LexicalRank == 0 || out.Hits[0].DenseRank == 0 {
		t.Fatal("RRF did not fuse")
	}
	out, e = db.Search(t.Context(), physician, input, fakeEmbed{fail: true})
	if e != nil || out.Mode != "lexical" || out.DegradedReason == "" {
		t.Fatal(e, out)
	}
	input.Context.Product = "other"
	out, e = db.Search(t.Context(), physician, input, fakeEmbed{})
	if e != nil || len(out.Hits) != 0 {
		t.Fatal("cross-product leakage", e)
	}
	input.Context = testkit.Context
	input.Query = "nonexistentterm"
	input.Mode = "lexical"
	out, e = db.Search(t.Context(), physician, input, nil)
	if e != nil || out.Status != "NO_EVIDENCE" {
		t.Fatal(e, out)
	}
	replacement := dbtest.AddDocument(t, db, testkit.Context)
	if e = db.Supersede(t.Context(), pharmacist, d.ID, replacement.ID); e != nil {
		t.Fatal(e)
	}
	input.DocumentID = d.ID
	input.Query = "renal"
	out, e = db.Search(t.Context(), physician, input, nil)
	if e != nil || len(out.Hits) > 0 {
		t.Fatal("superseded source leaked", e)
	}
	rs, e := db.Rules(t.Context(), "")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rs {
		if r.Rule.ID == pending.Rule.ID && r.Rule.Status == "APPROVED" {
			t.Fatal("retrieval approved rule")
		}
	}
}
func TestSeedRealSourceRemainsPending(t *testing.T) {
	db := dbtest.Database(t)
	path := "../../../data/knowledge.json"
	if _, e := os.Stat(path); e != nil {
		t.Fatal(e)
	}
	if e := db.Seed(t.Context(), path); e != nil {
		t.Fatal(e)
	}
	if e := db.Seed(t.Context(), path); e != nil {
		t.Fatal(e)
	}
	ds, e := db.Documents(t.Context())
	if e != nil || len(ds) != 1 {
		t.Fatal(e, ds)
	}
	rs, e := db.Rules(t.Context(), "PENDING")
	if e != nil || len(rs) != 1 {
		t.Fatal(e, rs)
	}
	if rs[0].Rule.Citation.Page != 6 {
		t.Fatal("wrong physical page")
	}
}
func TestHistoryReferencesAndPassagesAreImmutable(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	if _, e := db.Pool.Exec(t.Context(), `UPDATE chunks SET content='changed' WHERE document_id=$1`, d.ID); e == nil {
		t.Fatal("source passage changed")
	}
	if _, e := db.Pool.Exec(t.Context(), `DELETE FROM chunks WHERE document_id=$1`, d.ID); e == nil {
		t.Fatal("source passage deleted")
	}
	if _, e := db.Pool.Exec(t.Context(), `INSERT INTO assessments(id,actor_id,payload) VALUES('orphan','test','{"audit_hash":"missing"}')`); e == nil {
		t.Fatal("assessment without audit accepted")
	}
}

func TestPendingEditRequiresDigestAndGrounding(t *testing.T) {
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	r := dbtest.Pending(t, db, d)
	next := r.Rule
	next.Range.Upper = testkit.Float(29)
	if _, e := db.EditCandidate(t.Context(), pharmacist, r.Rule.ID, r.Digest, next); e == nil {
		t.Fatal("unsupported threshold")
	}
	next = r.Rule
	next.Action = "USE_WITH_CAUTION"
	changed, e := db.EditCandidate(t.Context(), pharmacist, r.Rule.ID, r.Digest, next)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Review(t.Context(), pharmacist, r.Rule.ID, "approve", store.ReviewInput{ExpectedDigest: r.Digest, Confirmed: true, Note: "fixture"}); !errors.Is(e, store.ErrConflict) {
		t.Fatal("stale review accepted", e)
	}
	if changed.Digest == r.Digest {
		t.Fatal("edit digest unchanged")
	}
}
