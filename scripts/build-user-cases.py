#!/usr/bin/env python3
"""Generate fictional patient inputs and source-linked clinical review tasks.

No clinical answer or approval is generated. Expected results describe software
behavior against a source-only knowledge snapshot with no approved rules.
"""
from copy import deepcopy
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def build():
    manifest = json.loads((ROOT / 'data/sources/pl-2026-11-09/manifest.json').read_text())
    docs = {d['product_id']: d for d in manifest['documents']}
    template = json.loads((ROOT / 'data/demo-two-drugs.json').read_text())
    template['assessment_at'] = '2026-11-09T12:00:00Z'
    template['patient']['measured_at'] = '2026-11-09T11:00:00Z'
    # Structured context strings are engineering mappings to these exact products.
    # Evidence quotes below preserve the original Polish text for human review.
    specs = [
        ('rpl-4282', template['medications'][0], 'Meronem, 1 g,',
         'szpitalne', 'dożylnego',
         'Check the unit-dose dependency, printed 10–25/26–50 gaps, dialysis timing and indication.'),
        ('rpl-24542', template['medications'][1], 'Ceftazidime Kabi, 1000 mg,',
         'Szpitalne zapalenie płuc.', 'dożyln',
         'Check loading versus maintenance dose, renal tables, infusion method and dialysis exceptions.'),
        ('rpl-7911', dict(product='Tartriakson 1 g', route='intravenous',
                          form='powder for solution for injection and infusion',
                          indication='hospital-acquired pneumonia'), 'Tartriakson, 1 g,',
         'szpitalne zapalenie płuc,', 'dożylnej',
         'Check the renal recommendation together with hepatic function and dialysis qualifiers.'),
        ('rpl-43752', dict(product='Metoclopramide hameln 5 mg/ml', route='intravenous',
                           form='solution for injection', indication='symptomatic nausea and vomiting'),
         'Metoclopramide hameln, 5 mg/ml,', 'Objawowym leczeniu nudności i wymiotów,', 'dożylnie',
         'Check renal dose reduction, maximum daily dose, weight limit, dose spacing and duration.'),
        ('rpl-46038', dict(product='Sugammadex Reddy 100 mg/ml', route='intravenous',
                           form='solution for injection', indication='reversal of rocuronium-induced neuromuscular blockade'),
         'Sugammadex Reddy, 100 mg/ml,', 'Odwrócenie blokady przewodnictwa', 'podawać dożylnie',
         'Check renal limitations, blockade depth, body-weight dosing and anaesthetist monitoring.'),
        ('rpl-37424', dict(product='PARACETAMOL MEDICINAE 500 mg', route='oral',
                           form='tablet', indication='mild to moderate pain'),
         'PARACETAMOL MEDICINAE, 500 mg,', 'łagodnego i umiarkowanego bólu', 'Wyłącznie podanie doustne.',
         'Check renal/hepatic warnings and total paracetamol exposure across products; do not invent a renal band.'),
        ('ema-cayston', dict(product='Cayston 75 mg', route='inhalation',
                            form='powder and solvent for nebuliser solution',
                            indication='suppressive therapy of chronic Pseudomonas aeruginosa infection in cystic fibrosis'),
         'Cayston 75 mg proszek', 'wywołanych przez Pseudomonas aeruginosa', 'Do podawania wziewnego.',
         'Check inhalation-specific renal advice and eligibility. Never transfer this recommendation to IV aztreonam.'),
    ]
    contexts = []
    for ident, context, product, indication, route, task in specs:
        d = docs[ident]
        text = (ROOT / 'data/sources/pl-2026-11-09' / d['text_path']).read_text()
        quotes = []
        for anchor in (product, indication, route):
            pos = text.index(anchor)
            # Preserve an exact full source line containing the anchor.
            start, end = text.rfind('\n', 0, pos) + 1, text.find('\n', pos)
            quotes.append(text[start:end if end >= 0 else len(text)])
        contexts.append(dict(source_id=ident, context=context, source_url=d['source_url'],
                             pdf_sha256=d['pdf_sha256'], text_sha256=d['extracted_text_sha256'],
                             context_evidence=quotes, mapping_review_status='NOT_REVIEWED',
                             clinical_review_task=task,
                             chpl_case_ids=[ident, ident + '-safety', ident + '-pk']))
    cases = []

    def add(ident, title, indices=(0,), mutate=None, metrics=('EGFR_CKDEPI_2021', 'CRCL_CG'),
            warnings=(), crcl=25.0, low=None, high=None, result_reason='NO_APPROVED_RULE'):
        request = deepcopy(template)
        request['medications'] = [deepcopy(contexts[i]['context']) for i in indices]
        if mutate:
            mutate(request)
        expected = dict(status='MANUAL_REVIEW_REQUIRED', result_reason=result_reason,
                        metrics=list(metrics), renal_warnings=list(warnings),
                        crcl=crcl if 'CRCL_CG' in metrics else None,
                        crcl_low=low, crcl_high=high, automated_dose_allowed=False)
        cases.append(dict(id=ident, title=title, synthetic_patient=True,
                          source_ids=[contexts[i]['source_id'] for i in indices],
                          request=request, expected_software=expected,
                          clinical_reference=None, clinical_review_status='NOT_REVIEWED'))

    for i, c in enumerate(contexts):
        add(c['source_id'] + '-baseline', 'Stable fictional adult: ' + c['context']['product'], (i,))
    # ChPL threshold probes are calculation/review tasks, not asserted dosing labels.
    for target in (9.99, 10, 10.01, 25, 25.5, 26, 50, 50.5, 51):
        add('meropenem-crcl-' + str(target).replace('.', '_'),
            f'Unrounded CrCl {target} mL/min: inspect the printed ChPL band',
            mutate=lambda r, v=target: r['patient']['creatinine'].update(value=70 / v), crcl=target)

    def patient(field, value):
        return lambda r: r['patient'].__setitem__(field, value)

    for ident, field, value, warning in (
        ('suspected-aki', 'suspected_aki', True, 'AKI_OR_UNKNOWN'),
        ('unknown-aki', 'suspected_aki', None, 'AKI_OR_UNKNOWN'),
        ('dialysis', 'dialysis', True, 'DIALYSIS_OR_UNKNOWN'),
        ('unknown-dialysis', 'dialysis', None, 'DIALYSIS_OR_UNKNOWN'),
        ('unstable', 'stable_renal_function', False, 'STABILITY_UNCONFIRMED'),
        ('child', 'age_years', 17, 'AGE_OUT_OF_SCOPE'),
        ('unknown-equation-sex', 'equation_sex', '', 'EQUATION_SEX_REQUIRED'),
        ('non-idms', 'idms_standardized', False, 'CREATININE_STANDARDIZATION_UNCONFIRMED'),
        ('uncertain-body-composition', 'typical_body_composition', False, 'BODY_COMPOSITION_UNCERTAIN'),
        ('stale-lab', 'measured_at', '2026-11-01T11:00:00Z', 'CREATININE_TIMESTAMP_INVALID_OR_STALE'),
        ('future-lab', 'measured_at', '2026-11-09T13:00:00Z', 'CREATININE_TIMESTAMP_INVALID_OR_STALE'),
        ('unsupported-creatinine-unit', 'creatinine', dict(value=2.8, unit='mmol/L'), 'CREATININE_UNIT_UNSUPPORTED'),
        ('extreme-body-size', 'weight', dict(value=120, unit='kg'), 'BODY_SIZE_REQUIRES_REVIEW'),
    ):
        add(ident, 'Require review: ' + ident, mutate=patient(field, value), metrics=(), warnings=(warning,))
    for field, warning in (('weight', 'CG_WEIGHT_REQUIRED'), ('height', 'CG_HEIGHT_REQUIRED_FOR_BODY_SIZE_CHECK')):
        add('missing-' + field, 'Do not substitute eGFR for unavailable CrCl',
            mutate=lambda r, f=field: r['patient'].pop(f), metrics=('EGFR_CKDEPI_2021',), warnings=(warning,))
    add('unconfirmed-weight', 'Actual weight is unconfirmed',
        mutate=patient('actual_weight_confirmed', False), metrics=('EGFR_CKDEPI_2021',),
        warnings=('CG_ACTUAL_WEIGHT_UNCONFIRMED',))
    add('equivalent-units', 'Equivalent serum creatinine in micromol/L',
        mutate=patient('creatinine', dict(value=247.52, unit='umol/L')))
    add('known-uncertainty', 'Known creatinine uncertainty spans a ChPL table boundary',
        mutate=patient('creatinine_bounds', dict(lower=2.6, upper=3, lower_inclusive=True, upper_inclusive=True)),
        low=70 / 3, high=70 / 2.6)
    add('invalid-uncertainty', 'Bounds do not contain the measured creatinine',
        mutate=patient('creatinine_bounds', dict(lower=1, upper=2, lower_inclusive=True, upper_inclusive=True)),
        metrics=(), warnings=('CREATININE_BOUNDS_INVALID',))
    add('two-medications', 'Preserve separate medication results and order', indices=(0, 1))
    add('three-medications', 'Review each source separately; no interaction safety claim', indices=(2, 3, 5))
    add('female-equation', 'Apply the female Cockcroft–Gault factor', mutate=patient('equation_sex', 'female'), crcl=21.25)
    add('missing-indication', 'Incomplete product context requires review',
        mutate=lambda r: r['medications'][0].update(indication=''), result_reason='MEDICATION_CONTEXT_REQUIRED')
    data = dict(schema_version=1, purpose='synthetic_patient_software_cases',
                description='Fictional patient values; product contexts linked to Polish ChPL. '
                            'Software expectations use a source-only snapshot with zero approved rules. '
                            'No clinical gold answers, prescribing advice or approvals.',
                contexts=contexts, cases=cases)
    (ROOT / 'data/user-cases.json').write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
    out = ROOT / 'outputs/user-cases/requests'
    out.mkdir(parents=True, exist_ok=True)
    for case in cases:
        (out / (case['id'] + '.json')).write_text(json.dumps(case['request'], ensure_ascii=False, indent=2) + '\n')
    print(f'Generated {len(cases)} synthetic user cases across {len(contexts)} ChPL product contexts; requests: {out}')


if __name__ == '__main__':
    build()
