import { useEffect,useRef,useState } from 'react';
import { Link,useNavigate,useSearchParams } from 'react-router-dom';
import { api,ApiError,contextLabel,displayTime,download,localTime,type Model,type Patient,type Observation,type Context,type Document } from './api';
import { useSession } from './session';
import { AssessmentSources } from './sources';
import { useDraft } from './workspace';
import { Alerts,Badge,Empty,Field,Json,PageTitle,ResourceState,Submit,useAction,useResource } from './ui';
type Medication={index:string;dose:string;interval:string;unit:string};
type Inputs={age:string;sex:string;weight:string;height:string;creatinine:string;unit:string;measured:string;low:string;high:string};
const blank=():Inputs=>({age:'',sex:'',weight:'',height:'',creatinine:'',unit:'mg/dL',measured:'',low:'',high:''});
const scopeLabels:{id:string;label:string}[]=[{id:'stable',label:'Stable renal function confirmed'},{id:'no-aki',label:'Absence of suspected acute kidney injury confirmed'},{id:'no-dialysis',label:'Absence of dialysis confirmed'},{id:'idms',label:'IDMS-standardized creatinine confirmed'},{id:'body',label:'Typical body composition confirmed'},{id:'actual-weight',label:'Confirmed actual body weight provided'}];
const twoDrugContexts:Context[]=[{product:'Meronem 1 g',route:'intravenous',form:'powder for solution for injection or infusion',indication:'hospital-acquired pneumonia'},{product:'Ceftazidime Kabi 1000 mg',route:'intravenous',form:'powder for solution for injection',indication:'hospital-acquired pneumonia'}];
export function AssessmentEditor(){const [params]=useSearchParams(),pid=params.get('patient'),oid=params.get('observation'),previous=params.get('previous')||'',catalog=useResource<Context[]>('/catalog'),patient=useResource<Patient>(pid?'/patients/'+encodeURIComponent(pid):null),observation=useResource<Observation>(pid&&oid?'/patients/'+encodeURIComponent(pid)+'/observations/'+encodeURIComponent(oid):null),[inputs,setInputs]=useState(blank),[flags,setFlags]=useState<Record<string,boolean>>({}),[medications,setMedications]=useState<Medication[]>([{index:'',dose:'',interval:'',unit:'mg'}]),action=useAction(),draft=useDraft(),nav=useNavigate(),pending=useRef<{signature:string;body:Model<'PatientAssessmentInput'>}|null>(null),presetLoaded=useRef(false);
 const lab=observation.data?.input,linked=!!pid&&!!oid,blocked=!!pid!==!!oid||!!previous&&!linked||patient.data?.archived||!!observation.data?.superseded_by_id;
 const presets=twoDrugContexts.map(expected=>catalog.data?.findIndex(c=>contextLabel(c)===contextLabel(expected))??-1);
 const singleExample = params.get('example') === 'synthetic' && !pid && !oid;
 function loadExample(indices: number[], {markDirty = true} = {}) {
   setInputs({age:'70',sex:'male',weight:'72',height:'175',creatinine:'2.8',unit:'mg/dL',measured:localTime(),low:'',high:''});
   setFlags(Object.fromEntries(scopeLabels.map(({id}) => [id, true])));
   setMedications(indices.map(index => ({index:String(index),dose:'',interval:'',unit:'mg'})));
   if (markDirty) draft.setDirty(true);
 }
 useEffect(() => {
   if (!catalog.data?.length || presetLoaded.current) return;
   if (singleExample) {
     presetLoaded.current = true;
     loadExample([0], {markDirty:false});
   } else if (params.get('example') === 'two-drugs' && presets.every(index => index >= 0)) {
     presetLoaded.current = true;
     loadExample(presets);
   }
 }, [catalog.data]);
 function change(key:keyof Inputs,value:string){setInputs(old=>({...old,[key]:value}));draft.setDirty(true);}
 const labValue=(key:keyof Inputs)=>lab?key==='creatinine'?String(lab.creatinine.value):key==='unit'?lab.creatinine.unit:key==='measured'?localTime(lab.measured_at):key==='low'?String(lab.creatinine_bounds?.lower??''):key==='high'?String(lab.creatinine_bounds?.upper??''):inputs[key]:inputs[key];
 function medication(index:number,key:keyof Medication,value:string){setMedications(old=>old.map((m,i)=>i===index?{...m,[key]:value}:m));draft.setDirty(true);}
 return <><PageTitle title="Prescription assessment" description={singleExample ? "Review or change the synthetic inputs, then select Calculate and save assessment." : "Choose exact medicine contexts and confirm current inputs. The saved result retains its evidence and versions."} actions={singleExample ? <button type="submit" form="assessment-form" disabled={action.busy || !catalog.data?.length}>{action.busy ? 'Saving…' : 'Calculate and save assessment'}</button> : <><button className="secondary" id="example" disabled={!catalog.data?.length} onClick={()=>loadExample([0])}>Load synthetic example</button>{presets.every(i=>i>=0)&&<button className="secondary" id="example-two-drugs" onClick={()=>loadExample(presets)}>Load two-drug example</button>}</>}/><ResourceState {...catalog} retry={catalog.reload}/>{pid&&<ResourceState {...patient} retry={patient.reload}/>} {oid&&<ResourceState {...observation} retry={observation.reload}/>}<Alerts error={action.error}/>{linked&&patient.data&&lab&&<div className="alert" id="assessment-patient-context"><strong>{patient.data.alias||patient.data.code}</strong> · sample {displayTime(lab.measured_at)}{previous&&' · recalculation after '+previous}<p>Selected laboratory fields are fixed. Confirm the remaining clinical inputs for this assessment.</p><Link to={'/patients/'+pid}>Back to case</Link></div>}{blocked&&<div role="alert" className="alert error">This case or observation cannot be used for a new assessment. Choose a current observation from an active case.<p><Link to={pid?'/patients/'+pid:'/patients'}>Open patient workspace</Link></p></div>}{catalog.data&&!catalog.data.length&&<Empty>No active medicine contexts are available. A reviewer must import an appropriate source first.</Empty>}{catalog.data?.length&&!blocked&&(!linked||patient.data&&lab)?<form id="assessment-form" className="card" onSubmit={e=>{e.preventDefault();void action.run(async()=>{
 const low=lab?.creatinine_bounds?.lower??inputs.low,high=lab?.creatinine_bounds?.upper??inputs.high;if(!lab&&(inputs.low==='')!==(inputs.high===''))throw new Error('Enter both uncertainty bounds.');
 const body:Model<'Request'>={assessment_at:new Date().toISOString(),patient:{age_years:Number(inputs.age),equation_sex:inputs.sex,creatinine:lab?.creatinine||{value:Number(inputs.creatinine),unit:inputs.unit},measured_at:lab?.measured_at||new Date(inputs.measured).toISOString(),stable_renal_function:flags.stable?true:null,suspected_aki:flags['no-aki']?false:null,dialysis:flags['no-dialysis']?false:null,idms_standardized:lab?lab.idms_standardized??null:flags.idms?true:null,typical_body_composition:flags.body?true:null,actual_weight_confirmed:flags['actual-weight']?true:null,...(inputs.weight!==''?{weight:{value:Number(inputs.weight),unit:'kg'}}:{}),...(inputs.height!==''?{height:{value:Number(inputs.height),unit:'cm'}}:{}),...(lab?.creatinine_bounds?{creatinine_bounds:lab.creatinine_bounds}:!lab&&inputs.low!==''?{creatinine_bounds:{lower:Number(low),upper:Number(high),lower_inclusive:true,upper_inclusive:true}}:{})},medications:medications.map(m=>catalog.data![Number(m.index)]),prescriptions:medications.flatMap((m,index)=>{if(!m.dose&&!m.interval)return [];if(!m.dose||!m.interval)throw new Error('Enter both dose and interval to check a prescription.');return [{medication_index:index,dose:{value:Number(m.dose),unit:m.unit},interval_hours:Number(m.interval)}];})};
 let out:Model<'AssessmentRecord'>;if(linked){const signature=JSON.stringify({...body,assessment_at:undefined});if(!pending.current||pending.current.signature!==signature)pending.current={signature,body:{observation_id:oid!,previous_assessment_id:previous,idempotency_key:crypto.randomUUID(),request:body}};out=await api('/patients/'+pid+'/assessments',{method:'POST',body:pending.current.body});}else out=await api('/assessments',{method:'POST',body});draft.setDirty(false);nav('/assessments/'+out.id);
 });}}><h2>Patient inputs</h2><div className="grid"><Field label="Age, years"><input id="age" type="number" min="0" max="150" required value={inputs.age} onChange={e=>change('age',e.target.value)}/></Field><Field label="Sex used in the equation"><select id="sex" required value={inputs.sex} onChange={e=>change('sex',e.target.value)}><option value="">Select</option><option value="female">Female</option><option value="male">Male</option></select></Field><Field label="Weight, kg"><input id="weight" type="number" min="0" step="any" value={inputs.weight} onChange={e=>change('weight',e.target.value)}/></Field><Field label="Height, cm"><input id="height" type="number" min="0" step="any" value={inputs.height} onChange={e=>change('height',e.target.value)}/></Field><Field label="Serum creatinine"><input id="creatinine" type="number" min="0" step="any" required disabled={linked} value={labValue('creatinine')} onChange={e=>change('creatinine',e.target.value)}/></Field><Field label="Units"><select id="unit" disabled={linked} value={labValue('unit')} onChange={e=>change('unit',e.target.value)}><option>mg/dL</option><option>umol/L</option></select></Field><Field label="Sample date and time"><input id="measured" type="datetime-local" required disabled={linked} value={labValue('measured')} onChange={e=>change('measured',e.target.value)}/></Field></div><details><summary>Known creatinine uncertainty interval</summary><p>Leave blank when unknown. A point estimate does not imply zero uncertainty.</p><div className="grid"><Field label="Lower bound"><input type="number" step="any" disabled={linked} value={labValue('low')} onChange={e=>change('low',e.target.value)}/></Field><Field label="Upper bound"><input type="number" step="any" disabled={linked} value={labValue('high')} onChange={e=>change('high',e.target.value)}/></Field></div></details><h2>Clinical scope confirmation</h2><div className="checks">{scopeLabels.map(f=><label className="check" key={f.id}><input id={f.id} type="checkbox" disabled={linked&&f.id==='idms'} checked={linked&&f.id==='idms'?lab?.idms_standardized===true:flags[f.id]||false} onChange={e=>{setFlags(old=>({...old,[f.id]:e.target.checked}));draft.setDirty(true);}}/>{f.label}</label>)}</div><h2>Medications</h2><div id="medications">{medications.map((m,index)=><section className="medication" key={index}><Field label={'Medicine '+(index+1)+' and exact context'}><select className="med-context" required value={m.index} onChange={e=>medication(index,'index',e.target.value)}><option value="">Select exact product, form, route and indication</option>{catalog.data!.map((c,i)=><option key={i} value={String(i)}>{contextLabel(c)}</option>)}</select></Field><div className="grid"><Field label="Current single dose, if known"><input className="dose" type="number" min="0" step="any" value={m.dose} onChange={e=>medication(index,'dose',e.target.value)}/></Field><Field label="Dose units"><select value={m.unit} onChange={e=>medication(index,'unit',e.target.value)}>{['mg','g','microgram','IU'].map(x=><option key={x}>{x}</option>)}</select></Field><Field label="Dosing interval, hours"><input className="interval" type="number" min="0" step="any" value={m.interval} onChange={e=>medication(index,'interval',e.target.value)}/></Field></div>{medications.length>1&&<button type="button" className="secondary" onClick={()=>{setMedications(old=>old.filter((_,i)=>i!==index));draft.setDirty(true);}}>Remove medicine</button>}</section>)}</div><button type="button" className="secondary" onClick={()=>{setMedications(old=>[...old,{index:'',dose:'',interval:'',unit:'mg'}]);draft.setDirty(true);}}>Add medication</button>{!singleExample&&<div className="submit-row"><Submit busy={action.busy}>Calculate and save assessment</Submit></div>}</form>:null}</>;
}
const reasonLabels:Record<string,string>={NO_APPROVED_RULE:'No approved rule is available',UNCERTAINTY_CROSSES_RULE_BOUNDARY:'The uncertainty interval crosses a rule boundary',CG_WEIGHT_REQUIRED:'Actual body weight is required for Cockcroft–Gault',STABILITY_UNCONFIRMED:'Stable renal function has not been confirmed',AKI_OR_UNKNOWN:'Acute kidney injury has not been excluded',DIALYSIS_OR_UNKNOWN:'Dialysis has not been excluded',CREATININE_TIMESTAMP_INVALID_OR_STALE:'The sample timestamp does not meet the prototype policy',REQUIRED_RENAL_METRIC_UNAVAILABLE:'The required renal metric is unavailable'};
const reason=(value:string)=>reasonLabels[value]||value.replaceAll('_',' ').toLowerCase();
const prescriptionStatuses: Record<string, string> = {
  MANUAL_REVIEW_REQUIRED: 'Prescription needs review',
  OUTSIDE_RULE_LIMITS: 'Outside rule limits',
  WITHIN_RULE_LIMITS: 'Within modeled limits',
};

