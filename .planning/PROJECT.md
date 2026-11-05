# RenalSafe

Academic CDSS for renal dose adjustment based on SmPC / ChPL. Synthetic patients only.

## Evidence and decisions

Project /home/apollo/Pwr/RenalSafe was empty on 2026-10-14; no existing code, Git history or GSD plans found. Materials are in ../Engineering-thesis.
Architecture revision 3 supersedes the older Java proposal: Go deterministic core, Python ingestion, PostgreSQL, React. The thesis draft and current user request supersede the proposal's automatic approval: all clinical candidates require pharmacist review. No LLM chooses a dose.

## Current scope

Implemented: deterministic Go renal engine and offline CLI; PostgreSQL persistence, role-bound review and immutable audit/replay; staff password sessions; synthetic patient/lab/result history; React workspace; local installation, migration and verified backup/restore. Current documentation: ../docs/README.md. Current state and evidence: STATE.md and ../docs/release-evidence.md.

Open: independent clinical approval and gold cases (Phase 2), local pending-only Qwen/MedGemma extraction and harder-PDF fidelity (Phase 4), real-model/independent thesis evaluation (Phase 5). Local inference is user-confirmed. Synthetic patients only; no patient-facing portal or automatically activated machine rules.
