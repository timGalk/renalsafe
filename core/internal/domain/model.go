// Package domain defines the versioned, JSON-serializable assessment contract.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

type Metric string

const (
	CrCl Metric = "CRCL_CG"
	EGFR Metric = "EGFR_CKDEPI_2021"
)

func (m Metric) Unit() string {
	if m == CrCl {
		return "mL/min"
	}
	if m == EGFR {
		return "mL/min/1.73m2"
	}
	return ""
}
func Finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func Digest(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		return ""
	}
	return BytesDigest(b)
}
func BytesDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Decode(b []byte, v any) error {
	if e := rejectDuplicateKeys(b); e != nil {
		return e
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var tail any
	if e := d.Decode(&tail); e != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}

type Quantity struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type Context struct {
	Product    string `json:"product"`
	Route      string `json:"route"`
	Form       string `json:"form"`
	Indication string `json:"indication"`
}

func (c Context) Valid() bool {
	return strings.TrimSpace(c.Product) != "" && strings.TrimSpace(c.Route) != "" && strings.TrimSpace(c.Form) != "" && strings.TrimSpace(c.Indication) != ""
}

type Patient struct {
	Age                    int       `json:"age_years"`
	Sex                    string    `json:"equation_sex"`
	Weight                 *Quantity `json:"weight,omitempty"`
	Height                 *Quantity `json:"height,omitempty"`
	Creatinine             Quantity  `json:"creatinine"`
	CreatinineBounds       *Interval `json:"creatinine_bounds,omitempty"`
	MeasuredAt             time.Time `json:"measured_at"`
	Stable                 *bool     `json:"stable_renal_function"`
	AKI                    *bool     `json:"suspected_aki"`
	Dialysis               *bool     `json:"dialysis"`
	IDMS                   *bool     `json:"idms_standardized"`
	TypicalBodyComposition *bool     `json:"typical_body_composition"`
	ActualWeightConfirmed  *bool     `json:"actual_weight_confirmed"`
}
type Request struct {
	At          time.Time `json:"assessment_at"`
	Patient     Patient   `json:"patient"`
	Medications []Context `json:"medications"`
}
type Interval struct {
	Lower          float64  `json:"lower"`
	Upper          *float64 `json:"upper,omitempty"`
	LowerInclusive bool     `json:"lower_inclusive"`
	UpperInclusive bool     `json:"upper_inclusive"`
}

func (i Interval) Valid() bool {
	return Finite(i.Lower) && i.Lower >= 0 && (i.Upper == nil && !i.UpperInclusive || i.Upper != nil && Finite(*i.Upper) && (*i.Upper > i.Lower || *i.Upper == i.Lower && i.LowerInclusive && i.UpperInclusive))
}
func (i Interval) Contains(v float64) bool {
	return Finite(v) && (v > i.Lower || v == i.Lower && i.LowerInclusive) && (i.Upper == nil || v < *i.Upper || v == *i.Upper && i.UpperInclusive)
}
func (i Interval) Covers(lo, hi float64) bool { return i.Contains(lo) && i.Contains(hi) }
func (i Interval) Overlaps(j Interval) bool {
	if i.Upper != nil && (*i.Upper < j.Lower || *i.Upper == j.Lower && !(i.UpperInclusive && j.LowerInclusive)) {
		return false
	}
	if j.Upper != nil && (*j.Upper < i.Lower || *j.Upper == i.Lower && !(j.UpperInclusive && i.LowerInclusive)) {
		return false
	}
	return true
}

type Source struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Version        string    `json:"version"`
	RetrievedAt    time.Time `json:"retrieved_at"`
	DocumentFile   string    `json:"document_file"`
	DocumentSHA256 string    `json:"document_sha256"`
	TextFile       string    `json:"text_file"`
	TextSHA256     string    `json:"text_sha256"`
	Synthetic      bool      `json:"synthetic"`
}
type Citation struct {
	DocumentSHA256 string `json:"document_sha256"`
	SourceVersion  string `json:"source_version"`
	SourceID       string `json:"source_id"`
	Section        string `json:"section"`
	Page           int    `json:"page"`
	Quote          string `json:"quote"`
}
type Approval struct {
	Reviewer      string    `json:"reviewer"`
	Role          string    `json:"role"`
	At            time.Time `json:"at"`
	ContentDigest string    `json:"content_digest"`
}
type Rule struct {
	ID       string    `json:"id"`
	Version  string    `json:"version"`
	Context  Context   `json:"context"`
	Metric   Metric    `json:"metric"`
	Unit     string    `json:"unit"`
	Range    Interval  `json:"range"`
	Action   string    `json:"action"`
	DoseText string    `json:"dose_text,omitempty"`
	Citation Citation  `json:"citation"`
	Status   string    `json:"status"`
	Approval *Approval `json:"approval,omitempty"`
}

func (r Rule) ContentDigest() string { r.Approval = nil; r.Status = ""; return Digest(r) }

type Knowledge struct {
	Version   string   `json:"version"`
	Synthetic bool     `json:"synthetic"`
	Sources   []Source `json:"sources"`
	Rules     []Rule   `json:"rules"`
}
type Estimate struct {
	Metric         Metric  `json:"metric"`
	Value          float64 `json:"value"`
	Unit           string  `json:"unit"`
	Low            float64 `json:"low"`
	High           float64 `json:"high"`
	FormulaVersion string  `json:"formula_version"`
}
type RenalResult struct {
	Estimates []Estimate `json:"estimates"`
	Warnings  []string   `json:"warnings"`
}
type Result struct {
	Context     Context   `json:"context"`
	Status      string    `json:"status"`
	Reasons     []string  `json:"reasons"`
	Action      string    `json:"action,omitempty"`
	DoseText    string    `json:"dose_text,omitempty"`
	Rule        *Rule     `json:"rule,omitempty"`
	Source      *Source   `json:"source,omitempty"`
	Selected    *Estimate `json:"selected,omitempty"`
	Explanation string    `json:"explanation"`
}
type Assessment struct {
	EngineVersion    string      `json:"engine_version"`
	PolicyVersion    string      `json:"policy_version"`
	KnowledgeVersion string      `json:"knowledge_version"`
	KnowledgeDigest  string      `json:"knowledge_digest"`
	Request          Request     `json:"request"`
	Renal            RenalResult `json:"renal"`
	Results          []Result    `json:"results"`
}

// Reject repeated keys before typed decoding; a silently overwritten unit or value
// is not an acceptable interpretation of an assessment or knowledge artifact.
func rejectDuplicateKeys(b []byte) error {
	d := json.NewDecoder(strings.NewReader(string(b)))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 128 {
			return fmt.Errorf("JSON nesting too deep")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return fmt.Errorf("duplicate or invalid JSON key")
				}
				seen[k] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e := walk(depth + 1); e != nil {
					return e
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, e = d.Token()
		return e
	}
	return walk(0)
}
