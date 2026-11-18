package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/extraction"
	"renalsafe/core/internal/httpapi"
	"renalsafe/core/internal/store"
	"renalsafe/core/internal/testkit"
	"testing"
)

type fixtureCombinationReviewer struct {
	extraction.Provider
	input []extraction.CombinationPassage
}

func (f *fixtureCombinationReviewer) Model() string { return "fixture" }
func (f *fixtureCombinationReviewer) ReviewCombination(_ context.Context, passages []extraction.CombinationPassage) (extraction.CombinationReview, error) {
	f.input = passages
	return extraction.CombinationReview{Provider: "fixture", Model: "fixture", Findings: []extraction.CombinationFinding{}}, nil
}

func TestCombinationReviewRespectsAssessmentAccessAndDocumentScope(t *testing.T) {
	db := dbtest.Database(t)
	first, second, wrongRoute := testkit.Context, testkit.Context, testkit.Context
	second.Product = "TRAINING B"
	wrongRoute.Route = "different route"
	// Prototype data marked synthetic is ineligible. The two other fixtures
	// exercise the scope of records the real app marks as public documents.
	dbtest.AddDocument(t, db, first)
	db.AllowSynthetic = false
	one := dbtest.AddDocument(t, db, first)
	two := dbtest.AddDocument(t, db, second)
	dbtest.AddDocument(t, db, wrongRoute)
	db.AllowSynthetic = true

	stub := &fixtureCombinationReviewer{}
	handler := (&httpapi.Server{DB: db, Auth: credentials(t), Extractor: stub}).Handler()
	req := dbtest.CurrentRequest()
	req.Medications = append(req.Medications[:0:0], first, second)
	w := request(handler, "physician", http.MethodPost, "/api/v1/assessments", req)
	var saved store.AssessmentRecord
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &saved) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	path := "/api/v1/assessments/" + saved.ID + "/combination-review"
	if w := request(handler, "pharmacist", http.MethodPost, path, struct{}{}); w.Code != http.StatusForbidden || stub.input != nil {
		t.Fatal("provider called without case access", w.Code)
	}
	if w := request(handler, "physician", http.MethodPost, path, map[string]string{"patient": "do not send"}); w.Code != http.StatusUnprocessableEntity || stub.input != nil {
		t.Fatal("unexpected body accepted", w.Code)
	}
	w = request(handler, "physician", http.MethodPost, path, struct{}{})
	var out httpapi.CombinationReviewResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Uncovered) != 0 || out.Limited {
		t.Fatal(w.Code, w.Body.String())
	}
	seen := map[string]bool{}
	for _, passage := range stub.input {
		if passage.DocumentID != one.ID && passage.DocumentID != two.ID || passage.Section == "1" {
			t.Fatal("unexpected public evidence scope", passage.DocumentID, passage.Section)
		}
		seen[passage.DocumentID] = true
	}
	if len(seen) != 2 {
		t.Fatalf("expected passages for both medicines, got %d documents", len(seen))
	}
	after := request(handler, "physician", http.MethodGet, "/api/v1/assessments/"+saved.ID, nil)
	var unchanged store.AssessmentRecord
	if after.Code != http.StatusOK || json.Unmarshal(after.Body.Bytes(), &unchanged) != nil ||
		domain.Digest(saved.Assessment) != domain.Digest(unchanged.Assessment) {
		t.Fatal("saved assessment changed")
	}
}
