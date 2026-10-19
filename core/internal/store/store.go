package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"renalsafe/core/internal/decision"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

const writeLock int64 = 726560019

type Store struct {
	Pool           *pgxpool.Pool
	AllowSynthetic bool
}
type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func Open(ctx context.Context, dsn string, allowSynthetic bool) (*Store, error) {
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		return nil, e
	}
	cfg.MaxConns = 8
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, e
	}
	s := &Store{Pool: pool, AllowSynthetic: allowSynthetic}
	if e = pool.Ping(ctx); e != nil {
		pool.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() { s.Pool.Close() }
func (s *Store) Migrate(ctx context.Context) error {
	return s.write(ctx, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY,digest text NOT NULL)`); e != nil {
			return e
		}
		files, e := migrations.ReadDir("migrations")
		if e != nil {
			return e
		}
		for _, f := range files {
			b, e := migrations.ReadFile("migrations/" + f.Name())
			if e != nil {
				return e
			}
			var digest string
			e = tx.QueryRow(ctx, `SELECT digest FROM schema_migrations WHERE name=$1`, f.Name()).Scan(&digest)
			if e == nil {
				if digest != domain.BytesDigest(b) {
					return fmt.Errorf("migration changed: %s", f.Name())
				}
				continue
			}
			if !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if _, e = tx.Exec(ctx, string(b)); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES($1,$2)`, f.Name(), domain.BytesDigest(b)); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *Store) write(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, e := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, writeLock); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) && (pg.Code == "23P01" || pg.Code == "23505") {
			return fmt.Errorf("%w: database constraint", ErrConflict)
		}
		return e
	}
	return tx.Commit(ctx)
}
func marshal(v any) []byte { b, _ := json.Marshal(v); return b }
func normalize(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return e
}
func revision(ctx context.Context, q querier) (int64, error) {
	var n int64
	e := q.QueryRow(ctx, `SELECT revision FROM knowledge_revision WHERE singleton`).Scan(&n)
	return n, e
}
func bump(ctx context.Context, q querier) error {
	_, e := q.Exec(ctx, `UPDATE knowledge_revision SET revision=revision+1 WHERE singleton`)
	return e
}
func snapshot(ctx context.Context, q querier, allow bool) (rules.Snapshot, error) {
	n, e := revision(ctx, q)
	if e != nil {
		return rules.Snapshot{}, e
	}
	k := domain.Knowledge{Version: fmt.Sprintf("db-r%d", n), Sources: []domain.Source{}, Rules: []domain.Rule{}}
	files := map[string][]byte{}
	rows, e := q.Query(ctx, `SELECT source,pdf,text_content,synthetic FROM documents WHERE status='ACTIVE' ORDER BY id`)
	if e != nil {
		return rules.Snapshot{}, e
	}
	for rows.Next() {
		var raw, pdf []byte
		var text string
		var synthetic bool
		if e = rows.Scan(&raw, &pdf, &text, &synthetic); e != nil {
			rows.Close()
			return rules.Snapshot{}, e
		}
		var source domain.Source
		if e = json.Unmarshal(raw, &source); e != nil {
			rows.Close()
			return rules.Snapshot{}, e
		}
		k.Sources = append(k.Sources, source)
		k.Synthetic = k.Synthetic || synthetic
		files[source.DocumentFile] = pdf
		files[source.TextFile] = []byte(text)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return rules.Snapshot{}, e
	}
	rows, e = q.Query(ctx, `SELECT r.payload FROM renal_rules r JOIN documents d ON r.document_id=d.id WHERE d.status='ACTIVE' ORDER BY r.id`)
	if e != nil {
		return rules.Snapshot{}, e
	}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return rules.Snapshot{}, e
		}
		var r domain.Rule
		if e = json.Unmarshal(b, &r); e != nil {
			rows.Close()
			return rules.Snapshot{}, e
		}
		k.Rules = append(k.Rules, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return rules.Snapshot{}, e
	}
	return rules.LoadWithFiles(marshal(k), files, allow)
}
func appendAudit(ctx context.Context, q querier, actor Actor, kind, id string, data any) (AuditEvent, error) {
	e := AuditEvent{Sequence: 1, At: time.Now().UTC(), Actor: actor, Kind: kind, EntityID: id, Data: marshal(data)}
	var last []byte
	var sequence int64
	var hash, previous string
	err := q.QueryRow(ctx, `SELECT sequence,hash,previous_hash,payload FROM audit_events ORDER BY sequence DESC LIMIT 1`).Scan(&sequence, &hash, &previous, &last)
	if err == nil {
		var prior AuditEvent
		if err = json.Unmarshal(last, &prior); err != nil {
			return AuditEvent{}, err
		}
		if prior.Digest() != prior.Hash || prior.Sequence != sequence || prior.Hash != hash || prior.Previous != previous {
			return AuditEvent{}, fmt.Errorf("audit tail corrupt")
		}
		e.Sequence = prior.Sequence + 1
		e.Previous = prior.Hash
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AuditEvent{}, err
	}
	e.Hash = e.Digest()
	_, err = q.Exec(ctx, `INSERT INTO audit_events(sequence,hash,previous_hash,payload) VALUES($1,$2,$3,$4)`, e.Sequence, e.Hash, e.Previous, marshal(e))
	return e, err
}
func (s *Store) Assess(ctx context.Context, actor Actor, req domain.Request) (AssessmentRecord, error) {
	var out AssessmentRecord
	if domain.ValidatePrescriptionInputs(req) != nil {
		return out, fmt.Errorf("%w: invalid prescription", ErrInvalid)
	}
	if req.At.IsZero() || len(req.Medications) < 1 || len(req.Medications) > 100 {
		return out, fmt.Errorf("%w: timestamp and 1-100 medications required", ErrInvalid)
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		kb, e := snapshot(ctx, tx, s.AllowSynthetic)
		if e != nil {
			return e
		}
		out = AssessmentRecord{ID: ID(), Actor: actor, CreatedAt: time.Now().UTC(), Assessment: decision.Assess(req, kb), Knowledge: kb.Knowledge()}
		// Audit excludes the final self-referential audit hash; assessment payload pins it.
		event, e := appendAudit(ctx, tx, actor, "ASSESSMENT", out.ID, out)
		if e != nil {
			return e
		}
		out.AuditHash = event.Hash
		_, e = tx.Exec(ctx, `INSERT INTO assessments(id,actor_id,payload) VALUES($1,$2,$3)`, out.ID, actor.ID, marshal(out))
		return e
	})
	return out, err
}
func (s *Store) Assessment(ctx context.Context, actor Actor, id string) (AssessmentRecord, error) {
	var r AssessmentRecord
	var b []byte
	e := s.Pool.QueryRow(ctx, `SELECT payload FROM assessments WHERE id=$1`, id).Scan(&b)
	if e != nil {
		return r, normalize(e)
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if actor.Role != "admin" && r.Actor.ID != actor.ID {
		return AssessmentRecord{}, ErrForbidden
	}
	return r, nil
}
func (s *Store) Replay(ctx context.Context, actor Actor, id string) (bool, error) {
	record, e := s.Assessment(ctx, actor, id)
	if e != nil {
		return false, e
	}
	files := map[string][]byte{}
	for _, source := range record.Knowledge.Sources {
		var pdf []byte
		var text string
		if e = s.Pool.QueryRow(ctx, `SELECT pdf,text_content FROM documents WHERE id=$1`, source.ID).Scan(&pdf, &text); e != nil {
			return false, normalize(e)
		}
		files[source.DocumentFile] = pdf
		files[source.TextFile] = []byte(text)
	}
	kb, e := rules.LoadWithFiles(marshal(record.Knowledge), files, s.AllowSynthetic)
	if e != nil {
		return false, e
	}
	if record.Assessment.EngineVersion != decision.Version {
		return false, fmt.Errorf("%w: original engine version unavailable", ErrConflict)
	}
	actual := decision.Assess(record.Assessment.Request, kb)
	return domain.Digest(actual) == domain.Digest(record.Assessment), nil
}
func (s *Store) Audit(ctx context.Context) ([]AuditEvent, error) {
	rows, e := s.Pool.Query(ctx, `SELECT sequence,hash,previous_hash,payload FROM audit_events ORDER BY sequence`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []AuditEvent{}
	prev := ""
	for rows.Next() {
		var raw []byte
		var sequence int64
		var hash, previous string
		if e = rows.Scan(&sequence, &hash, &previous, &raw); e != nil {
			return nil, e
		}
		var event AuditEvent
		if e = json.Unmarshal(raw, &event); e != nil {
			return nil, e
		}
		if event.Sequence != sequence || event.Hash != hash || event.Previous != previous || event.Sequence != int64(len(out)+1) || event.Previous != prev || event.Hash != event.Digest() {
			return nil, fmt.Errorf("audit chain corrupt")
		}
		out = append(out, event)
		prev = event.Hash
	}
	return out, rows.Err()
}
