package extraction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/evidence"
	"strings"
)

const DefaultNVIDIAModel = "moonshotai/kimi-k3.1-pro"
const nvidiaEndpoint = "https://integrate.api.nvidia.com/v1/chat/completions"
const nvidiaProtocol = "nvidia-chat-json-prompt-v2"
const nvidiaJSONInstruction = "\nReturn exactly one JSON object conforming to this JSON Schema. Do not include markdown fences, commentary or tool calls:\n"

// NVIDIA shares source preparation/validation with OpenAI, but uses the
// documented NVIDIA Chat Completions contract and a distinct credential.
type NVIDIA struct{ *OpenAI }

func NewNVIDIA(key, model string) (*NVIDIA, error) {
	if model == "" {
		model = DefaultNVIDIAModel
	}
	base, err := NewOpenAI(key, model)
	if err != nil {
		return nil, fmt.Errorf("invalid NVIDIA API key or model configuration")
	}
	return &NVIDIA{OpenAI: base}, nil
}

func (n *NVIDIA) Name() string { return "nvidia" }
func (n *NVIDIA) WithRetrieval(embedder evidence.Embedder) *NVIDIA {
	n.OpenAI.WithRetrieval(embedder)
	return n
}
func (n *NVIDIA) ManifestHash() string {
	return domain.Digest([]any{n.OpenAI.ManifestHash(), nvidiaProtocol, nvidiaEndpoint, nvidiaJSONInstruction, n.GenerationSettings()})
}

// Only send controls advertised for the selected NVIDIA model. Sampling
// parameters remain at provider defaults; Muse explicitly discourages greedy decoding.
func (n *NVIDIA) GenerationSettings() map[string]any {
	settings := map[string]any{"max_tokens": 10000, "stream": false}
	switch n.model {
	case "mistralai/mistral-large-4-1t-a52b":
		settings["reasoning_effort"] = "high"
	case "meta/muse-glimmer-30b", DefaultNVIDIAModel, "moonshotai/kimi-k3_1-pro":
		settings["reasoning_effort"] = ReasoningEffort
	}
	return settings
}
func (n *NVIDIA) Extract(ctx context.Context, input SourceInput) (ExtractionResult, error) {
	return n.extract(ctx, input, nvidiaEndpoint)
}
func (n *NVIDIA) extract(ctx context.Context, input SourceInput, endpoint string) (ExtractionResult, error) {
	var result ExtractionResult
	prepared, err := prepareSource(input)
	if err != nil {
		return result, err
	}
	prepared, result.Retrieval = n.selectSource(ctx, prepared)
	source, err := json.Marshal(prepared)
	if err != nil || len(source) > MaxInputBytes {
		return result, fmt.Errorf("input exceeds extraction budget")
	}
	text, err := n.complete(ctx, endpoint, n.completionPayload(Prompt, source, Schema()), &result)
	if err != nil {
		return ExtractionResult{}, err
	}
	return finishExtraction(text, prepared, input, result)
}

func (n *NVIDIA) completionPayload(prompt string, source []byte, schema map[string]any) map[string]any {
	encoded, _ := json.Marshal(schema)
	// Supply the schema in instructions and require strict local validation.
	payload := n.GenerationSettings()
	payload["model"] = n.model
	payload["messages"] = []map[string]string{
		{"role": "system", "content": prompt + nvidiaJSONInstruction + string(encoded)},
		{"role": "user", "content": string(source)},
	}
	return payload
}

func (n *NVIDIA) complete(ctx context.Context, endpoint string, payload map[string]any, result *ExtractionResult) ([]byte, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("provider request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.key)
	response, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider connection failed")
	}
	defer response.Body.Close()
	// No automatic paid retries or redirects, including NVIDIA's async 202.
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxOutputBytes+1))
	if err != nil || len(raw) > MaxOutputBytes {
		return nil, fmt.Errorf("provider response exceeds budget")
	}
	var envelope struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string            `json:"role"`
				Content   string            `json:"content"`
				Refusal   *string           `json:"refusal"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens       int                `json:"prompt_tokens"`
			CompletionTokens   int                `json:"completion_tokens"`
			TotalTokens        int                `json:"total_tokens"`
			PromptTokenDetails *InputTokenDetails `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.ID == "" || len(envelope.ID) > 200 || envelope.Model == "" || len(envelope.Model) > 200 || len(envelope.Choices) != 1 {
		return nil, fmt.Errorf("provider incomplete or invalid response")
	}
	if envelope.Model != n.model && !strings.HasPrefix(envelope.Model, n.model+"-") {
		return nil, fmt.Errorf("provider returned a different model")
	}
	choice := envelope.Choices[0]
	if choice.Message.Refusal != nil {
		return nil, fmt.Errorf("provider refused request")
	}
	if choice.FinishReason != "stop" || choice.Message.Role != "assistant" || choice.Message.Content == "" || len(choice.Message.ToolCalls) != 0 {
		return nil, fmt.Errorf("provider incomplete or invalid response")
	}
	result.Provider, result.Model, result.ResponseID = n.Name(), envelope.Model, envelope.ID
	if envelope.Usage != nil {
		result.Usage = &TokenUsage{InputTokens: envelope.Usage.PromptTokens, OutputTokens: envelope.Usage.CompletionTokens, TotalTokens: envelope.Usage.TotalTokens, InputTokenDetails: envelope.Usage.PromptTokenDetails}
	}
	return []byte(choice.Message.Content), nil
}
