import type { components } from './api.generated';
export type Model<K extends keyof components['schemas']> = components['schemas'][K];
export type Session=Model<'Session'>;export type Patient=Model<'SyntheticPatient'>;export type Observation=Model<'Observation'>;export type Summary=Model<'AssessmentSummary'>;export type Assessment=Model<'AssessmentRecord'>;export type Context=Model<'Context'>;export type Rule=Model<'StoredRule'>;export type Document=Model<'Document'>;
let csrf='',epoch=0;
const inflight=new Set<AbortController>();
export class ApiError extends Error { constructor(public status:number,public code:string){super(code.replaceAll('_',' '));} }
export function resetClient(token=''){epoch++;csrf=token;for(const controller of inflight)controller.abort();inflight.clear();}
export function invalidateRequests(){epoch++;for(const controller of inflight)controller.abort();inflight.clear();}
export async function api<T>(path:string,options:{method?:string;body?:unknown;blob?:boolean}={}):Promise<T>{
 const generation=epoch,controller=new AbortController();inflight.add(controller);
 try{
  const headers:Record<string,string>={};const multipart=options.body instanceof FormData;
  if(options.body!==undefined&&!multipart)headers['Content-Type']='application/json';if(csrf)headers['X-CSRF-Token']=csrf;
  const response=await fetch('/api/v1'+path,{method:options.method||'GET',headers,body:options.body===undefined?undefined:multipart?options.body as FormData:JSON.stringify(options.body),credentials:'same-origin',signal:controller.signal});
  if(generation!==epoch)throw new DOMException('Workspace changed','AbortError');
  if(!response.ok){const error=await response.json().catch(()=>({error:'request_failed'}));if(response.status===401&&!path.startsWith('/auth/login')&&path!=='/auth/session')window.dispatchEvent(new Event('sessionexpired'));throw new ApiError(response.status,error.error||'request_failed');}
  const value=options.blob?await response.blob():await response.json();if(generation!==epoch)throw new DOMException('Workspace changed','AbortError');return value as T;
 }finally{inflight.delete(controller);}
}
export function errorText(error:unknown){if(error instanceof DOMException&&error.name==='AbortError')return '';if(error instanceof ApiError){if(error.code==='invalid_credentials_or_try_later')return 'Sign-in failed. Check your credentials or try again later.';if(error.status===409)return 'This record changed or the request conflicts with saved history. Refresh the record before retrying.';if(error.status===403)return 'Your role cannot perform this action.';if(error.status===404)return 'This record is unavailable or you no longer have access.';if(error.status===401)return 'Sign in again to continue.';if(error.status===503)return 'The service is unavailable. No successful save was confirmed; retry the same request when it recovers.';}return error instanceof Error?error.message:'The request could not be completed.';}
export function contextLabel(c:Context){return [c.product,c.form,c.route,c.indication].join(' · ');}
export function localTime(value:string|Date=new Date()){const d=new Date(value);return new Date(d.getTime()-d.getTimezoneOffset()*60000).toISOString().slice(0,16);}
export const displayTime=(value:string)=>new Date(value).toLocaleString('en-GB');
export function download(blob:Blob,name:string){const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download=name;link.click();setTimeout(()=>URL.revokeObjectURL(url),10000);}
