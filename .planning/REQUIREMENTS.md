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


## Final workspace additions — planned 2026-11-05

- [x] **AUTH-01** Staff username/password accounts, bootstrap, activation/reset, disable and role administration — Phase 6.
- [x] **AUTH-02** Server sessions, expiry/revocation, CSRF, throttling and stable legacy actor identity — Phase 6.
- [x] **PAT-01** Synthetic profiles, ownership and per-case access checked for every operation — Phase 7.
- [x] **PAT-02** Dated renal observations, append-only corrections and explicit assessment-input selection — Phase 7.
- [x] **RES-01** Authorised paginated assessment history, atomic patient/observation association and retry protection — Phase 7.
- [x] **RES-02** Reopen/replay, version-aware comparison and bounded audited result exports — Phases 7–8.
- [x] **UI-01** React/TypeScript clinician, pharmacist and administrator journeys; mobile/accessibility/expiry states — Phase 8.
- [ ] **REL-01** Reproducible install/migration, backup restore, release manifest and end-to-end acceptance evidence — Phase 9.

Patient-facing accounts and real patient identifiers are outside the user-confirmed scope. Existing clinical and AI requirements retain their current status. Detailed acceptance and permissions are specified in FINAL-VERSION-PLAN.md.

Phase 7 implemented the result comparison/export workflow in the existing browser. At the Phase 7 checkpoint, React migration and final navigation/accessibility polish remained scheduled for Phase 8. Verification: phases/07-patient-history/07-01-SUMMARY.md.

Phase 8 React workspace, stable routes, role-aware screens, session/draft guards and administrator metadata/status UI verified. See phases/08-final-workspace/08-01-SUMMARY.md. REL-01 and independent clinical/model evidence remain open.
