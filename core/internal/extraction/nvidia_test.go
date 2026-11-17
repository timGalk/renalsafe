package extraction

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type extractionTransport func(*http.Request) (*http.Response, error)

func (f extractionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func nvidiaResponse(raw []byte) map[string]any {
	return map[string]any{
		"id": "chatcmpl_fixture", "model": DefaultNVIDIAModel,
		"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(raw)}}},
		"usage":   map[string]any{"prompt_tokens": 300, "completion_tokens": 100, "total_tokens": 400, "prompt_tokens_details": map[string]int{"cached_tokens": 200}},
	}
}
func responseJSON(value any) *http.Response {
	b, _ := json.Marshal(value)
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(b)))}
}

func TestNVIDIAContract(t *testing.T) {
	for _, retrieval := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "retrieval"}[retrieval], func(t *testing.T) {
			in, out := fixture()
			in.Passages = append(in.Passages, retrievalFixture().Passages[3])
			raw := providerFixture(t, in, out)
			p, err := NewNVIDIA("nvidia-fixture-secret", "")
			if err != nil {
				t.Fatal(err)
			}
			if retrieval {
				p.WithRetrieval(nil)
			}
			p.client.Transport = extractionTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != nvidiaEndpoint || req.Method != "POST" || req.Header.Get("Authorization") != "Bearer nvidia-fixture-secret" {
					t.Fatal("incorrect NVIDIA routing")
				}
				body, _ := io.ReadAll(req.Body)
				var payload map[string]json.RawMessage
				if json.Unmarshal(body, &payload) != nil || strings.Contains(string(body), "nvidia-fixture-secret") {
					t.Fatal("invalid or credential-bearing body")
				}
				for _, key := range []string{"input", "instructions", "store", "truncation", "response_format", "text"} {
					if _, ok := payload[key]; ok {
						t.Fatalf("unsupported NVIDIA field %s", key)
					}
				}
				if string(payload["model"]) != `"`+DefaultNVIDIAModel+`"` || string(payload["reasoning_effort"]) != `"low"` || string(payload["stream"]) != "false" || string(payload["max_tokens"]) != "10000" {
					t.Fatal("request settings")
				}
				var messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}
				if json.Unmarshal(payload["messages"], &messages) != nil || len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
					t.Fatal("messages")
				}
				schema, _ := json.Marshal(Schema())
				if messages[0].Content != Prompt+nvidiaJSONInstruction+string(schema) {
					t.Fatal("prompt/schema mismatch")
				}
				var prepared preparedSource
				if json.Unmarshal([]byte(messages[1].Content), &prepared) != nil {
					t.Fatal("source JSON")
				}
				if retrieval && (len(prepared.Evidence) != 1 || prepared.Coverage != filteredReview) {
					t.Fatal("retrieval not used")
				}
				return responseJSON(nvidiaResponse(raw)), nil
			})
			result, err := p.Extract(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			if result.Provider != "nvidia" || result.Model != DefaultNVIDIAModel || result.Usage == nil || result.Usage.InputTokens != 300 || result.Usage.InputTokenDetails.CachedTokens != 200 || result.Retrieval.Filtered != retrieval {
				t.Fatal("result metadata", result)
			}
			if retrieval && !strings.Contains(strings.Join(result.Output.Findings[0].Unresolved, "\n"), filteredReview) {
				t.Fatal("missing review requirement")
			}
		})
	}
}

