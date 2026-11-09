"""Paired evaluation; stdlib only. No model calls or automatic clinical adjudication."""
from collections import Counter
import csv
import hashlib
import json
from pathlib import Path
import random
import statistics
import unicodedata

FIELDS = ('metric', 'method', 'renal_unit', 'lower_value', 'lower_operator',
          'upper_value', 'upper_operator', 'dose_value', 'dose_unit',
          'interval_value', 'interval_unit', 'instruction', 'population', 'route')
GROUPS = {'renal': FIELDS[:7], 'dose': FIELDS[7:9], 'interval': FIELDS[9:11]}
TEXT_FIELDS = {'instruction', 'conditions', 'unresolved'}


def normalize(value, field):
    if value is None:
        return None
    # Deliberately conservative: no numeric rounding, unit conversion, case folding,
    # operator changes, or synonym substitution. Only prose terminal periods removed.
    value = ' '.join(unicodedata.normalize('NFC', value).split())
    return value[:-1] if field in TEXT_FIELDS and value.endswith('.') and not value.endswith('..') else value


def row_key(row, normalized=True):
    def norm(v, f):
        return normalize(v, f) if normalized else v
    return json.dumps({**{f: norm(row[f], f) for f in FIELDS},
                       **{f: sorted(norm(v, f) for v in row[f])
                          for f in ('conditions', 'unresolved')}}, sort_keys=True)


def counts(a, b):
    a, b = Counter(a), Counter(b)
    tp = sum((a & b).values())
    return dict(tp=tp, fp=sum(a.values())-tp, fn=sum(b.values())-tp)


def alignment(predicted, truth):
    """Maximum-weight one-to-one assignment (Hungarian), padded with empty rows.

    Renal attributes weigh 10 each; other non-null scalar matches weigh 1 each.
    Assignment only drives diagnostics. Primary exact-regimen counts are independent
    of this heuristic and compare complete records, including their qualifiers.
    """
    n = max(len(predicted), len(truth))
    if not n:
        return []
    p = predicted + [None] * (n-len(predicted))
    t = truth + [None] * (n-len(truth))
    costs = []
    for a in p:
        costs.append([-sum((10 if f in GROUPS['renal'] else 1)
                          for f in FIELDS if a is not None and b is not None
                          and a[f] is not None
                          and normalize(a[f], f) == normalize(b[f], f)) for b in t])
    u, v, match, way = [0]*(n+1), [0]*(n+1), [0]*(n+1), [0]*(n+1)
    for i in range(1, n+1):
        match[0] = i
        j0, minimum, used = 0, [float('inf')]*(n+1), [False]*(n+1)
        while True:
            used[j0] = True
            i0, delta, j1 = match[j0], float('inf'), 0
            for j in range(1, n+1):
                if not used[j]:
                    current = costs[i0-1][j-1]-u[i0]-v[j]
                    if current < minimum[j]:
                        minimum[j], way[j] = current, j0
                    if minimum[j] < delta:
                        delta, j1 = minimum[j], j
            for j in range(n+1):
                if used[j]:
                    u[match[j]] += delta
                    v[j] -= delta
                else:
                    minimum[j] -= delta
            j0 = j1
            if match[j0] == 0:
                break
        while j0:
            j1 = way[j0]
            match[j0] = match[j1]
            j0 = j1
    return [(p[match[j]-1], t[j-1]) for j in range(1, n+1)]


def score(expected, predicted):
    rows = predicted['findings'] if predicted is not None else []
    truth = expected['findings']
    result = {'whole_case_exact': predicted is not None and
              Counter(map(row_key, rows)) == Counter(map(row_key, truth)),
              'strict_case_exact': predicted is not None and
              Counter(row_key(r, False) for r in rows) == Counter(row_key(r, False) for r in truth),
              'abstention_tp': int(predicted is not None and predicted['abstain'] and expected['abstain']),
              'abstention_fp': int(predicted is not None and predicted['abstain'] and not expected['abstain']),
              'abstention_fn': int(expected['abstain'] and (predicted is None or not predicted['abstain']))}
    pairs = alignment(rows, truth)
    for name, fields in {**{f: (f,) for f in FIELDS}, **GROUPS,
                         'conditions': ('conditions',), 'unresolved': ('unresolved',)}.items():
        totals = dict(tp=0, fp=0, fn=0)
        def project(r):
            if r is None:
                return []
            if name in ('conditions', 'unresolved'):
                return [normalize(s, name) for s in r[name]]
            return [tuple(normalize(r[f], f) for f in fields)] if any(r[f] is not None for f in fields) else []
        for a, b in pairs:
            for k, value in counts(project(a), project(b)).items():
                totals[k] += value
        result[name] = totals
    result['findings'] = counts(map(row_key, rows), map(row_key, truth))
    return result


