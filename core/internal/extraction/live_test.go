package extraction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/evidence"
	"strconv"
	"strings"
	"testing"
	"time"
)

type captureLiveResponse struct {
	base          http.RoundTripper
	body          []byte
	input         json.RawMessage
	status        int
	headerSeconds float64
	errorKind     string
}

func (c *captureLiveResponse) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture only the source JSON actually transmitted, never headers or keys.
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		var payload struct {
			Input    string `json:"input"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		err = json.NewDecoder(body).Decode(&payload)
		body.Close()
		if err != nil {
			return nil, err
		}
		c.input = json.RawMessage(payload.Input)
		for _, message := range payload.Messages {
			if message.Role == "user" {
				c.input = json.RawMessage(message.Content)
			}
		}
	}
	started := time.Now()
	res, err := c.base.RoundTrip(req)
	if err != nil {
		var networkError net.Error
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout():
			c.errorKind = "TIMEOUT"
		case errors.Is(err, context.Canceled):
			c.errorKind = "CANCELED"
		default:
			c.errorKind = "CONNECTION_ERROR"
		}
		return nil, err
	}
	c.headerSeconds = time.Since(started).Seconds()
	c.status = res.StatusCode
	// Keep only successful public-document responses for evidence review. Never
	// capture the request Authorization header or provider error response bodies.
	if res.StatusCode == http.StatusOK {
		raw, e := io.ReadAll(io.LimitReader(res.Body, MaxOutputBytes+1))
		res.Body.Close()
		if e != nil {
			return nil, e
		}
		c.body = raw
		res.Body = io.NopCloser(bytes.NewReader(raw))
	}
	return res, nil
}

// Explicit opt-in: normal software tests never make paid provider calls.
func TestLiveMeronemExtraction(t *testing.T) {
	if os.Getenv("RENALSAFE_LIVE_EXTRACTION") != "1" {
		t.Skip("live API smoke: run scripts/smoke-online-extraction.py")
	}
	key := os.Getenv("OPENAI_API_KEY")
	if os.Getenv("RENALSAFE_EXTRACTION_PROVIDER") == "nvidia" {
		key = os.Getenv("NVIDIA_API_KEY")
	}
	if key == "" {
		t.Fatal("selected provider API key is missing")
	}
	root := filepath.Clean("../../..")
	var manifest struct {
		Documents []struct {
			ID       string `json:"product_id"`
			PDF      string `json:"pdf_path"`
			PDFHash  string `json:"pdf_sha256"`
			TextHash string `json:"extracted_text_sha256"`
			Pages    int    `json:"physical_page_count"`
		} `json:"documents"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "data/sources/pl-2026-11-09/manifest.json"))
	if err != nil || json.Unmarshal(raw, &manifest) != nil {
		t.Fatal("source manifest unavailable")
	}
	var pdf []byte
	var hash, textHash string
	var pages int
	for _, doc := range manifest.Documents {
		if doc.ID == "rpl-4282" {
			pdf, err = os.ReadFile(filepath.Join(root, "data/sources/pl-2026-11-09", doc.PDF))
			hash, textHash, pages = doc.PDFHash, doc.TextHash, doc.Pages
			break
		}
	}
	if err != nil || len(pdf) == 0 || domain.BytesDigest(pdf) != hash {
		t.Fatal("pinned Meronem PDF missing or changed")
	}
	timeoutSeconds := 90
	if raw := os.Getenv("RENALSAFE_LIVE_HTTP_TIMEOUT_SECONDS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 600 {
			t.Fatal("live HTTP timeout must be between 1 and 600 seconds")
		}
		timeoutSeconds = value
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Duration(timeoutSeconds+20)*time.Second)
	defer cancel()
	text, err := (evidence.Poppler{}).Extract(ctx, pdf)
	if err != nil {
		t.Fatal(err)
	}
	if domain.BytesDigest([]byte(text)) != textHash {
		t.Fatal("programmatic extraction differs from pinned text")
	}
	sections, err := evidence.Split(text, 1, pages)
	if err != nil {
		t.Fatal(err)
	}
	passages, err := evidence.Passages(sections, evidence.PassageTargetBytes)
	if err != nil {
		t.Fatal(err)
	}
	input := SourceInput{Context: domain.Context{Product: "Meronem 1 g", Route: "intravenous", Form: "powder for solution for injection or infusion", Indication: "hospital-acquired pneumonia"}, PDFHash: hash, Passages: []SourcePassage{}}
	for _, p := range passages {
		switch p.Name {
		case "4.1", "4.2", "4.3", "4.4", "5.2":
			input.Passages = append(input.Passages, SourcePassage{ID: "rpl-4282-" + strconv.Itoa(p.Start) + "-" + strconv.Itoa(p.End), Section: p.Name, Text: p.Text})
		}
	}
	retrievalMode := os.Getenv("RENALSAFE_EXTRACTION_RETRIEVAL")
	var embedder evidence.Embedder
	if retrievalMode == "hybrid" {
		if base := os.Getenv("RENALSAFE_OLLAMA_URL"); base != "" {
			embedder, err = evidence.NewOllama(base, os.Getenv("RENALSAFE_EMBEDDING_MODEL"))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	provider, err := ConfiguredProvider(os.Getenv, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Fatal("selected extraction provider is not configured")
	}
	capture := &captureLiveResponse{base: http.DefaultTransport}
	switch p := provider.(type) {
	case *OpenAI:
		p.client.Transport = capture
		p.client.Timeout = time.Duration(timeoutSeconds) * time.Second
	case *NVIDIA:
		p.client.Transport = capture
		p.client.Timeout = time.Duration(timeoutSeconds) * time.Second
	default:
		t.Fatal("unsupported live provider")
	}
	dir := filepath.Join(root, "outputs/online-extraction-smoke", time.Now().UTC().Format("20060102T150405Z"))
	if override := os.Getenv("RENALSAFE_LIVE_OUTPUT_DIR"); override != "" {
		dir = override
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, value any) {
		t.Helper()
		b, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		b = []byte(strings.ReplaceAll(string(b), key, "[REDACTED]"))
		if e = os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("input.json", input)
	started := time.Now()
	result, callErr := provider.Extract(ctx, input)
	if len(capture.input) > 0 {
		write("provider-input.json", capture.input)
	}
	if len(capture.body) > 0 {
		var response any
		if json.Unmarshal(capture.body, &response) == nil {
			write("provider-response.json", response)
		}
	}
	summary := map[string]any{"source": "Meronem / rpl-4282", "pdf_sha256": hash, "source_digest": domain.Digest(input), "prompt_schema_hash": ProviderManifest(provider), "requested_model": provider.Model(), "requested_retrieval": retrievalMode, "http_status": capture.status, "elapsed_seconds": time.Since(started).Seconds(), "api_calls": 1, "rules_created": 0, "rules_approved": 0, "source_sections": len(input.Passages), "software_validation_passed": callErr == nil, "clinical_review_status": "NOT_INDEPENDENTLY_REVIEWED"}
	summary["http_timeout_seconds"] = timeoutSeconds
	summary["connection_error_kind"] = capture.errorKind
	summary["response_header_seconds"] = capture.headerSeconds
	if p, ok := provider.(*NVIDIA); ok {
		summary["generation_settings"] = p.GenerationSettings()
	}
	if callErr != nil {
		summary["error"] = callErr.Error()
	} else {
		summary["provider"] = ProviderName(provider)
		summary["returned_model"] = result.Model
		summary["response_id"] = result.ResponseID
		summary["findings"] = len(result.Output.Findings)
		summary["retrieval"] = result.Retrieval
		summary["usage"] = result.Usage
		write("result.json", result)
		checks := reviewMeronemSource(result.Output)
		write("source-review.json", checks)
		passed := true
		for _, ok := range checks {
			passed = passed && ok
		}
		summary["meronem_source_regression_passed"] = passed
		if !passed {
			callErr = fmt.Errorf("Meronem source regression failed; inspect source-review.json")
			summary["error"] = callErr.Error()
		}
	}
	write("summary.json", summary)
	t.Log("Source-only live smoke artifacts:", dir)
	if callErr != nil {
		t.Fatal(callErr)
	}
	t.Logf("HTTP %d; %s; %d findings; validated source quotations; no rules created or approved", capture.status, result.Model, len(result.Output.Findings))
}

// Engineering regression against the three printed adult rows in the pinned
// ChPL. This is not an independent clinical label or prescribing approval.
func reviewMeronemSource(out ExtractionOutput) map[string]bool {
	checks := map[string]bool{"three_distinct_adult_rows": true, "relative_doses_not_converted": true, "baseline_dependencies_preserved": true, "method_not_assumed": true, "printed_boundaries_not_interpolated": true, "dialysis_context_preserved": false, "peritoneal_no_recommendation_preserved": false}
	wants := []struct{ lo, hi, op, dose, interval string }{{"26", "50", "", "jedna dawka jednostkowa", "12"}, {"10", "25", "", "pół dawki jednostkowej", "12"}, {"", "10", "<", "pół dawki jednostkowej", "24"}}
	all := strings.Builder{}
	for _, f := range out.Findings {
		for _, fact := range f.Facts {
			all.WriteString(fact.Quote)
			all.WriteString("\n")
		}
	}
	checks["dialysis_context_preserved"] = strings.Contains(normalized(all.String()), "po zakończeniu hemodializy")
	checks["peritoneal_no_recommendation_preserved"] = strings.Contains(normalized(all.String()), "Nie ustalono zaleceń dawkowania u pacjentów poddawanych dializie otrzewnowej.")
	for _, want := range wants {
		matches := 0
		for _, f := range out.Findings {
			values := map[string]string{}
			context := strings.Builder{}
			for _, fact := range f.Facts {
				values[fact.Field] = fact.Value
				context.WriteString(fact.Quote)
				context.WriteString("\n")
			}
			if values["lower_value"] != want.lo || values["upper_value"] != want.hi {
				continue
			}
			matches++
			checks["relative_doses_not_converted"] = checks["relative_doses_not_converted"] && values["dose_text"] == want.dose && values["interval_value"] == want.interval && values["interval_unit"] == "godzin" && values["dose_value"] == ""
			checks["baseline_dependencies_preserved"] = checks["baseline_dependencies_preserved"] && strings.Contains(normalized(context.String()), "500 mg lub 1 g") && len(f.Unresolved) > 0
			checks["method_not_assumed"] = checks["method_not_assumed"] && values["method"] == "" && strings.EqualFold(values["metric"], "klirens kreatyniny") && values["renal_unit"] == "ml/min"
			checks["printed_boundaries_not_interpolated"] = checks["printed_boundaries_not_interpolated"] && values["lower_operator"] == "" && values["upper_operator"] == want.op
		}
		checks["three_distinct_adult_rows"] = checks["three_distinct_adult_rows"] && matches == 1
	}
	return checks
}

// Recheck a saved real response without making another paid API request.
func TestRecordedMeronemSourceReview(t *testing.T) {
	dir := os.Getenv("RENALSAFE_REVIEW_MERONEM_DIR")
	if dir == "" {
		t.Skip("set RENALSAFE_REVIEW_MERONEM_DIR to a saved smoke directory")
	}
	var result ExtractionResult
	var input SourceInput
	read := func(name string, out any) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || json.Unmarshal(b, out) != nil {
			t.Fatal("saved smoke artifact unavailable", name)
		}
	}
	read("result.json", &result)
	read("input.json", &input)
	if err := Validate(result.Output, input); err != nil {
		t.Fatal(err)
	}
	checks := reviewMeronemSource(result.Output)
	raw, _ := json.MarshalIndent(checks, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "source-review.json"), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	for name, passed := range checks {
		if !passed {
			t.Error("source regression failed:", name)
		}
	}
	if !t.Failed() {
		t.Log("All printed-band, dose, interval, dependency and dialysis regression checks passed. No independent clinical approval.")
	}
}
