package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSectionOffsetsTablesAndPageBoundaries(t *testing.T) {
	text := "1. NAME OF THE MEDICINAL PRODUCT\nFixture\n\f4.2 Posology\nA  10  20\n\fB  30  40\n4.3 Contraindications\nFixture renal\n"
	out, e := Split(text, 1, 3)
	if e != nil || len(out) != 2 {
		t.Fatal(e, out)
	}
	if !out[0].Table || out[0].PageStart != 2 || out[0].PageEnd != 3 || !strings.Contains(out[0].Text, "B  30  40") {
		t.Fatal("table or physical page lost", out[0])
	}
	for _, c := range out {
		if text[c.Start:c.End] != c.Text {
			t.Fatal("wrong byte provenance")
		}
	}
}
func TestScopeRejectsOtherCombinedProducts(t *testing.T) {
	text := "cover\f1. NAME OF THE MEDICINAL PRODUCT\nFirst\n4.3 Contraindications\nrenal first\f1. NAME OF THE MEDICINAL PRODUCT\nSecond\n4.3 Contraindications\nrenal second\f"
	first, last, e := PrimaryProductScope(text)
	if e != nil || first != 2 || last != 2 {
		t.Fatal(first, last, e)
	}
	out, e := Split(text, first, last)
	if e != nil || len(out) != 1 || strings.Contains(out[0].Text, "second") {
		t.Fatal(e, out)
	}
}
func TestInvalidScopesAndEmptyText(t *testing.T) {
	for _, v := range []struct {
		text        string
		first, last int
	}{{"4.3 renal", 0, 1}, {"4.3 renal", 1, 5}, {"no headings", 1, 1}, {"4.3 renal\x00", 1, 1}} {
		if _, e := Split(v.text, v.first, v.last); e == nil {
			t.Fatal("invalid scope accepted")
		}
	}
}
func TestActualPDFExtraction(t *testing.T) {
	b, e := os.ReadFile("../../../data/sources/pradaxa-ema-2026-10-14.pdf")
	if e != nil {
		t.Fatal(e)
	}
	text, e := (Poppler{}).Extract(t.Context(), b)
	if e != nil {
		t.Fatal(e)
	}
	first, last, e := PrimaryProductScope(text)
	if e != nil {
		t.Fatal(e)
	}
	sections, e := Split(text, first, last)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, s := range sections {
		if s.Name == "4.3" && s.PageStart == 6 && strings.Contains(s.Text, "Severe renal impairment (CrCL < 30 mL/min) in adult patients") {
			found = true
		}
	}
	if !found {
		t.Fatal("real source citation/page missing")
	}
}

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func TestOllamaContractAndInvalidEmbeddings(t *testing.T) {
	for _, mode := range []string{"valid", "wrong_model", "wrong_dimensions", "zero_vector", "status", "network"} {
		t.Run(mode, func(t *testing.T) {
			o, e := NewOllama("http://127.0.0.1:11434", "fixture-embeddings")
			if e != nil {
				t.Fatal(e)
			}
			o.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api/embed" {
					t.Fatal(r.URL)
				}
				var in struct {
					Model    string   `json:"model"`
					Input    []string `json:"input"`
					Truncate bool     `json:"truncate"`
				}
				if e := json.NewDecoder(r.Body).Decode(&in); e != nil || in.Truncate || len(in.Input) != 1 {
					t.Fatal("contract", e)
				}
				if mode == "network" {
					return nil, fmt.Errorf("offline")
				}
				v := make([]float64, Dimensions)
				v[0] = 1
				model := o.Model()
				status := 200
				if mode == "wrong_model" {
					model = "other"
				}
				if mode == "wrong_dimensions" {
					v = v[:2]
				}
				if mode == "zero_vector" {
					v[0] = 0
				}
				if mode == "status" {
					status = 503
				}
				b, _ := json.Marshal(map[string]any{"model": model, "embeddings": [][]float64{v}})
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(b))), Header: make(http.Header)}, nil
			})}
			v, e := o.Embed(context.Background(), []string{"fixture renal"})
			if mode == "valid" {
				if e != nil || len(v) != 1 {
					t.Fatal(e)
				}
			} else if e == nil {
				t.Fatal("invalid provider accepted")
			}
		})
	}
}
func TestProviderMustBeLocal(t *testing.T) {
	for _, url := range []string{"https://example.org", "file:///tmp/model", "http://127.0.0.1:11434?token=secret"} {
		if _, e := NewOllama(url, "model"); e == nil {
			t.Fatal("nonlocal endpoint accepted")
		}
	}
}
