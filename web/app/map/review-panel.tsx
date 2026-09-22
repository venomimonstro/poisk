"use client";

import { FormEvent,useEffect,useState } from "react";

type Stats={review_count:number;average_rating:number};
type Reply={reply_id:number;body:string;version:number;updated_at:string};
type Review={review_id:number;place_id:number;rating:number;body:string;version:number;created_at:string;updated_at:string;owner_reply?:Reply};
type Permissions={my_review_id?:number;can_reply:boolean};

async function json<T>(url:string,init?:RequestInit):Promise<T>{const r=await fetch(url,{...init,cache:"no-store",credentials:"same-origin"});const data=await r.json().catch(()=>({}));if(!r.ok)throw new Error(data?.error||`http_${r.status}`);return data as T}
async function freshCSRF(){return (await json<{csrf_token:string}>("/api/account/csrf")).csrf_token}
async function ensureOK(response:Response){if(response.ok)return;const data=await response.json().catch(()=>({}));throw new Error(data?.error||`http_${response.status}`)}

export default function ReviewPanel({placeID}:{placeID:number}){
 const[stats,setStats]=useState<Stats>({review_count:0,average_rating:0});const[reviews,setReviews]=useState<Review[]>([]);const[permissions,setPermissions]=useState<Permissions|null>(null);const[rating,setRating]=useState(5);const[body,setBody]=useState("");const[replyTo,setReplyTo]=useState<number|null>(null);const[replyBody,setReplyBody]=useState("");const[message,setMessage]=useState("");
 async function load(){const[s,list]=await Promise.all([json<Stats>(`/api/reviews/places/${placeID}/rating`),json<{reviews:Review[]}>(`/api/reviews/places/${placeID}/reviews?limit=20`)]);setStats(s);setReviews(list.reviews);try{await json("/api/account/me");setPermissions(await json<Permissions>(`/api/reviews/places/${placeID}/permissions`))}catch{setPermissions(null)}}
 useEffect(()=>{setBody("");setMessage("");void load().catch(()=>setMessage("Отзывы временно недоступны"))},[placeID]);
 async function submit(e:FormEvent){e.preventDefault();if(!permissions){setMessage("Войдите в аккаунт, чтобы оставить отзыв");return}try{const csrf=await freshCSRF();await json(`/api/reviews/places/${placeID}/review`,{method:"PUT",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({rating,body})});setBody("");setMessage("Отзыв сохранён");await load()}catch(err){setMessage(err instanceof Error?err.message:"Не удалось сохранить отзыв")}}
 async function remove(){if(!permissions?.my_review_id)return;try{const csrf=await freshCSRF();const response=await fetch(`/api/reviews/reviews/${permissions.my_review_id}`,{method:"DELETE",credentials:"same-origin",headers:{"X-CSRF-Token":csrf}});await ensureOK(response);setMessage("Отзыв удалён");await load()}catch(err){setMessage(err instanceof Error?err.message:"Не удалось удалить отзыв")}}
 async function report(reviewID:number){if(!permissions){setMessage("Войдите в аккаунт, чтобы пожаловаться");return}try{const csrf=await freshCSRF();await json(`/api/reviews/reviews/${reviewID}/report`,{method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({reason:"OTHER",details:"Жалоба пользователя из карточки организации"})});setMessage("Жалоба отправлена")}catch(err){setMessage(err instanceof Error?err.message:"Не удалось отправить жалобу")}}
 async function reply(reviewID:number){if(!permissions?.can_reply)return;try{const csrf=await freshCSRF();await json(`/api/reviews/reviews/${reviewID}/reply`,{method:"PUT",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({body:replyBody})});setReplyBody("");setReplyTo(null);setMessage("Ответ владельца сохранён");await load()}catch(err){setMessage(err instanceof Error?err.message:"Не удалось сохранить ответ")}}
 return <section style={{display:"grid",gap:12,marginTop:14,borderTop:"1px solid #eee",paddingTop:12}}>
  <div><strong>Отзывы</strong> <span style={{color:"#666"}}>{stats.review_count?`${stats.average_rating.toFixed(1)} ★ · ${stats.review_count}`:"пока нет"}</span></div>
  <form onSubmit={submit} style={{display:"grid",gap:7}}>
   <div style={{display:"flex",gap:4}}>{[1,2,3,4,5].map(v=><button key={v} type="button" aria-label={`${v} из 5`} onClick={()=>setRating(v)} style={{border:0,background:"transparent",cursor:"pointer",fontSize:20,color:v<=rating?"#d99b00":"#bbb"}}>★</button>)}</div>
   <textarea value={body} onChange={e=>setBody(e.target.value)} minLength={10} maxLength={4000} placeholder={permissions?"Расскажите о своём опыте":"Войдите, чтобы оставить отзыв"} disabled={!permissions} style={{minHeight:76,resize:"vertical",padding:8,border:"1px solid #ddd",borderRadius:8}}/>
   <div style={{display:"flex",gap:8}}><button type="submit" disabled={!permissions||body.trim().length<10}>Сохранить отзыв</button>{permissions?.my_review_id&&<button type="button" onClick={()=>void remove()}>Удалить мой</button>}</div>
  </form>
  {message&&<div role="status" style={{fontSize:13,color:"#555"}}>{message}</div>}
  <div style={{display:"grid",gap:10,maxHeight:320,overflow:"auto"}}>{reviews.map(r=><article key={r.review_id} style={{padding:10,border:"1px solid #eee",borderRadius:9}}><div aria-label={`${r.rating} из 5`} style={{color:"#b57a00"}}>{"★".repeat(r.rating)}{"☆".repeat(5-r.rating)}</div><div style={{whiteSpace:"pre-wrap",overflowWrap:"anywhere"}}>{r.body}</div><small style={{color:"#777"}}>{new Date(r.updated_at).toLocaleDateString()}</small>{r.owner_reply&&<div style={{marginTop:8,padding:8,background:"#f6f6f6",borderRadius:7}}><strong>Ответ компании</strong><div style={{whiteSpace:"pre-wrap",overflowWrap:"anywhere"}}>{r.owner_reply.body}</div></div>}<div style={{display:"flex",gap:8,marginTop:7}}>{permissions?.my_review_id!==r.review_id&&<button type="button" onClick={()=>void report(r.review_id)}>Пожаловаться</button>}{permissions?.can_reply&&<button type="button" onClick={()=>{setReplyTo(r.review_id);setReplyBody(r.owner_reply?.body||"")}}>Ответить как владелец</button>}</div>{replyTo===r.review_id&&permissions?.can_reply&&<div style={{display:"grid",gap:6,marginTop:8}}><textarea value={replyBody} onChange={e=>setReplyBody(e.target.value)} minLength={2} maxLength={3000}/><button type="button" disabled={replyBody.trim().length<2} onClick={()=>void reply(r.review_id)}>Сохранить ответ</button></div>}</article>)}</div>
 </section>
}
