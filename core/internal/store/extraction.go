package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/extraction"
	"strings"
	"time"
)

type ExtractionInput struct {
	DocumentID string `json:"document_id"`
	RequestKey string `json:"request_key"`
}
type ExtractionSave struct {
	FindingIndex   int            `json:"finding_index"`
	ExpectedDigest string         `json:"expected_digest"`
	Candidate      CandidateInput `json:"candidate"`
}
type ExtractionJob struct {
	ID               string                       `json:"id"`
	DocumentID       string                       `json:"document_id"`
	Actor            Actor                        `json:"actor"`
	Status           string                       `json:"status"`
	Model            string                       `json:"model"`
	CreatedAt        time.Time                    `json:"created_at"`
	StartedAt        *time.Time                   `json:"started_at"`
	FinishedAt       *time.Time                   `json:"finished_at"`
	Source           extraction.SourceInput       `json:"source"`
	SourceDigest     string                       `json:"source_digest"`
	PromptSchemaHash string                       `json:"prompt_schema_hash"`
	Result           *extraction.ExtractionResult `json:"result"`
	Error            string                       `json:"error"`
	Digest           string                       `json:"digest"`
}

func extractionFailure(err error) string {
	// Only allow static diagnostics; never persist arbitrary provider error text.
	message := err.Error()
	for _, code := range []string{"202", "400", "401", "403", "404", "408", "413", "422", "429", "500", "502", "503", "504"} {
		if message == "provider HTTP "+code {
			return "Extraction provider request failed (HTTP " + code + "). Check account/model access or rate limits before requesting a new job."
		}
	}
	switch message {
	case "source changed", "source superseded":
		return "The source is no longer active. Select the current document version."
	case "configuration changed":
		return "The configured model or extraction prompt changed. Request a new job."
	case "requester permissions changed":
		return "The requesting account no longer has extraction permission."
	case "provider refused extraction", "provider refused request":
		return "The model refused extraction. No suggestions were saved."
	case "provider incomplete or invalid response":
		return "The model response was incomplete or invalid. No suggestions were saved."
	case "missing or ambiguous source evidence", "unsupported numeric fact", "unsupported source phrase":
		return "The model returned unsupported or ambiguous source evidence. No suggestions were saved."
	default:
		return "Extraction failed. Check configuration and request a new job explicitly."
	}
}

func extractionEditor(a Actor) bool { return a.Role == "pharmacist" || a.Role == "admin" }
func extractionJob(ctx context.Context, q querier, id string) (ExtractionJob, error) {
	var j ExtractionJob
	var b []byte
	err := q.QueryRow(ctx, `SELECT payload FROM extraction_jobs WHERE id=$1`, id).Scan(&b)
	if err != nil {
		return j, normalize(err)
	}
	err = json.Unmarshal(b, &j)
	return j, err
}
func persistJob(ctx context.Context, tx pgx.Tx, j *ExtractionJob) error {
	j.Digest = ""
	j.Digest = domain.Digest(*j)
	_, err := tx.Exec(ctx, `UPDATE extraction_jobs SET status=$2,payload=$3 WHERE id=$1`, j.ID, j.Status, marshal(j))
	return err
}
func extractionSource(ctx context.Context, q querier, doc Document) (extraction.SourceInput, error) {
	in := extraction.SourceInput{Context: doc.Context, PDFHash: doc.Source.DocumentSHA256, Passages: []extraction.SourcePassage{}}
	rows, err := q.Query(ctx, `SELECT id,section,content FROM chunks WHERE document_id=$1 AND section IN ('4.1','4.2','4.3','4.4','5.2') ORDER BY start_byte`, doc.ID)
	if err != nil {
		return in, err
	}
	defer rows.Close()
	hasDose := false
	for rows.Next() {
		var p extraction.SourcePassage
		if err = rows.Scan(&p.ID, &p.Section, &p.Text); err != nil {
			return in, err
		}
		in.Passages = append(in.Passages, p)
		hasDose = hasDose || p.Section == "4.2"
	}
	if err = rows.Err(); err != nil {
		return in, err
	}
	b, _ := json.Marshal(in)
	if !hasDose || len(b) > extraction.MaxInputBytes {
		return in, fmt.Errorf("%w: complete dosing context missing or source exceeds extraction budget", ErrInvalid)
	}
	return in, nil
}
func (s *Store) CreateExtraction(ctx context.Context, a Actor, in ExtractionInput, model string) (ExtractionJob, error) {
	return s.createExtraction(ctx, a, in, model, extraction.ManifestHash())
}

