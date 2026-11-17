package httpapi

import (
	"fmt"
	"net/http"
	"renalsafe/core/internal/extraction"
	"renalsafe/core/internal/store"
)

type ExtractionStatus struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
}

func (s *Server) extractionRoutes(api func(string, string, func(http.ResponseWriter, *http.Request, store.Actor) error)) {
	roles := "pharmacist admin"
	api("POST /api/v1/extractions/{id}/dismiss", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, err := s.DB.DismissExtraction(r.Context(), a, r.PathValue("id"))
		if err != nil {
			return err
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/extraction/status", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out := ExtractionStatus{Configured: s.Extractor != nil}
		if s.Extractor != nil {
			out.Model = s.Extractor.Model()
			out.Provider = extraction.ProviderName(s.Extractor)
		}
		return respond(w, 200, out)
	})
	api("GET /api/v1/extractions", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, err := s.DB.Extractions(r.Context(), a)
		if err != nil {
			return err
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/extractions", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.ExtractionInput
		if err := readJSON(w, r, &in); err != nil {
			return err
		}
		if s.Extractor == nil {
			return fmt.Errorf("online extraction is not configured")
		}
		out, err := s.DB.CreateExtractionForProvider(r.Context(), a, in, s.Extractor)
		if err != nil {
			return err
		}
		return respond(w, 202, out)
	})
	api("GET /api/v1/extractions/{id}", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, err := s.DB.Extraction(r.Context(), a, r.PathValue("id"))
		if err != nil {
			return err
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/extractions/{id}/cancel", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		out, err := s.DB.CancelExtraction(r.Context(), a, r.PathValue("id"))
		if err != nil {
			return err
		}
		return respond(w, 200, out)
	})
	api("POST /api/v1/extractions/{id}/candidate", roles, func(w http.ResponseWriter, r *http.Request, a store.Actor) error {
		var in store.ExtractionSave
		if err := readJSON(w, r, &in); err != nil {
			return err
		}
		out, err := s.DB.SaveExtraction(r.Context(), a, r.PathValue("id"), in)
		if err != nil {
			return err
		}
		return respond(w, 201, out)
	})
}
