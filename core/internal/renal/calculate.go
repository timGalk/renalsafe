// Package renal implements adult estimates; policy limits are prototype gates,
// not clinical reference intervals or a substitute for clinician assessment.
package renal

import (
	"math"
	"renalsafe/core/internal/domain"
	"time"
)

const PolicyVersion = "prototype-adult-stable-v1"

func yes(v *bool) bool { return v != nil && *v }
func no(v *bool) bool  { return v != nil && !*v }
func Calculate(p domain.Patient, at time.Time) domain.RenalResult {
	r := domain.RenalResult{Estimates: []domain.Estimate{}, Warnings: []string{}}
	block := func(s string) { r.Warnings = append(r.Warnings, s) }
	if p.Age < 18 || p.Age > 120 {
		block("AGE_OUT_OF_SCOPE")
	}
	if p.Sex != "female" && p.Sex != "male" {
		block("EQUATION_SEX_REQUIRED")
	}
	if !yes(p.Stable) {
		block("STABILITY_UNCONFIRMED")
	}
	if !no(p.AKI) {
		block("AKI_OR_UNKNOWN")
	}
	if !no(p.Dialysis) {
		block("DIALYSIS_OR_UNKNOWN")
	}
	if !yes(p.IDMS) {
		block("CREATININE_STANDARDIZATION_UNCONFIRMED")
	}
	if !yes(p.TypicalBodyComposition) {
		block("BODY_COMPOSITION_UNCERTAIN")
	}
	if at.IsZero() || p.MeasuredAt.IsZero() || p.MeasuredAt.After(at) || at.Sub(p.MeasuredAt) > 7*24*time.Hour {
		block("CREATININE_TIMESTAMP_INVALID_OR_STALE")
	}
	factor := 1.0
	switch p.Creatinine.Unit {
	case "mg/dL":
	case "umol/L":
		factor = 88.4
	default:
		block("CREATININE_UNIT_UNSUPPORTED")
	}
	sc := p.Creatinine.Value / factor
	if !domain.Finite(sc) || sc < 0.1 || sc > 30 {
		block("CREATININE_OUT_OF_POLICY")
	}
	lo, hi := sc, sc
	if p.CreatinineBounds != nil {
		b := p.CreatinineBounds
		if !b.Valid() || b.Upper == nil || !b.LowerInclusive || !b.UpperInclusive || !b.Contains(p.Creatinine.Value) {
			block("CREATININE_BOUNDS_INVALID")
		} else {
			lo = b.Lower / factor
			hi = *b.Upper / factor
			if lo < 0.1 || hi > 30 {
				block("CREATININE_BOUNDS_OUT_OF_POLICY")
			}
		}
	}
	if p.Weight != nil && (!domain.Finite(p.Weight.Value) || p.Weight.Unit != "kg" || p.Weight.Value < 20 || p.Weight.Value > 300) {
		block("WEIGHT_OUT_OF_POLICY_OR_UNIT")
	}
	if p.Height != nil && (!domain.Finite(p.Height.Value) || p.Height.Unit != "cm" || p.Height.Value < 100 || p.Height.Value > 250) {
		block("HEIGHT_OUT_OF_POLICY_OR_UNIT")
	}
	if p.Weight != nil && p.Height != nil && len(r.Warnings) == 0 {
		bmi := p.Weight.Value / math.Pow(p.Height.Value/100, 2)
		if bmi < 18.5 || bmi > 30 {
			block("BODY_SIZE_REQUIRES_REVIEW")
		}
	}
	if len(r.Warnings) > 0 {
		return r
	}
	add := func(m domain.Metric, f func(float64) float64, ver string) {
		r.Estimates = append(r.Estimates, domain.Estimate{Metric: m, Value: f(sc), Unit: m.Unit(), Low: f(hi), High: f(lo), FormulaVersion: ver})
	}
	add(domain.EGFR, func(s float64) float64 { return CKDEPI2021(p.Age, p.Sex, s) }, "CKD-EPI-2021-creatinine")
	if p.Weight == nil {
		block("CG_WEIGHT_REQUIRED")
	} else if p.Height == nil {
		block("CG_HEIGHT_REQUIRED_FOR_BODY_SIZE_CHECK")
	} else if !yes(p.ActualWeightConfirmed) {
		block("CG_ACTUAL_WEIGHT_UNCONFIRMED")
	} else {
		add(domain.CrCl, func(s float64) float64 { return CockcroftGault(p.Age, p.Sex, p.Weight.Value, s) }, "Cockcroft-Gault-1976-actual-weight")
	}
	return r
}

// These equation functions assume validated inputs. Use Calculate at the boundary.
func CockcroftGault(age int, sex string, weight, scr float64) float64 {
	v := (140 - float64(age)) * weight / (72 * scr)
	if sex == "female" {
		v *= 0.85
	}
	return v
}
func CKDEPI2021(age int, sex string, scr float64) float64 {
	k, a, f := 0.9, -0.302, 1.0
	if sex == "female" {
		k, a, f = 0.7, -0.241, 1.012
	}
	x := scr / k
	return 142 * math.Pow(math.Min(x, 1), a) * math.Pow(math.Max(x, 1), -1.2) * math.Pow(0.9938, float64(age)) * f
}
