---
phase: 01
status: complete
nyquist_compliant: true
---
# Validation strategy

Goal: demonstrate deterministic software behaviour, not clinical accuracy. Tests are offline and require no GPU, LLM, database or network.

- Domain: finite ranges, inclusion/exclusion at exact and adjacent representable values, JSON duplicate/unknown keys.
- Calculator: independent 60-digit decimal piecewise references, exact CG cases, unit equivalence; scope, missingness, freshness, unknown booleans, body-size and uncertain bounds.
- Rules: missing/stale human metadata, source hash/version identity, quote and threshold grounding, overlap, lifecycle, duplicate IDs, path escapes and synthetic KB rejection.
- Decision: strict context/metric, pending/rejected/superseded ineligibility, gaps, uncertainty crossing range, no CG→eGFR substitution, deterministic frozen snapshots, candidate remains PENDING.
- Audit/CLI: append chain, immutable prior events, replay, altered event, partial tail, writer lock, invalid source/input and persistence errors suppress output.

Commands: `cd core && go test -race -cover ./... && go vet ./...`. Independent references regenerate from `core/internal/renal/testdata/reference_decimal.py`. No human clinical approval inferred from this verification.
