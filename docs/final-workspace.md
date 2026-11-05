# Final React workspace — Phase 8

The running application now uses the React/TypeScript workspace. The Go calculator, clinical rule eligibility, source storage and patient-history contracts remain authoritative. This phase completes the staff-facing interface; final release/backup/recovery work and independent clinical/model evidence remain separate gates.

## Sign in and activate

Open http://127.0.0.1:8080/login and sign in with a staff username and password. Activate a new/reset account at `/activate` using its private single-use activation secret. The browser no longer offers access-key login. Local service keys remain available to setup/demo/evaluation scripts during migration.

For first setup, use `scripts/setup-accounts.py` as documented in `accounts.md`. If an existing account's activation has expired, an operator with the original active local administrator key can explicitly revoke its credentials and renew activation:

```sh
python3 scripts/setup-accounts.py workspace-admin --reset-existing \
  --output .local/workspace-admin-renewed.json
```

The output must be a new private file; it cannot overwrite prior credentials. Open it locally and activate the named account with a chosen password. This operator action revokes that account's existing credentials. It is not an unauthenticated recovery path. Lost administrator credentials still require the Phase 9 operator-recovery procedure.

## Pages and roles

| Page | Route | Workflow |
|---|---|---|
| Overview | `/dashboard` | Recent assigned cases and assessments; no invented global totals |
| Patients | `/patients` | Case list, pagination and physician-only synthetic-case creation |
| Patient | `/patients/{id}` | Observations, corrections, sharing, archive and result timeline |
| New assessment | `/assess/new` | Physician-only explicit inputs; choose a linked observation from its case |
| Saved results | `/history` | Product/status/date filters, pagination and unlinked legacy history |
| Saved assessment | `/assessments/{id}` | Read-only authoritative snapshot, replay, evidence and reports |
| Comparison | `/compare?left={id}&right={id}` | Authorised snapshots linked to the same synthetic case |
| Evidence | `/evidence` | Exact-context source search; explicit retrieval mode/fallback |
| Review | `/review` | Pharmacist/admin candidate preparation/editing; pharmacist-only approve/reject |
| Documents | `/documents` | Pharmacist/admin PDF upload, scope, source download, version lifecycle and indexing |
| My account | `/account` | Current-password verification, password change and session revocation |
| Staff accounts | `/admin/users` | Administrator staff provisioning, role/active state and activation/reset |
| Audit | `/admin/audit` | Administrator paginated metadata and full-chain verification |
| Status | `/admin/status` | Administrator database/source/rule coverage and embedding configuration |

Direct links and browser refresh restore the same page after session verification. Patient/lab links identify records, never credentials. Unknown API, health and static-asset URLs do not receive the SPA shell. The existing public training page remains at `/demo.html`.

Patient-level permissions apply to every API request. Hiding navigation is additional guidance, not authorization. An unauthorised role visiting a staff-only route receives an access-restricted screen; unavailable/private records produce an error with a retry option without displaying another case's data.

## Inputs, navigation and session behavior

Forms show explicit loading, error, empty and pending-save states. A dirty form blocks route changes/back navigation with **Keep editing** or **Discard changes and continue**; reload/close uses the browser's unsaved-input warning. Successful saves clear the relevant draft flag before navigating. Inputs stay in memory only; discarding or ending a session clears them rather than storing them locally.

Patient-linked assessments fetch the selected stored observation through an authorised single-record endpoint, so deep links work even when the sample is on a later history page. Superseded/archived inputs cannot start a new assessment. Synthetic examples preserve selected lab values; the other clinical inputs must be confirmed afresh. Result pages show saved API values and citations; rounded display values do not change rule matching. Unknown measurement uncertainty is labeled as unknown.

Evidence results clear when query/context controls change, and stale search replies cannot overwrite a changed query. Candidate source selectors remain fixed during edits, expected digests protect concurrent saves, and choosing a new candidate source/passage resets its draft fields. Pharmacist approval still requires explicit complete-review confirmation; no generated candidate is auto-approved.

Authenticated requests are cancelled/invalidated on route or session changes. A failed/expired session clears the protected React tree and pending inputs, preserves the requested path for a new sign-in and explains the session change. Logout, password change and account sign-in changes notify other tabs on the same browser; external browser revocation/expiry is learned on the next authenticated request. No persistent browser patient/credential cache or bearer-key UI is introduced. Sessions retain the fixed backend idle/absolute policy documented in `accounts.md`.

## Audit and system status

`/api/v1/admin/audit` returns at most 50 event summaries per cursor page (20 by default), excluding clinical payloads. Returned page entries and adjacency are hash-checked. `/admin/audit/verify` streams the complete stored chain and reports the observed head/count; it does not claim external anchoring or professional signatures. Legacy full-audit API remains for existing scripts, but the UI does not fetch it.

Status reports actual database/source/rule counts and the configured local embedding identity. Configuration does not establish provider availability or model quality; search/indexing still report failures explicitly. Clinical/manual-review gates are unchanged.

## Build and verification

See `web/README.md` for source layout, locked dependencies, generated API types and host/Docker builds. Node is a build dependency; the deployed application still serves from one Go process plus PostgreSQL.

```sh
npm ci --prefix web
npm run check:types --prefix web
npm run build --prefix web
python3 scripts/test-integration.py
# In the configured Playwright environment:
node scripts/smoke-workspace.cjs
```

The complete browser check creates disposable staff identities, synthetic cases/results and a pending candidate clone that is rejected after checking the edit/save flow. It never approves or edits an original real clinical rule. Fixture accounts are disabled, and synthetic case/audit history is retained. Established `smoke-accounts.cjs`, `smoke-patients.cjs` and `smoke-browser.cjs` entrypoints select the corresponding React checks. Reports/screenshots go to ignored `outputs/`.
