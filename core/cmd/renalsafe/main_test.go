package main

import (
	"bytes"
	"os"
	"path/filepath"
	"renalsafe/core/internal/audit"
	"testing"
)

func TestOfflineCLIAndAudit(t *testing.T) {
	req, e := os.ReadFile("../../../data/example-request.json")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	var out bytes.Buffer
	if e = run([]string{"-kb", "../../../data/knowledge.json", "-audit", path}, bytes.NewReader(req), &out); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	es, e := audit.Verify(b)
	if e != nil || len(es) != 1 {
		t.Fatal(e)
	}
	r := es[0].Assessment.Results[0]
	if r.Status != "MANUAL_REVIEW_REQUIRED" || r.Action != "" || r.DoseText != "" {
		t.Fatal(r)
	}
	if out.Len() == 0 {
		t.Fatal("missing output")
	}
	out.Reset()
	if e = run([]string{"-verify-audit", path}, bytes.NewReader(nil), &out); e != nil || !bytes.Contains(out.Bytes(), []byte(`"verified_events":1`)) {
		t.Fatal(e, out.String())
	}
}
func TestCLIFailureNeverEmitsAssessment(t *testing.T) {
	req, _ := os.ReadFile("../../../data/example-request.json")
	for _, c := range []struct {
		args []string
		data []byte
	}{{[]string{"-kb", "../../../data/knowledge.json"}, req}, {[]string{"-kb", "../../../data/knowledge.json", "-audit", filepath.Join(t.TempDir(), "missing", "a")}, req}, {[]string{"-kb", "../../../data/knowledge.json", "-audit", filepath.Join(t.TempDir(), "a")}, []byte(`{"unexpected":true}`)}} {
		var out bytes.Buffer
		if e := run(c.args, bytes.NewReader(c.data), &out); e == nil || out.Len() != 0 {
			t.Fatal("failed open", e, out.String())
		}
	}
}
