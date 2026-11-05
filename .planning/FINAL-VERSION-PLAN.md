# RenalSafe final-version implementation plan

Date: 2026-11-05. Status: planned, not implemented in this planning turn.

## Target and confirmed scope

Deliver a complete local academic clinician workspace: staff login → synthetic patient list → patient detail and lab history → new renal assessment → saved results with source evidence → reopen, compare, replay and print/export results. Pharmacists retain the separate source/rule review workflow; administrators manage staff and inspect audit/system status.

The user explicitly selected clinician-managed synthetic patient history, with no patient-facing portal. Continue the original Go/PostgreSQL/pgvector architecture and final React/TypeScript UI. Use the existing calculator, engine, source storage, review APIs and immutable result snapshots. This plan extends the original identifier-free assessments with synthetic case association; it does not introduce real patient identifiers.

Research/evidence: `FINAL-VERSION-RESEARCH.md`. Existing Phases 2, 4 and 5 stay open; new Phases 6–9 add the missing product workflow and final delivery gates.

## Current state versus final behavior

| Area | Implemented | Final addition |
|---|---|---|
| Login | File-provisioned high-entropy bearer keys; physician/pharmacist/admin roles | Staff accounts, password login, sessions, logout, expiry, reset, disable/role management |
| Patient input | One assessment's age, equation sex, renal input and medications | Synthetic case ID/alias, ownership, lab timeline, explicitly chosen inputs |
| Results | Durable assessment snapshots, creator/admin access and replay | Searchable history, patient linkage, result detail routes, comparison and export |
| Frontend | Embedded plain browser app; assessment/evidence/documents/review | React/TypeScript workspace, stable links, navigation, accessibility and complete loading/error/empty states |
| Knowledge and AI | Manual candidates, review, lexical/optional hybrid retrieval, evaluation runner | Reviewed coverage, pending-only assisted extraction, harder-PDF evidence and actual model experiments |
| Delivery | Compose, integration tests, browser smoke | Account/data migration, backup/restore verification, complete end-to-end release checks and final report |

## Product decisions for implementation

### Staff accounts and access

Username/password sign-in, administrator-created accounts and one-time temporary passwords with forced change. Password change requires current credentials; administrator reset revokes sessions and issues an expiring single-use activation/reset secret shown only at issuance. Do not store plaintext passwords, reset secrets or session tokens. Initial administrator is provisioned through an explicit bootstrap command without a fixed public default password. Prevent disabling/demoting the last active administrator.

Store Argon2id password hashes and randomly generated opaque sessions in PostgreSQL; store only session-token hashes. Use HttpOnly cookie sessions, Secure under HTTPS, SameSite=Lax and explicit CSRF validation for mutations. Check active account and current permissions on every request. Proposed configurable defaults: 30-minute idle and 8-hour absolute expiry; these are product defaults, not clinical standards. Rotate on authentication; revoke on logout, password reset, account disable and role change. Reject ambiguous requests carrying both session and bearer identities.

Use generic authentication errors and bounded login throttling by account and source address. Keep secrets out of audit/logs; record relevant success/failure/account-management events without logging credential material. Do not set logout state only in the browser: revoke the server session too.

Permission defaults for the single synthetic workspace:

| Operation | Physician | Pharmacist | Administrator |
|---|---|---|---|
| Create synthetic patient and assessment | Yes, own/assigned cases | No by default in new patient workflow | Manage access; existing administrative assessment inspection retained |
| Read patient, labs and results | Own/assigned cases | Explicitly assigned read access | All synthetic cases, audited access |
| Add/correct observations and create new assessments | Owner/assigned physician | No | No clinical authoring through admin screens |
| Prepare source candidates | No | Yes | Yes |
| Approve/reject clinical rules | No | Yes | No |
| Manage staff/access and inspect audit | No | No | Yes |

