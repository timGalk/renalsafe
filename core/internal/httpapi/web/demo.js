'use strict';

const reasons = {
  SINGLE_DOSE_EXCEEDS_SOURCE_LIMIT: 'Single dose exceeds the limit',
  INTERVAL_SHORTER_THAN_SOURCE_LIMIT: 'Dosing interval is shorter than the minimum',
  CG_WEIGHT_REQUIRED: 'Weight is required to calculate CrCl',
  CG_HEIGHT_REQUIRED_FOR_BODY_SIZE_CHECK: 'Height is required for the body size check',
  CG_ACTUAL_WEIGHT_UNCONFIRMED: 'Actual body weight is not confirmed',
  REQUIRED_RENAL_METRIC_UNAVAILABLE: 'Required renal metric CrCl is unavailable',
  UNCERTAINTY_CROSSES_RULE_BOUNDARY: 'Uncertainty interval crosses the rule boundary',
  REGIMEN_UNIT_MISMATCH: 'Entered dose units differ from rule units; manual review required',
  AKI_OR_UNKNOWN: 'Suspected AKI or its absence is not confirmed',
  DIALYSIS_OR_UNKNOWN: 'Dialysis or its absence is not confirmed',
  STABILITY_UNCONFIRMED: 'Stable renal function is not confirmed',
  CREATININE_TIMESTAMP_INVALID_OR_STALE: 'Sample time is invalid or older than the prototype policy allows',
  BODY_SIZE_REQUIRES_REVIEW: 'Body size is outside the supported prototype scope',
  CREATININE_BOUNDS_INVALID: 'Creatinine bounds are invalid or do not contain the measured value',
  AGE_OUT_OF_SCOPE: 'Age is outside the supported prototype range (18–120 years).',
  EQUATION_SEX_REQUIRED: 'Select the sex used by the equation.',
  CREATININE_STANDARDIZATION_UNCONFIRMED: 'Confirm that the creatinine test is IDMS-standardized.',
  BODY_COMPOSITION_UNCERTAIN: 'Typical body composition has not been confirmed.',
  CREATININE_UNIT_UNSUPPORTED: 'The creatinine unit is not supported.',
  CREATININE_OUT_OF_POLICY: 'Creatinine is outside the supported prototype range.',
  CREATININE_BOUNDS_OUT_OF_POLICY: 'Creatinine bounds are outside the supported prototype range.',
  WEIGHT_OUT_OF_POLICY_OR_UNIT: 'Weight or its units are outside the supported prototype scope.',
  HEIGHT_OUT_OF_POLICY_OR_UNIT: 'Height or its units are outside the supported prototype scope.',
  NO_UNIQUE_RANGE_MATCH: 'No single rule covers this kidney estimate.',
  APPROVAL_NOT_YET_EFFECTIVE: 'The assessment time predates the training rule.',
  NO_APPROVED_RULE: 'No applicable rule is available.',
};
const statuses = {
  WITHIN_RULE_LIMITS: 'Within training limits',
  OUTSIDE_RULE_LIMITS: 'Outside training limits',
  MANUAL_REVIEW_REQUIRED: 'Manual review required',
};
const actions = {
  REDUCE_DOSE: 'Reduced-dose rule applies',
  NO_CHANGE: 'No adjustment under this rule',
  EXTEND_INTERVAL: 'Extend the dosing interval',
  AVOID: 'Avoid use',
  USE_WITH_CAUTION: 'Use with caution',
};
const scenarioNames = {
  outside: 'One prescription exceeds limits',
  within: 'Both prescriptions within limits',
  'higher-function': 'Higher kidney function',
  'missing-weight': 'Missing patient weight',
  uncertainty: 'Uncertain creatinine',
};
const scopeChecks = {
  stable: 'stable_renal_function',
  'no-aki': 'suspected_aki',
  'no-dialysis': 'dialysis',
  idms: 'idms_standardized',
  body: 'typical_body_composition',
  'actual-weight': 'actual_weight_confirmed',
};

const el = id => document.getElementById(id);
function make(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}

const quickLogin = el('quick-login');
const loginStatus = el('quick-login-status');
const trainingCheck = el('prescription-check');
const demoDestinations = {
  physician: '/assess/new?example=synthetic',
  pharmacist: '/review/extractions',
};

