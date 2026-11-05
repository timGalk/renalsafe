package httpapi

import (
	"crypto/subtle"
	"net"
	"net/http"
	"renalsafe/core/internal/store"
	"strings"
)

const sessionCookie = "renalsafe_session"

func (s *Server) identity(r *http.Request) (store.Actor, bool) {
	cookie, e := r.Cookie(sessionCookie)
	header := r.Header.Get("Authorization")
	if e == nil && header != "" {
		return store.Actor{}, false
	}
	if e == nil {
		if s.DB == nil {
			return store.Actor{}, false
		}
		session, e := s.DB.Session(r.Context(), cookie.Value)
		if e != nil || session.MustChange && r.URL.Path != "/api/v1/auth/password" && r.URL.Path != "/api/v1/auth/logout" {
			return store.Actor{}, false
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(session.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			return store.Actor{}, false
		}
		*r = *r.WithContext(store.AuthorizeContext(r.Context(), session.Actor, cookie.Value))
		return session.Actor, true
	}
	if s.Auth == nil {
		return store.Actor{}, false
	}
	a, ok := s.Auth.Authenticate(header)
	if !ok {
		return a, false
	}
	if s.DB != nil {
		var err error
		a, err = s.DB.LegacyActor(r.Context(), a)
		if err != nil {
			return store.Actor{}, false
		}
	}
	*r = *r.WithContext(store.AuthorizeContext(r.Context(), a, ""))
	return a, true
}
func localSessionTransport(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host := r.Host
	if h, _, e := net.SplitHostPort(host); e == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || host == "::1"
}
func setSession(w http.ResponseWriter, r *http.Request, raw string) {
	age := 8 * 60 * 60
	if raw == "" {
		age = -1
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: raw, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (s *Server) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if s.DB == nil || !localSessionTransport(r) {
			fail(w, store.ErrForbidden)
			return
		}
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if e := readJSON(w, r, &in); e != nil {
			fail(w, e)
			return
		}
		if len(in.Username) > 80 || len(in.Password) > 256 {
			respond(w, 401, map[string]string{"error": "invalid_credentials_or_try_later"})
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		raw, out, e := s.DB.Login(r.Context(), strings.ToLower(strings.TrimSpace(in.Username)), in.Password, ip)
		if e != nil {
			respond(w, 401, map[string]string{"error": "invalid_credentials_or_try_later"})
			return
		}
		setSession(w, r, raw)
		respond(w, 200, out)
	})
	mux.HandleFunc("POST /api/v1/auth/activate", func(w http.ResponseWriter, r *http.Request) {
		if s.DB == nil || !localSessionTransport(r) {
			fail(w, store.ErrForbidden)
			return
		}
		var in struct {
			Username string `json:"username"`
			Secret   string `json:"activation_secret"`
			Password string `json:"password"`
		}
		if e := readJSON(w, r, &in); e != nil {
			fail(w, e)
			return
		}
		if e := s.DB.Activate(r.Context(), in.Username, in.Secret, in.Password); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]string{"status": "password_set_sign_in"})
	})
	mux.HandleFunc("GET /api/v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(sessionCookie)
		if e != nil || s.DB == nil || r.Header.Get("Authorization") != "" {
			respond(w, 401, map[string]string{"error": "authentication_required"})
			return
		}
		out, e := s.DB.Session(r.Context(), c.Value)
		if e != nil {
			respond(w, 401, map[string]string{"error": "authentication_required"})
			return
		}
		respond(w, 200, out)
	})
	protected := func(pattern string, admin bool, fn func(http.ResponseWriter, *http.Request, store.Actor) error) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			a, ok := s.identity(r)
			if !ok {
				respond(w, 401, map[string]string{"error": "authentication_required"})
				return
			}
			if admin && a.Role != "admin" {
				fail(w, store.ErrForbidden)
				return
			}
			if e := fn(w, r, a); e != nil {
				fail(w, e)
			}
		})
	}
	protected("POST /api/v1/auth/logout", false, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		c, e := r.Cookie(sessionCookie)
		if e != nil {
			return store.ErrInvalid
		}
		if e = s.DB.Logout(r.Context(), c.Value, a); e != nil {
			return e
		}
		setSession(w, r, "")
		return respond(w, 200, map[string]string{"status": "signed_out"})
	})
	protected("POST /api/v1/auth/password", false, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in struct {
			Current string `json:"current_password"`
			Next    string `json:"new_password"`
		}
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		if e := s.DB.ChangePassword(r.Context(), a, in.Current, in.Next); e != nil {
			return e
		}
		setSession(w, r, "")
		return respond(w, 200, map[string]string{"status": "password_changed_sign_in"})
	})
	protected("GET /api/v1/admin/users", true, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Users(r.Context(), a)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	save := func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in struct {
			Username string `json:"username"`
			Role     string `json:"role"`
			Active   bool   `json:"active"`
		}
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.ManageUser(r.Context(), a, r.PathValue("id"), in.Username, in.Role, in.Active)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	}
	protected("POST /api/v1/admin/users", true, save)
	protected("PATCH /api/v1/admin/users/{id}", true, save)
	protected("POST /api/v1/admin/users/{id}/reset", true, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		secret, e := s.DB.ResetUser(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		return respond(w, 200, map[string]string{"activation_secret": secret, "expires_in": "30 minutes"})
	})
}
