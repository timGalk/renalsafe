package dbtest

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/store"
	"renalsafe/core/internal/testkit"
	"testing"
	"time"
)

func Database(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("RENALSAFE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration test: run python3 scripts/test-integration.py")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/renalsafe_test" {
		t.Fatal("integration tests require dedicated /renalsafe_test database")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "test_" + store.ID()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	db, e := store.Open(ctx, u.String(), true)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); _ = admin.Close(ctx) })
	if e = db.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	return db
}

const DocumentText = "1. NAME OF THE MEDICINAL PRODUCT\nSYNTHETIC-NOT-A-MEDICINE\n\f4.2 Posology\nSynthetic fixture only: renal clearance 15 30 50 100.\nDose  10 mg  every 24 hours.\n\f4.3 Contraindications\nSynthetic renal fixture threshold 30. Not clinical evidence.\n4.4 Warnings\nSynthetic caution: renal dysfunction.\n5.2 Pharmacokinetics\nSynthetic elimination data.\n"

func AddDocument(t *testing.T, db *store.Store, context domain.Context) store.Document {
	t.Helper()
	d, e := db.AddDocument(t.Context(), store.Actor{ID: "fixture-admin", Role: "admin"}, store.DocumentInput{Context: context, URL: "https://example.invalid/synthetic", Version: store.ID(), PageStart: 1, PageEnd: 3}, []byte("%PDF-SYNTHETIC-FIXTURE"), DocumentText)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func Pending(t *testing.T, db *store.Store, d store.Document) store.StoredRule {
	t.Helper()
	cs, e := db.Chunks(t.Context(), d.ID)
	if e != nil {
		t.Fatal(e)
	}
	var chunk store.Chunk
	for _, c := range cs {
		if c.Section == "4.3" {
			chunk = c
		}
	}
	in := store.CandidateInput{ChunkID: chunk.ID, Rule: domain.Rule{Context: d.Context, Metric: domain.CrCl, Unit: domain.CrCl.Unit(), Range: domain.Interval{Lower: 0, Upper: testkit.Float(30), LowerInclusive: true}, Action: "AVOID", Citation: domain.Citation{Section: "4.3", Quote: "Synthetic renal fixture threshold 30. Not clinical evidence."}, Status: "PENDING"}}
	r, e := db.AddCandidate(t.Context(), store.Actor{ID: "fixture-pharmacist", Role: "pharmacist"}, in)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func CurrentRequest() domain.Request {
	r := testkit.Request()
	r.At = time.Now().UTC()
	r.Patient.MeasuredAt = r.At.Add(-time.Hour)
	r.Patient.Creatinine.Value = 3
	return r
}
