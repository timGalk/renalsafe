package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"renalsafe/core/internal/auth"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/httpapi"
	"renalsafe/core/internal/store"
	"renalsafe/core/internal/testkit"
	"strings"
	"testing"
)

func credentials(t *testing.T) *auth.Auth {
	t.Helper()
	cs := []auth.Credential{}
	for _, role := range []string{"physician", "pharmacist", "admin"} {
		cs = append(cs, auth.Credential{ID: "fixture-" + role, Role: role, TokenSHA256: domain.BytesDigest([]byte(strings.Repeat(role, 8)))})
	}
	a, e := auth.New(cs)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func request(h http.Handler, role, method, path string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if role != "" {
		r.Header.Set("Authorization", "Bearer "+strings.Repeat(role, 8))
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func setup(t *testing.T) (*store.Store, http.Handler, store.StoredRule) {
	t.Helper()
	db := dbtest.Database(t)
	d := dbtest.AddDocument(t, db, testkit.Context)
	rule := dbtest.Pending(t, db, d)
	s := &httpapi.Server{DB: db, Auth: credentials(t), PDF: fakePDF{}}
	return db, s.Handler(), rule
}
func TestHTTPAssessmentAndScopedRead(t *testing.T) {
	_, h, _ := setup(t)
	w := request(h, "physician", "POST", "/api/v1/assessments", dbtest.CurrentRequest())
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved store.AssessmentRecord
	if e := json.Unmarshal(w.Body.Bytes(), &saved); e != nil {
		t.Fatal(e)
	}
	if saved.AuditHash == "" || saved.Assessment.Results[0].Action != "" {
		t.Fatal(saved)
	}
	if w = request(h, "physician", "GET", "/api/v1/assessments/"+saved.ID, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request(h, "pharmacist", "GET", "/api/v1/assessments/"+saved.ID, nil); w.Code != 403 {
		t.Fatal("assessment leaked", w.Code)
	}
	if w = request(h, "physician", "POST", "/api/v1/assessments/"+saved.ID+"/replay", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"reproduced":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHTTPAuthAndCannotForgeApproval(t *testing.T) {
	_, h, rule := setup(t)
	if w := request(h, "", "GET", "/api/v1/documents", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	for _, role := range []string{"physician", "admin"} {
		if w := request(h, role, "POST", "/api/v1/rules/"+rule.Rule.ID+"/approve", store.ReviewInput{ExpectedDigest: rule.Digest, Confirmed: true, Note: "fixture"}); w.Code != 403 {
			t.Fatal("wrong role approved", role, w.Code)
		}
	}
	if w := request(h, "pharmacist", "POST", "/api/v1/rules/"+rule.Rule.ID+"/approve", map[string]any{"expected_digest": rule.Digest, "clinical_review_confirmed": true, "note": "fixture", "reviewer": "forged-admin"}); w.Code != 422 {
		t.Fatal("forged field accepted", w.Code)
	}
	if w := request(h, "pharmacist", "POST", "/api/v1/rules/"+rule.Rule.ID+"/approve", store.ReviewInput{ExpectedDigest: rule.Digest, Confirmed: true, Note: "SYNTHETIC TEST ONLY"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHTTPNoSuccessWhenAuditFails(t *testing.T) {
	db, h, _ := setup(t)
	_, e := db.Pool.Exec(t.Context(), `CREATE FUNCTION fail_http_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'failure'; END $$;CREATE TRIGGER fail_http_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_http_audit()`)
	if e != nil {
		t.Fatal(e)
	}
	w := request(h, "physician", "POST", "/api/v1/assessments", dbtest.CurrentRequest())
	if w.Code != 503 || strings.Contains(w.Body.String(), `"assessment"`) {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHTTPStrictJSONAndSourceEvidence(t *testing.T) {
	_, h, _ := setup(t)
	r := httptest.NewRequest("POST", "/api/v1/assessments", strings.NewReader(`{"assessment_at":"2026-10-19T12:00:00Z","assessment_at":"2026-10-19T13:00:00Z"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("physician", 8))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatal("duplicate JSON accepted", w.Code)
	}
	w = request(h, "physician", "POST", "/api/v1/evidence/search", store.SearchInput{Context: testkit.Context, Query: "renal"})
	if w.Code != 200 || strings.Contains(w.Body.String(), `"action"`) || !strings.Contains(w.Body.String(), "EVIDENCE_ONLY") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(h, "physician", "GET", "/api/v1/audit", nil)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestBrowserAssetsAndCrossOriginGuard(t *testing.T) {
	s := &httpapi.Server{Auth: credentials(t)}
	h := s.Handler()
	for _, path := range []string{"/", "/patients", "/patients/synthetic-case", "/assessments/synthetic-result", "/admin/audit", "/demo.html", "/healthz"} {
		w := request(h, "", "GET", path, nil)
		if w.Code != 200 {
			t.Fatal(path, w.Code)
		}
		if w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("CSP absent")
		}
	}
	for _, path := range []string{"/api/v1/not-a-route", "/healthz/not-a-route", "/assets/missing.js", "/app.js"} {
		w := request(h, "", "GET", path, nil)
		if w.Code != 404 || strings.Contains(w.Body.String(), `id="root"`) {
			t.Fatal("reserved route fell through to shell", path, w.Code)
		}
	}
	root := request(h, "", "GET", "/", nil).Body.String()
	start := strings.Index(root, `src="/assets/`)
	if start < 0 {
		t.Fatal("built React asset missing")
	}
	script := strings.Split(root[start+5:], `"`)[0]
	if w := request(h, "", "GET", script, nil); w.Code != 200 {
		t.Fatal("hashed asset unavailable", script, w.Code)
	}
	r := httptest.NewRequest("GET", "http://local.test/healthz", nil)
	r.Header.Set("Origin", "https://other.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin accepted")
	}
}

type fakePDF struct{}

func (fakePDF) Extract(context.Context, []byte) (string, error) { return dbtest.DocumentText, nil }
func TestHTTPDocumentUpload(t *testing.T) {
	_, h, _ := setup(t)
	in := store.DocumentInput{Context: testkit.Context, URL: "https://example.invalid/synthetic", Version: "synthetic-upload", PageStart: 1, PageEnd: 3}
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	metadata, _ := json.Marshal(in)
	_ = m.WriteField("metadata", string(metadata))
	part, e := m.CreateFormFile("file", "fixture.pdf")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = fmt.Fprint(part, "%PDF-SYNTHETIC")
	_ = m.Close()
	r := httptest.NewRequest("POST", "/api/v1/documents", &body)
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("pharmacist", 8))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 || !strings.Contains(w.Body.String(), "ACTIVE") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestOpenAPISchemasAndReferences(t *testing.T) {
	spec := httpapi.OpenAPI()
	if spec["openapi"] != "3.1.0" {
		t.Fatal("wrong version")
	}
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	raw, _ := json.Marshal(spec)
	var v any
	_ = json.Unmarshal(raw, &v)
	var visit func(any)
	visit = func(value any) {
		switch x := value.(type) {
		case map[string]any:
			if ref, ok := x["$ref"].(string); ok {
				if !strings.HasPrefix(ref, "#/components/schemas/") || schemas[strings.TrimPrefix(ref, "#/components/schemas/")] == nil {
					t.Fatal("unresolved reference", ref)
				}
			}
			for _, v := range x {
				visit(v)
			}
		case []any:
			for _, v := range x {
				visit(v)
			}
		}
	}
	visit(v)
	for _, name := range []string{"Prescription", "RegimenLimit", "RuleConditions", "EvidenceResult", "AssessmentRecord"} {
		if schemas[name] == nil {
			t.Fatal("missing payload", name)
		}
	}
}
