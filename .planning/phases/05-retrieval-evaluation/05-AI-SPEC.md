# AI / retrieval design contract

## Framework and implementation

Direct Go + pgx + PostgreSQL full text/pgvector; no agent or generation framework. Deterministic extractive RAG first. Local Ollama /api/embed optional, model name and returned dimension pinned; original text not truncated silently. Evidence-only requests contain product and search terms, no structured patient record. No model automatically downloaded. No generative dosing/explanation model in this slice.

## Domain guardrails

Exact product/route/form/indication and ACTIVE source scope before retrieval. SmPC sections 4.2, 4.3, 4.4 and 5.2 default. Fragments are untrusted document data, never instructions. Returned answer composes source fragments with provenance and an explicit unverified/manual-review label. No action/dose recommendation, no APPROVED transition, no changes to engine results. Preserve table text, physical pages, byte offsets, source SHA and version. Numeric/source checks do not validate clinical semantics.

## Evaluation strategy

Automated software gates: no cross-context/version leakage; citations reproduce stored bytes; positive and empty lexical retrieval; known vectors show expected RRF ranking; provider failure and dimension/model mismatch degrade explicitly; search never activates pending rule; rollback prevents unaudited response. Test fixture vectors are not represented as a real embedding-model benchmark.

Independent clinical/evidence gold set remains open: measure recall@k and citation precision, wrong-source count, unsupported personal-dose outputs and latency. Compare full text against real BGE-M3 hybrid after a local model and reviewed queries exist. No assertion of clinical validity or model quality from fixture tests.

## Monitoring

Record actor, retrieval mode, corpus revision, query/context, chosen chunk IDs and model identity in audit. Do not log credentials. Expose readiness separately from liveness. Provider failure cannot stop core assessments; DB failure must stop successful assessment output.
