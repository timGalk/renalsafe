package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/extraction"
	"renalsafe/core/internal/store"
	"renalsafe/core/internal/testkit"
	"strings"
	"testing"
	"time"
)

type extractionProvider struct {
	calls int
	run   func(extraction.SourceInput) (extraction.ExtractionResult, error)
}

func (p *extractionProvider) Model() string { return "fixture-model" }
func (p *extractionProvider) Extract(ctx context.Context, in extraction.SourceInput) (extraction.ExtractionResult, error) {
	p.calls++
	if p.run != nil {
		return p.run(in)
	}
	return extractionFixture(in), nil
}
func extractionFixture(in extraction.SourceInput) extraction.ExtractionResult {
	var main, condition extraction.SourcePassage
	for _, p := range in.Passages {
		if p.Section == "4.3" {
			main = p
		}
		if p.Section == "4.4" {
			condition = p
		}
	}
	return extraction.ExtractionResult{Model: "fixture-model-snapshot", ResponseID: "resp_fixture", Output: extraction.ExtractionOutput{Reason: "", Findings: []extraction.ExtractionFinding{{Summary: "Fictional source finding; not clinical advice", Facts: []extraction.ExtractionFact{{Field: "upper_value", Value: "30", ChunkID: main.ID, Quote: "Synthetic renal fixture threshold 30. Not clinical evidence."}, {Field: "condition", Value: "Synthetic caution: renal dysfunction.", ChunkID: condition.ID, Quote: "Synthetic caution: renal dysfunction."}}, Unresolved: []string{"Renal method remains unspecified"}}}}}
}
func extractionInput(j store.ExtractionJob) store.ExtractionSave {
	fact := j.Result.Output.Findings[0].Facts[0]
	return store.ExtractionSave{FindingIndex: 0, ExpectedDigest: j.Digest, Candidate: store.CandidateInput{ChunkID: fact.ChunkID, Rule: domain.Rule{Context: j.Source.Context, Metric: domain.CrCl, Unit: domain.CrCl.Unit(), Range: domain.Interval{Lower: 0, Upper: testkit.Float(30), LowerInclusive: true}, Action: "AVOID", Citation: domain.Citation{Section: "4.3", Quote: fact.Quote}, Status: "PENDING"}}}
}
func TestExtractionLifecycleAndPendingOnly(t *testing.T) {
	db := dbtest.Database(t)
	doc := dbtest.AddDocument(t, db, testkit.Context)
	p := &extractionProvider{}
	in := store.ExtractionInput{DocumentID: doc.ID, RequestKey: store.ID()}
	if _, err := db.CreateExtraction(t.Context(), physician, in, p.Model()); !errors.Is(err, store.ErrForbidden) {
		t.Fatal(err)
	}
	j, err := db.CreateExtraction(t.Context(), pharmacist, in, p.Model())
	if err != nil {
		t.Fatal(err)
	}
	again, err := db.CreateExtraction(t.Context(), pharmacist, in, p.Model())
	if err != nil || again.ID != j.ID {
		t.Fatal("duplicate request", err)
	}
	if _, err = db.Extraction(t.Context(), physician, j.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatal(err)
	}
	if err = db.RunExtraction(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	j, err = db.Extraction(t.Context(), pharmacist, j.ID)
	if err != nil || j.Status != "COMPLETED" || j.Result == nil || p.calls != 1 {
		t.Fatal(j, err)
	}
	rules, err := db.Rules(t.Context(), "")
	if err != nil || len(rules) != 0 {
		t.Fatal("model created rules", err)
	}
	save := extractionInput(j)
	stale := save
	stale.ExpectedDigest = "stale"
	if _, err = db.SaveExtraction(t.Context(), pharmacist, j.ID, stale); !errors.Is(err, store.ErrConflict) {
		t.Fatal(err)
	}
	if _, err = db.SaveExtraction(t.Context(), physician, j.ID, save); !errors.Is(err, store.ErrForbidden) {
		t.Fatal(err)
	}
	approved := save
	approved.Candidate.Rule.Status = "APPROVED"
	if _, err = db.SaveExtraction(t.Context(), pharmacist, j.ID, approved); !errors.Is(err, store.ErrInvalid) {
		t.Fatal("model approval accepted", err)
	}
	r, err := db.SaveExtraction(t.Context(), pharmacist, j.ID, save)
	if err != nil {
		t.Fatal(err)
	}
	if r.Rule.Status != "PENDING" || r.Rule.Approval != nil || r.Rule.Conditions == nil || len(r.Rule.Conditions.Unmodeled) != 2 {
		t.Fatal("qualifiers dropped", r)
	}
	duplicate, err := db.SaveExtraction(t.Context(), pharmacist, j.ID, save)
	if err != nil || duplicate.Rule.ID != r.Rule.ID {
		t.Fatal("duplicate save", err)
	}
	if _, err = db.Review(t.Context(), pharmacist, r.Rule.ID, "approve", store.ReviewInput{ExpectedDigest: r.Digest, Confirmed: true, Note: "Fictional test only"}); !errors.Is(err, store.ErrInvalid) {
		t.Fatal("unmodeled condition approved", err)
	}
	if _, err = db.Audit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DismissExtraction(t.Context(), pharmacist, j.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.SaveExtraction(t.Context(), pharmacist, j.ID, save); !errors.Is(err, store.ErrConflict) {
		t.Fatal("dismissed suggestions still saveable", err)
	}
}
func TestExtractionFailuresCancellationAndSourceChange(t *testing.T) {
	for _, kind := range []string{"queued-cancel", "running-cancel", "provider-error", "invented-quote", "source-superseded", "changed-model"} {
		t.Run(kind, func(t *testing.T) {
			db := dbtest.Database(t)
			doc := dbtest.AddDocument(t, db, testkit.Context)
			p := &extractionProvider{}
			j, err := db.CreateExtraction(t.Context(), pharmacist, store.ExtractionInput{DocumentID: doc.ID, RequestKey: store.ID()}, p.Model())
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "queued-cancel":
				if _, err = db.CancelExtraction(t.Context(), pharmacist, j.ID); err != nil {
					t.Fatal(err)
				}
			case "running-cancel":
				p.run = func(in extraction.SourceInput) (extraction.ExtractionResult, error) {
					_, err := db.CancelExtraction(t.Context(), pharmacist, j.ID)
					if err != nil {
						t.Error(err)
					}
					return extractionFixture(in), nil
				}
			case "provider-error":
				p.run = func(extraction.SourceInput) (extraction.ExtractionResult, error) {
					return extraction.ExtractionResult{}, fmt.Errorf("private provider diagnostic")
				}
			case "invented-quote":
				p.run = func(in extraction.SourceInput) (extraction.ExtractionResult, error) {
					out := extractionFixture(in)
					out.Output.Findings[0].Facts[0].Quote = "invented"
					return out, nil
				}
			case "source-superseded":
				replacement := dbtest.AddDocument(t, db, testkit.Context)
				if err = db.Supersede(t.Context(), pharmacist, doc.ID, replacement.ID); err != nil {
					t.Fatal(err)
				}
			case "changed-model":
				p.run = nil
				_, err = db.Pool.Exec(t.Context(), `UPDATE extraction_jobs SET model='old-model',payload=jsonb_set(payload,'{model}','"old-model"') WHERE id=$1`, j.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = db.RunExtraction(t.Context(), p); err != nil {
				t.Fatal(err)
			}
			j, err = db.Extraction(t.Context(), pharmacist, j.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := "FAILED"
			if strings.Contains(kind, "cancel") {
				want = "CANCELLED"
			}
			if j.Status != want || j.Result != nil || strings.Contains(j.Error, "private") {
				t.Fatal(j)
			}
			calls := p.calls
			if err = db.RunExtraction(t.Context(), p); err != nil || p.calls != calls {
				t.Fatal("automatic paid retry", err)
			}
			if (kind == "queued-cancel" || kind == "source-superseded" || kind == "changed-model") && p.calls != 0 {
				t.Fatal("provider called after invalidation")
			}
			if _, err = db.Audit(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestInterruptedExtractionDoesNotResubmit(t *testing.T) {
	db := dbtest.Database(t)
	doc := dbtest.AddDocument(t, db, testkit.Context)
	p := &extractionProvider{}
	j, err := db.CreateExtraction(t.Context(), pharmacist, store.ExtractionInput{DocumentID: doc.ID, RequestKey: store.ID()}, p.Model())
	if err != nil {
		t.Fatal(err)
	}
	j.Status = "RUNNING"
	old := time.Now().Add(-4 * time.Minute)
	j.StartedAt = &old
	b, _ := json.Marshal(j)
	if _, err = db.Pool.Exec(t.Context(), `UPDATE extraction_jobs SET status=$2,payload=$3 WHERE id=$1`, j.ID, j.Status, b); err != nil {
		t.Fatal(err)
	}
	if err = db.RunExtraction(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	j, err = db.Extraction(t.Context(), pharmacist, j.ID)
	if err != nil || j.Status != "FAILED" || p.calls != 0 {
		t.Fatal(j, err)
	}
}

func TestCandidateRejectsAmbiguousQuoteOccurrence(t *testing.T) {
	db := dbtest.Database(t)
	quote := "Synthetic renal fixture threshold 30. Not clinical evidence."
	text := strings.Replace(dbtest.DocumentText, quote, quote+"\n"+quote, 1)
	doc, err := db.AddDocument(t.Context(), store.Actor{ID: "fixture-admin", Role: "admin"}, store.DocumentInput{Context: testkit.Context, URL: "https://example.invalid/synthetic", Version: store.ID(), PageStart: 1, PageEnd: 3}, []byte("%PDF-SYNTHETIC-FIXTURE"), text)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := db.Chunks(t.Context(), doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range chunks {
		if chunk.Section == "4.3" {
			in := store.CandidateInput{ChunkID: chunk.ID, Rule: domain.Rule{Context: doc.Context, Metric: domain.CrCl, Unit: domain.CrCl.Unit(), Range: domain.Interval{Lower: 0, Upper: testkit.Float(30), LowerInclusive: true}, Action: "AVOID", Citation: domain.Citation{Section: "4.3", Quote: quote}, Status: "PENDING"}}
			if _, err = db.AddCandidate(t.Context(), pharmacist, in); !errors.Is(err, store.ErrInvalid) {
				t.Fatal("ambiguous rule citation accepted", err)
			}
			return
		}
	}
	t.Fatal("missing fixture chunk")
}
