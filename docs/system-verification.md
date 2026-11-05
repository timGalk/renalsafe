# Main system verification — 2026-10-19

Software verification of engine 0.2.0 and the local persisted application. This report does not assert clinical validation.

## Executed checks

- `python3 scripts/test-integration.py`: PASS; 56 top-level Go tests, with `-count=1 -race -cover`, against a dedicated disposable PostgreSQL 18 + pgvector container. Database tests executed, not skipped. The runner removed the test container and did not use the application database.
- `go vet ./...`: PASS.
- Browser smoke against the Compose application: physician assessment with PENDING source yields manual review, saved result and replay match, source evidence is visible, physician cannot see reviewer actions, pharmacist sees the pending rule without preselected confirmation, mobile layout fits 390 px, no JavaScript errors, no persistent browser credential storage. No real rule approved.
- Docker image builds with Go 1.25.1 and Poppler; nonroot application and loopback-only ports. PostgreSQL data persists in a named volume. Fresh integration databases apply both immutable migrations.
- Generated OpenAPI, JavaScript syntax and whitespace checks performed. Hosted GitHub CI configured, but no hosted execution claimed.

## Acceptance evidence

| Behavior | Executable evidence |
|---|---|
| Renal reference values, conversion, thresholds and uncertainty | `internal/renal/calculate_test.go`, `internal/decision/engine_test.go` |
| Optional age/mass conditions and explicit regimen limits; no inferred dose or unit conversion | `internal/decision/engine_test.go`, `internal/rules/snapshot_test.go` |
| Authentication, server-bound review identity, role restrictions, stale review | `internal/auth/auth_test.go`, `internal/httpapi/server_test.go`, `internal/store/store_test.go` |
| Database overlap exclusion, concurrent review, audit failure rollback | `internal/store/store_test.go` |
| Immutable source/assessment history, audit foreign key, historical replay after supersession | `internal/store/store_test.go`, migrations 001–002 |
| Physical page/section/byte spans, combined EMA product isolation, pending-only seed | `internal/evidence/evidence_test.go`, `internal/store/store_test.go` |
| Exact-context/version search, lexical and known-vector hybrid, explicit fallback, no rule activation | `internal/store/store_test.go` |
| Local provider protocol, invalid model/dimension/vector/outage | `internal/evidence/evidence_test.go` |
| Strict JSON, cross-origin rejection, upload limits, embedded browser and OpenAPI | `internal/httpapi/server_test.go` |

Coverage: decision 93.3%, renal 98.6%, audit 88.5%, auth 77.4%, evidence 87.4%, HTTP API 70.0%, store 76.2%, rules 67.0%, domain 57.1%, offline CLI 70.5%. Server/spec entrypoints have no direct coverage; browser and integration checks supplement package tests. These numbers are not clinical safety scores.

## Boundaries and next evidence

Only the previously pinned official EMA PDF and its PENDING candidate are seeded. No human clinical approval, five-product knowledge base or independent clinical gold set exists yet. Local role credentials identify accounts; they do not establish professional qualification.

Dense retrieval was verified with known synthetic vectors and a simulated provider. No real local embedding model or quality/latency benchmark was run. The deployed baseline is lexical retrieval; hybrid reports degradation explicitly until configured/indexed. Responses are deterministic extractive evidence templates, not free-form LLM generation. AI-assisted extraction, OCR, semantic table reconstruction and thesis E1–E3/E5 experiments remain open.

Source scope is declared by a reviewer. Quote/numeric checks ensure byte grounding, not semantic clinical correctness. Large sections can exceed an embedding model context; `truncate:false` fails rather than silently losing text. Model names require operational version pinning and reindexing after weight changes.

Database owner privileges can bypass triggers and rewrite history; external signatures/anchoring, restricted deployment roles, SSO, retention, backup/restore testing and production performance testing remain deployment work. Full history replay requires the recorded engine version; unavailable versions return a conflict.

## Two-drug run — 2026-10-23

The full disposable-database suite now passes 57 top-level tests including the real Polish PDF ingestion test. A live authenticated run imports both Polish sources and checks six two-drug synthetic cases, physical source identity, exact-context Polish retrieval, persisted reads, replay and verified audit. Repeated import is idempotent. No clinical rules change. See `two-drug-demo.md`; the new browser preset has static/API checks only because browser automation was blocked this session.

## Implementation continuation — 2026-11-05

Implemented and executed:

- New imports use exact-span passage partitioning with a soft 6000-byte target. Paragraph/table boundaries preserve physical pages and byte offsets. Large indivisible blocks remain whole. Tests verify UTF-8 provenance, deterministic partitioning, no loss/duplication, table/header/possible-footnote grouping and full content conservation on both pinned Polish PDFs. Existing persisted chunks are unchanged.
- Browser pending-rule editing uses the existing authenticated PATCH API, fixed source/chunk context and expected content digest. Exclusive/inclusive condition boundaries and separate unmodeled conditions are preserved in the form. Approval remains a separate pharmacist action.
- Retrieval evaluation runner records immutable source labels, active corpus/dataset hashes, requested/actual modes, revisions, individual responses, relevance metrics and latency. It rejects invalid labels and marks changing/degraded comparisons invalid. Seven offline scoring/validation regression tests pass, including duplicate hits, wrong source/section/context, abstention, absent independent-review metadata, fallback and version drift.
- `python3 scripts/test-integration.py`: PASS, all Go packages against a disposable PostgreSQL+pgvector database with race detection. The application database was not used for tests. `go vet ./...`: PASS.
- Browser smoke: PASS, including synthetic assessment, replay, source retrieval, pharmacist pending-edit/cancel, fixed source selectors, document workflow, mobile overflow and no persistent credential storage. No clinical rule was saved or approved by this smoke test.
- Local Compose application rebuilt and started successfully.
- Live lexical evaluation on six explicitly labeled software fixtures: four positive source/section cases found, two expected abstentions, recall@5 1.0 on positive cases, citation precision 1.0 where hits exist, zero wrong-source hits. Stable knowledge version/corpus; no fallback. Report: ignored `outputs/evaluation/lexical-software.json`. These trivial source checks are not a real-model or independent gold-set benchmark. Timing is machine/run-specific and available in that report.
- Evaluation scoring and JavaScript syntax checks added to CI; no hosted CI execution claimed.

