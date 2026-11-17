package extraction

import (
	"encoding/json"
	"fmt"
	"regexp"
	"renalsafe/core/internal/domain"
	"strings"
)

// Evidence blocks preserve every source byte, including table columns, blank
// lines and form feeds. IDs address occurrences; the model never copies quotes.
const EvidenceVersion = "exact-paragraph-blocks-v1"

type evidenceBlock struct {
	ID         string `json:"evidence_id"`
	Section    string `json:"section"`
	Text       string `json:"text"`
	chunkID    string
	start, end int
}
type preparedSource struct {
	Context  domain.Context  `json:"context"`
	PDFHash  string          `json:"pdf_sha256"`
	Evidence []evidenceBlock `json:"evidence"`
	Coverage string          `json:"coverage,omitempty"`
}
type modelFact struct {
	Field      string
	Value      string
	EvidenceID string
}
type contextReference struct {
	EvidenceID string `json:"evidence_id"`
}
type modelValue struct {
	Value      string `json:"value"`
	EvidenceID string `json:"evidence_id"`
}
type modelBand struct {
	Kind          string `json:"kind"`
	EvidenceID    string `json:"evidence_id"`
	LowerValue    string `json:"lower_value,omitempty"`
	UpperValue    string `json:"upper_value,omitempty"`
	LowerOperator string `json:"lower_operator,omitempty"`
	UpperOperator string `json:"upper_operator,omitempty"`
}
type modelFinding struct {
	Metric        *modelValue        `json:"metric"`
	Method        *modelValue        `json:"method"`
	RenalUnit     *modelValue        `json:"renal_unit"`
	Summary       string             `json:"summary"`
	Band          *modelBand         `json:"band"`
	DoseText      *modelValue        `json:"dose_text"`
	DoseValue     *modelValue        `json:"dose_value"`
	DoseUnit      *modelValue        `json:"dose_unit"`
	IntervalValue *modelValue        `json:"interval_value"`
	IntervalUnit  *modelValue        `json:"interval_unit"`
	Facts         []contextReference `json:"facts"`
	Unresolved    []string           `json:"unresolved"`
}
type modelOutput struct {
	Findings []modelFinding `json:"findings"`
	Abstain  *bool          `json:"abstain"`
	Reason   *string        `json:"reason"`
}

var paragraphBreak = regexp.MustCompile(`\n[ \t]*\n`)

func prepareSource(input SourceInput) (preparedSource, error) {
	prepared := preparedSource{Context: input.Context, PDFHash: input.PDFHash, Evidence: []evidenceBlock{}}
	ids := map[string]bool{}
	for _, p := range input.Passages {
		if p.ID == "" || ids[p.ID] || p.Text == "" {
			return prepared, fmt.Errorf("invalid source passages")
		}
		ids[p.ID] = true
		start := 0
		appendBlock := func(end int) {
			if end > start {
				prepared.Evidence = append(prepared.Evidence, evidenceBlock{ID: fmt.Sprintf("e%04d", len(prepared.Evidence)+1), Section: p.Section, Text: p.Text[start:end], chunkID: p.ID, start: start, end: end})
				start = end
			}
		}
		for _, boundary := range paragraphBreak.FindAllStringIndex(p.Text, -1) {
			appendBlock(boundary[1])
		}
		appendBlock(len(p.Text))
	}
	b, err := json.Marshal(prepared)
	if err != nil || len(b) > MaxInputBytes || len(prepared.Evidence) == 0 {
		return prepared, fmt.Errorf("input exceeds extraction budget")
	}
	return prepared, nil
}

