"""Build unlabelled cases from locally pinned Polish ChPLs; never infer labels."""
import hashlib
import json
from pathlib import Path
import re

from benchmark import validate_dataset

ROOT = Path(__file__).resolve().parent
SOURCES = ROOT.parents[1] / 'data/sources/pl-2026-11-09'


def digest(data):
    return hashlib.sha256(data).hexdigest()


def heading(text, section, title):
    # Require a heading title so inline cross-references cannot become boundaries.
    pattern = r'(?mi)(?:^|(?<=\f))[ \t]*' + re.escape(section) + r'\.?[ \t]+' + title
    matches = list(re.finditer(pattern, text))
    if len(matches) != 1:
        raise ValueError(f'Expected one heading {section} {title}; found {len(matches)}')
    return matches[0].start()


def build():
    manifest = json.loads((SOURCES / 'manifest.json').read_text())
    cases = []
    for doc in manifest['documents']:
        pdf = (SOURCES / doc['pdf_path']).read_bytes()
        raw = (SOURCES / doc['text_path']).read_bytes()
        # Some registry files have an XMP preamble before the PDF header.
        assert b'%PDF-' in pdf[:65536] and digest(pdf) == doc['pdf_sha256']
        assert digest(raw) == doc['extracted_text_sha256']
        text = raw.decode('utf-8')
        bounds = {s: heading(text, s, title) for s, title in (
            ('4.1', 'Wskazania'), ('4.2', 'Dawkowanie'),
            ('4.3', 'Przeciwwskazania'), ('4.4', 'Specjalne'),
            ('4.5', 'Interakcje'), ('5.2', 'Właściwości[ \t]+farmakokinetyczne'),
            ('5.3', 'Przedkliniczne'))}
        assert list(bounds.values()) == sorted(bounds.values())
        # Identity includes product name, strengths, composition and formulation.
        # No sentence/renal-keyword cropping: retain complete sections and footnotes.
        scopes = [('', ['1-4.2'], [(0, bounds['4.3'])]),
                  ('-safety', ['1-3', '4.3', '4.4'], [(0, bounds['4.1']), (bounds['4.3'], bounds['4.5'])]),
                  ('-pk', ['1-3', '5.2'], [(0, bounds['4.1']), (bounds['5.2'], bounds['5.3'])])]
        for suffix, sections, spans in scopes:
            ranges, pieces = [], []
            for start, end in spans:
                pieces.append(text[start:end])
                ranges.append(dict(start_char=start, end_char=end,
                                   start_byte=len(text[:start].encode('utf-8')),
                                   end_byte=len(text[:end].encode('utf-8')),
                                   physical_pages=list(range(text[:start].count('\f') + 1,
                                                             text[:end-1].count('\f') + 2))))
            source_text = '\n\n'.join(pieces)
            cases.append(dict(
                id=doc['product_id'] + suffix, product_id=doc['product_id'],
                document_id=doc['pdf_sha256'], active_substance=doc['active_substance'],
                evaluation_group=doc['active_substance'], language='pl', split='development',
                synthetic=False, source_text=source_text,
                source_sha256=digest(source_text.encode('utf-8')), expected=None,
                review=dict(status='NOT_REVIEWED'),
                provenance=dict(
                    source_url=doc['source_url'], source_type='ChPL',
                    retrieved_at=doc['retrieved_at'], pdf_sha256=doc['pdf_sha256'],
                    extracted_text_sha256=doc['extracted_text_sha256'],
                    pdf_path=str((SOURCES / doc['pdf_path']).relative_to(ROOT.parents[1])),
                    text_path=str((SOURCES / doc['text_path']).relative_to(ROOT.parents[1])),
                    sections=sections, source_ranges=ranges,
                    extraction=doc['extraction'],
                    note='Exact source slices joined with two newlines. Partial-document input; '
                         'do not infer absent dosing, cross-references or overall safety. '
                         'Check complete PDF and table layout before independent annotation. '
                         'Related cases share one development group; no clinical labels.')))
    dataset = dict(schema_version=1, purpose='unlabelled_pilot',
                   description='Product-specific Polish ChPL excerpts: full dosing context, '
                               'contraindications/warnings, and pharmacokinetics. '
                               'Pinned official PDFs; unlabelled development inputs, '
                               'not independent clinical cases or prescribing rules.', cases=cases)
    validate_dataset(dataset)
    (ROOT / 'public_pilot.json').write_text(json.dumps(dataset, ensure_ascii=False, indent=2) + '\n')
    print(f'Built {len(cases)} cases from {len(manifest["documents"])} ChPL snapshots')


if __name__ == '__main__':
    build()