function AssessmentOverview({assessment}: {assessment: Model<'Assessment'>}) {
  const {results, renal, request} = assessment;
  const manual = results.filter(result => result.status === 'MANUAL_REVIEW_REQUIRED');
  const flagged = results.filter(result => result.status === 'MANUAL_REVIEW_REQUIRED' ||
    (result.prescription_check && result.prescription_check.status !== 'WITHIN_RULE_LIMITS'));
  const reasons = new Set([
    ...flagged.flatMap(result => [...result.reasons, ...(result.prescription_check?.reasons ?? [])]),
    ...renal.warnings,
  ]);
  const outcome = manual.length ? 'Manual review required'
    : flagged.length ? 'Prescription needs review'
    : results.length > 1 ? 'Rules matched'
    : results.length === 1 ? 'Rule matched'
    : 'Kidney function';

  return (
    <section className={'card assessment-overview' + (flagged.length ? ' needs-review' : '')} aria-label="Assessment outcome">
      <div className="assessment-outcome">
        <h2>{outcome}</h2>
        {flagged.length > 0 && results.length > 1 && <p>{flagged.length} of {results.length} medicines need review.</p>}
        {reasons.size > 0 && (
          <ul className="assessment-reasons">
            {[...reasons].map(code => <li key={code}>{reason(code)}</li>)}
          </ul>
        )}
      </div>
      {renal.estimates.length > 0 ? (
        <>
          <dl className="assessment-metrics" aria-label="Kidney function estimates">
            {renal.estimates.map(estimate => (
              <div key={estimate.metric}>
                <dt>{estimate.metric === 'CRCL_CG' ? 'Creatinine clearance (CrCl)' : 'Estimated filtration rate (eGFR)'}</dt>
                <dd>{estimate.value.toLocaleString(undefined, {maximumFractionDigits: 2})} <small>{estimate.unit}</small></dd>
              </div>
            ))}
          </dl>
          <details className="assessment-calculation">
            <summary>Calculation details</summary>
            <p className="meta">The display is rounded. Rule matching uses the saved full-precision values.</p>
            {renal.estimates.map(estimate => (
              <p key={estimate.metric}>
                <strong>{estimate.metric === 'CRCL_CG' ? 'Cockcroft–Gault' : 'CKD-EPI 2021'}</strong> · {estimate.value} {estimate.unit} · {estimate.formula_version}
                {request.patient.creatinine_bounds && <><br/>Computed range {estimate.low}–{estimate.high}</>}
              </p>
            ))}
            {!request.patient.creatinine_bounds && <p>Measurement uncertainty was not supplied.</p>}
          </details>
        </>
      ) : <p className="assessment-no-estimates">Kidney function could not be estimated from these inputs.</p>}
    </section>
  );
}

