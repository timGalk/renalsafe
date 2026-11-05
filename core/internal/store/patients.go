package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"renalsafe/core/internal/domain"
	"strings"
	"time"
)

type SyntheticPatient struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Alias     string    `json:"alias"`
	OwnerID   string    `json:"owner_id"`
	Archived  bool      `json:"archived"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}
type PatientInput struct {
	Alias     string `json:"alias"`
	Synthetic bool   `json:"synthetic"`
}
type PatientUpdate struct {
	Alias           string `json:"alias"`
	Archived        bool   `json:"archived"`
	ExpectedVersion int64  `json:"expected_version"`
}
type ObservationInput struct {
	Creatinine domain.Quantity  `json:"creatinine"`
	Bounds     *domain.Interval `json:"creatinine_bounds,omitempty"`
	MeasuredAt time.Time        `json:"measured_at"`
	IDMS       *bool            `json:"idms_standardized"`
	CorrectsID string           `json:"corrects_id,omitempty"`
	Reason     string           `json:"correction_reason,omitempty"`
}
type Observation struct {
	cursorAt       time.Time
	ID             string           `json:"id"`
	PatientID      string           `json:"patient_id"`
	AuthorID       string           `json:"author_id"`
	CreatedAt      time.Time        `json:"created_at"`
	Input          ObservationInput `json:"input"`
	SupersededByID string           `json:"superseded_by_id,omitempty"`
}
type PatientAssessmentInput struct {
	ObservationID string         `json:"observation_id"`
	PreviousID    string         `json:"previous_assessment_id,omitempty"`
	RetryKey      string         `json:"idempotency_key"`
	Request       domain.Request `json:"request"`
}
type PatientPage struct {
	Items []SyntheticPatient `json:"items"`
	Next  string             `json:"next_cursor,omitempty"`
}
type ObservationPage struct {
	Items []Observation `json:"items"`
	Next  string        `json:"next_cursor,omitempty"`
}
type AssessmentSummary struct {
	cursorAt         time.Time
	ID               string             `json:"id"`
	CreatedAt        time.Time          `json:"created_at"`
	ActorID          string             `json:"actor_id"`
	PatientID        string             `json:"patient_id,omitempty"`
	PatientCode      string             `json:"patient_code,omitempty"`
	ObservationID    string             `json:"observation_id,omitempty"`
	PreviousID       string             `json:"previous_assessment_id,omitempty"`
	Products         []string           `json:"products"`
	Statuses         []string           `json:"statuses"`
	Renal            domain.RenalResult `json:"renal"`
	EngineVersion    string             `json:"engine_version"`
	KnowledgeVersion string             `json:"knowledge_version"`
}
type AssessmentPage struct {
	Items []AssessmentSummary `json:"items"`
	Next  string              `json:"next_cursor,omitempty"`
}
type HistoryFilter struct {
	PatientID, Cursor, Product, Status string
	Unlinked                           bool
	From, To                           *time.Time
	Limit                              int
}
type pageCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func cursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString(marshal(pageCursor{at, id}))
}
func pagination(raw string, limit int) (*time.Time, string, int, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 || len(raw) > 300 {
		return nil, "", 0, ErrInvalid
	}
	if raw == "" {
		return nil, "", limit, nil
	}
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return nil, "", 0, ErrInvalid
	}
	var c pageCursor
	if domain.Decode(b, &c) != nil || c.At.IsZero() || len(c.ID) != 32 {
		return nil, "", 0, ErrInvalid
	}
	return &c.At, c.ID, limit, nil
}
func patient(ctx context.Context, q querier, id string) (SyntheticPatient, error) {
	var p SyntheticPatient
	e := q.QueryRow(ctx, `SELECT id,code,alias,owner_id,archived,version,created_at FROM synthetic_patients WHERE id=$1`, id).Scan(&p.ID, &p.Code, &p.Alias, &p.OwnerID, &p.Archived, &p.Version, &p.CreatedAt)
	return p, normalize(e)
}
func patientPermission(ctx context.Context, q querier, a Actor, id string, write bool) error {
	if write && a.Role != "physician" {
		return ErrForbidden
	}
	if e := requireActive(ctx, q, a, ""); e != nil {
		return e
	}
	var ok bool
	e := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM synthetic_patients p WHERE p.id=$1 AND ($2='admin' OR p.owner_id=$3 OR EXISTS(SELECT 1 FROM patient_access WHERE patient_id=p.id AND user_id=$3)))`, id, a.Role, a.ID).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
