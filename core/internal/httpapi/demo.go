package httpapi

import (
	"encoding/json"
	"renalsafe/core/internal/decision"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"renalsafe/core/internal/store"
	"time"
)

// demoSnapshot is isolated from the clinical database.
func demoSnapshot() (rules.Snapshot, error) {
	at := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	upper := 30.0
	text := []byte("FICTIONAL SOFTWARE FIXTURE. Not a medicine or clinical evidence. CrCl 0 to below 30: maximum single dose 10 mg, minimum interval 24 hours. CrCl 30 or above: maximum single dose 20 mg, minimum interval 12 hours.")
	hash := domain.BytesDigest(text)
	source := domain.Source{ID: "training", URL: "https://example.invalid/software-fixture", Version: "training-v1", RetrievedAt: at.Add(-48 * time.Hour), DocumentFile: "fixture.txt", TextFile: "fixture.txt", DocumentSHA256: hash, TextSHA256: hash, Synthetic: true}
	contexts := []domain.Context{{Product: "TRAINING A — fictional", Route: "fixture", Form: "fixture", Indication: "software demonstration"}, {Product: "TRAINING B — fictional", Route: "fixture", Form: "fixture", Indication: "software demonstration"}}
	kb := domain.Knowledge{Version: "training-only-v1", Synthetic: true, Sources: []domain.Source{source}, Rules: []domain.Rule{}}
	for _, ctx := range contexts {
		for i, interval := range []domain.Interval{{Lower: 0, Upper: &upper, LowerInclusive: true}, {Lower: 30, LowerInclusive: true}} {
			dose, hours, action, instructions := 10.0, 24.0, "REDUCE_DOSE", "Training limit: 10 mg per dose; interval at least 24 hours."
			if i == 1 {
				dose, hours, action, instructions = 20, 12, "NO_CHANGE", "Training limit: 20 mg per dose; interval at least 12 hours."
			}
			rule := domain.Rule{ID: ctx.Product + "-" + action, Version: "training-v1", Context: ctx, Metric: domain.CrCl, Unit: domain.CrCl.Unit(), Range: interval, Action: action, DoseText: instructions, Status: "APPROVED", RegimenLimit: &domain.RegimenLimit{MaxSingleDose: domain.Quantity{Value: dose, Unit: "mg"}, MinIntervalHours: hours}, Citation: domain.Citation{SourceID: source.ID, SourceVersion: source.Version, DocumentSHA256: hash, Section: "software-fixture", Page: 1, Quote: string(text)}}
			// Fixture metadata required by the normal snapshot validator. This is explicitly
			// not a human review, and is never inserted into the application's rule store.
			rule.Approval = &domain.Approval{Reviewer: "SOFTWARE-FIXTURE-NOT-A-HUMAN-REVIEW", Role: "pharmacist", At: at.Add(-24 * time.Hour), ContentDigest: rule.ContentDigest()}
			kb.Rules = append(kb.Rules, rule)
		}
	}
	encoded, err := json.Marshal(kb)
	if err != nil {
		return rules.Snapshot{}, err
	}
	return rules.LoadWithFiles(encoded, map[string][]byte{"fixture.txt": text}, true)
}

func demoScenarios() (any, error) {
	snapshot, err := demoSnapshot()
	if err != nil {
		return nil, err
	}
	at := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	yes, no := true, false
	contexts := []domain.Context{snapshot.Knowledge().Rules[0].Context, snapshot.Knowledge().Rules[2].Context}
	type scenario struct {
		ID          string            `json:"id"`
		Title       string            `json:"title"`
		Description string            `json:"description"`
		Assessment  domain.Assessment `json:"assessment"`
	}
	out := []scenario{}
	for _, id := range []string{"within", "outside", "higher-function", "missing-weight", "uncertainty"} {
		req := domain.Request{At: at, Patient: domain.Patient{Age: 70, Sex: "male", Weight: &domain.Quantity{Value: 72, Unit: "kg"}, Height: &domain.Quantity{Value: 175, Unit: "cm"}, Creatinine: domain.Quantity{Value: 2.8, Unit: "mg/dL"}, MeasuredAt: at.Add(-time.Hour), Stable: &yes, AKI: &no, Dialysis: &no, IDMS: &yes, TypicalBodyComposition: &yes, ActualWeightConfirmed: &yes}, Medications: contexts, Prescriptions: []domain.Prescription{{MedicationIndex: 0, Dose: domain.Quantity{Value: 10, Unit: "mg"}, IntervalHours: 24}, {MedicationIndex: 1, Dose: domain.Quantity{Value: 10, Unit: "mg"}, IntervalHours: 24}}}
		title, description := "1. Both regimens within rule limits", "CrCl 25 mL/min. Training limits for each product: maximum 10 mg per dose, minimum interval 24 h."
		switch id {
		case "outside":
			title, description = "2. Two products, different results", "A: 10 mg every 24 h. B: 20 mg every 12 h — the single dose exceeds the limit and the interval is too short."
			req.Prescriptions[1].Dose.Value = 20
			req.Prescriptions[1].IntervalHours = 12
		case "higher-function":
			title, description = "3. Different renal range, different action", "Creatinine 1 mg/dL → CrCl 70 mL/min. The engine selects NO_CHANGE with limits of 20 mg per dose and a minimum interval of 12 h."
			req.Patient.Creatinine.Value = 1
			for i := range req.Prescriptions {
				req.Prescriptions[i].Dose.Value = 20
				req.Prescriptions[i].IntervalHours = 12
			}
		case "missing-weight":
			title, description = "4. Manual review: missing weight", "eGFR is available, but these rules require CrCl. Renal metrics cannot be substituted."
			req.Patient.Weight = nil
		case "uncertainty":
			title, description = "5. Manual review: uncertainty", "Creatinine 2.8 mg/dL, bounds 2–3 mg/dL → CrCl 23.33–35 mL/min. This interval crosses the training boundary of 30."
			hi := 3.0
			req.Patient.CreatinineBounds = &domain.Interval{Lower: 2, Upper: &hi, LowerInclusive: true, UpperInclusive: true}
		}
		out = append(out, scenario{id, title, description, decision.Assess(req, snapshot)})
	}
	return struct {
		Mode      string     `json:"mode"`
		Notice    string     `json:"notice"`
		Scenarios []scenario `json:"scenarios"`
	}{"SYNTHETIC_SOFTWARE_DEMO", "Fictional products and rules. APPROVED is a software fixture state only, not clinical approval. Results are not saved to the clinical database or audit log.", out}, nil
}

// assessDemo accepts custom synthetic inputs with the fixed fictional catalog only.
func assessDemo(req domain.Request) (domain.Assessment, error) {
	snapshot, err := demoSnapshot()
	if err != nil {
		return domain.Assessment{}, err
	}
	if req.At.IsZero() || len(req.Medications) < 1 || len(req.Medications) > 2 || domain.ValidatePrescriptionInputs(req) != nil {
		return domain.Assessment{}, store.ErrInvalid
	}
	allowed := map[domain.Context]bool{}
	for _, rule := range snapshot.Knowledge().Rules {
		allowed[rule.Context] = true
	}
	seen := map[domain.Context]bool{}
	for _, context := range req.Medications {
		if !allowed[context] || seen[context] {
			return domain.Assessment{}, store.ErrInvalid
		}
		seen[context] = true
	}
	return decision.Assess(req, snapshot), nil
}