func (s *Store) CreateExtractionForProvider(ctx context.Context, a Actor, in ExtractionInput, p extraction.Provider) (ExtractionJob, error) {
	return s.createExtraction(ctx, a, in, p.Model(), extraction.ProviderManifest(p))
}

func (s *Store) createExtraction(ctx context.Context, a Actor, in ExtractionInput, model, manifest string) (ExtractionJob, error) {
	var j ExtractionJob
	if !extractionEditor(a) {
		return j, ErrForbidden
	}
	if in.DocumentID == "" || len(in.RequestKey) < 16 || len(in.RequestKey) > 100 || model == "" {
		return j, ErrInvalid
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		var existing string
		err := tx.QueryRow(ctx, `SELECT id FROM extraction_jobs WHERE actor_id=$1 AND request_key=$2`, a.ID, in.RequestKey).Scan(&existing)
		if err == nil {
			j, err = extractionJob(ctx, tx, existing)
			if j.DocumentID != in.DocumentID || j.Model != model {
				return ErrConflict
			}
			return err
		}
		if err != pgx.ErrNoRows {
			return err
		}
		doc, err := document(ctx, tx, in.DocumentID)
		if err != nil {
			return err
		}
		if doc.Status != "ACTIVE" {
			return ErrConflict
		}
		var pending int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM extraction_jobs WHERE status IN ('QUEUED','RUNNING')`).Scan(&pending); err != nil {
			return err
		}
		if pending >= 10 {
			return fmt.Errorf("%w: extraction queue full", ErrConflict)
		}
		source, err := extractionSource(ctx, tx, doc)
		if err != nil {
			return err
		}
		j = ExtractionJob{ID: ID(), DocumentID: doc.ID, Actor: a, Status: "QUEUED", Model: model, CreatedAt: time.Now().UTC(), Source: source, SourceDigest: domain.Digest(source), PromptSchemaHash: manifest}
		j.Digest = domain.Digest(j)
		if _, err = tx.Exec(ctx, `INSERT INTO extraction_jobs(id,document_id,actor_id,request_key,model,status,payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, j.ID, doc.ID, a.ID, in.RequestKey, model, j.Status, marshal(j)); err != nil {
			return err
		}
		_, err = appendAudit(ctx, tx, a, "EXTRACTION_QUEUED", j.ID, map[string]any{"document_id": doc.ID, "source_digest": j.SourceDigest, "model": model, "prompt_schema_hash": j.PromptSchemaHash})
		return err
	})
	return j, err
}
func (s *Store) Extraction(ctx context.Context, a Actor, id string) (ExtractionJob, error) {
	if !extractionEditor(a) {
		return ExtractionJob{}, ErrForbidden
	}
	return extractionJob(ctx, s.Pool, id)
}
func (s *Store) Extractions(ctx context.Context, a Actor) ([]ExtractionJob, error) {
	if !extractionEditor(a) {
		return nil, ErrForbidden
	}
	rows, err := s.Pool.Query(ctx, `SELECT payload FROM extraction_jobs ORDER BY created_at DESC,id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExtractionJob{}
	for rows.Next() {
		var raw []byte
		var j ExtractionJob
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Store) CancelExtraction(ctx context.Context, a Actor, id string) (ExtractionJob, error) {
	return s.endExtraction(ctx, a, id, false)
}
func (s *Store) DismissExtraction(ctx context.Context, a Actor, id string) (ExtractionJob, error) {
	return s.endExtraction(ctx, a, id, true)
}
func (s *Store) endExtraction(ctx context.Context, a Actor, id string, dismiss bool) (ExtractionJob, error) {
	var j ExtractionJob
	if !extractionEditor(a) {
		return j, ErrForbidden
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		var err error
		j, err = extractionJob(ctx, tx, id)
		if err != nil {
			return err
		}
		target := "CANCELLED"
		if dismiss {
			target = "DISMISSED"
		}
		if j.Status == target {
			return nil
		}
		if dismiss && j.Status != "COMPLETED" || !dismiss && j.Status != "QUEUED" && j.Status != "RUNNING" {
			return ErrConflict
		}
		j.Status = target
		now := time.Now().UTC()
		j.FinishedAt = &now
		if err = persistJob(ctx, tx, &j); err != nil {
			return err
		}
		_, err = appendAudit(ctx, tx, a, "EXTRACTION_"+target, id, map[string]string{"document_id": j.DocumentID})
		return err
	})
	return j, err
}

// RunExtraction processes at most one job. A persisted RUNNING job is never
// automatically resubmitted after a crash, avoiding duplicate paid API calls.
func (s *Store) RunExtraction(ctx context.Context, p extraction.Provider) error {
	var j ExtractionJob
	err := s.write(ctx, func(tx pgx.Tx) error {
		// Expired jobs fail visibly; an operator can explicitly request a new job.
		rows, err := tx.Query(ctx, `SELECT id FROM extraction_jobs WHERE status='RUNNING' AND (payload->>'started_at')::timestamptz < now()-interval '3 minutes'`)
		if err != nil {
			return err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			old, e := extractionJob(ctx, tx, id)
			if e != nil {
				return e
			}
			old.Status = "FAILED"
			old.Error = "Worker interrupted; request a new job explicitly."
			now := time.Now().UTC()
			old.FinishedAt = &now
			if e = persistJob(ctx, tx, &old); e != nil {
				return e
			}
			if _, e = appendAudit(ctx, tx, old.Actor, "EXTRACTION_FAILED", id, map[string]string{"reason": "worker_interrupted"}); e != nil {
				return e
			}
		}
		var active int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM extraction_jobs WHERE status='RUNNING'`).Scan(&active); err != nil {
			return err
		}
		if active > 0 {
			return nil
		}
		var id string
		err = tx.QueryRow(ctx, `SELECT id FROM extraction_jobs WHERE status='QUEUED' ORDER BY created_at,id LIMIT 1`).Scan(&id)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		j, err = extractionJob(ctx, tx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		j.StartedAt = &now
		j.Status = "RUNNING"
		if err = persistJob(ctx, tx, &j); err != nil {
			return err
		}
		_, err = appendAudit(ctx, tx, j.Actor, "EXTRACTION_STARTED", id, map[string]string{"model": j.Model})
		return err
	})
	if err != nil || j.ID == "" {
		return err
	}
	var result extraction.ExtractionResult
	// Recheck source lifecycle and requester permissions before transmitting text.
	doc, e := document(ctx, s.Pool, j.DocumentID)
	if e != nil || doc.Status != "ACTIVE" || doc.Source.DocumentSHA256 != j.Source.PDFHash {
		err = fmt.Errorf("source changed")
	} else if j.Model != p.Model() || j.PromptSchemaHash != extraction.ProviderManifest(p) || j.SourceDigest != domain.Digest(j.Source) {
		err = fmt.Errorf("configuration changed")
	} else {
		// Fixture actors need no accounts; real authenticated actors must still be active.
		if !s.AllowSynthetic {
			err = requireActive(ctx, s.Pool, j.Actor, "")
		}
		if err == nil {
			call, cancel := context.WithTimeout(ctx, 95*time.Second)
			result, err = p.Extract(call, j.Source)
			cancel()
		}
	}
	if err == nil {
		err = extraction.Validate(result.Output, j.Source)
		if result.Model == "" || result.ResponseID == "" {
			err = fmt.Errorf("provider identity missing")
		}
	}
	// Use a short independent context to persist interruption/failure on shutdown.
	finish, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.write(finish, func(tx pgx.Tx) error {
		current, e := extractionJob(finish, tx, j.ID)
		if e != nil {
			return e
		}
		if current.Status != "RUNNING" {
			return nil
		}
		doc, e := document(finish, tx, j.DocumentID)
		if e != nil {
			return e
		}
		if doc.Status != "ACTIVE" {
			err = fmt.Errorf("source superseded")
		}
		if !s.AllowSynthetic && requireActive(finish, tx, j.Actor, "") != nil {
			err = fmt.Errorf("requester permissions changed")
		}
		now := time.Now().UTC()
		current.FinishedAt = &now
		current.Status = "COMPLETED"
		if err != nil {
			current.Status = "FAILED"
			current.Error = extractionFailure(err)
		} else {
			current.Result = &result
		}
		if e = persistJob(finish, tx, &current); e != nil {
			return e
		}
		_, e = appendAudit(finish, tx, j.Actor, "EXTRACTION_"+current.Status, j.ID, map[string]string{"document_id": j.DocumentID, "result_digest": domain.Digest(current.Result)})
		return e
	})
}

