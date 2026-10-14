// Package decision contains no model calls or implicit metric conversions.
package decision

import (
	"fmt"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/renal"
	"renalsafe/core/internal/rules"
)

const Version = "0.1.0"

func Assess(req domain.Request, kb rules.Snapshot) domain.Assessment {
	k := kb.Knowledge()
	a := domain.Assessment{EngineVersion: Version, PolicyVersion: renal.PolicyVersion, KnowledgeVersion: k.Version, KnowledgeDigest: kb.Digest(), Request: req, Renal: renal.Calculate(req.Patient, req.At), Results: []domain.Result{}}
	for _, ctx := range req.Medications {
		r := domain.Result{Context: ctx, Status: "MANUAL_REVIEW_REQUIRED", Reasons: []string{}}
		fail := func(reason string) {
			r.Reasons = append(r.Reasons, reason)
			r.Explanation = "Manual review required: " + reason + ". No automated action or dose is provided."
			a.Results = append(a.Results, r)
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
		a.Results = append(a.Results, r)
	}
	return a
}
