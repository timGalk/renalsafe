# Phase 9 release evidence — 2026-11-05

> **Dated checkpoint.** These checks ran on 2026-11-05 against migrations 001–004. Migration 005 (online extraction jobs), the online OpenAI/NVIDIA extraction workflow and later React changes (including the 2026-11-18 saved-result and inline-source updates) were added afterwards and have not yet been covered by a repeated installation, recovery and all-role browser run. Re-run `scripts/check-installation.py`, `scripts/recovery.py verify … --browser` and `scripts/smoke-workspace.cjs` before citing these results for the current revision.

This report records executed engineering checks for the local academic RenalSafe application. It is not evidence of clinical validation, model quality or production suitability. Implementation and installation instructions: `release-recovery.md`.

## Executed checks

| Check | Observed result |
|---|---|
| Frontend generated API types, strict TypeScript and production build | PASS |
| Container application build | PASS; React build and Go binary embedding verified; cached dependency layers reused, not an asserted clean-room or bit-reproducible build |
| Full Go suite against independent PostgreSQL/pgvector with race detection | PASS; transactional failure, concurrency, role/session/permission, immutable history and replay coverage included |
| Go vet and regenerated OpenAPI equality | PASS |
| Retrieval software-fixture unit tests | PASS, 7 tests; not independent clinical/model evaluation |
| Recovery safety regressions | PASS, 5 tests: corruption, unexpected paths, incomplete bundles, public output path and overwrite refusal |
| Actual local workspace snapshot | PASS; one exported PostgreSQL snapshot shared by custom dump and all-table canonical fingerprints; private setup files copied separately |
| Recovery of populated workspace | PASS; every public table count/hash matched; identical after DB restart; stored PDFs, rules, cases/labs, results, credentials and audit included |
| Recovered application validation | PASS; migrations idempotent, restored sessions/reset links cleared, audit valid, all 36 saved assessments exported and historically replayed |
| All-role React acceptance on restored clone | PASS; physician/pharmacist/admin account, patient/lab/history/export/replay/comparison/source-review and session/role checks |
| Fresh installation | PASS; actual bootstrap, new DB, migrations 001–004, PENDING-only seed, first assessment/replay, unchanged tables on repeat startup |
| Populated legacy schema upgrade | PASS; 001/002 database with actual source/result fixture upgraded to 003/004; original source/chunk/rule/result data unchanged, audit chain valid, historical replay reproduced |
| Dependency failure | PASS; disallowed DB connections leave application liveness available, readiness 503, assessment requests unsuccessful, no extra assessment committed; reconnect restores audit/replay |
| Privileged administrator recovery | PASS on disposable fixture; new identity provisioned/audited, staff administration restored, previous disabled admin stays disabled |
| Changed migration protection | PASS; application refuses startup when recorded migration digest is altered |

Recovery/browser tests used uniquely named disposable containers and loopback ports. No live patient/result/rule data was altered by the recovery drill. Test browser users/cases/candidate clones exist only in the restored clone, which was removed. Browser fixture clinical candidates were rejected; no rule was approved. The original live service was not restarted or replaced by this work.

The first backup attempt failed on a SQL sorting expression and was corrected; only a bundle with a completion manifest and a successful verification report counts as evidence. Earlier outage/image-selection harness failures were corrected before acceptance. The final automated checks use the rebuilt release image identified in private reports; they do not infer binary identity from a previously running container's mutable image tag.

## Reproducibility artifacts

- Private snapshot and successful verification reports: `.local/backups/phase9-20261105-v2/`. These contain sensitive configuration/hashes and must not be committed or submitted with the thesis.
- Installation/migration/outage/fixture-restore reports: `.local/release-checks/renalsafe-install-*/verification.json` for successful runs. Failed directories without a report are not passes.
- Browser checks and screenshots: ignored `outputs/react-workspace-smoke.json` and `outputs/react-*.png`. These are generated on disposable restored cases.
- Public source/build identity manifest: generate with `scripts/release-manifest.py`; source/lock/migration/document hashes, host tool versions and actual image IDs. No credentials or raw assessment inputs are emitted by this manifest.
- Repeatable runners: `scripts/test-integration.py`, `scripts/test-evaluation.py`, `scripts/test-recovery.py`, `scripts/check-installation.py`, `scripts/recovery.py` and `scripts/smoke-workspace.cjs`.

Timestamps and concrete source/binary identities belong to the generated manifests. Backups capture a point in time: this is not continuous replication, point-in-time recovery, zero data loss, authenticated backup signing or a measured recovery-time service guarantee. Off-host encrypted copies and image archives remain operator responsibilities. No operating-system crash/power-loss simulation was performed.

## Current thesis completion reconciliation

Updated 2026-11-19 against the implementation. The executed Phase 9 checks above remain a dated checkpoint and do not certify later extraction changes. Extraction-specific verification and the development smoke check are recorded in the [online extraction guide](online-extraction-guide.md).

| Area | Current evidence | Remaining work |
|---|---|---|
| Deterministic core / persisted API | Implemented and regression-tested | Clinical source coverage remains separate |
| Staff accounts / synthetic history / React UI | Implemented; normal and restored multi-role acceptance passed | Real-patient deployment and patient-facing accounts are outside confirmed scope |
| Application delivery (REL-01, Phase 9) | Local installation, migration, recovery drill, failure checks and operation/evidence documentation delivered | Repeat verification for each delivered source/image and keep off-host backup copies |
| Clinical knowledge (Phase 2) | Pinned source documents and pending candidate workflow | Five independently reviewed product rule sets, regulatory applicability and independent gold labels |
| Assisted ingestion (Phase 4) | Manual ingestion, passage splitting, online OpenAI/NVIDIA extraction jobs, suggestion review and explicit PENDING saves implemented | Independent reviewed examples, model-quality evaluation and harder-table/OCR fidelity evaluation |
| Retrieval/models/thesis experiments (Phase 5) | Scoped lexical/optional hybrid software and engineering evaluation runner | Actual pinned model comparisons and independent clinical/evidence cases; original approximately 20-product/200-case thesis target remains open |

The application can demonstrate source storage, manual preparation/review, renal calculations and reproducible synthetic case history. Cases lacking eligible approved rules return manual review. It cannot be described as a validated autonomous dose-extraction/recommendation system. Online extraction was added after the Phase 9 checkpoint; it produces suggestions for human review and does not establish clinical accuracy.

## Thesis-ready description of the implemented scope

The implementation separates deterministic assessment from clinical knowledge preparation. Source documents, extracted passages, structured rule candidates, review decisions and assessment snapshots are versioned and linked through durable audit records. Staff authenticate through server-side sessions and work with synthetic patient histories. A recovery experiment restored a consistent database snapshot into an isolated deployment, verified complete table fingerprints before and after restart, and reproduced all 36 historical assessments. Separate checks verified fresh installation, additive migration of populated legacy data, fail-closed database dependency behavior and multi-role browser workflows. These results support the reproducibility of the implemented application workflow; independent clinical validation and model-quality evaluation remain future work.
