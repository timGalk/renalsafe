package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type AuditSummary struct {
	Sequence int64     `json:"sequence"`
	Hash     string    `json:"hash"`
	Previous string    `json:"previous_hash"`
	At       time.Time `json:"at"`
	Actor    Actor     `json:"actor"`
	Kind     string    `json:"kind"`
	EntityID string    `json:"entity_id"`
}
type AuditPage struct {
	Items           []AuditSummary `json:"items"`
	Next            string         `json:"next_cursor,omitempty"`
	PageHashesValid bool           `json:"page_hashes_valid"`
}
type AuditVerification struct {
	VerifiedThrough int64  `json:"verified_through"`
	HeadHash        string `json:"head_hash"`
	Valid           bool   `json:"valid"`
}

func (s *Store) AuditSummaries(ctx context.Context, a Actor, raw string, limit int) (AuditPage, error) {
	out := AuditPage{Items: []AuditSummary{}}
	if e := requireActive(ctx, s.Pool, a, "admin"); e != nil {
		return out, e
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return out, ErrInvalid
	}
	var before int64
	if raw != "" {
		var e error
		before, e = strconv.ParseInt(raw, 10, 64)
		if e != nil || before <= 0 {
			return out, ErrInvalid
		}
	}
	rows, e := s.Pool.Query(ctx, `SELECT sequence,hash,previous_hash,payload FROM audit_events WHERE ($1::bigint=0 OR sequence<$1) ORDER BY sequence DESC LIMIT $2`, before, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	var newerPrevious string
	var newerSequence int64
	for rows.Next() {
		var seq int64
		var hash, prev string
		var b []byte
		if e = rows.Scan(&seq, &hash, &prev, &b); e != nil {
			return out, e
		}
		var event AuditEvent
		if e = json.Unmarshal(b, &event); e != nil {
			return out, e
		}
		if event.Sequence != seq || event.Hash != hash || event.Previous != prev || event.Digest() != hash || newerSequence != 0 && (newerSequence != seq+1 || newerPrevious != hash) {
			return out, fmt.Errorf("audit page corrupt")
		}
		newerPrevious = prev
		newerSequence = seq
		out.Items = append(out.Items, AuditSummary{seq, hash, prev, event.At, event.Actor, event.Kind, event.EntityID})
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.Next = strconv.FormatInt(out.Items[limit-1].Sequence, 10)
	}
	out.PageHashesValid = true
	return out, nil
}
func (s *Store) VerifyAudit(ctx context.Context, a Actor) (AuditVerification, error) {
	out := AuditVerification{}
	if e := requireActive(ctx, s.Pool, a, "admin"); e != nil {
		return out, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT sequence,hash,previous_hash,payload FROM audit_events ORDER BY sequence`)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var seq int64
		var hash, prev string
		var b []byte
		if e = rows.Scan(&seq, &hash, &prev, &b); e != nil {
			return out, e
		}
		var event AuditEvent
		if e = json.Unmarshal(b, &event); e != nil {
			return out, e
		}
		if seq != out.VerifiedThrough+1 || prev != out.HeadHash || event.Sequence != seq || event.Hash != hash || event.Previous != prev || event.Digest() != hash {
			return out, fmt.Errorf("audit chain corrupt")
		}
		out.VerifiedThrough = seq
		out.HeadHash = hash
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	out.Valid = true
	return out, nil
}

type SystemStatus struct {
	Database             string `json:"database"`
	EngineVersion        string `json:"engine_version"`
	KnowledgeRevision    int64  `json:"knowledge_revision"`
	ActiveDocuments      int64  `json:"active_documents"`
	PendingRules         int64  `json:"pending_rules"`
	ApprovedRules        int64  `json:"approved_rules"`
	EmbeddingsConfigured bool   `json:"embeddings_configured"`
	EmbeddingModel       string `json:"embedding_model,omitempty"`
}

func (s *Store) SystemStatus(ctx context.Context, a Actor) (SystemStatus, error) {
	out := SystemStatus{Database: "ready"}
	if e := requireActive(ctx, s.Pool, a, "admin"); e != nil {
		return out, e
	}
	e := s.Pool.QueryRow(ctx, `SELECT revision,(SELECT count(*) FROM documents WHERE status='ACTIVE'),(SELECT count(*) FROM renal_rules WHERE status='PENDING'),(SELECT count(*) FROM renal_rules WHERE status='APPROVED') FROM knowledge_revision WHERE singleton`).Scan(&out.KnowledgeRevision, &out.ActiveDocuments, &out.PendingRules, &out.ApprovedRules)
	return out, e
}
