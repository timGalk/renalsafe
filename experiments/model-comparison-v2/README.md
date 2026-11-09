# RenalSafe paired extraction experiment v2

Upload `renalsafe_model_comparison_v2.ipynb` to Kaggle or Colab. All runtime code,
datasets, tests and this guide are embedded. The original experiment is unchanged.

## Run sequence

1. Run all cells with the default `RUN_MODELS=False`. This executes software checks
   and creates annotation templates. The selected dataset defaults to the real ChPL
   pilot. It does not download models or invent results.
2. For a GPU smoke run, enable GPU and Internet, authorize the `HF_TOKEN` secret,
   accept MedGemma access terms, then set `RUN_MODELS=True`. Keep the ChPL pilot
   selected for unlabelled output inspection; preflight both tokenizers first.
3. Prepare independent reference answers against original PDFs. Use
   `annotation_template.json` as a starting structure, plus the labelled synthetic
   file as a schema example. Do not label from either model's predictions.

The bundled `public_pilot.json` contains **138 unlabelled excerpts from 46
Polish ChPLs** covering 46 of the 48 unique medicines requested. Each product has
three inputs: the document beginning through section 4.2 (dosing), product identity
plus complete sections 4.3–4.4 (contraindications/warnings), and product identity plus
complete section 5.2 (pharmacokinetics). These are related document excerpts, not
138 independent patient cases. All cases for a substance share `evaluation_group`
and the development split. Metamizole is represented once.

Every input is copied from an official RPL or EMA Polish ChPL PDF, without invented
doses or reference answers. The PDFs, layout-preserving text and acquisition manifest
are pinned under `data/sources/pl-2026-11-09/` in the repository. Each case records
PDF/text/input hashes, source URL, section scope, physical pages, and exact character
and UTF-8 byte offsets into the extracted text. The standalone notebook embeds the
case text and provenance; the full PDFs remain in the repository.

See [CHPL_CASES.md](CHPL_CASES.md) for coverage and rebuilding. **Wekuronium and
nifedypina remain missing:** suitable official Polish product-specific ChPLs were not
located. Aztreonam uses Cayston for inhalation; it does not cover intravenous
aztreonam. Products and formulations cannot be generalized to other routes or
indications. The user categories do not establish that every listed drug is indicated
for continuous sedation or any other particular use.

Clinical applicability, table semantics and labels have not been independently
reviewed. Safety/PK excerpts intentionally omit dosing sections and must be assessed
only against their supplied context; do not infer doses from the missing sections.
Obtain the complete PDF and follow all cross-references before annotation. Complete
sections retain tables and footnotes; some exceed the default token budget. Preflight
both tokenizers and adjust budgets on development only; never silently truncate.
No case is an approved prescribing rule or a held-out benchmark.

4. Set `purpose` to `reviewed_pilot` for development only, or `held_out_evaluation`
   for a dataset containing both development and test cases. Populate review metadata
   truthfully. Supply `evaluation_group` for every reviewed case: versions,
   translations, related products and near-duplicates belong to one group. Give all
   members the same split. Grouping is a reviewer decision, not automatic detection.
5. Set `DATASET` to your attached labelled JSON and use a new `RUN_NAME`. Develop on
   `SPLIT='development'`. Preflight every development input with both tokenizers;
   revise budgets on development only. Keep full headings and footnotes. Never crop
   test documents to improve a score.
6. Once development is finished, rerun with the final complete dataset (including
   independently labelled test products), final settings, and `FREEZE_PROTOCOL=True`.
   Save `frozen_protocol.json`. Freezing requires reviewed labels and both models.
7. Use `SPLIT='test'`, `FREEZE_PROTOCOL=False`, a new run name, and set `PROTOCOL_PATH`
   to that saved file. The notebook reuses its model revisions and rejects changes
   to the dataset, settings, prompt, scoring code or package versions. This guard
   cannot prove that a human has never inspected the test answers.
8. Download the ZIP. Give reviewers only `blind_review.csv`; retain the separate
   model identity key. Review all outputs, including failures, with model names hidden.

## Annotation rules

- One finding per distinct renal band/regimen, with its own conditions and unresolved
  dependencies. Preserve population, route, loading doses and dialysis qualifiers.
