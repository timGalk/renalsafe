package extraction

import (
	"encoding/json"
	"net/http"
	"renalsafe/core/internal/domain"
	"strings"
	"testing"
)

func TestCombinationReviewsSelectOriginalExcerpts(t *testing.T) {
	openAI, _ := NewOpenAI("fixture", "test-model")
	nvidia, _ := NewNVIDIA("fixture", "")
	passages := []CombinationPassage{{
		Context: domain.Context{Product: "TRAINING A"}, Section: "4.4", Text: "Synthetic fixture:\nreview kidney function.",
		Source:     domain.Source{URL: "https://example.invalid/local-provenance", Version: "v1"},
		DocumentID: "local-document-id", PageStart: 2, PageEnd: 2,
	}}
	const modelJSON = `{"findings":[{"topic":"MONITORING","passage_ids":[1]}]}`
	for _, tc := range []struct {
		name, endpoint string
		client         *http.Client
		reviewer       CombinationReviewer
		reply          map[string]any
	}{
		{"openai", openAIEndpoint, openAI.client, openAI, map[string]any{
			"id": "fixture-response", "model": "test-model", "status": "completed",
			"output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{
				map[string]any{"type": "output_text", "text": modelJSON},
			}}},
		}},
		{"nvidia", nvidiaEndpoint, nvidia.client, nvidia, nvidiaResponse([]byte(modelJSON))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.client.Transport = extractionTransport(func(req *http.Request) (*http.Response, error) {
				var payload struct {
					Input    string `json:"input"`
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if req.URL.String() != tc.endpoint || json.NewDecoder(req.Body).Decode(&payload) != nil {
					t.Fatal("unexpected provider request")
				}
				input := payload.Input
				if tc.name == "nvidia" {
					input = payload.Messages[1].Content
				}
				if !strings.Contains(input, `"passage_id":1`) || !strings.Contains(input, "review kidney function") ||
					strings.Contains(input, "local-document-id") || strings.Contains(input, "local-provenance") {
					t.Fatal("unexpected source payload", input)
				}
				return responseJSON(tc.reply), nil
			})
			out, err := tc.reviewer.ReviewCombination(t.Context(), passages)
			if err != nil || out.Provider != tc.name || len(out.Findings) != 1 {
				t.Fatal(out, err)
			}
			citation := out.Findings[0].Citations[0]
			if citation.Quote != passages[0].Text || citation.DocumentID != passages[0].DocumentID || citation.PageStart != 2 {
				t.Fatal("citation was not resolved to the original passage", citation)
			}
		})
	}
}

func TestCombinationRejectsUnsupportedModelOutput(t *testing.T) {
	passages := []CombinationPassage{{Text: "Synthetic fixture only: review kidney function."}}
	for _, raw := range []string{
		`{"findings":[{"topic":"MONITORING","passage_ids":[2]}]}`,
		`{"findings":[{"topic":"MONITORING","passage_ids":[1],"quote":"model-generated text"}]}`,
		`{"findings":[{"topic":"POTENTIAL_CONFLICT","passage_ids":[1]}]}`,
		`{"findings":[{"topic":"POTENTIAL_CONFLICT","passage_ids":[1,1]}]}`,
		`{"findings":[{"topic":"MONITORING","recommendation":"fictional dose","passage_ids":[1]}]}`,
	} {
		if _, err := resolveCombination([]byte(raw), passages, ExtractionResult{}); err == nil {
			t.Errorf("accepted unsupported model output %s", raw)
		}
	}
}

func TestCombinationExcerptsPreserveTextAndPages(t *testing.T) {
	passage := CombinationPassage{
		Text:      "Context on page 3.\n\nTable line 1\nTable line 2\fTable continues on page 4.\n\nSeparate note on page 4.",
		PageStart: 3, PageEnd: 4, DocumentID: "source",
	}
	excerpts := passage.Excerpts()
	if len(excerpts) != 3 {
		t.Fatalf("unexpected number of excerpts: %d", len(excerpts))
	}
	if excerpts[1].PageStart != 3 || excerpts[1].PageEnd != 4 || excerpts[2].PageStart != 4 {
		t.Fatal("excerpts lost physical page spans")
	}
	var joined strings.Builder
	for _, excerpt := range excerpts {
		if excerpt.DocumentID != passage.DocumentID {
			t.Fatal("excerpt lost its source identity")
		}
		joined.WriteString(excerpt.Text)
	}
	if joined.String() != passage.Text {
		t.Fatal("source text changed when producing excerpts")
	}
}
