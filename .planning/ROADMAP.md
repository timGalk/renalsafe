# Roadmap

## v0.1: verifiable core and main system

- [x] **Phase 1: Decision engine** — deterministic offline core (2026-10-14).
- [ ] **Phase 2: Clinical knowledge base** — official sources and independent clinical approval.
- [x] **Phase 3: Persisted API** — durable storage and authenticated review (2026-10-19).
- [ ] **Phase 4: Ingestion and review** — manual slice complete; assisted extraction/harder PDFs open.
- [ ] **Phase 5: Retrieval and evaluation** — retrieval slice complete; independent experiments open.

ADR-002 revises the original chronological dependency: Phase 3 software can build on verified Phase 1 while Phase 2 clinical work remains open. No unreviewed real rule becomes eligible.

### Phase 1: Decision engine

**Goal:** Reproduce renal decisions from explicit inputs and immutable knowledge, failing closed on uncertainty.
**Depends on:** Nothing.
**Requirements:** ENG-01–ENG-07.
**Success Criteria:** Explicit units, approved-only exact matching, reproducible cited explanations, mandatory audit and offline regression.
**Plans:** 3/3 complete.

- [x] 01-01-PLAN.md — domain and deterministic tracer.
- [x] 01-02-PLAN.md — renal calculation and uncertainty.
- [x] 01-03-PLAN.md — audit, CLI and regression.

### Phase 2: Clinical knowledge base

**Goal:** Five independently reviewed product-specific rule sets and gold cases from current official SmPC/ChPL.
**Depends on:** Phase 1.
**Requirements:** KB-01, KB-02.
**Success Criteria:** Pinned sources, confirmed regulatory applicability, qualified human review and independent expected outcomes. Software fixtures cannot satisfy these criteria.
**Plans:** 0/1 complete.

- [ ] 02-01-PLAN.md — sources and clinical review checkpoint.

### Phase 3: Persisted API

**Goal:** OpenAPI assessments, durable atomic audit, role-bound review and historical replay.
**Depends on:** Phase 1; Phase 2 required before clinical use.
**Requirements:** API-01–API-04, ENG-08.
**Success Criteria:** Database exclusion of approved overlaps, pharmacist-only review, no success before commit, immutable historical sources and reproducible decisions, browser workflow.
**Plans:** 2/2 complete; software verified.

- [x] 03-01-PLAN.md — API, schema, authentication, transactions and replay.
- [x] 03-02-PLAN.md — lifecycle, browser and integration verification.

### Phase 4: Ingestion and review

**Goal:** Grounded PDF extraction with visible source context and pending-only rule preparation.
**Depends on:** Phase 3.
**Requirements:** ING-01, ING-02.
**Success Criteria:** Bytes/versions/sections/pages pinned, no automatic approval, table/context fidelity measured on harder documents.
**Plans:** 1/2 complete; manual slice verified.

- [x] 04-01-PLAN.md — bounded PDF ingestion, manual candidates and review.
- [ ] 04-02-PLAN.md — table-aware splitting, assisted extraction and harder PDF evaluation.

### Phase 5: Retrieval and evaluation

**Goal:** Guarded evidence retrieval and reproducible E1–E3/E5 thesis experiments.
**Depends on:** Phase 4 manual slice; Phase 2 for clinical outcomes.
**Requirements:** RAG-01, RAG-02, EVAL-01.
**Success Criteria:** Exact-source retrieval, explicit fallback and citations; independent gold set around 20 products/200 synthetic cases; source-correctness and unsafe-error measures reported.
**Plans:** 1/2 complete; retrieval infrastructure verified.

- [x] 05-01-PLAN.md — lexical/optional hybrid and extractive evidence.
- [ ] 05-02-PLAN.md — real-model comparison, clinical gold and thesis evaluation.

## Progress

| Phase | Plans | Status | Completed |
|---|---|---|---|
| 1 | 3/3 | Software complete | 2026-10-14 |
| 2 | 0/1 | Human clinical review needed | — |
| 3 | 2/2 | Software complete | 2026-10-19 |
| 4 | 1/2 | Manual slice complete | — |
| 5 | 1/2 | Retrieval slice complete | — |

Clinical validation, assisted extraction and real-model/gold-set evaluation are not claimed complete.


## Final academic workspace extension — planned 2026-11-05

User confirmed clinician-managed synthetic patient history, without a patient portal. Original account/session login and React frontend remain unfinished original architecture scope. Detailed contract and research: [FINAL-VERSION-PLAN.md](FINAL-VERSION-PLAN.md), [FINAL-VERSION-RESEARCH.md](FINAL-VERSION-RESEARCH.md).

- [x] **Phase 6: Staff accounts and sessions** — username/password accounts, sessions, recovery, administration, preserved actor identity. Plan: `phases/06-accounts-sessions/06-01-PLAN.md`. Depends on Phase 3.
- [ ] **Phase 7: Synthetic patient history** — profiles, dated observations, permissions, saved results, replay and export. Plan: `phases/07-patient-history/07-01-PLAN.md`. Depends on Phase 6.
- [ ] **Phase 8: Final React clinician workspace** — complete navigation and staff workflows over the existing Go API. Plan: `phases/08-final-workspace/08-01-PLAN.md`. Depends on Phase 7.
- [ ] **Phase 9: Release integration and evidence** — installation/migration, backup restore, full end-to-end verification and thesis evidence reconciliation. Plan: `phases/09-release-validation/09-01-PLAN.md`. Depends on Phase 8; full thesis completion also depends on Phases 2/4/5.

Next engineering action: Phase 7. Clinical review/label preparation can proceed independently. Phases 2/4/5 remain open; existing completion counts above describe the original five-phase milestone only. A usable academic application release and complete projected thesis-system evidence have separate explicit gates in the final-version plan.