- Record only explicitly supported information; missing scalar values are null.
- Numbers are decimal strings with decimal comma changed to dot. No unit conversion,
  dose calculation, assumed metric or boundary inclusivity.
- Instructions, conditions and unresolved dependencies are source phrases. Decide
  their scope during development; retain meaningful qualifiers, not just numbers.
- Each populated scalar has a source quote supporting that value in context. A word
  merely appearing elsewhere is insufficient. Prefer the smallest complete supporting
  passage, including the table heading/row context when needed.
- No supported instruction means empty findings and abstain=true with a reason.
- Ideally two qualified reviewers annotate independently and adjudicate disagreements.
  Keep reviewer notes separately and record the final label version. A metadata field
  cannot certify that review actually took place.

## Outcomes, fixed before test

Primary: complete-regimen precision/recall/F1 (all fields and associated qualifiers),
and group-macro whole-case content accuracy. A case passes only if all complete
findings match and all supplied quotations are present. This is NOT a measurement of
evidence correctness: semantic support still needs manual review.

Normalization: NFC Unicode and collapsed whitespace for strings; terminal periods
removed only from instruction/condition/unresolved prose. Case, numbers, operators,
units and synonyms are otherwise exact. Strict unnormalized case accuracy is also
reported. Equivalent but differently scoped phrases require adjudication, not an
unreported relaxation after test.

Diagnostics: one-to-one maximum-weight row alignment; each matching non-null renal
attribute weighs 10, other scalar fields weigh 1. Field/group/qualifier counts are
computed inside assigned pairs, never pooled across rows. This alignment heuristic
can be ambiguous; it does not determine the primary complete-regimen score.

Malformed/truncated/runtime failures fail case accuracy and produce missing reference
findings. Precision alone can hide such failures; always report recall, case accuracy,
and status counts. An invalid output on a negative case fails case accuracy even
though there are no positive findings to miss. Extra/missing records count separately.

Paired comparison uses identical case sets. Report both/one/neither model successes.
The effect is Qwen minus MedGemma, averaging case differences within each evaluation
group and then weighting groups equally. A seeded 2,000-resample paired group bootstrap
provides a 95% percentile interval; fewer than two groups gives no interval. Small
datasets or uniform results can give misleadingly narrow intervals; a zero-width
interval is not proof of population certainty. Report sample sizes and all outcomes.
No automatic winner is declared. Synthetic results never establish model quality.

The blinded CSV has blank human ratings for evidence support, wrong band/dose
association, missed qualifiers, unsupported additions and notes. Blank means
unreviewed, not correct. The notebook does not fabricate a semantic evidence score.

## Efficiency and reproducibility

Both use the same sources, prompt, greedy decoding and NF4/compute-dtype policy,
but their own chat templates. Qwen thinking is disabled. Conclusions apply to this
configuration, not every possible tuning of these models.

Timing is exploratory single-pass GPU generation time. The paired report uses only
cases successfully completed by both models and includes the matched count and all
failure counts. There is no warm-up or repeated timing trial, so do not claim a speed
winner. Peak memory is PyTorch allocation, not total device usage. For a dedicated
efficiency study, use separate repeated runs with warm-up and counterbalanced order.

Per-model manifests record hardware, package versions, model revisions, source,
prompt and code hashes. Checkpoints reuse errors as well as successes; do not remove
failed cases selectively. New settings need a new run name. Save results before a
hosted session ends. Secrets are read from environment/notebook secret storage and
never embedded. Dataset files should contain public or synthetic documents only.

The GPU adapter and dependency pins are inherited from the original experiment;
actual model loading and GPU execution have not been validated locally. Use the smoke
run first. If dependencies must change, resolve that during development and refreeze.

## Maintenance

Run `python experiments/model-comparison-v2/build_chpl_cases.py` to rebuild cases from pinned sources,
then `python experiments/model-comparison-v2/build_notebook.py` to rebuild the notebook.
Run `python -m unittest discover -s experiments/model-comparison-v2 -p 'test_*.py' -v`.
The builder reads the original notebook as a structural template and embeds only
allowlisted v2 files. The generated notebook runs independently of the repository.