function MedicineResult({result, documents}: {
  result: Model<'Assessment'>['results'][number];
  documents: ReturnType<typeof useResource<Document[]>>;
}) {
  const {context, rule, prescription_check: prescription} = result;
  const manual = result.status === 'MANUAL_REVIEW_REQUIRED';
  const prescriptionStatus = prescription && (prescriptionStatuses[prescription.status] ?? reason(prescription.status));
  const flagged = manual || (prescription && prescription.status !== 'WITHIN_RULE_LIMITS');
  const label = manual ? 'Manual review' : prescriptionStatus ?? 'Rule matched';

  return (
    <details className="medicine-result">
      <summary className="medicine-result-summary">
        <span className="medicine-result-name">{context.product}</span>
        <Badge tone={flagged ? 'warning' : 'neutral'}>{label}</Badge>
      </summary>
      <div className="medicine-result-body">
        <p className="meta">{[context.form, context.route, context.indication].join(' · ')}</p>
        {manual
          ? <p className="meta">A clinician needs to review this result before choosing a dose.</p>
          : <><h3>{result.action ? reason(result.action) : 'Matched rule'}</h3>{result.dose_text && <p>{result.dose_text}</p>}</>}
        {result.reasons.length > 0 && (
          <ul className="assessment-reasons">
            {[...new Set(result.reasons)].map(code => <li key={code}>{reason(code)}</li>)}
          </ul>
        )}
        {prescription && (
          <section className="regimen">
            <h3>Prescription check</h3>
            <p>{prescriptionStatus}</p>
            {prescription.reasons.map((code, index) => <p key={index}>{reason(code)}</p>)}
            <small>This check addresses only the source rule's modeled limits.</small>
          </section>
        )}
        <AssessmentSources context={context} citation={rule?.citation} documents={documents}/>
        <details>
          <summary>Technical result</summary>
          <p>{result.explanation}</p>
          {rule && <>
            <blockquote>{rule.citation.quote}</blockquote>
            <p className="meta">Rule {rule.id} · version {rule.version} · section {rule.citation.section} · page {rule.citation.page}</p>
            <p className="meta">Source version {rule.citation.source_version} · SHA-256 {rule.citation.document_sha256}</p>
          </>}
        </details>
      </div>
    </details>
  );
}

