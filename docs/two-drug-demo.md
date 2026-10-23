# Two-drug synthetic demonstration — 2026-10-23

The local application is running at http://127.0.0.1:8080/?example=two-drugs . Two official Polish ChPLs are now imported into the persisted database with explicit demonstration contexts: Meronem 1 g and Ceftazidime Kabi 1000 mg, intravenous route, hospital-acquired pneumonia. These metadata choices are for source inspection and a synthetic renal-assessment demonstration, not a recommendation to co-prescribe the products.

No clinical rule was created, edited or approved. Both new products intentionally return `MANUAL_REVIEW_REQUIRED` / `NO_APPROVED_RULE`, without an action or dose. The existing Pradaxa candidate stays PENDING. An ACTIVE source document does not imply an APPROVED clinical rule.

## Reproduce

```sh
docker compose up -d core
python3 scripts/demo-two-drugs.py
```

The script reads role keys from `.local/tokens.json` without printing them, verifies the pinned PDF hashes, imports missing exact-context documents through the authenticated API, checks source downloads and section 4.2, then saves six synthetic assessments and replays each. It reuses unchanged ACTIVE imports, refuses conflicting versions or existing approved rules for the example contexts, and confirms that all rule records are unchanged afterward. Each rerun appends new synthetic assessment and audit records.

Request fixture: `data/demo-two-drugs.json`. Its fixed timestamps support offline reproducibility; the live script supplies a fresh assessment time and a sample one hour earlier. No patient identifiers or real patient data are used.

## Observed results

| Synthetic case | Expected and observed outcome |
|---|---|
| Male, 70 years, 72 kg, 175 cm, creatinine 2.8 mg/dL | CrCl 25 mL/min; eGFR 23.53527 mL/min/1.73 m² |
| Equivalent creatinine 247.52 µmol/L | Same renal estimates |
| Creatinine 1.0 mg/dL | CrCl 70 mL/min; eGFR 80.96714 mL/min/1.73 m² |
| Suspected acute kidney injury | No renal estimates; `AKI_OR_UNKNOWN` |
| Missing weight | eGFR available; CrCl absent; `CG_WEIGHT_REQUIRED` |
| Creatinine bounds 2.6–3.0 mg/dL | CrCl interval 23.33333–26.92308 mL/min |

Every case returns two correctly ordered product results requiring manual review, has a persisted audit hash, is retrievable and reproduces from its saved snapshot. These cases verify software behavior, not clinical thresholds or prescribing safety.

For each drug, the Polish query `nerek` restricted to section 4.2 returns the correct source passage and PDF hash, without cross-product evidence. Mode is lexical with `LOCAL_EMBEDDER_NOT_CONFIGURED`; no semantic-model quality claim. The server's audit endpoint verifies the chain and all six assessment events are present. Two consecutive runs confirmed source import reuse.

Full results, requests, source-retrieval responses and audit summary are in ignored `outputs/two-drug-demo/`. Sources and original hashes remain pinned in `data/sources/pl-2026-10-23/manifest.json`.

## Interface and checks

Sign in with the local physician key and choose **Load two-drug example**, then **Calculate and save assessment**. The URL above presets both products after sign-in if their exact source contexts are available. Use **Find source passages** to inspect evidence and **Verify reproducibility** to replay the result. The default source query covers English `renal` or Polish `nerek`.

All 57 top-level Go tests pass with `-count=1 -race -cover` against a dedicated disposable PostgreSQL+pgvector database. JavaScript syntax, Python compilation, embedded asset/CSP checks and application build pass. Health/readiness and served English preset assets are checked after deployment.

Browser automation was blocked by the browser URL security policy in this session. The new preset was not visually tested end-to-end; verification here is through APIs, static assets and package tests. No alternative browser automation was used to bypass the block.

A single `docker stats --no-stream` sample around the live run showed 43.89 MiB for the Go application and 68.7 MiB for PostgreSQL (about 113 MiB together). This excludes the browser, Docker daemon and build-time memory and is not a peak-load or capacity benchmark.
