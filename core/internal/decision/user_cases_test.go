package decision

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"strings"
	"testing"
	"time"
)

// Run the saved fictional patient requests through the real engine with pinned
// ChPL sources and zero approved rules. Never manufacture a clinical approval.
func TestSyntheticUserCases(t *testing.T) {
	dataRoot := "../../../data"
	var suite struct {
		Contexts []struct {
			SourceID string   `json:"source_id"`
			Quotes   []string `json:"context_evidence"`
		} `json:"contexts"`
		Cases []struct {
			ID        string          `json:"id"`
			Synthetic bool            `json:"synthetic_patient"`
			SourceIDs []string        `json:"source_ids"`
			Request   json.RawMessage `json:"request"`
			Expected  struct {
				Status   string          `json:"status"`
				Reason   string          `json:"result_reason"`
				Metrics  []domain.Metric `json:"metrics"`
				Warnings []string        `json:"renal_warnings"`
				CrCl     *float64        `json:"crcl"`
				Low      *float64        `json:"crcl_low"`
				High     *float64        `json:"crcl_high"`
				Dose     bool            `json:"automated_dose_allowed"`
			} `json:"expected_software"`
			Clinical json.RawMessage `json:"clinical_reference"`
			Review   string          `json:"clinical_review_status"`
		} `json:"cases"`
	}
	readJSON := func(path string, out any) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dataRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, out); err != nil {
			t.Fatal(err)
		}
	}
	readJSON("user-cases.json", &suite)
	if len(suite.Cases) < 30 || len(suite.Contexts) != 7 {
		t.Fatal("user case coverage missing")
	}
	var manifest struct {
		Documents []struct {
			ID        string    `json:"product_id"`
			URL       string    `json:"source_url"`
			PDF       string    `json:"pdf_path"`
			Text      string    `json:"text_path"`
			PDFHash   string    `json:"pdf_sha256"`
			TextHash  string    `json:"extracted_text_sha256"`
			Retrieved time.Time `json:"retrieved_at"`
		} `json:"documents"`
	}
	readJSON("sources/pl-2026-11-09/manifest.json", &manifest)
	k := domain.Knowledge{Version: "user-cases-source-only-v1", Sources: []domain.Source{}, Rules: []domain.Rule{}}
	sourceIDs := map[string]bool{}
	for _, context := range suite.Contexts {
		for _, doc := range manifest.Documents {
			if doc.ID != context.SourceID {
				continue
			}
			pdf := filepath.Join("sources/pl-2026-11-09", doc.PDF)
			text := filepath.Join("sources/pl-2026-11-09", doc.Text)
			b, err := os.ReadFile(filepath.Join(dataRoot, text))
			if err != nil {
				t.Fatal(err)
			}
			for _, quote := range context.Quotes {
				if strings.TrimSpace(quote) == "" || !strings.Contains(string(b), quote) {
					t.Fatalf("context quote missing: %s", context.SourceID)
				}
			}
			k.Sources = append(k.Sources, domain.Source{ID: doc.ID, URL: doc.URL,
				Version: "retrieved-2026-11-09-" + doc.PDFHash, RetrievedAt: doc.Retrieved,
				DocumentFile: pdf, TextFile: text, DocumentSHA256: doc.PDFHash, TextSHA256: doc.TextHash})
			sourceIDs[doc.ID] = true
		}
		if !sourceIDs[context.SourceID] {
			t.Fatalf("missing source %s", context.SourceID)
		}
	}
	b, _ := json.Marshal(k)
	snapshot, err := rules.Load(b, dataRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range suite.Cases {
		t.Run(c.ID, func(t *testing.T) {
			if ids[c.ID] || !c.Synthetic || string(c.Clinical) != "null" || c.Review != "NOT_REVIEWED" || c.Expected.Dose {
				t.Fatal("duplicate case or fabricated clinical reference/approval")
			}
			ids[c.ID] = true
			var req domain.Request
			if err := domain.Decode(c.Request, &req); err != nil {
				t.Fatal(err)
			}
			if len(c.SourceIDs) != len(req.Medications) {
				t.Fatal("medication/source mapping lost")
			}
			for _, id := range c.SourceIDs {
				if !sourceIDs[id] {
					t.Fatal("unknown source", id)
				}
			}
			a := Assess(req, snapshot)
			if len(a.Results) != len(req.Medications) || !reflect.DeepEqual(a.Renal.Warnings, c.Expected.Warnings) {
				t.Fatalf("result count or warnings differ: %+v", a)
			}
			metrics := []domain.Metric{}
			for _, estimate := range a.Renal.Estimates {
				metrics = append(metrics, estimate.Metric)
				if estimate.Metric == domain.CrCl {
					for _, pair := range []struct {
						expected *float64
						actual   float64
					}{
						{c.Expected.CrCl, estimate.Value}, {c.Expected.Low, estimate.Low}, {c.Expected.High, estimate.High},
					} {
						if pair.expected != nil && math.Abs(*pair.expected-pair.actual) > 1e-10 {
							t.Fatalf("CrCl mismatch: expected %v, got %+v", *pair.expected, estimate)
						}
					}
				}
			}
			if !reflect.DeepEqual(metrics, c.Expected.Metrics) {
				t.Fatalf("metric substitution/omission: %v, expected %v", metrics, c.Expected.Metrics)
			}
			for i, r := range a.Results {
				if r.Context != req.Medications[i] || r.Status != c.Expected.Status ||
					!reflect.DeepEqual(r.Reasons, []string{c.Expected.Reason}) || r.Action != "" ||
					r.DoseText != "" || r.Rule != nil || r.Source != nil || r.Selected != nil || r.PrescriptionCheck != nil {
					t.Fatalf("unsafe result or incorrect medication association: %+v", r)
				}
			}
		})
	}
}
