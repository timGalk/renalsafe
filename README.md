# RenalSafe

Local academic CDSS for renal-dose adjustment with deterministic decisions, source evidence and human-reviewed rules. **Synthetic patients only.**

## Start the application

Prerequisites: Docker with Compose and Python 3. The bootstrap creates private local role keys; it refuses to overwrite existing credentials.

```sh
python3 scripts/bootstrap-local.py
docker compose up -d --build
```

Open **http://127.0.0.1:8080**. Open `.local/tokens.json` locally and copy the `physician` key into the login field. Choose “Load synthetic example”, select a product and calculate. Use the `pharmacist` key for document/rule review and `admin` for administration. The browser keeps the key only in memory, never localStorage, cookies or URLs.

The seeded real-source candidate remains **PENDING**: the example returns manual review without an automated action or dose. Do not approve clinical records merely to make the demonstration return a recommendation. Approved fixtures exist only in isolated tests.

```sh
docker compose stop
# Resume later, preserving the database:
docker compose up -d
```

`.env` holds the local database configuration, `.local/users.json` holds credential hashes, and `.local/tokens.json` holds private keys. They are ignored by Git. The hashes file is readable by the unprivileged application container; plaintext keys remain private. Back up the Docker volume independently; stopping containers does not erase history. This local deployment uses a database-owner migration role and local bearer keys, not a production identity system.

## Implemented workflow

- **Assessment:** multiple exact product/form/route/indication contexts, CG and CKD-EPI 2021, stable-adult scope gates, known creatinine uncertainty, deterministic cited result, optional current-dose/interval check.
- **Structured rules:** source-backed age/weight ranges and optional explicit maximum single dose/minimum interval. No dose-unit conversion or inferred regimen. Unmodeled clinical conditions block approval. “Within rule limits” does not establish overall prescribing safety.
- **Persistence:** PostgreSQL+pgvector, versioned KB snapshots, immutable source bytes/chunks/history, exact-context APPROVED range exclusion, transaction-scoped locking, assessment+audit atomic commit, audit foreign key and chain verification.
- **Review:** PENDING-only candidate creation/editing, pharmacist-only approval/rejection, expected content digest, explicit clinical-review attestation, server-bound reviewer identity and timestamp. Administrators cannot approve rules via the review API. Approved records are immutable.
- **Sources:** bounded PDF upload, exact declared product/page scope, Poppler layout-preserving extraction, physical pages/sections/byte offsets, source hashes and original PDF download. EMA combined seed is restricted to its first product. No automatic source-URL download or OCR.
- **Evidence:** exact context + ACTIVE document filtering, full-text baseline, optional model-specific dense retrieval, RRF fusion, explicit fallback labels, provenance and evidence-only template responses. Source passages never change an assessment or approve a rule.
- **Browser:** assessment, evidence search, document upload, pending candidate preparation and pharmacist review, with mobile layout and no patient identifiers.
- **Contract:** OpenAPI 3.1 at `/openapi.json`; checked-in `api/openapi.json` generated from JSON payload types.

## Run the two-drug demonstration

```sh
docker compose up -d core
python3 scripts/demo-two-drugs.py
```

This imports the two pinned Polish ChPLs using local role credentials, then checks six synthetic patient cases, evidence retrieval, durable audit and historical replay. It creates no clinical rules or approvals. Repeated runs reuse the source imports and append new synthetic assessments. Results go to ignored `outputs/two-drug-demo/`.

Open http://127.0.0.1:8080/?example=two-drugs and sign in with the physician key. The **Load two-drug example** button fills Meronem 1 g and Ceftazidime Kabi 1000 mg. Choose **Calculate and save assessment**. For the baseline, CrCl is 25 mL/min and both products require manual review because no applicable approved rules exist. This demonstration does not establish that combining the medicines is appropriate. See `docs/two-drug-demo.md` for checks and limits.

## Local embeddings (optional)

No model is automatically downloaded or started. Configure a local Ollama server with a pinned model identifier producing **1024-dimensional** nonzero vectors, for example an independently installed BGE-M3 model. Set `RENALSAFE_OLLAMA_URL` and `RENALSAFE_EMBEDDING_MODEL`, then recreate the core container. For a host model server from Docker use the supported local endpoint `http://host.docker.internal:11434`.