Existing unlinked assessments keep creator/admin permissions, including historic pharmacist-created records. New patient-associated reads follow patient access rules as well as endpoint roles. Sharing grants access to that patient's existing history; make that effect explicit in the UI. Access revocation applies immediately to lists, details, replay and exports. Account administration does not grant pharmacist approval authority.

### Synthetic patient and lab history

A profile contains generated synthetic code, optional fictional display alias, owner, archived state and timestamps. Avoid names, PESEL, contact information and dates of birth. Clinical calculation inputs live in observations/assessment snapshots rather than mutable profile defaults. Age is explicitly supplied for the dated assessment, not extrapolated silently.

A creatinine observation contains value, original unit, sample time, optional uncertainty bounds, IDMS confirmation, author and record time. Corrections append a replacement referencing the prior observation and a reason; do not overwrite the original. Archive profiles without deleting result history. Clinical scope flags, weight/height and current medication regimen must be confirmed for each new assessment; selecting an older result must not silently reuse stale confirmations.

Patient page: summary → dated observations → assessment timeline. New assessment selects a specific observation, shows its age/units, collects remaining inputs, then submits to the existing deterministic engine. Results show renal estimates, per-drug status/action if available, manual-review reasons, checked dose limits, evidence and recorded versions. CrCl and indexed eGFR use separate labels/scales. Comparison shows differing inputs and rule/engine versions, without presenting a changed result as improvement in health.

### Results, history and export

Add cursor-paginated patient and assessment lists, date/status/product filtering and stable detail routes. Show recently saved results after login and refresh. Preserve result snapshots exactly; a recalculation creates a new assessment referencing its predecessor. Replay verifies the old snapshot with its recorded engine version and remains distinct from recalculation using current knowledge.

Export a bounded JSON result and printable report with synthetic code, inputs/sample time, units, results, reasons, citations, source/rule/engine versions, assessment ID and timestamp. Printable HTML/browser PDF is sufficient for the first final UI; a server PDF renderer is optional. Export permissions equal result-read permissions and export events are audited. No secrets or unrelated patient history in reports.

### Data and API additions

Proposed storage: `app_users`, `auth_sessions`, `account_reset_tokens`, `synthetic_patients`, `patient_access`, `renal_observations`, and append-only `patient_assessments` links. Add indexes for owner/date and patient/date pagination. Observation corrections reference predecessors. Assessment links identify the observation and optional prior assessment; validate that all belong to the same authorised case.

Keep the existing `domain.Request` and deterministic calculator interface. A new patient assessment API builds that request from explicit selected observation and current inputs; the stored result remains the authoritative input/output snapshot. Commit assessment, patient/observation link and audit together; recheck access inside the write transaction. Idempotency keys prevent duplicate records on client retries. Add a new migration rather than modifying migrations 001/002 or historical payloads.

Proposed API groups under `/api/v1`:

- `/auth/login`, `/auth/logout`, `/auth/session`, `/auth/password`, `/auth/activate`; `/admin/users` and explicit reset/disable/session-revocation operations.
- `/patients` and `/patients/{id}`; explicit archive and access-management operations.
- `/patients/{id}/observations`, correction endpoint, `/patients/{id}/assessments` for list/create.
- `/assessments` GET for authorised history; reuse assessment detail/replay; add authorised export and predecessor linkage.
- Reuse catalog, evidence, document and clinical review endpoints; audit summary/status UI uses bounded pagination rather than fetching unlimited history.

Preserve stable existing actor IDs during account migration. Do not infer that old assessments belong to the same patient from matching clinical inputs. Existing records appear under “Unlinked assessments”; linking, if offered, is an explicit audited action with authorisation, without changing the assessment snapshot. Validate all migration/replay paths against a copy/disposable database before the local database upgrade.

## Delivery sequence and acceptance gates

### Phase 6 — Staff accounts and sessions

1. Add account/session schema, password/activation/reset primitives and bootstrap migration from existing role identities.
2. Implement session endpoints, revocation, CSRF, throttling and account administration. Preserve explicit script compatibility during transition.
3. Add minimal login/account screens in the current app so authentication can be verified before UI migration.

