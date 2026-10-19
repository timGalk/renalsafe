// Package rules validates and freezes a content-addressed knowledge snapshot.
package rules

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"renalsafe/core/internal/domain"
	"strconv"
	"strings"
)

type Snapshot struct {
	data   domain.Knowledge
	digest string
}

func (s Snapshot) Digest() string { return s.digest }
func (s Snapshot) Knowledge() domain.Knowledge {
	var k domain.Knowledge
	b := mustJSON(s.data)
	_ = domain.Decode(b, &k)
	return k
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// Load validates files pinned by an offline knowledge bundle.
func Load(b []byte, root string, allowSynthetic bool) (Snapshot, error) {
	read := func(path, hash string) ([]byte, error) {
		if path == "" || filepath.IsAbs(path) {
			return nil, fmt.Errorf("relative source path required")
		}
		clean := filepath.Clean(path)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("source escapes root")
		}
		base, e := filepath.EvalSymlinks(root)
		if e != nil {
			return nil, e
		}
		real, e := filepath.EvalSymlinks(filepath.Join(base, clean))
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(base, real)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("symlink escapes root")
		}
		v, e := os.ReadFile(real)
		if e != nil {
			return nil, e
		}
		if domain.BytesDigest(v) != hash {
			return nil, fmt.Errorf("source hash mismatch: %s", path)
		}
		return v, nil
	}
	return load(b, read, allowSynthetic)
}

