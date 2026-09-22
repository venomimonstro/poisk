"use client";

import { FormEvent,useState } from "react";

export type MapSearchResult={id:number;name:string;address:string;city_key?:string;category_key?:string;phone?:string;website?:string;latitude?:number;longitude?:number;quality_score:number;source_count:number};

export default function MapSearchPanel({onSelect}:{onSelect:(item:MapSearchResult)=>void}){
 const[q,setQ]=useState("");const[items,setItems]=useState<MapSearchResult[]>([]);const[status,setStatus]=useState("");
 async function search(e?:FormEvent){e?.preventDefault();const query=q.trim();if(!query){setItems([]);setStatus("");return}setStatus("Ищем…");try{const r=await fetch(`/api/geo/search?q=${encodeURIComponent(query)}&limit=20`,{cache:"no-store",headers:{Accept:"application/json"}});const data=await r.json().catch(()=>({}));if(!r.ok)throw new Error(data?.error||"geo_unavailable");setItems(data.results||[]);setStatus(data.results?.length?`${data.results.length} результатов`:"Ничего не найдено")}catch{setStatus("Поиск временно недоступен")}}
 return <aside style={{position:"absolute",zIndex:4,top:70,left:14,width:"min(390px,calc(100vw - 28px))",maxHeight:"calc(100vh - 90px)",display:"grid",gap:8,padding:12,borderRadius:14,background:"rgba(255,255,255,.97)",boxShadow:"0 8px 28px rgba(0,0,0,.16)",overflow:"hidden"}} aria-label="Поиск организаций">
  <form onSubmit={search} style={{display:"flex",gap:7}}><input value={q} onChange={e=>setQ(e.target.value)} maxLength={256} placeholder="Организация, услуга или адрес" style={{flex:1,minWidth:0,padding:"11px 12px",border:"1px solid #ccc",borderRadius:10,font:"inherit"}}/><button type="submit" style={{padding:"0 14px",border:0,borderRadius:10,fontWeight:700,cursor:"pointer"}}>Найти</button></form>
  {status&&<small style={{color:"#666"}}>{status}</small>}
  {items.length>0&&<div style={{display:"grid",gap:5,overflow:"auto",maxHeight:"calc(100vh - 170px)"}}>{items.map(item=><button key={item.id} type="button" onClick={()=>onSelect(item)} style={{textAlign:"left",padding:10,border:"1px solid #eee",borderRadius:10,background:"#fff",cursor:"pointer"}}><strong>{item.name}</strong>{item.address&&<div style={{fontSize:13,color:"#666",marginTop:3}}>{item.address}</div>}<small>{item.category_key||"Организация"}</small></button>)}</div>}
 </aside>
}
