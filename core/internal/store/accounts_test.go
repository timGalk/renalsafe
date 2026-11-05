package store_test

import (
	"renalsafe/core/internal/dbtest"
	"renalsafe/core/internal/store"
	"testing"
)

const staffPassword = "synthetic-account-password-2026"

func TestAccountLifecycle(t *testing.T) {
	db := dbtest.Database(t)
	ctx := t.Context()
	admin := store.Actor{ID: "admin", Role: "admin"}
	doc := store.Actor{ID: "doctor", Role: "physician"}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	must(db.Provision(ctx, []store.Actor{admin, doc}))
	must(db.Provision(ctx, []store.Actor{{ID: "doctor", Role: "admin"}}))
	a, e := db.LegacyActor(ctx, doc)
	must(e)
	if a.Role != "physician" {
		t.Fatal("provision overwrote role")
	}
	if _, e = db.ManageUser(ctx, admin, "admin", "admin", "physician", true); e == nil {
		t.Fatal("last admin demoted")
	}
	secret, e := db.ResetUser(ctx, admin, "doctor")
	must(e)
	if _, e = db.LegacyActor(ctx, doc); e == nil {
		t.Fatal("reset left legacy key active")
	}
	must(db.Activate(ctx, "doctor", secret, staffPassword))
	if e = db.Activate(ctx, "doctor", secret, staffPassword); e == nil {
		t.Fatal("activation reused")
	}
	if _, _, e = db.Login(ctx, "doctor", "wrong", "ip"); e == nil {
		t.Fatal("bad password accepted")
	}
	raw, session, e := db.Login(ctx, "doctor", staffPassword, "ip")
	must(e)
	if session.Actor != doc || session.CSRF == "" {
		t.Fatal("wrong identity")
	}
	_, e = db.Session(ctx, raw)
	must(e)
	must(db.Logout(ctx, raw, doc))
	if _, e = db.Session(ctx, raw); e == nil {
		t.Fatal("logout not revoked")
	}
	raw, _, e = db.Login(ctx, "doctor", staffPassword, "ip")
	must(e)
	_, e = db.ManageUser(ctx, admin, "doctor", "doctor", "pharmacist", true)
	must(e)
	if _, e = db.Session(ctx, raw); e == nil {
		t.Fatal("role change left session")
	}
	raw, _, e = db.Login(ctx, "doctor", staffPassword, "ip")
	must(e)
	_, e = db.Pool.Exec(ctx, `UPDATE auth_sessions SET last_seen=now()-interval '31 minutes'`)
	must(e)
	if _, e = db.Session(ctx, raw); e == nil {
		t.Fatal("idle expiry ignored")
	}
	raw, _, e = db.Login(ctx, "doctor", staffPassword, "ip")
	must(e)
	_, e = db.Pool.Exec(ctx, `UPDATE auth_sessions SET expires_at=now()-interval '1 second'`)
	must(e)
	if _, e = db.Session(ctx, raw); e == nil {
		t.Fatal("absolute expiry ignored")
	}
	raw, _, e = db.Login(ctx, "doctor", staffPassword, "ip")
	must(e)
	current := store.Actor{ID: "doctor", Role: "pharmacist"}
	must(db.ChangePassword(ctx, current, staffPassword, staffPassword+"-new"))
	if _, e = db.Session(ctx, raw); e == nil {
		t.Fatal("password change left session")
	}
	// Authorisation captured before revocation cannot commit a subsequent write.
	raw, _, e = db.Login(ctx, "doctor", staffPassword+"-new", "ip")
	must(e)
	authorised := store.AuthorizeContext(ctx, current, raw)
	_, e = db.ManageUser(ctx, admin, "doctor", "doctor", "pharmacist", false)
	must(e)
	if _, e = db.Assess(authorised, current, dbtest.CurrentRequest()); e == nil {
		t.Fatal("revoked writer committed")
	}
	if _, _, e = db.Login(ctx, "doctor", staffPassword+"-new", "ip"); e == nil {
		t.Fatal("disabled login accepted")
	}
	users, e := db.Users(ctx, admin)
	must(e)
	if len(users) != 2 {
		t.Fatal(users)
	}
	events, e := db.Audit(ctx)
	must(e)
	if len(events) == 0 {
		t.Fatal("missing audit")
	}
}
func TestLoginThrottleAndActivationExpiry(t *testing.T) {
	db := dbtest.Database(t)
	ctx := t.Context()
	admin := store.Actor{ID: "admin", Role: "admin"}
	if e := db.Provision(ctx, []store.Actor{admin}); e != nil {
		t.Fatal(e)
	}
	secret, e := db.ResetUser(ctx, admin, "admin")
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Pool.Exec(ctx, `UPDATE app_users SET reset_expires=now()-interval '1 second'`)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Activate(ctx, "admin", secret, staffPassword); e == nil {
		t.Fatal("expired activation")
	}
	for i := 0; i < 11; i++ {
		if _, _, e = db.Login(ctx, "unknown", "incorrect", "ip"); e == nil {
			t.Fatal("unknown login")
		}
	}
	var n int
	if e = db.Pool.QueryRow(ctx, `SELECT max(attempts) FROM login_limits`).Scan(&n); e != nil || n != 11 {
		t.Fatal("limits not durable", e, n)
	}
}

func TestConcurrentActivationHasOneWinner(t *testing.T) {
	db := dbtest.Database(t)
	ctx := t.Context()
	a := store.Actor{ID: "admin", Role: "admin"}
	if e := db.Provision(ctx, []store.Actor{a}); e != nil {
		t.Fatal(e)
	}
	secret, e := db.ResetUser(ctx, a, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	outcomes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { outcomes <- db.Activate(ctx, a.ID, secret, staffPassword) }()
	}
	wins := 0
	for i := 0; i < 2; i++ {
		if <-outcomes == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected one activation winner, got %d", wins)
	}
}

func TestPendingAdministratorCannotReplaceLastUsableAdmin(t *testing.T) {
	db := dbtest.Database(t)
	ctx := t.Context()
	admin := store.Actor{ID: "admin", Role: "admin"}
	if e := db.Provision(ctx, []store.Actor{admin}); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ManageUser(ctx, admin, "", "pending-admin", "admin", true); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ManageUser(ctx, admin, admin.ID, "admin", "physician", true); e == nil {
		t.Fatal("pending administrator allowed last usable admin demotion")
	}
	if _, e := db.ManageUser(ctx, admin, admin.ID, "admin", "admin", false); e == nil {
		t.Fatal("pending administrator allowed last usable admin disabling")
	}
}
