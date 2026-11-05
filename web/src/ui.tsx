import { useEffect,useRef,useState,type ReactNode,type FormEvent } from 'react';
import { api,errorText } from './api';
export function useResource<T>(path:string|null){const [data,setData]=useState<T>();const [error,setError]=useState('');const [loading,setLoading]=useState(true);const [revision,setRevision]=useState(0);const previousPath=useRef<string|null>(null);useEffect(()=>{let active=true;if(!path){setLoading(false);setData(undefined);return;}setLoading(true);if(previousPath.current!==path)setData(undefined);previousPath.current=path;setError('');api<T>(path).then(value=>{if(active)setData(value);}).catch(e=>{if(active)setError(errorText(e));}).finally(()=>{if(active)setLoading(false);});return()=>{active=false;};},[path,revision]);return {data,error,loading,reload:()=>setRevision(x=>x+1),setData};}
export function useAction(){const [busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');const lock=useRef(false);async function run(fn:()=>Promise<void>){if(lock.current)return;lock.current=true;setBusy(true);setError('');setNotice('');try{await fn();}catch(e){setError(errorText(e));}finally{lock.current=false;setBusy(false);}}return {busy,error,notice,setError,setNotice,run};}
export function Field({label,id,children,hint}:{label:string;id?:string;children:ReactNode;hint?:string}){return <label htmlFor={id} className="field"><span>{label}</span>{children}{hint&&<small>{hint}</small>}</label>;}
export function Alerts({error,notice}:{error?:string;notice?:string}){return <>{error&&<div role="alert" className="alert error">{error}</div>}{notice&&<div role="status" className="alert success">{notice}</div>}</>;}
export function Loading(){return <p role="status" className="loading">Loading workspace records…</p>;}
export function ResourceState({loading,error,retry}:{loading:boolean;error:string;retry:()=>void}){return <>{loading&&<Loading/>}{error&&<div role="alert" className="alert error">{error} <button className="secondary" onClick={retry}>Retry</button></div>}</>;}
export function Empty({children}:{children:ReactNode}){return <div className="empty">{children}</div>;}
export function Badge({children,tone='neutral'}:{children:ReactNode;tone?:string}){return <span className={'badge '+tone}>{children}</span>;}
export function Submit({busy,children,disabled=false}:{busy:boolean;children:ReactNode;disabled?:boolean}){return <button disabled={busy||disabled} type="submit">{busy?'Saving…':children}</button>;}
export function formValues(event:FormEvent<HTMLFormElement>){event.preventDefault();return new FormData(event.currentTarget);}
export const text=(data:FormData,name:string)=>String(data.get(name)||'');
export const numeric=(data:FormData,name:string)=>Number(text(data,name));
export function Json({value}:{value:unknown}){return <pre>{JSON.stringify(value,null,2)}</pre>;}
export function PageTitle({title,description,actions}:{title:string;description?:string;actions?:ReactNode}){return <header className="page-title"><div><p className="eyebrow">RENALSAFE WORKSPACE</p><h1 tabIndex={-1}>{title}</h1>{description&&<p>{description}</p>}</div>{actions&&<div className="actions">{actions}</div>}</header>;}