quickLogin.addEventListener('submit', async event => {
  event.preventDefault();
  const role = event.submitter?.value;
  if (!Object.hasOwn(demoDestinations, role)) return;
  const buttons = quickLogin.querySelectorAll('button');
  buttons.forEach(button => { button.disabled = true; });
  loginStatus.textContent = 'Opening the demo workspace…';
  loginStatus.hidden = false;
  try {
    const response = await fetch('/api/v1/auth/demo', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({role}),
    });
    if (!response.ok) throw new Error('Demo sign-in failed');
    if (typeof BroadcastChannel !== 'undefined') {
      const channel = new BroadcastChannel('renalsafe-session');
      channel.postMessage('changed');
      channel.close();
    }
    location.assign(demoDestinations[role]);
  } catch {
    loginStatus.textContent = 'Could not open the demo workspace. Try again or use the fictional check below.';
  } finally {
    buttons.forEach(button => { button.disabled = false; });
  }
});

async function loadDemoLogin() {
  try {
    const response = await fetch('/api/v1/auth/demo');
    if (response.ok && (await response.json()).enabled === true) {
      quickLogin.hidden = false;
      return;
    }
  } catch {
    // The fictional check is still available without account access.
  }
  loginStatus.textContent = 'Demo sign-in is unavailable on this server. Try the fictional check below.';
  loginStatus.hidden = false;
  trainingCheck.open = true;
}

if (location.hash === '#prescription-check') trainingCheck.open = true;
void loadDemoLogin();

const form = el('demo-form');
const scenario = el('scenario');
const editInputs = el('edit-inputs');
const status = el('demo-status');
const result = el('demo-result');
const technicalDetails = el('technical-details');
const evaluate = el('evaluate');
let catalog = [];
let revision = 0;

function showStatus(message) {
  status.textContent = message;
  status.hidden = !message;
}

function clearResult() {
  revision++;
  result.hidden = true;
  technicalDetails.hidden = true;
  technicalDetails.open = false;
  el('demo-json').textContent = '';
}

function dateValue(date) {
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}

function productName(context) {
  return context.product.split(' — ')[0];
}

function doseSummary(dose, unit, interval) {
  return dose && interval ? `${dose} ${unit} every ${interval} h` : 'Enter dose and interval';
}

function updatePreview() {
  const weight = el('weight').value;
  const creatinine = el('creatinine').value;
  const unit = el('unit').selectedOptions[0].textContent;
  el('patient-preview').textContent = [
    el('age').value ? `Age ${el('age').value}` : 'Age missing',
    weight ? `${weight} kg` : 'Weight missing',
    creatinine ? `Creatinine ${creatinine} ${unit}` : 'Creatinine missing',
  ].join(' · ');
  const prescriptions = catalog.flatMap((context, index) => {
    const included = el(`include-${index}`).checked;
    el(`prescription-fields-${index}`).disabled = !included;
    if (!included) return [];
    const regimen = doseSummary(el(`dose-${index}`).value, el(`dose-unit-${index}`).value, el(`interval-${index}`).value);
    return [make('li', `${productName(context)} · ${regimen}`)];
  });
  el('prescription-preview').replaceChildren(...prescriptions);
}

function fill(request) {
  const patient = request.patient;
  const fields = {
    age: patient.age_years,
    sex: patient.equation_sex,
    weight: patient.weight?.value ?? '',
    height: patient.height?.value ?? '',
    creatinine: patient.creatinine.value,
    unit: patient.creatinine.unit,
    measured: dateValue(new Date(Date.now() - 3600000)),
    lower: patient.creatinine_bounds?.lower ?? '',
    upper: patient.creatinine_bounds?.upper ?? '',
  };
  for (const [id, value] of Object.entries(fields)) el(id).value = value;
  for (const [id, key] of Object.entries(scopeChecks)) {
    el(id).checked = patient[key] === !id.startsWith('no-');
  }
  catalog.forEach((_, index) => {
    const prescription = request.prescriptions.find(item => item.medication_index === index);
    el(`include-${index}`).checked = Boolean(prescription);
    el(`dose-${index}`).value = prescription?.dose.value ?? 10;
    el(`interval-${index}`).value = prescription?.interval_hours ?? 24;
    el(`dose-unit-${index}`).value = prescription?.dose.unit ?? 'mg';
  });
  clearResult();
  showStatus('');
  updatePreview();
}

