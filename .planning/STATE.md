---
gsd_state_version: "1.0"
milestone: v0.1
milestone_name: verifiable core and main system
progress:
  total_phases: 9
  completed_phases: 6
  total_plans: 14
  completed_plans: 11
current_phase: 4
current_phase_name: ingestion-review
status: ready-for-local-extraction
last_updated: 2026-11-05
---
# Current state

Phases 1, 3 and 6–9 are complete for the local academic application. Phases 2, 4 and 5 remain open. Staff password sessions, synthetic patient/lab history, saved result replay/export, the React workspace and local release/recovery tooling are implemented. Clinical approval and model-quality evidence are separate unresolved gates.

## Next work

1. Phase 4: local Qwen3.5-4B capability spike and pending-only extraction worker, using phases/04-ingestion-review/04-AI-SPEC.md and 04-02-PLAN.md. Local inference is user-confirmed; usable GPU memory/runtime and exact model artifacts remain unverified. MedGemma 1.5 4B is a comparison candidate, not a deployed model.
2. Phase 2: current Polish ChPL applicability, five independently reviewed product rule sets and independent expected outcomes. Never fabricate approval or activate machine suggestions automatically.
3. Phase 5: pinned real embedding/model comparisons and independent thesis experiments; approximately 20 products/200 synthetic cases remain the original target.

## Verification and boundaries

Phase 9 restored a consistent snapshot into disposable containers, matched all table fingerprints before/after restart, replayed all 36 saved assessments and passed all-role React acceptance. Fresh installation, populated legacy migration, database outage handling, administrator recovery and changed-migration refusal passed. Full Go database/race suite, vet/OpenAPI, frontend/Docker builds and recovery/evaluation checks passed. Current evidence: ../docs/release-evidence.md; operation: ../docs/release-recovery.md.

No real extraction model or clinical benchmark is claimed. Runtime database-owner privilege, off-host backup responsibility and mutable upstream build inputs remain documented limits. Private credentials/backups/output artifacts are ignored. Training fixtures and the standalone CLI remain supported features. Source PDFs, migration history and historical assessment snapshots are retained.

## Navigation

- Current documentation: ../docs/README.md.
- Roadmap: ROADMAP.md; machine-readable state: state.json.
- Detailed phase contracts and verification: phases/.
- Dated narrative and earlier verification: history/README.md. Historical filenames and commands do not define current entry points.