Index an active document from the Documents screen. Only renal sections 4.2, 4.3, 4.4 and 5.2 are embedded. Requests use `/api/embed` with `truncate:false`; oversized sections fail explicitly instead of silently dropping context. Model/dimension mismatches and outages cannot produce a fabricated semantic result. Search falls back to full text with a reason when no usable local vectors exist. Use immutable model versions and reindex when weights change; a model name alone is not a cryptographic weights attestation.

The hybrid path was tested with known fixture vectors and a simulated provider. **No real BGE-M3 quality benchmark or generative LLM was run.** Answers display extracted passages and deterministic labels; free-form generative explanations and AI rule extraction remain separate work.

## Test and regenerate contract

Prerequisites for host tests: Go 1.25+, `pdftotext` (Poppler), Docker for integration tests.

```sh
cd core
go vet ./...
go test -race ./...
go run ./cmd/renalsafe-openapi > ../api/openapi.json
cd ..
python3 scripts/test-integration.py
```

Without `RENALSAFE_TEST_DATABASE_URL`, real-database cases skip explicitly. The integration runner provisions a uniquely named disposable PostgreSQL+pgvector container, runs all tests with race detection, then removes that container. It never uses the application's database. Hosted CI runs the same integration runner.

Optional browser smoke: install Playwright in your test environment, or set `NODE_PATH` to an existing installation; set `RENALSAFE_CHROMIUM_PATH` if needed, then run `node scripts/smoke-browser.cjs` against the running application. It reads local keys without printing them, checks the physician/reviewer workflow, and never approves a real clinical rule. Screenshots and summary go to ignored `outputs/`.

## Offline CLI

```sh
cd core
go run ./cmd/renalsafe -kb ../data/knowledge.json -audit ./example.audit.jsonl < ../data/example-request.json
go run ./cmd/renalsafe -verify-audit ./example.audit.jsonl
```

The pure decision/calculator/CLI path still needs no database, network or LLM at runtime. The server provides persistent API/browser workflows separately.

## Scope and limitations

No clinically approved dosing rule exists yet. Offline bundle reviewer strings remain trusted metadata; the server binds review identity and role to provisioned credentials. A human pharmacist must check the complete SmPC and rule semantics; byte/quote/numeric validation alone does not establish clinical validity. The decision only addresses renal criteria; it cannot establish overall prescribing safety.

The prototype policy (version `prototype-adult-stable-v1`) gates adults 18–120, creatinine 0.1–30 mg/dL, supplied weight 20–300 kg and height 100–250 cm, a sample at most seven days old, explicit stable renal function, no suspected AKI/dialysis, IDMS standardization and typical body composition. Supplied BMI below 18.5 or above 30 blocks advice. These are **engineering scope gates**, not clinical thresholds or validated laboratory freshness guidance. Missing height/weight/actual-weight confirmation prevents CG, while valid eGFR remains available. No weight imputation, creatinine floor/rounding, cap, BSA de-indexing or automatic metric substitution.

Creatinine uncertainty bounds, if supplied, use the same unit as creatinine and must be a finite closed interval containing the measured value. The derived renal interval must fit a single rule fully. Without bounds, measurement uncertainty is unknown, not zero: only the point value is matched. The calculator has ordinary estimation error and results at clinical cutoffs require independent clinical review.

Journal locking is fail closed: competing writers or a leftover lock after a crash stop output. Inspect the journal before manual lock recovery. Hash chains detect corruption and ordinary edits; complete rewriting or truncating whole events needs external anchoring to detect. The API adds transactional persistence and role-bound review; cryptographic human signatures, external anchoring, retention and disaster recovery remain future work.


## Current status and next steps

GSD Phase 1 software core and Phase 3 persisted API are verified. Phase 2 clinical validation remains open. Phase 4 has the manual ingestion/review slice; AI-assisted extraction and harder/OCR PDFs remain open. Phase 5 has lexical/hybrid retrieval infrastructure; independent clinical/retrieval gold sets and generative evaluation remain open. `docs/system-verification.md` records concrete executed checks and limitations.

Next clinical work: current Polish ChPL applicability, five independently reviewed product-specific rule sets and gold cases. Next AI work: real pinned local embeddings and benchmark against full text, table-aware splitting for large sections, pending-only extraction worker, then guarded generative explanations. No clinician identity or clinical validation result was invented.
