package decision

import (
	"math"
	"os"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"renalsafe/core/internal/testkit"
	"testing"
	"time"
)

func TestThresholdsUnrounded(t *testing.T) {
	k, root := testkit.Fixture(t)
	s := testkit.Load(t, k, root)
	for _, c := range []struct {
		scr    float64
		action string
	}{{math.Nextafter(70.0/30.0, math.Inf(1)), "AVOID"}, {70.0 / 30.0, "NO_CHANGE"}, {math.Nextafter(70.0/30.0, 0), "NO_CHANGE"}} {
		req := testkit.Request()
		req.Patient.Creatinine.Value = c.scr
		r := Assess(req, s).Results[0]
		if r.Status != "RULE_MATCHED" || r.Action != c.action {
			t.Fatalf("%.17g %+v", c.scr, r)
		}
		if r.Selected.Metric != domain.CrCl || r.Source == nil || r.Rule == nil || r.Explanation == "" {
			t.Fatal(r)
		}
	}
}
func manual(t *testing.T, r domain.Result) {
	t.Helper()
	if r.Status != "MANUAL_REVIEW_REQUIRED" || r.Action != "" || r.DoseText != "" || r.Rule != nil {
		t.Fatalf("unsafe fallback %+v", r)
	}
}
func TestLifecycleEligibility(t *testing.T) {
	for _, status := range []string{"PENDING", "REJECTED", "SUPERSEDED"} {
		k, root := testkit.Fixture(t)
		for i := range k.Rules {
			k.Rules[i].Status = status
		}
		manual(t, Assess(testkit.Request(), testkit.Load(t, k, root)).Results[0])
	}
}
func TestContextNoWildcard(t *testing.T) {
	k, root := testkit.Fixture(t)
	s := testkit.Load(t, k, root)
	for _, change := range []func(*domain.Context){func(c *domain.Context) { c.Product = "other" }, func(c *domain.Context) { c.Form = "other" }, func(c *domain.Context) { c.Route = "other" }, func(c *domain.Context) { c.Indication = "other" }, func(c *domain.Context) { c.Indication = "" }} {
		req := testkit.Request()
		change(&req.Medications[0])
		manual(t, Assess(req, s).Results[0])
	}
}
func TestUncertainty(t *testing.T) {
	k, root := testkit.Fixture(t)
	s := testkit.Load(t, k, root)
	req := testkit.Request()
	req.Patient.CreatinineBounds = &domain.Interval{Lower: 2, Upper: testkit.Float(3), LowerInclusive: true, UpperInclusive: true}
	r := Assess(req, s).Results[0]
	manual(t, r)
	if r.Reasons[0] != "UNCERTAINTY_CROSSES_RULE_BOUNDARY" {
		t.Fatal(r)
	}
	req.Patient.Creatinine.Value = 3
	req.Patient.CreatinineBounds = &domain.Interval{Lower: 2.9, Upper: testkit.Float(3.1), LowerInclusive: true, UpperInclusive: true}
	if r = Assess(req, s).Results[0]; r.Action != "AVOID" {
		t.Fatal(r)
	}
}
func TestMissingCGNeverUsesEGFR(t *testing.T) {
	k, root := testkit.Fixture(t)
	req := testkit.Request()
	req.Patient.Weight = nil
	manual(t, Assess(req, testkit.Load(t, k, root)).Results[0])
}
func TestMetricAmbiguityAndEGFR(t *testing.T) {
	k, root := testkit.Fixture(t)
	r := k.Rules[0]
	r.ID = "egfr"
	r.Metric = domain.EGFR
	r.Unit = domain.EGFR.Unit()
	r.Range = domain.Interval{Lower: 0, LowerInclusive: true}
	testkit.Sign(&r)
	k.Rules = append(k.Rules, r)
	manual(t, Assess(testkit.Request(), testkit.Load(t, k, root)).Results[0])
	k.Rules = []domain.Rule{r}
	result := Assess(testkit.Request(), testkit.Load(t, k, root)).Results[0]
	if result.Selected == nil || result.Selected.Metric != domain.EGFR {
		t.Fatal(result)
	}
}
func TestGapAndInvalidPatient(t *testing.T) {
	k, root := testkit.Fixture(t)
	k.Rules = k.Rules[:1]
	manual(t, Assess(testkit.Request(), testkit.Load(t, k, root)).Results[0])
	req := testkit.Request()
	req.Patient.AKI = testkit.Bool(true)
	manual(t, Assess(req, testkit.Load(t, k, root)).Results[0])
	manual(t, Assess(testkit.Request(), rules.Snapshot{}).Results[0])
}
func TestDeterministicFrozenSnapshot(t *testing.T) {
	k, root := testkit.Fixture(t)
	s := testkit.Load(t, k, root)
	req := testkit.Request()
	a := Assess(req, s)
	for i := 0; i < 100; i++ {
		if domain.Digest(Assess(req, s)) != domain.Digest(a) {
			t.Fatal("nondeterministic")
		}
	}
	copy := s.Knowledge()
	copy.Rules[0].Action = "NO_CHANGE"
	if domain.Digest(Assess(req, s)) != domain.Digest(a) {
		t.Fatal("snapshot mutated")
	}
	k.Version = "v2"
	if testkit.Load(t, k, root).Digest() == s.Digest() {
		t.Fatal("version not addressed")
	}
}
func TestRealCandidatePending(t *testing.T) {
	b, e := os.ReadFile("../../../data/knowledge.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := rules.Load(b, "../../../data", false)
	if e != nil {
		t.Fatal(e)
	}
	k := s.Knowledge()
	if k.Synthetic || len(k.Rules) != 1 || k.Rules[0].Status != "PENDING" {
		t.Fatal(k)
	}
	req := testkit.Request()
	req.Medications = []domain.Context{k.Rules[0].Context}
	req.Patient.Creatinine.Value = 3
	manual(t, Assess(req, s).Results[0])
}

func TestFutureApprovalNotEffective(t *testing.T) {
	k, root := testkit.Fixture(t)
	for i := range k.Rules {
		k.Rules[i].Approval.At = testkit.At.Add(time.Hour)
	}
	manual(t, Assess(testkit.Request(), testkit.Load(t, k, root)).Results[0])
}

func TestSourceBackedConditionsAndRegimenComparison(t *testing.T) {
	k, root := testkit.Fixture(t)
	req := testkit.Request()
	k.Rules[1].Conditions = &domain.RuleConditions{AgeYears: &domain.Interval{Lower: 18, Upper: testkit.Float(120), LowerInclusive: true, UpperInclusive: true}, WeightKG: &domain.Interval{Lower: 20, Upper: testkit.Float(100), LowerInclusive: true, UpperInclusive: true}}
	k.Rules[1].RegimenLimit = &domain.RegimenLimit{MaxSingleDose: domain.Quantity{Value: 10, Unit: "mg"}, MinIntervalHours: 24}
	testkit.Sign(&k.Rules[1])
	s := testkit.Load(t, k, root)
	for _, c := range []struct {
		dose, interval float64
		unit, status   string
	}{{10, 24, "mg", "WITHIN_RULE_LIMITS"}, {11, 12, "mg", "OUTSIDE_RULE_LIMITS"}, {1, 24, "g", "MANUAL_REVIEW_REQUIRED"}} {
		req.Prescriptions = []domain.Prescription{{MedicationIndex: 0, Dose: domain.Quantity{Value: c.dose, Unit: c.unit}, IntervalHours: c.interval}}
		r := Assess(req, s).Results[0]
		if r.PrescriptionCheck == nil || r.PrescriptionCheck.Status != c.status {
			t.Fatalf("%+v %+v", c, r)
		}
	}
	k.Rules[1].Conditions.AgeYears.Upper = testkit.Float(30)
	testkit.Sign(&k.Rules[1])
	manual(t, Assess(req, testkit.Load(t, k, root)).Results[0])
	k.Rules[1].Conditions = nil
	k.Rules[1].RegimenLimit = nil
	testkit.Sign(&k.Rules[1])
	r := Assess(req, testkit.Load(t, k, root)).Results[0]
	if r.PrescriptionCheck.Status != "MANUAL_REVIEW_REQUIRED" {
		t.Fatal("invented regimen limit")
	}
	req.Prescriptions = append(req.Prescriptions, req.Prescriptions[0])
	manual(t, Assess(req, testkit.Load(t, k, root)).Results[0])
}
