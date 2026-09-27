"use client";

import { useEffect,useState } from "react";

type Check={name:string;pass:boolean;detail:string};
type Report={ready:boolean;git_commit:string;release_version:string;expected_schema:number;applied_schema:number;checked_at:string;checks:Check[]};
const card:React.CSSProperties={border:"1px solid #e2e2e2",borderRadius:12,padding:16,background:"#fff"};
async function read<T>(url:string):Promise<T>{const r=await fetch(url,{cache:"no-store",credentials:"same-origin"});const d=await r.json().catch(()=>({}));if(!r.ok)throw new Error(d?.error||`http_${r.status}`);return d as T}
export default function ReadinessPage(){
 const[report,setReport]=useState<Report|null>(null);const[error,setError]=useState("");
 async function load(){try{setReport(await read<Report>("/api/admin/readiness"));setError("")}catch(e){setError(e instanceof Error?e.message:"unavailable")}}
 useEffect(()=>{void load()},[]);
 return <main style={{maxWidth:1180,margin:"32px auto",padding:24,display:"grid",gap:16}}>
  <header style={{display:"flex",justifyContent:"space-between",gap:16,alignItems:"center"}}><div><a href="/admin">← Admin</a><h1 style={{margin:"8px 0 0"}}>Commercial readiness</h1><p style={{margin:"6px 0 0",color:"#666"}}>Read-only verdict из canonical readiness gate. Здесь нельзя вручную выставить READY.</p></div><button onClick={()=>void load()}>Обновить</button></header>
  {error?<section style={{...card,borderColor:"#e4b2b2"}}><strong>Gate unavailable</strong><div>{error}</div><p>Проверьте POISK_GIT_COMMIT, POISK_RELEASE_VERSION и доступность migration directory.</p></section>:null}
  {report?<>
   <section style={{...card,borderColor:report.ready?"#8dc98a":"#e4b2b2",borderWidth:2}}><div style={{fontSize:28,fontWeight:700}}>{report.ready?"READY":"NOT READY"}</div><div>Release {report.release_version} · schema {report.applied_schema}/{report.expected_schema}</div><div style={{fontFamily:"monospace",wordBreak:"break-all",fontSize:12}}>commit {report.git_commit}</div><div style={{fontSize:12,color:"#777"}}>checked {new Date(report.checked_at).toLocaleString()}</div></section>
   <section style={{display:"grid",gap:10}}>{report.checks.map(c=><article key={c.name} style={{...card,borderColor:c.pass?"#b8dcb5":"#e4b2b2"}}><div style={{display:"flex",justifyContent:"space-between",gap:12}}><strong>{c.name}</strong><span style={{fontWeight:700}}>{c.pass?"PASS":"FAIL"}</span></div><div style={{marginTop:8,fontFamily:"monospace",fontSize:12,whiteSpace:"pre-wrap",overflowWrap:"anywhere"}}>{c.detail}</div></article>)}</section>
  </>:null}
 </main>
}
