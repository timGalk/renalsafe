"""Rebuild a portable notebook from allowlisted sources, never from repo/private state."""
import json
from pathlib import Path
import textwrap

ROOT = Path(__file__).resolve().parent
FILES = ('benchmark.py', 'gpu_run.py', 'test_benchmark.py', 'requirements.txt',
         'synthetic_cases.json', 'public_pilot.json', 'README.md')


def build():
    cells = []

    def cell(kind, id, source):
        value = dict(cell_type=kind, id=id, metadata={}, source=textwrap.dedent(source).strip() + '\n')
        if kind == 'code':
            value.update(execution_count=None, outputs=[])
        cells.append(value)

    cell('markdown', 'intro', '''
    # RenalSafe: Qwen vs MedGemma extraction experiment

    Import this file into Kaggle Notebooks (or Google Colab). No repository access,
    app deployment or database is required. All benchmark code and inputs are embedded.

    **First run:** choose a GPU accelerator, enable Internet, accept access terms for
    [MedGemma](https://huggingface.co/google/medgemma-1.5-4b-it) using your Hugging Face
    account, and add a notebook secret named `HF_TOKEN` with read access to that model.
    Authorize this notebook to read the secret. Do not paste tokens into cells.

    The default eight examples are **fictional software fixtures**, not clinical evidence.
    Two bundled public Polish document extracts have **no reviewed labels** and produce
    outputs/timings only. Independent labels are needed for a clinical comparison.

    This notebook has been checked offline; actual GPU execution and model compatibility
    remain to be verified on your assigned runtime. Runs do not create application rules.
    ''')
    cell('code', 'configuration', '''
    from pathlib import Path
    import sys

    assert sys.version_info >= (3, 11), 'This package requires Python 3.11 or later.'
    BASE = Path('/kaggle/working') if Path('/kaggle/working').exists() else Path('/content') if Path('/content').exists() else Path.cwd()
    WORK = BASE / 'renalsafe-benchmark'
    WORK.mkdir(parents=True, exist_ok=True)

    RUN_MODELS = True  # False runs only offline software checks; no GPU/download required.
    MODELS_TO_RUN = ['qwen', 'medgemma']  # Run separately if your GPU time is limited.
    DATASET = 'synthetic_cases.json'  # Or 'public_pilot.json', or an attached dataset's absolute path.
    SPLIT = 'development'  # Use 'test' only with a frozen, independently reviewed held-out dataset.
    QUANTIZATION = 'nf4'  # Same precision policy for both; 'none' requires more memory.
    MAX_INPUT_TOKENS = 4096  # Increase deliberately for long public source bundles.
    MAX_NEW_TOKENS = 2048
    RUN_NAME = 'synthetic-v1'  # Use a NEW name after changing data, prompt, settings or runtime.
    # Optional immutable model commit SHAs. None resolves once and saves model_revisions.json.
    REVISIONS = {'qwen': None, 'medgemma': None}
    OUTPUT = WORK / 'runs' / RUN_NAME
    OUTPUT.mkdir(parents=True, exist_ok=True)
    ''')
    payload = {name: (ROOT / name).read_text(encoding='utf-8') for name in FILES}
    cell('code', 'bundled-files', 'import json\nfrom pathlib import Path\n'
         + 'BUNDLED = json.loads(' + repr(json.dumps(payload, ensure_ascii=False)) + ')\n'
         + 'for name, content in BUNDLED.items():\n    (WORK / name).write_text(content, encoding="utf-8")\n'
         + 'print("Benchmark code and datasets prepared:", WORK)')
    cell('markdown', 'offline-explanation', '''
    ## Check the benchmark software
    These tests use deliberately fake answers to check scoring, leakage protection,
    malformed output and checkpoint handling. They are not model results.
    ''')
    cell('code', 'offline-tests', '''
    import subprocess
    # The notebook packaging test is checked in the repository; other tests work anywhere.
    subprocess.run([sys.executable, '-c',
        "import unittest, test_benchmark; suite=unittest.defaultTestLoader.loadTestsFromTestCase(test_benchmark.BenchmarkTests); "
        "suite=unittest.TestSuite(t for t in suite if t._testMethodName != 'test_notebook_is_self_contained_and_unexecuted'); "
        "result=unittest.TextTestRunner(verbosity=1).run(suite); raise SystemExit(not result.wasSuccessful())"],
        cwd=WORK, check=True)
    ''')
    cell('code', 'dependencies', '''
    if RUN_MODELS:
        import torch
        assert torch.cuda.is_available(), 'Select a GPU accelerator in notebook settings, then restart this session.'
        print('GPU:', torch.cuda.get_device_name(0))
        subprocess.run([sys.executable, '-m', 'pip', 'install', '--quiet', '-r', str(WORK / 'requirements.txt')], check=True)
        # Model code runs in fresh processes, so new packages do not require a kernel import reload.
    ''')
    cell('markdown', 'data-explanation', '''
    ## Select and check inputs
    Leave the synthetic dataset selected for the first run. To use real sources, select
    `public_pilot.json` above or attach your own JSON dataset and use its absolute path.
    Public source bundles can exceed the default input budget: increase it only if memory
    permits. Input is never silently cut. Keep table headings, footnotes and cross-references.

    The runner never includes expected answers or reviewer metadata in model prompts.
    For a thesis evaluation, follow the label/split protocol in the bundled README.
    ''')
    cell('code', 'dataset-check', '''
    sys.path.insert(0, str(WORK))
    import benchmark as bench
    dataset_path = Path(DATASET) if Path(DATASET).is_absolute() else WORK / DATASET
    dataset = bench.validate_dataset(json.loads(dataset_path.read_text()))
    selected = [c for c in dataset['cases'] if c['split'] == SPLIT]
    assert selected, 'No cases in selected split.'
    print('Purpose:', dataset['purpose'], '| selected cases:', len(selected), '| labelled:', sum(c.get('expected') is not None for c in selected))
    print('Dataset SHA256:', bench.digest(dataset))
    ''')
    cell('code', 'access-and-revisions', '''
    import os
    import re
    if RUN_MODELS:
        hf_token = os.environ.get('HF_TOKEN')
        if not hf_token:
            try:
                from kaggle_secrets import UserSecretsClient
                hf_token = UserSecretsClient().get_secret('HF_TOKEN')
            except Exception:
                try:
                    from google.colab import userdata
                    hf_token = userdata.get('HF_TOKEN')
                except Exception:
                    pass
        if 'medgemma' in MODELS_TO_RUN and not hf_token:
            raise RuntimeError('Add/authorize the HF_TOKEN notebook secret and accept MedGemma access terms first.')
        from huggingface_hub import HfApi
        revision_file = OUTPUT / 'model_revisions.json'
        resolved = json.loads(revision_file.read_text()) if revision_file.exists() else {}
        for name in MODELS_TO_RUN:
            assert name in bench.MODELS, 'Unknown model'
            requested = REVISIONS[name]
            if name in resolved and requested and resolved[name] != requested:
                raise RuntimeError('Revision changed: choose a new RUN_NAME.')
            if name not in resolved:
                try:
                    resolved[name] = HfApi().model_info(bench.MODELS[name], revision=requested or 'main', token=hf_token).sha
                except Exception as exc:
                    raise RuntimeError(f'Model access failed for {name}: {type(exc).__name__}. Check Internet, secret permissions and accepted terms.') from None
                assert re.fullmatch(r'[a-f0-9]{40}', resolved[name]), 'No immutable model revision returned'
                bench.atomic_json(revision_file, resolved)
        print('Pinned revisions:', resolved)  # Revisions are public identifiers; never print hf_token.
    ''')
    cell('markdown', 'run-explanation', '''
    ## Run both models sequentially
    Both use the same text, prompt, schema, greedy decoding and precision policy.
    Qwen thinking is disabled. This controlled baseline is not either vendor's optimized
    sampling benchmark. Model-specific chat templates and architectures still differ.

    Each model runs in a fresh process on GPU 0; the next starts after its memory is
    released. Checkpoints are saved after every case. Rerunning skips completed cases,
    including failed cases. A changed run identity is refused rather than mixed.
    Download partial outputs before ending a session. New sessions need saved outputs
    copied back into the same run directory to resume; changed environments require a new run.
    ''')
    cell('code', 'gpu-comparison', '''
    run_status = {}
    if RUN_MODELS:
        child_env = os.environ.copy()
        if hf_token:
            child_env['HF_TOKEN'] = hf_token
        child_env['HF_HUB_DISABLE_TELEMETRY'] = '1'
        child_env['HF_HUB_DISABLE_PROGRESS_BARS'] = '1'
        child_env['TOKENIZERS_PARALLELISM'] = 'false'
        for name in MODELS_TO_RUN:
            print('Running:', name, flush=True)
            command = [sys.executable, str(WORK / 'gpu_run.py'), '--model', name,
                '--revision', resolved[name], '--dataset', str(dataset_path),
                '--output', str(OUTPUT / name), '--split', SPLIT,
                '--quantization', QUANTIZATION, '--max-input-tokens', str(MAX_INPUT_TOKENS),
                '--max-new-tokens', str(MAX_NEW_TOKENS)]
            completed = subprocess.run(command, env=child_env, cwd=WORK)
            run_status[name] = 'completed' if completed.returncode == 0 else 'failed'
            bench.atomic_json(OUTPUT / 'run_status.json', run_status)
        child_env.pop('HF_TOKEN', None)
    else:
        print('Offline checks only. Set RUN_MODELS=True to generate actual model results.')
    ''')
    cell('code', 'comparison-and-export', '''
    import csv
    import shutil
    rows = []
    for name in MODELS_TO_RUN:
        summary_file = OUTPUT / name / 'summary.json'
        if not summary_file.exists() or run_status.get(name) != 'completed':
            rows.append({'model': name, 'run_status': run_status.get(name, 'not_run')})
            continue
        report = json.loads(summary_file.read_text())
        rows.append({'model': name, 'run_status': run_status[name], **{k: report[k] for k in
            ['purpose', 'cases', 'labelled_cases', 'schema_valid_rate', 'whole_case_exact_rate',
             'citation_substring_rate', 'median_generation_seconds', 'peak_allocated_bytes']}})
    columns = list(dict.fromkeys(k for row in rows for k in row))
    with (OUTPUT / 'comparison.csv').open('w', newline='') as handle:
        writer = csv.DictWriter(handle, fieldnames=columns)
        writer.writeheader()
        writer.writerows(rows)
    try:
        import pandas as pd
        display(pd.DataFrame(rows))
    except ImportError:
        print(json.dumps(rows, indent=2))
    archive = shutil.make_archive(str(BASE / ('renalsafe-' + RUN_NAME)), 'zip', OUTPUT)
    from IPython.display import FileLink, display
    display(FileLink(archive))
    print('Saved results:', archive, '\\nAlso available through the notebook output-files panel.')
    ''')
    cell('markdown', 'interpretation', '''
    ## Interpret results
    - Download `comparison.csv`, each model's `summary.json`, `cases.csv`, raw
      `predictions.jsonl`, `manifest.json`, and the input/prompt snapshots in the ZIP.
    - Schema validity and quotation substring matches do not prove correct interpretation.
    - Field/group precision and recall are exact multisets across findings in each case.
      They can miss row-association errors; also inspect whole-finding/case matches.
    - False positives are **label mismatches**, not automatically clinically unsupported
      claims. A reviewer must adjudicate extra facts, synonyms, ambiguity and severity.
    - Failures count against case accuracy. Unlabelled sources have no accuracy score.
    - Timing includes generation (first invocation may be slower), excludes loading and
      tokenization; peak memory is PyTorch GPU allocation, not total device consumption.
      Compare only matching hardware/settings. Load time is recorded separately per case.
    - Greedy decoding/seeds do not ensure bit-for-bit reproducibility across runtimes.
    - No model winner or clinical validation can be inferred from the synthetic fixtures.

    References: [Qwen model card](https://huggingface.co/Qwen/Qwen3.5-4B),
    [Qwen Transformers API](https://huggingface.co/docs/transformers/en/model_doc/qwen3_5),
    [Google MedGemma documentation](https://developers.google.com/health-ai-developer-foundations/medgemma/model-card),
    [NF4 quantization](https://huggingface.co/docs/transformers/en/quantization/bitsandbytes).
    ''')
    notebook = dict(nbformat=4, nbformat_minor=5, metadata={
        'kernelspec': {'display_name': 'Python 3', 'language': 'python', 'name': 'python3'},
        'language_info': {'name': 'python', 'version': '3.11.0'}}, cells=cells)
    path = ROOT / 'renalsafe_model_comparison.ipynb'
    path.write_text(json.dumps(notebook, indent=1, ensure_ascii=False) + '\n', encoding='utf-8')
    print(path)


if __name__ == '__main__':
    build()
