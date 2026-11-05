package evidence

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPassagesLosslessTableAndProductScope(t *testing.T) {
	text := "other product\f4.2 Posology\n\n" + strings.Repeat("Zażółć gęślą. ", 20) + "\n\nRenal table and units\n\nCrCl  Dose\n\n10  20\n\f20  30\n\nFootnote applies to the table.\n\n" + strings.Repeat("Review conditions. ", 20) + "\n4.3 Contraindications\nNone in fixture\fother product"
	sections, err := Split(text, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	passages, err := Passages(sections, 180)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	table := false
	for _, p := range passages {
		if text[p.Start:p.End] != p.Text || !utf8.ValidString(p.Text) {
			t.Fatal("invalid byte provenance")
		}
		if p.PageStart != 1+strings.Count(text[:p.Start], "\f") || p.PageEnd != 1+strings.Count(text[:p.End], "\f") {
			t.Fatal("physical page mismatch")
		}
		if strings.Contains(p.Text, "other product") {
			t.Fatal("escaped product scope")
		}
		if strings.Contains(p.Text, "CrCl") {
			table = p.Table && strings.Contains(p.Text, "Renal table and units") && strings.Contains(p.Text, "20  30") && strings.Contains(p.Text, "Footnote applies")
		}
		joined += p.Text
	}
	expected := ""
	for _, s := range sections {
		expected += s.Text
	}
	if joined != expected || !table || len(passages) <= len(sections) {
		t.Fatal("loss, split table, or no partition")
	}
	again, _ := Passages(sections, 180)
	for i := range passages {
		if passages[i] != again[i] {
			t.Fatal("nondeterministic")
		}
	}
}

func TestPassagesKeepsIndivisibleBlocks(t *testing.T) {
	text := "4.2 Posology\nHeader  Dose\n" + strings.Repeat("10  20\n", 100)
	s := Section{Name: "4.2", PageStart: 1, PageEnd: 1, End: len(text), Text: text, Table: true}
	out, err := Passages([]Section{s}, 100)
	if err != nil || len(out) != 1 || out[0].Text != text {
		t.Fatal("table truncated", err)
	}
	if _, err = Passages([]Section{s}, 0); err == nil {
		t.Fatal("invalid target")
	}
	s.End++
	if _, err = Passages([]Section{s}, 100); err == nil {
		t.Fatal("invalid span")
	}
}
