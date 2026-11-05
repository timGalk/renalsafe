package httpapi

import (
	"net/http"
	"renalsafe/core/internal/decision"
	"renalsafe/core/internal/store"
)

func (s *Server) adminRoutes(api func(string, string, func(http.ResponseWriter, *http.Request, store.Actor) error)) {
	api("GET /api/v1/admin/status", "admin", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.SystemStatus(r.Context(), a)
		if e != nil {
			return e
		}
		out.EngineVersion = decision.Version
		out.EmbeddingsConfigured = s.Embedder != nil
		if s.Embedder != nil {
			out.EmbeddingModel = s.Embedder.Model()
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/admin/audit", "admin", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		cursor, n, e := pageQuery(r)
		if e != nil {
			return e
		}
		out, e := s.DB.AuditSummaries(r.Context(), a, cursor, n)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/admin/audit/verify", "admin", func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, e := s.DB.VerifyAudit(r.Context(), a)
		if e != nil {
			return e
		}
		return respond(w, 200, out)
	})
}
