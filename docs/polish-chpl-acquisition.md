# Polish ChPL source acquisition — 2026-10-23

Downloaded two official Polish-language ChPL PDFs from the Register of Medicinal Products (RPL). Original bytes are retained under `data/sources/pl-2026-10-23/`; `manifest.json` records official/resolved URLs, retrieval timestamps, PDF/text SHA-256, file sizes, physical page counts and verification status. The retrieval date is not asserted to be the document revision date.

| Substance requested | Selected document | RPL ID | PDF pages | Section 4.2 pages |
|---|---|---|---|---|
| Meropenem (listed as Mropenem) | Meronem, 500 mg and 1 g, powder for solution for injection or infusion | 4282 | 14 | 2–3 |
| Ceftazidime (Ceftazydym) | Ceftazidime Kabi, 1000 mg, powder for solution for injection | 24542 | 16 | 2–6 |

Official endpoints:

- https://rejestrymedyczne.ezdrowie.gov.pl/api/rpl/medicinal-products/4282/characteristic
- https://rejestrymedyczne.ezdrowie.gov.pl/api/rpl/medicinal-products/24542/characteristic

## Verification performed

Poppler UTF-8 layout extraction succeeds. The actual Go parser recognizes 11 sections in each source: 4.1–4.9, 5.1 and 5.2. Section 4.2 contains the Polish dosing heading and renal subsection, retains table layout text and exact source byte offsets, and starts on physical page 2 in each PDF. Verified PDF/text checksums and physical page counts. Rendered title pages and renal-table pages inspected: Meronem page 2; Ceftazidime Kabi page 4. Preview files are in ignored `outputs/chpl-review/`.

Reproducible check:

```sh
cd core
go test -race ./internal/evidence -count=1 -v
```

The entire evidence package passes, including `TestPolishChPLSourceExtraction` with separate real-document cases for both products. This is source-ingestion verification, not independent clinical validation or semantic reconstruction of table cells.

## Clinical and integration state

No new clinical rules have been created or activated, and no changes have been made to the running database. The existing seeded candidate remains pending. Meronem covers two strengths in one document; an exact product, route, indication and page scope must be declared for each future rule context. The automatic first-product detector is for the English EMA seed; Polish uploads use manually declared scope.

Before a two-medicine decision demonstration, choose exact contexts, prepare source-grounded PENDING rules, resolve table footnotes, renal metric, continuous versus intermittent administration, loading versus maintenance dosing, age/weight and dialysis exceptions, and obtain pharmacist review. Do not infer a dose or a continuous range across printed table gaps. Drug–drug interaction assessment is separate and is not implemented.
