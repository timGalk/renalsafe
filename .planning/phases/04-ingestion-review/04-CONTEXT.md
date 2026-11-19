# Current ingestion and extraction context

Updated 2026-11-19. Go/PostgreSQL provides bounded Poppler ingestion, immutable source passages, manual and assisted candidate preparation, authenticated review and audit. The React workspace exposes these workflows. All real candidates require explicit pharmacist approval; clinical Phase 2 remains open.

The online extraction decision on 2026-11-09 superseded the earlier local-only proposal. The application now has an in-process Go worker, OpenAI/NVIDIA provider adapters, durable jobs, validated suggestions and explicit PENDING saves. There is no Python extraction worker or required local inference runtime. Optional local Ollama embeddings serve evidence retrieval independently.

Current implementation and configuration: [online extraction guide](../../../docs/online-extraction-guide.md). Remaining work: [04-02-PLAN.md](04-02-PLAN.md), governed by [04-AI-SPEC.md](04-AI-SPEC.md). Independent source labels, harder-table/OCR fidelity and model-quality evaluation remain open. Preserve relative-dose, loading, footnote, dialysis and renal-method uncertainty; never discard a condition to fit executable fields.

Earlier Qwen/MedGemma research is retained in [history](../../history/local-extraction-research-2026-11-05.md). The [paired notebook](../../../experiments/model-comparison-v2/README.md) is a separate experiment, not the deployed extraction architecture. The original 2026-10-19 manual-ingestion slice and its verification remain recorded in the phase summaries.
