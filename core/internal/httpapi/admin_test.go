package httpapi_test

import (
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/httpapi"
	"strings"
	"testing"
)

func TestAdministratorStatusAndBoundedAudit(t *testing.T) {
	db := dbtest.Database(t)
	auth := credentials(t)
	if e := db.Provision(t.Context(), auth.Actors()); e != nil {
		t.Fatal(e)
	}
	h := (&httpapi.Server{DB: db, Auth: auth}).Handler()
	for _, path := range []string{"/api/v1/admin/status", "/api/v1/admin/audit"} {
		if w := request(h, "physician", "GET", path, nil); w.Code != 403 {
			t.Fatal("role bypass", path, w.Code)
		}
		w := request(h, "admin", "GET", path, nil)
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if path == "/api/v1/admin/audit" && strings.Contains(w.Body.String(), `"data"`) {
			t.Fatal("audit payload exposed")
		}
	}
	if w := request(h, "admin", "POST", "/api/v1/admin/audit/verify", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatal("verification failed", w.Code)
	}
	if w := request(h, "admin", "GET", "/api/v1/admin/audit?cursor=invalid", nil); w.Code != 422 {
		t.Fatal("invalid cursor accepted")
	}
}