// LoadWithFiles applies the same validation to immutable documents read from storage.
func LoadWithFiles(b []byte, files map[string][]byte, allowSynthetic bool) (Snapshot, error) {
	read := func(path, hash string) ([]byte, error) {
		clean := filepath.Clean(path)
		if path == "" || filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("invalid source path")
		}
		v, ok := files[path]
		if !ok || domain.BytesDigest(v) != hash {
			return nil, fmt.Errorf("source missing or changed: %s", path)
		}
		return v, nil
	}
	return load(b, read, allowSynthetic)
}
func load(b []byte, read func(string, string) ([]byte, error), allowSynthetic bool) (Snapshot, error) {
	var k domain.Knowledge
	if e := domain.Decode(b, &k); e != nil {
		return Snapshot{}, e
	}
	if k.Version == "" || k.Sources == nil || k.Rules == nil {
		return Snapshot{}, fmt.Errorf("version, sources and rules required")
	}
	if k.Synthetic && !allowSynthetic {
		return Snapshot{}, fmt.Errorf("synthetic KB prohibited")
	}
	sources := map[string]domain.Source{}
	texts := map[string]string{}
	for _, s := range k.Sources {
		u, e := url.Parse(s.URL)
		if s.ID == "" || s.Version == "" || s.RetrievedAt.IsZero() || e != nil || u.Scheme != "https" || u.Host == "" {
			return Snapshot{}, fmt.Errorf("invalid source metadata")
		}
		if _, ok := sources[s.ID]; ok {
			return Snapshot{}, fmt.Errorf("duplicate source")
		}
		if s.Synthetic && (!k.Synthetic || !allowSynthetic) {
			return Snapshot{}, fmt.Errorf("synthetic source prohibited")
		}
		if _, e = read(s.DocumentFile, s.DocumentSHA256); e != nil {
			return Snapshot{}, e
		}
		v, e := read(s.TextFile, s.TextSHA256)
		if e != nil {
			return Snapshot{}, e
		}
		sources[s.ID] = s
		texts[s.ID] = string(v)
	}
	ids := map[string]bool{}
	actions := map[string]bool{"NO_CHANGE": true, "REDUCE_DOSE": true, "EXTEND_INTERVAL": true, "AVOID": true, "USE_WITH_CAUTION": true}
	for _, r := range k.Rules {
		if r.ID == "" || ids[r.ID] || r.Version == "" || !r.Context.Valid() || !r.Range.Valid() || r.Metric.Unit() == "" || r.Unit != r.Metric.Unit() || !actions[r.Action] {
			return Snapshot{}, fmt.Errorf("invalid or duplicate rule: %s", r.ID)
		}
		ids[r.ID] = true
		switch r.Status {
		case "PENDING", "APPROVED", "REJECTED", "SUPERSEDED":
		default:
			return Snapshot{}, fmt.Errorf("invalid lifecycle")
		}
		if (r.Action == "REDUCE_DOSE" || r.Action == "EXTEND_INTERVAL") && r.DoseText == "" {
			return Snapshot{}, fmt.Errorf("dose instructions required")
		}
		if r.Action == "AVOID" && r.DoseText != "" {
			return Snapshot{}, fmt.Errorf("avoid cannot prescribe a dose")
		}
		_, ok := sources[r.Citation.SourceID]
		if !ok || r.Citation.DocumentSHA256 != sources[r.Citation.SourceID].DocumentSHA256 || r.Citation.SourceVersion != sources[r.Citation.SourceID].Version || r.Citation.Quote == "" || r.Citation.Section == "" || r.Citation.Page < 1 || !strings.Contains(texts[r.Citation.SourceID], r.Citation.Quote) {
			return Snapshot{}, fmt.Errorf("citation ungrounded")
		}
		// Numeric presence is a necessary grounding check, not semantic validation.
		nums := map[float64]bool{}
		for _, token := range regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`).FindAllString(r.Citation.Quote, -1) {
			v, _ := strconv.ParseFloat(token, 64)
			nums[v] = true
		}
		if c := r.Conditions; c != nil {
			for _, rg := range []*domain.Interval{c.AgeYears, c.WeightKG} {
				if rg == nil {
					continue
				}
				if !rg.Valid() || rg.Lower != 0 && !nums[rg.Lower] || rg.Upper != nil && !nums[*rg.Upper] {
					return Snapshot{}, fmt.Errorf("unsupported condition range or absent source number")
				}
			}
			if r.Status == "APPROVED" && len(c.Unmodeled) > 0 {
				return Snapshot{}, fmt.Errorf("unmodeled clinical conditions require manual review")
			}
		}
		if cap := r.RegimenLimit; cap != nil {
			validUnit := cap.MaxSingleDose.Unit == "mg" || cap.MaxSingleDose.Unit == "g" || cap.MaxSingleDose.Unit == "microgram" || cap.MaxSingleDose.Unit == "IU"
			if !validUnit || !domain.Finite(cap.MaxSingleDose.Value) || cap.MaxSingleDose.Value <= 0 || !domain.Finite(cap.MinIntervalHours) || cap.MinIntervalHours <= 0 || !nums[cap.MaxSingleDose.Value] || !nums[cap.MinIntervalHours] || r.Action == "AVOID" {
				return Snapshot{}, fmt.Errorf("invalid or ungrounded structured regimen")
			}
		}
		if r.Range.Lower != 0 && !nums[r.Range.Lower] {
			return Snapshot{}, fmt.Errorf("lower threshold absent from quote")
		}
		if r.Range.Upper != nil && !nums[*r.Range.Upper] {
			return Snapshot{}, fmt.Errorf("upper threshold absent from quote")
		}
		for _, token := range regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`).FindAllString(r.DoseText, -1) {
			v, _ := strconv.ParseFloat(token, 64)
			if !nums[v] {
				return Snapshot{}, fmt.Errorf("dose number absent from quote")
			}
		}
		if r.Status == "APPROVED" && r.Approval != nil && r.Approval.At.Before(sources[r.Citation.SourceID].RetrievedAt) {
			return Snapshot{}, fmt.Errorf("approval predates source retrieval")
		}
		if r.Status == "APPROVED" && (r.Approval == nil || r.Approval.Reviewer == "" || r.Approval.Role != "pharmacist" || r.Approval.At.IsZero() || r.Approval.ContentDigest != r.ContentDigest()) {
			return Snapshot{}, fmt.Errorf("approval missing or stale")
		}
	}
	for i, a := range k.Rules {
		if a.Status != "APPROVED" {
			continue
		}
		for _, b := range k.Rules[i+1:] {
			if b.Status == "APPROVED" && a.Context == b.Context && a.Metric == b.Metric && a.Range.Overlaps(b.Range) {
				return Snapshot{}, fmt.Errorf("approved overlap: %s/%s", a.ID, b.ID)
			}
		}
	}
	return Snapshot{data: k, digest: domain.Digest(k)}, nil
}
