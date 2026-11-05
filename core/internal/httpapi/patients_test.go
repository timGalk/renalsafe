package httpapi_test

import (
	"encoding/json"
	"net/http/httptest"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/httpapi"
	"renalsafe/core/internal/store"
	"strings"
	"testing"
	"time"
)

func TestPatientHTTPWorkflowAndIsolation(t *testing.T) {
	db := dbtest.Database(t)
	auth := credentials(t)
	if e := db.Provision(t.Context(), auth.Actors()); e != nil {
		t.Fatal(e)
	}
	h := (&httpapi.Server{DB: db, Auth: auth}).Handler()
	call := func(role, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+strings.Repeat(role, 8))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := call("physician", "POST", "/api/v1/patients", `{"alias":"Synthetic HTTP case","synthetic":true}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p store.SyntheticPatient
	json.Unmarshal(w.Body.Bytes(), &p)
	if w = call("pharmacist", "GET", "/api/v1/patients/"+p.ID, ""); w.Code != 404 {
		t.Fatal("unassigned pharmacist read", w.Code)
	}
	if w = call("pharmacist", "POST", "/api/v1/patients", `{"synthetic":true}`); w.Code != 403 {
		t.Fatal("pharmacist authored case")
	}
	in := store.ObservationInput{Creatinine: dbtest.CurrentRequest().Patient.Creatinine, MeasuredAt: time.Now().Add(-time.Hour), IDMS: dbtest.CurrentRequest().Patient.IDMS}
	raw, _ := json.Marshal(in)
	w = call("physician", "POST", "/api/v1/patients/"+p.ID+"/observations", string(raw))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var obs store.Observation
	json.Unmarshal(w.Body.Bytes(), &obs)
	assessment := store.PatientAssessmentInput{ObservationID: obs.ID, RetryKey: store.ID(), Request: dbtest.CurrentRequest()}
	raw, _ = json.Marshal(assessment)
	w = call("physician", "POST", "/api/v1/patients/"+p.ID+"/assessments", string(raw))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result store.AssessmentRecord
	json.Unmarshal(w.Body.Bytes(), &result)
	for _, suffix := range []string{"", "/export", "/report"} {
		if w = call("pharmacist", "GET", "/api/v1/assessments/"+result.ID+suffix, ""); w.Code != 404 {
			t.Fatal("unassigned result read", suffix, w.Code)
		}
	}
	w = call("physician", "PUT", "/api/v1/patients/"+p.ID+"/access/fixture-pharmacist", `{"granted":true}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("pharmacist", "GET", "/api/v1/assessments/"+result.ID+"/report", ""); w.Code != 200 || !strings.Contains(w.Body.String(), p.Code) {
		t.Fatal("shared report missing", w.Code)
	}
	if w = call("physician", "GET", "/api/v1/patients/"+p.ID+"/assessments?limit=1", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("physician", "GET", "/api/v1/assessments?limit=not-a-number", ""); w.Code != 422 {
		t.Fatal("invalid pagination accepted")
	}
	if w = call("physician", "GET", "/api/v1/assessments?from=not-a-date", ""); w.Code != 422 {
		t.Fatal("invalid date accepted")
	}
	w = call("physician", "PUT", "/api/v1/patients/"+p.ID+"/access/fixture-pharmacist", `{"granted":false}`)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = call("pharmacist", "POST", "/api/v1/assessments/"+result.ID+"/replay", ""); w.Code != 404 {
		t.Fatal("revoked replay")
	}
}
