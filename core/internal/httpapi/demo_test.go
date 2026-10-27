package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"renalsafe/core/internal/domain"
	"testing"
)

func TestIsolatedDemoScenarios(t *testing.T) {
	// No database or credentials: fixed fixtures are independent of clinical data.
	w := httptest.NewRecorder()
	(&Server{}).Handler().ServeHTTP(w, httptest.NewRequest("GET", "/demo/scenarios.json", nil))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var data struct {
		Mode      string `json:"mode"`
		Scenarios []struct {
			ID         string            `json:"id"`
			Assessment domain.Assessment `json:"assessment"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Mode != "SYNTHETIC_SOFTWARE_DEMO" || len(data.Scenarios) != 5 {
		t.Fatal("missing demo labels/scenarios")
	}
	for _, s := range data.Scenarios {
		if len(s.Assessment.Results) != 2 || s.Assessment.KnowledgeDigest == "" {
			t.Fatal("missing engine output")
		}
		for i, r := range s.Assessment.Results {
			switch s.ID {
			case "within", "outside", "higher-function":
				want := "WITHIN_RULE_LIMITS"
				if s.ID == "outside" && i == 1 {
					want = "OUTSIDE_RULE_LIMITS"
				}
				if r.Status != "RULE_MATCHED" || r.PrescriptionCheck == nil || r.PrescriptionCheck.Status != want {
					t.Fatalf("%s: %+v", s.ID, r)
				}
				action := "REDUCE_DOSE"
				if s.ID == "higher-function" {
					action = "NO_CHANGE"
				}
				if r.Action != action {
					t.Fatal("wrong range action")
				}
				if want == "OUTSIDE_RULE_LIMITS" && len(r.PrescriptionCheck.Reasons) != 2 {
					t.Fatal("dose and interval must fail")
				}
			default:
				if r.Status != "MANUAL_REVIEW_REQUIRED" || r.Action != "" || r.DoseText != "" {
					t.Fatal("unsafe refusal")
				}
				reason := "REQUIRED_RENAL_METRIC_UNAVAILABLE"
				if s.ID == "uncertainty" {
					reason = "UNCERTAINTY_CROSSES_RULE_BOUNDARY"
				}
				found := false
				for _, got := range r.Reasons {
					found = found || got == reason
				}
				if !found {
					t.Fatal("missing refusal reason")
				}
			}
		}
	}
}

func TestCustomDemoAssessment(t *testing.T) {
	initial, err := demoScenarios()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(initial)
	var examples struct {
		Scenarios []struct {
			Assessment domain.Assessment `json:"assessment"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(b, &examples); err != nil {
		t.Fatal(err)
	}
	req := examples.Scenarios[0].Assessment.Request
	handler := (&Server{}).Handler()
	send := func(req domain.Request) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/demo/assess", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)
		return w
	}
	req.Prescriptions[0].Dose.Value = 9
	req.Prescriptions[1].Dose.Value = 17
	req.Prescriptions[1].IntervalHours = 18
	w := send(req)
	if w.Code != 200 {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	var a domain.Assessment
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Request.Prescriptions[1].Dose.Value != 17 || a.Results[0].PrescriptionCheck.Status != "WITHIN_RULE_LIMITS" || a.Results[1].PrescriptionCheck.Status != "OUTSIDE_RULE_LIMITS" || len(a.Results[1].PrescriptionCheck.Reasons) != 2 {
		t.Fatal("custom input was not evaluated independently")
	}
	req.Patient.Creatinine.Value = 1
	w = send(req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &a)
	if a.Results[1].Action != "NO_CHANGE" || a.Results[1].PrescriptionCheck.Status != "WITHIN_RULE_LIMITS" {
		t.Fatal("custom creatinine did not change selected rule")
	}
	req.Medications[0].Product = "Meronem 1 g"
	if w = send(req); w.Code != 422 {
		t.Fatal("real product accepted in fictional mode")
	}
	req = examples.Scenarios[0].Assessment.Request
	req.Prescriptions[0].Dose.Value = -1
	if w = send(req); w.Code != 422 {
		t.Fatal("invalid prescription accepted")
	}
}
