"use client";

import { FormEvent,useEffect,useState } from "react";

type AddressState={internet_enabled:boolean;domain?:string;address?:string|null};
type Delivery={delivery_id:number;address:string;recipient_type:string;status:string;attempts:number;remote_queue_id?:string;last_error_code?:string;last_error_detail?:string;submitted_at?:string;delivered_at?:string;bounced_at?:string;updated_at:string};

async function api<T>(url:string,init?:RequestInit):Promise<T>{const r=await fetch(url,{...init,credentials:"same-origin",cache:"no-store"});const data=r.status===204?null:await r.json().catch(()=>({}));if(!r.ok)throw new Error(data?.error||`http_${r.status}`);return data as T}

export function InternetAddressPanel({csrf}:{csrf:string}){
 const[state,setState]=useState<AddressState|null>(null);const[local,setLocal]=useState("");const[busy,setBusy]=useState(false);const[error,setError]=useState("");
 async function load(){try{setState(await api<AddressState>("/api/mail/internet/address"));setError("")}catch(e){setError(e instanceof Error?e.message:"internet_mail_unavailable")}}
 useEffect(()=>{void load()},[]);
 async function claim(e:FormEvent){e.preventDefault();const value=local.trim().toLowerCase();if(!value)return;setBusy(true);try{await api("/api/mail/internet/address",{method:"PUT",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({local_part:value})});setLocal("");await load()}catch(e){setError(e instanceof Error?e.message:"address_failed")}finally{setBusy(false)}}
 if(!state)return <div style={muted}>{error||"Internet-почта…"}</div>;
 if(!state.internet_enabled)return <div style={muted}>Internet-почта выключена администратором.</div>;
 if(state.address)return <div style={{marginTop:22,paddingTop:16,borderTop:"1px solid #eee"}}><div style={caption}>Ваш адрес в интернете</div><strong style={{fontSize:13,wordBreak:"break-all"}}>{state.address}</strong><div style={muted}>Приём и отправка через защищённый почтовый шлюз.</div></div>;
 return <form onSubmit={claim} style={{marginTop:22,paddingTop:16,borderTop:"1px solid #eee",display:"grid",gap:7}}><div style={caption}>Создать внешний адрес</div><div style={{display:"flex",alignItems:"center",gap:4}}><input value={local} onChange={e=>setLocal(e.target.value)} pattern="[a-z0-9][a-z0-9._+\-]{0,63}" maxLength={64} placeholder="name" style={{minWidth:0,width:"100%",padding:"7px 8px",border:"1px solid #d1d5db",borderRadius:8}}/><span style={{fontSize:12}}>@{state.domain}</span></div><button disabled={busy||!local.trim()} style={smallBtn}>{busy?"…":"Создать один раз"}</button>{error&&<div style={{fontSize:11,color:"#991b1b"}}>{error}</div>}</form>
}

export function DeliveryStatusPanel({messageID}:{messageID:number}){
 const[items,setItems]=useState<Delivery[]>([]);const[error,setError]=useState("");
 useEffect(()=>{let cancelled=false;void api<{deliveries:Delivery[]}>(`/api/mail/internet/messages/${messageID}/deliveries`).then(v=>{if(!cancelled){setItems(v.deliveries||[]);setError("")}}).catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:"delivery_status_unavailable")});return()=>{cancelled=true}},[messageID]);
 if(error)return <div style={{fontSize:12,color:"#991b1b",marginTop:12}}>Статус внешней доставки недоступен.</div>;
 if(items.length===0)return null;
 return <section style={{marginTop:16,padding:"14px 0",borderTop:"1px solid #eee"}}><strong>Доставка в интернет</strong><div style={{display:"grid",gap:7,marginTop:8}}>{items.map(d=><div key={d.delivery_id} style={{display:"flex",justifyContent:"space-between",gap:12,fontSize:13}}><span style={{overflow:"hidden",textOverflow:"ellipsis"}}>{d.address}</span><span title={d.last_error_detail||d.last_error_code||""} style={{fontWeight:700,color=statusColor(d.status)}}>{statusLabel(d.status)}</span></div>)}</div></section>
}

function statusLabel(v:string){switch(v){case"READY":return"В очереди";case"LEASED":return"Отправляется";case"RETRY":return"Повтор";case"SUBMITTED":return"Передано MTA";case"DELIVERED":return"Доставлено";case"BOUNCED":return"Не доставлено";case"DEAD":return"Ошибка";default:return v}}
function statusColor(v:string){if(v==="DELIVERED")return"#166534";if(v==="BOUNCED"||v==="DEAD")return"#991b1b";if(v==="RETRY")return"#92400e";return"#475569"}
const muted:React.CSSProperties={fontSize:11,color:"#6b7280",marginTop:5,lineHeight:1.4};const caption:React.CSSProperties={fontSize:11,color:"#6b7280",marginBottom:5};const smallBtn:React.CSSProperties={border:"1px solid #d1d5db",borderRadius:8,padding:"7px 9px",background:"white",cursor:"pointer",font:"inherit",fontSize:12};
