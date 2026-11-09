"""No GPU, downloads, model output or clinical labels needed for these regressions."""
import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import benchmark as b

ROOT = Path(__file__).parent


class FakeAdapter:
    identity = {"model_id": "FAKE-NOT-A-MODEL", "revision": "fixture-v1"}
    calls = 0

    def __init__(self, answers):
        self.answers = answers

    def generate(self, case):
        self.calls += 1
        answer = self.answers[case["id"]]
        if isinstance(answer, Exception):
            raise answer
        return {"raw": answer, "truncated": False, "generation_seconds": 1.0}


class BenchmarkTests(unittest.TestCase):
    def setUp(self):
        self.data = json.loads((ROOT / "synthetic_cases.json").read_text())
        self.case = self.data["cases"][0]
        self.expected = self.case["expected"]

    def test_bundled_datasets(self):
        b.validate_dataset(self.data)
        b.validate_dataset(json.loads((ROOT / "public_pilot.json").read_text()))

    def test_exact_and_duplicates(self):
        self.assertTrue(b.score(self.expected, self.expected)["whole_case_exact"])
        prediction = copy.deepcopy(self.expected)
        prediction["findings"] *= 2
        result = b.score(self.expected, prediction)
        self.assertFalse(result["whole_case_exact"])
        self.assertEqual(result["dose"]["fp"], 1)

    def test_wrong_dose_and_boundary(self):
        prediction = copy.deepcopy(self.expected)
        prediction["findings"][0]["dose_value"] = "999"
        prediction["findings"][0]["upper_operator"] = "<="
        result = b.score(self.expected, prediction)
        self.assertEqual(result["dose"]["fp"], 1)
        self.assertEqual(result["dose"]["fn"], 1)
        self.assertEqual(result["renal"]["tp"], 0)

    def test_swapped_doses_do_not_pass_whole_findings(self):
        expected = copy.deepcopy(self.expected)
        second = copy.deepcopy(expected["findings"][0])
        second.update(dose_value="100", upper_value="10")
        expected["findings"].append(second)
        predicted = copy.deepcopy(expected)
        predicted["findings"][0]["dose_value"], predicted["findings"][1]["dose_value"] = (
            predicted["findings"][1]["dose_value"], predicted["findings"][0]["dose_value"])
        result = b.score(expected, predicted)
        self.assertEqual(result["findings"]["tp"], 0)
        self.assertFalse(result["whole_case_exact"])

    def test_missed_condition(self):
        expected = copy.deepcopy(self.expected)
        expected["findings"][0]["conditions"] = ["Monitor renal function."]
        self.assertEqual(b.score(expected, self.expected)["conditions"]["fn"], 1)

    def test_invalid_output_not_successful_abstention(self):
        abstain = {"findings": [], "abstain": True, "abstention_reasons": ["No instruction"]}
        result = b.score(abstain, None)
        self.assertFalse(result["whole_case_exact"])
        self.assertEqual(result["abstention_fn"], 1)
        self.assertEqual(b.score(abstain, abstain)["abstention_tp"], 1)

    def test_strict_parsing(self):
        for raw in ['{"a":1,"a":2}', '{"a": NaN}', '```json\n{}\n```']:
            with self.assertRaises(ValueError):
                b.strict_json(raw)
        for mutate in (lambda p: p.update(extra=True), lambda p: p.update(abstain=True),
                       lambda p: p["findings"][0].update(dose_value=250),
                       lambda p: p["findings"][0].update(upper_operator="about")):
            pred = copy.deepcopy(self.expected)
            mutate(pred)
            with self.assertRaises(ValueError):
                b.validate_output(pred)

    def test_invented_quote_detected(self):
        pred = copy.deepcopy(self.expected)
        pred["findings"][0]["evidence"]["dose_value"] = "Invented quotation"
        hits, total = b.citation_counts(pred, self.case["source_text"])
        self.assertEqual(hits, total - 1)

    def test_dataset_integrity_and_leakage(self):
        for key in ("product_id", "document_id"):
            data = copy.deepcopy(self.data)
            extra = copy.deepcopy(data["cases"][0])
            extra.update(id="another", product_id="another", document_id="another", split="test")
            extra[key] = data["cases"][0][key]
            data["cases"].append(extra)
            with self.assertRaisesRegex(ValueError, "leakage"):
                b.validate_dataset(data)
        data = copy.deepcopy(self.data)
        data["cases"][0]["source_text"] += "edited"
        with self.assertRaisesRegex(ValueError, "checksum"):
            b.validate_dataset(data)
        data = copy.deepcopy(self.data)
        data["purpose"] = "reviewed_pilot"
        with self.assertRaisesRegex(ValueError, "independent"):
            b.validate_dataset(data)

    def test_labels_are_never_in_prompt(self):
        case = dict(self.case, expected="SECRET_LABEL", review="PRIVATE_REVIEW")
        payload = b.canonical(b.source_payload(case))
        self.assertNotIn("SECRET_LABEL", payload)
        self.assertNotIn("PRIVATE_REVIEW", payload)

    @patch.object(b, "environment", return_value={"fixture": True})
    def test_resume_failures_and_identity(self, _):
        data = copy.deepcopy(self.data)
        data["cases"] = data["cases"][:3]
        answers = {c["id"]: b.canonical(c["expected"]) for c in data["cases"]}
        answers[data["cases"][1]["id"]] = "broken JSON"
        answers[data["cases"][2]["id"]] = RuntimeError("DO-NOT-EXPORT-SECRET")
        adapter = FakeAdapter(answers)
        with tempfile.TemporaryDirectory() as folder:
            report = b.run(adapter, data, folder)
            self.assertEqual(report["cases"], 3)
            self.assertEqual(report["whole_case_exact_rate"], 1/3)
            self.assertEqual(adapter.calls, 3)
            self.assertEqual(b.run(adapter, data, folder), report)
            self.assertEqual(adapter.calls, 3)
            self.assertNotIn("DO-NOT-EXPORT-SECRET", (Path(folder)/"predictions.jsonl").read_text())
            data["description"] = "changed dataset"
            with self.assertRaisesRegex(ValueError, "Resume mismatch"):
                b.run(adapter, data, folder)

    @patch.object(b, "environment", return_value={"fixture": True})
    def test_truncation_is_failure_and_unlabelled_has_no_accuracy(self, _):
        data = copy.deepcopy(self.data)
        data["cases"] = data["cases"][:1]
        data["cases"][0]["expected"] = None
        adapter = FakeAdapter({})
        adapter.generate = lambda c: {"raw": b.canonical(self.expected), "truncated": True}
        with tempfile.TemporaryDirectory() as folder:
            report = b.run(adapter, data, folder)
        self.assertEqual(report["status_counts"], {"truncated": 1})
        self.assertIsNone(report["whole_case_exact_rate"])
        self.assertEqual(report["metrics"], {})

    def test_notebook_is_self_contained_and_unexecuted(self):
        notebook = json.loads((ROOT / "renalsafe_model_comparison.ipynb").read_text())
        self.assertEqual(notebook["nbformat"], 4)
        cells = notebook["cells"]
        for cell in cells:
            if cell["cell_type"] == "code":
                compile("".join(cell["source"]), "notebook", "exec")
                self.assertEqual(cell["outputs"], [])
                self.assertIsNone(cell["execution_count"])
        embedded = next(c for c in cells if c.get("id") == "bundled-files")
        with tempfile.TemporaryDirectory() as folder:
            exec("".join(embedded["source"]), {"WORK": Path(folder)})
            for name in ("benchmark.py", "synthetic_cases.json", "public_pilot.json", "requirements.txt"):
                self.assertEqual((Path(folder)/name).read_bytes(), (ROOT/name).read_bytes())


if __name__ == "__main__":
    unittest.main()
