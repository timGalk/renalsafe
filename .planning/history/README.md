# Historical evidence

Current state is [../STATE.md](../STATE.md); current user/developer guides are [../../docs/README.md](../../docs/README.md). These records retain earlier evidence without presenting outdated UI or setup instructions as current.

| Record | Scope |
|---|---|
| [System verification](system-verification-2026-11-05.md) | API/manual ingestion/retrieval checkpoints and account/patient/React acceptance through Phase 8 |
| [Two-drug run](two-drug-demo-2026-10-23.md) | Dated Polish source/API experiment, original numerical outcomes and execution limits |
| [Local extraction research](local-extraction-research-2026-11-05.md) | Superseded Qwen/MedGemma local-worker proposal; current extraction uses online providers |
| [State chronology](state-through-phase9-2026-11-05.md) | Original chronological planning/implementation notes through Phase 9 |
| [Phase 1 verification](../phases/01-decision-engine/01-VERIFICATION.md) | Canonical early engine verification, including formerly duplicated offline/install notes |

Paths inside dated prose are repository-root references unless stated otherwise. The retired `smoke-accounts.cjs`, `smoke-browser.cjs` and `smoke-patients.cjs` wrappers all delegated to `scripts/smoke-workspace.cjs`; use `RENALSAFE_SMOKE_SCOPE=accounts`, `review` or `patients` with that runner. The default scope is `all`. There is no browser access-key login in the current React application.
