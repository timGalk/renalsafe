import { createContext,useContext,useEffect,useState,type ReactNode } from 'react';
import { api,resetClient,type Session } from './api';
type Auth={session:Session|null;loading:boolean;reason:string;generation:number;login:(s:Session)=>void;end:(reason?:string)=>void};
const SessionContext=createContext<Auth>(null!);
const channel=typeof BroadcastChannel!=='undefined'?new BroadcastChannel('renalsafe-session'):null;
export function SessionProvider({children}:{children:ReactNode}){
 const [session,setSession]=useState<Session|null>(null),[loading,setLoading]=useState(true),[reason,setReason]=useState(''),[generation,setGeneration]=useState(0);
 function end(message=''){resetClient();setSession(null);setGeneration(x=>x+1);setLoading(false);setReason(message);}
 function login(value:Session){resetClient(value.csrf_token);setSession(value);setGeneration(x=>x+1);setReason('');channel?.postMessage('changed');}
 useEffect(()=>{let active=true;api<Session>('/auth/session').then(s=>{if(active){resetClient(s.csrf_token);setSession(s);}}).catch(()=>{}).finally(()=>{if(active)setLoading(false);});const expired=()=>{end('Your session ended. Unsaved inputs were cleared. Sign in again.');channel?.postMessage('ended');};const changed=()=>end('The account session changed in another tab. Sign in again.');window.addEventListener('sessionexpired',expired);if(channel)channel.onmessage=changed;return()=>{active=false;window.removeEventListener('sessionexpired',expired);if(channel)channel.onmessage=null;};},[]);
 return <SessionContext.Provider value={{session,loading,reason,generation,login,end}}>{children}</SessionContext.Provider>;
}
export const useSession=()=>useContext(SessionContext);
export function broadcastLogout(){channel?.postMessage('ended');}
