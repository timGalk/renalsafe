# Online ChPL extraction

The application now supports online document interpretation through the OpenAI
Responses API. The selected default model is **`gpt-5.4-mini`**. Extraction produces
cited suggestions, and an explicit review action can save a PENDING rule. It never
approves a rule or changes a patient assessment.

## Configure

Add the API key to the local, Git-ignored `.env` file on the server:

```dotenv
OPENAI_API_KEY=your-api-key
RENALSAFE_OPENAI_MODEL=gpt-5.4-mini
```

Keep the existing database settings. Do not put the key in browser code, committed
files or chat messages. The model variable is optional: the server uses
`gpt-5.4-mini` when a key is configured without a model override. With no key,
online extraction is disabled and manual document/rule workflows remain available.

Rebuild and start:

```sh
docker compose up -d --build
```

The server's explicit startup migration adds extraction job/candidate tables.
No additional model package, Python worker or local GPU is required. The key is
passed only to the server container and used for the fixed HTTPS OpenAI endpoint.

### NVIDIA-hosted Kimi K3.1 Pro

To select NVIDIA instead of OpenAI, configure the ignored `.env` file:

```dotenv
RENALSAFE_EXTRACTION_PROVIDER=nvidia
NVIDIA_API_KEY=your-nvidia-api-key
RENALSAFE_NVIDIA_MODEL=moonshotai/kimi-k3.1-pro
RENALSAFE_EXTRACTION_RETRIEVAL=full
```

