# Staff accounts and sessions

Phase 6 adds database-backed staff login to the existing local academic application. Existing actor IDs, assessments, source approvals and audit records are preserved by additive migration 003. Provisioning does not overwrite managed roles, passwords or disabled states. Source-key access remains an explicit transition for scripts and existing local operators.

## First password-based administrator

After bootstrap and Compose startup:

```sh
python3 scripts/setup-accounts.py workspace-admin --role admin \
  --output .local/workspace-admin-activation.json
```

The command uses the existing local administrator service key to create a new staff identity, with no default password. It reserves a private 0600 output file and refuses to overwrite it. Open that file locally, then use **Activate or reset an account** at http://127.0.0.1:8080. Enter its username and activation secret, and choose a password of at least 15 characters. Sign in through the normal username/password form.

Secrets expire after 30 minutes and can be used once. If activation expires, open **Local service key · migration access**, sign in with the original administrator key, open **Staff accounts** and issue a new activation for the account. Share activation secrets privately. Passwords and secrets must not be committed to the repository.

## Staff administration

Administrators can create staff accounts, issue activation/reset secrets, change roles and disable/enable accounts. Only pharmacists can approve clinical rules. Disabling/demoting the last active administrator is rejected.

Reset immediately clears the password, disables that identity's legacy key and revokes all sessions; activation sets the new password. Role changes and disabling revoke sessions and disable the legacy key. Changing your own password requires the current password, revokes sessions and disables the legacy key. Updating an account's role/active state revokes its sessions; an affected browser must sign in again. The original configured administrator key is retained unless that specific account is reset, disabled, demoted or changes its password. No public registration or email delivery is required.

Activation/reset uses a one-time random secret rather than a temporary password. The browser shows new secrets only in the administration screen; clear them by signing out or navigating away/reloading. A local operator should retain access to the original private administrator credentials during migration. Losing all administrator credentials requires privileged operator recovery; no unauthenticated recovery backdoor exists.

## Session behavior

- Argon2id password hashes with random salts, 19 MiB memory, two iterations, one lane; passwords bounded to 15–256 bytes on the server. Parameters are fixed in this version and need benchmarking before deployment changes.
- Opaque 256-bit session identifiers; only hashes are stored. Cookie is HttpOnly, SameSite=Lax, path `/`, eight-hour maximum age; Secure under direct HTTPS. HTTP login/activation is limited to a loopback host. Proxy TLS termination is not inferred from forwarded headers.
- Server idle expiry is 30 minutes and absolute expiry eight hours. These defaults are currently fixed; configurable timeout policy is a follow-up refinement.
- Session is restored after page reload through `/auth/session`; CSRF token is held in memory. All cookie-authenticated mutation endpoints require `X-CSRF-Token`. Cross-origin requests are rejected by the existing origin guard.
- Supplying both cookie and bearer identities is rejected. Account status and role are read from the database; authorised database writes recheck the credential inside the write transaction so revocation cannot be bypassed by an already-running request.
- Logout revokes the server session. An invalid session hides the workspace on the next API request. The browser does not persist credentials in localStorage/sessionStorage; signing out clears displayed results and secrets. Multiple tabs learn revocation on their next request; immediate cross-tab clearing is not implemented.
- Login attempt counters are durable fixed 15-minute windows: 10 per username, 60 per source address. Failures use the same public error. Source address is the direct connection address; no trust is placed in arbitrary forwarding headers. This conservative local policy can affect users sharing one address.

No hospital identity federation, MFA, email recovery or real-patient deployment is asserted by this academic account implementation.

## Verification

`python3 scripts/test-integration.py` runs account lifecycle, activation reuse/expiry, legacy revocation, stable actor provisioning, role restrictions, last-admin protection, session idle/absolute expiry, password change, pending-write revocation, CSRF and cookie transport tests in an isolated disposable database. `scripts/smoke-accounts.cjs` creates a disposable physician account, exercises browser activation/login/reload/logout, then disables the account. It prints no credentials. Existing clinical workflow smoke remains separate.

A disabled smoke account and its audit events remain in the local application; they are intentionally not deleted from history. Never run browser checks against real patient data.

Commit verification also checks that an unactivated administrator without usable credentials cannot count as a replacement when disabling or demoting the last usable administrator.
