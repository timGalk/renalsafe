package extraction

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"renalsafe/core/internal/evidence"
	"strings"
	"testing"
)

func retrievalFixture() SourceInput {
	warnings := strings.Builder{}
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&warnings, "Packaging information paragraph %d.\n\n", i)
	}
	return SourceInput{PDFHash: "fixture", Passages: []SourcePassage{
		{ID: "indication", Section: "4.1", Text: "Fictional indication.\n\n"},
		{ID: "dosing", Section: "4.2", Text: "Renal dose depends on the baseline table.\n\nAll baseline instructions retained.\n\n"},
		{ID: "contraindication", Section: "4.3", Text: "Contraindication without a matching keyword."},
		{ID: "warnings", Section: "4.4", Text: warnings.String()},
	}}
}

func TestHeapRankingAndTieOrder(t *testing.T) {
	hits := bestBlocks([]float64{2, 4, 4, 1, 4, 0}, 2)
	if len(hits) != 2 || hits[0].index != 1 || hits[1].index != 2 {
		t.Fatal(hits)
	}
	if len(bestBlocks([]float64{1}, 0)) != 0 {
		t.Fatal("zero capacity")
	}
}

func TestRetrievalPreservesContextAndOffsets(t *testing.T) {
	in := retrievalFixture()
	in.Passages[3].Text += "Header for special population.\n\nMonitoring before treatment.\n\nZaburzenia czynności nerek: caution.\n\nAn associated footnote.\n\nA cross-reference.\n\n"
	full, err := prepareSource(in)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := NewOpenAI("fixture", "fixture")
	selected, report := p.WithRetrieval(nil).selectSource(t.Context(), full)
	if !report.Filtered || report.Mode != "lexical" || report.SelectedBytes >= report.OriginalBytes || report.SelectedBytes > retrievalBytes || selected.Coverage == "" {
		t.Fatal(report)
	}
	byID := map[string]evidenceBlock{}
	for _, b := range selected.Evidence {
		byID[b.ID] = b
	}
	for _, original := range full.Evidence {
		b, found := byID[original.ID]
		if found && b != original {
			t.Fatal("source bytes or offsets changed")
		}
		if original.Section != "4.4" && !found {
			t.Fatal("core section dropped")
		}
		if strings.Contains(original.Text, "footnote") && !found {
			t.Fatal("footnote dropped")
		}
		if strings.Contains(original.Text, "cross-reference") && !found {
			t.Fatal("adjacent dependency dropped")
		}
	}
	// Exact occurrence IDs cannot resolve an omitted source block.
	for _, b := range full.Evidence {
		if _, found := byID[b.ID]; !found {
			wire := fmt.Sprintf(`{"findings":[{"summary":"bad reference","band":null,"metric":null,"method":null,"renal_unit":null,"dose_text":null,"dose_value":null,"dose_unit":null,"interval_value":null,"interval_unit":null,"facts":[{"evidence_id":%q}],"unresolved":[]}],"abstain":false,"reason":""}`, b.ID)
			if _, err := resolveModelOutput([]byte(wire), selected, in); err == nil {
				t.Fatal("omitted evidence accepted")
			}
			break
		}
	}
}

type retrievalEmbedder struct{ invalid bool }

func (retrievalEmbedder) Model() string { return "fixture-embedding" }
func (e retrievalEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if e.invalid {
		return [][]float64{{1}}, nil
	}
	result := make([][]float64, len(texts))
	for i, text := range texts {
		result[i] = make([]float64, evidence.Dimensions)
		result[i][0] = -1
		if text == retrievalQuery || strings.Contains(text, "Unusual handling guidance") {
			result[i][0] = 1
		}
	}
	return result, nil
}

func TestHybridAddsSemanticOnlyEvidence(t *testing.T) {
	in := retrievalFixture()
	in.Passages[3].Text += "Unusual handling guidance.\n\n"
	full, _ := prepareSource(in)
	p, _ := NewOpenAI("fixture", "fixture")
	selected, report := p.WithRetrieval(retrievalEmbedder{}).selectSource(t.Context(), full)
	if report.Mode != "hybrid" || !report.Filtered || report.EmbeddingModel != "fixture-embedding" {
		t.Fatal(report)
	}
	found := false
	for _, b := range selected.Evidence {
		found = found || strings.Contains(b.Text, "Unusual handling guidance")
	}
	if !found {
		t.Fatal("semantic-only passage omitted")
	}
	_, failed := p.WithRetrieval(retrievalEmbedder{invalid: true}).selectSource(t.Context(), full)
	if failed.Filtered || failed.Mode != "full" || failed.FallbackReason != "LOCAL_EMBEDDER_UNAVAILABLE_OR_INVALID" {
		t.Fatal(failed)
	}
}

