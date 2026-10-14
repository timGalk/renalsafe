# Roadmap

## v0.1: verifiable core before application

- [x] **Phase 1: Decision engine** - deterministic offline core. (completed 2026-10-14)
- [ ] **Phase 2: Clinical knowledge base** - official sources and independent clinical approval.
- [ ] **Phase 3: Persisted API** - durable storage and authenticated approval.
- [ ] **Phase 4: Ingestion and review** - pending extraction and review workflow.
- [ ] **Phase 5: Retrieval and evaluation** - guarded retrieval and thesis experiments.

### Phase 1: Decision engine

**Goal:** Reproduce renal-rule decisions from explicit inputs and an immutable knowledge snapshot, failing closed on uncertainty.
**Depends on:** Nothing
**Requirements:** ENG-01, ENG-02, ENG-03, ENG-04, ENG-05, ENG-06, ENG-07
**Success Criteria:** Typed contexts; correct equations and units; approved-only exact matching; no fallback dose; reproducible cited explanations; mandatory audit before CLI output; offline safety tests.
**Plans:** 3/3 plans complete

Plans:

- [x] 01-01-PLAN.md - domain and deterministic tracer
- [x] 01-02-PLAN.md - renal calculation and uncertainty
- [x] 01-03-PLAN.md - audit, CLI and regression

### Phase 2: Clinical knowledge base

**Goal:** Five clinically reviewed exact product-specific rule sets grounded in current official SmPC/ChPL, with independent gold cases.
**Depends on:** Phase 1
**Requirements:** KB-01, KB-02
**Success Criteria:** Source bytes/version/section pinned; no ambiguous candidates activated; authenticated human approval and independent expected outcomes. Mechanical tests cannot satisfy human clinical validation.
**Plans:** 1 plan

Plans:

- [ ] 02-01-PLAN.md - sources and clinical review checkpoint

### Phase 3: Persisted API

**Goal:** OpenAPI assessments and transactional storage with RBAC, immutable rule versions and durable audit.
**Depends on:** Phase 2
**Requirements:** API-01
**Success Criteria:** Approved overlaps excluded by exact product/route/form/indication/metric; only pharmacist can approve; persistence failure never emits success; historical replay.
**Plans:** 1 outline; refine through GSD before execution

Plans:

- [ ] 03-01-PLAN.md - API, schema, RBAC and integration tests

### Phase 4: Ingestion and review

**Goal:** Preserve PDF tables and sections; extracted rules remain pending until human review.
**Depends on:** Phase 3
**Requirements:** ING-01
**Success Criteria:** Quote checks, visible full source context, audited approval/rejection and supersession; no auto-approval path.
**Plans:** 1 outline; refine through GSD before execution

Plans:

- [ ] 04-01-PLAN.md - ingestion and reviewer workflow

### Phase 5: Retrieval and evaluation

**Goal:** Passage-only fallback, guarded explanations and reproducible E1–E3/E5 experiments.
**Depends on:** Phase 4
**Requirements:** EVAL-01
**Success Criteria:** Independent gold set of about 20 products and 200 synthetic cases; source-correctness and unsafe-error measures; documented limitations and thesis results.
**Plans:** 1 outline; refine through GSD before execution

Plans:

- [ ] 05-01-PLAN.md - retrieval, evaluation and thesis evidence

## Progress

| Phase | Plans Complete | Status | Completed |
|---|---|---|---|
| 1. Decision engine | 3/3 | Complete    | 2026-10-14 |
| 2. Clinical knowledge base | 0/1 | In progress; review needed | - |
| 3. Persisted API | 0/1 | Not started | - |
| 4. Ingestion and review | 0/1 | Not started | - |
| 5. Retrieval and evaluation | 0/1 | Not started | - |

Phase 1 completion concerns software behaviour on synthetic fixtures, not clinical validation.
