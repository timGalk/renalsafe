# Development and verification

For the Docker installation and first sign-in, start with the [repository README](../README.md#installation). Run the commands below from the repository root unless stated otherwise. Host prerequisites are Go 1.25+, Node 22.12+ with npm, Python 3, Poppler (`pdftotext`), and Docker with Compose for database checks.

## Test and regenerate contract

Prerequisites for host tests: Go 1.25+, Node 22.12+ with npm, `pdftotext` (Poppler), Docker for integration tests. Build the embedded React assets before testing all Go packages. Docker builds them automatically.

```sh
npm ci --prefix web
npm run check:types --prefix web
npm run build --prefix web
cd core
go vet ./...
go test -race ./...
cd ..
python3 scripts/test-integration.py
```

Without `RENALSAFE_TEST_DATABASE_URL`, real-database cases skip explicitly. The integration runner provisions a uniquely named disposable PostgreSQL+pgvector container, runs all tests with race detection, then removes that container. It never uses the application's database. Hosted CI runs the same integration runner.

Optional browser smoke: install Playwright in the test environment, or set `PLAYWRIGHT_MODULE` to an existing installation and `RENALSAFE_CHROMIUM_PATH` if needed, then run `node scripts/smoke-workspace.cjs`. It provisions disposable staff using private local setup credentials, uses password sessions in the browser and prints no credentials. It checks clinician, patient/results, source review and administrator workflows. Fixture clinical candidates are rejected; no original real rule is approved. Reports/screenshots go to ignored `outputs/`.

After changing API payloads or routes, regenerate both the contract and client:

```sh
cd core
go run ./cmd/renalsafe-openapi > ../api/openapi.json
cd ..
npm run types --prefix web
npm run check:types --prefix web
npm run build --prefix web
```

Frontend development and the Vite proxy are documented in [web/README.md](../web/README.md). Operational checks are listed in [scripts/README.md](../scripts/README.md).

## Local embeddings (optional)

No model is automatically downloaded or started. Configure a local Ollama server with a pinned model identifier producing **1024-dimensional** nonzero vectors, for example an independently installed BGE-M3 model. Set `RENALSAFE_OLLAMA_URL` and `RENALSAFE_EMBEDDING_MODEL`, then recreate the core container. For a host model server from Docker use the supported local endpoint `http://host.docker.internal:11434`.

Index an active document from the Documents screen. Only renal sections 4.2, 4.3, 4.4 and 5.2 are embedded. Requests use `/api/embed` with `truncate:false`; oversized sections fail explicitly instead of silently dropping context. Model/dimension mismatches and outages cannot produce a fabricated semantic result. Search falls back to full text with a reason when no usable local vectors exist. Use immutable model versions and reindex when weights change; a model name alone is not a cryptographic weights attestation.

The hybrid path was tested with known fixture vectors and a simulated provider. **No real BGE-M3 quality benchmark or generative LLM was run for that retrieval benchmark.** Answers display extracted passages and deterministic labels; free-form generative explanations remain separate work. Online ChPL extraction into reviewed PENDING candidates is available through OpenAI or NVIDIA; see [setup and limits](online-extraction-guide.md).

## Continue ingestion and evaluation

New source imports use lossless passages with a soft 6000-byte target. Detected table blocks, introductions and possible footnotes remain together; oversized blocks stay whole. Existing imported chunks/citations remain immutable. This is layout-aware engineering, not verified table semantics or OCR.

Pending rules can now be edited in the browser with fixed source context and stale-write protection. Corrections remain PENDING and still require the separate pharmacist review.

Run the source retrieval baseline after importing the two Polish sources:

```sh
python3 scripts/evaluate-retrieval.py data/retrieval-software-fixtures.json \
  --modes lexical --output outputs/evaluation/lexical-software.json
python3 scripts/test-evaluation.py
```

The six software cases do not replace independent gold labels. The runner supports a pinned-model lexical/hybrid comparison, reports degradation and knowledge drift, and records source recall, citation precision, abstention and latency. See [evaluation guide](evaluation-guide.md) for labeling, model configuration and reporting limits.

## Offline CLI

```sh
cd core
go run ./cmd/renalsafe -kb ../data/knowledge.json -audit ./example.audit.jsonl < ../data/example-request.json
go run ./cmd/renalsafe -verify-audit ./example.audit.jsonl
```

The pure decision/calculator/CLI path still needs no database, network or LLM at runtime. The server provides persistent API/browser workflows separately.