The default provider remains `openai`. The selected provider uses only its own
key; a missing NVIDIA key disables NVIDIA extraction rather than sending an
OpenAI key to NVIDIA. The review screen names the document recipient. The NVIDIA
adapter uses the fixed `https://integrate.api.nvidia.com/v1/chat/completions`
endpoint, `reasoning_effort: low`, `stream: false`, and `max_tokens: 10000`.
The model ID above is the default in NVIDIA's
[Kimi API reference](https://docs.api.nvidia.com/nim/reference/moonshotai-kimi-k3_1-pro-infer),
even though the Build page URL uses an underscore.

The NVIDIA reference does not advertise `response_format` for this model. Its
adapter therefore supplies the same JSON schema in the system instructions and
strictly validates the returned JSON and exact source evidence locally. It does
not claim provider-enforced structured output. Refusals, fenced/non-JSON output,
truncation, unexpected tool calls, multiple choices, model mismatch and unsupported
facts fail the job. As with OpenAI, local validity does not establish completeness.
Redirects and automatic retries are disabled. An asynchronous HTTP 202 fails
visibly; the adapter does not poll it. The existing 90-second HTTP timeout and
95-second job deadline still apply. Result provenance records provider, model,
response ID, usage and retrieval coverage; provider protocol changes are included
in the manifest hash and invalidate previously queued jobs.

Run the same two-request comparison without changing the running server:

```sh
python3 scripts/smoke-online-extraction.py --provider nvidia --compare
```

The NVIDIA response's prompt/completion token counts are normalized to the same
input/output usage fields used by the review screen and comparison report. Do not
assume NVIDIA supports the OpenAI-specific `store: false` setting; it is not sent.
Only the public product document is transmitted, as in the OpenAI workflow.

On 2026-11-17, local adapter tests passed, but the live requests returned HTTP 404.
The configured key could list 32 NVIDIA models, with no Kimi entry; an additional
tiny check of the underscore spelling also returned 404. No Kimi extraction or
quality comparison completed. Account/model availability must be resolved before
claiming a successful live test.

### Other NVIDIA models

The same adapter can select these model IDs through `RENALSAFE_NVIDIA_MODEL` or
the smoke runner's `--model` option:

| Model | NVIDIA ID | Sent reasoning setting |
| --- | --- | --- |
| Mistral Large 4 | `mistralai/mistral-large-4-1t-a52b` | `high` (its supported choices are `high`/`none`) |
| Muse Glimmer 30B | `meta/muse-glimmer-30b` | `low` |
| GLM 5.3 Flash | `z-ai/glm-5.3-flash` | Omitted; provider default |

These settings follow NVIDIA's model-specific references for
[Mistral](https://docs.api.nvidia.com/nim/reference/mistralai-mistral-large-4-1t-a52b-infer),
[Muse](https://docs.api.nvidia.com/nim/reference/meta-muse-glimmer-30b-infer), and
[GLM](https://docs.api.nvidia.com/nim/reference/z-ai-glm-5-3-flash-infer).
Sampling parameters stay at each provider's defaults. All use the same source,
extraction instructions, local validation, 10,000 generated-token cap and existing
HTTP deadline. Reasoning settings therefore differ across models; this is a test
of the configured extraction pipeline, not a controlled model-capability ranking.

```sh
python3 scripts/smoke-online-extraction.py --provider nvidia --model mistralai/mistral-large-4-1t-a52b --compare
python3 scripts/smoke-online-extraction.py --provider nvidia --model meta/muse-glimmer-30b --compare
python3 scripts/smoke-online-extraction.py --provider nvidia --model z-ai/glm-5.3-flash --compare
```

Each command makes two requests and leaves the server configuration unchanged.
For longer diagnostic runs, append `--timeout-seconds 300` to allow five minutes
per request. This override is confined to the live test process; the production
HTTP timeout and worker deadline are unchanged. The runner accepts 1–600 seconds,
adjusts the enclosing test deadline, and records the requested timeout, response
header timing and sanitized connection-error category in each comparison report.
Reports include requested/returned model, HTTP status and generation settings,
including when the model returned text that failed local validation. A failed
validation has no successful completeness score; do not interpret an empty check
map as passing. Token comparisons remain available for returned responses with
the same source and returned model, even if their extracted content was rejected.

On 2026-11-17, NVIDIA listed all three models, but each full-input and retrieval
request reached the 90-second client limit without a response. A separate
32-token Muse diagnostic with reasoning disabled also timed out after 30 seconds.
No token usage, completeness score or quality comparison was obtained. The local
extraction/model-configuration tests passed; inference availability/latency remains
unresolved. The six extraction attempts and diagnostic are summarized under
`outputs/extraction-comparison/nvidia-three-models-20261117/REPORT.md`.

A subsequent user-supplied Mistral image example succeeded in both streaming
(first event 0.76 seconds, completed in 22.6 seconds) and non-streaming mode
(27.3 seconds). Thus the earlier document-request timeouts do not establish model
unavailability or a requirement to use streaming. Large-document extraction
latency and quality remain unresolved. Exact diagnostic inputs and timing are
recorded in `outputs/nvidia-stream-diagnostic/REPORT.md`.

Repeating the three models' full/retrieval comparisons with a test-only
300-second HTTP limit also produced no responses for all six completed document
requests. Sending the exact large Mistral request independently through curl
reproduced the 300-second timeout, so changing HTTP clients alone did not resolve
it. In separate user-supplied curl samples, Gemma 4 31B correctly returned 391 in
52.8 seconds, Muse correctly selected 9.8 in 118.7 seconds, and GLM reached the
300-second limit. These small-task successes do not validate document extraction;
the document-request cause and clinical quality remain unresolved. Interrupted
local attempts are recorded separately from completed timeout tests. See
`outputs/extraction-comparison/nvidia-five-minute-20261117/REPORT.md`.

## Use

### Optional retrieval to reduce input

Set `RENALSAFE_EXTRACTION_RETRIEVAL=hybrid` in the server environment (also
forwarded by Compose). The default is `full` until retrieval recall has been
validated on independently reviewed documents. This setting selects exact source
blocks before the OpenAI request; it does not change the saved original source.

Selection combines BM25 over English/Polish renal and dependency word stems with
cosine similarity from the existing local embedding provider, then reciprocal rank
fusion. Bounded min-heaps retain up to 12 candidates per channel and up to 12
optional fused candidates, with deterministic source-order ties. To use the
semantic channel, configure `RENALSAFE_OLLAMA_URL` and `RENALSAFE_EMBEDDING_MODEL`
as for evidence search. Without a local embedder, the report explicitly records
lexical mode. Embeddings are computed in batches for each job; existing database
embeddings are section-level and are not reused for these paragraph blocks.

Complete sections 4.1, 4.2 and 4.3 are mandatory. Every lexical renal match retains
two neighboring blocks on each side within the same source passage. Passages
containing detected numeric tables are retained in full. Optional hits expand
with the same neighbor window; overlapping windows are deduplicated by occurrence
ID, and all retained text stays in source order with original byte offsets.
The target is **48,000 serialized source bytes**, not an exact token limit, and
includes selection coverage metadata. Optional windows that do not fit are
skipped whole. Mandatory context exceeding that target, no lexical renal anchors,
or a failed/invalid semantic provider causes full-input fallback, still subject
to the existing 120,000-byte hard limit. Selection also falls back to full input
when it would not reduce the payload. Semantic retrieval has a 10-second total
deadline within the existing extraction job deadline; it makes no paid API calls.

This is an experimental recall/cost tradeoff. Heuristic table and dependency
recognition does not establish exhaustive coverage. Every filtered finding gets
an unresolved full-document review requirement, including dependencies beyond the
retained windows. An abstention applies only to the selected passages. The result
records selection mode, fallback reason, exact evidence IDs, input digest, block
and byte counts, and provider-reported input/output/cached-token usage when
available. The review screen displays usage and coverage. Retrieval settings and
embedding-model identity are pinned in the job's prompt/schema hash.

An offline check on the saved Meronem input reduced serialized source content from
23,625 to 21,187 bytes (about 10%) using lexical selection. This is not a measured
token/cost saving or a clinical recall benchmark; fixed prompt/schema overhead,
output size and prompt caching also affect cost. Full-input mode records token
usage as well, allowing comparison before enabling selection. Run the local
measurement with `cd core && go test ./internal/extraction -run TestRecordedRetrievalSize -v`
when saved inputs exist under `outputs/online-extraction-smoke/`.

For a live comparison using the configured API key, run:

```sh
python3 scripts/smoke-online-extraction.py --compare
```

This makes **two paid requests**, one per mode, with no automatic retries. Both
use the same pinned public Meronem source and configured model. The runner saves
the exact transmitted source, responses, usage, seven source regression checks
and `comparison.json` under `outputs/extraction-comparison/`. It creates no rules.
Without a local embedding model the second run tests lexical fallback; only a
recorded `mode: hybrid` establishes that the live semantic channel ran. Compare
input tokens separately from variable output tokens, cached tokens and latency.

The live comparison on 2026-11-17 used the same `gpt-5.4-mini-2026-03-17` model
and source in both modes. Full input passed all seven source checks with 9,269
input tokens; lexical retrieval used 8,471 input tokens (8.61% fewer) but passed
only five checks. It omitted the hemodialysis-timing and peritoneal-dialysis
absence-of-recommendation findings, although both evidence blocks were transmitted.
This is a failed completeness result, not evidence that filtering removed those
passages. Keep full-input mode as the default. One run per mode cannot establish
a failure rate or clinical recall. No local embedding model was configured, so
the semantic channel remains tested with fixture vectors only.

### Review workflow

1. Sign in as a pharmacist or administrator. Upload the official ChPL with its
   exact product, formulation, route, indication and physical page scope.
2. Open **Documents → Extract rule suggestions**, or **Rule review → Online
   extraction**. Select an ACTIVE source and confirm sending its public content.
3. Start extraction. A durable job moves from QUEUED to RUNNING, then COMPLETED
   or FAILED. The screen refreshes while work is pending. The 50 latest jobs are
   listed with their source and status.
4. Compare each suggested fact and quotation with the original passages and full
   PDF. Model summaries are interpretations; quote presence does not establish
   clinical correctness. Reject the suggestions if they are unsuitable.
5. Choose **Review and prepare pending rule**. Confirm the renal method, action,
   numeric boundaries and boundary inclusion explicitly. The source quote and
   supported dosing phrase are prefilled where available. Correct the candidate
   against the full ChPL and save it as PENDING.
6. Review the saved candidate through the existing pharmacist approval workflow.
   Administrators can prepare candidates but cannot approve them. The model has
   no approval capability.

Unresolved conditions and condition/population/route/indication, loading dose,
maintenance, dialysis, daily maximum, monitoring and dependency facts are retained
as unmodeled qualifiers during conversion. They prevent approval until a human
has reviewed and explicitly resolved them through pending-candidate editing. Do
not remove a qualifier merely to make approval succeed. Some findings cannot be
represented by the current executable rule schema and must remain non-executable.

## Data and failure handling

- In default full-input mode, only the selected product context and complete stored
  sections 4.1, 4.2, 4.3, 4.4 and 5.2 are sent. Optional retrieval selects blocks as
  described above. No patient record, assessment or app credential is included
  in the prompt. Document scope/applicability still depends on the upload declaration.
- Section 4.2 must be present. Input is limited to 120,000 serialized UTF-8 bytes;
  oversized documents are rejected rather than silently truncated. This is a byte
  budget, not a claim about tokenizer counts. The provider also receives
  `truncation: disabled` and a 10,000-output-token limit.
- Strict JSON output uses fixed slots for one renal band, one dose and one interval.
  Numeric values and operators have separate types; mixed printed bands are rejected.
  Programmatic evidence IDs resolve exact original blocks and byte offsets, so the
  model does not reproduce quotations or qualifier text. Repeated text is resolved
  by its exact occurrence. Metric/method capitalization can be restored from one
  unambiguous source phrase, with the original model spelling recorded; units and
  other values stay case-sensitive. Unknown fields, missing evidence, unsupported source phrases,
  malformed responses, refusals and incomplete generations fail the job. Missing
  renal instructions can produce an explicit abstention.
- A job records requested and returned model IDs, response ID, input/source digest,
  PDF hash, prompt/schema hash, timestamps and validated output. Pinned chunks
  retain physical-page and byte provenance. Provider error bodies are discarded;
  API keys are never saved in job/audit payloads or logged by the adapter.
- The provider request uses `store: false` and `reasoning.effort: low`; this is an API setting, not a claim
  that the provider retains no operational data under every account policy.
- The queue allows ten active jobs and runs one provider request at a time.
  Requests time out within 95 seconds. There are no automatic paid retries.
  RUNNING jobs left by a stopped worker fail after three minutes; retry explicitly
  with a new job if appropriate. Changing the configured model/prompt invalidates
  an old queued job instead of silently using a different configuration.
- Cancelling prevents a result from being used. An already submitted provider
  request may continue and incur charges. Rejecting completed suggestions retains
  history and prevents subsequent candidate saves from that job.
- Creating jobs is idempotent per actor/request key. Saving a finding is idempotent
  per job/finding index and transactional with candidate provenance and audit.
  Changed permissions or superseded sources prevent applicable writes/results.

## Verification and remaining work

The adapter has local mock-HTTP tests for the Responses contract, malformed and
incomplete outputs, refusals, provider errors, quotation grounding and decimal
fragments. Database tests cover the job lifecycle, source invalidation, cancellation,
interrupted workers, stale digests, duplicate requests/saves, pending-only conversion
and preservation of unresolved conditions. No real clinical rule is approved by
these tests.

Run:

```sh
npm run check:types --prefix web
npm run build --prefix web
python3 scripts/test-integration.py
```

The implementation is a text-document extraction increment. Scanned PDFs/OCR,
vision/table reconstruction, independent clinical gold labels and measured real
GPT-5.4 Mini extraction accuracy remain separate work. Test a configured real API
account on public ChPLs before relying on any model-assisted clinical review.

Software verification on 2026-11-09: generated client types match the OpenAPI contract,
TypeScript/production build passes, and the full Go suite passes with race
detection against a disposable PostgreSQL/pgvector database. Adapter tests use
local mock HTTP responses. Interactive browser
verification was unavailable because the browser access host was not running.

Official API references: [GPT-5.4 Mini](https://developers.openai.com/api/docs/models/gpt-5.4-mini)
and [Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=responses).

Development check on 2026-11-10: a real `gpt-5.4-mini` Meronem extraction passed
strict validation after schema/evidence improvements. A separate check against
the pinned source confirmed the three printed adult bands, relative doses,
12/12/24-hour intervals, baseline dependencies, dialysis context and absence of
an assumed renal estimation method. Five findings were returned; no rule was
created or approved. This is one development case, not an independently reviewed
clinical benchmark. Earlier failed attempts remain saved for error analysis.

Run the opt-in live check (one paid public-document request):

```sh
python3 scripts/smoke-online-extraction.py
```

It reads the local ignored `.env` without printing credentials, extracts the pinned
Meronem PDF programmatically, and saves source/input/output/review artifacts under
`outputs/online-extraction-smoke/`. Normal test runs skip all paid calls. Clinical
review is still required; malformed or unsupported results are never accepted just
to make this smoke test pass.
