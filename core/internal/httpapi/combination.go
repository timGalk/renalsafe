package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/extraction"
	"renalsafe/core/internal/store"
	"strconv"
	"strings"
)

type CombinationReviewResponse struct {
	Review    extraction.CombinationReview `json:"review"`
	Uncovered []domain.Context             `json:"uncovered"`
	Limited   bool                         `json:"limited"`
}

func (s *Server) combinationRoutes(api func(string, string, func(http.ResponseWriter, *http.Request, store.Actor) error)) {
	const roles = "physician pharmacist admin"
	api("GET /api/v1/combination-review/status", roles, func(w http.ResponseWriter, r *http.Request, _ store.Actor) error {
		_, configured := s.Extractor.(extraction.CombinationReviewer)
		out := ExtractionStatus{Configured: configured && s.DB != nil}
		if out.Configured {
			out.Provider = extraction.ProviderName(s.Extractor)
			out.Model = s.Extractor.Model()
		}
		return respond(w, http.StatusOK, out)
	})
	api("POST /api/v1/assessments/{id}/combination-review", roles, s.combinationReview)
}

func (s *Server) combinationReview(w http.ResponseWriter, r *http.Request, actor store.Actor) error {
	var request struct{}
	if err := readJSON(w, r, &request); err != nil {
		return err
	}
	reviewer, configured := s.Extractor.(extraction.CombinationReviewer)
	if !configured || s.DB == nil {
		return inputError{http.StatusServiceUnavailable, "combination_review_unavailable"}
	}
	ctx := r.Context()
	id := r.PathValue("id")
	record, err := s.DB.Assessment(ctx, actor, id)
	if err != nil {
		return err
	}
	contexts := distinctContexts(record.Assessment.Request.Medications)
	if len(contexts) < 2 {
		return fmt.Errorf("%w: select at least two distinct medicine contexts for a combination review", store.ErrInvalid)
	}

	docs, err := s.DB.Documents(ctx)
	if err != nil {
		return err
	}
	out := CombinationReviewResponse{Uncovered: []domain.Context{}}
	passages := []extraction.CombinationPassage{}
	// Leave room for passage IDs and divide the source budget evenly so a long
	// document cannot crowd out the other medicines. Keep whole source excerpts.
	perContext := (extraction.MaxInputBytes - 4096) / len(contexts)
	for _, context := range contexts {
		start := len(passages)
		remaining := perContext
		for _, doc := range docs {
			if doc.Context != context || !publicCombinationDocument(doc) {
				continue
			}
			chunks, err := s.DB.Chunks(ctx, doc.ID)
			if err != nil {
				return err
			}
			for _, chunk := range chunks {
				switch chunk.Section {
				case "4.2", "4.3", "4.4", "4.5", "5.2":
				default:
					continue
				}
				passage := extraction.CombinationPassage{
					Context: doc.Context, Section: chunk.Section, Text: chunk.Text, Source: doc.Source,
					DocumentID: doc.ID, PageStart: chunk.PageStart, PageEnd: chunk.PageEnd,
				}
				for _, excerpt := range passage.Excerpts() {
					encoded, err := json.Marshal(excerpt)
					if err != nil {
						return err
					}
					size := len(encoded) + 32
					if size > remaining {
						out.Limited = true
						continue
					}
					remaining -= size
					passages = append(passages, excerpt)
				}
			}
		}
		if len(passages) == start {
			out.Uncovered = append(out.Uncovered, context)
		}
	}
	if len(passages) == 0 {
		return fmt.Errorf("%w: no public source passages are available for this combination", store.ErrInvalid)
	}
	out.Review, err = reviewer.ReviewCombination(ctx, passages)
	if err != nil {
		if errors.Is(err, extraction.ErrInvalidCombinationReview) {
			log.Print("RenalSafe combination review returned invalid source references")
			return inputError{http.StatusServiceUnavailable, "combination_review_invalid_references"}
		}
		log.Print("RenalSafe combination review failed: ", combinationFailure(err))
		return inputError{http.StatusServiceUnavailable, "combination_review_failed"}
	}
	// Access may have changed while the provider request was in progress.
	if _, err = s.DB.Assessment(ctx, actor, id); err != nil {
		return err
	}
	return respond(w, http.StatusOK, out)
}

func distinctContexts(medications []domain.Context) []domain.Context {
	contexts := []domain.Context{}
	seen := map[domain.Context]bool{}
	for _, context := range medications {
		if context.Valid() && !seen[context] {
			contexts = append(contexts, context)
			seen[context] = true
		}
	}
	return contexts
}

func publicCombinationDocument(doc store.Document) bool {
	source, err := url.Parse(doc.Source.URL)
	return doc.Status == "ACTIVE" && !doc.Source.Synthetic && err == nil && source.Scheme == "https" && source.Host != ""
}

func combinationFailure(err error) string {
	message := err.Error()
	if status, found := strings.CutPrefix(message, "provider HTTP "); found {
		if code, parseErr := strconv.Atoi(status); parseErr == nil && code >= 100 && code <= 599 {
			return fmt.Sprintf("provider HTTP %d", code)
		}
	}
	switch message {
	case "provider connection failed", "provider incomplete or invalid response", "provider refused request",
		"provider returned a different model", "provider did not return one text response",
		"provider response exceeds budget", "combination evidence is empty or exceeds budget":
		return message
	default:
		return "provider request did not complete"
	}
}
