"use client";

import { useEffect,useState } from "react";

type AdminMe={admin_id:number;email:string;role:string;expires_at:string};
type Review={review_id:number;place_id:number;place_name:string;rating:number;body:string;status:string;version:number;open_reports:number;updated_at:string};
type Preview={preview_token:string;expires_at:string;review:Review;action:string;reason:string};
const card:React.CSSProperties={border:"1px solid #e2e2e2",borderRadius:12,padding:16,background:"#fff"};
async function json<T>(url:string,init?:RequestInit):Promise<T>{const r=await fetch(url,{...init,cache:"no-store",credentials:"same-origin"});const data=await r.json().catch(()=>({}));if(!r.ok)throw new Error(data?.error||`http_${r.status}`);return data as T}

export default function ReviewsAdmin(){
 const[me,setMe]=useState<AdminMe|null>(null);const[csrf,setCSRF]=useState("");const[rows,setRows]=useState<Review[]>([]);const[selected,setSelected]=useState<Review|null>(null);const[action,setAction]=useState("HIDE");const[reason,setReason]=useState("Проверено модератором");const[preview,setPreview]=useState<Preview|null>(null);const[status,setStatus]=useState("");const[error,setError]=useState("");
 const operator=me?.role==="OPERATOR"||me?.role==="SUPERADMIN";
 async function load(){try{const[m,c,list]=await Promise.all([json<AdminMe>("/api/admin/me"),json<{csrf_token:string}>("/api/admin/csrf"),json<{results:Review[]}>(`/api/admin/reviews/moderation?limit=100${status?`&status=${encodeURIComponent(status)}`:""}`)]);setMe(m);setCSRF(c.csrf_token);setRows(list.results);setError("")}catch(e){setError(e instanceof Error?e.message:"unavailable")}}
 useEffect(()=>{void load()},[status]);
 async function makePreview(){if(!selected||!operator)return;try{const p=await json<Preview>("/api/admin/reviews/moderation/preview",{method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({review_id:selected.review_id,action,reason})});setPreview(p);setError("")}catch(e){setError(e instanceof Error?e.message:"preview_failed")}}
 async function apply(){if(!preview||!operator)return;try{await json("/api/admin/reviews/moderation/apply",{method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({preview_token:preview.preview_token})});setPreview(null);setSelected(null);await load()}catch(e){setError(e instanceof Error?e.message:"apply_failed")}}
 return <main style={{maxWidth:1180,margin:"32px auto",padding:24,display:"grid",gap:16}}>
  <header><h1>Reviews moderation</h1>{me&&<small>{me.email} · {me.role}</small>}</header>
  {error&&<div role="alert" style={{...card,borderColor:"#b00"}}>{error}</div>}
  <section style={card}><div style={{display:"flex",gap:8,alignItems:"center",flexWrap:"wrap"}}><strong>Queue</strong><select value={status} onChange={e=>{setStatus(e.target.value);setSelected(null);setPreview(null)}}><option value="">PENDING + reported</option><option>VISIBLE</option><option>PENDING</option><option>HIDDEN</option><option>REJECTED</option><option>DELETED</option></select><button onClick={()=>void load()}>Обновить</button></div></section>
  <section style={card}><div style={{display:"grid",gap:8}}>{rows.map(r=><button key={r.review_id} type="button" onClick={()=>{setSelected(r);setPreview(null);setAction(r.status==="VISIBLE"?"HIDE":"SHOW")}} style={{textAlign:"left",padding:12,border:"1px solid #ddd",borderRadius:10,background:selected?.review_id===r.review_id?"#eee":"white"}}><strong>#{r.review_id} · {r.place_name}</strong><div>{"★".repeat(r.rating)}{"☆".repeat(5-r.rating)} · {r.status} · reports {r.open_reports} · v{r.version}</div><div style={{whiteSpace:"pre-wrap",overflowWrap:"anywhere",marginTop:5}}>{r.body}</div><small>{new Date(r.updated_at).toLocaleString()}</small></button>)}</div></section>
  {selected&&<section style={card}><h2>Review #{selected.review_id}</h2>{operator?<><div style={{display:"grid",gridTemplateColumns:"180px 1fr auto",gap:8}}><select value={action} onChange={e=>{setAction(e.target.value);setPreview(null)}}><option>HIDE</option><option>SHOW</option><option>REJECT</option></select><input value={reason} maxLength={240} onChange={e=>{setReason(e.target.value);setPreview(null)}} placeholder="Причина модерации"/><button onClick={()=>void makePreview()}>Preview</button></div>{preview&&<div style={{marginTop:12,padding:12,border:"1px solid #d99",borderRadius:10}}><strong>Подтвердите действие</strong><div>{preview.action}: {preview.review.place_name}, review #{preview.review.review_id}</div><div>Причина: {preview.reason}</div><div>Версия: {preview.review.version}; истекает {new Date(preview.expires_at).toLocaleString()}</div><button style={{marginTop:10}} onClick={()=>void apply()}>Apply moderation</button></div>}</>:<p>Read-only роль: изменение отзывов недоступно.</p>}</section>}
 </main>
}
