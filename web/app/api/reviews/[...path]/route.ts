import { NextRequest, NextResponse } from "next/server";

const internalBase=(process.env.API_INTERNAL_BASE_URL||"http://localhost:8080").replace(/\/$/,"");

function allowed(path:string[],method:string){
  if(path.length===3&&path[0]==="places"&&/^\d+$/.test(path[1])&&path[2]==="reviews")return method==="GET";
  if(path.length===3&&path[0]==="places"&&/^\d+$/.test(path[1])&&path[2]==="rating")return method==="GET";
  if(path.length===3&&path[0]==="places"&&/^\d+$/.test(path[1])&&path[2]==="permissions")return method==="GET";
  if(path.length===3&&path[0]==="places"&&/^\d+$/.test(path[1])&&path[2]==="review")return method==="PUT";
  if(path.length===2&&path[0]==="reviews"&&/^\d+$/.test(path[1]))return method==="DELETE";
  if(path.length===3&&path[0]==="reviews"&&/^\d+$/.test(path[1])&&path[2]==="report")return method==="POST";
  if(path.length===3&&path[0]==="reviews"&&/^\d+$/.test(path[1])&&path[2]==="reply")return method==="PUT";
  return false;
}

async function proxy(request:NextRequest,path:string[]){
  if(!allowed(path,request.method))return NextResponse.json({error:"not_found"},{status:404});
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),6000);
  try{
    const headers=new Headers({Accept:"application/json"});
    const cookie=request.headers.get("cookie");const csrf=request.headers.get("x-csrf-token");const contentType=request.headers.get("content-type");
    if(cookie)headers.set("Cookie",cookie);if(csrf)headers.set("X-CSRF-Token",csrf);if(contentType)headers.set("Content-Type",contentType);
    const suffix=request.nextUrl.search||"";const body=["GET","HEAD"].includes(request.method)?undefined:await request.text();
    const upstream=await fetch(`${internalBase}/api/reviews/${path.join("/")}${suffix}`,{method:request.method,headers,body,cache:"no-store",redirect:"manual",signal:controller.signal});
    const text=await upstream.text();return new NextResponse(text||null,{status:upstream.status,headers:{"Content-Type":upstream.headers.get("Content-Type")||"application/json; charset=utf-8","Cache-Control":"no-store","X-Content-Type-Options":"nosniff"}});
  }catch{return NextResponse.json({error:"reviews_backend_error"},{status:502})}finally{clearTimeout(timer)}
}

export async function GET(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function POST(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function PUT(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function DELETE(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
