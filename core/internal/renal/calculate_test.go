package renal

import (
	"math"
	"renalsafe/core/internal/domain"
	"testing"
	"time"
)

func boolp(v bool) *bool { return &v }
func valid() domain.Patient {
	return domain.Patient{Age: 70, Sex: "male", Weight: &domain.Quantity{Value: 72, Unit: "kg"}, Height: &domain.Quantity{Value: 175, Unit: "cm"}, Creatinine: domain.Quantity{Value: 1, Unit: "mg/dL"}, MeasuredAt: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC), Stable: boolp(true), AKI: boolp(false), Dialysis: boolp(false), IDMS: boolp(true), TypicalBodyComposition: boolp(true), ActualWeightConfirmed: boolp(true)}
}

var at = time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)

func TestNumericReferences(t *testing.T) {
	// CG independently hand-calculated: (140-70)*72/(72*1)=70.
	if v := CockcroftGault(70, "male", 72, 1); v != 70 {
		t.Fatal(v)
	}
	if v := CockcroftGault(70, "female", 72, 1); v != 59.5 {
		t.Fatal(v)
	}
	// Independent high-precision decimal reference values from NIDDK piecewise formula.
	cases := []struct {
		age     int
		sex     string
		s, want float64
	}{{40, "female", 0.5, 121.51934217125223}, {40, "female", 0.7, 112.0543067987975}, {40, "female", 1.4, 48.77446995172494}, {40, "male", 0.5, 132.2331480736179}, {40, "male", 0.9, 110.72559960355484}, {40, "male", 1.8, 48.19611655308788}}
	for _, c := range cases {
		if v := CKDEPI2021(c.age, c.sex, c.s); math.Abs(v-c.want) > 1e-9 {
			t.Errorf("%+v got %.15g", c, v)
		}
	}
}
func TestUnitEquivalence(t *testing.T) {
	p := valid()
	a := Calculate(p, at)
	p.Creatinine = domain.Quantity{Value: 88.4, Unit: "umol/L"}
	b := Calculate(p, at)
	if domain.Digest(a) != domain.Digest(b) {
		t.Fatalf("%+v %+v", a, b)
	}
}
func TestSafetyGates(t *testing.T) {
	cases := map[string]func(*domain.Patient){"minor": func(p *domain.Patient) { p.Age = 17 }, "sex": func(p *domain.Patient) { p.Sex = "unknown" }, "AKI": func(p *domain.Patient) { p.AKI = boolp(true) }, "dialysis": func(p *domain.Patient) { p.Dialysis = boolp(true) }, "unknown": func(p *domain.Patient) { p.Stable = nil }, "idms": func(p *domain.Patient) { p.IDMS = nil }, "body": func(p *domain.Patient) { p.TypicalBodyComposition = nil }, "zero": func(p *domain.Patient) { p.Creatinine.Value = 0 }, "nan": func(p *domain.Patient) { p.Creatinine.Value = math.NaN() }, "inf": func(p *domain.Patient) { p.Creatinine.Value = math.Inf(1) }, "unit": func(p *domain.Patient) { p.Creatinine.Unit = "mmol/L" }, "weightunit": func(p *domain.Patient) { p.Weight.Unit = "lb" }, "heightunit": func(p *domain.Patient) { p.Height.Unit = "m" }, "stale": func(p *domain.Patient) { p.MeasuredAt = at.Add(-8 * 24 * time.Hour) }, "future": func(p *domain.Patient) { p.MeasuredAt = at.Add(time.Second) }, "obesity": func(p *domain.Patient) { p.Weight.Value = 150 }, "malnutrition": func(p *domain.Patient) { p.Weight.Value = 30 }, "bounds": func(p *domain.Patient) { x := 2.0; p.CreatinineBounds = &domain.Interval{Lower: 0.5, Upper: &x} }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			p := valid()
			change(&p)
			r := Calculate(p, at)
			if len(r.Estimates) != 0 || len(r.Warnings) == 0 {
				t.Fatalf("unsafe estimate %+v", r)
			}
		})
	}
}
func TestMissingCGInputsNeverSubstitute(t *testing.T) {
	for _, change := range []func(*domain.Patient){func(p *domain.Patient) { p.Weight = nil }, func(p *domain.Patient) { p.Height = nil }, func(p *domain.Patient) { p.ActualWeightConfirmed = nil }} {
		p := valid()
		change(&p)
		r := Calculate(p, at)
		if len(r.Estimates) != 1 || r.Estimates[0].Metric != domain.EGFR || len(r.Warnings) == 0 {
			t.Fatal(r)
		}
	}
}
func TestUncertaintyMonotonic(t *testing.T) {
	p := valid()
	hi := 1.1
	p.CreatinineBounds = &domain.Interval{Lower: 0.9, Upper: &hi, LowerInclusive: true, UpperInclusive: true}
	r := Calculate(p, at)
	for _, e := range r.Estimates {
		if !(e.Low < e.Value && e.Value < e.High) {
			t.Fatal(e)
		}
	}
}
func TestAdultAndFreshnessBoundary(t *testing.T) {
	p := valid()
	p.Age = 18
	p.MeasuredAt = at.Add(-7 * 24 * time.Hour)
	if r := Calculate(p, at); len(r.Estimates) != 2 {
		t.Fatal(r)
	}
}