function ResultContent({assessment, assessmentId}: {assessment: Model<'Assessment'>; assessmentId?: string}) {
  const documents = useResource<Document[]>(assessment.results.length ? '/documents' : null);
  return (
    <>
      <AssessmentOverview assessment={assessment}/>
      {assessmentId && <CombinationReviewPanel id={assessmentId} assessment={assessment}/>}
      {assessment.results.length > 0 && (
        <section className="card assessment-medicines" aria-label="Medicines and sources">
          <h2>Medicines and sources</h2>
          <p className="meta">Open a medicine to see its result and source documents.</p>
          {assessment.results.map((result, index) => <MedicineResult key={index} result={result} documents={documents}/>)}
        </section>
      )}
    </>
  );
}
const reviewTopics: Record<string, [string, string]> = {
  POTENTIAL_CONFLICT: ['Possible source conflict', 'Compare the quoted statements under the same conditions. Different medicines can have different instructions.'],
  RENAL_CRITERIA: ['Kidney-function criteria', 'Check which kidney measure, thresholds, or dialysis conditions each statement applies to.'],
  APPLICABILITY: ['Applicability', 'Check whether the formulation, route, indication, and population match the assessment.'],
  MONITORING: ['Warnings and monitoring', 'Check when the quoted warning or monitoring requirement applies.'],
  COADMINISTRATION: ['Co-administration wording', 'Check which products or drug classes the source statement names.'],
};

