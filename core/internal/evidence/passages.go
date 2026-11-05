package evidence

import (
	"fmt"
	"regexp"
	"strings"
)

// PassageTargetBytes is a soft bound, not a model token limit. An indivisible
// table or paragraph may exceed it; providers must still reject rather than truncate.
const PassageTargetBytes = 6000

var paragraphBreak = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)
var columns = regexp.MustCompile(`\S(?: {2,}|\t+)\S`)

// Passages partitions each section into contiguous, lossless source spans. We
// split only at paragraph breaks: never inside a row, a paragraph or a page break.
// Adjacent table paragraphs and their preceding paragraph (possible caption or
// header) stay together. No column interpretation or manufactured header is added.
// Ambiguous/oversized blocks remain whole for manual inspection.
func Passages(sections []Section, target int) ([]Section, error) {
	if target < 1 {
		return nil, fmt.Errorf("positive passage target required")
	}
	out := []Section{}
	for _, sec := range sections {
		if sec.End-sec.Start != len(sec.Text) {
			return nil, fmt.Errorf("inconsistent source span")
		}
		if len(sec.Text) <= target {
			out = append(out, sec)
			continue
		}
		type block struct {
			start, end int
			table      bool
		}
		blocks := []block{}
		start := 0
		for _, loc := range paragraphBreak.FindAllStringIndex(sec.Text, -1) {
			blocks = append(blocks, block{start, loc[1], columns.MatchString(sec.Text[start:loc[1]])})
			start = loc[1]
		}
		if start < len(sec.Text) {
			blocks = append(blocks, block{start, len(sec.Text), columns.MatchString(sec.Text[start:])})
		}
		// A table may have blank lines within its header or between rows. Keep
		// consecutive aligned paragraphs with their immediate introductory paragraph.
		joins := make([]bool, len(blocks))
		for i, b := range blocks {
			if b.table {
				if i > 0 {
					joins[i-1] = true
				}
				joins[i] = true // keep the following paragraph as possible footnote
			}
		}
		groups := []block{}
		for i, b := range blocks {
			if i > 0 && joins[i-1] {
				groups[len(groups)-1].end = b.end
			} else {
				groups = append(groups, b)
			}
		}
		begin, end := 0, 0
		emit := func() {
			if end == begin {
				return
			}
			p := sec
			p.Start, p.End = sec.Start+begin, sec.Start+end
			p.Text = sec.Text[begin:end]
			p.PageStart = sec.PageStart + strings.Count(sec.Text[:begin], "\f")
			p.PageEnd = p.PageStart + strings.Count(p.Text, "\f")
			p.Table = columns.MatchString(p.Text)
			out = append(out, p)
			begin = end
		}
		for _, b := range groups {
			if end > begin && b.end-begin > target {
				emit()
			}
			end = b.end
		}
		emit()
	}
	return out, nil
}

// SplitPassages is used for new imports only; previously persisted chunks and
// their clinical citations remain immutable.
func SplitPassages(text string, first, last int) ([]Section, error) {
	sections, err := Split(text, first, last)
	if err != nil {
		return nil, err
	}
	return Passages(sections, PassageTargetBytes)
}