func resolveModelOutput(raw []byte, prepared preparedSource, input SourceInput) (ExtractionOutput, error) {
	var wire modelOutput
	if err := domain.Decode(raw, &wire); err != nil {
		return ExtractionOutput{}, fmt.Errorf("provider output is not a strict finding envelope")
	}
	if wire.Abstain == nil || wire.Reason == nil || wire.Findings == nil {
		return ExtractionOutput{}, fmt.Errorf("provider output is not a strict finding envelope")
	}
	var required struct {
		Findings []map[string]json.RawMessage `json:"findings"`
	}
	if json.Unmarshal(raw, &required) != nil {
		return ExtractionOutput{}, fmt.Errorf("invalid finding envelope")
	}
	for _, f := range required.Findings {
		for _, key := range []string{"summary", "band", "metric", "method", "renal_unit", "dose_text", "dose_value", "dose_unit", "interval_value", "interval_unit", "facts", "unresolved"} {
			if _, ok := f[key]; !ok {
				return ExtractionOutput{}, fmt.Errorf("missing finding field")
			}
		}
	}
	blocks := map[string]evidenceBlock{}
	for _, b := range prepared.Evidence {
		blocks[b.ID] = b
	}
	out := ExtractionOutput{Findings: []ExtractionFinding{}, Abstain: *wire.Abstain, Reason: *wire.Reason}
	for i, f := range wire.Findings {
		if f.Facts == nil {
			return ExtractionOutput{}, fmt.Errorf("finding %d: invalid finding", i+1)
		}
		finding := ExtractionFinding{Summary: f.Summary, Facts: []ExtractionFact{}, Unresolved: f.Unresolved}
		facts := []modelFact{}
		if f.Band != nil {
			band := f.Band
			block, ok := blocks[band.EvidenceID]
			if !ok || !validBand(*band, block.Text) {
				return ExtractionOutput{}, fmt.Errorf("finding %d: unsupported renal band", i+1)
			}
			for _, pair := range []struct{ field, value string }{{"lower_value", band.LowerValue}, {"upper_value", band.UpperValue}, {"lower_operator", band.LowerOperator}, {"upper_operator", band.UpperOperator}} {
				if pair.value != "" {
					facts = append(facts, modelFact{Field: pair.field, Value: pair.value, EvidenceID: band.EvidenceID})
				}
			}
			if band.Kind == "printed_range" {
				finding.Unresolved = append(finding.Unresolved, "Printed range boundary inclusivity requires source review.")
			}
		}
		for _, pair := range []struct {
			field string
			value *modelValue
		}{{"metric", f.Metric}, {"method", f.Method}, {"renal_unit", f.RenalUnit}, {"dose_text", f.DoseText}, {"dose_value", f.DoseValue}, {"dose_unit", f.DoseUnit}, {"interval_value", f.IntervalValue}, {"interval_unit", f.IntervalUnit}} {
			if pair.value != nil {
				facts = append(facts, modelFact{Field: pair.field, Value: pair.value.Value, EvidenceID: pair.value.EvidenceID})
			}
		}
		for _, ref := range f.Facts {
			block, ok := blocks[ref.EvidenceID]
			if !ok {
				return ExtractionOutput{}, fmt.Errorf("finding %d: unknown evidence reference", i+1)
			}
			facts = append(facts, modelFact{Field: "condition", Value: normalized(block.Text), EvidenceID: ref.EvidenceID})
		}
		if f.Band != nil && f.Method == nil {
			finding.Unresolved = append(finding.Unresolved, "Renal estimation method has not been verified; review the source.")
		}
		if f.Band != nil && f.DoseText != nil && f.DoseValue == nil {
			finding.Unresolved = append(finding.Unresolved, "No absolute dose extracted; review baseline-dose and indication dependencies.")
		}
		for _, fact := range facts {
			block, ok := blocks[fact.EvidenceID]
			if !ok {
				return ExtractionOutput{}, fmt.Errorf("finding %d: unknown evidence reference", i+1)
			}
			value := fact.Value
			modelValue := ""
			// Restore source spelling only for an unambiguous metric/method name.
			// Units, dose wording, numbers and operators remain strictly exact.
			if (fact.Field == "metric" || fact.Field == "method") && !strings.Contains(normalized(block.Text), normalized(value)) {
				words := strings.Fields(value)
				escaped := []string{}
				for _, word := range words {
					escaped = append(escaped, regexp.QuoteMeta(word))
				}
				if len(escaped) > 0 {
					matches := regexp.MustCompile("(?i)"+strings.Join(escaped, `\s+`)).FindAllStringIndex(block.Text, -1)
					if len(matches) == 1 {
						modelValue = value
						value = normalized(block.Text[matches[0][0]:matches[0][1]])
					}
				}
			}
			if isNumeric(fact.Field) {
				value = strings.ReplaceAll(value, ",", ".")
			}
			start, end := block.start, block.end
			finding.Facts = append(finding.Facts, ExtractionFact{Field: fact.Field, Value: value, ModelValue: modelValue, ChunkID: block.chunkID, Quote: block.Text, StartByte: &start, EndByte: &end})
		}
		out.Findings = append(out.Findings, finding)
	}
	if err := Validate(out, input); err != nil {
		return ExtractionOutput{}, err
	}
	return out, nil
}

// Verify a printed band as a whole rather than borrowing numbers/operators
// from unrelated rows in the same table. This is syntax, not clinical approval.
func validBand(b modelBand, text string) bool {
	n := func(v string) string { return strings.ReplaceAll(regexp.QuoteMeta(v), `\.`, `[.,]`) }
	printed := func(op, v string) bool {
		return v != "" && scalarNumber.MatchString(v) && regexp.MustCompile(regexp.QuoteMeta(op)+`[ \t]*`+n(v)+`([^0-9.,]|$)`).MatchString(text)
	}
	lower := func() bool {
		return (b.LowerOperator == ">" || b.LowerOperator == ">=" || b.LowerOperator == "≥") && printed(b.LowerOperator, b.LowerValue)
	}
	upper := func() bool {
		return (b.UpperOperator == "<" || b.UpperOperator == "<=" || b.UpperOperator == "≤") && printed(b.UpperOperator, b.UpperValue)
	}
	switch b.Kind {
	case "printed_range":
		return b.LowerOperator == "" && b.UpperOperator == "" && scalarNumber.MatchString(b.LowerValue) && scalarNumber.MatchString(b.UpperValue) && regexp.MustCompile(`(^|[^0-9.,])`+n(b.LowerValue)+`[ \t]*[-–—][ \t]*`+n(b.UpperValue)+`([^0-9.,]|$)`).MatchString(text)
	case "upper_bound":
		return b.LowerValue == "" && b.LowerOperator == "" && upper()
	case "lower_bound":
		return b.UpperValue == "" && b.UpperOperator == "" && lower()
	case "bounded_interval":
		return lower() && upper()
	default:
		return false
	}
}