function CombinationReviewPanel({id, assessment}: {id: string; assessment: Model<'Assessment'>}) {
  const contexts = new Set(assessment.request.medications.map(context => contextLabel(context)));
  const available = contexts.size > 1;
  const service = useResource<Model<'ExtractionStatus'>>(available ? '/combination-review/status' : null);
  const [summary, setSummary] = useState<Model<'CombinationReviewResponse'>>();
  const action = useAction();
  if (!available) return null;

  async function generate() {
    await action.run(async () => {
      setSummary(undefined);
      try {
        setSummary(await api<Model<'CombinationReviewResponse'>>('/assessments/' + encodeURIComponent(id) + '/combination-review', {
          method: 'POST', body: {},
        }));
      } catch (error) {
        if (error instanceof ApiError && error.code === 'combination_review_failed') {
          throw new Error('The AI provider could not complete the review. Try again or open the sources below.');
        }
        if (error instanceof ApiError && error.code === 'combination_review_invalid_references') {
          throw new Error('The AI returned invalid source references. Try again or open the sources below.');
        }
        if (error instanceof ApiError && error.code === 'combination_review_unavailable') {
          throw new Error('AI combination review is currently unavailable.');
        }
        throw error;
      }
    });
  }

  return (
    <section className="card combination-review" aria-labelledby="combination-review-title">
      <div className="section-head">
        <div>
          <h2 id="combination-review-title">AI source review</h2>
          <p>Find possible conflicts or warnings to check across these medicines.</p>
        </div>
        {service.data?.configured && (
          <button type="button" disabled={action.busy} onClick={() => void generate()}>
            {action.busy ? 'Reviewing source passages…' : 'Generate AI summary'}
          </button>
        )}
      </div>
      <ResourceState {...service} retry={service.reload}/>
      {service.data && !service.data.configured && <p className="meta">AI review is unavailable. You can open each medicine’s sources below.</p>}
      {service.data?.configured && (
        <p className="meta">Sends public medicine-label excerpts to {service.data.provider === 'openai' ? 'OpenAI' : service.data.provider === 'nvidia' ? 'NVIDIA' : service.data.provider}. Use the findings to guide manual review.</p>
      )}
      <Alerts error={action.error}/>
      {action.busy && <p role="status" className="meta">Reviewing current source documents…</p>}
      {summary && (
        <div className="combination-findings">
          <h3>What to check</h3>
          {summary.limited && <p>The review used selected passages. Open the full documents for context.</p>}
          {summary.uncovered.length > 0 && <p>Source excerpts unavailable for {summary.uncovered.map(context => context.product).join(', ')}.</p>}
          {summary.review.findings.length === 0 && <p>The AI returned no checks from these excerpts. Review the full sources for the combination.</p>}
          {summary.review.findings.map((finding, index) => {
            const [topic, guidance] = reviewTopics[finding.topic] ?? ['Source check', 'Review the original passage in context.'];
            return (
              <details key={index} className="combination-finding">
                <summary>{topic} · {[...new Set(finding.citations.map(citation => citation.product))].join(', ')}</summary>
                <p>{guidance}</p>
                {finding.citations.map((citation, sourceIndex) => (
                  <figure key={sourceIndex}>
                    <blockquote>{citation.quote}</blockquote>
                    <figcaption className="meta">
                      {citation.product} · Section {citation.section} · {citation.page_start === citation.page_end ? 'page ' + citation.page_start : 'pages ' + citation.page_start + '–' + citation.page_end}
                      {' · '}<a href={citation.source_url} target="_blank" rel="noopener noreferrer">Official source ↗</a>
                    </figcaption>
                  </figure>
                ))}
              </details>
            );
          })}
          <details>
            <summary>About this review</summary>
            <p className="meta">Source sections 4.2–4.5 and 5.2 · {summary.review.model}</p>
          </details>
        </div>
      )}
    </section>
  );
}

