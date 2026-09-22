"use client";

import { useState } from "react";

type UserRow={user_id:number;email:string;status:string;email_verified_at?:string;failed_login_count:number;locked_until?:string;created_at:string};
type SiteRow={site_id:number;user_id:number;email:string;host:string;origin:string;status:string;verification_method?:string;verified_at?:string};
type InvoiceRow={invoice_id:number;account_id:number;subscription_id?:number;status:string;amount_kopecks:number;currency:string;due_at?:string;paid_at?:string;created_at:string};
type SessionRow={session_id:number;user_id:number;expires_at:string;last_seen_at:string;created_at:string;revoked_at?:string};
type SecurityEventRow={event_id:number;user_id?:number;event_type:string;created_at:string};

const card:React.CSSProperties={border:"1px solid #e2e2e2",borderRadius:12,padding:16,background:"#fff"};
const input:React.CSSProperties={width:"100%",boxSizing:"border-box",padding:"10px 12px",border:"1px solid #ccc",borderRadius:8,font:"inherit"};
const button:React.CSSProperties={padding:"9px 12px",border:0,borderRadius:8,cursor:"pointer",fontWeight:700};

async function getJSON<T>(url:string):Promise<T>{const r=await fetch(url,{cache:"no-store",credentials:"same-origin"});const data=await r.json().catch(()=>({}));if(!r.ok)throw new Error(typeof data?.error==="string"?data.error:`http_${r.status}`);return data as T}

export default function SupportPanels({role}:{role:string}){
  const operator=role==="OPERATOR"||role==="SUPERADMIN";
  const [query,setQuery]=useState("");const [users,setUsers]=useState<UserRow[]>([]);const [sites,setSites]=useState<SiteRow[]>([]);const [invoices,setInvoices]=useState<InvoiceRow[]>([]);const [selectedUser,setSelectedUser]=useState<UserRow|null>(null);const [sessions,setSessions]=useState<SessionRow[]>([]);const [events,setEvents]=useState<SecurityEventRow[]>([]);const [error,setError]=useState("");
  async function search(){try{const q=encodeURIComponent(query);const [u,s,b]=await Promise.all([getJSON<{results:UserRow[]}>(`/api/admin/users?q=${q}&limit=50`),getJSON<{results:SiteRow[]}>(`/api/admin/webmaster/sites?q=${q}&limit=50`),getJSON<{results:InvoiceRow[]}>(`/api/admin/billing/invoices?limit=30`)]);setUsers(u.results||[]);setSites(s.results||[]);setInvoices(b.results||[]);setError("")}catch(e){setError(e instanceof Error?e.message:"support_unavailable")}}
  async function inspectUser(user:UserRow){if(!operator)return;try{const [s,e]=await Promise.all([getJSON<{results:SessionRow[]}>(`/api/admin/users/sessions?user_id=${user.user_id}&limit=25`),getJSON<{results:SecurityEventRow[]}>(`/api/admin/users/security-events?user_id=${user.user_id}&limit=50`)]);setSelectedUser(user);setSessions(s.results||[]);setEvents(e.results||[]);setError("")}catch(err){setError(err instanceof Error?err.message:"user_diagnostics_unavailable")}}
  return <section style={{display:"grid",gap:12}}>
    <div style={card}><h2 style={{marginTop:0}}>Пользователи, Webmaster и billing</h2><div style={{display:"flex",gap:8}}><input style={input} placeholder="email или домен" value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")void search()}}/><button style={button} onClick={()=>void search()}>Найти</button></div>{error&&<div role="alert" style={{marginTop:10,color:"#991b1b"}}>{error}</div>}</div>
    <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(300px,1fr))",gap:12}}>
      <div style={card}><strong>Consumer users</strong><div style={{display:"grid",gap:6,marginTop:10}}>{users.map(u=><button key={u.user_id} style={{...button,textAlign:"left",background:selectedUser?.user_id===u.user_id?"#e5e7eb":"#f8f8f8"}} onClick={()=>void inspectUser(u)} disabled={!operator}><div>{u.email}</div><small>{u.status} · failed {u.failed_login_count}{u.locked_until?" · locked":""}</small></button>)}{!users.length&&<small>Нет загруженных результатов.</small>}</div></div>
      <div style={card}><strong>Webmaster sites</strong><div style={{display:"grid",gap:8,marginTop:10}}>{sites.map(s=><div key={s.site_id}><div>{s.host}</div><small>{s.email} · {s.status}{s.verification_method?` · ${s.verification_method}`:""}</small></div>)}{!sites.length&&<small>Нет загруженных результатов.</small>}</div></div>
      <div style={card}><strong>Последние invoices</strong><div style={{display:"grid",gap:8,marginTop:10}}>{invoices.map(i=><div key={i.invoice_id}><div>#{i.invoice_id} · {i.status}</div><small>{(i.amount_kopecks/100).toLocaleString("ru-RU")} {i.currency} · account {i.account_id}</small></div>)}{!invoices.length&&<small>Нет загруженных результатов.</small>}</div></div>
    </div>
    {operator&&selectedUser&&<div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(320px,1fr))",gap:12}}>
      <div style={card}><strong>Сессии: {selectedUser.email}</strong><div style={{display:"grid",gap:8,marginTop:10}}>{sessions.map(s=><div key={s.session_id}><div>session #{s.session_id}{s.revoked_at?" · revoked":""}</div><small>last {new Date(s.last_seen_at).toLocaleString()} · expires {new Date(s.expires_at).toLocaleString()}</small></div>)}{!sessions.length&&<small>Нет сессий.</small>}</div></div>
      <div style={card}><strong>Security events</strong><div style={{display:"grid",gap:8,marginTop:10}}>{events.map(e=><div key={e.event_id}><div>{e.event_type}</div><small>{new Date(e.created_at).toLocaleString()}</small></div>)}{!events.length&&<small>Нет событий.</small>}</div></div>
    </div>}
  </section>
}
