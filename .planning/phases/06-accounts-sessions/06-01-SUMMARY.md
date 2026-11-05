# Phase 6 software implementation — 2026-11-05

Implemented the staff account/session workflow from FINAL-VERSION-PLAN.md. New migration 003 adds users, session hashes, activation/reset metadata and durable login counters without rewriting existing clinical history. Existing configured actor IDs migrate idempotently; managed account changes survive restart. Arbitrary existing actor IDs receive a safe login username while retaining their historical identity.

Argon2id passwords, salted hashes, 256-bit random session/reset secrets, HttpOnly/SameSite cookie transport, Secure under direct HTTPS, explicit mutation CSRF and ambiguous identity rejection are implemented. Password sessions expire after 30 idle minutes or eight absolute hours. Authentication fails closed on inactive/revoked accounts. Database writes recheck permission under the transaction lock, preventing a previously authorised revoked request from committing.

Staff administration includes create, active state, role changes and single-use reset/activation, with last-active-admin protection and audit. Browser has normal username/password login, activation, own-password change and administrator account screens. Existing keys remain transitional for scripts. No secret is logged/audited or stored in browser persistent storage. Password activation replaces a temporary-password workflow; it sets the chosen password and consumes a 30-minute secret atomically.

Verification executed: full Go race/integration suite on a disposable PostgreSQL/pgvector database; vet; password salting/verification; activation expiry/reuse/concurrent use; idle/absolute session expiry; role/disable/reset/password/logout revocation; writer revocation; CSRF; HTTPS Secure cookie; ambiguous credentials. Browser activation/password login/reload/logout checks pass on a disposable staff identity. Existing assessment/evidence/replay/pharmacist-edit/cancel/mobile smoke passes with migration-key access. Compose rebuilt, migration applied, OpenAPI regenerated. No clinical rules approved.

Initial workspace-admin account prepared in the running local application; its activation details are in a private ignored local file. The operator chooses the password in the browser. Browser smoke identities remain disabled with audit history.

Limits: fixed timeout policy rather than configurable per deployment; no MFA/SSO/email recovery or privileged lost-admin recovery CLI; direct TLS/loopback transport only (no forwarded-header trust); other tabs clear on next API request. These are documented in docs/accounts.md. Academic software completion does not establish production identity or clinical validation. Next engineering phase: 7, synthetic patient/result history.

Commit verification also checks that an unactivated administrator without usable credentials cannot count as a replacement when disabling or demoting the last usable administrator.
