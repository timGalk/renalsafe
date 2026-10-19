package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"path/filepath"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/evidence"
	"renalsafe/core/internal/rules"
	"strings"
	"time"
)

func (s *Store) Documents(ctx context.Context) ([]Document, error) {
	rows, e := s.Pool.Query(ctx, `SELECT id,context,source,status,page_start,page_end,created_at FROM documents ORDER BY created_at DESC,id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, e := scanDocument(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanDocument(row scanner) (Document, error) {
	var d Document
	var c, b []byte
	e := row.Scan(&d.ID, &c, &b, &d.Status, &d.PageStart, &d.PageEnd, &d.CreatedAt)
	if e != nil {
		return d, normalize(e)
	}
	if e = json.Unmarshal(c, &d.Context); e != nil {
		return d, e
	}
	e = json.Unmarshal(b, &d.Source)
	return d, e
}
func document(ctx context.Context, q querier, id string) (Document, error) {
	return scanDocument(q.QueryRow(ctx, `SELECT id,context,source,status,page_start,page_end,created_at FROM documents WHERE id=$1`, id))
}
func chunks(ctx context.Context, q querier, id string) ([]Chunk, error) {
	rows, e := q.Query(ctx, `SELECT id,document_id,section,page_start,page_end,start_byte,end_byte,content,is_table FROM chunks WHERE document_id=$1 ORDER BY start_byte`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Chunk{}
	for rows.Next() {
		var c Chunk
		if e = rows.Scan(&c.ID, &c.DocumentID, &c.Section, &c.PageStart, &c.PageEnd, &c.Start, &c.End, &c.Text, &c.IsTable); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) Chunks(ctx context.Context, id string) ([]Chunk, error) {
	if _, e := document(ctx, s.Pool, id); e != nil {
		return nil, e
	}
	return chunks(ctx, s.Pool, id)
}
func (s *Store) DocumentPDF(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	e := s.Pool.QueryRow(ctx, `SELECT pdf FROM documents WHERE id=$1`, id).Scan(&b)
	return b, normalize(e)
}
func insertDocument(ctx context.Context, tx pgx.Tx, d Document, pdf []byte, text string, sections []evidence.Section) error {
	if _, e := tx.Exec(ctx, `INSERT INTO documents(id,context,source,status,page_start,page_end,pdf,text_content,synthetic,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, d.ID, marshal(d.Context), marshal(d.Source), d.Status, d.PageStart, d.PageEnd, pdf, text, d.Source.Synthetic, d.CreatedAt); e != nil {
		return e
	}
	for _, sec := range sections {
		if _, e := tx.Exec(ctx, `INSERT INTO chunks(id,document_id,section,page_start,page_end,start_byte,end_byte,content,is_table) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, ID(), d.ID, sec.Name, sec.PageStart, sec.PageEnd, sec.Start, sec.End, sec.Text, sec.Table); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) AddDocument(ctx context.Context, actor Actor, in DocumentInput, pdf []byte, text string) (Document, error) {
	var out Document
	u, e := url.Parse(in.URL)
	if e != nil || u.Scheme != "https" || u.Host == "" || !in.Context.Valid() || strings.TrimSpace(in.Version) == "" || !strings.HasPrefix(string(pdf), "%PDF-") {
		return out, fmt.Errorf("%w: official source URL, version, exact context and PDF required", ErrInvalid)
	}
	secs, e := evidence.Split(text, in.PageStart, in.PageEnd)
	if e != nil {
		return out, fmt.Errorf("%w: %v", ErrInvalid, e)
	}
	out = Document{ID: ID(), Context: in.Context, Status: "ACTIVE", PageStart: in.PageStart, PageEnd: in.PageEnd, CreatedAt: time.Now().UTC()}
	out.Source = domain.Source{ID: out.ID, URL: in.URL, Version: in.Version, RetrievedAt: out.CreatedAt, DocumentFile: "documents/" + out.ID + ".pdf", DocumentSHA256: domain.BytesDigest(pdf), TextFile: "documents/" + out.ID + ".txt", TextSHA256: domain.BytesDigest([]byte(text)), Synthetic: s.AllowSynthetic}
	err := s.write(ctx, func(tx pgx.Tx) error {
		if e := insertDocument(ctx, tx, out, pdf, text, secs); e != nil {
			return e
		}
		if e := bump(ctx, tx); e != nil {
			return e
		}
		_, e := appendAudit(ctx, tx, actor, "DOCUMENT_IMPORTED", out.ID, out)
		return e
	})
	return out, err
}
func (s *Store) Supersede(ctx context.Context, actor Actor, oldID, replacementID string) error {
	return s.write(ctx, func(tx pgx.Tx) error {
		old, e := document(ctx, tx, oldID)
		if e != nil {
			return e
		}
		replacement, e := document(ctx, tx, replacementID)
		if e != nil {
			return e
		}
		if oldID == replacementID || old.Status != "ACTIVE" || replacement.Status != "ACTIVE" || old.Context != replacement.Context {
			return ErrConflict
		}
		if _, e = tx.Exec(ctx, `UPDATE documents SET status='SUPERSEDED' WHERE id=$1`, oldID); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE renal_rules SET status='SUPERSEDED',payload=jsonb_set(payload,'{status}','"SUPERSEDED"') WHERE document_id=$1 AND status IN ('PENDING','APPROVED')`, oldID); e != nil {
			return e
		}
		if e = bump(ctx, tx); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, actor, "DOCUMENT_SUPERSEDED", oldID, map[string]string{"replacement_id": replacementID})
		return e
	})
}