func TestNVIDIARejectsInvalidResponses(t *testing.T) {
	for _, kind := range []string{"http", "pending", "connection", "truncated", "refusal", "tool", "wrong-model", "missing-id", "multiple-choices", "wrong-role", "malformed", "fenced", "unsupported-fact", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			in, out := fixture()
			raw := providerFixture(t, in, out)
			p, _ := NewNVIDIA("fixture", "")
			p.client.Transport = extractionTransport(func(*http.Request) (*http.Response, error) {
				envelope := nvidiaResponse(raw)
				choice := envelope["choices"].([]any)[0].(map[string]any)
				message := choice["message"].(map[string]any)
				switch kind {
				case "http":
					return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("private provider diagnostic"))}, nil
				case "pending":
					return &http.Response{StatusCode: 202, Body: io.NopCloser(strings.NewReader("pending"))}, nil
				case "connection":
					return nil, errors.New("private provider diagnostic")
				case "truncated":
					choice["finish_reason"] = "length"
				case "refusal":
					message["refusal"] = "refused"
				case "tool":
					message["tool_calls"] = []any{map[string]any{"id": "unexpected"}}
				case "wrong-model":
					envelope["model"] = "different-model"
				case "missing-id":
					delete(envelope, "id")
				case "multiple-choices":
					envelope["choices"] = []any{choice, choice}
				case "wrong-role":
					message["role"] = "user"
				case "malformed":
					message["content"] = "not JSON"
				case "fenced":
					message["content"] = "```json\n" + string(raw) + "\n```"
				case "unsupported-fact":
					message["content"] = strings.ReplaceAll(string(raw), `"1.5"`, `"9999"`)
				case "oversized":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", MaxOutputBytes+1)))}, nil
				}
				return responseJSON(envelope), nil
			})
			if _, err := p.Extract(context.Background(), in); err == nil || strings.Contains(err.Error(), "private provider diagnostic") {
				t.Fatal("invalid response accepted or diagnostics leaked", err)
			}
		})
	}
}

func TestProviderConfigurationSeparatesCredentials(t *testing.T) {
	values := map[string]string{"OPENAI_API_KEY": "openai-only", "NVIDIA_API_KEY": "nvidia-only", "RENALSAFE_EXTRACTION_PROVIDER": "nvidia"}
	get := func(key string) string { return values[key] }
	p, err := ConfiguredProvider(get, nil)
	if err != nil || ProviderName(p) != "nvidia" || p.Model() != DefaultNVIDIAModel || p.(*NVIDIA).key != "nvidia-only" {
		t.Fatal(p, err)
	}
	baseline := ProviderManifest(p)
	values["RENALSAFE_EXTRACTION_RETRIEVAL"] = "hybrid"
	p, err = ConfiguredProvider(get, nil)
	if err != nil || ProviderManifest(p) == baseline || !p.(*NVIDIA).retrieval {
		t.Fatal("retrieval configuration not pinned", err)
	}
	delete(values, "NVIDIA_API_KEY")
	if p, err = ConfiguredProvider(get, nil); p != nil || err != nil {
		t.Fatal("must not fall back to OpenAI credentials")
	}
	values["RENALSAFE_EXTRACTION_PROVIDER"] = "openai"
	p, err = ConfiguredProvider(get, nil)
	if err != nil || p.(*OpenAI).key != "openai-only" || ProviderManifest(p) == baseline {
		t.Fatal("provider identity not pinned", err)
	}
	values["RENALSAFE_EXTRACTION_PROVIDER"] = "unknown"
	if _, err = ConfiguredProvider(get, nil); err == nil {
		t.Fatal("unknown provider")
	}
	if _, err = NewNVIDIA("", ""); err == nil {
		t.Fatal("missing key")
	}
}

func TestNVIDIAModelSpecificSettings(t *testing.T) {
	for _, tc := range []struct{ model, effort string }{
		{"mistralai/mistral-large-4-1t-a52b", "high"},
		{"meta/muse-glimmer-30b", "low"},
		{"z-ai/glm-5.3-flash", ""},
	} {
		t.Run(tc.model, func(t *testing.T) {
			p, err := NewNVIDIA("fixture", tc.model)
			if err != nil {
				t.Fatal(err)
			}
			settings := p.GenerationSettings()
			if tc.effort == "" {
				if _, ok := settings["reasoning_effort"]; ok {
					t.Fatal("unsupported reasoning control")
				}
			} else if settings["reasoning_effort"] != tc.effort {
				t.Fatal("wrong reasoning setting")
			}
			if settings["max_tokens"] != 10000 || settings["stream"] != false {
				t.Fatal("budgets changed")
			}
			if _, ok := settings["temperature"]; ok {
				t.Fatal("provider sampling default overridden")
			}
		})
	}
}
