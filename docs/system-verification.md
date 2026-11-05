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


## Staff accounts/session implementation — 2026-11-05

Phase 6 adds migration 003, database-backed staff identities, Argon2id passwords, activation/reset, cookie sessions, revocation, CSRF and staff administration. Existing actor IDs and clinical snapshots are preserved. The account lifecycle and browser workflow described in `accounts.md` were executed against disposable test identities; no clinical rules approved.

Full isolated PostgreSQL/pgvector Go suite passes with race detection. Account tests cover salting, invalid passwords, activation expiry/reuse/concurrent consumption, last-admin protection, stable provisioning, role/disable/reset/password/logout revocation, idle/absolute expiry, in-flight writer revocation, missing CSRF, secure cookies under HTTPS and ambiguous cookie/bearer identity rejection. `go vet ./...` passes. New browser account smoke passes activation, login, refresh restoration, logout surviving refresh, role visibility and no persistent browser storage. Existing assessment/evidence/replay/pharmacist review/mobile browser smoke passes after opening the new migration-key panel. OpenAPI regenerated and local Compose app rebuilt with additive migration.

Fixed session timeouts, direct TLS/loopback scope, next-request multi-tab clearing and privileged lost-admin recovery limits are recorded in `accounts.md`. Phase 7 patient profiles and history remain next; no patient history feature is claimed by account completion.
