---
phase: 01-decision-engine
status: passed
verified: 2026-10-14
score: 7/7 software requirements
scope: software-only
---
# Phase 1 verification

Goal achieved for software: deterministic, auditable renal rule selection with explicit units, source identity and safe uncertainty handling, tested offline. This is not clinical validation.

| Requirement | Evidence | Result |
|---|---|---|
| ENG-01 | domain/model.go; context and interval tests | passed |
| ENG-02 | renal/calculate.go; six independent reference points, conversion and missing-CG tests | passed |
| ENG-03 | rules/snapshot.go; lifecycle, overlap, source/approval version, path and JSON tests | passed |
| ENG-04 | decision/engine_test.go; unsafe patient, missing rule, gaps and uncertainty tests | passed |
| ENG-05 | source/rule digest, frozen snapshot and deterministic repeated assessment test | passed |
| ENG-06 | audit/log_test.go; chain, full snapshots, replay, corruption, lock and failure tests | passed |
| ENG-07 | all 30 top-level tests passed with race detection; vet passed; binary smoke verified audit and pending-only output | passed |

Coverage: decision 98.4%, renal 98.6%, audit 88.5%, rules 74.4%, domain 62.5%, CLI 71.4%. Testkit is fixture support and has no own test suite. Coverage is a software measure only.

Human clinical items belong to Phase 2: regulatory applicability, source currency, full contraindication/indication context, weight and freshness policy, rule semantics and independent gold cases. No clinical rule approved. API/database security, hosted CI execution and external audit anchoring not implemented in Phase 1.

## Offline smoke

`example-request.json`: synthetic adult, age 70, actual weight 72 kg, creatinine 2.8 mg/dL. CG=25 mL/min; CKD-EPI 2021=23.53527009785177 mL/min/1.73m2. The pending-only clinical snapshot yields `MANUAL_REVIEW_REQUIRED / NO_APPROVED_RULE` with no action or dose. Journal validation reports one verified event. This demonstrates fail-safe eligibility rather than a clinical recommendation.

## Installed repository verification

After copying to `/home/apollo/Pwr/RenalSafe`, all 30 top-level tests passed again with race detection, `go vet ./...` passed and normal `go build` succeeded with VCS metadata enabled. Local repository initialized and implementation committed; hosted CI has not run.