function number(id) {
  const input = el(id);
  if (!Number.isFinite(input.valueAsNumber)) {
    throw new Error(`Enter a valid number for ${input.labels[0].textContent.toLowerCase()}.`);
  }
  return input.valueAsNumber;
}

function buildRequest() {
  const patient = {
    age_years: number('age'),
    equation_sex: el('sex').value,
    creatinine: {value: number('creatinine'), unit: el('unit').value},
    measured_at: new Date(el('measured').value).toISOString(),
  };
  if (el('weight').value !== '') patient.weight = {value: number('weight'), unit: 'kg'};
  if (el('height').value !== '') patient.height = {value: number('height'), unit: 'cm'};
  for (const [id, key] of Object.entries(scopeChecks)) {
    patient[key] = id.startsWith('no-') !== el(id).checked;
  }
  const lower = el('lower').value;
  const upper = el('upper').value;
  if (Boolean(lower) !== Boolean(upper)) throw new Error('Enter both creatinine bounds or leave both blank.');
  if (lower) {
    patient.creatinine_bounds = {
      lower: number('lower'), upper: number('upper'), lower_inclusive: true, upper_inclusive: true,
    };
  }

  const medications = [];
  const prescriptions = [];
  catalog.forEach((context, index) => {
    if (!el(`include-${index}`).checked) return;
    prescriptions.push({
      medication_index: medications.length,
      dose: {value: number(`dose-${index}`), unit: el(`dose-unit-${index}`).value},
      interval_hours: number(`interval-${index}`),
    });
    medications.push(context);
  });
  if (!medications.length) throw new Error('Include at least one fictional product.');
  return {assessment_at: new Date().toISOString(), patient, medications, prescriptions};
}

const format = value => Number(value.toFixed(2)).toString();
function rangeText(range) {
  if (range.lower === 0 && range.upper !== undefined && !range.upper_inclusive) return `below ${range.upper} mL/min`;
  if (range.upper === undefined) return `${range.lower} mL/min or above`;
  return `${range.lower}–${range.upper} mL/min`;
}

function renderMetrics(assessment) {
  const metrics = make('div', undefined, 'metrics');
  for (const [key, title, units] of [
    ['CRCL_CG', 'CrCl · used by the rules', 'mL/min'],
    ['EGFR_CKDEPI_2021', 'eGFR', 'mL/min/1.73 m²'],
  ]) {
    const estimate = assessment.renal.estimates.find(item => item.metric === key);
    const metric = make('p', undefined, 'metric');
    const value = make('strong', estimate ? `${format(estimate.value)} ` : 'Unavailable');
    if (estimate) value.append(make('span', units, 'metric-unit'));
    metric.append(make('span', title), value);
    if (estimate && assessment.request.patient.creatinine_bounds) {
      metric.append(make('span', `Estimated range ${format(estimate.low)}–${format(estimate.high)} ${units}`));
    }
    metrics.append(metric);
  }
  return metrics;
}

function reasonList(codes) {
  const list = make('ul', undefined, 'outcome-reasons');
  for (const code of codes) list.append(make('li', reasons[code] || code));
  return list;
}

function renderDecision(decision, prescription) {
  const panel = make('article', undefined, 'panel');
  const check = decision.prescription_check;
  const outcome = check?.status || 'MANUAL_REVIEW_REQUIRED';
  panel.append(
    make('h3', productName(decision.context)),
    make('p', statuses[outcome] || statuses.MANUAL_REVIEW_REQUIRED,
      outcome === 'WITHIN_RULE_LIMITS' ? 'result-status' : 'result-status attention'),
    make('p', `Entered: ${doseSummary(String(prescription.dose.value), prescription.dose.unit, String(prescription.interval_hours))}`),
  );
  const codes = [...new Set([...decision.reasons, ...(check?.reasons || [])])];
  if (codes.length) panel.append(reasonList(codes));
  if (!decision.rule) return panel;

  const {rule, selected} = decision;
  const limit = rule.regimen_limit;
  if (limit) {
    const {value, unit} = limit.max_single_dose;
    panel.append(make('p', `Training limit: up to ${value} ${unit} per dose, at least ${limit.min_interval_hours} h apart.`));
  }
  const details = make('details');
  details.append(
    make('summary', 'Why this rule?'),
    make('p', `${actions[decision.action] || decision.action}. Cockcroft–Gault CrCl is ${format(selected.value)} mL/min. The matched range is ${rangeText(rule.range)}.`),
    make('p', `Rule ${rule.id} · ${rule.version}`, 'muted'),
    make('blockquote', rule.citation.quote),
  );
  panel.append(details);
  return panel;
}

