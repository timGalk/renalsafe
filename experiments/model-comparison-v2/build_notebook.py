"""Build an independent v2 notebook without altering the original experiment."""
import json
from pathlib import Path
import textwrap

ROOT = Path(__file__).resolve().parent
FILES = ('benchmark.py', 'evaluation.py', 'gpu_run.py', 'requirements.txt',
         'synthetic_cases.json', 'public_pilot.json', 'README.md', 'CHPL_CASES.md', 'test_evaluation.py')


def build():
    notebook = json.loads((ROOT.parent/'model-comparison/renalsafe_model_comparison.ipynb').read_text())
    by_id = {c['id']: c for c in notebook['cells']}

    def replace(id, source):
        by_id[id]['source'] = textwrap.dedent(source).strip()+'\n'

    def after(id, new_id, kind, source):
        cell = dict(cell_type=kind, id=new_id, metadata={}, source=textwrap.dedent(source).strip()+'\n')
        if kind == 'code':
            cell.update(outputs=[], execution_count=None)
        notebook['cells'].insert(notebook['cells'].index(by_id[id])+1, cell)
        by_id[new_id] = cell

    replace('intro', '''
    # RenalSafe v2: paired Qwen–MedGemma extraction experiment

    **Start with Run all in offline mode.** This checks the software and exports an
    annotation template. No model results or independent reference labels are invented.

    Workflow: software check → independently label real documents → develop prompt →
    freeze protocol → evaluate held-out products → blinded error review.

    For inference: select a CUDA GPU, enable Internet, accept MedGemma terms on Hugging
    Face, and authorize a notebook secret named `HF_TOKEN`. Never paste the token here.
    Upload this notebook alone to Kaggle/Colab. Read the bundled `README.md` for annotation
    rules, metric definitions and limitations. GPU compatibility still needs a smoke run.
    ''')
    config = by_id['configuration']['source']
    config = config.replace("'renalsafe-benchmark'", "'renalsafe-benchmark-v2'")
    config = config.replace('RUN_MODELS = True', 'RUN_MODELS = False')
    config = config.replace("RUN_NAME = 'synthetic-v1'", "RUN_NAME = 'smoke-v2'")
    config = config.replace("DATASET = 'synthetic_cases.json'", "DATASET = 'public_pilot.json'")
    config = config.replace("# Or 'public_pilot.json', or an attached dataset's absolute path.",
                            "# Real ChPL excerpts; or an attached independently labelled dataset.")
    config += '''
# Freeze only after independent labels and development are complete.
FREEZE_PROTOCOL = False
PROTOCOL_PATH = WORK / 'frozen_protocol.json'  # Or path to an attached frozen file.
'''
    replace('configuration', config)
    payload = {name: (ROOT/name).read_text() for name in FILES}
    replace('bundled-files', 'import json\nBUNDLED = json.loads('+repr(json.dumps(payload, ensure_ascii=False))+')\n'
            'for name, content in BUNDLED.items():\n    (WORK/name).write_text(content, encoding="utf-8")\n'
            'print("Prepared standalone experiment:", WORK)')
    replace('offline-tests', '''
    import subprocess
    subprocess.run([sys.executable, '-m', 'unittest', 'test_evaluation', '-v'], cwd=WORK, check=True)
    ''')
    replace('data-explanation', '''
    ## Prepare and select reference data

    Default: `public_pilot.json`, 138 unlabelled excerpts from 46 official Polish
    ChPLs. Each product has dosing, safety and pharmacokinetic inputs grouped by
    substance. Exact source slices and PDF/text hashes are recorded. Full snapshots
    are retained in the repository. These inputs do not support a model ranking.
    Wekuronium and nifedypina lack suitable official Polish sources; aztreonam is
    inhaled Cayston only. Verify complete PDFs, tables and applicability before labelling.
    Fictional examples remain only as software regression fixtures.

    The next cell exports an annotation template without model-generated answers.
    Complete labels against original PDFs, record actual reviewer provenance, and group
    related products/versions/translations using `evaluation_group`. Keep groups disjoint
    across development and test. Use `purpose='held_out_evaluation'` for final evaluation;
    that file must include both splits and independent labels for every case.

    Set DATASET to your labelled file's absolute path. Set SPLIT to development until
    the protocol is frozen. Keep source context intact; inputs are never silently cut.
    ''')
    replace('dataset-check', '''
    sys.path.insert(0, str(WORK))
    import importlib
    import evaluation
    import benchmark as bench
    importlib.reload(evaluation)
    importlib.reload(bench)
    dataset_path = Path(DATASET) if Path(DATASET).is_absolute() else WORK / DATASET
    dataset = bench.validate_dataset(json.loads(dataset_path.read_text()))
    selected = [c for c in dataset['cases'] if c['split'] == SPLIT]
    assert selected, 'No cases in selected split.'
    if SPLIT == 'test':
        assert dataset['purpose'] == 'held_out_evaluation', 'Test requires independently reviewed held-out data.'
    print('Purpose:', dataset['purpose'], '| cases:', len(selected),
          '| labelled:', sum(c.get('expected') is not None for c in selected))
    print('Dataset SHA256:', bench.digest(dataset))
    template = json.loads((WORK/'public_pilot.json').read_text())
    template['purpose'] = 'reviewed_pilot'
    template['description'] = 'INCOMPLETE annotation template: complete independent labels before use.'
    for case in template['cases']:
        case['expected'] = None
        case['evaluation_group'] = case['product_id']  # Reviewer must group related versions/products.
        case['review'] = {'status': 'pending', 'reviewer_id': '', 'label_version': '', 'reviewed_at': ''}
    bench.atomic_json(OUTPUT/'annotation_template.json', template)
    (OUTPUT/'annotation_and_experiment_guide.md').write_text((WORK/'README.md').read_text())
    ''')
    access = by_id['access-and-revisions']['source']
    access = access.replace("    for name in MODELS_TO_RUN:", '''    frozen = json.loads(Path(PROTOCOL_PATH).read_text()) if Path(PROTOCOL_PATH).exists() else None
    if SPLIT == 'test' and frozen is None:
        raise ValueError('Attach the protocol frozen during development before a test run.')
    for name in MODELS_TO_RUN:''')
    access = access.replace('        requested = REVISIONS[name]', '''        requested = REVISIONS[name]
        if frozen:
            pinned = frozen['revisions'][name]
            if requested is not None and requested != pinned:
                raise ValueError('Requested revision differs from frozen protocol.')
            requested = pinned''')
    replace('access-and-revisions', access)
    after('access-and-revisions', 'freeze-protocol', 'code', '''
    import hashlib
    if RUN_MODELS:
        if SPLIT == 'test' or FREEZE_PROTOCOL:
            assert set(MODELS_TO_RUN) == {'qwen', 'medgemma'}, 'Final paired experiment needs both models.'
            assert dataset['purpose'] == 'held_out_evaluation', 'Freeze only a complete independently reviewed dataset.'
        protocol = {
            'version': 2, 'dataset_sha256': bench.digest(dataset),
            'prompt_sha256': bench.digest(bench.PROMPT), 'schema_sha256': bench.digest(bench.SCHEMA),
            'code_sha256': {name: hashlib.sha256((WORK/name).read_bytes()).hexdigest()
                            for name in ['benchmark.py', 'evaluation.py', 'gpu_run.py']},
            'revisions': {name: resolved[name] for name in MODELS_TO_RUN},
            'quantization': QUANTIZATION, 'max_input_tokens': MAX_INPUT_TOKENS,
            'max_new_tokens': MAX_NEW_TOKENS, 'do_sample': False, 'num_beams': 1,
            'seed': 42, 'qwen_thinking': False, 'packages': bench.environment()['packages'],
            'primary': 'complete_regimen_f1_and_group_macro_case_content_accuracy',
            'bootstrap_resamples': 2000, 'bootstrap_seed': 42,
        }
        evaluation.protocol_guard(PROTOCOL_PATH, protocol, SPLIT, FREEZE_PROTOCOL)
        bench.atomic_json(OUTPUT/'protocol.json', protocol)
        if Path(PROTOCOL_PATH).exists():
            bench.atomic_json(OUTPUT/'frozen_protocol.json', protocol)
        print('Protocol:', 'frozen' if Path(PROTOCOL_PATH).exists() else 'development, not frozen')
    ''')
    after('gpu-comparison', 'paired-analysis', 'code', '''
    # Do not compare incomplete runs or results from different datasets/settings.
    if RUN_MODELS and all(run_status.get(name) == 'completed' for name in ['qwen', 'medgemma']):
        records, manifests = {}, {}
        for name in ['qwen', 'medgemma']:
            records[name] = [json.loads(line) for line in (OUTPUT/name/'predictions.jsonl').read_text().splitlines()]
            manifests[name] = json.loads((OUTPUT/name/'manifest.json').read_text())
            assert {r['case_id'] for r in records[name]} == {c['id'] for c in selected}, 'Incomplete case coverage.'
        a, b = manifests['qwen'], manifests['medgemma']
        for field in ['dataset_sha256', 'split', 'prompt_sha256', 'schema_sha256', 'code_sha256', 'evaluation_sha256', 'environment']:
            assert a[field] == b[field], 'Incompatible paired manifests: '+field
        for field in ['gpu', 'gpu_bytes', 'cuda', 'dtype', 'quantization', 'max_input_tokens', 'max_new_tokens']:
            assert a['model'][field] == b['model'][field], 'Incompatible paired model settings: '+field
        paired = evaluation.paired_report(dataset, records['qwen'], records['medgemma'])
        paired['purpose'] = dataset['purpose']
        bench.atomic_json(OUTPUT/'paired_comparison.json', paired)
        evaluation.export_blind_review(dataset, records['qwen'], records['medgemma'], OUTPUT)
        print(json.dumps(paired, indent=2))
    else:
        print('No paired model analysis: both complete runs are required. Offline tests are not model results.')
    ''')
    export = by_id['comparison-and-export']['source']
    export = export.replace("'citation_substring_rate', 'median_generation_seconds', 'peak_allocated_bytes']}})",
                            "'strict_case_exact_rate', 'citation_substring_rate', 'peak_allocated_bytes']},\n"
                            "                 'complete_regimen_f1': report['metrics'].get('findings', {}).get('f1'),\n"
                            "                 'evidence_correctness': report['evidence_correctness']})")
    export = export.replace("import shutil", "import shutil\ntry:\n    from IPython.display import FileLink, display\nexcept ImportError:\n    display = print\n    FileLink = str")
    export = export.replace('from IPython.display import FileLink, display\ndisplay(FileLink(archive))', 'display(FileLink(archive))')
    replace('comparison-and-export', export)
    replace('interpretation', '''
    ## Read the results correctly

    - `comparison.csv`: normalized/strict case accuracy, complete-regimen F1, schema
      success, quote presence, memory. Quote presence does not establish semantic support.
    - Per-model `summary.json`: aligned field/qualifier diagnostics and failure counts.
      Primary complete-regimen scores preserve all field associations and qualifiers.
    - `paired_comparison.json`: paired success outcomes and equal-group-weight accuracy
      difference (Qwen minus MedGemma), with a paired group bootstrap interval. This is
      not the same estimand as case-weighted accuracy in comparison.csv.
    - `blind_review.csv`: all predictions for human evidence/error review. Blank ratings
      are unreviewed. Send this file alone; retain the separate identity key yourself.
    - `frozen_protocol.json`, manifests and raw outputs: preserve these for reproducibility.
    - Timing is exploratory, single-pass, on successful common cases only. It includes
      cold start and supports no speed ranking. Failures remain in accuracy denominators.

    No automatic winner is declared. Small or synthetic datasets do not establish
    clinical performance. Exact extractive scoring still needs adjudication for equivalent
    phrase scopes. Do not change normalization or labels in response to a preferred result.

    For a new session, restore the run folder and frozen protocol, or use a new run name.
    Read the exported annotation_and_experiment_guide.md for the full protocol.
    ''')
    path = ROOT/'renalsafe_model_comparison_v2.ipynb'
    path.write_text(json.dumps(notebook, indent=1, ensure_ascii=False)+'\n')
    return path


if __name__ == '__main__':
    print(build())
