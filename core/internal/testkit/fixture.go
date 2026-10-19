// Package testkit creates NON-CLINICAL fixtures, never bundled into the CLI.
package testkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"testing"
	"time"
)

func Bool(v bool) *bool        { return &v }
func Float(v float64) *float64 { return &v }

var At = time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
var Context = domain.Context{Product: "SYNTHETIC-NOT-A-MEDICINE", Route: "fixture", Form: "fixture", Indication: "fixture"}

func Request() domain.Request {
	return domain.Request{At: At, Patient: domain.Patient{Age: 70, Sex: "male", Weight: &domain.Quantity{Value: 72, Unit: "kg"}, Height: &domain.Quantity{Value: 175, Unit: "cm"}, Creatinine: domain.Quantity{Value: 70.0 / 30.0, Unit: "mg/dL"}, MeasuredAt: At.Add(-time.Hour), Stable: Bool(true), AKI: Bool(false), Dialysis: Bool(false), IDMS: Bool(true), TypicalBodyComposition: Bool(true), ActualWeightConfirmed: Bool(true)}, Medications: []domain.Context{Context}}
}
func Fixture(t *testing.T) (domain.Knowledge, string) {
	t.Helper()
	root := t.TempDir()
	text := []byte("Synthetic software fixture only. Renal boundary 30; age 18 to 120; weight 20 to 100; dose 10 mg every 24 hours. Not clinical evidence.")
	if e := os.WriteFile(filepath.Join(root, "source.txt"), text, 0600); e != nil {
		t.Fatal(e)
	}
	h := domain.BytesDigest(text)
	k := domain.Knowledge{Version: "synthetic-v1", Synthetic: true, Sources: []domain.Source{{ID: "fixture", URL: "https://example.invalid/synthetic", Version: "fixture-v1", RetrievedAt: At.Add(-24 * time.Hour), DocumentFile: "source.txt", DocumentSHA256: h, TextFile: "source.txt", TextSHA256: h, Synthetic: true}}, Rules: []domain.Rule{}}
	for i, rg := range []domain.Interval{{Lower: 0, Upper: Float(30), LowerInclusive: true}, {Lower: 30, LowerInclusive: true}} {
		action, id := "AVOID", "low"
		if i == 1 {
			action, id = "NO_CHANGE", "high"
		}
		r := domain.Rule{ID: id, Version: "fixture-1", Context: Context, Metric: domain.CrCl, Unit: domain.CrCl.Unit(), Range: rg, Action: action, Citation: domain.Citation{SourceID: "fixture", DocumentSHA256: h, SourceVersion: "fixture-v1", Section: "fixture", Page: 1, Quote: string(text)}, Status: "APPROVED"}
		Sign(&r)
		k.Rules = append(k.Rules, r)
	}
	return k, root
}
func Sign(r *domain.Rule) {
	r.Approval = &domain.Approval{Reviewer: "SYNTHETIC-TEST-REVIEWER", Role: "pharmacist", At: At.Add(-time.Hour), ContentDigest: r.ContentDigest()}
}
func Load(t *testing.T, k domain.Knowledge, root string) rules.Snapshot {
	t.Helper()
	b, e := json.Marshal(k)
	if e != nil {
		t.Fatal(e)
	}
	s, e := rules.Load(b, root, true)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
