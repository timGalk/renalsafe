# Calculator references

Checked 2026-10-14 against the architecture revision 3 and official primary sources:

- NIDDK adult equations: https://www.niddk.nih.gov/research-funding/research-programs/kidney-clinical-research-epidemiology/laboratory/glomerular-filtration-rate-equations/adults — CKD-EPI 2021 creatinine formula, age at least 18, IDMS creatinine, indexed units, 88.4 conversion.
- NIDDK dosing considerations: https://www.niddk.nih.gov/research-funding/research-programs/kidney-clinical-research-epidemiology/laboratory/ckd-drug-dosing-providers — estimation limitations, atypical body sizes and uncertainty near dosage cutoffs. Combined creatinine/cystatin C calculation is deferred; no cystatin C input invented.
- National Kidney Foundation formula: https://www.kidney.org/ckd-epi-creatinine-equation-2021 — corroborates the CKD-EPI constants.
- Cockcroft–Gault original article: Cockcroft DW, Gault MH, Prediction of creatinine clearance from serum creatinine, Nephron 1976;16:31–41; DOI 10.1159/000180580, cited in the local thesis §2.1. Formula explicit in local thesis and architecture; official EMA product source mandates CG for adult Pradaxa. Weight interpretation and body-size exceptions remain prototype limitations.

CG mL/min = (140 − age) × actual weight kg / (72 × creatinine mg/dL), multiplied by 0.85 for female equation sex.
CKD-EPI indexed mL/min/1.73m2 = 142 × min(SCr/k,1)^alpha × max(SCr/k,1)^(-1.200) × 0.9938^age × (1.012 for female). k=0.7/0.9, alpha=-0.241/-0.302 for female/male. No race parameter.

Unit names are intentionally canonical API tokens (`umol/L`, not an implicit locale-dependent symbol). Caller must supply units and equation sex explicitly. These equations do not establish a reliable estimate in AKI, dialysis, atypical muscle mass or unsupported populations.