Acceptance: successful/failed login, mandatory password change, idle/absolute expiry, logout across reloads, revoked sessions, role change, disabled accounts and last-admin protection. Cookies have correct environment-specific flags; unauthenticated and cross-site writes fail. Existing assessments still belong to the same actors; review rights remain pharmacist-only. Concurrent reset/use and revoke/write tests fail closed.

### Phase 7 — Synthetic patient history and saved results

1. Add synthetic profiles, per-case permissions and append-only observations/corrections.
2. Implement authorised list/detail APIs and atomic assessment linkage, retry protection, unlinked-history access and exports.
3. Add the patient timeline and saved-results workflow; explicitly choose observations and reconfirm current inputs.

Acceptance: two physicians cannot list/read/export/replay each other's cases; assigned pharmacist sees only assigned cases; administrator access is audited. Corrected observations leave old results reproducible. New results survive restart, stale edits conflict, pagination does not leak records, duplicate submissions do not duplicate results. Archived patients retain readable history. A measurement from another patient cannot be attached to an assessment.

### Phase 8 — Final React clinician workspace

1. Create `web/` with TypeScript and generated API types, building static assets into the Go-served application to retain one browser origin.
2. Migrate screens incrementally: login → dashboard/patients → patient/labs → assessment/results → evidence/review/documents → administration.
3. Add direct-link reload support, readable result comparison, printing, keyboard navigation, mobile layouts and consistent state handling. Remove the browser key-login workflow after account migration is verified.

Acceptance: staff can complete login → create synthetic case → record sample → assess multiple drugs → reopen/replay/export results. Source review/edit/stale-conflict behavior remains intact. Session expiry handles unsaved input explicitly and clears patient data on logout; no persistent browser credential/patient cache. Direct route refresh works and does not intercept `/api/*` or health routes. Clinical result content matches saved backend snapshots.

### Phase 9 — Release integration and final evidence

1. Consolidate clean installation, migration/rollback instructions, backup/restore test, readiness/model status, user guide and seeded synthetic demonstration. Separate database migration privileges from runtime where practical, and document actual remaining owner privileges.
2. Execute final end-to-end checks across all roles, source version changes, concurrency, dependency failure, restart and restore. Freeze the generated OpenAPI and release manifest with component/model/source versions.
3. Reconcile outstanding Phases 2/4/5 and thesis experiments, documenting actual results, exclusions and unmet gates. Do not rename incomplete experiments as completed features.

Academic application release gate: Phases 6–8 acceptance passes plus regression tests, reproducible install, preservation of history and verified backup restore. Cases without approved rules clearly return manual review.

Full projected thesis-system gate additionally requires the planned assisted ingestion flow and its measured fidelity, independently reviewed product/rule/case coverage, and actual model experiments. Preserve original target of approximately 20 products/200 synthetic cases, with five reviewed product sets as the initial clinical milestone; adjustments require explicit supervisor/user decisions. E1 uses reference/calculator evidence; E2 needs extraction field/citation error evaluation; E3 needs independently labeled recommendation cases; E5 measures assessment latency/extraction time, not just retrieval latency. Optional explanation generation/E4 remains gated on grounded-model evaluation; E6 remains stretch scope as in the original architecture.

No fabricated clinical approvals, invented model benchmarks or default activation of machine candidates. Human review blocks clinical activation and independent evidence claims; it need not block account/patient/UI engineering or pending-only worker plumbing. Selecting clinical extraction/OCR models and scoring their semantics still follows the review gate in Phase 4.

## Implementation order

Start Phase 6 next, then Phase 7, then Phase 8; integrate Phase 9 throughout and execute its final release checks last. Continue source review and independent label preparation alongside these software phases. Once reviewed extraction examples and a pinned local model are available, finish the remaining Phase 4/5 experiments. No runtime code changes are part of this planning update.
