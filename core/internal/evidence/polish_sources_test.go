package evidence

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This verifies source ingestion and provenance, not clinical rule semantics.
func TestPolishChPLSourceExtraction(t *testing.T) {
	const root = "../../../data/sources/pl-2026-10-23"
	var manifest struct {
		Documents []struct {
			Product    string `json:"product"`
			PDF        string `json:"pdf_file"`
			PDFHash    string `json:"pdf_sha256"`
			Text       string `json:"text_file"`
			TextHash   string `json:"text_sha256"`
			Pages      int    `json:"pdf_pages"`
			Review     string `json:"clinical_review_status"`
			Activation string `json:"rule_activation"`
		} `json:"documents"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &manifest); err != nil || len(manifest.Documents) != 2 {
		t.Fatal("two pinned Polish documents required", err)
	}
	for _, doc := range manifest.Documents {
		t.Run(doc.Product, func(t *testing.T) {
			readPinned := func(name, digest string) []byte {
				t.Helper()
				b, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != digest {
					t.Fatal("pinned source changed", name, err)
				}
				return b
			}
			pdf := readPinned(doc.PDF, doc.PDFHash)
			readPinned(doc.Text, doc.TextHash)
			text, err := (Poppler{}).Extract(t.Context(), pdf)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Review != "NOT_REVIEWED" || doc.Activation != "NONE" {
				t.Fatal("source acquisition must not assert clinical approval")
			}
			if strings.Count(strings.TrimRight(text, "\f\r\n \t"), "\f")+1 != doc.Pages {
				t.Fatal("physical page count differs")
			}
			// Polish imports require explicit page scope; the automatic EMA
			// first-product detector is deliberately not used here.
			sections, err := Split(text, 1, doc.Pages)
			if err != nil || len(sections) != 11 {
				t.Fatal("expected sections 4.1–4.9, 5.1 and 5.2", err, len(sections))
			}
			foundDosing := false
			for _, sec := range sections {
				if text[sec.Start:sec.End] != sec.Text || sec.PageStart < 1 || sec.PageEnd > doc.Pages {
					t.Fatal("source byte or physical page provenance lost")
				}
				if sec.Name == "4.2" {
					foundDosing = sec.PageStart == 2 && sec.Table &&
						strings.Contains(sec.Text, "Dawkowanie i sposób podawania") &&
						strings.Contains(sec.Text, "Zaburzenia czynności nerek")
					t.Logf("section 4.2: physical pages %d–%d, exact source byte spans retained", sec.PageStart, sec.PageEnd)
				}
			}
			passages, err := Passages(sections, PassageTargetBytes)
			if err != nil {
				t.Fatal(err)
			}
			var before, after strings.Builder
			for _, sec := range sections {
				before.WriteString(sec.Text)
			}
			for _, p := range passages {
				if text[p.Start:p.End] != p.Text {
					t.Fatal("passage source span mismatch")
				}
				after.WriteString(p.Text)
			}
			if before.String() != after.String() {
				t.Fatal("real source content lost or duplicated")
			}
			t.Logf("%d sections partitioned into %d lossless passages", len(sections), len(passages))
			if !foundDosing {
				t.Fatal("Polish dosing/renal passage or table layout marker missing")
			}
		})
	}
}
