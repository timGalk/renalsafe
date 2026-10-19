# ADR-002: main system and source evidence (2026-10-19)

Accepted by the user's implementation request following the proposed main-system/RAG scope. Software infrastructure may proceed from verified Phase 1 in parallel with clinical Phase 2, without activating unreviewed real rules. This revises the initial strict chronological roadmap dependency.

Keep deterministic Go decisions and source-bound approvals; add pgx/PostgreSQL transactional persistence and a minimal embedded browser client. The client is plain HTML/CSS/JS for a self-contained runnable slice; React may replace it against the stable API. PDF extraction uses installed Poppler in-process, not a new Python/LLM worker. Preserve section text and table columns rather than pretending to reconstruct every table's semantics.

Use exact metadata filtering before lexical/dense ranking. Start with deterministic extractive evidence responses. Optional local Ollama embeddings and pgvector cosine search add RRF; no generation framework, cloud patient prompts or automatic model downloads. Lack of a real embedding deployment cannot disable core assessments. Clinical approval and independent real-model evaluation are not replaced by software fixtures.

Database writes share a transactional advisory lock and append-only audit. It is intentionally conservative for this academic workload; optimize concurrency only with benchmark evidence. Migrations are content-hashed and immutable. Direct database-owner access can bypass application guarantees and needs separate deployment controls outside this local prototype.
