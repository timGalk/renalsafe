# Synthetic patient cases and logic review

The saved suite in [`data/user-cases.json`](../data/user-cases.json) contains **39
fictional patient requests across seven product contexts**. Medicine information
links to the pinned Polish ChPLs. Ages, laboratory values, weights, timestamps and
patient histories are synthetic engineering inputs; they are not facts from ChPLs.
No clinical reference answer, reviewer identity or approval has been generated.

The contexts cover Meronem, Ceftazidime Kabi, Tartriakson, Metoclopramide hameln,
Sugammadex Reddy, Paracetamol MEDICINAE and inhaled Cayston. Each context includes
exact original Polish source lines supporting the product/formulation, indication
and route, plus PDF/text hashes and references to the dosing, safety and PK excerpts.
The English structured mappings remain explicitly unreviewed. These examples do
not assert that administering any medicine or combination is appropriate.

## Case coverage and expected behavior

| Cases | Count | Software expectation |
|---|---:|---|
| One stable adult per product | 7 | CrCl 25 mL/min; eGFR also calculated; no approved rule, so manual review |
| Meropenem CrCl probes: 9.99, 10, 10.01, 25, 25.5, 26, 50, 50.5, 51 | 9 | Preserve unrounded values; ask a reviewer to inspect printed ChPL bands and gaps; no dose inferred |
| AKI/unknown AKI, dialysis/unknown dialysis, unstable function, child, unknown equation sex, non-IDMS, uncertain body composition, stale/future lab, unsupported creatinine unit, extreme body size | 13 | No renal estimates; exact prototype policy warning |
| Missing weight, missing height, unconfirmed actual weight | 3 | eGFR available; CrCl unavailable; no automatic metric substitution |
| Equivalent mg/dL and micromol/L inputs | 1 | Same CrCl, within floating-point tolerance |
| Known creatinine uncertainty | 1 | CrCl interval 70/3 to 70/2.6 mL/min, with point estimate 25 |
| Invalid uncertainty bounds | 1 | No estimates; bounds warning |
| Two/three medication contexts | 2 | Separate results in request order; no interaction safety conclusion |
| Female equation input | 1 | CrCl 21.25 mL/min using the existing equation factor |
| Missing indication | 1 | Incomplete-context reason; no recommendation |

The expected results deliberately use a **source-only snapshot with zero approved
rules**. All drug results require manual review and contain no action, dose,
selected rule or prescription verdict. These are software assertions, not reviewed
clinical gold labels. A live database with different approved rules may produce
different results. The numerical gates (including seven-day freshness and BMI)
come from the app's prototype policy, not from ChPL clinical recommendations.

## Run and inspect

From the repository root:

```sh
python3 scripts/build-user-cases.py
cd core
go test ./internal/decision -run TestSyntheticUserCases -count=1 -v
go test ./internal/rules -run TestPolishDecimalGrounding -count=1 -v
```

The generator also exports individual API-shaped request bodies to
`outputs/user-cases/requests/`. It makes no API calls and changes no live patient,
assessment, document or rule records. The requests use fixed historical timestamps
so offline results are reproducible. For a present-time browser demonstration,
deliberately update the assessment and observation timestamps; preserve the stale
and future relationships for those scenarios.

The Go case test loads and verifies the pinned source PDF/text hashes, checks the
context quotations, strictly decodes each request, and asserts the calculator
metrics/warnings, CrCl values/uncertainty, medication ordering and absence of an
unsupported automated recommendation.

## Logic review findings

**Fixed: decimal-comma numeric grounding.** The source validator previously read
`1,5` as separate integer tokens `1` and `5`. This rejected a supported scalar 1.5
and could allow an unsupported integer 5 to pass its numeric-presence check.
`core/internal/rules/snapshot.go` now treats decimal commas and decimal points as
one numeric token in both quotations and dose text. Five regression scenarios
reproduced the failures before the fix and pass after it. They use fictional
evidence, never an approved real medicine. This remains a necessary syntax check;
number presence cannot establish dose-to-band association, correct units, clinical
meaning, or the absence of contraindications.

| Reviewed path | Behavior and evidence |
|---|---|
| Eligibility and exact context | `decision.Assess` uses APPROVED rules only and exact product, formulation, route and indication; existing lifecycle/context tests exercise rejection paths |
| Renal metric | CG and indexed eGFR remain distinct; missing CG never substitutes eGFR; existing metric tests plus the new missing-input cases |
| Boundaries and uncertainty | Unrounded matching; one unique interval; the entire known uncertainty interval must fit; existing threshold, gap and uncertainty tests |
| Prescription limits | Only explicit structured single-dose/interval limits; mismatched units require review; no total-daily-dose or interaction safety inference |
| Source and review | Snapshot verifies pinned bytes and quotes; live candidate creation additionally binds exact document/chunk/page/context |
| Approval | Pharmacist role, explicit confirmation, current digest and ACTIVE document required; reviewer identity is server-bound |
| Concurrent review | Transaction serialization and database range exclusion prevent overlapping approvals; competing approval tests |
| History and audit | Stored knowledge snapshots support replay; audit failure rolls back assessment; immutable-source and replay tests |

AKI or dialysis can appear in the renal warnings while a drug result says
`NO_APPROVED_RULE`: the engine checks rule eligibility before selecting a metric.
This is expected with the source-only suite. The full assessment must be inspected,
not only the first medication reason.

## Human review scenarios

| Role/action | Expected outcome |
|---|---|
| Physician enters one generated case | Inspect patient inputs and per-product results; do not interpret manual review as permission to prescribe |
| Pharmacist follows a context's `clinical_review_task` | Inspect complete original ChPL, all relevant tables/footnotes/cross-references and exact formulation; record an independent clinical answer separately |
| Admin or physician attempts rule approval | Forbidden; administrator privileges do not confer clinical approval |
| Pharmacist reviews a changed candidate using an old digest | Conflict; reload and inspect the current candidate |
| Pharmacist attempts approval without explicit clinical confirmation | Rejected |
| Pharmacist attempts an overlapping approved interval | Rejected, including under competing transactions |
| Candidate has unresolved hepatic, dialysis, loading-dose, blockade-depth, duration or daily-total conditions | Keep pending and record unmodeled conditions; the present structured model cannot establish these conditions |
| Reopen a saved assessment after later knowledge changes | Replay against its recorded snapshot; do not overwrite historical results |

Real ChPL rules remain unapproved. Existing automated tests exercise successful
approval and recommendation paths only with clearly fictional fixture medicines
in isolated test storage. Generating cases does not complete clinical validation.

## Executed verification — 2026-11-09

All 39 saved patient scenarios and all five decimal-grounding regressions passed.
`python3 scripts/test-integration.py` also passed the complete Go suite with race
detection and coverage against a uniquely named disposable PostgreSQL/pgvector
database. This includes HTTP/store review permissions, stale digests, concurrent
overlap exclusion, audit rollback, immutable sources and historical replay. The
runner removed its test container afterwards. No live clinical records were
approved or changed. Browser interaction and independent clinical validation were
not part of this run.
