---
gsd_state_version: "1.0"
milestone: v0.1
milestone_name: verifiable core and main system
progress:
  total_phases: 9
  completed_phases: 3
  total_plans: 14
  completed_plans: 8
current_phase: 7
current_phase_name: patient-history
status: ready-for-patient-history
last_updated: 2026-11-05
---
# State

Phase 2 clinical review is the next human dependency. Phases 1 and 3 software verified; manual Phase 4 and retrieval Phase 5 slices implemented in parallel under ADR-002. Phase 4/5 overall goals remain open.

## Completed

Phase 1 plans 01-01/02/03; Phase 3 plans 03-01/02; Phase 4 plan 04-01; Phase 5 plan 05-01. Go engine 0.2.0, PostgreSQL+pgvector, transactional audit and replay, role-bound review, PDF ingestion, browser, OpenAPI and evidence-only retrieval. 56 top-level tests pass with race detection against disposable real PostgreSQL; vet and browser smoke pass. See docs/system-verification.md.

## Next actions

1. Phase 2 / 02-01: current Polish ChPL applicability, five exact products, independent pharmacist review and gold cases. The real EMA candidate stays PENDING.
2. Phase 4 / 04-02: table-aware splitting, optional pending-only AI extraction and difficult PDF/OCR evaluation.
3. Phase 5 / 05-02: deploy a pinned local embedding model; compare lexical/hybrid on independently reviewed queries; complete clinical gold set and thesis experiments before considering guarded generation.

## Decisions and limits

ADR-002 allows software infrastructure alongside clinical Phase 2 without activating candidates. Embedded plain browser client and Poppler implement the current slice. RAG is extractive; local embeddings optional. No real embedding-quality result or generative model run claimed. All approvals in tests use synthetic fixtures in a disposable database.

GSD planning, plan check, execution and verification performed inline using the skill adapter fallback. No independent agent review claimed. Database-owner bypass, production identity, external audit anchoring and disaster recovery remain documented deployment limits.

## Presentation update — 2026-10-19

The full browser interface, accessibility labels, client messages and numeric formatting now use English. English browser smoke covers assessment, evidence, review, document upload controls and mobile layout. Clinical sources and decision semantics are unchanged. See docs/demo-guide.md for a short English walkthrough.

## Polish sources acquired — 2026-10-23

Two official Polish RPL ChPLs downloaded and pinned: Meronem (500 mg/1 g, RPL 4282, 14 pages) and Ceftazidime Kabi (1000 mg, RPL 24542, 16 pages). UTF-8 extraction and actual Go section/page/byte provenance checks pass with race detection. Acquisition manifest and report: data/sources/pl-2026-10-23/manifest.json and docs/polish-chpl-acquisition.md. This adds source evidence for Phase 2; no new rules or approvals, database import or clinical validation claimed. KB-01 remains partial and KB-02 remains open.

## Two-drug synthetic run — 2026-10-23

Two Polish sources are now imported into the running database (Meronem 1 g and Ceftazidime Kabi 1000 mg, exact IV hospital-acquired pneumonia contexts). Six synthetic scenarios pass through the live API with renal reference/unit/uncertainty checks, per-drug manual review, isolated Polish evidence, immutable reads, replay and audited persistence. Import reuse confirmed on rerun. Full disposable PostgreSQL suite: 57 top-level tests pass with race detection. English two-drug preset added; static assets verified, visual browser automation blocked by URL policy. No clinical rule created/approved and no interaction assessment claimed. Reproduce with scripts/demo-two-drugs.py; report docs/two-drug-demo.md.

## Implementation continuation — 2026-11-05

Added lossless layout-aware passage partitioning for new imports (soft 6000-byte target; table groups remain indivisible; prior chunks/citations immutable). Real Polish extraction fixtures verify complete content conservation. Browser now supports pending candidate correction with fixed source/chunk identity, expected digest and separate pharmacist approval; condition inclusivity and multiple unmodeled conditions are preserved.

Added local retrieval evaluation runner, six explicitly labeled engineering cases, scoring/run-validity regression tests and CI checks. Reports pin dataset/corpus identity and knowledge/model versions; degraded hybrid or changing corpus cannot count as a valid comparison. The live lexical software baseline passes all six cases with zero wrong-source hits; two expected abstentions. It is not an independent gold-set/model benchmark or clinical result. Full disposable database/race suite, vet and browser smoke (including candidate edit/cancel) pass. Local application rebuilt and started. See docs/evaluation-guide.md and docs/system-verification.md.

Phase 4/5 plans remain open: independent table/candidate review, OCR/harder PDF semantics, assisted extraction, actual pinned-model comparison and independent clinical/retrieval gold sets still required. No clinical rule created or approved this session. No model downloaded or generative explanation enabled.

## Final-version planning — 2026-11-05

User requested a plan/research reconciliation for login, patient results and final delivery, and explicitly selected a clinician workspace with synthetic patient history (no patient portal). Original architecture/code comparison and current OWASP research saved in FINAL-VERSION-RESEARCH.md; detailed design/acceptance contract in FINAL-VERSION-PLAN.md. Added unexecuted Phases 6–9 for accounts, patient/results history, final React UI and release validation. Next engineering work: Phase 6. Clinical review remains an independent unresolved dependency for activation/full thesis evidence. No runtime changes or tests performed in this planning update.

## Phase 6 executed — 2026-11-05

Staff accounts, Argon2id passwords, opaque cookie sessions, CSRF, idle/absolute expiry, durable login throttles, single-use activation/reset, password change and staff administration implemented. Database writes recheck captured session/key authorisation inside their transaction. Role change/disable/reset/password change revoke credentials; last active administrator protected. Existing role identities and assessment payloads preserved; legacy service keys remain transitional and are not silently re-enabled on restart.

Additive migration 003 applied to local Compose app. Initial workspace-admin account created with a private 0600 activation file, no default password. Full disposable-database race suite, vet, password/session lifecycle and browser activation/login/reload/logout checks pass. Existing assessment/replay/review browser smoke also passes. API contract regenerated. See docs/accounts.md and phases/06-accounts-sessions/06-01-SUMMARY.md. Timeout policy is fixed at 30 minutes idle/eight hours absolute in this version; multi-tab clearing occurs on next request, and privileged lost-admin recovery remains an operator task.

Next: Phase 7, synthetic patient profiles, renal observations and result history. Phases 2/4/5 clinical and model evidence remain open. Phase 7–9 features have not been implemented by the account work.
