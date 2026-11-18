# Synthetic demonstration and training

Use synthetic data only. Start the application and activate staff accounts using [accounts.md](accounts.md). The demonstration shows software behavior; independent clinical validation remains open.

## Clinician and pharmacist walkthrough

1. Open http://127.0.0.1:8080/login and sign in as a physician. If quick demo sign-in is enabled, **Demo as physician** opens **New assessment** directly. **More sign-in options** contains staff and admin sign-in.
2. Open **New assessment** (`/assess/new`) and select **Load synthetic example**. Review units, sample time, scope confirmations and the exact medicine context. For longitudinal history, create a synthetic case under **Patients**, add an observation and choose **Use this observation** first; example loading preserves that selected lab.
3. Choose **Calculate and save assessment**. The seeded candidate is PENDING, so no approved rule yields **Manual review required**, without an automated action or dose.
4. Reopen the saved result from **Saved results**. The top card shows the outcome, reasons for review and kidney estimates. Open a row under **Medicines and sources** to read section 4.2 or download the original PDF. **Advanced source search** opens a wider search for the same medicine context. **Assessment details and exports** contains reproducibility, JSON export and the printable report.
5. Sign out and sign in with a pharmacist account. Open **Rule review**, inspect the pending candidate, quotation, complete source PDF and separate review controls. Leave the real candidate pending; never approve a clinical record merely for a demonstration.
6. Open **Documents** to show declared product/page scope and source lifecycle controls. Evidence shows the actual retrieval mode and any fallback; no embedding or extraction model is assumed active.

For a saved assessment with multiple medicine contexts, **AI source review** offers **Generate AI summary**. The configured extraction provider flags possible source conflicts, applicability limits or warnings from current public medicine labels. It receives selected text from sections 4.2–4.5 and 5.2 with the product context. The model selects numbered excerpts and the application displays their original text, source and page. The summary is a separate, temporary reading aid.

Current acceptance evidence and limits are in [release-evidence.md](release-evidence.md); dated pre-React checks are in [.planning/history](../.planning/history/README.md).

## Two-drug source-grounded example

```sh
python3 scripts/demo-two-drugs.py
```

This script uses private local service keys without printing them. It verifies pinned PDF hashes, imports missing exact-context Polish sources, checks PDF downloads/section 4.2, saves six synthetic cases and replays each. Unchanged ACTIVE imports are reused; conflicting versions or existing approved rules for these demonstration contexts cause refusal. Rule records are checked for changes; no clinical rule is created or approved. Every rerun appends assessment/audit history.

Then open http://127.0.0.1:8080/assess/new?example=two-drugs and sign in as a physician. **Load two-drug example** is available when the exact contexts have been imported: Meronem 1 g and Ceftazidime Kabi 1000 mg, IV hospital-acquired pneumonia. These contexts support source inspection, not a co-prescribing recommendation or drug–drug interaction assessment.

| Scenario | Expected software behavior |
|---|---|
| Male, 70 years, 72 kg, 175 cm, creatinine 2.8 mg/dL | CrCl 25 mL/min; eGFR about 23.53527 mL/min/1.73 m² |
| Equivalent creatinine 247.52 µmol/L | Same estimates |
| Creatinine 1.0 mg/dL | CrCl 70 mL/min; eGFR about 80.96714 mL/min/1.73 m² |
| Suspected AKI | No renal estimates; `AKI_OR_UNKNOWN` |
| Missing weight | CrCl unavailable; `CG_WEIGHT_REQUIRED` |
| Creatinine bounds 2.6–3.0 mg/dL | CrCl interval about 23.33333–26.92308 mL/min |

Without applicable approved rules, both medicine results require manual review and provide no action/dose. The script supplies fresh assessment/sample timestamps; `data/demo-two-drugs.json` preserves the fixed fixture. Source queries such as Polish `nerek` retain each product's scope. Reports are written to ignored `outputs/two-drug-demo/`; pinned sources are in `data/sources/pl-2026-10-23/manifest.json`. The [original dated run](../.planning/history/two-drug-demo-2026-10-23.md) retains its actual measurements and historical execution limitations.

## Isolated decision-engine teaching mode

Open http://127.0.0.1:8080/demo.html. When [local quick sign-in](../README.md#start-the-application) is enabled, **Quick sign-in as physician** opens a prefilled synthetic assessment. **Quick sign-in as pharmacist** opens the rule extraction workspace. A configured extraction provider is required to start a live run. See the [extraction guide](online-extraction-guide.md).

Open **Try the fictional prescription check without signing in** for an isolated training case. The loaded example sets up two fictional prescriptions, one within the training limits and one outside. Select **Check prescription** to run the Go engine, or choose a different example from the dropdown. Expand **Edit patient and prescription** to customize the inputs. Changing them clears the previous result. **Technical details** shows the exact request, engine response and knowledge digest.

Products TRAINING A/B and their numerical limits are fictional, not Meronem or ceftazidime dosing guidance. APPROVED and reviewer fields in raw JSON are explicitly marked software fixture metadata, not human clinical review. GET `/demo/scenarios.json` supplies fixed examples; POST `/demo/assess` evaluates custom inputs against the isolated synthetic snapshot. It accepts only the fictional catalog, uses strict bounded JSON parsing and the same origin check as the application, and has no database or authentication dependency. Results are not saved to the clinical database or audit log. Use synthetic data only.

Verification: `TestIsolatedDemoScenarios` checks range-dependent actions, independent regimen violations and refusal reasons. `TestCustomDemoAssessment` verifies arbitrary doses/intervals, a changed creatinine selecting another rule, and rejection of real products and invalid prescriptions. Both use the HTTP handler without a database/auth object.

The result shows the calculated CrCl and eGFR followed by the outcome for each product. Each product card shows the entered prescription, the applicable fictional limits or the reason for manual review. Expand **Why this rule?** for the matched range, equation name, rule version and fixture quotation.