func patientManager(ctx context.Context, q querier, a Actor, p SyntheticPatient) error {
	if e := patientPermission(ctx, q, a, p.ID, false); e != nil {
		return e
	}
	if a.Role != "admin" && (a.Role != "physician" || p.OwnerID != a.ID) {
		return ErrForbidden
	}
	return nil
}
func (s *Store) CreatePatient(ctx context.Context, a Actor, in PatientInput) (SyntheticPatient, error) {
	p := SyntheticPatient{ID: ID(), Alias: strings.TrimSpace(in.Alias), OwnerID: a.ID}
	if !in.Synthetic || len(p.Alias) > 80 {
		return p, ErrInvalid
	}
	p.Code = "SYN-" + strings.ToUpper(p.ID)
	e := s.write(ctx, func(tx pgx.Tx) error {
		if e := requireActive(ctx, tx, a, "physician"); e != nil {
			return e
		}
		if e := tx.QueryRow(ctx, `INSERT INTO synthetic_patients(id,code,alias,owner_id) VALUES($1,$2,$3,$4) RETURNING created_at,version`, p.ID, p.Code, p.Alias, a.ID).Scan(&p.CreatedAt, &p.Version); e != nil {
			return e
		}
		_, e := appendAudit(ctx, tx, a, "PATIENT_CREATED", p.ID, p)
		return e
	})
	return p, e
}
func (s *Store) Patients(ctx context.Context, a Actor, raw string, limit int) (PatientPage, error) {
	out := PatientPage{Items: []SyntheticPatient{}}
	at, id, limit, e := pagination(raw, limit)
	if e != nil {
		return out, e
	}
	e = s.write(ctx, func(tx pgx.Tx) error {
		if e := requireActive(ctx, tx, a, ""); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT p.id,p.code,p.alias,p.owner_id,p.archived,p.version,p.created_at FROM synthetic_patients p WHERE ($1='admin' OR p.owner_id=$2 OR EXISTS(SELECT 1 FROM patient_access WHERE patient_id=p.id AND user_id=$2)) AND ($3::timestamptz IS NULL OR (p.created_at,p.id)<($3,$4)) ORDER BY p.created_at DESC,p.id DESC LIMIT $5`, a.Role, a.ID, at, id, limit+1)
		if e != nil {
			return e
		}
		for rows.Next() {
			var p SyntheticPatient
			if e = rows.Scan(&p.ID, &p.Code, &p.Alias, &p.OwnerID, &p.Archived, &p.Version, &p.CreatedAt); e != nil {
				rows.Close()
				return e
			}
			out.Items = append(out.Items, p)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(out.Items) > limit {
			out.Items = out.Items[:limit]
			last := out.Items[limit-1]
			out.Next = cursor(last.CreatedAt, last.ID)
		}
		if a.Role == "admin" {
			_, e = appendAudit(ctx, tx, a, "PATIENT_LIST_READ", "", map[string]int{"count": len(out.Items)})
		}
		return e
	})
	return out, e
}
func (s *Store) Patient(ctx context.Context, a Actor, id string) (SyntheticPatient, error) {
	var out SyntheticPatient
	e := s.write(ctx, func(tx pgx.Tx) error {
		if e := patientPermission(ctx, tx, a, id, false); e != nil {
			return e
		}
		var e error
		out, e = patient(ctx, tx, id)
		if e == nil && a.Role == "admin" {
			_, e = appendAudit(ctx, tx, a, "PATIENT_READ", id, map[string]string{"status": "read"})
		}
		return e
	})
	return out, e
}
func (s *Store) UpdatePatient(ctx context.Context, a Actor, id string, in PatientUpdate) (SyntheticPatient, error) {
	var out SyntheticPatient
	if len(strings.TrimSpace(in.Alias)) > 80 {
		return out, ErrInvalid
	}
	e := s.write(ctx, func(tx pgx.Tx) error {
		p, e := patient(ctx, tx, id)
		if e != nil {
			return e
		}
		if e = patientManager(ctx, tx, a, p); e != nil {
			return e
		}
		if p.Version != in.ExpectedVersion {
			return ErrConflict
		}
		if _, e = tx.Exec(ctx, `UPDATE synthetic_patients SET alias=$2,archived=$3,version=version+1 WHERE id=$1`, id, strings.TrimSpace(in.Alias), in.Archived); e != nil {
			return e
		}
		out, e = patient(ctx, tx, id)
		if e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, a, "PATIENT_UPDATED", id, out)
		return e
	})
	return out, e
}

type PatientAccess struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (s *Store) PatientAccess(ctx context.Context, a Actor, id string) ([]PatientAccess, error) {
	out := []PatientAccess{}
	e := s.write(ctx, func(tx pgx.Tx) error {
		p, e := patient(ctx, tx, id)
		if e != nil {
			return e
		}
		if e = patientManager(ctx, tx, a, p); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT u.id,u.role FROM patient_access x JOIN app_users u ON u.id=x.user_id WHERE x.patient_id=$1 ORDER BY u.id`, id)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v PatientAccess
			if e = rows.Scan(&v.UserID, &v.Role); e != nil {
				return e
			}
			out = append(out, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if a.Role == "admin" {
			_, e = appendAudit(ctx, tx, a, "PATIENT_ACCESS_READ", id, map[string]int{"count": len(out)})
		}
		return e
	})
	return out, e
}
func (s *Store) SetPatientAccess(ctx context.Context, a Actor, id, userID string, grant bool) error {
	return s.write(ctx, func(tx pgx.Tx) error {
		p, e := patient(ctx, tx, id)
		if e != nil {
			return e
		}
		if e = patientManager(ctx, tx, a, p); e != nil {
			return e
		}
		if userID == p.OwnerID {
			return ErrInvalid
		}
		if grant {
			var role string
			var active bool
			if e = tx.QueryRow(ctx, `SELECT role,active FROM app_users WHERE id=$1`, userID).Scan(&role, &active); e != nil {
				return normalize(e)
			}
			if !active || (role != "physician" && role != "pharmacist") {
				return ErrInvalid
			}
			_, e = tx.Exec(ctx, `INSERT INTO patient_access VALUES($1,$2) ON CONFLICT DO NOTHING`, id, userID)
		} else {
			_, e = tx.Exec(ctx, `DELETE FROM patient_access WHERE patient_id=$1 AND user_id=$2`, id, userID)
		}
		if e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, a, "PATIENT_ACCESS_CHANGED", id, map[string]any{"user_id": userID, "granted": grant})
		return e
	})
}
func validObservation(in ObservationInput) bool {
	q := in.Creatinine
	if !domain.Finite(q.Value) || q.Value <= 0 || (q.Unit != "mg/dL" && q.Unit != "umol/L") || in.MeasuredAt.IsZero() || in.MeasuredAt.After(time.Now()) || len(in.Reason) > 1000 {
		return false
	}
	if in.CorrectsID != "" && strings.TrimSpace(in.Reason) == "" {
		return false
	}
	if in.Bounds != nil {
		b := in.Bounds
		if !b.Valid() || b.Upper == nil || !b.LowerInclusive || !b.UpperInclusive || !b.Contains(q.Value) {
			return false
		}
	}
	return true
}
func (s *Store) AddObservation(ctx context.Context, a Actor, id string, in ObservationInput) (Observation, error) {
	out := Observation{ID: ID(), PatientID: id, AuthorID: a.ID, CreatedAt: time.Now().UTC(), Input: in}
	if !validObservation(in) {
		return out, ErrInvalid
	}
	e := s.write(ctx, func(tx pgx.Tx) error {
		if e := patientPermission(ctx, tx, a, id, true); e != nil {
			return e
		}
		p, e := patient(ctx, tx, id)
		if e != nil {
			return e
		}
		if p.Archived {
			return ErrConflict
		}
		if in.CorrectsID != "" {
			var eligible bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM renal_observations o WHERE o.id=$1 AND o.patient_id=$2 AND NOT EXISTS(SELECT 1 FROM renal_observations WHERE corrects_id=o.id))`, in.CorrectsID, id).Scan(&eligible); e != nil {
				return e
			}
			if !eligible {
				return ErrConflict
			}
		}
		if _, e = tx.Exec(ctx, `INSERT INTO renal_observations(id,patient_id,measured_at,corrects_id,payload) VALUES($1,$2,$3,NULLIF($4,''),$5)`, out.ID, id, in.MeasuredAt, in.CorrectsID, marshal(out)); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, a, "OBSERVATION_RECORDED", out.ID, out)
		return e
	})
	return out, e
}
func (s *Store) Observations(ctx context.Context, a Actor, id, raw string, limit int) (ObservationPage, error) {
	out := ObservationPage{Items: []Observation{}}
	at, last, limit, e := pagination(raw, limit)
	if e != nil {
		return out, e
	}
	e = s.write(ctx, func(tx pgx.Tx) error {
		if e := patientPermission(ctx, tx, a, id, false); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT o.payload,COALESCE(c.id,''),o.measured_at FROM renal_observations o LEFT JOIN renal_observations c ON c.corrects_id=o.id WHERE o.patient_id=$1 AND ($2::timestamptz IS NULL OR (o.measured_at,o.id)<($2,$3)) ORDER BY o.measured_at DESC,o.id DESC LIMIT $4`, id, at, last, limit+1)
		if e != nil {
			return e
		}
		for rows.Next() {
			var raw []byte
			var o Observation
			if e = rows.Scan(&raw, &o.SupersededByID, &o.cursorAt); e != nil {
				rows.Close()
				return e
			}
			if e = json.Unmarshal(raw, &o); e != nil {
				rows.Close()
				return e
			}
			out.Items = append(out.Items, o)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(out.Items) > limit {
			out.Items = out.Items[:limit]
			last := out.Items[limit-1]
			out.Next = cursor(last.cursorAt, last.ID)
		}
		if a.Role == "admin" {
			_, e = appendAudit(ctx, tx, a, "OBSERVATION_LIST_READ", id, map[string]int{"count": len(out.Items)})
		}
		return e
	})
	return out, e
}
func (s *Store) AssessPatient(ctx context.Context, a Actor, id string, in PatientAssessmentInput) (AssessmentRecord, error) {
	var out AssessmentRecord
	if len(in.RetryKey) < 16 || len(in.RetryKey) > 128 {
		return out, ErrInvalid
	}
	e := s.write(ctx, func(tx pgx.Tx) error {
		if e := patientPermission(ctx, tx, a, id, true); e != nil {
			return e
		}
		digest := domain.Digest(in)
		var oldID, oldDigest string
		e := tx.QueryRow(ctx, `SELECT assessment_id,request_digest FROM patient_assessments WHERE patient_id=$1 AND actor_id=$2 AND retry_key=$3`, id, a.ID, in.RetryKey).Scan(&oldID, &oldDigest)
		if e == nil {
			if digest != oldDigest {
				return ErrConflict
			}
			var raw []byte
			if e = tx.QueryRow(ctx, `SELECT payload FROM assessments WHERE id=$1`, oldID).Scan(&raw); e != nil {
				return e
			}
			return json.Unmarshal(raw, &out)
		}
		if e != pgx.ErrNoRows {
			return e
		}
		p, e := patient(ctx, tx, id)
		if e != nil {
			return e
		}
		if p.Archived {
			return ErrConflict
		}
		var raw []byte
		e = tx.QueryRow(ctx, `SELECT payload FROM renal_observations o WHERE o.id=$1 AND o.patient_id=$2 AND NOT EXISTS(SELECT 1 FROM renal_observations WHERE corrects_id=o.id)`, in.ObservationID, id).Scan(&raw)
		if e != nil {
			return normalize(e)
		}
		var obs Observation
		if e = json.Unmarshal(raw, &obs); e != nil {
			return e
		}
		if in.PreviousID != "" {
			var ok bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM patient_assessments WHERE assessment_id=$1 AND patient_id=$2)`, in.PreviousID, id).Scan(&ok); e != nil {
				return e
			}
			if !ok {
				return ErrInvalid
			}
		}
		req := in.Request
		req.Patient.Creatinine = obs.Input.Creatinine
		req.Patient.CreatinineBounds = obs.Input.Bounds
		req.Patient.MeasuredAt = obs.Input.MeasuredAt
		req.Patient.IDMS = obs.Input.IDMS
		if e = validateAssessmentRequest(req); e != nil {
			return e
		}
		out, e = s.assessTx(ctx, tx, a, req)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO patient_assessments(assessment_id,patient_id,observation_id,previous_id,actor_id,retry_key,request_digest) VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7)`, out.ID, id, in.ObservationID, in.PreviousID, a.ID, in.RetryKey, digest); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, a, "PATIENT_ASSESSMENT_LINKED", out.ID, map[string]string{"patient_id": id, "observation_id": in.ObservationID, "previous_assessment_id": in.PreviousID})
		return e
	})
	return out, e
}

