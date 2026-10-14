---
phase: 01-decision-engine
plan: "02"
subsystem: core
provides: ["Calculators and uncertainty"]
completed: 2026-10-14
execution: inline-skill-fallback
---
# Calculators and uncertainty

CG actual-weight and CKD-EPI 2021 implemented; explicit conversion, adult/stability/AKI/dialysis/standardization/body-size/freshness gates. Six independent 60-digit numeric reference points and safety tests pass. Unrounded threshold matching and interval uncertainty tested.

## Validation

`go test -race -cover ./...` passed; `go vet ./...` passed. Local Go SDK 1.25.1 retrieved from official go.dev into the task workspace because the host had no Go on PATH. Staging build used `-buildvcs=false` due to the chat's inaccessible placeholder .git. Build in installed repository is checked separately.

## Limits and deviations

No clinical approval, clinician identity, complete five-drug dataset, API/database, UI or LLM integration fabricated. Plans and execution performed inline under GSD skill fallback; no multi-agent guarantees claimed. Independent clinical validation is Phase 2. Local audit is a prototype journal; authenticated persistence and external anchoring remain Phase 3.
