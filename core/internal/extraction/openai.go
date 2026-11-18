package extraction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"renalsafe/core/internal/evidence"
	"strings"
	"time"
)

type Provider interface {
	Model() string
	Extract(context.Context, SourceInput) (ExtractionResult, error)
}

type OpenAI struct {
	key, model string
	client     *http.Client
	retrieval  bool
	embedder   evidence.Embedder
}

const openAIEndpoint = "https://api.openai.com/v1/responses"
const DefaultModel = "gpt-5.4-mini"
const ReasoningEffort = "low"

func NewOpenAI(key, model string) (*OpenAI, error) {
	if model == "" {
		model = DefaultModel
	}
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n") || strings.TrimSpace(model) == "" || len(model) > 200 {
		return nil, fmt.Errorf("invalid OpenAI API key or model configuration")
	}
	return &OpenAI{key: key, model: model, client: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (o *OpenAI) Model() string { return o.model }
func (o *OpenAI) Name() string  { return "openai" }
func (o *OpenAI) Extract(ctx context.Context, input SourceInput) (ExtractionResult, error) {
	// Fixed HTTPS endpoint prevents sending the key to a caller-selected host.
	return o.extract(ctx, input, openAIEndpoint)
}
func (o *OpenAI) extract(ctx context.Context, input SourceInput, endpoint string) (ExtractionResult, error) {
	var result ExtractionResult
	prepared, err := prepareSource(input)
	if err != nil {
		return result, err
	}
	prepared, result.Retrieval = o.selectSource(ctx, prepared)
	source, err := json.Marshal(prepared)
	if err != nil || len(source) > MaxInputBytes {
		return result, fmt.Errorf("input exceeds extraction budget")
	}
	text, err := o.complete(ctx, endpoint, o.completionPayload(Prompt, source, "renal_findings", Schema(), 10000), &result)
	if err != nil {
		return ExtractionResult{}, err
	}
	return finishExtraction(text, prepared, input, result)
}

func (o *OpenAI) completionPayload(prompt string, source []byte, name string, schema map[string]any, maxTokens int) map[string]any {
	return map[string]any{
		"model": o.model, "instructions": prompt, "input": string(source),
		"store": false, "max_output_tokens": maxTokens, "truncation": "disabled",
		"text":      map[string]any{"format": map[string]any{"type": "json_schema", "name": name, "strict": true, "schema": schema}},
		"reasoning": map[string]string{"effort": ReasoningEffort},
	}
}

func (o *OpenAI) complete(ctx context.Context, endpoint string, payload map[string]any, result *ExtractionResult) ([]byte, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("provider request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.key)
	response, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider connection failed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("provider HTTP %d", response.StatusCode)
	} // Never persist provider error bodies or credentials.
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxOutputBytes+1))
	if err != nil || len(raw) > MaxOutputBytes {
		return nil, fmt.Errorf("provider response exceeds budget")
	}
	var envelope struct {
		Usage  *TokenUsage `json:"usage"`
		ID     string      `json:"id"`
		Model  string      `json:"model"`
		Status string      `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Status != "completed" || envelope.ID == "" || len(envelope.ID) > 200 || envelope.Model == "" || len(envelope.Model) > 200 {
		return nil, fmt.Errorf("provider incomplete or invalid response")
	}
	if envelope.Model != o.model && !strings.HasPrefix(envelope.Model, o.model+"-") {
		return nil, fmt.Errorf("provider returned a different model")
	}
	texts := []string{}
	for _, item := range envelope.Output {
		if item.Type == "message" && item.Role == "assistant" {
			for _, part := range item.Content {
				if part.Type == "refusal" {
					return nil, fmt.Errorf("provider refused request")
				}
				if part.Type == "output_text" {
					texts = append(texts, part.Text)
				}
			}
		}
	}
	if len(texts) != 1 {
		return nil, fmt.Errorf("provider did not return one text response")
	}
	result.Provider = o.Name()
	result.Model, result.ResponseID = envelope.Model, envelope.ID
	result.Usage = envelope.Usage
	return []byte(texts[0]), nil
}
