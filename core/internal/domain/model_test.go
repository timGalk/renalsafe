package domain

import (
	"math"
	"testing"
)

func TestRangeEndpoints(t *testing.T) {
	u := 30.0
	i := Interval{Lower: 15, Upper: &u, LowerInclusive: true}
	for _, c := range []struct {
		v  float64
		ok bool
	}{{14.99, false}, {15, true}, {math.Nextafter(30, 0), true}, {30, false}, {math.NaN(), false}, {math.Inf(1), false}} {
		if i.Contains(c.v) != c.ok {
			t.Fatal(c)
		}
	}
	j := Interval{Lower: 30, LowerInclusive: true}
	if i.Overlaps(j) {
		t.Fatal("adjacent overlap")
	}
	i.UpperInclusive = true
	if !i.Overlaps(j) {
		t.Fatal("closed endpoint missed")
	}
}
func TestDuplicateNestedKeys(t *testing.T) {
	var v any
	for _, s := range []string{`{"x":{"v":1,"v":2}}`, `[{"unit":"mg/dL","unit":"umol/L"}]`, `{} {}`} {
		if Decode([]byte(s), &v) == nil {
			t.Fatal("ambiguous JSON")
		}
	}
}
