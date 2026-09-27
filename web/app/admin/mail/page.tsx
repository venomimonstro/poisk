"use client";

import { useEffect,useState } from "react";

type AdminMe={admin_id:number;email:string;role:string;expires_at:string};
type DNS={domain:string;selector:string;mx:boolean;spf:boolean;dmarc:boolean;dkim:boolean;ready:boolean;drift:boolean;reasons?:string[];checked_at:string};
type Health={active_aliases:number;ready:number;leased:number;retry:number;submitted:number;delivered:number;bounced:number;dead:number;inbound_24h:number;delivered_24h:number;bounced_24h:number;dead_24h:number;active_suppressions:number;cooling_domains:number;oldest_queued_seconds:number;oldest_submitted_seconds:number;replay_guard_rows:number;inbound_receipts_processing:number;dns?:DNS;measured_at:string};
type Dead={delivery_id:number;message_id:number;sender_mailbox_id:number;attempts:number;error_code:string;retryable:boolean;updated_at:string};
type Preview={preview_token:string;expires_at:string;delivery:Dead};
const card:React.CSSProperties={border:"1px solid #e2e2e2",borderRadius:12,padding:16,background:"#fff"};
async function json<T>(url:string,init?:RequestInit):Promise<T>{const r=await fetch(url,{...init,cache:"no-store",credentials:"same-origin"});const d=await r.json().catch(()=>({}));if(!r.ok)throw new Error(d?.error||`http_${r.status}`);return d as T}
function age(seconds:number){if(seconds<=0)return "—";if(seconds<60)return `${seconds}s`;if(seconds<3600)return `${Math.floor(seconds/60)}m`;return `${Math.floor(seconds/3600)}h`}

export default function MailOpsPage(){
 const[me,setMe]=useState<AdminMe|null>(null);const[csrf,setCSRF]=useState("");const[health,setHealth]=useState<Health|null>(null);const[dead,setDead]=useState<Dead[]>([]);const[preview,setPreview]=useState<Preview|null>(null);const[error,setError]=useState("");
 const operator=me?.role==="OPERATOR"||me?.role==="SUPERADMIN";
 async function load(){try{const[m,c,h,d]=await Promise.all([json<AdminMe>("/api/admin/me"),json<{csrf_token:string}>("/api/admin/csrf"),json<Health>("/api/admin/mail/gateway-health"),json<{deliveries:Dead[]}>("/api/admin/mail/dead-letters?limit=100")]);setMe(m);setCSRF(c.csrf_token);setHealth(h);setDead(d.deliveries);setError("")}catch(e){setError(e instanceof Error?e.message:"unavailable")}}
 useEffect(()=>{void load()},[]);
 async function makePreview(item:Dead){if(!operator||!item.retryable)return;try{const p=await json<Preview>("/api/admin/mail/dead-letters/retry/preview",{method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({delivery_id:item.delivery_id})});setPreview(p);setError("")}catch(e){setError(e instanceof Error?e.message:"preview_failed")}}
 async function apply(){if(!operator||!preview)return;try{await json("/api/admin/mail/dead-letters/retry/apply",{method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({preview_token:preview.preview_token})});setPreview(null);await load()}catch(e){setError(e instanceof Error?e.message:"retry_failed")}}
 return <main style={{maxWidth:1180,margin:"32px auto",padding:24,display:"grid",gap:16}}>
  <header style={{display:"flex",justifyContent:"space-between",alignItems:"center"}}><div><h1 style={{margin:0}}>Internet Mail operations</h1>{me&&<small>{me.email} · {me.role}</small>}</div><button onClick={()=>void load()}>Обновить</button></header>
  {error&&<div role="alert" style={{...card,borderColor:"#b00"}}>{error}</div>}
  {health&&<>
   <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(220px,1fr))",gap:12}}>
    <section style={card}><strong>Outbound queue</strong><div>ready {health.ready} · leased {health.leased} · retry {health.retry}</div><div>submitted {health.submitted} · dead {health.dead}</div><div>oldest queued {age(health.oldest_queued_seconds)} · submitted {age(health.oldest_submitted_seconds)}</div></section>
    <section style={card}><strong>24 hours</strong><div>delivered {health.delivered_24h} · bounced {health.bounced_24h}</div><div>dead {health.dead_24h} · inbound {health.inbound_24h}</div><div>replay guard {health.replay_guard_rows} · inbound processing {health.inbound_receipts_processing}</div></section>
    <section style={{...card,borderColor:health.active_suppressions||health.cooling_domains?"#f0c36d":"#e2e2e2"}}><strong>Deliverability protection</strong><div>active suppressions {health.active_suppressions}</div><div>cooling domains {health.cooling_domains}</div><div>active external aliases {health.active_aliases}</div></section>
    <section style={{...card,borderColor:health.dns?.ready&&!health.dns?.drift?"#a8d5a2":"#f0c36d"}}><strong>DNS readiness</strong>{health.dns?<><div>{health.dns.domain} · selector {health.dns.selector}</div><div>MX {health.dns.mx?"✓":"✗"} · SPF {health.dns.spf?"✓":"✗"} · DMARC {health.dns.dmarc?"✓":"✗"} · DKIM {health.dns.dkim?"✓":"✗"}</div><div>ready {health.dns.ready?"YES":"NO"} · drift {health.dns.drift?"YES":"NO"}</div>{health.dns.reasons?.length?<div>{health.dns.reasons.join(", ")}</div>:null}<small>{new Date(health.dns.checked_at).toLocaleString()}</small></>:<div>Нет DNS snapshot.</div>}</section>
   </div>
  </>}
  <section style={card}><h2 style={{marginTop:0}}>Dead letters</h2><p style={{marginTop:0}}>Адреса получателей и содержимое писем здесь намеренно не показываются. Повтор разрешён только для transport/transient ошибок.</p><div style={{display:"grid",gap:8}}>{dead.length===0?<div>Очередь пуста.</div>:dead.map(item=><div key={item.delivery_id} style={{display:"grid",gridTemplateColumns:"1fr auto",gap:12,padding:12,border:"1px solid #ddd",borderRadius:10}}><div><strong>Delivery #{item.delivery_id}</strong><div>message #{item.message_id} · mailbox #{item.sender_mailbox_id} · attempts {item.attempts}</div><div>{item.error_code||"без кода"} · {item.retryable?"retryable":"blocked"}</div><small>{new Date(item.updated_at).toLocaleString()}</small></div>{operator&&item.retryable?<button onClick={()=>void makePreview(item)}>Retry preview</button>:null}</div>)}</div></section>
  {preview&&<section style={{...card,borderColor:"#d99"}}><h2 style={{marginTop:0}}>Подтвердите повтор</h2><div>Delivery #{preview.delivery.delivery_id} · previous error {preview.delivery.error_code}</div><div>Preview истекает {new Date(preview.expires_at).toLocaleString()}</div><p>Повтор не обходит suppression и ограничен тремя ручными перезапусками.</p><button onClick={()=>void apply()}>Apply retry</button> <button onClick={()=>setPreview(null)}>Отмена</button></section>}
 </main>
}