func TestRetrievalConservativeFallbacks(t *testing.T) {
	for _, scenario := range []string{"table", "budget", "no-anchor"} {
		t.Run(scenario, func(t *testing.T) {
			in := retrievalFixture()
			switch scenario {
			case "table":
				in.Passages[3].Text += "Value    Instruction\n10-25    retain the whole passage\n\nFootnote elsewhere."
			case "budget":
				in.Passages[1].Text += strings.Repeat("Preserve full dosing context. ", 2000)
			case "no-anchor":
				in.Passages[1].Text = "No keywords in this source."
			}
			full, err := prepareSource(in)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := NewOpenAI("fixture", "fixture")
			selected, report := p.WithRetrieval(nil).selectSource(t.Context(), full)
			if report.Filtered || report.Mode != "full" || len(selected.Evidence) != len(full.Evidence) {
				t.Fatal(report)
			}
		})
	}
}

func TestRetrievalManifestPinsConfiguration(t *testing.T) {
	p, _ := NewOpenAI("fixture", "fixture")
	baseline := ProviderManifest(p)
	lexical := ProviderManifest(p.WithRetrieval(nil))
	hybrid := ProviderManifest(p.WithRetrieval(retrievalEmbedder{}))
	if baseline == lexical || lexical == hybrid || baseline != ManifestHash() {
		t.Fatal("configuration not pinned")
	}
}

func TestRetrievalSkipsOversizedOptionalWindowWhole(t *testing.T) {
	in := retrievalFixture()
	in.Passages[3].Text += "Monitoring " + strings.Repeat("optional context ", 3100)
	full, err := prepareSource(in)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := NewOpenAI("fixture", "fixture")
	selected, report := p.WithRetrieval(nil).selectSource(t.Context(), full)
	if !report.Filtered || report.SelectedBytes > retrievalBytes {
		t.Fatal(report)
	}
	for _, b := range selected.Evidence {
		if strings.Contains(b.Text, "optional context") {
			t.Fatal("oversized window partially admitted")
		}
	}
}

// Optional local corpus measurement. No network or paid model calls.
func TestRecordedRetrievalSize(t *testing.T) {
	files, _ := filepath.Glob("../../../outputs/online-extraction-smoke/*/input.json")
	if len(files) == 0 {
		t.Skip("no recorded public source inputs")
	}
	raw, err := os.ReadFile(files[len(files)-1])
	if err != nil {
		t.Fatal(err)
	}
	var in SourceInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	full, err := prepareSource(in)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := NewOpenAI("fixture", "fixture")
	selected, report := p.WithRetrieval(nil).selectSource(t.Context(), full)
	t.Logf("Recorded source: %d -> %d bytes; %d -> %d blocks; mode=%s; fallback=%s", report.OriginalBytes, report.SelectedBytes, report.OriginalBlocks, report.SelectedBlocks, report.Mode, report.FallbackReason)
	if report.SelectedBytes > report.OriginalBytes && !report.Filtered {
		t.Fatal("full source changed")
	}
	// Regression coverage of previously cited evidence, not an independent gold set.
	resultBytes, err := os.ReadFile(filepath.Join(filepath.Dir(files[len(files)-1]), "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded ExtractionResult
	if err := json.Unmarshal(resultBytes, &recorded); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, finding := range recorded.Output.Findings {
		for _, fact := range finding.Facts {
			found := false
			for _, b := range selected.Evidence {
				found = found || b.chunkID == fact.ChunkID && strings.Contains(b.Text, fact.Quote)
			}
			if !found {
				t.Fatalf("previously cited %s evidence omitted from section source %s", fact.Field, fact.ChunkID)
			}
			count++
		}
	}
	t.Logf("Retained source evidence for all %d previously recorded facts (not an independent recall benchmark)", count)
}