function render(assessment) {
  const heading = make('h2', 'Check result');
  heading.id = 'result-title';
  result.replaceChildren(heading, renderMetrics(assessment));
  const shownReasons = new Set(assessment.results.flatMap(item => item.reasons));
  const otherWarnings = assessment.renal.warnings.filter(code => !shownReasons.has(code));
  if (otherWarnings.length) result.append(reasonList(otherWarnings));
  for (const [index, decision] of assessment.results.entries()) {
    const prescription = assessment.request.prescriptions.find(item => item.medication_index === index);
    result.append(renderDecision(decision, prescription));
  }
  el('demo-json').textContent = JSON.stringify(assessment, null, 2);
  result.hidden = false;
  technicalDetails.hidden = false;
}

function prescriptionFields(context, index) {
  const panel = make('article', undefined, 'medication');
  const label = make('label', `Include ${productName(context)}`);
  const include = make('input');
  include.type = 'checkbox';
  include.id = `include-${index}`;
  label.prepend(include);
  const grid = make('fieldset', undefined, 'grid');
  grid.id = `prescription-fields-${index}`;
  grid.setAttribute('aria-label', `${productName(context)} regimen`);
  for (const [key, title] of [['dose', 'Amount per dose'], ['interval', 'Hours between doses']]) {
    const field = make('label', title);
    const input = make('input');
    input.id = `${key}-${index}`;
    input.type = 'number';
    input.step = 'any';
    input.min = '0';
    field.append(input);
    grid.append(field);
  }
  const units = make('label', 'Dose units');
  const select = make('select');
  select.id = `dose-unit-${index}`;
  for (const unit of ['mg', 'g']) select.append(new Option(unit));
  units.append(select);
  grid.append(units);
  panel.append(label, grid);
  return panel;
}

editInputs.addEventListener('input', () => {
  const hadResult = !result.hidden || evaluate.disabled;
  scenario.value = '';
  clearResult();
  showStatus(hadResult ? 'Inputs changed. Run the check again.' : '');
  updatePreview();
});
form.addEventListener('invalid', event => {
  for (let detail = event.target.closest('details'); detail; detail = detail.parentElement.closest('details')) {
    detail.open = true;
  }
}, true);
form.addEventListener('submit', async event => {
  event.preventDefault();
  clearResult();
  const current = revision;
  evaluate.disabled = true;
  showStatus('Checking prescription…');
  try {
    const response = await fetch('/demo/assess', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      credentials: 'omit',
      body: JSON.stringify(buildRequest()),
    });
    if (!response.ok) throw new Error('Unable to check the prescription. Review your inputs and try again.');
    const assessment = await response.json();
    if (current !== revision) return;
    render(assessment);
    showStatus('');
    result.focus({preventScroll: true});
    result.scrollIntoView({block: 'start'});
  } catch (error) {
    if (current === revision) showStatus(error.message);
  } finally {
    evaluate.disabled = false;
  }
});

async function loadScenarios() {
  const response = await fetch('/demo/scenarios.json', {credentials: 'omit'});
  if (!response.ok) throw new Error('Could not load training examples. Refresh the page to try again.');
  const data = await response.json();
  catalog = data.scenarios[0].assessment.request.medications;
  el('demo-notice').textContent = data.notice;
  el('prescriptions').replaceChildren(...catalog.map(prescriptionFields));
  const custom = new Option('Custom inputs', '');
  custom.disabled = true;
  scenario.append(custom);
  for (const item of data.scenarios) scenario.append(new Option(scenarioNames[item.id] || item.title, item.id));
  scenario.value = 'outside';
  const loadSelection = () => fill(data.scenarios.find(item => item.id === scenario.value).assessment.request);
  loadSelection();
  scenario.addEventListener('change', loadSelection);
  scenario.disabled = false;
  evaluate.disabled = false;
  editInputs.hidden = false;
  el('patient-preview').hidden = false;
  el('prescription-preview').hidden = false;
}

loadScenarios().catch(() => showStatus('Could not load training examples. Refresh the page to try again.'));
