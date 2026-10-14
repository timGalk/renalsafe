# Research

Local sources: Architecture revision 3 §§1–5, 8–10; thesis draft §§1.2–1.4, 2.1 and 2.5. Conflict resolution: Go replaces older Java stack; manual approval replaces old proposal auto-approval. No validated dosing dataset exists locally.

Primary reference: https://www.niddk.nih.gov/research-funding/research-programs/kidney-clinical-research-epidemiology/laboratory/glomerular-filtration-rate-equations/adults — adult CKD-EPI 2021 constants, standardized creatinine, conversion 88.4. https://www.niddk.nih.gov/research-funding/research-programs/kidney-clinical-research-epidemiology/laboratory/ckd-drug-dosing-providers — body size, creatinine limitations, critical cutoffs. CG formula also explicit in local architecture and thesis; verify against official primary SmPC if retrieved.

Risk controls: float comparisons use unrounded values; no epsilon tolerance converts a threshold. A value immediately below the boundary stays below it. Numeric uncertainty spanning multiple actions blocks advice. Never interpret absence of a rule as normal dosing. Source hashes prove byte identity, not clinical validation; approval metadata records a human decision, cannot authenticate a human by itself. Audit chain needs external anchoring and RBAC in Phase 3 to resist a privileged rewrite.
