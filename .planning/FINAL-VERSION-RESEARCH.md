# Final-version research reconciliation

Date: 2026-11-05. Purpose: reconcile earlier design with current implementation before planning accounts, synthetic patients and final delivery. This is planning research, not a new test run.

## Evidence already available

| Evidence | Finding | Planning consequence |
|---|---|---|
| `../Engineering-thesis/RenalSafe_Architecture_Plan.tex`, component/API/data-model/stack sections | Go API, staff sessions, `POST /auth/login`, `app_user(login,password hash,role)`, React/TypeScript, immutable assessment inputs and per-drug outputs were planned | Normal account login is unfinished original scope, not a new invention |
| Same architecture, data model and privacy sections | Assessment inputs have no identifiers; synthetic patients only; no patient account or longitudinal patient entity specified | User-confirmed synthetic patient history is an explicit extension; no patient-facing portal |
| `docs/ADR-002-main-system.md` and Phase 3 CONTEXT/RESEARCH | Plain embedded browser, local bearer keys, Go/Poppler ingestion and direct PostgreSQL retrieval accepted for the runnable slice | Reuse backend; React migration can be incremental. Python worker is remaining scope, not a prerequisite for login |
| `core/internal/auth/auth.go` | Static credential hashes loaded from file; request binds server-known identity and role | Current “login” is key entry, with no passwords, sessions, expiry or account administration |
| `core/internal/store/store.go`, `models.go`, migrations 001/002 | Assessments store inputs/results/knowledge snapshots and audit atomically; creator/admin read permission; historical replay | Reuse result persistence. Add lists and patient associations without rewriting old payloads |
| `core/internal/domain/model.go` | Patient is an assessment input value, not an identified longitudinal record | Add a separate synthetic case/profile model; keep calculation inputs explicit and immutable |
| `core/internal/httpapi/server.go`, `web/app.js` | Assessment creation/detail/replay, evidence, documents, rule review exist; result UI is session-local; no history/list endpoints | Patient list, timeline and reopening saved results are genuinely missing |
| `docs/system-verification.md`, `docs/evaluation-guide.md` | Database/browser checks and six lexical software fixtures passed in earlier implementation work; real-model/independent gold evaluation remains open | Do not treat software fixture scores as thesis completion |

The original architecture also names a Python extraction worker, optional guarded explanations, BGE-M3 and MedGemma experiments. Those remain proposals or gated unfinished work. No authentication planning depends on choosing a clinical model.

## Focused current research

Primary references consulted 2026-11-05:

1. [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html): password hashes need a password-specific adaptive algorithm. Use Argon2id with per-password salts; validate parameters against current guidance and benchmark on the target machine during implementation. Existing SHA-256 token hashing is appropriate for random keys, not passwords.
2. [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html): use protected session identifiers, rotate at authentication and invalidate on logout/expiry. Proposed application: opaque server-side sessions with HttpOnly cookies, Secure on HTTPS, SameSite and server-enforced idle/absolute expiry; explicit CSRF protection for cookie-authenticated mutations. Local HTTP is a separately bounded loopback development mode.
3. [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html): generic login failures, throttling and reauthentication for sensitive changes. Proposed application: admin-provisioned staff accounts, forced change after temporary credentials/reset, revocation after password or role changes. No public sign-up/email delivery dependency for the local academic version.
4. [OWASP Authorization](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html): deny by default and check permissions for each request and object. Proposed application: patient membership plus staff role; a role alone must not expose another clinician's case. Check list/detail/lab/assessment/replay/export endpoints consistently.

These application choices are design inferences from the cited guidance and the existing architecture; OWASP does not prescribe our product workflow, timeout durations or permission matrix.

## Recommended decisions and scope

- Confirmed by user: clinician workspace with synthetic patient history; no patient-facing login.
- Restore the originally planned staff username/password login with server-side sessions. Keep an explicit, revocable service-key path only if scripts need it; disable the legacy browser key workflow in the final UI.
- Keep one local academic workspace. External identity providers, organisations/tenants and real-patient deployment are separate scope.
- Use React/TypeScript for the final UI, as originally projected, while preserving the verified Go service and HTTP contract. Exact package versions are selected/locked during implementation rather than guessed in this plan.
- Keep a narrow renal laboratory scope: creatinine observations with units/time/uncertainty and relevant input snapshots. A general hospital laboratory system was not researched or requested.
- Preserve the original role identities when moving existing users into the database. History ownership and pharmacist reviewer attribution must not change.
- Expose synthetic patient trends as history of observations/estimates; do not infer AKI, stable function or treatment efficacy from a chart.
- Distinguish the usable academic application release from complete thesis evaluation evidence. Neither label implies clinical deployment readiness.
