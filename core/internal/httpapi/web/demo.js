'use strict';
const el=id=>document.getElementById(id);
const make=(tag,text,cls)=>{const n=document.createElement(tag);if(text!==undefined)n.textContent=text;if(cls)n.className=cls;return n;};
const labels={RULE_MATCHED:'Rule matched',WITHIN_RULE_LIMITS:'Regimen is within the training rule limits',OUTSIDE_RULE_LIMITS:'Regimen is outside the training rule limits',MANUAL_REVIEW_REQUIRED:'Manual review required',SINGLE_DOSE_EXCEEDS_SOURCE_LIMIT:'Single dose exceeds the limit',INTERVAL_SHORTER_THAN_SOURCE_LIMIT:'Dosing interval is shorter than the minimum',CG_WEIGHT_REQUIRED:'Weight is required to calculate CrCl',CG_HEIGHT_REQUIRED_FOR_BODY_SIZE_CHECK:'Height is required for the body size check',CG_ACTUAL_WEIGHT_UNCONFIRMED:'Actual body weight is not confirmed',REQUIRED_RENAL_METRIC_UNAVAILABLE:'Required renal metric CrCl is unavailable',UNCERTAINTY_CROSSES_RULE_BOUNDARY:'Uncertainty interval crosses the rule boundary',REGIMEN_UNIT_MISMATCH:'Entered dose units differ from rule units; manual review required',AKI_OR_UNKNOWN:'Suspected AKI or its absence is not confirmed',DIALYSIS_OR_UNKNOWN:'Dialysis or its absence is not confirmed',STABILITY_UNCONFIRMED:'Stable renal function is not confirmed',CREATININE_TIMESTAMP_INVALID_OR_STALE:'Sample time is invalid or older than the prototype policy allows',BODY_SIZE_REQUIRES_REVIEW:'Body size is outside the supported prototype scope',CREATININE_BOUNDS_INVALID:'Creatinine bounds are invalid or do not contain the measured value'};
Object.assign(labels,{AGE_OUT_OF_SCOPE:'Age is outside the supported prototype range (18–120 years).',EQUATION_SEX_REQUIRED:'Select the sex used by the equation.',CREATININE_STANDARDIZATION_UNCONFIRMED:'Confirm that the creatinine test is IDMS-standardized.',BODY_COMPOSITION_UNCERTAIN:'Typical body composition has not been confirmed.',CREATININE_UNIT_UNSUPPORTED:'The creatinine unit is not supported.',CREATININE_OUT_OF_POLICY:'Creatinine is outside the supported prototype range.',CREATININE_BOUNDS_OUT_OF_POLICY:'Creatinine bounds are outside the supported prototype range.',WEIGHT_OUT_OF_POLICY_OR_UNIT:'Weight or its units are outside the supported prototype scope.',HEIGHT_OUT_OF_POLICY_OR_UNIT:'Height or its units are outside the supported prototype scope.',NO_UNIQUE_RANGE_MATCH:'No single rule covers this kidney estimate.',APPROVAL_NOT_YET_EFFECTIVE:'The assessment time predates the training rule.',NO_APPROVED_RULE:'No applicable rule is available.'});
let catalog=[],revision=0;
const checks={'stable':'stable_renal_function','no-aki':'suspected_aki','no-dialysis':'dialysis','idms':'idms_standardized','body':'typical_body_composition','actual-weight':'actual_weight_confirmed'};
function invalidate(){revision++;el('demo-result').replaceChildren(make('p','Inputs changed. Run a new check to see the result.','empty-result'));el('demo-json').textContent='';el('demo-status').textContent='Inputs changed. Select Check prescription to calculate a new result.';}
function dateValue(d){return new Date(d.getTime()-d.getTimezoneOffset()*60000).toISOString().slice(0,16);}
function fill(req){
 const p=req.patient;
 for(const [id,value] of Object.entries({age:p.age_years,sex:p.equation_sex,weight:p.weight?.value??'',height:p.height?.value??'',creatinine:p.creatinine.value,unit:p.creatinine.unit,measured:dateValue(new Date(Date.now()-3600000)),lower:p.creatinine_bounds?.lower??'',upper:p.creatinine_bounds?.upper??''}))el(id).value=value;
 for(const [id,key] of Object.entries(checks))el(id).checked=id.startsWith('no-')?p[key]===false:p[key]===true;
 catalog.forEach((c,i)=>{const rx=req.prescriptions.find(x=>x.medication_index===i);el('include-'+i).checked=!!rx;el('dose-'+i).value=rx?.dose.value??10;el('interval-'+i).value=rx?.interval_hours??24;el('dose-unit-'+i).value=rx?.dose.unit??'mg';});invalidate();
}
function number(id){const n=el(id).valueAsNumber;if(!Number.isFinite(n))throw new Error('Enter a valid number for '+el(id).parentElement.firstChild.textContent);return n;}
function request(){
 const p={age_years:number('age'),equation_sex:el('sex').value,creatinine:{value:number('creatinine'),unit:el('unit').value},measured_at:new Date(el('measured').value).toISOString()};
 for(const id of ['weight','height'])if(el(id).value!=='')p[id]={value:number(id),unit:id==='weight'?'kg':'cm'};
 for(const [id,key] of Object.entries(checks))p[key]=id.startsWith('no-')?!el(id).checked:el(id).checked;
 if(el('lower').value!==''||el('upper').value!==''){if(el('lower').value===''||el('upper').value==='')throw new Error('Enter both creatinine bounds or leave both blank.');p.creatinine_bounds={lower:number('lower'),upper:number('upper'),lower_inclusive:true,upper_inclusive:true};}
 const medications=[],prescriptions=[];
 catalog.forEach((context,i)=>{if(el('include-'+i).checked){prescriptions.push({medication_index:medications.length,dose:{value:number('dose-'+i),unit:el('dose-unit-'+i).value},interval_hours:number('interval-'+i)});medications.push(context);}});
 if(!medications.length)throw new Error('Include at least one fictional product.');
 return {assessment_at:new Date().toISOString(),patient:p,medications,prescriptions};
}
const actions={REDUCE_DOSE:'A reduced-dose regimen applies',NO_CHANGE:'No adjustment under this rule',EXTEND_INTERVAL:'Extend the dosing interval',AVOID:'Avoid use',USE_WITH_CAUTION:'Use with caution'};
const format=n=>Number(n.toFixed(2)).toString();
function rangeText(r){if(r.lower===0&&r.upper!==undefined&&!r.upper_inclusive)return `below ${r.upper} mL/min`;if(r.upper===undefined)return `${r.lower} mL/min or above`;return `${r.lower}–${r.upper} mL/min`;}
function render(a){
 const root=el('demo-result');root.replaceChildren(make('h2','3. Your result'));
 const checks=a.results.map(r=>r.prescription_check?.status||'MANUAL_REVIEW_REQUIRED');
 const within=checks.filter(x=>x==='WITHIN_RULE_LIMITS').length,outside=checks.filter(x=>x==='OUTSIDE_RULE_LIMITS').length,manual=checks.length-within-outside;
 root.append(make('p',`${within} within rule limits · ${outside} outside rule limits · ${manual} need manual review`,'result-summary'));
 const estimates=a.renal.estimates,metrics=make('div',undefined,'metric-grid');
 for(const key of ['CRCL_CG','EGFR_CKDEPI_2021']){
  const e=estimates.find(x=>x.metric===key),isCrCl=key==='CRCL_CG',card=make('div',undefined,'metric-card'+(isCrCl?' used':''));
  card.append(make('span',isCrCl?'Used for these rules':'Additional estimate','badge'),make('h3',isCrCl?'Creatinine clearance (CrCl)':'Estimated filtration rate (eGFR)'),make('p',isCrCl?'Cockcroft–Gault equation':'CKD-EPI 2021 equation','meta'));
  if(e){const value=make('p',format(e.value)+' ','metric-number');value.append(make('span',isCrCl?'mL/min':'mL/min/1.73 m²'));card.append(value);if(a.request.patient.creatinine_bounds)card.append(make('p',`Range from entered uncertainty: ${format(e.low)}–${format(e.high)} ${isCrCl?'mL/min':'mL/min/1.73 m²'}.`));}
  else card.append(make('p','Not available','manual'));
  card.append(make('p',isCrCl?'The training rules use this estimate to choose a dose limit and interval.':'Adjusted to a standard body surface area of 1.73 m². Shown for context; these training rules use CrCl.','meta'));metrics.append(card);
 }
 root.append(metrics,make('p','These are estimates of kidney function, not percentages. Different equations can give different values.','meta'));
 for(const w of a.renal.warnings)root.append(make('p',labels[w]||w,'manual'));
 for(const [i,r] of a.results.entries()){
  const card=make('article',undefined,'decision-card'),rx=a.request.prescriptions.find(p=>p.medication_index===i),c=r.prescription_check;
  card.append(make('h3',r.context.product),make('p',`Your prescription: ${rx.dose.value} ${rx.dose.unit} every ${rx.interval_hours} hours.`));
  if(r.rule){const e=r.selected,cap=r.rule.regimen_limit,info=make('div',undefined,'rule-info');
   info.append(make('p',`Why this rule? CrCl is ${format(e.value)} mL/min, which is ${rangeText(r.rule.range)}.`,'why'),make('p',`Matched kidney rule: ${actions[r.action]||r.action}.`));card.append(info);
   if(cap){
    const wrap=make('div',undefined,'table-wrap'),table=make('table');table.append(make('caption','Entered prescription compared with the matched training rule'));
    const head=make('thead'),hr=make('tr');for(const t of ['Check','Your input','Rule limit','Result'])hr.append(make('th',t));head.append(hr);table.append(head);const body=make('tbody');
    const sameUnit=rx.dose.unit===cap.max_single_dose.unit;
    for(const cells of [
     ['Single dose',`${rx.dose.value} ${rx.dose.unit}`,`At most ${cap.max_single_dose.value} ${cap.max_single_dose.unit}`,sameUnit?(rx.dose.value<=cap.max_single_dose.value?'Within limit':'Too high'):'Different units — manual review'],
     ['Time between doses',`${rx.interval_hours} hours`,`At least ${cap.min_interval_hours} hours`,rx.interval_hours>=cap.min_interval_hours?'Within limit':'Too short']
    ]){const tr=make('tr');for(const value of cells)tr.append(make('td',value));body.append(tr);}table.append(body);wrap.append(table);card.append(wrap);
   }
   if(c){const title=c.status==='WITHIN_RULE_LIMITS'?'Within the checked kidney rule limits':c.status==='OUTSIDE_RULE_LIMITS'?'Prescription needs review: rule limits exceeded':'Cannot complete the prescription check';card.append(make('p',title,c.status==='WITHIN_RULE_LIMITS'?'matched':'manual'));for(const reason of c.reasons)card.append(make('p',labels[reason]||reason));}
   card.append(make('p','This checks the entered prescription against fictional limits. It does not establish overall prescribing safety.','meta'));
   const details=make('details'),summary=make('summary','Show the rule and calculation');details.append(summary,make('p',`Rule ID: ${r.rule.id}. Version: ${r.rule.version}.`,'meta'));
   const p=a.request.patient;if(e.metric==='CRCL_CG'){const factor=p.equation_sex==='female'?' × 0.85':'';const scr=p.creatinine.unit==='umol/L'?p.creatinine.value/88.4:p.creatinine.value;details.append(make('p',`CrCl = (140 − ${p.age_years}) × ${p.weight.value} / (72 × ${format(scr)})${factor} = ${format(e.value)} mL/min. Creatinine is expressed in mg/dL for this equation.`));}
   details.append(make('blockquote',r.rule.citation.quote));card.append(details);
  }else{
   card.append(make('p','Cannot complete the check — manual review required','manual'));
   for(const reason of r.reasons)card.append(make('p',labels[reason]||reason));
   let next='Review the patient inputs and applicability checks, then run the check again.';
   if(r.reasons.includes('CG_WEIGHT_REQUIRED'))next='Next step: enter the synthetic patient’s weight in kg, then check again. eGFR cannot replace CrCl in these rules.';
   if(r.reasons.includes('CG_HEIGHT_REQUIRED_FOR_BODY_SIZE_CHECK'))next='Next step: enter height in cm so the prototype can check body size.';
   if(r.reasons.includes('UNCERTAINTY_CROSSES_RULE_BOUNDARY'))next='Why blocked? The plausible CrCl range spans both sides of 30 mL/min. A single rule cannot cover the full range; independent review is required.';
   card.append(make('p',next),make('p','No action or dose has been recommended.','meta'));
  }
  root.append(card);
 }
 el('demo-json').textContent=JSON.stringify(a,null,2);
}
el('demo-form').addEventListener('input',invalidate);
el('demo-form').onsubmit=async event=>{event.preventDefault();const current=revision;el('evaluate').disabled=true;el('demo-result').replaceChildren();el('demo-json').textContent='';el('demo-status').textContent='Evaluating…';try{const response=await fetch('/demo/assess',{method:'POST',headers:{'Content-Type':'application/json'},credentials:'omit',body:JSON.stringify(request())});const out=await response.json();if(!response.ok)throw new Error(out.error||'Evaluation failed');if(current!==revision)return;render(out);el('demo-result').scrollIntoView({behavior:'smooth',block:'start'});el('demo-status').textContent='Evaluation complete. Results correspond to the inputs above.';}catch(e){if(current===revision)el('demo-status').textContent=e.message;}finally{el('evaluate').disabled=false;}};
(async()=>{
 const response=await fetch('/demo/scenarios.json',{credentials:'omit'});if(!response.ok)throw new Error('Unable to load training examples.');const data=await response.json();catalog=data.scenarios[0].assessment.request.medications;el('demo-notice').textContent=data.notice;
 catalog.forEach((c,i)=>{const card=make('article',undefined,'medication'),label=make('label',c.product),include=make('input');include.type='checkbox';include.id='include-'+i;label.prepend(include);card.append(label);const grid=make('div',undefined,'grid');for(const [key,title]of [['dose','Amount per dose'],['interval','Hours between doses']]){const l=make('label',title),input=make('input');input.id=key+'-'+i;input.type='number';input.step='any';input.min='0';l.append(input);grid.append(l);}const l=make('label','Dose units'),select=make('select');select.id='dose-unit-'+i;for(const unit of ['mg','g']){const opt=make('option',unit);select.append(opt);}l.append(select);grid.append(l);card.append(grid);el('prescriptions').append(card);});
 const presetNames=['Both within limits','One prescription exceeds limits','Change kidney function','Missing patient weight','Uncertain creatinine'];for(const [i,s] of data.scenarios.entries()){const b=make('button',presetNames[i],'secondary');b.type='button';b.onclick=()=>{fill(s.assessment.request);if(s.id==='uncertainty')el('patient-details').open=true;};el('presets').append(b);}fill(data.scenarios[0].assessment.request);el('demo-status').textContent='Example loaded. Edit any inputs, then select Check prescription.';
})().catch(e=>{el('demo-status').textContent=e.message;el('evaluate').disabled=true;});
