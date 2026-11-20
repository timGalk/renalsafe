# Kaggle / Colab model comparison

Upload **`renalsafe_model_comparison.ipynb`** to Kaggle Notebooks or Colab. It embeds all
required code and datasets; no GitHub permission, RenalSafe deployment, database, or
local service is needed. The Python sources alongside it are the editable originals.

> **Superseded.** This first harness is kept for reference. Use
> [model-comparison-v2](../model-comparison-v2/README.md) for current paired experiments.

Status: implemented experiment harness with offline regression tests. The Qwen and
MedGemma notebook runs have since been completed outside the repository; their
extracted outputs are not tracked and are not reported here as validated results.
Dependency pins are available registry versions checked 2026-11-05, not a claim of
successful GPU compatibility. There are no measured model
scores, independent clinical labels, or approved rules in this package.

## First run

1. Import the `.ipynb` file into a new notebook. Select a CUDA GPU accelerator and
   enable Internet for package/model downloads. The runtime needs Python 3.11+.
2. On Hugging Face, request/accept access to
   [MedGemma 1.5 4B](https://huggingface.co/google/medgemma-1.5-4b-it).
   Add a read token with access to that model as a Kaggle/Colab secret named `HF_TOKEN`;
   authorize the notebook to read it. Never put its value in a cell or uploaded dataset.
3. Run the notebook with the default synthetic dataset. It first tests the scorer,
   installs pinned inference packages, resolves immutable model revisions, then runs
   Qwen and MedGemma sequentially. A model must fit on GPU 0; there is no CPU offload
   or multi-GPU sharding. Availability, memory and session limits depend on your account.
4. Download the result ZIP through the link or notebook output-files panel. On Kaggle,
   results are under `/kaggle/working/renalsafe-benchmark/runs/`.
5. Inspect raw responses and failures before interpreting summary scores. For real source
   exploration change `DATASET` to `public_pilot.json` and use a new `RUN_NAME`.

With `RUN_MODELS=False`, the notebook checks its software without GPU access or model
downloads. Those tests deliberately use fake responses and are not benchmark results.

## What is compared

| Item | Configuration |
|---|---|
| Models | `Qwen/Qwen3.5-4B`, `google/medgemma-1.5-4b-it` |
| Task | Extract structured renal dosing instructions from supplied text |
| Input | Identical source bundle and prompt; model-specific chat templates |
| Generation | Greedy, one beam, seed 42, Qwen thinking disabled |
| Precision | NF4 double quantization, same GPU-dependent FP16/BF16 compute policy |
| Serving | Transformers in the notebook GPU session; no hosted inference API |
| Validation | Strict JSON/types/keys, evidence per populated scalar, exact quote checks |
| Outputs | Raw responses, individual checkpoints, JSON/CSV summaries and run manifests |

NF4 is a memory-saving experimental profile, not equivalent to full precision. Set
`QUANTIZATION='none'` for a separate experiment when memory permits. This deliberately
controlled decoding profile differs from Qwen's recommended sampled profiles; a ranking
would apply only to the tested configuration. No model selection is claimed now.

## Datasets and labels

`synthetic_cases.json`: eight fictional, engineer-authored examples covering strict
boundaries, decimal commas, missing metrics, relative doses, dialysis conditions,
ambiguous ranges, negative passages and injected instructions. Useful for software and
initial GPU smoke checks; never report these as pharmacist-reviewed clinical accuracy.

`public_pilot.json`: two **unlabelled** source bundles from the repository's pinned
Meronem and Ceftazidime Kabi Polish ChPL snapshots. Product IDs, official URLs, acquisition
dates, PDF/text hashes and physical pages are retained. Bundles include preceding context
and dosing pages; they do not guarantee that every external reference is resolved.
They have `expected: null`, so no semantic accuracy score is calculated. Review against
the original PDF before labelling. These two products are a development pilot, not a
held-out clinical benchmark. Public sources are not patient records.

To supply your own dataset, copy the fixture JSON structure, use public source passages,
set `synthetic: false`, preserve complete tables/footnotes, provide provenance, and compute
`source_sha256 = hashlib.sha256(source_text.encode('utf-8')).hexdigest()` after all page
markers/formatting are final. Attach the JSON as notebook data and set `DATASET` to its
absolute path. The file may contain only sources you may use, synthetic data, and labels;
do not attach database exports, patient histories, app secrets or private backups.

For `purpose: reviewed_pilot` or `held_out_evaluation`, each case requires an independent
reviewer's `expected` envelope and:

```json
"review": {
  "status": "independently_reviewed",
  "reviewer_id": "your-actual-pseudonymous-reviewer-id",
  "label_version": "your-actual-label-version",
  "reviewed_at": "actual-review-date"
}
```

Do not fill this with fictitious review information. Metadata validation cannot establish
reviewer qualifications or that a review actually happened. Use the model outputs only
as predictions; never copy them into reference labels and call that independent review.

The output contract is in `benchmark.py` (`SCHEMA`, `PROMPT`). Every non-null scalar needs
an evidence quote, numbers are decimal strings, units/metric/method are not inferred,
and conditions/unresolved dependencies are retained as exact source phrases. An uncertain
range can have numeric bounds with null operators. Unknown fields remain null. Relative
and weight-based instructions must not be converted into invented fixed doses.

Start with 10–20 independently labelled examples across English/Polish documents; report
the exact number of products. Freeze prompts on development products, then reserve
different products/documents for `split: test`. The validator rejects products or document
IDs present in both splits. `held_out_evaluation` requires both splits in the dataset.
Run `SPLIT='test'` only after freezing the experiment. Label corrections require a new
dataset version and run. Same-product versions, translated copies and near-duplicate
documents also need manual grouping; identifier checks cannot detect all leakage.

## Metrics and practical limits

- Whole-case exact match compares complete findings including associated conditions and
  unresolved phrases; citations must also pass substring checks. Evidence span wording
  and abstention-reason prose are not compared for exact equality.
- Per-field and grouped renal/dose/interval precision, recall and F1 use exact multisets
  within each case. Duplicates count as extras. Group metrics can miss cross-row association
  errors, so whole-finding/case match must be reported alongside them.
- Conditions/unresolved recall counts omitted reference phrases. Paraphrases are mismatches
  under this extractive protocol; independent review must adjudicate equivalent wording.
- False positives are label mismatches, **not a validated hallucination or clinical harm
  rate**. Likewise an exact quote can be attached to the wrong interpretation. Manually
  review raw outputs for unsupported facts, missed qualifiers and severity.
- JSON/schema errors, input-budget errors, truncation and runtime failures remain in
  case-level denominators. Invalid output is not a successful abstention. Undefined
  precision/recall is `null`, not a perfect score. No labels means no semantic scores.
- Latency is synchronized GPU generation time, excluding tokenization and model loading.
  First generation can be slower; there is no warm-up or repeated-trial speed claim.
  Individual timings and load time are retained. Memory measures PyTorch peak allocated
  and reserved bytes on GPU 0, not total GPU or host RAM use. Compare identical hardware
  and settings; use repeated separate runs before making efficiency claims.

## Reproducibility and interruption

The notebook resolves each model to an immutable Hugging Face commit and writes
`model_revisions.json` before loading weights. Manifests record model/processor revision,
chat-template hash, dependency versions, GPU/CUDA identity, decoding/precision settings,
source/dataset/prompt/schema/code hashes. Per-run input and prompt snapshots are exported.
Registry pins cover direct inference dependencies; the GPU image's PyTorch/CUDA and
transitive packages are captured in the manifest, not claimed to be fully locked.

Each case is atomically checkpointed. Rerunning in the same environment reuses all saved
cases, including errors; it does not silently retry failures or cherry-pick answers. A
changed identity refuses resume. Use a new `RUN_NAME` for changed settings/environment
or an explicit repeat. To resume after a session ends, download the partial run folder
first, upload it as notebook input, then copy it back under the configured writable run
directory (including `model_revisions.json` and model subdirectories). Do not run concurrent
writers against one run folder. These files are not automatically persistent across sessions.

## Troubleshooting

| Symptom | Action |
|---|---|
| No CUDA GPU | Select a GPU accelerator, restart the session, rerun |
| Gated model / access error | Accept MedGemma terms using the same account as the secret token; check read scope and notebook authorization |
| Import/load error | Check Python 3.11+, platform Torch/CUDA compatibility and pinned library versions in a fresh runtime; do not report this as a model score |
| Out of memory | Keep NF4, run only one model, reduce token budgets or select a larger available GPU; use a new run name for changed settings |
| `runtime_error` on a long input | Check token budget first; preserve whole table/context and raise the budget if memory permits, or design/review a smaller self-contained bundle |
| `truncated` | Increase output budget in a new run; retain and report the truncated attempt |
| `invalid_json` / `invalid_schema` | Inspect raw output; the benchmark deliberately does not repair it or infer missing doses |
| Resume mismatch | Start a new run name or restore the original exact configuration and environment |

Provider exceptions are exported as type/stage only to avoid recording access URLs or
tokens. Model-load failures are shown separately, without manufactured per-case scores.
Generation failures keep case records. Neither implies the other model is clinically better.

## Maintain / verify locally

```sh
python3 experiments/model-comparison/build_notebook.py
python3 -m unittest discover -s experiments/model-comparison -p 'test_*.py' -v
```

Rebuild the notebook after editing any bundled source, dataset or this README. Tests check
that embedded files match their originals and that notebook code cells compile. The
notebook is stored without execution outputs or tokens. Upload the `.ipynb` alone.

References checked 2026-11-05:
[Qwen model card](https://huggingface.co/Qwen/Qwen3.5-4B),
[Qwen Transformers class](https://huggingface.co/docs/transformers/en/model_doc/qwen3_5),
[MedGemma model card/API](https://developers.google.com/health-ai-developer-foundations/medgemma/model-card),
[NF4 documentation](https://huggingface.co/docs/transformers/en/quantization/bitsandbytes),
[Kaggle notebooks](https://www.kaggle.com/docs/notebooks),
[Colab resource limits](https://research.google.com/colaboratory/faq.html).
