package store

import (
	"context"
	"github.com/jackc/pgx/v5"
)

var demoRoles = []string{"physician", "pharmacist", "admin"}

// PrepareDemoAccounts reserves passwordless demo-only identities and revokes old
// demo sessions on every startup, including when the feature is switched off.
func (s *Store) PrepareDemoAccounts(ctx context.Context, enabled bool) error {
	return s.write(ctx, func(tx pgx.Tx) error {
		for _, role := range demoRoles {
			id := "demo-" + role
			if _, err := tx.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, id); err != nil {
				return err
			}
			if !enabled {
				continue
			}
			tag, err := tx.Exec(ctx, `INSERT INTO app_users(id,username,role,legacy_allowed,must_change) VALUES($1,$1,$2,false,false) ON CONFLICT DO NOTHING`, id, role)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				if _, err = appendAudit(ctx, tx, Actor{ID: "account-provisioner", Role: "system"}, "ACCOUNT_PROVISIONED", id, Actor{ID: id, Role: role}); err != nil {
					return err
				}
			}
			var ok bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_users WHERE id=$1 AND username=$1 AND role=$2 AND NOT legacy_allowed AND password_hash='' AND NOT must_change)`, id, role).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return ErrInvalid
			}
		}
		return nil
	})
}

func (s *Store) DemoLogin(ctx context.Context, role string) (string, Session, error) {
	if !validRole(role) {
		return "", Session{}, ErrForbidden
	}
	id, raw := "demo-"+role, token()
	err := s.write(ctx, func(tx pgx.Tx) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_users WHERE id=$1 AND username=$1 AND role=$2 AND active AND NOT legacy_allowed AND password_hash='' AND NOT must_change FOR UPDATE)`, id, role).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrForbidden
		}
		if _, err := tx.Exec(ctx, `INSERT INTO auth_sessions(token_hash,user_id) VALUES($1,$2)`, tokenHash(raw), id); err != nil {
			return err
		}
		_, err := appendAudit(ctx, tx, Actor{ID: id, Role: role}, "LOGIN", id, map[string]string{"status": "demo_session_started"})
		return err
	})
	if err != nil {
		return "", Session{}, err
	}
	return raw, Session{Username: id, Actor: Actor{ID: id, Role: role}, CSRF: CSRF(raw)}, nil
}
