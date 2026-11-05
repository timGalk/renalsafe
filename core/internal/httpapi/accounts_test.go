package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/httpapi"
	"renalsafe/core/internal/store"
	"strings"
	"testing"
)

func TestSessionCookieCSRFAndRevocation(t *testing.T) {
	db := dbtest.Database(t)
	ctx := t.Context()
	a := store.Actor{ID: "admin", Role: "admin"}
	if e := db.Provision(ctx, []store.Actor{a}); e != nil {
		t.Fatal(e)
	}
	secret, e := db.ResetUser(ctx, a, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Activate(ctx, a.ID, secret, "synthetic-long-password"); e != nil {
		t.Fatal(e)
	}
	h := (&httpapi.Server{DB: db, Auth: credentials(t)}).Handler()
	request := func(method, path, body, csrf string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("POST", "/api/v1/auth/login", `{"username":"admin","password":"synthetic-long-password"}`, "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("cookie flags")
	}
	var session store.Session
	if e = json.Unmarshal(w.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	if w = request("GET", "/api/v1/me", "", "", cookies[0]); w.Code != 200 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("GET", "http://localhost/api/v1/me", nil)
	r.AddCookie(cookies[0])
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("admin", 8))
	ambiguous := httptest.NewRecorder()
	h.ServeHTTP(ambiguous, r)
	if ambiguous.Code != 401 {
		t.Fatal("ambiguous credentials accepted")
	}
	secure := httptest.NewRequest("POST", "https://localhost/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"synthetic-long-password"}`))
	secure.Header.Set("Content-Type", "application/json")
	tlsResponse := httptest.NewRecorder()
	h.ServeHTTP(tlsResponse, secure)
	if tlsResponse.Code != 200 || !tlsResponse.Result().Cookies()[0].Secure {
		t.Fatal("HTTPS cookie missing Secure")
	}
	if w = request("POST", "/api/v1/auth/logout", "", "", cookies[0]); w.Code != 401 {
		t.Fatal("missing csrf accepted")
	}
	if w = request("POST", "/api/v1/auth/logout", "", session.CSRF, cookies[0]); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("GET", "/api/v1/auth/session", "", "", cookies[0]); w.Code != 401 {
		t.Fatal("logged out session usable")
	}
}
