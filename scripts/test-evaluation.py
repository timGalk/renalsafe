#!/usr/bin/env python3
"""Offline regression of evaluation scoring and invalid-run handling."""
import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('evaluation', Path(__file__).with_name('evaluate-retrieval.py'))
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)
SHA = 'a'*64
CONTEXT = dict(product='fixture', route='oral', form='tablet', indication='fixture')
LABEL = dict(document_sha256=SHA, section='4.2', quote='exact renal quote')
CASE = dict(id='fixture', context=CONTEXT, query='renal', relevant=[LABEL])
HIT = dict(source=dict(document_sha256=SHA), chunk=dict(document_id='fixture-document', section='4.2', text='exact renal quote'))


class EvaluationTests(unittest.TestCase):
    def test_recall_does_not_count_duplicates(self):
        case = dict(CASE, relevant=[LABEL, dict(LABEL, quote='missing')])
        result = dict(hits=[HIT, HIT], status='EVIDENCE_ONLY')
        score = e.score(case, result, {SHA})
        self.assertEqual(score['recall_at_k'], .5)
        self.assertEqual(score['citation_precision'], 1)

    def test_wrong_source_and_section_cannot_match(self):
        wrong = copy.deepcopy(HIT)
        wrong['source']['document_sha256'] = 'b'*64
        section = copy.deepcopy(HIT)
        section['chunk']['section'] = '4.3'
        score = e.score(CASE, dict(hits=[wrong, section], status='EVIDENCE_ONLY'), {SHA})
        self.assertEqual(score['wrong_source_count'], 1)
        self.assertEqual(score['recall_at_k'], 0)
        self.assertEqual(score['citation_precision'], 0)

    def test_same_pdf_in_wrong_context_is_rejected(self):
        score = e.score(CASE, dict(hits=[HIT], status='EVIDENCE_ONLY'), {SHA}, {'other-context-document'})
        self.assertEqual(score['wrong_source_count'], 1)
        self.assertEqual(score['recall_at_k'], 0)

    def test_abstention_denominators(self):
        case = dict(CASE, relevant=[], expect_abstention=True)
        score = e.score(case, dict(hits=[], status='NO_EVIDENCE'), {SHA})
        self.assertIsNone(score['recall_at_k'])
        self.assertIsNone(score['citation_precision'])
        self.assertTrue(score['expected_abstention_correct'])
        self.assertTrue(score['status_consistent'])

    def test_dataset_validation(self):
        dataset = dict(schema_version=1, purpose='software_fixture', cases=[CASE])
        e.validate(dataset)
        for change in [dict(cases=[]), dict(purpose='independent_evaluation'), dict(cases=[CASE, CASE]),
                       dict(cases=[dict(CASE, relevant=[])]), dict(cases=[dict(CASE, relevant=[LABEL, LABEL])])]:
            with self.assertRaises(ValueError):
                e.validate(dict(dataset, **change))

    def test_local_api_only(self):
        for url in ['https://example.com', 'http://127.0.0.1@evil.com', 'http://localhost/?key=x']:
            with self.assertRaises(ValueError): e.API(url, 'not-a-key')

    def test_fallback_and_revision_drift_invalidates_comparison(self):
        class FakeAPI:
            def __init__(self, drift): self.n, self.drift = 0, drift
            def call(self, path, body=None):
                if path == '/documents':
                    return [dict(id='fixture-document', context=CONTEXT, source=dict(document_sha256=SHA), page_start=1, page_end=1, status='ACTIVE')]
                self.n += 1
                return dict(hits=[HIT], status='EVIDENCE_ONLY', mode='lexical',
                            degraded_reason='LOCAL_EMBEDDER_NOT_CONFIGURED' if body['mode']=='hybrid' else '',
                            knowledge_revision=self.n if self.drift else 1)
        dataset = dict(schema_version=1, purpose='software_fixture', cases=[CASE])
        report = e.run(FakeAPI(False), dataset, ['lexical','hybrid'], 5, 'fixture')
        self.assertTrue(report['complete'])
        self.assertFalse(report['comparison_valid'])
        report = e.run(FakeAPI(True), dataset, ['lexical','hybrid'], 5, 'fixture')
        self.assertFalse(report['complete'])


if __name__ == '__main__': unittest.main()
