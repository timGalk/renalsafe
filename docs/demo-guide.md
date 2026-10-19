# RenalSafe demonstration

Use synthetic patient data only. The demonstration shows the implemented software workflow; independent clinical validation is still pending.

1. Open http://127.0.0.1:8080 and sign in with the local physician key from `.local/tokens.json`. Keep the key private during screen sharing.
2. Select **Load synthetic example**. Explain that patient inputs include units, sample time and explicit confirmations of the supported clinical scope. Review the selected product, then choose **Calculate and save assessment**.
3. Show the renal function estimates and **Manual review required** result. The seeded source candidate has not been clinically approved, so the engine provides no automated action or dose.
4. Select **Verify reproducibility** to recompute the result using the saved knowledge version. Select **Find source passages** to inspect the SmPC section, pages, version and quotation. Retrieval supplies evidence for review and does not change the decision.
5. Sign out and sign in with the pharmacist key. Open **Rule review**. Show the pending candidate, source quotation, full SmPC download, review notes and unchecked clinical-review confirmation. Leave the real candidate pending during the demonstration.
6. Open **Documents** to show exact product/page scope and PDF ingestion controls. Explain that local semantic retrieval is optional; the current deployment uses full-text retrieval with an explicit fallback message.

The local test suite passed 56 Go tests with race detection and real PostgreSQL integration checks. Browser smoke covers English assessment, evidence, reviewer/document screens and mobile layout. The full implementation and remaining clinical/model evaluation work are recorded in `system-verification.md` and the GSD roadmap.
