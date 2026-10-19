package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"strconv"
	"strings"
	"time"
)

func rangeLiteral(r domain.Interval) string {
	left, right := "(", ")"
	if r.LowerInclusive {
		left = "["
	}
	if r.UpperInclusive {
		right = "]"
	}
	upper := ""
	if r.Upper != nil {
		upper = strconv.FormatFloat(*r.Upper, 'g', -1, 64)
	}
	return left + strconv.FormatFloat(r.Lower, 'g', -1, 64) + "," + upper + right
}
func insertRule(ctx context.Context, tx pgx.Tx, r domain.Rule, docID, chunkID string) error {
	_, e := tx.Exec(ctx, `INSERT INTO renal_rules(id,document_id,chunk_id,product,route,form,indication,metric,kidney_range,status,payload,digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::numrange,$10,$11,$12)`, r.ID, docID, chunkID, r.Context.Product, r.Context.Route, r.Context.Form, r.Context.Indication, r.Metric, rangeLiteral(r.Range), r.Status, marshal(r), r.ContentDigest())
	return e
}
func storedRule(ctx context.Context, q querier, id string) (StoredRule, error) {
	var out StoredRule
	var b []byte
	e := q.QueryRow(ctx, `SELECT payload,document_id,chunk_id,digest FROM renal_rules WHERE id=$1`, id).Scan(&b, &out.DocumentID, &out.ChunkID, &out.Digest)
	if e != nil {
		return out, normalize(e)
	}
	e = json.Unmarshal(b, &out.Rule)
	return out, e
}
func (s *Store) Rules(ctx context.Context, status string) ([]StoredRule, error) {
	rows, e := s.Pool.Query(ctx, `SELECT payload,document_id,chunk_id,digest FROM renal_rules WHERE ($1='' OR status=$1) ORDER BY id`, status)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []StoredRule{}
	for rows.Next() {
		var r StoredRule
		var b []byte
		if e = rows.Scan(&b, &r.DocumentID, &r.ChunkID, &r.Digest); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &r.Rule); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func validateRule(ctx context.Context, tx pgx.Tx, r domain.Rule, allow bool) error {
	kb, e := snapshot(ctx, tx, allow)
	if e != nil {
		return e
	}
	k := kb.Knowledge()
	found := false
	for i := range k.Rules {
		if k.Rules[i].ID == r.ID {
			k.Rules[i] = r
			found = true
		}
	}
	if !found {
		k.Rules = append(k.Rules, r)
	}
	files := map[string][]byte{}
	for _, src := range k.Sources {
		var pdf []byte
		var text string
		if e = tx.QueryRow(ctx, `SELECT pdf,text_content FROM documents WHERE id=$1`, src.ID).Scan(&pdf, &text); e != nil {
			return e
		}
		files[src.DocumentFile] = pdf
		files[src.TextFile] = []byte(text)
	}
	if _, e = rules.LoadWithFiles(marshal(k), files, allow); e != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, e)
	}
	return nil
}
func (s *Store) AddCandidate(ctx context.Context, actor Actor, in CandidateInput) (StoredRule, error) {
	var out StoredRule
	if actor.Role != "pharmacist" && actor.Role != "admin" {
		return out, ErrForbidden
	}
	if in.Rule.Status != "PENDING" || in.Rule.Approval != nil {
		return out, fmt.Errorf("%w: candidates must be PENDING without approval", ErrInvalid)
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		var c Chunk
		if e := tx.QueryRow(ctx, `SELECT document_id,section,page_start,content FROM chunks WHERE id=$1`, in.ChunkID).Scan(&c.DocumentID, &c.Section, &c.PageStart, &c.Text); e != nil {
			return normalize(e)
		}
		doc, e := document(ctx, tx, c.DocumentID)
		if e != nil {
			return e
		}
		if doc.Status != "ACTIVE" {
			return ErrConflict
		}
		r := in.Rule
		if r.Context != doc.Context {
			return fmt.Errorf("%w: exact product context required", ErrInvalid)
		}
		idx := strings.Index(c.Text, r.Citation.Quote)
		if r.Citation.Quote == "" || idx < 0 || r.Citation.Section != c.Section {
			return fmt.Errorf("%w: quote must occur in selected section", ErrInvalid)
		}
		page := c.PageStart + strings.Count(c.Text[:idx], "\f")
		if r.Citation.Page != 0 && r.Citation.Page != page {
			return fmt.Errorf("%w: wrong physical page", ErrInvalid)
		}
		r.ID = ID()
		r.Version = "1"
		r.Citation = domain.Citation{SourceID: doc.ID, SourceVersion: doc.Source.Version, DocumentSHA256: doc.Source.DocumentSHA256, Section: c.Section, Page: page, Quote: r.Citation.Quote}
		if e = validateRule(ctx, tx, r, s.AllowSynthetic); e != nil {
			return e
		}
		if e = insertRule(ctx, tx, r, doc.ID, in.ChunkID); e != nil {
			return e
		}
		if e = bump(ctx, tx); e != nil {
			return e
		}
		out = StoredRule{Rule: r, DocumentID: doc.ID, ChunkID: in.ChunkID, Digest: r.ContentDigest()}
		_, e = appendAudit(ctx, tx, actor, "CANDIDATE_CREATED", r.ID, out)
		return e
	})
	return out, err
}
func (s *Store) Review(ctx context.Context, actor Actor, id, action string, in ReviewInput) (StoredRule, error) {
	var out StoredRule
	if actor.Role != "pharmacist" {
		return out, ErrForbidden
	}
	if (action != "approve" && action != "reject") || in.ExpectedDigest == "" || strings.TrimSpace(in.Note) == "" || len(in.Note) > 4000 {
		return out, fmt.Errorf("%w: digest and review note required", ErrInvalid)
	}
	if action == "approve" && !in.Confirmed {
		return out, fmt.Errorf("%w: explicit human clinical confirmation required", ErrInvalid)
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		r, e := storedRule(ctx, tx, id)
		if e != nil {
			return e
		}
		d, e := document(ctx, tx, r.DocumentID)
		if e != nil {
			return e
		}
		if r.Rule.Status != "PENDING" || d.Status != "ACTIVE" || in.ExpectedDigest != r.Digest {
			return ErrConflict
		}
		if action == "approve" {
			r.Rule.Status = "APPROVED"
			r.Rule.Approval = &domain.Approval{Reviewer: actor.ID, Role: actor.Role, At: time.Now().UTC(), ContentDigest: r.Rule.ContentDigest()}
		} else {
			r.Rule.Status = "REJECTED"
			r.Rule.Approval = nil
		}
		if e = validateRule(ctx, tx, r.Rule, s.AllowSynthetic); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE renal_rules SET status=$2,payload=$3 WHERE id=$1`, id, r.Rule.Status, marshal(r.Rule)); e != nil {
			return e
		}
		if e = bump(ctx, tx); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, actor, "RULE_"+strings.ToUpper(action), id, map[string]any{"rule": r, "review_note": in.Note})
		out = r
		return e
	})
	return out, err
}

func (s *Store) EditCandidate(ctx context.Context, actor Actor, id, expected string, next domain.Rule) (StoredRule, error) {
	var out StoredRule
	if actor.Role != "pharmacist" && actor.Role != "admin" {
		return out, ErrForbidden
	}
	if expected == "" || next.Status != "PENDING" || next.Approval != nil {
		return out, ErrInvalid
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		old, e := storedRule(ctx, tx, id)
		if e != nil {
			return e
		}
		doc, e := document(ctx, tx, old.DocumentID)
		if e != nil {
			return e
		}
		if old.Rule.Status != "PENDING" || old.Digest != expected || doc.Status != "ACTIVE" {
			return ErrConflict
		}
		if next.Context != old.Rule.Context || next.Citation.SourceID != old.Rule.Citation.SourceID || next.Citation.SourceVersion != old.Rule.Citation.SourceVersion || next.Citation.DocumentSHA256 != old.Rule.Citation.DocumentSHA256 {
			return ErrInvalid
		}
		var text, section string
		var page int
		if e = tx.QueryRow(ctx, `SELECT content,section,page_start FROM chunks WHERE id=$1`, old.ChunkID).Scan(&text, &section, &page); e != nil {
			return e
		}
		idx := strings.Index(text, next.Citation.Quote)
		if next.Citation.Quote == "" || idx < 0 || next.Citation.Section != section || next.Citation.Page != page+strings.Count(text[:idx], "\f") {
			return ErrInvalid
		}
		next.ID = old.Rule.ID
		next.Version = old.Rule.Version
		if e = validateRule(ctx, tx, next, s.AllowSynthetic); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE renal_rules SET metric=$2,kidney_range=$3::numrange,payload=$4,digest=$5 WHERE id=$1`, id, next.Metric, rangeLiteral(next.Range), marshal(next), next.ContentDigest()); e != nil {
			return e
		}
		if e = bump(ctx, tx); e != nil {
			return e
		}
		out = StoredRule{Rule: next, DocumentID: old.DocumentID, ChunkID: old.ChunkID, Digest: next.ContentDigest()}
		_, e = appendAudit(ctx, tx, actor, "CANDIDATE_EDITED", id, out)
		return e
	})
	return out, err
}
