import { useContext,useEffect,useLayoutEffect,useRef,Component,type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter,RouterProvider,Link,NavLink,useBlocker,useLocation,useNavigate } from 'react-router-dom';
import { api,invalidateRequests,errorText } from './api';
import { SessionProvider,useSession,broadcastLogout } from './session';
import { DraftContext,DraftProvider } from './workspace';
import { Loading } from './ui';
import { Login,Account,Users,Audit,Status } from './staff';
import { Dashboard,Patients,PatientDetail,History } from './patients';
import { AssessmentEditor,ResultPage,Comparison } from './results';
import { Evidence,Documents,Review } from './sources';
import './style.css';
class ErrorBoundary extends Component<{children:ReactNode},{failed:boolean}>{state={failed:false};static getDerivedStateFromError(){return {failed:true};}render(){return this.state.failed?<div role="alert" className="empty"><h1>The page could not be displayed</h1><p>Reload to restore your session. Unsaved inputs on this page may be lost.</p><button onClick={()=>location.reload()}>Reload workspace</button></div>:this.props.children;}}
function DraftGuard(){const {dirty,isDirty}=useContext(DraftContext);const blocker=useBlocker(({currentLocation,nextLocation})=>isDirty()&&(currentLocation.pathname+currentLocation.search!==nextLocation.pathname+nextLocation.search));const ref=useRef<HTMLDialogElement>(null);useEffect(()=>{if(blocker.state==='blocked')ref.current?.showModal();else ref.current?.close();},[blocker.state]);useEffect(()=>{if(!dirty)return;const warn=(event:BeforeUnloadEvent)=>{event.preventDefault();};window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);},[dirty]);return <dialog ref={ref} aria-labelledby="draft-title" onCancel={e=>{e.preventDefault();if(blocker.state==='blocked')blocker.reset();}}><h2 id="draft-title">Leave unsaved changes?</h2><p>Your inputs are held only in this tab. Leaving this page discards them.</p><div className="actions"><button autoFocus onClick={()=>{if(blocker.state==='blocked')blocker.reset();}}>Keep editing</button><button className="danger" onClick={()=>{if(blocker.state==='blocked')blocker.proceed();}}>Discard changes and continue</button></div></dialog>;}
function Logout(){const {end}=useSession();const nav=useNavigate();const started=useRef(false);useEffect(()=>{if(started.current)return;started.current=true;api('/auth/logout',{method:'POST'}).then(()=>{broadcastLogout();end();nav('/login',{replace:true});}).catch(e=>{if(errorText(e))alert(errorText(e));nav('/account',{replace:true});});},[]);return <Loading/>;}
function Shell(){const auth=useSession(),loc=useLocation();useLayoutEffect(()=>{invalidateRequests();},[loc.pathname,loc.search]);useEffect(()=>{document.title='RenalSafe · '+(loc.pathname.split('/')[1]||'Workspace');const heading=document.querySelector<HTMLElement>('h1');heading?.focus();window.scrollTo(0,0);},[loc.pathname,loc.search]);if(auth.loading)return <main className="auth-shell"><Loading/></main>;if(!auth.session)return <Login/>;return <DraftProvider key={auth.generation}><Workspace/></DraftProvider>;}
function Workspace(){const {session}=useSession(),loc=useLocation();const role=session!.actor.role;const parts=loc.pathname.split('/').filter(Boolean);let page:ReactNode;
 if(session!.must_change_password&&loc.pathname!=='/account'&&loc.pathname!=='/logout')page=<Account/>;
 else if(loc.pathname==='/logout')page=<Logout/>;
 else if(parts[0]==='patients'&&parts.length===2)page=<PatientDetail id={parts[1]}/>;
 else if(loc.pathname==='/patients')page=<Patients/>;
 else if(loc.pathname==='/history')page=<History/>;
 else if(loc.pathname==='/assess/new')page=role==='physician'?<AssessmentEditor/>:<div className="empty"><h1>Access restricted</h1><p>Patient assessment authoring requires the physician role.</p></div>;
 else if(parts[0]==='assessments'&&parts.length===2)page=<ResultPage id={parts[1]}/>;
 else if(loc.pathname==='/compare')page=<Comparison/>;
 else if(loc.pathname==='/evidence')page=<Evidence/>;
 else if(loc.pathname==='/account')page=<Account/>;
 else if((parts[0]==='review'||loc.pathname==='/documents')&&role==='physician'||parts[0]==='admin'&&role!=='admin')page=<div className="empty"><h1 tabIndex={-1}>Access restricted</h1><p>Your role cannot open this page.</p><Link to="/dashboard">Return to workspace</Link></div>;
 else if(loc.pathname==='/review')page=<Review/>;
 else if(loc.pathname==='/documents')page=<Documents/>;
 else if(loc.pathname==='/admin/users')page=<Users/>;
 else if(loc.pathname==='/admin/audit')page=<Audit/>;
 else if(loc.pathname==='/admin/status')page=<Status/>;
 else if(['/','/dashboard','/login','/activate'].includes(loc.pathname))page=<Dashboard/>;
 else page=<div className="empty"><h1 tabIndex={-1}>Page not found</h1><p>Choose a workspace section to continue.</p><Link to="/dashboard">Open dashboard</Link></div>;
 const item=(to:string,label:string,id?:string)=><NavLink to={to} id={id}>{label}</NavLink>;
 return <><a className="skip" href="#content">Skip to content</a><header className="topbar"><Link to="/dashboard" className="brand"><span className="brand-mark">R</span><span>RenalSafe<small>Renal decision workspace</small></span></Link><div className="identity"><span>{session!.username}<small>{role}</small></span><Link to="/logout" id="logout" className="button secondary">Sign out</Link></div></header><div className="app-layout" id="workspace"><aside className="sidebar"><p className="nav-label">CLINICIAN WORKSPACE</p><nav aria-label="Workspace sections">{item('/dashboard','Overview')}{item('/patients','Patients','patients-nav')}{item('/history','Saved results','history-nav')}{role==='physician'&&item('/assess/new','New assessment')}{item('/evidence','Source evidence')}{role!=='physician'&&<>{item('/review','Rule review','review-nav')}{item('/documents','Documents','documents-nav')}</>}{item('/account','My account')}{role==='admin'&&<><p className="nav-label">ADMINISTRATION</p>{item('/admin/users','Staff accounts','admin-nav')}{item('/admin/audit','Audit history')}{item('/admin/status','System status')}</>}</nav><div className="sidebar-note"><strong>Synthetic cases only</strong><p>Source-backed decisions with a traceable review history.</p><a href="/demo.html">Training scenarios ↗</a></div></aside><main id="content" className="content"><div className="scope-note">Academic workspace · Synthetic cases only · Confirm scope for every assessment.</div><ErrorBoundary key={loc.pathname+loc.search}><div key={loc.pathname+loc.search}>{page}</div></ErrorBoundary><footer>Deterministic calculations · Reviewed rules · Traceable source evidence</footer></main></div><DraftGuard/></>;
}
const router=createBrowserRouter([{path:'*',element:<SessionProvider><Shell/></SessionProvider>}]);
createRoot(document.getElementById('root')!).render(<RouterProvider router={router}/>);
