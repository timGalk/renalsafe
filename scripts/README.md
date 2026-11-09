# Operational and verification scripts

Run commands from the repository root. Python uses the standard library unless a guide explicitly states otherwise. Browser checks require a configured Playwright/Chromium installation. For initial setup and build prerequisites see [README](../README.md); for data-sensitive release actions use [recovery instructions](../docs/release-recovery.md).

| Task | Canonical command |
|---|---|
| First local configuration | `python3 scripts/bootstrap-local.py` |
| Staff activation/reset | `python3 scripts/setup-accounts.py USER --output .local/NEW_FILE.json` |
| Import/check two-drug synthetic example | `python3 scripts/demo-two-drugs.py` |
| Generate source-linked synthetic patient cases | `python3 scripts/build-user-cases.py` ([cases and logic review](../docs/user-cases-and-logic-review.md)) |
| Retrieval experiment | `python3 scripts/evaluate-retrieval.py DATASET --modes lexical --output outputs/NEW_REPORT.json` |
| Evaluation unit checks | `python3 scripts/test-evaluation.py` |
| All Go database/race checks | `python3 scripts/test-integration.py` (build frontend first) |
| All-role React acceptance | `node scripts/smoke-workspace.cjs` |
| Account browser check | `RENALSAFE_SMOKE_SCOPE=accounts node scripts/smoke-workspace.cjs` |
| Patient/history browser check | `RENALSAFE_SMOKE_SCOPE=patients node scripts/smoke-workspace.cjs` |
| Clinician/source/review browser check | `RENALSAFE_SMOKE_SCOPE=review node scripts/smoke-workspace.cjs` |
| Backup | `python3 scripts/recovery.py backup .local/backups/NEW_DIRECTORY` |
| Isolated restore acceptance | `python3 scripts/recovery.py verify .local/backups/DIRECTORY --browser` |
| Recovery safety checks | `python3 scripts/test-recovery.py` |
| Disposable installation/migration/failure checks | `python3 scripts/check-installation.py` (build release image first) |
| Public release identities | `python3 scripts/release-manifest.py --output outputs/NEW_MANIFEST.json` |
| Prepare privileged admin recovery | `python3 scripts/prepare-admin-recovery.py --output .local/NEW_DIRECTORY` |

`browser-fixtures.cjs` and `recovery.py` also supply shared helpers to the runners; they are not redundant copies. The three former `smoke-accounts.cjs`, `smoke-browser.cjs` and `smoke-patients.cjs` aliases have been removed; their scopes already exist in the canonical browser runner.

Browser checks create synthetic records and disable their fixture users afterwards; audit/history deliberately remain when run against the normal workspace. Prefer the isolated restored application for full release acceptance. `RENALSAFE_BASE_URL` selects a loopback test origin. `PLAYWRIGHT_MODULE` and `RENALSAFE_CHROMIUM_PATH` can select an existing browser-test installation. Browser results use `outputs/react-workspace-smoke.json` for every scope.

Setup/demo/evaluation scripts still require private service keys; these are supported automation interfaces, not the removed browser key-login UI. Never print or commit them. Backup/recovery commands refuse overwrites, and the automatic restore drill does not overwrite the live database. See the recovery guide before performing an actual operator cutover.
