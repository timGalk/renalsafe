---
gsd_state_version: "1.0"
milestone: v0.1
milestone_name: verifiable core and main system
progress:
  total_phases: 5
  completed_phases: 2
  total_plans: 10
  completed_plans: 7
current_phase: 2
current_phase_name: clinical-kb
status: ready-for-clinical-review
last_updated: 2026-10-19
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
