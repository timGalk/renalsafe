package extraction

import (
	"container/heap"
	"context"
	"encoding/json"
	"math"
	"regexp"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/evidence"
	"strings"
	"time"
)

const RetrievalVersion = "renal-bm25-rrf-windows-v1"
const retrievalTopK = 12
const retrievalBytes = 48000 // Serialized source bytes, not a tokenizer estimate.
const filteredReview = "Only selected source passages were sent to the model. Review the full document for omitted warnings, dependencies and cross-references before using this finding."
const retrievalQuery = "renal kidney impairment creatinine clearance dialysis dosing warnings monitoring nerek niewydolność klirens kreatyniny dializa dawkowanie ostrzeżenia monitorowanie"

// RetrievalReport records the exact evidence sent, without duplicating its text.
type RetrievalReport struct {
	Version        string   `json:"version"`
	Mode           string   `json:"mode"`
	EmbeddingModel string   `json:"embedding_model,omitempty"`
	FallbackReason string   `json:"fallback_reason,omitempty"`
	Filtered       bool     `json:"filtered"`
	OriginalBlocks int      `json:"original_blocks"`
	SelectedBlocks int      `json:"selected_blocks"`
	OriginalBytes  int      `json:"original_bytes"`
	SelectedBytes  int      `json:"selected_bytes"`
	EvidenceIDs    []string `json:"evidence_ids"`
	InputDigest    string   `json:"input_digest"`
}

type TokenUsage struct {
	InputTokens       int                `json:"input_tokens"`
	OutputTokens      int                `json:"output_tokens"`
	TotalTokens       int                `json:"total_tokens"`
	InputTokenDetails *InputTokenDetails `json:"input_tokens_details,omitempty"`
}

type InputTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// WithRetrieval enables experimental, bounded source selection. A nil embedder
// uses lexical retrieval and records the degradation explicitly.
func (o *OpenAI) WithRetrieval(embedder evidence.Embedder) *OpenAI {
	o.retrieval = true
	o.embedder = embedder
	return o
}

func (o *OpenAI) ManifestHash() string {
	if !o.retrieval {
		return ManifestHash()
	}
	model := ""
	if o.embedder != nil {
		model = o.embedder.Model()
	}
	return domain.Digest([]any{ManifestHash(), RetrievalVersion, retrievalTopK, retrievalBytes, model})
}

func ProviderManifest(p Provider) string {
	if configured, ok := p.(interface{ ManifestHash() string }); ok {
		return configured.ManifestHash()
	}
	return ManifestHash()
}

type rankedBlock struct {
	index int
	score float64
}
type blockHeap []rankedBlock

func (h blockHeap) Len() int { return len(h) }
func (h blockHeap) Less(i, j int) bool {
	if h[i].score == h[j].score {
		return h[i].index > h[j].index
	}
	return h[i].score < h[j].score
}
func (h blockHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *blockHeap) Push(v any)   { *h = append(*h, v.(rankedBlock)) }
func (h *blockHeap) Pop() any {
	old := *h
	v := old[len(old)-1]
	*h = old[:len(old)-1]
	return v
}

// Keep only k candidates per channel, with stable source-order tie breaking.
func bestBlocks(scores []float64, k int) []rankedBlock {
	h := &blockHeap{}
	if k <= 0 {
		return nil
	}
	for i, score := range scores {
		if score <= 0 || math.IsNaN(score) || math.IsInf(score, 0) {
			continue
		}
		v := rankedBlock{i, score}
		if h.Len() < k {
			heap.Push(h, v)
		} else if score > (*h)[0].score || score == (*h)[0].score && i < (*h)[0].index {
			(*h)[0] = v
			heap.Fix(h, 0)
		}
	}
	out := make([]rankedBlock, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(rankedBlock)
	}
	return out
}

