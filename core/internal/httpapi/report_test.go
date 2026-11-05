package httpapi

import (
	"bytes"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/store"
	"strings"
	"testing"
)

func TestPrintableReportEscapesSourceText(t *testing.T) {
	attack := `<script>alert("untrusted source")</script>`
	export := store.AssessmentExport{ID: "synthetic", PatientCode: "SYN-fixture", Assessment: domain.Assessment{Results: []domain.Result{{Context: domain.Context{Product: attack}, Explanation: attack, Rule: &domain.Rule{Citation: domain.Citation{Quote: attack}}}}}}
	var out bytes.Buffer
	if e := reportTemplate.Execute(&out, export); e != nil {
		t.Fatal(e)
	}
	html := out.String()
	if strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("source markup was not escaped")
	}
}
