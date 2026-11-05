package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/jackc/pgx/v5"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/password"
	"strings"
)

type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	Active     bool   `json:"active"`
	MustChange bool   `json:"must_change_password"`
}
type Session struct {
	Username   string `json:"username"`
	Actor      Actor  `json:"actor"`
	MustChange bool   `json:"must_change_password"`
	CSRF       string `json:"csrf_token"`
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func tokenHash(s string) string { return domain.BytesDigest([]byte(s)) }
func CSRF(token string) string  { return tokenHash("csrf:" + token) }
func validRole(s string) bool   { return s == "admin" || s == "physician" || s == "pharmacist" }
func validUsername(s string) bool {
	if len(s) < 3 || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", c)) {
			return false
		}
	}
	return true
}

// Provision preserves actor IDs, configured role changes never overwrite managed accounts.
func (s *Store) Provision(ctx context.Context, actors []Actor) error {
	return s.write(ctx, func(tx pgx.Tx) error {
		for _, a := range actors {
			if !validRole(a.Role) || a.ID == "" {
				return ErrInvalid
			}
			username := a.ID
			if !validUsername(username) {
				username = "staff-" + tokenHash(a.ID)[:16]
			}
			tag, e := tx.Exec(ctx, `INSERT INTO app_users(id,username,role) VALUES($1,$2,$3) ON CONFLICT(id) DO NOTHING`, a.ID, username, a.Role)
			if e != nil {
				return e
			}
			if tag.RowsAffected() > 0 {
				if _, e = appendAudit(ctx, tx, Actor{ID: "account-provisioner", Role: "system"}, "ACCOUNT_PROVISIONED", a.ID, a); e != nil {
					return e
				}
			}
		}
		return nil
	})
}
func (s *Store) LegacyActor(ctx context.Context, a Actor) (Actor, error) {
	var active, allowed bool
	e := s.Pool.QueryRow(ctx, `SELECT role,active,legacy_allowed FROM app_users WHERE id=$1`, a.ID).Scan(&a.Role, &active, &allowed)
	if e == pgx.ErrNoRows {
		return a, nil
	}
	if e != nil {
		return Actor{}, e
	}
	if !active || !allowed {
		return Actor{}, ErrForbidden
	}
	return a, nil
}
func (s *Store) Users(ctx context.Context, a Actor) ([]User, error) {
	if a.Role != "admin" {
		return nil, ErrForbidden
	}
	rows, e := s.Pool.Query(ctx, `SELECT id,username,role,active,must_change FROM app_users ORDER BY username LIMIT 500`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Username, &u.Role, &u.Active, &u.MustChange); e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Store) ManageUser(ctx context.Context, a Actor, id, username, role string, active bool) (User, error) {
	out := User{ID: id, Username: username, Role: role, Active: active}
	if a.Role != "admin" {
		return out, ErrForbidden
	}
	if !validUsername(username) || !validRole(role) {
		return out, ErrInvalid
	}
	e := s.write(ctx, func(tx pgx.Tx) error {
		if e := requireActive(ctx, tx, a, "admin"); e != nil {
			return e
		}
		if id == "" {
			out.ID = ID()
			out.MustChange = true
			_, e := tx.Exec(ctx, `INSERT INTO app_users(id,username,role,active,legacy_allowed) VALUES($1,$2,$3,$4,false)`, out.ID, username, role, active)
			if e != nil {
				return e
			}
		} else {
			var oldRole string
			var oldActive bool
			if e := tx.QueryRow(ctx, `SELECT role,active FROM app_users WHERE id=$1`, id).Scan(&oldRole, &oldActive); e != nil {
				return normalize(e)
			}
			if oldRole == "admin" && oldActive && (!active || role != "admin") {
				var n int
				if e := tx.QueryRow(ctx, `SELECT count(*) FROM app_users WHERE role='admin' AND active AND (password_hash<>'' OR legacy_allowed)`).Scan(&n); e != nil {
					return e
				}
				if n <= 1 {
					return ErrConflict
				}
			}
			if _, e := tx.Exec(ctx, `UPDATE app_users SET username=$2,legacy_allowed=CASE WHEN NOT $4 OR role<>$3 THEN false ELSE legacy_allowed END,role=$3,active=$4 WHERE id=$1`, id, username, role, active); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, id); e != nil {
				return e
			}
		}
		_, e := appendAudit(ctx, tx, a, "ACCOUNT_UPDATED", out.ID, out)
		return e
	})
	return out, e
}
func requireActive(ctx context.Context, q querier, a Actor, role string) error {
	var actual string
	var active bool
	if e := q.QueryRow(ctx, `SELECT role,active FROM app_users WHERE id=$1`, a.ID).Scan(&actual, &active); e != nil {
		return ErrForbidden
	}
	if !active || actual != a.Role || role != "" && actual != role {
		return ErrForbidden
	}
	return nil
}
func (s *Store) ResetUser(ctx context.Context, a Actor, id string) (string, error) {
	if a.Role != "admin" {
		return "", ErrForbidden
	}
	secret := token()
	e := s.write(ctx, func(tx pgx.Tx) error {
		if e := requireActive(ctx, tx, a, "admin"); e != nil {
			return e
		}
		tag, e := tx.Exec(ctx, `UPDATE app_users SET reset_hash=$2,reset_expires=now()+interval '30 minutes',password_hash='',legacy_allowed=false,must_change=true WHERE id=$1 AND active`, id, tokenHash(secret))
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrNotFound
		}
		if _, e = tx.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, id); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, a, "ACCOUNT_RESET", id, map[string]string{"status": "activation_required"})
		return e
	})
	return secret, e
}
func (s *Store) Activate(ctx context.Context, username, secret, next string) error {
	if len(secret) != 43 || !validUsername(username) {
		return ErrForbidden
	}
	var eligible bool
	if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_users WHERE username=$1 AND reset_hash=$2 AND reset_expires>now() AND active)`, username, tokenHash(secret)).Scan(&eligible); e != nil {
		return e
	}
	if !eligible {
		return ErrForbidden
	}
	hash, e := password.Hash(next)
	if e != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, e)
	}
	if len(secret) != 43 {
		return ErrForbidden
	}
	return s.write(ctx, func(tx pgx.Tx) error {
		var id string
		e := tx.QueryRow(ctx, `UPDATE app_users SET password_hash=$3,reset_hash='',reset_expires=NULL,must_change=false WHERE username=$1 AND reset_hash=$2 AND reset_expires>now() AND active RETURNING id`, username, tokenHash(secret), hash).Scan(&id)
		if e != nil {
			return ErrForbidden
		}
		_, e = appendAudit(ctx, tx, Actor{ID: id, Role: "account"}, "ACCOUNT_ACTIVATED", id, map[string]string{"status": "password_set"})
		return e
	})
}

// Throttles both account and source address, with bounded, durable fixed windows.
func (s *Store) Login(ctx context.Context, username, pass, ip string) (string, Session, error) {
	var out Session
	blocked := false
	e := s.write(ctx, func(tx pgx.Tx) error {
		for i, key := range []string{"account:" + username, "ip:" + ip} {
			var n int
			if e := tx.QueryRow(ctx, `INSERT INTO login_limits(key,attempts) VALUES($1,1) ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN login_limits.window_start<now()-interval '15 minutes' THEN 1 ELSE login_limits.attempts+1 END,window_start=CASE WHEN login_limits.window_start<now()-interval '15 minutes' THEN now() ELSE login_limits.window_start END RETURNING attempts`, tokenHash(key)).Scan(&n); e != nil {
				return e
			}
			limit := 10
			if i == 1 {
				limit = 60
			}
			blocked = blocked || n > limit
		}
		_, e := tx.Exec(ctx, `DELETE FROM login_limits WHERE window_start<now()-interval '1 day'`)
		return e
	})
	if e != nil {
		return "", out, e
	}
	if blocked {
		return "", out, ErrForbidden
	}
	var hash string
	var u User
	e = s.Pool.QueryRow(ctx, `SELECT id,role,password_hash,active,must_change FROM app_users WHERE username=$1`, username).Scan(&u.ID, &u.Role, &hash, &u.Active, &u.MustChange)
	// A fixed dummy hash path gives unknown/unactivated accounts equivalent KDF work.
	if e == pgx.ErrNoRows || hash == "" {
		hash = "argon2id$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	valid := password.Verify(hash, pass) && e == nil && u.Active
	raw := token()
	e = s.write(ctx, func(tx pgx.Tx) error {
		if !valid {
			_, e := appendAudit(ctx, tx, Actor{ID: "anonymous", Role: "anonymous"}, "LOGIN_FAILED", "", map[string]string{"account_hash": tokenHash(username)})
			return e
		}
		var current string
		var active bool
		if e := tx.QueryRow(ctx, `SELECT password_hash,active FROM app_users WHERE id=$1`, u.ID).Scan(&current, &active); e != nil {
			return e
		}
		if current != hash || !active {
			return ErrForbidden
		}
		if _, e := tx.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at<now() OR last_seen<now()-interval '30 minutes'`); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO auth_sessions(token_hash,user_id) VALUES($1,$2)`, tokenHash(raw), u.ID); e != nil {
			return e
		}
		_, e := appendAudit(ctx, tx, Actor{ID: u.ID, Role: u.Role}, "LOGIN", u.ID, map[string]string{"status": "session_started"})
		return e
	})
	if e != nil {
		return "", out, e
	}
	if !valid {
		return "", out, ErrForbidden
	}
	out = Session{Username: username, Actor: Actor{ID: u.ID, Role: u.Role}, MustChange: u.MustChange, CSRF: CSRF(raw)}
	return raw, out, nil
}
func (s *Store) Session(ctx context.Context, raw string) (Session, error) {
	var out Session
	if len(raw) != 43 {
		return out, ErrForbidden
	}
	e := s.Pool.QueryRow(ctx, `UPDATE auth_sessions s SET last_seen=now() FROM app_users u WHERE s.token_hash=$1 AND s.user_id=u.id AND u.active AND s.expires_at>now() AND s.last_seen>now()-interval '30 minutes' RETURNING u.id,u.role,u.must_change,u.username`, tokenHash(raw)).Scan(&out.Actor.ID, &out.Actor.Role, &out.MustChange, &out.Username)
	if e != nil {
		return out, ErrForbidden
	}
	out.CSRF = CSRF(raw)
	return out, nil
}
func (s *Store) Logout(ctx context.Context, raw string, a Actor) error {
	return s.write(ctx, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash=$1`, tokenHash(raw)); e != nil {
			return e
		}
		_, e := appendAudit(ctx, tx, a, "LOGOUT", a.ID, map[string]string{"status": "revoked"})
		return e
	})
}
func (s *Store) ChangePassword(ctx context.Context, a Actor, current, next string) error {
	hash, e := password.Hash(next)
	if e != nil {
		return ErrInvalid
	}
	var old string
	if e = s.Pool.QueryRow(ctx, `SELECT password_hash FROM app_users WHERE id=$1 AND active`, a.ID).Scan(&old); e != nil || !password.Verify(old, current) {
		return ErrForbidden
	}
	return s.write(ctx, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE app_users SET password_hash=$2,must_change=false,legacy_allowed=false WHERE id=$1 AND password_hash=$3 AND active`, a.ID, hash, old)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, e = tx.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, a.ID); e != nil {
			return e
		}
		_, e = appendAudit(ctx, tx, a, "PASSWORD_CHANGED", a.ID, map[string]string{"status": "sessions_revoked"})
		return e
	})
}

type authContextKey struct{}
type authContext struct {
	Actor Actor
	Raw   string
}

func AuthorizeContext(ctx context.Context, a Actor, raw string) context.Context {
	return context.WithValue(ctx, authContextKey{}, authContext{a, raw})
}
func checkWriteAuthorization(ctx context.Context, tx pgx.Tx) error {
	v, ok := ctx.Value(authContextKey{}).(authContext)
	if !ok {
		return nil
	}
	if v.Raw != "" {
		var allowed bool
		e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_sessions s JOIN app_users u ON u.id=s.user_id WHERE s.token_hash=$1 AND u.id=$2 AND u.role=$3 AND u.active AND s.expires_at>now() AND s.last_seen>now()-interval '30 minutes')`, tokenHash(v.Raw), v.Actor.ID, v.Actor.Role).Scan(&allowed)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrForbidden
		}
		return nil
	}
	var role string
	var active, legacy bool
	e := tx.QueryRow(ctx, `SELECT role,active,legacy_allowed FROM app_users WHERE id=$1`, v.Actor.ID).Scan(&role, &active, &legacy)
	if e == pgx.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if !active || !legacy || role != v.Actor.Role {
		return ErrForbidden
	}
	return nil
}
