# AI-SPEC — Phase 4: source-grounded rule extraction

Updated 2026-11-19 against the implementation. The online extraction decision of 2026-11-09 supersedes the local-only Python/Ollama worker proposal. Current setup, provider settings and executed checks are documented in the [online extraction guide](../../../docs/online-extraction-guide.md). Earlier model research is [historical evidence](../../history/local-extraction-research-2026-11-05.md).

## Implemented architecture

- An in-process Go worker uses the `extraction.Provider` interface with OpenAI Responses or NVIDIA Chat Completions adapters. A missing key disables the selected provider; keys are never shared between providers or sent to the browser. No local GPU, Python worker or extraction framework is required.
- PostgreSQL migration 005 adds durable extraction jobs and candidate provenance. Pharmacists and administrators can request, inspect, cancel or dismiss jobs and explicitly prepare PENDING candidates. Pharmacist approval remains a separate operation.
- Only the selected public document and exact product context are sent to the configured provider. Patient records are excluded. Source instructions are treated as data; models receive no tools or approval authority.
- The default source mode preserves the complete selected text. Experimental hybrid retrieval is opt-in, records coverage and fallback reasons, and adds an unresolved review condition when text is filtered. It does not establish complete extraction.
- Model responses are validated against the structured envelope, pinned document identity, source spans, exact quotations and numeric evidence. Invalid or incomplete responses fail visibly. Quote presence alone does not establish correct interpretation or clinical completeness.
- Job completion stores suggestions separately from executable rules. Explicit candidate saves pass through existing rule validation and preserve job/finding provenance; duplicate saves are idempotent. Unrepresented loading, relative-dose, dialysis or other conditions remain unresolved and prevent approval.
- Jobs record requested/returned model identity, response identity, source and prompt/schema digests, timing and available usage. Secrets are excluded from job/audit output. Provider configuration changes invalidate stale queued jobs.

Implementation: `core/internal/extraction/`, `core/internal/store/extraction.go`, `core/internal/httpapi/extraction.go`, and `web/src/extractions.tsx`. Paths are relative to the repository root.

## Verification and remaining gates

Mock-provider and database tests cover malformed/truncated output, source grounding, authorization, cancellation, interrupted jobs, stale source/configuration, duplicate requests/saves and PENDING-only conversion. A real OpenAI Meronem development check is recorded in the runtime guide. These checks do not establish clinical accuracy or complete extraction.

Remaining acceptance requires:

1. Independently reviewed source cases and expected findings against complete original PDFs. Do not use either model's predictions as reference labels or invent reviewer provenance.
2. Development and held-out groups separated by product/document and related versions/translations; freeze prompt, schema, source hashes and settings before test evaluation.
3. Whole-rule and field correctness, evidence support, omitted qualifiers, unsupported additions, abstention, human correction time and latency. Include invalid output, truncation and provider failures in denominators; undefined scores remain undefined.
4. Reviewed harder-table/page fixtures preserving headings, footnotes, product boundaries and exact spans. Scanned PDFs, OCR and vision reconstruction require separate provenance and fidelity acceptance.
5. Reproducible provider/model, source, dataset, prompt/schema and software identities. Hosted model names are not a cryptographic attestation of model weights. Report development smoke checks separately from independently reviewed benchmarks.

The [paired comparison experiment](../../../experiments/model-comparison-v2/README.md) supports annotation, frozen protocols, grouped splits and blinded review. Its Qwen/MedGemma inference is an optional experiment, not an application dependency or a selected clinical model. No model may approve a rule or select a patient dose.
