package extraction

import (
	"fmt"
	"renalsafe/core/internal/evidence"
)

// ConfiguredProvider selects a provider without sharing keys between services.
// A missing key disables the selected provider; it never falls back to another.
func ConfiguredProvider(getenv func(string) string, embedder evidence.Embedder) (Provider, error) {
	mode := getenv("RENALSAFE_EXTRACTION_RETRIEVAL")
	if mode != "" && mode != "full" && mode != "hybrid" {
		return nil, fmt.Errorf("RENALSAFE_EXTRACTION_RETRIEVAL must be full or hybrid")
	}
	switch getenv("RENALSAFE_EXTRACTION_PROVIDER") {
	case "", "openai":
		if getenv("OPENAI_API_KEY") == "" {
			return nil, nil
		}
		p, err := NewOpenAI(getenv("OPENAI_API_KEY"), getenv("RENALSAFE_OPENAI_MODEL"))
		if err == nil && mode == "hybrid" {
			p.WithRetrieval(embedder)
		}
		return p, err
	case "nvidia":
		if getenv("NVIDIA_API_KEY") == "" {
			return nil, nil
		}
		p, err := NewNVIDIA(getenv("NVIDIA_API_KEY"), getenv("RENALSAFE_NVIDIA_MODEL"))
		if err == nil && mode == "hybrid" {
			p.WithRetrieval(embedder)
		}
		return p, err
	default:
		return nil, fmt.Errorf("RENALSAFE_EXTRACTION_PROVIDER must be openai or nvidia")
	}
}

func ProviderName(p Provider) string {
	if named, ok := p.(interface{ Name() string }); ok {
		return named.Name()
	}
	return "custom"
}

// Both wire protocols resolve and validate exactly the same evidence envelope.
func finishExtraction(text []byte, prepared preparedSource, input SourceInput, result ExtractionResult) (ExtractionResult, error) {
	var err error
	result.Output, err = resolveModelOutput(text, prepared, input)
	if err != nil {
		return ExtractionResult{}, err
	}
	if result.Retrieval != nil && result.Retrieval.Filtered {
		for i := range result.Output.Findings {
			result.Output.Findings[i].Unresolved = append(result.Output.Findings[i].Unresolved, filteredReview)
		}
		if result.Output.Abstain {
			result.Output.Reason += " " + filteredReview
		}
	}
	if err = Validate(result.Output, input); err != nil {
		return ExtractionResult{}, err
	}
	return result, nil
}