var retrievalWords = regexp.MustCompile(`[\pL\pN]+`)
var renalRoots = []string{"renal", "kidney", "nerk", "nerek", "creatinin", "kreatynin", "clearance", "klirens", "crcl", "egfr", "dialys", "dializ", "hemodial", "haemodial", "peritone", "otrzewn"}
var dependencyRoots = []string{"dos", "dawk", "warn", "ostrze", "monitor", "maximum", "maksymal", "ostroż", "caution"}
var tableColumns = regexp.MustCompile(`(?m)\S[\t ]{2,}\S`)
var digit = regexp.MustCompile(`[0-9]`)
var sectionHeading = regexp.MustCompile(`^[ \t]*[1-6]\.[1-9][ \t]+\pL`)

func termCounts(text string, roots []string) ([]int, int) {
	words := retrievalWords.FindAllString(strings.ToLower(text), -1)
	counts := make([]int, len(roots))
	for _, word := range words {
		for j, root := range roots {
			if strings.HasPrefix(word, root) {
				counts[j]++
			}
		}
	}
	return counts, len(words)
}

func lexicalScores(blocks []evidenceBlock) ([]float64, []bool) {
	roots := append(append([]string{}, renalRoots...), dependencyRoots...)
	counts, lengths := make([][]int, len(blocks)), make([]int, len(blocks))
	df, anchors := make([]int, len(roots)), make([]bool, len(blocks))
	total := 0
	for i, b := range blocks {
		counts[i], lengths[i] = termCounts(b.Text, roots)
		total += lengths[i]
		for j, count := range counts[i] {
			if count > 0 {
				df[j]++
				anchors[i] = anchors[i] || j < len(renalRoots)
			}
		}
	}
	avg := math.Max(1, float64(total)/float64(len(blocks)))
	scores := make([]float64, len(blocks))
	for i := range blocks {
		for j, count := range counts[i] {
			if count > 0 {
				idf := math.Log(1 + (float64(len(blocks)-df[j])+0.5)/(float64(df[j])+0.5))
				tf := float64(count)
				scores[i] += idf * tf * 2.2 / (tf + 1.2*(0.25+0.75*float64(lengths[i])/avg))
			}
		}
	}
	return scores, anchors
}

func denseScores(ctx context.Context, embedder evidence.Embedder, blocks []evidenceBlock) ([]float64, bool) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	query, err := embedder.Embed(ctx, []string{retrievalQuery})
	if err != nil || len(query) != 1 || !evidence.ValidVector(query[0]) {
		return nil, false
	}
	scores := make([]float64, len(blocks))
	for start := 0; start < len(blocks); start += 16 {
		end := min(start+16, len(blocks))
		texts := make([]string, end-start)
		for i := start; i < end; i++ {
			texts[i-start] = blocks[i].Text
		}
		vectors, err := embedder.Embed(ctx, texts)
		if err != nil || len(vectors) != len(texts) {
			return nil, false
		}
		for i, v := range vectors {
			if !evidence.ValidVector(v) {
				return nil, false
			}
			dot, a, b := 0.0, 0.0, 0.0
			for j, x := range v {
				y := query[0][j]
				dot, a, b = dot+x*y, a+x*x, b+y*y
			}
			scores[start+i] = math.Max(0, dot/math.Sqrt(a*b))
		}
	}
	return scores, true
}

func sourceSize(p preparedSource) int {
	b, _ := json.Marshal(p)
	return len(b)
}