func validateHistory(f HistoryFilter) error {
	if len(f.Product) > 120 || len(f.Status) > 80 || f.From != nil && f.To != nil && f.From.After(*f.To) {
		return ErrInvalid
	}
	return nil
}
func (s *Store) AssessmentHistory(ctx context.Context, a Actor, f HistoryFilter) (AssessmentPage, error) {
	out := AssessmentPage{Items: []AssessmentSummary{}}
	at, id, limit, e := pagination(f.Cursor, f.Limit)
	if e != nil {
		return out, e
	}
	if e = validateHistory(f); e != nil {
		return out, e
	}
	e = s.write(ctx, func(tx pgx.Tx) error {
		if e := requireActive(ctx, tx, a, ""); e != nil {
			return e
		}
		if f.PatientID != "" {
			if e := patientPermission(ctx, tx, a, f.PatientID, false); e != nil {
				return e
			}
		}
		rows, e := tx.Query(ctx, `SELECT a.payload,a.created_at,COALESCE(l.patient_id,''),COALESCE(p.code,''),COALESCE(l.observation_id,''),COALESCE(l.previous_id,'') FROM assessments a LEFT JOIN patient_assessments l ON l.assessment_id=a.id LEFT JOIN synthetic_patients p ON p.id=l.patient_id
 WHERE (l.assessment_id IS NULL AND (a.actor_id=$1 OR $2='admin') OR l.assessment_id IS NOT NULL AND ($2='admin' OR p.owner_id=$1 OR EXISTS(SELECT 1 FROM patient_access WHERE patient_id=p.id AND user_id=$1)))
 AND ($3='' OR l.patient_id=$3) AND (NOT $4 OR l.assessment_id IS NULL)
 AND ($5::timestamptz IS NULL OR (a.created_at,a.id)<($5,$6)) AND ($7::timestamptz IS NULL OR a.created_at>=$7) AND ($8::timestamptz IS NULL OR a.created_at<=$8)
 AND ($9='' AND $10='' OR EXISTS(SELECT 1 FROM jsonb_array_elements(a.payload->'assessment'->'results') r WHERE ($9='' OR strpos(lower(r->'context'->>'product'),lower($9))>0) AND ($10='' OR r->>'status'=$10)))
 ORDER BY a.created_at DESC,a.id DESC LIMIT $11`, a.ID, a.Role, f.PatientID, f.Unlinked, at, id, f.From, f.To, f.Product, f.Status, limit+1)
		if e != nil {
			return e
		}
		for rows.Next() {
			var raw []byte
			var item AssessmentSummary
			if e = rows.Scan(&raw, &item.cursorAt, &item.PatientID, &item.PatientCode, &item.ObservationID, &item.PreviousID); e != nil {
				rows.Close()
				return e
			}
			var r AssessmentRecord
			if e = json.Unmarshal(raw, &r); e != nil {
				rows.Close()
				return e
			}
			item.ID = r.ID
			item.CreatedAt = r.CreatedAt
			item.ActorID = r.Actor.ID
			item.Renal = r.Assessment.Renal
			item.EngineVersion = r.Assessment.EngineVersion
			item.KnowledgeVersion = r.Assessment.KnowledgeVersion
			item.Products = []string{}
			item.Statuses = []string{}
			for _, v := range r.Assessment.Results {
				item.Products = append(item.Products, v.Context.Product)
				item.Statuses = append(item.Statuses, v.Status)
			}
			out.Items = append(out.Items, item)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(out.Items) > limit {
			out.Items = out.Items[:limit]
			last := out.Items[limit-1]
			out.Next = cursor(last.cursorAt, last.ID)
		}
		if a.Role == "admin" {
			_, e = appendAudit(ctx, tx, a, "ASSESSMENT_HISTORY_READ", f.PatientID, map[string]int{"count": len(out.Items)})
		}
		return e
	})
	return out, e
}
func assessmentPermission(ctx context.Context, q querier, a Actor, id string) error {
	var owner, patientID string
	e := q.QueryRow(ctx, `SELECT a.actor_id,COALESCE(l.patient_id,'') FROM assessments a LEFT JOIN patient_assessments l ON l.assessment_id=a.id WHERE a.id=$1`, id).Scan(&owner, &patientID)
	if e != nil {
		return normalize(e)
	}
	if patientID != "" {
		return patientPermission(ctx, q, a, patientID, false)
	}
	if a.Role != "admin" && owner != a.ID {
		return ErrForbidden
	}
	return nil
}
func assessmentRead(ctx context.Context, q querier, a Actor, id string) (AssessmentRecord, error) {
	var out AssessmentRecord
	if e := assessmentPermission(ctx, q, a, id); e != nil {
		return out, e
	}
	var raw []byte
	e := q.QueryRow(ctx, `SELECT payload FROM assessments WHERE id=$1`, id).Scan(&raw)
	if e != nil {
		return out, normalize(e)
	}
	e = json.Unmarshal(raw, &out)
	return out, e
}

type AssessmentExport struct {
	SchemaVersion int               `json:"schema_version"`
	ID            string            `json:"id"`
	PatientID     string            `json:"patient_id,omitempty"`
	PatientCode   string            `json:"patient_code,omitempty"`
	ObservationID string            `json:"observation_id,omitempty"`
	PreviousID    string            `json:"previous_assessment_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	Actor         Actor             `json:"actor"`
	Assessment    domain.Assessment `json:"assessment"`
	AuditHash     string            `json:"audit_hash"`
}

func (s *Store) ExportAssessment(ctx context.Context, a Actor, id string) (AssessmentExport, error) {
	var out AssessmentExport
	e := s.write(ctx, func(tx pgx.Tx) error {
		r, e := assessmentRead(ctx, tx, a, id)
		if e != nil {
			return e
		}
		out = AssessmentExport{SchemaVersion: 1, ID: r.ID, CreatedAt: r.CreatedAt, Actor: r.Actor, Assessment: r.Assessment, AuditHash: r.AuditHash}
		e = tx.QueryRow(ctx, `SELECT l.patient_id,p.code,l.observation_id,COALESCE(l.previous_id,'') FROM patient_assessments l JOIN synthetic_patients p ON p.id=l.patient_id WHERE l.assessment_id=$1`, id).Scan(&out.PatientID, &out.PatientCode, &out.ObservationID, &out.PreviousID)
		if e != nil && e != pgx.ErrNoRows {
			return e
		}
		if len(marshal(out)) > 4<<20 {
			return fmt.Errorf("%w: report exceeds 4 MiB", ErrInvalid)
		}
		_, e = appendAudit(ctx, tx, a, "ASSESSMENT_EXPORTED", id, map[string]string{"patient_id": out.PatientID})
		return e
	})
	return out, e
}

type StaffMember struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (s *Store) StaffDirectory(ctx context.Context, a Actor) ([]StaffMember, error) {
	if e := requireActive(ctx, s.Pool, a, ""); e != nil {
		return nil, e
	}
	out := []StaffMember{}
	rows, e := s.Pool.Query(ctx, `SELECT id,username,role FROM app_users WHERE active AND role IN ('physician','pharmacist') ORDER BY username LIMIT 500`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var m StaffMember
		if e = rows.Scan(&m.ID, &m.Username, &m.Role); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
