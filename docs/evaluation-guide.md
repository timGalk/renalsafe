# Retrieval evaluation

The runner compares source-grounded retrieval against explicit labels through the local authenticated API. It never creates or approves rules, imports sources, or starts/downloads models. Each search adds the normal durable audit event.

## Software baseline

Start the app and import the two Polish documents with the existing demonstration, then run:

```sh
python3 scripts/demo-two-drugs.py
python3 scripts/evaluate-retrieval.py data/retrieval-software-fixtures.json \
  --modes lexical --output outputs/evaluation/lexical-software.json
python3 scripts/test-evaluation.py
```

The six supplied cases check source identity, section/quote matching and no-hit behavior for two contexts. They are engineering fixtures, not an independent relevance or clinical gold set. They contain no patient dosing labels. Running the import demonstration also appends its synthetic assessments.

Credentials are read from `.local/tokens.json` (physician role), never printed or included in reports. The runner accepts only a loopback HTTP origin and rejects redirects. Use `--tokens` for another private local keys file and `--base-url` for a different local port.

## Independent labels

Copy `data/retrieval-software-fixtures.json` and replace the cases with independently reviewed query/evidence labels. Set `purpose` to `independent_evaluation` and add:

```json
"review": {
  "reviewer": "Actual reviewer identifier",
  "reviewed_at": "Actual review date",
  "method": "Actual labeling protocol, adjudication and exclusions"
}
```

These strings record supplied metadata; they do not establish the reviewer's qualification or independence. Never label software fixtures as independent clinical validation.

Every case needs a unique ID, exact product/form/route/indication, query, and `relevant` labels containing PDF SHA-256, section and an exact quotation. Use an empty label array with `expect_abstention: true` for an expected no-hit case. Positive labels must refer to an ACTIVE source in that exact context before execution. Labels should enumerate relevant evidence units; incomplete labeling can depress apparent precision. An optional `section` restricts the query. Document version identity is pinned through source hashes; source review and clinical approval remain separate.

## Local model comparison

Configure/index an independently installed local 1024-dimensional embedding model as documented in README. Then run both modes against the same labeled corpus:

```sh
python3 scripts/evaluate-retrieval.py path/to/reviewed-labels.json \
  --modes lexical hybrid --model-revision 'actual-immutable-weights-revision' \
  --k 5 --output outputs/evaluation/lexical-hybrid.json
```

`--model-revision` is an operator declaration, not weights attestation. Returned model identifiers are recorded independently. No real embedding benchmark is claimed by providing this runner.

Reports include dataset/corpus hashes, source metadata, requests, full cited responses, knowledge revisions, actual modes/degradation, model identity, individual metrics, aggregate metrics and latency. Keep reports local; reviewed queries and source text are included. Reports are written to ignored `outputs/` by convention.

- Recall@k counts distinct labeled evidence units found. Duplicate hits cannot inflate recall.
- Citation precision counts returned passages matching at least one label. This is relevance precision, not clinical validity.
- Reciprocal rank measures the first relevant hit. Mean metrics exclude undefined denominators and report no-hit cases separately.
- Wrong-source counts include hits outside the active exact-context document IDs, even when the PDF hash is reused for another context.
- Abstention means no retrieved passages. Dense retrieval may return low-relevance hits; this metric exposes that behavior without assuming a clinical meaning.
- Latency reports median and nearest-rank p95 including the API's transaction/audit overhead. This small harness is not a production load test.

A run is incomplete if knowledge revisions/model identity change or the active corpus differs at the end. Hybrid fallback is explicitly counted and invalidates the requested comparison. Exit 0 means the experiment ran consistently, not that quality met a clinical acceptance threshold; exit 2 means an incomplete/degraded comparison (with a saved report). Invalid datasets/API failures exit unsuccessfully without presenting partial results as complete. No E1–E3/E5 thesis completion, unsupported generative answer score or clinical unsafe-error rate is inferred from retrieval labels.

## Passage splitting and existing sources

New imports partition oversized sections at paragraph boundaries with a soft 6000-byte target. Adjacent detected table paragraphs, their immediate introduction and possible following footnote remain in one contiguous source span, including page breaks. Text is not duplicated, trimmed, normalized or supplemented with invented headers. Each passage retains section identity, physical pages and exact immutable byte offsets.

Layout detection is heuristic. A table/paragraph too large or ambiguous to split stays whole, potentially exceeding the target. The provider still uses `truncate:false`, so indexing can fail explicitly. This does not implement OCR, semantic table reconstruction or a guaranteed model token budget. Previously stored chunks and citations are unchanged; reindexing an existing import does not rechunk it. Do not reimport clinical sources merely to bypass review of source/candidate applicability.