Still open: qualified clinical approval, five-product reviewed rule coverage, independent clinical/retrieval labels, actual pinned-model comparison, harder-table semantic fidelity/OCR and assisted extraction. No generative dosing/explanation model added. Passage layout detection is heuristic; an indivisible block may still exceed the provider's context. No E1–E3/E5 thesis or clinical validation completion is asserted.

## Staff accounts/session implementation — 2026-11-05

Phase 6 adds migration 003, database-backed staff identities, Argon2id passwords, activation/reset, cookie sessions, revocation, CSRF and staff administration. Existing actor IDs and clinical snapshots are preserved. The account lifecycle and browser workflow described in `accounts.md` were executed against disposable test identities; no clinical rules approved.

Full isolated PostgreSQL/pgvector Go suite passes with race detection. Account tests cover salting, invalid passwords, activation expiry/reuse/concurrent consumption, last-admin protection, stable provisioning, role/disable/reset/password/logout revocation, idle/absolute expiry, in-flight writer revocation, missing CSRF, secure cookies under HTTPS and ambiguous cookie/bearer identity rejection. `go vet ./...` passes. New browser account smoke passes activation, login, refresh restoration, logout surviving refresh, role visibility and no persistent browser storage. Existing assessment/evidence/replay/pharmacist review/mobile browser smoke passes after opening the new migration-key panel. OpenAPI regenerated and local Compose app rebuilt with additive migration.

Fixed session timeouts, direct TLS/loopback scope, next-request multi-tab clearing and privileged lost-admin recovery limits are recorded in `accounts.md`. Phase 7 patient profiles and history remain next; no patient history feature is claimed by account completion.

## Synthetic patient/history implementation — 2026-11-05

Phase 7 adds additive migration 004, synthetic case profiles/access, dated append-only creatinine observations and corrections, immutable case/observation/predecessor assessment links, atomic assessment/audit/idempotency and authorised paginated history. Existing unlinked records remain unmodified. Patient sharing governs result detail/replay/export, including previous authors; role-demoted pharmacists cannot manage cases. Archive blocks new clinical writes while preserving history.

Executed: full isolated PostgreSQL/pgvector Go suite with race detection, vet, JavaScript syntax, generated contract equality and report markup-escaping regression. New tests cover cross-user/case isolation, pharmacist read-only access, grant/revoke, selected stored lab overriding submitted copies, concurrent retry deduplication and changed-body conflict, corrections/replay, archive/stale versions, immutable rows, transaction/audit rollback, unlinked history, fine timestamp pagination and store reopening.

The final live patient browser check passed create/lab/select/assess/save/reopen/export, session reload, sharing/revocation, correction/recalculation/predecessor, old replay, snapshot comparison, printable download, archive, mobile overflow and no persistent browser storage. Account smoke and existing assessment/evidence/pharmacist-review smoke also passed. Fixture staff were disabled without deleting case/audit history. After an explicit local core restart, the archived test case and both assessment exports were read successfully and both historical replays matched. Summary: ignored `outputs/patient-smoke.json`.

See `patient-history.md` for operation and bounds. React/stable client routes remain Phase 8; no clinical validation, real patient records, general lab system or server PDF renderer is claimed. No clinical rules were approved.

## React workspace implementation — 2026-11-05

Phase 8 replaces the served imperative browser/key-login client with actual React/TypeScript components and generated API types. Dedicated clinician/patient/results/source/review/staff/account/admin routes support direct links/reloads. Same-origin Go serving preserves API/health/static 404 behavior and CSP; locked frontend assets build inside Docker. Added single-observation access for stable assessment links, bounded audit metadata pages, streaming chain verification and administrator configuration/status.

Executed: strict TypeScript, generated API-type equality, production build, clean Docker build/start, full disposable PostgreSQL/pgvector Go suite with race detection, vet and OpenAPI regeneration equality. New backend checks cover role-gated status/audit, page limits/metadata exclusion, corruption rejection, complete verification and browser-route/hashed-asset/API isolation.

The final all-role React browser suite passed activation/login/no-key-entry, patient/observation/assessment, unsaved-form Keep editing/Escape/discard, exact stored result/export parity, result/comparison/document reload routes, replay/JSON/printable report, exact-context evidence/fallback, case isolation/sharing/revocation, pharmacist read-only behavior, immutable correction/recalculation, archive, mobile overflow and no persistent browser storage. A new pending clone was saved through React then rejected without clinical approval; original pending rules remained unchanged. Staff/audit/status screens, role-denied routes and server revocation clearing protected inputs also passed. No bearer key was sent by the React client and no JavaScript runtime error occurred. Desktop/mobile screenshots inspected; report/artifacts in ignored outputs/react-workspace-smoke.json and outputs/react-*.png.

No independent clinical validation, real-model benchmark, release recovery/backup completion or production readiness is asserted. Phase 9 and unfinished clinical/ingestion/evaluation gates remain open. See final-workspace.md.
