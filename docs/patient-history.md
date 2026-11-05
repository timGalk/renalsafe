# Synthetic patient and result history

Phase 7 provides clinician-managed synthetic cases with dated creatinine observations, saved assessments and result history. This is an academic workflow; a synthetic patient does not have a login. No real names, identifiers, birth dates or contact information are required. A case has a generated `SYN-…` code, optional fictional alias, owner, version and archived state.

## Use the workspace

1. Sign in as a physician. Open **Patients** after staff sign-in. Transitional local physician service-key access is also available.
2. Confirm the case is synthetic and create it with an optional fictional alias.
3. Record creatinine with its original units, sample date/time and IDMS status. Unknown IDMS remains unknown. Optional uncertainty requires both closed bounds containing the observation.
4. Choose **Use this observation**. The assessment screen fixes the selected laboratory fields; enter and confirm age, body measurements, medicine contexts and clinical scope for this assessment. Loading a synthetic example preserves the selected lab.
5. Calculate and save. The result and observation association commit with audit in one transaction. A failed/retried submission uses the same idempotency key; identical concurrent retries return the same assessment, while changed content under the same key conflicts.
6. Open **Saved results**, or use the case's assessment timeline, to reopen a result. Saved details are read-only. Replay checks the historical snapshot; **Prepare recalculation** creates a new assessment after explicitly choosing an observation and reconfirming inputs.

Switching to an unlinked assessment is explicit. Previous assessments are listed under their original creator/admin permissions; they are not grouped into synthetic cases by guessing from clinical inputs.

## Corrections, sharing and archive

**Correct observation** creates a replacement with a mandatory reason. The original observation, authorship, sample time and old assessment snapshots remain intact. An observation can have one direct correction, preventing concurrent branches; a replacement can itself be corrected. Superseded observations are shown in history and cannot be selected for a newly submitted assessment. An exact retry of an already committed assessment returns its original record.

The case owner (while a physician) or administrator can grant/revoke access through **Case access**. Active staff are selected from a bounded physician/pharmacist directory. Sharing applies to the case's existing history:

- Owner and assigned physicians can record observations and assess an active case.
- Assigned pharmacists can read, replay and export its results. They cannot author or manage the case.
- Administrators can inspect and manage access/archive; clinical authoring is not exposed through patient APIs. Administrative case, lab and result reads and exports are audited.
- Revocation removes access to linked result details, history, replay and exports, including results previously authored by that assignee. Role changes are checked too; a physician demoted to pharmacist retains only read access to their existing cases.

Owner/admin changes to alias/archive require the expected profile version. Archive prevents new observations and assessments; history remains available. An archived case can be restored through the same versioned update. Membership changes and authoring are serialized inside the existing transaction lock, so access is rechecked before a write commits.

## History and reports

Patient, observation and assessment lists use cursors (default 20, maximum 50 records). Results can be filtered by saved-time range, product text, outcome status and unlinked-only. Product/status filters must match the same medicine outcome when used together. Cursors use canonical database timestamps, including when an input sample contains finer timestamp precision.

**Compare snapshot** selects two results from the same synthetic case. The comparison shows saved inputs, renal estimates/units, outcomes, dose checks and engine/knowledge/rule/source versions. It does not infer clinical improvement, AKI or stability from changes.

**Export result JSON** exports one assessment's inputs, outcomes, citations, trace identifiers, patient code, observation and predecessor link. The immutable knowledge digest is retained; unrelated public KB rules and other patient history are not bundled. JSON export is limited to 4 MiB. **Printable report** downloads escaped HTML containing the saved inputs, renal outputs, per-medicine reasons/citations and version/audit identifiers. Open the file and use the browser's Print command to print or save as PDF. No server PDF renderer is required. Each export checks current read access and records an audit event. Downloaded files remain where the user saves them.

## Storage and API

Migration 004 adds synthetic profiles, case access, append-only observations and append-only assessment links. Database triggers prevent reassignment of case identity, modification/deletion of observations and result links, wrong-patient observation association and cross-case predecessors. Existing assessments, engine inputs/outputs, source bytes and migrations 001–003 are unchanged.

New patient assessment requests build the existing deterministic engine request using the selected stored creatinine, uncertainty, sample time and IDMS fields; submitted copies of those laboratory fields cannot override the selected observation. Current age/sex/weight/height, scope flags, medicines and regimen inputs remain explicit. The engine still owns calculation, eligibility, manual-review gates and cited decisions.

See `/openapi.json` for payloads and query parameters:

- `/api/v1/patients` and `/patients/{id}`: list/create/read/versioned update.
- `/patients/{id}/access` and `/access/{user}`: inspect/grant/revoke.
- `/patients/{id}/observations` and `/observations/{observation}/corrections`: history/append/correction.
- `/patients/{id}/assessments`: history/atomic assessment creation.
- `/api/v1/assessments` GET: authorised global/unlinked history; existing detail/replay paths now also apply case permissions.
- `/assessments/{id}/export` and `/report`: audited single-result exports.
- `/api/v1/staff`: active physician/pharmacist directory for sharing, without authentication secrets.

## Reproduce checks

```sh
python3 scripts/test-integration.py
# With the existing Playwright environment configured:
node scripts/smoke-patients.cjs
```

The integration suite uses a disposable database. The browser check creates synthetic cases/assessments under disposable staff accounts and disables those accounts afterwards; case/history/audit records remain deliberately intact. Its summary is saved in ignored `outputs/patient-smoke.json`. The browser check verifies creation, observation selection, saved results/reload, export, sharing/revocation, read-only pharmacist access, correction/recalculation, replay, comparison, archive and mobile layout.

The existing browser client implements this phase; the React workspace and stable client routes remain Phase 8. No automatic clinical flags, real-patient deployment, general lab panel or clinical validation is claimed.