def protocol_guard(path, protocol, split, freeze=False):
    """Test requires an unchanged protocol frozen during development; never overwrite."""
    path = Path(path)
    if path.exists():
        if json.loads(path.read_text()) != protocol:
            raise ValueError('Frozen protocol mismatch. Restore it or start a new development experiment.')
    elif split == 'test':
        raise ValueError('Test requires a protocol frozen during development.')
    elif freeze:
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open('x') as f:
            json.dump(protocol, f, indent=2)


def paired_report(dataset, left, right, resamples=2000, seed=42):
    cases = {c['id']: c for c in dataset['cases']}
    a, b = ({r['case_id']: r for r in records} for records in (left, right))
    if len(a) != len(left) or len(b) != len(right) or set(a) != set(b) or not a:
        raise ValueError('Paired comparison requires identical, unique, nonempty case sets.')
    if not set(a) <= set(cases):
        raise ValueError('Unknown case in paired results.')
    labelled = [i for i in sorted(a) if cases[i].get('expected') is not None]
    matched = [i for i in sorted(a) if a[i]['status'] == b[i]['status'] == 'ok'
               and all('generation_seconds' in r for r in (a[i], b[i]))]
    report = {'cases': len(a), 'labelled_cases': len(labelled),
              'successful_paired_timing_cases': len(matched),
              'latency_note': 'Exploratory single-pass generation latency, including cold start; no speed ranking.',
              'paired_latency_median_seconds': {
                  name: statistics.median(records[i]['generation_seconds'] for i in matched) if matched else None
                  for name, records in [('qwen', a), ('medgemma', b)]},
              'status_counts': {name: dict(Counter(r['status'] for r in records.values()))
                                for name, records in [('qwen', a), ('medgemma', b)]}}
    if not labelled:
        report['accuracy'] = None
        return report
    groups = {}
    outcomes = Counter()
    for i in labelled:
        x, y = (int(records[i]['scores']['whole_case_exact']) for records in (a, b))
        outcomes[['neither', 'medgemma_only', 'qwen_only', 'both'][2*x+y]] += 1
        c = cases[i]
        groups.setdefault(c.get('evaluation_group', c['product_id']), []).append(x-y)
    # Equal weight per independent product/duplicate group; keep both predictions paired.
    differences = [statistics.mean(values) for values in groups.values()]
    rng = random.Random(seed)
    interval = None
    if len(groups) >= 2:
        draws = sorted(statistics.mean(rng.choices(differences, k=len(differences))) for _ in range(resamples))
        interval = [draws[int(.025*(resamples-1))], draws[int(.975*(resamples-1))]]
    report['accuracy'] = {'paired_case_outcomes': dict(outcomes), 'independent_groups': len(groups),
                          'group_macro_accuracy_difference_qwen_minus_medgemma': statistics.mean(differences),
                          'paired_group_bootstrap_95_percentile_interval': interval,
                          'resamples': resamples, 'seed': seed,
                          'note': 'Small group counts give unstable intervals. Fixture results are software checks only.'}
    return report


def export_blind_review(dataset, left, right, folder):
    """Export all cases, including failures. Keep key separate from reviewer workbook."""
    folder = Path(folder)
    cases = {c['id']: c for c in dataset['cases']}
    entries = [(model, r) for model, records in [('qwen', left), ('medgemma', right)] for r in records]
    random.Random(42).shuffle(entries)
    key = {}
    with (folder/'blind_review.csv').open('w', newline='', encoding='utf-8') as f:
        fields = ['review_id', 'case_id', 'source_text', 'reference_json', 'prediction_json',
                  'status', 'evidence_supported', 'wrong_band_dose_association',
                  'missing_qualifier', 'unsupported_addition', 'notes']
        writer = csv.DictWriter(f, fieldnames=fields)
        writer.writeheader()
        for n, (model, record) in enumerate(entries):
            rid = f'R{n+1:04d}'
            case = cases[record['case_id']]
            key[rid] = {'model': model, 'case_id': case['id']}
            writer.writerow(dict(review_id=rid, case_id=case['id'], source_text=case['source_text'],
                                 reference_json=json.dumps(case.get('expected'), ensure_ascii=False),
                                 prediction_json=record.get('raw'), status=record['status']))
    (folder/'review_key_DO_NOT_SEND_TO_REVIEWERS.json').write_text(json.dumps(key, indent=2))
