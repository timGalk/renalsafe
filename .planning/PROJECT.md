# RenalSafe

Academic CDSS for renal dose adjustment based on SmPC / ChPL. Synthetic patients only.

## Evidence and decisions

Project /home/apollo/Pwr/RenalSafe was empty on 2026-10-14; no existing code, Git history or GSD plans found. Materials are in ../Engineering-thesis.
Architecture revision 3 supersedes the older Java proposal: Go deterministic core, Python ingestion, PostgreSQL, React. The thesis draft and current user request supersede the proposal's automatic approval: all clinical candidates require pharmacist review. No LLM chooses a dose.

## Current scope

Implement domain, explicit units and scope limitations, CG and CKD-EPI 2021, strict JSON knowledge snapshots, approved-only exact product/route/form/indication/metric matching, boundary tests, cited template explanation, append-only hash-chained audit and offline CLI. Clinical approval and complete application are separate phases.
