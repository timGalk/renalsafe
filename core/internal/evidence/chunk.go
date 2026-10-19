// Package evidence provides source-grounded retrieval without a prescribing model.
package evidence

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Section struct {
	Name                           string
	PageStart, PageEnd, Start, End int
	Text                           string
	Table                          bool
}

var heading = regexp.MustCompile(`(?m)^[ \t\f]*(4\.[1-9]|5\.[12])[ \t]+[^\r\n]+`)
var productHeading = regexp.MustCompile(`(?mi)^[ \t]*1\.[ \t]+NAME OF THE MEDICINAL PRODUCT`)

// PrimaryProductScope bounds the first product in a combined EMA document.
func PrimaryProductScope(text string) (int, int, error) {
	pages := strings.Split(text, "\f")
	start, end := 0, len(pages)
	for i, p := range pages {
		if productHeading.MatchString(p) {
			if start == 0 {
				start = i + 1
			} else {
				end = i
				break
			}
		}
	}
	if start == 0 {
		return 0, 0, fmt.Errorf("product scope must be declared manually")
	}
	return start, end, nil
}

// Split keeps full section text (including table columns and page breaks) intact.
// Offsets are byte offsets in the immutable full extracted document text.
func Split(text string, first, last int) ([]Section, error) {
	if !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
		return nil, fmt.Errorf("invalid document text")
	}
	pages := strings.Split(text, "\f")
	if first < 1 || last < first || last > len(pages) {
		return nil, fmt.Errorf("invalid physical page scope")
	}
	offset := 0
	for i := 0; i < first-1; i++ {
		offset += len(pages[i]) + 1
	}
	scoped := strings.Join(pages[first-1:last], "\f")
	matches := heading.FindAllStringSubmatchIndex(scoped, -1)
	out := []Section{}
	for i, m := range matches {
		end := len(scoped)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		begin := m[2]
		raw := scoped[begin:end]
		name := scoped[m[2]:m[3]]
		// Stop after section 5.2 at the next top-level section, if present.
		stop := regexp.MustCompile(`(?m)^[ \t]*[6-9]\.[ \t]+`).FindStringIndex(raw)
		if stop != nil {
			raw = raw[:stop[0]]
			end = begin + len(raw)
		}
		if len(strings.TrimSpace(raw)) < 8 {
			continue
		}
		p1 := first + strings.Count(scoped[:begin], "\f")
		p2 := p1 + strings.Count(raw, "\f")
		out = append(out, Section{Name: name, PageStart: p1, PageEnd: p2, Start: offset + begin, End: offset + end, Text: raw, Table: strings.Contains(raw, "\t") || regexp.MustCompile(`\S {2,}\S`).MatchString(raw)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no SmPC sections in declared scope")
	}
	return out, nil
}
