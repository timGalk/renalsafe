# Clinical review backlog

No approved clinical rules. One source-grounded PENDING contraindication candidate, no dose schedule. SmPC retrieved directly from EMA on 2026-10-14, PDF SHA256 6fdebd4ffab4d65c0f88f2d9fedf86e816358c723b5b8edac8e90f1c47487499; content-addressed retrieval version, not an invented revision date. The full combined product information contains multiple strengths, adult and paediatric formulations: this candidate is restricted to the first 75 mg hard-capsule product, adult primary VTE prevention. Section 4.3, physical PDF page 6; the renal metric is explicitly Cockcroft–Gault in section 4.2. Quote occurrence and file identity checked mechanically. Full context, indication spelling/mapping, formulation scope and completeness require a pharmacist. No conclusion is made above or at 30 mL/min; no normal-dose rule inferred.

Review checklist for a human: confirm regulatory applicability in Poland and current ChPL; inspect full §§4.1–4.5; verify exact range inclusivity and age restrictions; review all competing conditions; record reviewer identity, time and content digest only after approval; construct independent expected cases. KB JSON is a trusted administrative artifact, not an approval API. Offline role strings/digests detect omissions and edits but do not authenticate identity. The implemented API enforces staff roles and binds approvals to the authenticated pharmacist and content digest; it does not provide cryptographic human signatures.

Other products named in architecture are a proposed dataset, not a source of dose rules. Their candidates remain absent until official sources and clinical validation are available.

## Calculator and approval boundaries

The decision only addresses renal criteria; it cannot establish overall prescribing safety. Byte, quote and numeric validation alone do not establish clinical validity.

The prototype policy (version `prototype-adult-stable-v1`) gates adults 18–120, creatinine 0.1–30 mg/dL, supplied weight 20–300 kg and height 100–250 cm, a sample at most seven days old, explicit stable renal function, no suspected AKI/dialysis, IDMS standardization and typical body composition. Supplied BMI below 18.5 or above 30 blocks advice. These are **engineering scope gates**, not clinical thresholds or validated laboratory freshness guidance. Missing height/weight/actual-weight confirmation prevents CG, while valid eGFR remains available. No weight imputation, creatinine floor/rounding, cap, BSA de-indexing or automatic metric substitution.

Creatinine uncertainty bounds, if supplied, use the same unit as creatinine and must be a finite closed interval containing the measured value. The derived renal interval must fit a single rule fully. Without bounds, measurement uncertainty is unknown, not zero: only the point value is matched. The calculator has ordinary estimation error and results at clinical cutoffs require independent clinical review.
