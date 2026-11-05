CREATE TABLE app_users (
 id text PRIMARY KEY, username text UNIQUE NOT NULL, role text NOT NULL CHECK(role IN ('physician','pharmacist','admin')),
 password_hash text NOT NULL DEFAULT '', active boolean NOT NULL DEFAULT true,
 legacy_allowed boolean NOT NULL DEFAULT true, must_change boolean NOT NULL DEFAULT true,
 reset_hash text NOT NULL DEFAULT '', reset_expires timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE auth_sessions (
 token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES app_users(id),
 created_at timestamptz NOT NULL DEFAULT now(), last_seen timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '8 hours'
);
CREATE INDEX auth_sessions_user_idx ON auth_sessions(user_id);
CREATE TABLE login_limits (
 key text PRIMARY KEY, attempts integer NOT NULL, window_start timestamptz NOT NULL DEFAULT now()
);
