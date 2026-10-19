// Package store persists immutable clinical evidence, versioned rules and assessments.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"renalsafe/core/internal/domain"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("stale or conflicting state")
	ErrInvalid   = errors.New("invalid input")
	ErrForbidden = errors.New("forbidden")
)

type Actor struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}
type Document struct {
	ID        string         `json:"id"`
	Context   domain.Context `json:"context"`
	Source    domain.Source  `json:"source"`
	Status    string         `json:"status"`
	PageStart int            `json:"page_start"`
	PageEnd   int            `json:"page_end"`
	CreatedAt time.Time      `json:"created_at"`
}
type DocumentInput struct {
	Context   domain.Context `json:"context"`
	URL       string         `json:"url"`
	Version   string         `json:"version"`
	PageStart int            `json:"page_start"`
	PageEnd   int            `json:"page_end"`
}
type Chunk struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	Section    string `json:"section"`
	PageStart  int    `json:"page_start"`
	PageEnd    int    `json:"page_end"`
	Start      int    `json:"start_byte"`
	End        int    `json:"end_byte"`
	Text       string `json:"text"`
	IsTable    bool   `json:"is_table"`
}
type StoredRule struct {
	Rule       domain.Rule `json:"rule"`
	DocumentID string      `json:"document_id"`
	ChunkID    string      `json:"chunk_id"`
	Digest     string      `json:"digest"`
}
type CandidateInput struct {
	Rule    domain.Rule `json:"rule"`
	ChunkID string      `json:"chunk_id"`
}
type ReviewInput struct {
	ExpectedDigest string `json:"expected_digest"`
	Confirmed      bool   `json:"clinical_review_confirmed"`
	Note           string `json:"note"`
}
type AssessmentRecord struct {
	ID         string            `json:"id"`
	Actor      Actor             `json:"actor"`
	CreatedAt  time.Time         `json:"created_at"`
	Assessment domain.Assessment `json:"assessment"`
	Knowledge  domain.Knowledge  `json:"knowledge"`
	AuditHash  string            `json:"audit_hash"`
}
type AuditEvent struct {
	Sequence int64           `json:"sequence"`
	Previous string          `json:"previous_hash"`
	At       time.Time       `json:"at"`
	Actor    Actor           `json:"actor"`
	Kind     string          `json:"kind"`
	EntityID string          `json:"entity_id"`
	Data     json.RawMessage `json:"data"`
	Hash     string          `json:"hash"`
}

func (e AuditEvent) Digest() string {
	// JSONB reorders object keys and normalizes numeric syntax. Hash the same
	// canonical semantic JSON before insertion and after reading from PostgreSQL.
	var data any
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return ""
	}
	e.Data = marshal(data)
	e.Hash = ""
	return domain.Digest(e)
}
func ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
