package store_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBoundedAuditMetadataAndVerification(t *testing.T) {
	db := historyDB(t)
	ctx := t.Context()
	p := createCase(t, db)
	_ = caseObservation(t, db, p)
	page, e := db.AuditSummaries(ctx, caseAdmin, "", 1)
	if e != nil || len(page.Items) != 1 || page.Next == "" || !page.PageHashesValid {
		t.Fatal("audit page", e)
	}
	next, e := db.AuditSummaries(ctx, caseAdmin, page.Next, 1)
	if e != nil || len(next.Items) != 1 || next.Items[0].Sequence >= page.Items[0].Sequence {
		t.Fatal("audit cursor", e)
	}
	encoded, _ := json.Marshal(page)
	if strings.Contains(string(encoded), "creatinine") || strings.Contains(string(encoded), `"data"`) {
		t.Fatal("clinical payload exposed in summary")
	}
	verified, e := db.VerifyAudit(ctx, caseAdmin)
	if e != nil || !verified.Valid || verified.VerifiedThrough < page.Items[0].Sequence {
		t.Fatal("audit verification", e)
	}
	if _, e = db.AuditSummaries(ctx, caseOwner, "", 20); e == nil {
		t.Fatal("physician accessed audit")
	}
	if _, e = db.AuditSummaries(ctx, caseAdmin, "invalid", 20); e == nil {
		t.Fatal("bad cursor")
	}
	if _, e = db.SystemStatus(ctx, caseReviewer); e == nil {
		t.Fatal("non-admin status access")
	}
	status, e := db.SystemStatus(ctx, caseAdmin)
	if e != nil || status.Database != "ready" {
		t.Fatal("status", e)
	}
	if _, e = db.Observation(ctx, casePeer, p.ID, "missing"); e == nil {
		t.Fatal("observation access bypass")
	}
}

func TestAuditMetadataRejectsCorruption(t *testing.T) {
	db := historyDB(t)
	ctx := t.Context()
	if _, e := db.Pool.Exec(ctx, `ALTER TABLE audit_events DISABLE TRIGGER immutable_audit; UPDATE audit_events SET payload=jsonb_set(payload,'{kind}','"tampered"') WHERE sequence=1`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.AuditSummaries(ctx, caseAdmin, "", 50); e == nil {
		t.Fatal("corrupt metadata page reported valid")
	}
	if _, e := db.VerifyAudit(ctx, caseAdmin); e == nil {
		t.Fatal("corrupt chain verified")
	}
}
