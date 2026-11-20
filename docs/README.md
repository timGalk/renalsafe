# RenalSafe documentation

For a first test, follow the [installation instructions](../README.md#installation), then open the [clinician and pharmacist walkthrough](demo-guide.md#clinician-and-pharmacist-walkthrough). The walkthrough uses fictional patient data and works without an AI provider. Clinical rules still await independent review.

## Current guides

### Using the application

- [Staff accounts](accounts.md) covers activation, roles and password recovery.
- [Clinician workspace](final-workspace.md) describes the screens available to each role. [Patient history](patient-history.md) covers observations, corrections and saved assessments.
- [AI extraction](online-extraction-guide.md) explains how to configure OpenAI or NVIDIA and review suggestions against the source document.

### Clinical review and evaluation

- [Clinical review checklist](clinical-review.md) describes the pending rule and what a pharmacist needs to verify. [The synthetic case guide](user-cases-and-logic-review.md) describes the 39 fictional scenarios and their expected software behavior.
- [Calculator references](calculator-sources.md) and [Polish ChPL source records](polish-chpl-acquisition.md) document the equations and source documents used in the prototype.
- [Retrieval evaluation](evaluation-guide.md) covers the baseline and independent labels. The [model comparison guide](../experiments/model-comparison-v2/README.md) covers the paired experiments and ChPL pilot.

### Development and operations

- [Architecture](architecture.md) maps the application and database. [Development](development.md) covers host builds, tests and API regeneration.
- [Release and recovery](release-recovery.md) covers migration, backups and restore checks. [Release evidence](release-evidence.md) records which checks passed and which thesis questions remain open.

## Decisions and historical evidence

- [Architecture decision ADR-002](ADR-002-main-system.md) explains the original API and retrieval design. The current interface uses React.
- [Project roadmap](../.planning/ROADMAP.md) and [historical records](../.planning/history/README.md) track phases and dated verification, including the [earlier local-model research](../.planning/history/local-extraction-research-2026-11-05.md).
- [Thesis materials manifest](materials-manifest.json) lists checksums for the original design files stored in the sibling thesis project.
