package httpapi_test

import (
	"net/http"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/httpapi"
	"renalsafe/core/internal/store"
	"renalsafe/core/internal/testkit"
	"testing"
)

func TestExtractionHTTPPermissionsAndDisabled(t *testing.T) {
	db := dbtest.Database(t)
	doc := dbtest.AddDocument(t, db, testkit.Context)
	h := (&httpapi.Server{DB: db, Auth: credentials(t)}).Handler()
	for _, path := range []string{"/api/v1/extraction/status", "/api/v1/extractions", "/api/v1/extractions/unknown"} {
		if w := request(h, "physician", http.MethodGet, path, nil); w.Code != 403 {
			t.Fatal(path, w.Code)
		}
		if w := request(h, "", http.MethodGet, path, nil); w.Code != 401 {
			t.Fatal(path, w.Code)
		}
	}
	if w := request(h, "pharmacist", "GET", "/api/v1/extraction/status", nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := request(h, "pharmacist", "POST", "/api/v1/extractions", store.ExtractionInput{DocumentID: doc.ID, RequestKey: store.ID()}); w.Code != 503 {
		t.Fatal("disabled provider queued a job", w.Code)
	}
}
