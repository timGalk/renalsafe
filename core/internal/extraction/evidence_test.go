package extraction

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEvidenceBlocksPreserveEveryByte(t *testing.T) {
	text := "4.2 Dawkowanie\n\nZażółć gęślą jaźń.\n\n Klirens   Dawka   Częstość\n 10-25     pół     co 12 godzin\n 26-50     jedna   co 12 godzin\n\n\fPrzypis.\n"
	in := SourceInput{Passages: []SourcePassage{{ID: "source", Section: "4.2", Text: text}}}
	prepared, err := prepareSource(in)
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, b := range prepared.Evidence {
		all.WriteString(b.Text)
		if text[b.start:b.end] != b.Text {
			t.Fatal("offset mismatch")
		}
	}
	if all.String() != text {
		t.Fatal("source text cropped or changed")
	}
	for _, b := range prepared.Evidence {
		if strings.Contains(b.Text, "10-25") && !strings.Contains(b.Text, "Klirens") {
			t.Fatal("table detached from heading")
		}
	}
}

func TestResolveRepeatedEvidenceByOccurrence(t *testing.T) {
	in := SourceInput{Passages: []SourcePassage{{ID: "source", Section: "4.2", Text: "same phrase\n\nsame phrase\n\n"}}}
	prepared, err := prepareSource(in)
	if err != nil {
		t.Fatal(err)
	}
	no := false
	reason := ""
	wire := modelOutput{Findings: []modelFinding{{Summary: "fixture", Facts: []contextReference{{EvidenceID: prepared.Evidence[1].ID}}, Unresolved: []string{}}}, Abstain: &no, Reason: &reason}
	b, _ := json.Marshal(wire)
	out, err := resolveModelOutput(b, prepared, in)
	if err != nil {
		t.Fatal(err)
	}
	fact := out.Findings[0].Facts[0]
	if fact.StartByte == nil || *fact.StartByte != prepared.Evidence[1].start || fact.EndByte == nil || in.Passages[0].Text[*fact.StartByte:*fact.EndByte] != fact.Quote {
		t.Fatal("exact occurrence lost")
	}
	// Unknown references and forged offsets fail; repeated text is never guessed.
	wire.Findings[0].Facts[0].EvidenceID = "e9999"
	b, _ = json.Marshal(wire)
	if _, err = resolveModelOutput(b, prepared, in); err == nil {
		t.Fatal("fabricated evidence id accepted")
	}
	wrong := 1
	fact.StartByte = &wrong
	out.Findings[0].Facts[0] = fact
	if Validate(out, in) == nil {
		t.Fatal("changed source offsets accepted")
	}
}

func TestRegimenAndFactTypeRejection(t *testing.T) {
	for _, kind := range []string{"mixed-bands", "number-in-operator", "text-in-number", "operator-without-bound", "reversed-bounds", "synthesized-phrase"} {
		t.Run(kind, func(t *testing.T) {
			in, out := fixture()
			f := out.Findings[0].Facts[0]
			switch kind {
			case "mixed-bands":
				out.Findings[0].Facts = append(out.Findings[0].Facts, f)
			case "number-in-operator":
				out.Findings[0].Facts = append(out.Findings[0].Facts, ExtractionFact{Field: "upper_operator", Value: "1.5", ChunkID: f.ChunkID, Quote: f.Quote})
			case "text-in-number":
				out.Findings[0].Facts[0].Value = "2,5 mg every 12 hours"
			case "operator-without-bound":
				out.Findings[0].Facts[0] = ExtractionFact{Field: "upper_operator", Value: "<", ChunkID: f.ChunkID, Quote: f.Quote}
			case "reversed-bounds":
				out.Findings[0].Facts = append(out.Findings[0].Facts, ExtractionFact{Field: "lower_value", Value: "12", ChunkID: f.ChunkID, Quote: f.Quote})
			case "synthesized-phrase":
				out.Findings[0].Facts = append(out.Findings[0].Facts, ExtractionFact{Field: "dose_text", Value: "2.5 mg twice daily", ChunkID: f.ChunkID, Quote: f.Quote})
			}
			if Validate(out, in) == nil {
				t.Fatal("invalid finding accepted")
			}
		})
	}
}