func (s *Store) SaveExtraction(ctx context.Context, a Actor, id string, in ExtractionSave) (StoredRule, error) {
	var out StoredRule
	if !extractionEditor(a) {
		return out, ErrForbidden
	}
	err := s.write(ctx, func(tx pgx.Tx) error {
		j, err := extractionJob(ctx, tx, id)
		if err != nil {
			return err
		}
		if j.Status != "COMPLETED" || j.Digest != in.ExpectedDigest || j.Result == nil || in.FindingIndex < 0 || in.FindingIndex >= len(j.Result.Output.Findings) {
			return ErrConflict
		}
		doc, err := document(ctx, tx, j.DocumentID)
		if err != nil {
			return err
		}
		if doc.Status != "ACTIVE" || doc.Source.DocumentSHA256 != j.Source.PDFHash {
			return ErrConflict
		}
		var existing string
		err = tx.QueryRow(ctx, `SELECT rule_id FROM extraction_candidates WHERE job_id=$1 AND finding_index=$2`, id, in.FindingIndex).Scan(&existing)
		if err == nil {
			out, err = storedRule(ctx, tx, existing)
			return err
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if in.Candidate.Rule.Status != "PENDING" || in.Candidate.Rule.Approval != nil || in.Candidate.Rule.Context != doc.Context {
			return ErrInvalid
		}
		finding := j.Result.Output.Findings[in.FindingIndex]
		allowed := false
		for _, fact := range finding.Facts {
			if fact.ChunkID == in.Candidate.ChunkID {
				allowed = true
			}
		}
		if !allowed {
			return ErrInvalid
		}
		// Never drop unresolved or non-executable qualifiers during conversion.
		unmodeled := append([]string{}, finding.Unresolved...)
		for _, fact := range finding.Facts {
			switch fact.Field {
			case "condition", "population", "route", "indication", "loading_dose", "maintenance_dose", "dialysis", "daily_maximum", "monitoring", "dependency":
				unmodeled = append(unmodeled, fact.Field+": "+fact.Value)
			}
		}
		if len(unmodeled) > 0 {
			if in.Candidate.Rule.Conditions == nil {
				in.Candidate.Rule.Conditions = &domain.RuleConditions{}
			}
			for _, condition := range unmodeled {
				found := false
				for _, old := range in.Candidate.Rule.Conditions.Unmodeled {
					if strings.TrimSpace(old) == strings.TrimSpace(condition) {
						found = true
					}
				}
				if !found {
					in.Candidate.Rule.Conditions.Unmodeled = append(in.Candidate.Rule.Conditions.Unmodeled, condition)
				}
			}
		}
		if err = s.addCandidateTx(ctx, tx, a, in.Candidate, &out); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO extraction_candidates(job_id,finding_index,rule_id) VALUES($1,$2,$3)`, id, in.FindingIndex, out.Rule.ID); err != nil {
			return err
		}
		_, err = appendAudit(ctx, tx, a, "EXTRACTION_CANDIDATE_SAVED", id, map[string]any{"finding_index": in.FindingIndex, "rule_id": out.Rule.ID, "suggestion_digest": j.Digest})
		return err
	})
	return out, err
}
