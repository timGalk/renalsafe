# Online document extraction and rule drafting

Decision: 2026-11-09. The user selected online models accessed through an API key,
then selected OpenAI **gpt-5.4-mini**. This supersedes the earlier local-only
inference decision. The first integration is implemented; see the
[configuration and workflow guide](online-extraction-guide.md).

## Current gap

The application now has durable extraction jobs, an OpenAI Responses adapter,
validated source-backed suggestions, browser review and explicit PENDING saves.
The ChPL corpus, notebooks and synthetic cases remain preparation/verification
material rather than independent clinical gold labels. Real-provider capability
and clinical accuracy still require evaluation.

## Implemented flow

1. A pharmacist or administrator selects an ACTIVE, versioned document and exact
   product context, then requests an extraction job.
2. A server-side worker calls the configured online model using an API key from
   deployment secret configuration. Keys never enter browser bundles, source
   control, prompts, model output, audit payloads or logs. Send only the selected
   public document content and product context, without patient records.
3. The model returns structured findings with field-level source evidence,
   renal metrics/units, boundaries, dose wording, population/route/indication,
   dependencies, unresolved conditions and abstentions. Preserve loading doses,
   relative doses, dialysis timing, daily totals and monitoring qualifiers.
4. The application validates the schema, pinned source identity, evidence spans
   and numeric grounding. Unsupported fields and missing context require review.
   Tables, headings and footnotes stay together; content is never silently cropped.
5. Store suggestions separately from executable rules. Show the original PDF,
   quotations and proposed fields for correction or rejection. Convert supported
   findings into PENDING candidates only through the existing candidate validator.
6. Pharmacist approval remains explicit and separate. A model cannot approve a
   rule or alter an assessment. Conditions the executable schema cannot represent
   stay unresolved and prevent approval.

## Original implementation checklist

The checklist below records the design scope. The first text-based integration
implements provider configuration, jobs, suggestion validation, review and
pending-only conversion. Independent labels, real-model evaluation and harder
PDF handling remain outstanding. See the guide for exact limits.

- Select the provider/model and record the API/model version and prompt/schema
  hashes for every run. Define endpoint configuration and server-only key handling.
- Implement durable job status, bounded requests, timeouts, cancellation, rate
  limits, retry policy and duplicate-save protection. Provider errors must not
  disable manual review or historical replay.
- Add the suggestion schema, source verification and supported conversion to the
  existing rule contract; preserve non-executable findings for human review.
- Add document extraction controls, job progress and side-by-side suggestion review.
- Test with a fake provider before real API calls: malformed/truncated responses,
  invented quotations, prompt injection in documents, stale sources, unsupported
  units/conditions, retries and unauthorized approval.
- Evaluate real extraction against independently reviewed ChPL answers. Keep
  development and held-out groups separate and measure missed qualifiers,
  unsupported additions and human correction time. Synthetic patient cases verify
  software behavior; they do not establish extraction accuracy.

The existing [extraction design](ai-rule-extraction-plan.md) retains detailed
evidence and review requirements. Its local model research is historical context,
not the selected deployment direction. No API key is required merely to run the
current deterministic application or offline tests; online extraction requires one.
