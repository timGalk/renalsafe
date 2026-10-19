package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/evidence"
	"sort"
	"strconv"
	"strings"
)

type SearchInput struct {
	Context    domain.Context `json:"context"`
	Query      string         `json:"query"`
	DocumentID string         `json:"document_id,omitempty"`
	Section    string         `json:"section,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Mode       string         `json:"mode,omitempty"`
}
type Hit struct {
	Chunk       Chunk         `json:"chunk"`
	Source      domain.Source `json:"source"`
	Score       float64       `json:"score"`
	LexicalRank int           `json:"lexical_rank,omitempty"`
	DenseRank   int           `json:"dense_rank,omitempty"`
}
type EvidenceResult struct {
	Status            string `json:"status"`
	Mode              string `json:"mode"`
	DegradedReason    string `json:"degraded_reason,omitempty"`
	KnowledgeRevision int64  `json:"knowledge_revision"`
	EmbeddingModel    string `json:"embedding_model,omitempty"`
	Hits              []Hit  `json:"hits"`
	Answer            string `json:"answer"`
	AuditHash         string `json:"audit_hash"`
}

func vectorLiteral(v []float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.FormatFloat(x, 'g', -1, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
func ranked(ctx context.Context, q querier, in SearchInput, v []float64, model string) ([]Hit, error) {
	base := `SELECT c.id,c.document_id,c.section,c.page_start,c.page_end,c.start_byte,c.end_byte,c.content,c.is_table,d.source FROM chunks c JOIN documents d ON d.id=c.document_id WHERE d.status='ACTIVE' AND d.context=$1::jsonb AND ($2='' OR d.id=$2) AND ($3='' AND c.section IN ('4.2','4.3','4.4','5.2') OR c.section=$3)`
	args := []any{marshal(in.Context), in.DocumentID, in.Section}
	if v == nil {
		base += ` AND c.search_text @@ websearch_to_tsquery('simple',$4) ORDER BY ts_rank_cd(c.search_text,websearch_to_tsquery('simple',$4)) DESC,c.id LIMIT 50`
		args = append(args, in.Query)
	} else {
		base += ` AND c.embedding IS NOT NULL AND c.embedding_model=$4 ORDER BY c.embedding <=> $5::vector,c.id LIMIT 50`
		args = append(args, model, vectorLiteral(v))
	}
	rows, e := q.Query(ctx, base, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Hit{}
	for rows.Next() {
		var h Hit
		var raw []byte
		if e = rows.Scan(&h.Chunk.ID, &h.Chunk.DocumentID, &h.Chunk.Section, &h.Chunk.PageStart, &h.Chunk.PageEnd, &h.Chunk.Start, &h.Chunk.End, &h.Chunk.Text, &h.Chunk.IsTable, &raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &h.Source); e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (s *Store) Search(ctx context.Context, actor Actor, in SearchInput, embedder evidence.Embedder) (EvidenceResult, error) {
	out := EvidenceResult{Status: "NO_EVIDENCE", Mode: "lexical", Hits: []Hit{}, Answer: "No source evidence found. Manual review required; no personal dosing recommendation is generated."}
	if !in.Context.Valid() || len(strings.TrimSpace(in.Query)) < 2 || len(in.Query) > 500 || in.Limit < 0 || in.Limit > 20 {
		return out, fmt.Errorf("%w: exact context, query and limit 1-20 required", ErrInvalid)
	}
	if in.Section != "" && in.Section != "4.1" && in.Section != "4.2" && in.Section != "4.3" && in.Section != "4.4" && in.Section != "4.5" && in.Section != "4.6" && in.Section != "4.7" && in.Section != "4.8" && in.Section != "4.9" && in.Section != "5.1" && in.Section != "5.2" {
		return out, fmt.Errorf("%w: unsupported SmPC section", ErrInvalid)
	}
	if in.Mode != "" && in.Mode != "hybrid" && in.Mode != "lexical" {
		return out, fmt.Errorf("%w: unknown retrieval mode", ErrInvalid)
	}
	if in.Limit == 0 {
		in.Limit = 5
	}
	var v []float64
	model := ""
	if in.Mode != "lexical" {
		if embedder == nil {
			out.DegradedReason = "LOCAL_EMBEDDER_NOT_CONFIGURED"
		} else {
			vecs, e := embedder.Embed(ctx, []string{in.Query})
			if e != nil || len(vecs) != 1 || !evidence.ValidVector(vecs[0]) {
				out.DegradedReason = "LOCAL_EMBEDDER_UNAVAILABLE_OR_INVALID"
			} else {
				v = vecs[0]
				model = embedder.Model()
				out.EmbeddingModel = model
			}
		}
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		n, e := revision(ctx, tx)
		if e != nil {
			return e
		}
		out.KnowledgeRevision = n
		lex, e := ranked(ctx, tx, in, nil, "")
		if e != nil {
			return e
		}
		dense := []Hit{}
		if v != nil {
			dense, e = ranked(ctx, tx, in, v, model)
			if e != nil {
				return e
			}
			if len(dense) > 0 {
				out.Mode = "hybrid"
			} else {
				out.DegradedReason = "NO_MATCHING_MODEL_EMBEDDINGS"
			}
		}
		combined := map[string]Hit{}
		for i, h := range lex {
			h.LexicalRank = i + 1
			h.Score = 1 / (60 + float64(i+1))
			combined[h.Chunk.ID] = h
		}
		for i, h := range dense {
			prior, ok := combined[h.Chunk.ID]
			if !ok {
				prior = h
			}
			prior.DenseRank = i + 1
			prior.Score += 1 / (60 + float64(i+1))
			combined[h.Chunk.ID] = prior
		}
		for _, h := range combined {
			out.Hits = append(out.Hits, h)
		}
		sort.Slice(out.Hits, func(i, j int) bool {
			if out.Hits[i].Score == out.Hits[j].Score {
				return out.Hits[i].Chunk.ID < out.Hits[j].Chunk.ID
			}
			return out.Hits[i].Score > out.Hits[j].Score
		})
		if len(out.Hits) > in.Limit {
			out.Hits = out.Hits[:in.Limit]
		}
		if len(out.Hits) > 0 {
			out.Status = "EVIDENCE_ONLY"
			out.Answer = "Source passages retrieved for manual review. These passages are not an approved patient-specific dosing recommendation."
		}
		event, e := appendAudit(ctx, tx, actor, "EVIDENCE_SEARCH", "", map[string]any{"request": in, "response": out})
		out.AuditHash = event.Hash
		return e
	})
	return out, err
}
func (s *Store) IndexEmbeddings(ctx context.Context, actor Actor, id string, embedder evidence.Embedder) (int, error) {
	if actor.Role != "pharmacist" && actor.Role != "admin" {
		return 0, ErrForbidden
	}
	if embedder == nil {
		return 0, fmt.Errorf("%w: local embedder not configured", ErrInvalid)
	}
	cs, e := s.Chunks(ctx, id)
	if e != nil {
		return 0, e
	}
	vectors := map[string][]float64{}
	for _, c := range cs {
		if c.Section != "4.2" && c.Section != "4.3" && c.Section != "4.4" && c.Section != "5.2" {
			continue
		}
		vs, e := embedder.Embed(ctx, []string{c.Text})
		if e != nil {
			return 0, e
		}
		if len(vs) != 1 || !evidence.ValidVector(vs[0]) {
			return 0, fmt.Errorf("invalid embeddings")
		}
		vectors[c.ID] = vs[0]
	}
	if len(vectors) == 0 {
		return 0, fmt.Errorf("%w: no renal sections to index", ErrInvalid)
	}
	e = s.write(ctx, func(tx pgx.Tx) error {
		d, e := document(ctx, tx, id)
		if e != nil {
			return e
		}
		if d.Status != "ACTIVE" {
			return ErrConflict
		}
		for cid, v := range vectors {
			if _, e = tx.Exec(ctx, `UPDATE chunks SET embedding=$2::vector,embedding_model=$3 WHERE id=$1`, cid, vectorLiteral(v), embedder.Model()); e != nil {
				return e
			}
		}
		if e = bump(ctx, tx); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, actor, "EMBEDDINGS_INDEXED", id, map[string]any{"model": embedder.Model(), "chunks": len(vectors)})
		return e
	})
	return len(vectors), e
}
