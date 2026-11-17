package extraction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fixture() (SourceInput, ExtractionOutput) {
	text := "Fictional fixture. CrCl < 1,5 ml/min: 2,5 mg every 12 hours."
	in := SourceInput{PDFHash: "fixture", Passages: []SourcePassage{{ID: "chunk", Section: "4.2", Text: text}}}
	out := ExtractionOutput{Findings: []ExtractionFinding{{Summary: "Fictional finding", Facts: []ExtractionFact{{Field: "upper_value", Value: "1.5", ChunkID: "chunk", Quote: text}, {Field: "upper_operator", Value: "<", ChunkID: "chunk", Quote: text}}, Unresolved: []string{}}}, Reason: ""}
	return in, out
}
func providerFixture(t *testing.T, in SourceInput, out ExtractionOutput) []byte {
	t.Helper()
	prepared, err := prepareSource(in)
	if err != nil {
		t.Fatal(err)
	}
	wire := modelOutput{Findings: []modelFinding{}, Abstain: &out.Abstain, Reason: &out.Reason}
	for _, finding := range out.Findings {
		f := modelFinding{Summary: finding.Summary, Facts: []contextReference{}, Unresolved: finding.Unresolved}
		for _, fact := range finding.Facts {
			id := ""
			for _, block := range prepared.Evidence {
				if block.chunkID == fact.ChunkID && strings.Contains(block.Text, fact.Quote) {
					id = block.ID
					break
				}
			}
			if id == "" {
				t.Fatal("fixture quote not found")
			}
			value := &modelValue{Value: fact.Value, EvidenceID: id}
			switch fact.Field {
			case "lower_value", "upper_value", "lower_operator", "upper_operator":
				if f.Band == nil {
					f.Band = &modelBand{EvidenceID: id}
				}
				switch fact.Field {
				case "lower_value":
					f.Band.LowerValue = fact.Value
				case "upper_value":
					f.Band.UpperValue = fact.Value
				case "lower_operator":
					f.Band.LowerOperator = fact.Value
				case "upper_operator":
					f.Band.UpperOperator = fact.Value
				}
			case "metric":
				f.Metric = value
			case "method":
				f.Method = value
			case "renal_unit":
				f.RenalUnit = value
			case "dose_text":
				f.DoseText = value
			case "dose_value":
				f.DoseValue = value
			case "dose_unit":
				f.DoseUnit = value
			case "interval_value":
				f.IntervalValue = value
			case "interval_unit":
				f.IntervalUnit = value
			default:
				f.Facts = append(f.Facts, contextReference{EvidenceID: id})
			}
		}
		if f.Band != nil {
			switch {
			case f.Band.LowerValue != "" && f.Band.UpperValue != "":
				f.Band.Kind = "printed_range"
			case f.Band.UpperValue != "":
				f.Band.Kind = "upper_bound"
			default:
				f.Band.Kind = "lower_bound"
			}
		}
		wire.Findings = append(wire.Findings, f)
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOpenAIResponsesContract(t *testing.T) {
	in, out := fixture()
	raw := providerFixture(t, in, out)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-secret" || r.Method != "POST" {
			t.Error("missing server credential")
		}
		var request map[string]any
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Fatal("request JSON")
		}
		format := request["text"].(map[string]any)["format"].(map[string]any)
		if request["store"] != false || request["truncation"] != "disabled" || format["strict"] != true || format["type"] != "json_schema" || request["instructions"] != Prompt {
			t.Error("provider contract changed")
		}
		if strings.Contains(request["input"].(string), "fixture-secret") {
			t.Error("credential in prompt")
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "resp_fixture", "model": "configured-model-snapshot", "status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": string(raw)}}}}})
	}))
	defer server.Close()
	p, _ := NewOpenAI("fixture-secret", "configured-model")
	result, err := p.extract(context.Background(), in, server.URL)
	if err != nil || result.ResponseID != "resp_fixture" || result.Model != "configured-model-snapshot" {
		t.Fatal(result, err)
	}
}

