// Package httpapi serves the local assessment, evidence and pharmacist review workflow.
package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"renalsafe/core/internal/auth"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/evidence"
	"renalsafe/core/internal/store"
	"strings"
	"time"
)

//go:embed web/dist web/demo.*
var assets embed.FS

type Server struct {
	DB       *store.Store
	Auth     *auth.Auth
	Embedder evidence.Embedder
	PDF      evidence.PDFExtractor
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.accountRoutes(mux)
	mux.HandleFunc("POST /demo/assess", func(w http.ResponseWriter, r *http.Request) {
		var req domain.Request
		if err := readJSON(w, r, &req); err != nil {
			fail(w, err)
			return
		}
		out, err := assessDemo(req)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, out)
	})
	mux.HandleFunc("GET /demo/scenarios.json", func(w http.ResponseWriter, r *http.Request) {
		out, err := demoScenarios()
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, out)
	})
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) { _ = respond(w, 200, OpenAPI()) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "alive"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if e := s.DB.Pool.Ping(ctx); e != nil {
			respond(w, 503, map[string]string{"status": "database_unavailable"})
			return
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	api := func(pattern string, roles string, fn func(http.ResponseWriter, *http.Request, store.Actor) error) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			actor, ok := s.identity(r)
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				respond(w, 401, map[string]string{"error": "authentication_required"})
				return
			}
			if !strings.Contains(" "+roles+" ", " "+actor.Role+" ") {
				fail(w, store.ErrForbidden)
				return
			}
			if e := fn(w, r, actor); e != nil {
				fail(w, e)
			}
		})
	}
	s.patientRoutes(api)
	s.adminRoutes(api)
	all := "physician pharmacist admin"
	edit := "pharmacist admin"
	api("GET /api/v1/me", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error { return respond(w, 200, a) })
	api("GET /api/v1/catalog", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		docs, e := s.DB.Documents(r.Context())
		if e != nil {
			return e
		}
		contexts := []domain.Context{}
		seen := map[domain.Context]bool{}
		for _, d := range docs {
			if d.Status == "ACTIVE" && !seen[d.Context] {
				contexts = append(contexts, d.Context)
				seen[d.Context] = true
			}
		}
		return respond(w, 200, contexts)
	})
	api("POST /api/v1/assessments", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in domain.Request
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.Assess(r.Context(), a, in)
		if e != nil {
			return e
		}
		w.Header().Set("Location", "/api/v1/assessments/"+out.ID)
		return respond(w, 201, out)
	})
	api("GET /api/v1/assessments/{id}", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Assessment(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/assessments/{id}/replay", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		ok, e := s.DB.Replay(r.Context(), a, r.PathValue("id"))
		if e != nil {
			return e
		}
		return respond(w, 200, map[string]bool{"reproduced": ok})
	})
	api("GET /api/v1/documents", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Documents(r.Context())
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/documents/{id}/chunks", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Chunks(r.Context(), r.PathValue("id"))
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/documents/{id}/pdf", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.DocumentPDF(r.Context(), r.PathValue("id"))
		if e != nil {
			return e
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "source.pdf"}))
		_, e = w.Write(out)
		return e
	})
	api("POST /api/v1/documents", edit, s.upload)
	api("POST /api/v1/documents/{id}/supersede", edit, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in struct {
			ReplacementID string `json:"replacement_id"`
		}
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		if e := s.DB.Supersede(r.Context(), a, r.PathValue("id"), in.ReplacementID); e != nil {
			return e
		}
		return respond(w, 200, map[string]string{"status": "SUPERSEDED"})
	})
	api("POST /api/v1/documents/{id}/embeddings", edit, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		count, e := s.DB.IndexEmbeddings(r.Context(), a, r.PathValue("id"), s.Embedder)
		if e != nil {
			return e
		}
		return respond(w, 200, map[string]int{"indexed_chunks": count})
	})
	api("GET /api/v1/rules", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Rules(r.Context(), r.URL.Query().Get("status"))
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/rules", edit, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.CandidateInput
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.AddCandidate(r.Context(), a, in)
		if e != nil {
			return e
		}
		return respond(w, 201, out)
	})
	api("PATCH /api/v1/rules/{id}", edit, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in struct {
			Expected string      `json:"expected_digest"`
			Rule     domain.Rule `json:"rule"`
		}
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.EditCandidate(r.Context(), a, r.PathValue("id"), in.Expected, in.Rule)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	for _, action := range []string{"approve", "reject"} {
		api("POST /api/v1/rules/{id}/"+action, "pharmacist", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
			var in store.ReviewInput
			if e := readJSON(w, r, &in); e != nil {
				return e
			}
			out, e := s.DB.Review(r.Context(), a, r.PathValue("id"), action, in)
			if e != nil {
				return e
			}
			return respond(w, 200, out)
		})
	}
	api("POST /api/v1/evidence/search", all, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.SearchInput
		if e := readJSON(w, r, &in); e != nil {
			return e
		}
		out, e := s.DB.Search(r.Context(), a, in, s.Embedder)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/audit", "admin", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.Audit(r.Context())
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	mux.Handle("GET /", browserAssets())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if origin := r.Header.Get("Origin"); origin != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if origin != scheme+"://"+r.Host {
				respond(w, 403, map[string]string{"error": "cross_origin_forbidden"})
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func respond(w http.ResponseWriter, status int, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, e = w.Write(append(b, '\n'))
	return e
}

type inputError struct {
	status  int
	message string
}

func (e inputError) Error() string { return e.message }
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" {
		return inputError{415, "application_json_required"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	b, e := io.ReadAll(r.Body)
	if e != nil {
		return inputError{413, "request_too_large"}
	}
	if e = domain.Decode(b, v); e != nil {
		return fmt.Errorf("%w: invalid or ambiguous JSON", store.ErrInvalid)
	}
	return nil
}
func fail(w http.ResponseWriter, e error) {
	var input inputError
	switch {
	case errors.As(e, &input):
		respond(w, input.status, map[string]string{"error": input.message})
	case errors.Is(e, store.ErrInvalid):
		respond(w, 422, map[string]string{"error": e.Error()})
	case errors.Is(e, store.ErrNotFound):
		respond(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, store.ErrForbidden):
		respond(w, 403, map[string]string{"error": "forbidden"})
	case errors.Is(e, store.ErrConflict):
		respond(w, 409, map[string]string{"error": "stale_or_conflicting_state"})
	default:
		respond(w, 503, map[string]string{"error": "operation_not_committed_or_dependency_unavailable"})
	}
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request, a store.Actor) error {
	r.Body = http.MaxBytesReader(w, r.Body, 14<<20)
	if e := r.ParseMultipartForm(1 << 20); e != nil {
		return inputError{413, "invalid_or_oversized_multipart"}
	}
	defer r.MultipartForm.RemoveAll()
	if len(r.MultipartForm.Value) != 1 || len(r.MultipartForm.Value["metadata"]) != 1 || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		return store.ErrInvalid
	}
	var in store.DocumentInput
	if e := domain.Decode([]byte(r.FormValue("metadata")), &in); e != nil {
		return store.ErrInvalid
	}
	f, _, e := r.FormFile("file")
	if e != nil {
		return store.ErrInvalid
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 12<<20+1))
	if e != nil || len(b) > 12<<20 {
		return inputError{413, "pdf_too_large"}
	}
	if s.PDF == nil {
		return fmt.Errorf("PDF extractor unavailable")
	}
	text, e := s.PDF.Extract(r.Context(), b)
	if e != nil {
		return fmt.Errorf("%w: %v", store.ErrInvalid, e)
	}
	out, e := s.DB.AddDocument(r.Context(), a, in, b, text)
	if e != nil {
		return e
	}
	return respond(w, 201, out)
}
