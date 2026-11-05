package store_test

import (
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/store"
	"sync"
	"testing"
	"time"
)

var caseOwner = store.Actor{ID: "case-doctor", Role: "physician"}
var casePeer = store.Actor{ID: "case-peer", Role: "physician"}
var caseReviewer = store.Actor{ID: "case-pharmacist", Role: "pharmacist"}
var caseAdmin = store.Actor{ID: "case-admin", Role: "admin"}

func historyDB(t *testing.T) *store.Store {
	t.Helper()
	db := dbtest.Database(t)
	if e := db.Provision(t.Context(), []store.Actor{caseOwner, casePeer, caseReviewer, caseAdmin}); e != nil {
		t.Fatal(e)
	}
	return db
}
func createCase(t *testing.T, db *store.Store) store.SyntheticPatient {
	t.Helper()
	p, e := db.CreatePatient(t.Context(), caseOwner, store.PatientInput{Alias: "Synthetic case", Synthetic: true})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func caseObservation(t *testing.T, db *store.Store, p store.SyntheticPatient) store.Observation {
	t.Helper()
	yes := true
	o, e := db.AddObservation(t.Context(), caseOwner, p.ID, store.ObservationInput{Creatinine: domain.Quantity{Value: 3, Unit: "mg/dL"}, MeasuredAt: time.Now().UTC().Add(-time.Hour), IDMS: &yes})
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func TestPatientPermissionsAndImmutableHistory(t *testing.T) {
	db := historyDB(t)
	ctx := t.Context()
	p := createCase(t, db)
	o := caseObservation(t, db, p)
	if _, e := db.Patient(ctx, casePeer, p.ID); e == nil {
		t.Fatal("cross-user patient read")
	}
	page, e := db.Patients(ctx, casePeer, "", 20)
	if e != nil || len(page.Items) != 0 {
		t.Fatal("patient list leakage", e)
	}
	if e = db.SetPatientAccess(ctx, caseOwner, p.ID, caseReviewer.ID, true); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Patient(ctx, caseReviewer, p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.AddObservation(ctx, caseReviewer, p.ID, o.Input); e == nil {
		t.Fatal("pharmacist wrote lab")
	}
	in := store.PatientAssessmentInput{ObservationID: o.ID, RetryKey: store.ID(), Request: dbtest.CurrentRequest()}
	in.Request.Patient.Creatinine.Value = 999
	r, e := db.AssessPatient(ctx, caseOwner, p.ID, in)
	if e != nil {
		t.Fatal(e)
	}
	if r.Assessment.Request.Patient.Creatinine.Value != 3 {
		t.Fatal("client overrode selected lab")
	}
	if again, e := db.AssessPatient(ctx, caseOwner, p.ID, in); e != nil || again.ID != r.ID {
		t.Fatal("retry not idempotent", e)
	}
	changed := in
	changed.Request.Patient.Age++
	if _, e = db.AssessPatient(ctx, caseOwner, p.ID, changed); e == nil {
		t.Fatal("retry-key body conflict accepted")
	}
	if ok, e := db.Replay(ctx, caseReviewer, r.ID); e != nil || !ok {
		t.Fatal("shared replay failed", e)
	}
	if _, e = db.ExportAssessment(ctx, casePeer, r.ID); e == nil {
		t.Fatal("cross-user export")
	}
	if _, e = db.ExportAssessment(ctx, caseReviewer, r.ID); e != nil {
		t.Fatal("shared export", e)
	}
	if e = db.SetPatientAccess(ctx, caseOwner, p.ID, caseReviewer.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Assessment(ctx, caseReviewer, r.ID); e == nil {
		t.Fatal("revoked reviewer read")
	}
	if e = db.SetPatientAccess(ctx, caseOwner, p.ID, casePeer.ID, true); e != nil {
		t.Fatal(e)
	}
	peerIn := in
	peerIn.RetryKey = store.ID()
	peerResult, e := db.AssessPatient(ctx, casePeer, p.ID, peerIn)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.SetPatientAccess(ctx, caseOwner, p.ID, casePeer.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Assessment(ctx, casePeer, peerResult.ID); e == nil {
		t.Fatal("creator bypassed revoked patient access")
	}
	corrected := o.Input
	corrected.CorrectsID = o.ID
	corrected.Reason = "Synthetic transcription correction"
	corrected.Creatinine.Value = 2
	newO, e := db.AddObservation(ctx, caseOwner, p.ID, corrected)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.AddObservation(ctx, caseOwner, p.ID, corrected); e == nil {
		t.Fatal("forked correction accepted")
	}
	labs, e := db.Observations(ctx, caseOwner, p.ID, "", 20)
	if e != nil || len(labs.Items) != 2 {
		t.Fatal("correction history missing", e)
	}
	found := false
	for _, lab := range labs.Items {
		if lab.ID == o.ID {
			found = lab.SupersededByID == newO.ID
		}
	}
	if !found {
		t.Fatal("superseded lab status missing")
	}
	if ok, e := db.Replay(ctx, caseOwner, r.ID); e != nil || !ok {
		t.Fatal("correction changed prior result", e)
	}
	stale := in
	stale.RetryKey = store.ID()
	if _, e = db.AssessPatient(ctx, caseOwner, p.ID, stale); e == nil {
		t.Fatal("corrected lab reused for new assessment")
	}
	p2 := createCase(t, db)
	wrong := in
	wrong.RetryKey = store.ID()
	if _, e = db.AssessPatient(ctx, caseOwner, p2.ID, wrong); e == nil {
		t.Fatal("wrong-patient sample linked")
	}
	next := in
	next.RetryKey = store.ID()
	next.ObservationID = newO.ID
	next.PreviousID = r.ID
	nr, e := db.AssessPatient(ctx, caseOwner, p.ID, next)
	if e != nil {
		t.Fatal(e)
	}
	if nr.ID == r.ID || nr.Assessment.Request.Patient.Creatinine.Value != 2 {
		t.Fatal("recalculation not new snapshot")
	}
	if _, e = db.Pool.Exec(ctx, `UPDATE renal_observations SET payload='{}' WHERE id=$1`, o.ID); e == nil {
		t.Fatal("observation not immutable")
	}
	if _, e = db.Pool.Exec(ctx, `UPDATE patient_assessments SET patient_id=$1 WHERE assessment_id=$2`, p2.ID, r.ID); e == nil {
		t.Fatal("link not immutable")
	}
	archived, e := db.UpdatePatient(ctx, caseOwner, p.ID, store.PatientUpdate{Alias: p.Alias, Archived: true, ExpectedVersion: p.Version})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.UpdatePatient(ctx, caseOwner, p.ID, store.PatientUpdate{Alias: p.Alias, ExpectedVersion: p.Version}); e == nil {
		t.Fatal("stale version accepted")
	}
	if !archived.Archived {
		t.Fatal("not archived")
	}
	next.RetryKey = store.ID()
	if _, e = db.AssessPatient(ctx, caseOwner, p.ID, next); e == nil {
		t.Fatal("archived case accepted new result")
	}
	if _, e = db.AddObservation(ctx, caseOwner, p.ID, o.Input); e == nil {
		t.Fatal("archived case accepted sample")
	}
	if _, e = db.Assessment(ctx, caseOwner, r.ID); e != nil {
		t.Fatal("archive hid history", e)
	}
}
func TestPatientAssessmentAtomicityConcurrencyAndPagination(t *testing.T) {
	db := historyDB(t)
	ctx := t.Context()
	p := createCase(t, db)
	o := caseObservation(t, db, p)
	in := store.PatientAssessmentInput{ObservationID: o.ID, RetryKey: store.ID(), Request: dbtest.CurrentRequest()}
	var wg sync.WaitGroup
	ids := make(chan string, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := db.AssessPatient(ctx, caseOwner, p.ID, in); ids <- r.ID; errs <- e }()
	}
	wg.Wait()
	for i := 0; i < 2; i++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	first := <-ids
	if second := <-ids; first != second {
		t.Fatal("concurrent retry duplicated")
	}
	in.RetryKey = store.ID()
	in.Request.At = in.Request.At.Add(time.Second)
	r2, e := db.AssessPatient(ctx, caseOwner, p.ID, in)
	if e != nil {
		t.Fatal(e)
	}
	firstPage, e := db.AssessmentHistory(ctx, caseOwner, store.HistoryFilter{PatientID: p.ID, Limit: 1})
	if e != nil || len(firstPage.Items) != 1 || firstPage.Next == "" {
		t.Fatal("history page", e)
	}
	secondPage, e := db.AssessmentHistory(ctx, caseOwner, store.HistoryFilter{PatientID: p.ID, Limit: 1, Cursor: firstPage.Next})
	if e != nil || len(secondPage.Items) != 1 || secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatal("history cursor repeats", e)
	}
	_ = r2
	legacy, e := db.Assess(ctx, caseOwner, dbtest.CurrentRequest())
	if e != nil {
		t.Fatal(e)
	}
	unlinked, e := db.AssessmentHistory(ctx, caseOwner, store.HistoryFilter{Unlinked: true})
	if e != nil || len(unlinked.Items) != 1 || unlinked.Items[0].ID != legacy.ID {
		t.Fatal("legacy history missing", e)
	}
	if _, e = db.AssessmentHistory(ctx, casePeer, store.HistoryFilter{PatientID: p.ID}); e == nil {
		t.Fatal("patient history permission bypass")
	}
	eventsBefore, e := db.Audit(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var before int
	if e = db.Pool.QueryRow(ctx, `SELECT count(*) FROM assessments`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	_, e = db.Pool.Exec(ctx, `CREATE FUNCTION reject_case_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture rollback'; END $$; CREATE TRIGGER reject_case_link BEFORE INSERT ON patient_assessments FOR EACH ROW EXECUTE FUNCTION reject_case_link()`)
	if e != nil {
		t.Fatal(e)
	}
	in.RetryKey = store.ID()
	if _, e = db.AssessPatient(ctx, caseOwner, p.ID, in); e == nil {
		t.Fatal("link failure committed")
	}
	var after int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM assessments`).Scan(&after)
	if before != after {
		t.Fatal("orphan assessment after rollback")
	}
	eventsAfter, e := db.Audit(ctx)
	if e != nil || len(eventsAfter) != len(eventsBefore) {
		t.Fatal("audit was not atomic with failed link", e)
	}
	if _, e = db.Patients(ctx, caseOwner, "bad cursor", 20); e == nil {
		t.Fatal("invalid cursor accepted")
	}
}

func TestPatientPaginationRoleChangesAndReopening(t *testing.T) {
	db := historyDB(t)
	ctx := t.Context()
	p := createCase(t, db)
	_ = createCase(t, db)
	cases, e := db.Patients(ctx, caseOwner, "", 1)
	if e != nil || len(cases.Items) != 1 || cases.Next == "" {
		t.Fatal("case pagination", e)
	}
	more, e := db.Patients(ctx, caseOwner, cases.Next, 1)
	if e != nil || len(more.Items) != 1 || more.Items[0].ID == cases.Items[0].ID {
		t.Fatal("case cursor repeated", e)
	}
	measured := time.Now().Add(-time.Hour).Truncate(time.Second).Add(123456789 * time.Nanosecond)
	for i := 0; i < 2; i++ {
		if _, e = db.AddObservation(ctx, caseOwner, p.ID, store.ObservationInput{Creatinine: domain.Quantity{Value: 2, Unit: "mg/dL"}, MeasuredAt: measured}); e != nil {
			t.Fatal(e)
		}
	}
	labs, e := db.Observations(ctx, caseOwner, p.ID, "", 1)
	if e != nil || len(labs.Items) != 1 || labs.Next == "" {
		t.Fatal("lab pagination", e)
	}
	next, e := db.Observations(ctx, caseOwner, p.ID, labs.Next, 1)
	if e != nil || len(next.Items) != 1 || next.Items[0].ID == labs.Items[0].ID {
		t.Fatal("nanosecond sample cursor repeated", e)
	}
	reopened, e := store.Open(ctx, db.Pool.Config().ConnString(), true)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	loaded, e := reopened.Patient(ctx, caseOwner, p.ID)
	if e != nil || loaded.Code != p.Code {
		t.Fatal("case not durable across store reopen", e)
	}
	if _, e = db.ManageUser(ctx, caseAdmin, caseOwner.ID, caseOwner.ID, "pharmacist", true); e != nil {
		t.Fatal(e)
	}
	demoted := store.Actor{ID: caseOwner.ID, Role: "pharmacist"}
	if _, e = db.UpdatePatient(ctx, demoted, p.ID, store.PatientUpdate{Alias: "changed", ExpectedVersion: p.Version}); e == nil {
		t.Fatal("demoted owner managed patient")
	}
	if e = db.SetPatientAccess(ctx, demoted, p.ID, casePeer.ID, true); e == nil {
		t.Fatal("demoted owner granted patient access")
	}
	if _, e = db.Patient(ctx, demoted, p.ID); e != nil {
		t.Fatal("demoted owner lost read-only access", e)
	}
}
