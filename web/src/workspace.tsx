import { createContext,useCallback,useContext,useEffect,useId,useRef,useState,type ReactNode } from 'react';
type DraftState={dirty:boolean;isDirty:()=>boolean;mark:(id:string,value:boolean)=>void};
export const DraftContext=createContext<DraftState>({dirty:false,isDirty:()=>false,mark:()=>{}});
export function DraftProvider({children}:{children:ReactNode}){const flags=useRef(new Set<string>());const [,render]=useState(0);const mark=useCallback((id:string,value:boolean)=>{if(flags.current.has(id)===value)return;if(value)flags.current.add(id);else flags.current.delete(id);render(x=>x+1);},[]);return <DraftContext.Provider value={{dirty:flags.current.size>0,isDirty:()=>flags.current.size>0,mark}}>{children}</DraftContext.Provider>;}
export function useDraft(){const {mark}=useContext(DraftContext),id=useId();const [dirty,setLocal]=useState(false);useEffect(()=>()=>mark(id,false),[id,mark]);return {dirty,setDirty:(value:boolean)=>{mark(id,value);setLocal(value);}};}
