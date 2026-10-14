# RenalSafe

Verifiable offline core for the renal-dose CDSS thesis. Go 1.25+, standard library only. **Academic prototype, synthetic patients only.**

## Run

```sh
cd core
go test -race ./...
go vet ./...
go run ./cmd/renalsafe -kb ../data/knowledge.json -audit ./example.audit.jsonl < ../data/example-request.json
go run ./cmd/renalsafe -verify-audit ./example.audit.jsonl
```

The example calculates renal function but returns `MANUAL_REVIEW_REQUIRED`, `NO_APPROVED_RULE` and no action/dose. The real EMA candidate is PENDING. Test-only synthetic approved rules prove the complete matching path; the CLI rejects synthetic knowledge snapshots. No LLM, database or network required at runtime.

## Implemented

`core/internal/domain`: explicit patient quantities, context and metric, open/closed intervals, citations bound to source version/hash, review metadata, deterministic assessment.
`renal`: Cockcroft–Gault with explicitly confirmed actual weight, CKD-EPI 2021 race-free indexed eGFR; mg/dL and umol/L creatinine conversion; conservative scope and freshness gates.
`rules`: strict JSON (including duplicate-key rejection), pinned file identity and quote/numeric grounding, immutable snapshots, approved-only eligibility, overlap and stale-approval rejection.
`decision`: exact product/route/form/indication and metric match, gaps and uncertainty fail closed, template explanation with full citation; original unrounded numbers select the rule.
`audit`: required persistent journal before CLI output; full input, calculations, rule/source and knowledge snapshots, engine/policy/KB versions, SHA256 chain; replay and corruption tests.

## Scope and limitations

No clinically approved dosing rule exists yet. Reviewer role strings are trusted offline metadata, not authenticated approval. A human pharmacist must check the complete SmPC and rule semantics; byte/quote/numeric validation alone does not establish clinical validity. The decision only addresses renal criteria; it cannot establish overall prescribing safety.

The prototype policy (version `prototype-adult-stable-v1`) gates adults 18–120, creatinine 0.1–30 mg/dL, supplied weight 20–300 kg and height 100–250 cm, a sample at most seven days old, explicit stable renal function, no suspected AKI/dialysis, IDMS standardization and typical body composition. Supplied BMI below 18.5 or above 30 blocks advice. These are **engineering scope gates**, not clinical thresholds or validated laboratory freshness guidance. Missing height/weight/actual-weight confirmation prevents CG, while valid eGFR remains available. No weight imputation, creatinine floor/rounding, cap, BSA de-indexing or automatic metric substitution.

Creatinine uncertainty bounds, if supplied, use the same unit as creatinine and must be a finite closed interval containing the measured value. The derived renal interval must fit a single rule fully. Without bounds, measurement uncertainty is unknown, not zero: only the point value is matched. The calculator has ordinary estimation error and results at clinical cutoffs require independent clinical review.

Journal locking is fail closed: competing writers or a leftover lock after a crash stop output. Inspect the journal before manual lock recovery. Hash chains detect corruption and ordinary edits; complete rewriting or truncating whole events needs external anchoring to detect. Transactional database persistence, authenticated users, approval authorization, cryptographic signatures, retention and disaster recovery belong to Phase 3.

## Plans and sources

GSD state: `.planning/STATE.md`; roadmap: `.planning/ROADMAP.md`; plans, summaries and verification: `.planning/phases/`. GSD executed inline because no subagents were requested. Source inventory and document conflicts: `docs/materials-manifest.json`, `.planning/PROJECT.md`; clinical source and review backlog: `docs/clinical-review.md`; software validation: `docs/verification.md`.

Next: obtain current Polish ChPL applicability and pharmacist review; prepare five product-specific gold rule sets with independent cases. Then implement persisted API, review/ingestion workflow and the wider evaluation dataset. No UI/LLM work should precede core clinical validation.