func (o *OpenAI) selectSource(ctx context.Context, full preparedSource) (preparedSource, *RetrievalReport) {
	report := &RetrievalReport{Version: RetrievalVersion, Mode: "full", OriginalBlocks: len(full.Evidence), OriginalBytes: sourceSize(full)}
	finish := func(p preparedSource) (preparedSource, *RetrievalReport) {
		report.SelectedBlocks, report.SelectedBytes = len(p.Evidence), sourceSize(p)
		report.Filtered = report.SelectedBlocks < report.OriginalBlocks
		report.EvidenceIDs = make([]string, len(p.Evidence))
		for i, b := range p.Evidence {
			report.EvidenceIDs[i] = b.ID
		}
		report.InputDigest = domain.Digest(p)
		return p, report
	}
	if !o.retrieval {
		return finish(full)
	}
	lexical, anchors := lexicalScores(full.Evidence)
	selected := map[int]bool{}
	// Windows are indivisible; no slicing table rows or renumbering evidence.
	window := func(index int) []int {
		indices := []int{}
		for i := max(0, index-2); i < min(len(full.Evidence), index+3); i++ {
			if full.Evidence[i].chunkID == full.Evidence[index].chunkID {
				indices = append(indices, i)
			}
		}
		return indices
	}
	tables := map[string]bool{}
	for _, b := range full.Evidence {
		for _, line := range strings.Split(b.Text, "\n") {
			if tableColumns.MatchString(line) && digit.MatchString(line) && !sectionHeading.MatchString(line) {
				tables[b.chunkID] = true
			}
		}
	}
	hasAnchor := false
	for i, b := range full.Evidence {
		if b.Section == "4.1" || b.Section == "4.2" || b.Section == "4.3" || tables[b.chunkID] {
			selected[i] = true
		}
		if anchors[i] {
			hasAnchor = true
			for _, j := range window(i) {
				selected[j] = true
			}
		}
	}
	materialize := func() preparedSource {
		p := full
		p.Evidence = make([]evidenceBlock, 0, len(selected))
		for i, b := range full.Evidence {
			if selected[i] {
				p.Evidence = append(p.Evidence, b)
			}
		}
		if len(p.Evidence) < len(full.Evidence) {
			p.Coverage = filteredReview
		}
		return p
	}
	if !hasAnchor || len(selected) == len(full.Evidence) {
		report.FallbackReason = "FULL_CONTEXT_REQUIRED"
		return finish(full)
	}
	if sourceSize(materialize()) > retrievalBytes {
		report.FallbackReason = "MANDATORY_CONTEXT_EXCEEDS_TARGET"
		return finish(full)
	}
	report.Mode = "lexical"
	for i := range selected {
		lexical[i] = 0
	}
	lists := [][]rankedBlock{bestBlocks(lexical, retrievalTopK)}
	if o.embedder == nil {
		report.FallbackReason = "LOCAL_EMBEDDER_NOT_CONFIGURED"
	} else {
		report.EmbeddingModel = o.embedder.Model()
		dense, ok := denseScores(ctx, o.embedder, full.Evidence)
		if !ok {
			// A failed semantic channel must not silently shrink evidence coverage.
			report.Mode, report.FallbackReason = "full", "LOCAL_EMBEDDER_UNAVAILABLE_OR_INVALID"
			return finish(full)
		}
		report.Mode = "hybrid"
		for i := range selected {
			dense[i] = 0
		}
		lists = append(lists, bestBlocks(dense, retrievalTopK))
	}
	fused := make([]float64, len(full.Evidence))
	for _, list := range lists {
		for rank, b := range list {
			fused[b.index] += 1 / (60 + float64(rank+1))
		}
	}
	// Mandatory evidence is exempt from k. The heap bounds optional expansion.
	for i := range selected {
		fused[i] = 0
	}
	for _, hit := range bestBlocks(fused, retrievalTopK) {
		added := []int{}
		for _, i := range window(hit.index) {
			if !selected[i] {
				selected[i] = true
				added = append(added, i)
			}
		}
		if sourceSize(materialize()) > retrievalBytes {
			for _, i := range added {
				delete(selected, i)
			}
		}
	}
	chosen := materialize()
	if sourceSize(chosen) >= report.OriginalBytes {
		report.Mode, report.FallbackReason = "full", "NO_INPUT_REDUCTION"
		return finish(full)
	}
	return finish(chosen)
}