func TestWireSchemaUsesTypedValuesAndNoModelQuotes(t *testing.T) {
	properties := Schema()["properties"].(map[string]any)
	finding := properties["findings"].(map[string]any)["items"].(map[string]any)
	facts := finding["properties"].(map[string]any)["facts"].(map[string]any)["items"].(map[string]any)
	p := facts["properties"].(map[string]any)
	if p["evidence_id"] == nil || p["quote"] != nil || p["chunk_id"] != nil {
		t.Fatal("model can author quotations")
	}
	props := finding["properties"].(map[string]any)
	for _, field := range []string{"band", "dose_text", "dose_value", "dose_unit", "interval_value", "interval_unit"} {
		if props[field] == nil {
			t.Fatal("missing fixed regimen slot", field)
		}
	}
	if len(props["band"].(map[string]any)["anyOf"].([]any)) != 5 {
		t.Fatal("typed renal bands missing")
	}
}

func TestPrintedBandAssociation(t *testing.T) {
	text := "Klirens ml/min    Dawka\n26-50  jedna\n10-25 pół\n< 10 pół\n"
	for _, tc := range []struct {
		b     modelBand
		valid bool
	}{
		{modelBand{Kind: "printed_range", LowerValue: "26", UpperValue: "50"}, true},
		{modelBand{Kind: "printed_range", LowerValue: "26", UpperValue: "25"}, false},
		{modelBand{Kind: "printed_range", LowerValue: "26", UpperValue: "50", LowerOperator: ">="}, false},
		{modelBand{Kind: "upper_bound", UpperValue: "10", UpperOperator: "<"}, true},
		{modelBand{Kind: "upper_bound", UpperValue: "10", UpperOperator: "<="}, false},
		{modelBand{Kind: "lower_bound", LowerValue: "10", LowerOperator: ">"}, false},
	} {
		if validBand(tc.b, text) != tc.valid {
			t.Fatalf("wrong band association %+v", tc)
		}
	}
}

func TestMetricSpellingRestoredFromUniqueSourceOnly(t *testing.T) {
	for _, tc := range []struct {
		name, field, value, source string
		valid                      bool
	}{
		{"metric case", "metric", "klirens kreatyniny", "Klirens kreatyniny (ml/min).", true},
		{"unknown metric", "metric", "GFR", "Klirens kreatyniny (ml/min).", false},
		{"ambiguous metric", "metric", "klirens kreatyniny", "Klirens kreatyniny; Klirens kreatyniny", false},
		{"unit case stays exact", "dose_unit", "Mg", "Dawka 5 mg", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := SourceInput{Passages: []SourcePassage{{ID: "source", Section: "4.2", Text: tc.source}}}
			prepared, err := prepareSource(in)
			if err != nil {
				t.Fatal(err)
			}
			no := false
			reason := ""
			f := modelFinding{Summary: "fixture", Facts: []contextReference{}, Unresolved: []string{}}
			value := &modelValue{Value: tc.value, EvidenceID: prepared.Evidence[0].ID}
			if tc.field == "metric" {
				f.Metric = value
			} else {
				f.DoseUnit = value
			}
			raw, _ := json.Marshal(modelOutput{Findings: []modelFinding{f}, Abstain: &no, Reason: &reason})
			out, err := resolveModelOutput(raw, prepared, in)
			if (err == nil) != tc.valid {
				t.Fatalf("expected valid=%v, got %v", tc.valid, err)
			}
			if err == nil && (out.Findings[0].Facts[0].Value != "Klirens kreatyniny" || out.Findings[0].Facts[0].ModelValue != tc.value) {
				t.Fatal("original model value or exact source spelling lost")
			}
		})
	}
}
