# RenalSafe demonstration

Use synthetic patient data only. The demonstration shows the implemented software workflow; independent clinical validation is still pending.

1. Open http://127.0.0.1:8080 and sign in with the local physician key from `.local/tokens.json`. Keep the key private during screen sharing.
2. Select **Load synthetic example**. Explain that patient inputs include units, sample time and explicit confirmations of the supported clinical scope. Review the selected product, then choose **Calculate and save assessment**.
3. Show the renal function estimates and **Manual review required** result. The seeded source candidate has not been clinically approved, so the engine provides no automated action or dose.
4. Select **Verify reproducibility** to recompute the result using the saved knowledge version. Select **Find source passages** to inspect the SmPC section, pages, version and quotation. Retrieval supplies evidence for review and does not change the decision.
5. Sign out and sign in with the pharmacist key. Open **Rule review**. Show the pending candidate, source quotation, full SmPC download, review notes and unchecked clinical-review confirmation. Leave the real candidate pending during the demonstration.
6. Open **Documents** to show exact product/page scope and PDF ingestion controls. Explain that local semantic retrieval is optional; the current deployment uses full-text retrieval with an explicit fallback message.

The local test suite passed 56 Go tests with race detection and real PostgreSQL integration checks. Browser smoke covers English assessment, evidence, reviewer/document screens and mobile layout. The full implementation and remaining clinical/model evaluation work are recorded in `system-verification.md` and the GSD roadmap.

## Two-drug example

Run `python3 scripts/demo-two-drugs.py` once to import the pinned Polish sources and verify the scenarios. Then open http://127.0.0.1:8080/?example=two-drugs and sign in as physician. **Load two-drug example** selects both products with a synthetic patient; **Calculate and save assessment** returns CrCl 25 mL/min and manual review for each medicine. Follow each source passage separately. No drug–drug interaction conclusion or automated dose is provided. Full checks and current browser-verification limits are in `two-drug-demo.md`.

## Isolated decision-engine teaching mode

Open http://127.0.0.1:8080/demo.html (no sign-in required). Edit synthetic patient inputs, include either or both fictional products, and enter custom single doses, units and intervals. Select **Check prescription** to execute the production Go engine. Five preset buttons fill editable inputs only; no preset result is displayed. Editing inputs clears the previous result. The result shows calculated metrics, the matched range, rule/version, action, dose/interval comparison and explicit refusal reasons. Expand the full response to inspect the exact request and knowledge digest.

Products TRAINING A/B and their numerical limits are fictional, not Meronem or ceftazidime dosing guidance. APPROVED and reviewer fields in raw JSON are explicitly marked software fixture metadata, not human clinical review. GET `/demo/scenarios.json` supplies fixed examples; POST `/demo/assess` evaluates custom inputs against the isolated synthetic snapshot. It accepts only the fictional catalog, uses strict bounded JSON parsing and the same origin check as the application, and has no database or authentication dependency. Results are not saved to the clinical database or audit log. Use synthetic data only.

Verification: `TestIsolatedDemoScenarios` checks range-dependent actions, independent regimen violations and refusal reasons. `TestCustomDemoAssessment` verifies arbitrary doses/intervals, a changed creatinine selecting another rule, and rejection of real products and invalid prescriptions. Both use the HTTP handler without a database/auth object.

The training UI uses plain-English metric cards: CrCl is marked as used by the rules, while eGFR is shown as an additional estimate. Each matched product has a table comparing the entered single dose and interval with rule limits; rule identifiers, the equation and fixture quotation are available in expandable details. Manual-review results explain missing inputs and the next step. Browser checks confirmed mixed prescription results and the missing-weight path after this UI update.
