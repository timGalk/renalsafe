# Documentation

Current implementation: Go deterministic engine/API, React workspace, PostgreSQL 18 with pgvector in Docker, local staff sessions and synthetic patient history. Phases 6–9 are implemented; clinical approval, assisted extraction and real-model evaluation remain open. AI rule extraction is planned locally; no model currently creates rules.

## Current guides

| Topic | Canonical document |
|---|---|
| Start/build/test | [Repository README](../README.md) |
| Architecture, repository layout and database | [Architecture](architecture.md) |
| Staff activation, roles and sessions | [Accounts](accounts.md) |
| Routes, forms and browser behavior | [Workspace](final-workspace.md) |
| Synthetic cases, labs and saved results | [Patient history](patient-history.md) |
| Synthetic demonstration and isolated training | [Demonstration](demo-guide.md) |
| ChPL-linked patient scenarios and decision/approval review | [User cases and logic review](user-cases-and-logic-review.md) |
| Clinical review requirements | [Clinical review](clinical-review.md) |
| Calculator references | [Calculator sources](calculator-sources.md) |
| Pinned Polish source acquisition | [Source provenance](polish-chpl-acquisition.md) |
| Retrieval baseline and evaluation | [Evaluation](evaluation-guide.md) |
| Proposed local Qwen/MedGemma extraction | [AI extraction plan](ai-rule-extraction-plan.md) |
| Uploadable Kaggle/Colab model comparison | [Experiment notebook and guide](../experiments/model-comparison/README.md) |
| Installation, migration and backup/recovery | [Release operations](release-recovery.md) |
| Executed acceptance and thesis boundaries | [Release evidence](release-evidence.md) |

## Decisions and historical evidence

[ADR-002](ADR-002-main-system.md) records the original persisted-API/extractive-RAG decision; its plain-browser choice has since been superseded by React. Dated verification and planning narrative are indexed under [.planning/history](../.planning/history/README.md). Historical counts/commands describe those checkpoints, not the current application. Phase plans, requirements and summaries remain in [.planning](../.planning/ROADMAP.md).

[materials-manifest.json](materials-manifest.json) records hashes of the original thesis/design inputs. Its paths are relative to the repository root and point to an external sibling project; these inputs are not runtime dependencies or bundled source files.

Generated browser bundles, dependencies, reports, screenshots and local working copies are ignored. Private credentials and database backups belong under `.local/`; never commit them. Immutable migrations, source PDFs/text and original provenance manifests are retained even when old: they support audit and historical replay.