func TestOpenAIRetrievalContract(t *testing.T) {
	in, out := fixture()
	in.Passages = append(in.Passages, retrievalFixture().Passages[3])
	raw := providerFixture(t, in, out)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var sent preparedSource
		if err := json.Unmarshal([]byte(request.Input), &sent); err != nil {
			t.Fatal(err)
		}
		if sent.Coverage != filteredReview || len(sent.Evidence) != 1 || sent.Evidence[0].ID != "e0001" {
			t.Fatal("unexpected selected source", sent)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id": "resp_fixture", "model": "configured-model", "status": "completed",
			"usage":  map[string]any{"input_tokens": 300, "output_tokens": 100, "total_tokens": 400, "input_tokens_details": map[string]int{"cached_tokens": 200}},
			"output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": string(raw)}}}},
		})
	}))
	defer server.Close()
	p, _ := NewOpenAI("fixture-secret", "configured-model")
	result, err := p.WithRetrieval(nil).extract(t.Context(), in, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Retrieval == nil || !result.Retrieval.Filtered || result.Retrieval.SelectedBlocks != 1 || result.Usage == nil || result.Usage.TotalTokens != 400 || result.Usage.InputTokenDetails.CachedTokens != 200 {
		t.Fatal("missing retrieval or usage audit", result)
	}
	conditions := strings.Join(result.Output.Findings[0].Unresolved, "\n")
	if !strings.Contains(conditions, filteredReview) {
		t.Fatal("missing full-document review requirement")
	}
}

func TestSelectedDefaultModel(t *testing.T) {
	p, err := NewOpenAI("fixture-secret", "")
	if err != nil || p.Model() != "gpt-5.4-mini" {
		t.Fatal("selected default model changed", err)
	}
	if _, err = NewOpenAI("", ""); err == nil {
		t.Fatal("missing API key accepted")
	}
}
func TestInvalidProviderResponses(t *testing.T) {
	in, out := fixture()
	raw := providerFixture(t, in, out)
	for _, tc := range []struct {
		name, status, kind, text string
		httpStatus               int
	}{
		{"incomplete", "incomplete", "output_text", string(raw), 200},
		{"wrong model", "completed", "output_text", string(raw), 200},
		{"refused", "completed", "refusal", "", 200},
		{"malformed", "completed", "output_text", "{}", 200},
		{"missing explicit abstain", "completed", "output_text", `{"findings":[],"reason":"none"}`, 200},
		{"unknown field", "completed", "output_text", `{"findings":[],"abstain":true,"reason":"none","approve":true}`, 200},
		{"provider auth", "completed", "output_text", "fixture-secret", 401},
		{"rate limited", "completed", "output_text", "fixture-secret", 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.httpStatus)
				returnedModel := "model"
				if tc.name == "wrong model" {
					returnedModel = "different-model"
				}
				json.NewEncoder(w).Encode(map[string]any{"id": "resp", "model": returnedModel, "status": tc.status, "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": tc.kind, "text": tc.text}}}}})
			}))
			defer server.Close()
			p, _ := NewOpenAI("fixture-secret", "model")
			_, err := p.extract(context.Background(), in, server.URL)
			if err == nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal("unsafe provider error", err)
			}
		})
	}
}
func TestEvidenceRejection(t *testing.T) {
	for _, kind := range []string{"fabricated", "wrong-chunk", "ambiguous", "decimal-fragment", "unknown-field", "empty-facts"} {
		t.Run(kind, func(t *testing.T) {
			in, out := fixture()
			f := &out.Findings[0].Facts[0]
			switch kind {
			case "fabricated":
				f.Quote = "invented"
			case "wrong-chunk":
				f.ChunkID = "other"
			case "ambiguous":
				in.Passages[0].Text += in.Passages[0].Text
			case "decimal-fragment":
				f.Value = "5"
			case "unknown-field":
				f.Field = "approve"
			case "empty-facts":
				out.Findings[0].Facts = []ExtractionFact{}
			}
			if Validate(out, in) == nil {
				t.Fatal("unsupported finding accepted")
			}
		})
	}
	in, _ := fixture()
	if Validate(ExtractionOutput{Findings: []ExtractionFinding{}, Abstain: true, Reason: "No instruction supported"}, in) != nil {
		t.Fatal("valid abstention rejected")
	}
	p, _ := NewOpenAI("secret", "model")
	in.Passages[0].Text = strings.Repeat("x", MaxInputBytes)
	if _, err := p.extract(context.Background(), in, "invalid-url"); err == nil {
		t.Fatal("oversized source was not rejected before network")
	}
}