export function ResultPage({id}: {id: string}) {
  const path = '/assessments/' + encodeURIComponent(id);
  const resource = useResource<Model<'AssessmentExport'>>(path + '/export');
  const action = useAction();
  const {session} = useSession();
  const out = resource.data;

  return (
    <>
      <header className="saved-assessment-header">
        <Link to="/history">← Saved results</Link>
        <h1 tabIndex={-1}>Saved assessment</h1>
        {out && <p className="meta">{out.patient_code || 'Unlinked assessment'} · {displayTime(out.created_at)}</p>}
      </header>
      <ResourceState {...resource} retry={resource.reload}/>
      {out && (
        <div id="assessment-result">
          <ResultContent assessment={out.assessment} assessmentId={out.id}/>
          <details className="card assessment-tools">
            <summary>Assessment details and exports</summary>
            <div className="actions">
              <button type="button" className="secondary" disabled={action.busy} onClick={() => void action.run(async () => {
                const replay = await api<{reproduced: boolean}>(path + '/replay', {method: 'POST'});
                action.setNotice(replay.reproduced
                  ? 'The result was reproduced using the saved knowledge version.'
                  : 'The saved result did not reproduce. Review the recorded versions.');
              })}>Verify reproducibility</button>
              <button type="button" className="secondary" disabled={action.busy} onClick={() => void action.run(async () =>
                download(await api<Blob>(path + '/export', {blob: true}), 'RenalSafe-' + id + '.json')
              )}>Export result JSON</button>
              <button type="button" className="secondary" disabled={action.busy} onClick={() => void action.run(async () => {
                download(await api<Blob>(path + '/report', {blob: true}), 'RenalSafe-' + id + '.html');
                action.setNotice('Report downloaded. Open it and use Print to print or save as PDF.');
              })}>Printable report</button>
              {out.patient_id && <Link className="button secondary" to={'/patients/' + out.patient_id}>Open patient</Link>}
              {out.patient_id && session?.actor.role === 'physician' &&
                <Link className="button secondary" to={'/patients/' + out.patient_id + '?recalculate=' + id}>Prepare recalculation</Link>}
            </div>
            <Alerts error={action.error} notice={action.notice}/>
            <dl>
              <dt>Assessment ID</dt><dd>{out.id}</dd>
              <dt>Sample time</dt><dd>{displayTime(out.assessment.request.patient.measured_at)}</dd>
              <dt>Engine</dt><dd>{out.assessment.engine_version}</dd>
              <dt>Policy</dt><dd>{out.assessment.policy_version}</dd>
              <dt>Knowledge</dt><dd>{out.assessment.knowledge_version}</dd>
              <dt>Knowledge digest</dt><dd>{out.assessment.knowledge_digest}</dd>
              <dt>Audit hash</dt><dd>{out.audit_hash}</dd>
              <dt>Observation</dt><dd>{out.observation_id || 'Unlinked'}</dd>
              <dt>Previous assessment</dt><dd>{out.previous_assessment_id || 'None'}</dd>
            </dl>
            <details>
              <summary>Saved inputs</summary>
              <Json value={out.assessment.request}/>
            </details>
          </details>
        </div>
      )}
    </>
  );
}
export function Comparison(){const [params]=useSearchParams(),left=params.get('left'),right=params.get('right'),a=useResource<Model<'AssessmentExport'>>(left?'/assessments/'+encodeURIComponent(left)+'/export':null),b=useResource<Model<'AssessmentExport'>>(right?'/assessments/'+encodeURIComponent(right)+'/export':null);const valid=a.data?.patient_id&&a.data.patient_id===b.data?.patient_id;return <><PageTitle title="Saved snapshot comparison" description="Changes in inputs or knowledge may change a result. This does not establish a change in health."/><ResourceState {...a} retry={a.reload}/><ResourceState {...b} retry={b.reload}/>{(!left||!right)&&<Empty>Select two saved results from the same synthetic case.</Empty>}{a.data&&b.data&&!valid&&<p className="alert error" role="alert">Only results linked to the same synthetic patient can be compared.</p>}{valid&&a.data&&b.data&&<div className="comparison-grid">{[a.data,b.data].map((r,i)=><section key={i}><h2>{displayTime(r.created_at)}</h2><p className="meta">{r.id}</p><p>Engine {r.assessment.engine_version} · knowledge {r.assessment.knowledge_version}</p><details className="card" open><summary>Saved inputs</summary><Json value={r.assessment.request}/></details><ResultContent assessment={r.assessment}/></section>)}</div>}</>;}
