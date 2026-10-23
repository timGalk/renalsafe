# Requirements

- [x] **ENG-01** Typed patient, exact product/context, rule and source identity
- [x] **ENG-02** CG/CKD-EPI with explicit units, no substitution or rounding at matching
- [x] **ENG-03** Strict snapshots; pending/rejected/superseded ineligible; overlap rejected
- [x] **ENG-04** Gaps, missing/invalid inputs, unsupported context yield manual review without action/dose
- [x] **ENG-05** Deterministic cited explanation and reproducible snapshot identity
- [x] **ENG-06** Append-only audit with input/result/version snapshots; corruption detected
- [x] **ENG-07** Threshold, numeric reference, unsafe input, determinism and audit tests
- [ ] **KB-01** Versioned official source files and section-grounded pending candidates
- [ ] **KB-02** Independent pharmacist approval and gold cases for five products
- [x] **API-01** OpenAPI + durable storage + RBAC
- [ ] **ING-01** Pending-only extraction and review
- [ ] **EVAL-01** Gold set and reported E1–E3/E5

## Traceability

| Requirement | Phase | Status |
|---|---|---|
| ENG-01 | Phase 1 | Complete |
| ENG-02 | Phase 1 | Complete |
| ENG-03 | Phase 1 | Complete |
| ENG-04 | Phase 1 | Complete |
| ENG-05 | Phase 1 | Complete |
| ENG-06 | Phase 1 | Complete |
| ENG-07 | Phase 1 | Complete |
| KB-01 | Phase 2 | Partial: EMA source/pending candidate plus two Polish RPL sources; five-product reviewed scope unfinished |
| KB-02 | Phase 2 | Pending human clinical review |
| API-01 | Phase 3 | Complete (software) |
| ING-01 | Phase 4 | Manual slice complete; assisted extraction/harder PDF evaluation open |
| EVAL-01 | Phase 5 | Pending |

## Main-system / RAG slice (2026-10-19)

- [x] API-02: transactional audit and historical replay.
- [x] API-03: authenticated role-bound review, stale-write detection and overlap exclusion.
- [x] API-04: browser assessment and review workflow.
- [x] ING-02: declared document scope, section/page grounding and pending-only candidates.
- [x] RAG-01: exact-context/version-filtered lexical evidence.
- [x] RAG-02: optional local dense retrieval, RRF, explicit fallback and extractive answers.

| Requirement | Phase | Status |
|---|---|---|
| ENG-08 | Phase 3 | Explicit age/mass conditions and regimen checks verified with fixtures |
| API-02 | Phase 3 | Complete: atomic audit/history/replay |
| API-03 | Phase 3 | Complete: role/digest/overlap checks |
| API-04 | Phase 3 | Complete: browser smoke |
| ING-02 | Phase 4 | Complete: declared scope/page/section/pending checks |
| RAG-01 | Phase 5 | Complete: scoped full text |
| RAG-02 | Phase 5 | Software path complete; real-model quality evaluation remains EVAL-01 |

- [x] **ENG-08** Explicit source-backed age/mass conditions and prescribed-dose/interval checks without inferred regimens.
