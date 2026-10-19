// Package decision contains no model calls or implicit metric conversions.
package decision

import (
	"fmt"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/renal"
	"renalsafe/core/internal/rules"
)

const Version = "0.2.0"

func Assess(req domain.Request, kb rules.Snapshot) domain.Assessment {
	k := kb.Knowledge()
	a := domain.Assessment{EngineVersion: Version, PolicyVersion: renal.PolicyVersion, KnowledgeVersion: k.Version, KnowledgeDigest: kb.Digest(), Request: req, Renal: renal.Calculate(req.Patient, req.At), Results: []domain.Result{}}
	for medicationIndex, ctx := range req.Medications {
		r := domain.Result{Context: ctx, Status: "MANUAL_REVIEW_REQUIRED", Reasons: []string{}}
		fail := func(reason string) {
			r.Reasons = append(r.Reasons, reason)
			r.Explanation = "Manual review required: " + reason + ". No automated action or dose is provided."
			a.Results = append(a.Results, r)
		}
		if domain.ValidatePrescriptionInputs(req) != nil {
			fail("INVALID_PRESCRIPTION_INPUT")
			continue
		}
		if !ctx.Valid() {
			fail("MEDICATION_CONTEXT_REQUIRED")
			continue
		}
		if k.Version == "" || kb.Digest() == "" {
			fail("KNOWLEDGE_SNAPSHOT_INVALID")
			continue
		}
		eligible := []domain.Rule{}
		metrics := map[domain.Metric]bool{}
		for _, rule := range k.Rules {
			if rule.Status == "APPROVED" && rule.Context == ctx {
				eligible = append(eligible, rule)
				metrics[rule.Metric] = true
			}
		}
		if len(eligible) == 0 {
			fail("NO_APPROVED_RULE")
			continue
		}
		if len(metrics) != 1 {
			fail("RENAL_METRIC_AMBIGUOUS")
			continue
		}
		var estimate *domain.Estimate
		for i := range a.Renal.Estimates {
			if metrics[a.Renal.Estimates[i].Metric] {
				v := a.Renal.Estimates[i]
				estimate = &v
			}
		}
		if estimate == nil {
			r.Reasons = append(r.Reasons, a.Renal.Warnings...)
			fail("REQUIRED_RENAL_METRIC_UNAVAILABLE")
			continue
		}
		matches := []domain.Rule{}
		for _, rule := range eligible {
			if rule.Range.Contains(estimate.Value) {
				matches = append(matches, rule)
			}
		}
		if len(matches) != 1 {
			fail("NO_UNIQUE_RANGE_MATCH")
			continue
		}
		rule := matches[0]
		if req.At.Before(rule.Approval.At) {
			fail("APPROVAL_NOT_YET_EFFECTIVE")
			continue
		}
		if c := rule.Conditions; c != nil {
			reason := ""
			if len(c.Unmodeled) > 0 {
				reason = "UNMODELED_CLINICAL_CONDITIONS"
			}
			if c.AgeYears != nil && !c.AgeYears.Contains(float64(req.Patient.Age)) {
				reason = "AGE_CONDITION_NOT_MET"
			}
			if c.WeightKG != nil {
				if req.Patient.Weight == nil || req.Patient.Weight.Unit != "kg" {
					reason = "CONDITION_INPUT_MISSING"
				} else if !c.WeightKG.Contains(req.Patient.Weight.Value) {
					reason = "WEIGHT_CONDITION_NOT_MET"
				}
			}
			if reason != "" {
				fail(reason)
				continue
			}
		}
		if !rule.Range.Covers(estimate.Low, estimate.High) {
			fail("UNCERTAINTY_CROSSES_RULE_BOUNDARY")
			continue
		}
		var source domain.Source
		for _, s := range k.Sources {
			if s.ID == rule.Citation.SourceID {
				source = s
			}
		}
		r.Status = "RULE_MATCHED"
		r.Action = rule.Action
		r.DoseText = rule.DoseText
		r.Rule = &rule
		r.Source = &source
		r.Selected = estimate
		r.Explanation = fmt.Sprintf("%s: %s = %.6g %s matches rule %s version %s. Action: %s. SmPC %s, section %s, page %d. Quote: %s", ctx.Product, estimate.Metric, estimate.Value, estimate.Unit, rule.ID, rule.Version, rule.Action, source.Version, rule.Citation.Section, rule.Citation.Page, rule.Citation.Quote)
		if rule.DoseText != "" {
			r.Explanation += " Instructions: " + rule.DoseText
		}
		for _, rx := range req.Prescriptions {
			if rx.MedicationIndex == medicationIndex {
				r.PrescriptionCheck = comparePrescription(rx, rule)
			}
		}
		a.Results = append(a.Results, r)
	}
	return a
}

// Compare only explicit source-backed limits. No unit conversion or dosing inference.
func comparePrescription(rx domain.Prescription, r domain.Rule) *domain.PrescriptionCheck {
	out := &domain.PrescriptionCheck{Status: "MANUAL_REVIEW_REQUIRED", Reasons: []string{}}
	if r.Action == "AVOID" {
		out.Status = "OUTSIDE_RULE_LIMITS"
		out.Reasons = append(out.Reasons, "MATCHED_RULE_AVOIDS_USE")
		return out
	}
	if r.RegimenLimit == nil {
		out.Reasons = append(out.Reasons, "NO_STRUCTURED_REGIMEN_LIMIT")
		return out
	}
	cap := r.RegimenLimit
	if rx.Dose.Unit != cap.MaxSingleDose.Unit {
		out.Reasons = append(out.Reasons, "REGIMEN_UNIT_MISMATCH")
		return out
	}
	out.Status = "WITHIN_RULE_LIMITS"
	if rx.Dose.Value > cap.MaxSingleDose.Value {
		out.Reasons = append(out.Reasons, "SINGLE_DOSE_EXCEEDS_SOURCE_LIMIT")
	}
	if rx.IntervalHours < cap.MinIntervalHours {
		out.Reasons = append(out.Reasons, "INTERVAL_SHORTER_THAN_SOURCE_LIMIT")
	}
	if len(out.Reasons) > 0 {
		out.Status = "OUTSIDE_RULE_LIMITS"
	}
	return out
}
