"use client";

import { useEffect, useState } from "react";
import SupportPanels from "../SupportPanels";

type AdminMe={admin_id:number;email:string;role:string;expires_at:string};

export default function AdminSupportPage(){
  const [me,setMe]=useState<AdminMe|null>(null);const [error,setError]=useState("");
  useEffect(()=>{void fetch("/api/admin/me",{cache:"no-store",credentials:"same-origin"}).then(async r=>{const data=await r.json().catch(()=>({}));if(!r.ok)throw new Error(data?.error||"unauthorized");setMe(data as AdminMe)}).catch(e=>setError(e instanceof Error?e.message:"unauthorized"))},[]);
  if(error)return <main style={{maxWidth:900,margin:"50px auto",padding:24}}><a href="/admin">← Admin</a><h1>Support diagnostics</h1><p>{error}. Войдите через /admin.</p></main>;
  if(!me)return <main style={{maxWidth:900,margin:"50px auto",padding:24}}>Загрузка…</main>;
  return <main style={{maxWidth:1180,margin:"32px auto",padding:24,display:"grid",gap:16}}><header style={{display:"flex",justifyContent:"space-between",alignItems:"center"}}><div><a href="/admin">← Admin</a><h1 style={{margin:"8px 0 0"}}>Support diagnostics</h1><small>{me.email} · {me.role}</small></div></header><SupportPanels role={me.role}/></main>
}
