"""Portable regressions: all predictions here are artificial, never model results."""
import copy
import itertools
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import benchmark as b
import evaluation as e

ROOT = Path(__file__).parent


class EvaluationTests(unittest.TestCase):
    def setUp(self):
        self.data = json.loads((ROOT/'synthetic_cases.json').read_text())
        self.expected = copy.deepcopy(self.data['cases'][0]['expected'])

    def test_swapped_doses_lose_aligned_credit(self):
        self.expected['findings'] += copy.deepcopy(self.data['cases'][1]['expected']['findings'])
        pred = copy.deepcopy(self.expected)
        pred['findings'][0]['dose_value'], pred['findings'][1]['dose_value'] = '100', '250'
        result = e.score(self.expected, pred)
        self.assertEqual(result['dose'], dict(tp=0, fp=2, fn=2))
        self.assertEqual(result['findings']['tp'], 0)
        self.assertFalse(result['whole_case_exact'])

    def test_qualifiers_stay_attached(self):
        self.expected['findings'] += copy.deepcopy(self.data['cases'][1]['expected']['findings'])
        pred = copy.deepcopy(self.expected)
        pred['findings'][0]['conditions'], pred['findings'][1]['conditions'] = pred['findings'][1]['conditions'], []
        self.assertEqual(e.score(self.expected, pred)['conditions'], dict(tp=0, fp=1, fn=1))

    def test_normalization_keeps_critical_values_strict(self):
        pred = copy.deepcopy(self.expected)
        pred['findings'][0]['instruction'] = '250  mg every 12 hours.'
        result = e.score(self.expected, pred)
        self.assertTrue(result['whole_case_exact'])
        self.assertFalse(result['strict_case_exact'])
        for key, value in [('dose_value', '2500'), ('dose_unit', 'g'), ('upper_operator', '<=')]:
            changed = copy.deepcopy(pred)
            changed['findings'][0][key] = value
            self.assertFalse(e.score(self.expected, changed)['whole_case_exact'])

    def test_order_duplicates_and_missing(self):
        self.expected['findings'] += copy.deepcopy(self.data['cases'][1]['expected']['findings'])
        pred = copy.deepcopy(self.expected)
        pred['findings'].reverse()
        self.assertTrue(e.score(self.expected, pred)['whole_case_exact'])
        pred['findings'].append(copy.deepcopy(pred['findings'][0]))
        self.assertEqual(e.score(self.expected, pred)['findings']['fp'], 1)
        self.assertEqual(e.score(self.expected, None)['dose']['fn'], 2)

    def test_assignment_is_globally_optimal(self):
        rows = [copy.deepcopy(self.expected['findings'][0]) for _ in range(3)]
        for i, row in enumerate(rows):
            row.update(upper_value=str(i), dose_value=str(100*i))
        pred = copy.deepcopy(rows)
        pred[0]['upper_value'] = '2'
        pred[2]['dose_value'] = '100'
        def weight(a, c):
            return sum((10 if f in e.GROUPS['renal'] else 1) for f in e.FIELDS
                       if a[f] is not None and a[f] == c[f])
        optimum = max(sum(weight(a, c) for a, c in zip(pred, permutation)) for permutation in itertools.permutations(rows))
        self.assertEqual(sum(weight(a, c) for a, c in e.alignment(pred, rows)), optimum)

    def test_quote_presence_is_not_semantic_support(self):
        pred = copy.deepcopy(self.expected)
        pred['findings'][0]['evidence'] = {k: 'FICTIONAL' for k in pred['findings'][0]['evidence']}
        hits, total = b.citation_counts(pred, self.data['cases'][0]['source_text'])
        self.assertEqual(hits, total)  # This must never be advertised as evidence correctness.
        self.assertTrue(e.score(self.expected, pred)['whole_case_exact'])

    def test_negative_failure_is_not_correct_abstention(self):
        expected = self.data['cases'][-1]['expected']
        self.assertFalse(e.score(expected, None)['whole_case_exact'])
        self.assertEqual(e.score(expected, None)['abstention_fn'], 1)

    def test_protocol_missing_changed_and_frozen(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)/'protocol.json'
            with self.assertRaisesRegex(ValueError, 'frozen'):
                e.protocol_guard(path, {'version': 1}, 'test')
            e.protocol_guard(path, {'version': 1}, 'development', True)
            e.protocol_guard(path, {'version': 1}, 'test')
            with self.assertRaisesRegex(ValueError, 'mismatch'):
                e.protocol_guard(path, {'version': 2}, 'test')
            self.assertEqual(json.loads(path.read_text()), {'version': 1})

    def records(self):
        return [dict(case_id=c['id'], status='ok', raw=b.canonical(c['expected']),
                     scores=e.score(c['expected'], c['expected']), generation_seconds=1)
                for c in self.data['cases']]

    def test_paired_results_and_failure_timing(self):
        left, right = self.records(), self.records()
        right[0].update(status='runtime_error', scores=e.score(self.expected, None))
        right[0].pop('generation_seconds')
        report = e.paired_report(self.data, left, right)
        self.assertEqual(report['successful_paired_timing_cases'], 7)
        self.assertEqual(report['accuracy']['paired_case_outcomes']['qwen_only'], 1)
        self.assertEqual(report['accuracy']['group_macro_accuracy_difference_qwen_minus_medgemma'], 1/8)
        self.assertEqual(report, e.paired_report(self.data, left, right))
        with self.assertRaises(ValueError):
            e.paired_report(self.data, left, right[:-1])
        with self.assertRaises(ValueError):
            e.paired_report(self.data, left+left[:1], right)

    def test_bootstrap_weights_groups_not_passages(self):
        left, right = self.records()[:3], self.records()[:3]
        for c in self.data['cases'][:2]:
            c['evaluation_group'] = 'same-product-group'
        right[0]['scores']['whole_case_exact'] = False
        report = e.paired_report(self.data, left, right)
        self.assertEqual(report['accuracy']['independent_groups'], 2)
        self.assertEqual(report['accuracy']['group_macro_accuracy_difference_qwen_minus_medgemma'], .25)

    def test_unlabelled_has_no_accuracy(self):
        records = self.records()
        for c in self.data['cases']:
            c['expected'] = None
        self.assertIsNone(e.paired_report(self.data, records, records)['accuracy'])

    def test_group_and_identical_source_leakage(self):
        extra = copy.deepcopy(self.data['cases'][0])
        extra.update(id='extra', product_id='other', document_id='other', split='test')
        self.data['cases'].append(extra)
        with self.assertRaisesRegex(ValueError, 'leakage'):
            b.validate_dataset(self.data)
        self.data['cases'].pop()
        self.data['cases'][0]['evaluation_group'] = 'group'
        self.data['cases'][1].update(evaluation_group='group', split='test')
        with self.assertRaisesRegex(ValueError, 'leakage'):
            b.validate_dataset(self.data)

    def test_blind_review_hides_model_names(self):
        with tempfile.TemporaryDirectory() as directory:
            e.export_blind_review(self.data, self.records(), self.records(), directory)
            text = (Path(directory)/'blind_review.csv').read_text()
            self.assertNotIn('qwen', text)
            self.assertNotIn('medgemma', text)
            key = json.loads((Path(directory)/'review_key_DO_NOT_SEND_TO_REVIEWERS.json').read_text())
            self.assertEqual(len(key), 16)

    @patch.object(b, 'environment', return_value={'test': True})
    def test_run_errors_checkpoint_and_summary(self, _):
        class Fake:
            identity = {'model_id': 'FAKE-NOT-A-MODEL'}
            calls = 0
            def generate(adapter, case):
                adapter.calls += 1
                return dict(raw='invalid' if adapter.calls == 1 else b.canonical(case['expected']),
                            truncated=False, generation_seconds=1)
        adapter = Fake()
        with tempfile.TemporaryDirectory() as directory:
            report = b.run(adapter, self.data, directory)
            self.assertEqual(report['whole_case_exact_rate'], 7/8)
            self.assertIn('human review', report['evidence_correctness'])
            self.assertEqual(b.run(adapter, self.data, directory), report)
            self.assertEqual(adapter.calls, 8)


if __name__ == '__main__':
    unittest.main()
