#!/usr/bin/env python3
"""Import pinned ChPLs and verify synthetic two-drug cases against local RenalSafe.

Creates source documents and synthetic assessment/audit records. Never creates,
edits or approves a clinical rule. Reuses unchanged ACTIVE document imports.
"""
from copy import deepcopy
from datetime import datetime, timedelta, timezone
from hashlib import sha256
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import HTTPRedirectHandler, Request, build_opener
import json
import math
import secrets
import sys

ROOT = Path(__file__).resolve().parents[1]
BASE = "http://127.0.0.1:8080"


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise RuntimeError("Redirect refused; local credentials must stay local")


def run():
    keys = json.loads((ROOT / ".local/tokens.json").read_text())
    opener = build_opener(NoRedirect())

    def api(path, role=None, method="GET", body=None, content_type=None, raw=False):
        headers = {}
        if role:
            headers["Authorization"] = "Bearer " + keys[role]
        if body is not None and not isinstance(body, bytes):
            body = json.dumps(body, ensure_ascii=False).encode()
            content_type = "application/json"
        if content_type:
            headers["Content-Type"] = content_type
        try:
            with opener.open(Request(BASE + path, data=body, headers=headers, method=method), timeout=45) as response:
                payload = response.read()
                return payload if raw else json.loads(payload)
        except HTTPError as error:
            raise RuntimeError(f"{method} {path}: HTTP {error.code}: {error.read(400).decode(errors='replace')}") from None

    require(api("/healthz")["status"] == "alive", "Application is not live")
    require(api("/readyz")["status"] == "ready", "Database is not ready")
    template = json.loads((ROOT / "data/demo-two-drugs.json").read_text())
    source_root = ROOT / "data/sources/pl-2026-10-23"
    manifest = json.loads((source_root / "manifest.json").read_text())
    documents = api("/api/v1/documents", "pharmacist")
    rules_before = api("/api/v1/rules", "pharmacist")
    require(not any(x["rule"]["status"] == "APPROVED" and x["rule"]["context"] in template["medications"] for x in rules_before),
            "Real rules are already approved; this manual-review demo must not override them")
    imported = []

    for context, source in zip(template["medications"], manifest["documents"], strict=True):
        require(context["product"].startswith(source["product"]), "Source/product mapping mismatch")
        pdf = (source_root / source["pdf_file"]).read_bytes()
        require(sha256(pdf).hexdigest() == source["pdf_sha256"], "Pinned PDF hash mismatch")
        require(source["clinical_review_status"] == "NOT_REVIEWED", "Unexpected source review state")
        same_context = [d for d in documents if d["status"] == "ACTIVE" and d["context"] == context]
        matching = [d for d in same_context if d["source"]["document_sha256"] == source["pdf_sha256"]]
        require(not same_context or len(matching) == 1, "Existing document differs; explicit version review is required")
        if matching:
            document = matching[0]
        else:
            metadata = {"context": context, "url": source["url"],
                        "version": "retrieved-2026-10-23-sha256-" + source["pdf_sha256"],
                        "page_start": 1, "page_end": source["pdf_pages"]}
            boundary = "RenalSafe" + secrets.token_hex(16)
            body = (f'--{boundary}\r\nContent-Disposition: form-data; name="metadata"\r\n\r\n'.encode()
                    + json.dumps(metadata, ensure_ascii=False).encode()
                    + f'\r\n--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="source.pdf"\r\nContent-Type: application/pdf\r\n\r\n'.encode()
                    + pdf + f'\r\n--{boundary}--\r\n'.encode())
            document = api("/api/v1/documents", "pharmacist", "POST", body, "multipart/form-data; boundary=" + boundary)
            documents.append(document)
        require(document["source"]["document_sha256"] == source["pdf_sha256"], "Imported PDF identity mismatch")
        saved_pdf = api("/api/v1/documents/" + document["id"] + "/pdf", "physician", raw=True)
        require(sha256(saved_pdf).hexdigest() == source["pdf_sha256"], "Downloaded source differs from pinned PDF")
        chunks = api("/api/v1/documents/" + document["id"] + "/chunks", "physician")
        require(len(chunks) == 11, "Expected eleven source sections")
        require(any(c["section"] == "4.2" and "Zaburzenia czynności nerek" in c["text"] for c in chunks),
                "Polish renal passage missing")
        imported.append({"id": document["id"], "product": context["product"], "sections": len(chunks),
                         "pdf_sha256": source["pdf_sha256"]})

    now = datetime.now(timezone.utc)
    template["assessment_at"] = now.isoformat()
    template["patient"]["measured_at"] = (now - timedelta(hours=1)).isoformat()
    cases = {"baseline": deepcopy(template), "equivalent_units": deepcopy(template),
             "lower_creatinine": deepcopy(template), "suspected_aki": deepcopy(template),
             "missing_weight": deepcopy(template), "known_uncertainty": deepcopy(template)}
    cases["equivalent_units"]["patient"]["creatinine"] = {"value": 247.52, "unit": "umol/L"}
    cases["lower_creatinine"]["patient"]["creatinine"]["value"] = 1.0
    cases["suspected_aki"]["patient"]["suspected_aki"] = True
    cases["missing_weight"]["patient"].pop("weight")
    cases["known_uncertainty"]["patient"]["creatinine_bounds"] = {
        "lower": 2.6, "upper": 3.0, "lower_inclusive": True, "upper_inclusive": True}
    output = ROOT / "outputs/two-drug-demo"
    output.mkdir(parents=True, exist_ok=True)
    results = []

    for name, request in cases.items():
        saved = api("/api/v1/assessments", "physician", "POST", request)
        actual = saved["assessment"]
        require([r["context"] for r in actual["results"]] == template["medications"], "Two-drug result identity/order lost")
        require(all(r["status"] == "MANUAL_REVIEW_REQUIRED" and not r.get("action") and not r.get("dose_text")
                    for r in actual["results"]), "Unapproved medicine produced an automated action/dose")
        estimates = {e["metric"]: e for e in actual["renal"]["estimates"]}
        if name == "suspected_aki":
            require(not estimates and "AKI_OR_UNKNOWN" in actual["renal"]["warnings"], "AKI did not block estimates")
        elif name == "missing_weight":
            require("CRCL_CG" not in estimates and "EGFR_CKDEPI_2021" in estimates, "Missing weight handling failed")
        else:
            expected = 70.0 if name == "lower_creatinine" else 25.0
            require(math.isclose(estimates["CRCL_CG"]["value"], expected, rel_tol=1e-12), "Cockcroft–Gault reference mismatch")
        if name == "known_uncertainty":
            require(math.isclose(estimates["CRCL_CG"]["low"], 70 / 3.0, rel_tol=1e-12)
                    and math.isclose(estimates["CRCL_CG"]["high"], 70 / 2.6, rel_tol=1e-12), "Uncertainty propagation mismatch")
        fetched = api("/api/v1/assessments/" + saved["id"], "physician")
        require(fetched == saved and saved["audit_hash"], "Saved assessment changed or audit missing")
        replay = api("/api/v1/assessments/" + saved["id"] + "/replay", "physician", "POST")
        require(replay["reproduced"] is True, "Historical replay failed")
        (output / (name + "-request.json")).write_text(json.dumps(request, indent=2) + "\n")
        (output / (name + "-result.json")).write_text(json.dumps(saved, ensure_ascii=False, indent=2) + "\n")
        results.append({"case": name, "assessment_id": saved["id"], "renal": actual["renal"], "reproduced": True,
                        "drug_statuses": [r["status"] for r in actual["results"]], "audit_hash": saved["audit_hash"]})

    retrieval = []
    for context, document in zip(template["medications"], imported, strict=True):
        found = api("/api/v1/evidence/search", "physician", "POST", {
            "context": context, "document_id": document["id"], "section": "4.2", "query": "nerek", "mode": "hybrid", "limit": 5})
        require(found["status"] == "EVIDENCE_ONLY" and found["hits"] and found["audit_hash"], "Source evidence not returned/audited")
        require(all(h["chunk"]["document_id"] == document["id"] and h["source"]["document_sha256"] == document["pdf_sha256"]
                    for h in found["hits"]), "Cross-product evidence leakage")
        (output / ("evidence-" + document["id"] + ".json")).write_text(json.dumps(found, ensure_ascii=False, indent=2) + "\n")
        retrieval.append({"product": context["product"], "mode": found["mode"], "degraded_reason": found.get("degraded_reason"),
                          "hits": len(found["hits"]), "section": "4.2"})

    require(api("/api/v1/rules", "pharmacist") == rules_before, "Demo changed clinical rule state")
    audit = api("/api/v1/audit", "admin")  # Server verifies the hash chain before returning it.
    require(all(any(e["hash"] == r["audit_hash"] and e["entity_id"] == r["assessment_id"] for e in audit) for r in results),
            "Assessment audit event missing")
    summary = {"status": "passed", "synthetic_patients_only": True, "documents": imported, "cases": results,
               "retrieval": retrieval, "clinical_rules_changed": False, "audit_events_verified": len(audit)}
    (output / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"status": "passed", "documents": [d["product"] for d in imported], "cases": list(cases),
                      "baseline_crcl_ml_min": next(e["value"] for e in results[0]["renal"]["estimates"] if e["metric"] == "CRCL_CG"),
                      "clinical_rules_changed": False, "output": str(output)}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        run()
    except Exception as error:
        print("Demo failed: " + str(error), file=sys.stderr)
        sys.exit(1)
