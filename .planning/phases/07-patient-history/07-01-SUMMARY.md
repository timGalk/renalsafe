# Phase 7 software implementation — 2026-11-05

Implemented the synthetic patient/history plan on the existing Go API and embedded browser, retaining the Phase 8 React migration as the next frontend step.

Migration 004 adds generated synthetic case codes, optional fictional aliases, immutable owner identity, versioned archive/alias changes, case membership, append-only dated creatinine observations/corrections and immutable assessment links. Database triggers reject wrong-case observations/predecessors and protect observation/link history. Existing assessment payloads and engine versions are not rewritten; older results remain unlinked under original creator/admin access.

Patient-linked assessment creation rechecks current case permissions inside the transaction, selects a stored non-superseded observation, and supplies its creatinine/units/uncertainty/time/IDMS to the unchanged engine. Remaining clinical inputs stay explicit. Assessment, observation/case/predecessor link and audit commit together. Per-case/author idempotency keys deduplicate identical concurrent retries and reject changed requests. Corrected observations and archived cases reject new assessments; historical snapshots/replays stay intact.

Permissions: owner/assigned physicians write active cases; assigned pharmacists read/replay/export only; administrators inspect/manage/audit without patient-authoring privileges. Revocation applies even to an assignee's own previously authored linked results. A physician demoted to pharmacist cannot manage their existing cases. Owner/admin sharing is explicit; legacy unlinked results are not assigned to patients heuristically.

Added cursor-paginated patient/observation/result lists, same-outcome product/status and saved-time filters, read-only saved-result reopening, explicit recalculation, same-case comparison with inputs and knowledge/rule/source versions, bounded audited JSON export and escaped printable HTML. Canonical database timestamps drive cursors so fine input precision cannot repeat a row. Patient data/credentials are not placed in persistent browser storage, and late responses cannot repopulate a changed patient/login workspace.

Executed verification:

- Full Go test suite against isolated disposable PostgreSQL+pgvector, with race detection: PASS. New store/API tests cover case isolation, pharmacist restrictions, sharing/revocation including creator bypass, selected-lab authority, retry conflicts/concurrency, immutable corrections and historical replay, wrong-case association, stale versions, archive, transaction/audit rollback, legacy/unlinked history, precise pagination, role demotion and store reopening.
- `go vet ./...`, generated OpenAPI equality and JavaScript syntax: PASS. Printable-report markup escaping test: PASS.
- Patient browser smoke: PASS on disposable staff identities, including creation/observation, selected lab retained during example loading, saved result/export, reload, shared pharmacist read-only access/revocation, correction/new recalculation/predecessor, original replay, comparison/printable export, archive and mobile/no-browser-storage checks.
- Existing account activation/login/restore/logout smoke: PASS. Existing physician assessment/evidence/replay and pharmacist edit/cancel/mobile smoke: PASS.
- Local Compose rebuilt with additive migration. Explicit core restart followed by authenticated reads/exports and replay of both saved test results: PASS; archived case remained readable. Browser summary is in ignored `outputs/patient-smoke.json`. Fixture staff accounts were disabled and synthetic case/audit history retained.

Limits: creatinine-focused synthetic cases, no real identifiers or patient portal; browser HTML download plus Print rather than server PDF rendering; existing browser client rather than React; stable client routes/final UI polish remain Phase 8. No clinical rule approved, clinical validation or model-quality claim added. The patient-history checkpoint is recorded as a separate implementation commit.