// Seed validates an existing offline bundle and imports its pending records only.
// Existing documents are never overwritten and startup is idempotent.
func (s *Store) Seed(ctx context.Context, path string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	snap, e := rules.Load(b, filepath.Dir(path), s.AllowSynthetic)
	if e != nil {
		return e
	}
	k := snap.Knowledge()
	for _, r := range k.Rules {
		if r.Status != "PENDING" {
			return fmt.Errorf("seed must contain PENDING rules only")
		}
	}
	return s.write(ctx, func(tx pgx.Tx) error {
		for _, src := range k.Sources {
			var exists bool
			if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM documents WHERE id=$1)`, src.ID).Scan(&exists); e != nil {
				return e
			}
			if exists {
				continue
			}
			var scope domain.Context
			for _, r := range k.Rules {
				if r.Citation.SourceID == src.ID {
					scope = r.Context
					break
				}
			}
			if !scope.Valid() {
				return fmt.Errorf("seed requires exact product context")
			}
			pdf, e := os.ReadFile(filepath.Join(filepath.Dir(path), src.DocumentFile))
			if e != nil {
				return e
			}
			raw, e := os.ReadFile(filepath.Join(filepath.Dir(path), src.TextFile))
			if e != nil {
				return e
			}
			text := string(raw)
			first, last, e := evidence.PrimaryProductScope(text)
			if e != nil {
				return e
			}
			sections, e := evidence.Split(text, first, last)
			if e != nil {
				return e
			}
			d := Document{ID: src.ID, Context: scope, Source: src, Status: "ACTIVE", PageStart: first, PageEnd: last, CreatedAt: time.Now().UTC()}
			if e = insertDocument(ctx, tx, d, pdf, text, sections); e != nil {
				return e
			}
			cs, e := chunks(ctx, tx, src.ID)
			if e != nil {
				return e
			}
			for _, r := range k.Rules {
				if r.Citation.SourceID != src.ID {
					continue
				}
				found := ""
				for _, c := range cs {
					idx := strings.Index(c.Text, r.Citation.Quote)
					if c.Section == r.Citation.Section && idx >= 0 && c.PageStart+strings.Count(c.Text[:idx], "\f") == r.Citation.Page {
						found = c.ID
						break
					}
				}
				if found == "" {
					return fmt.Errorf("seed quote outside declared product scope")
				}
				if e = insertRule(ctx, tx, r, src.ID, found); e != nil {
					return e
				}
			}
			if e = bump(ctx, tx); e != nil {
				return e
			}
			if _, e = appendAudit(ctx, tx, Actor{ID: "seed-importer", Role: "system"}, "PENDING_SEED_IMPORTED", src.ID, d); e != nil {
				return e
			}
		}
		return nil
	})
}
