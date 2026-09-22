import { NextRequest, NextResponse } from "next/server";

const internalBase=(process.env.API_INTERNAL_BASE_URL||"http://localhost:8080").replace(/\/$/,"");
const folders=new Set(["INBOX","SENT","DRAFTS","TRASH","SPAM"]);
const itemActions=new Set(["read","star","trash","restore","spam"]);

function allowed(path:string[],method:string){
  if(path.length===1&&path[0]==="me")return method==="GET";
  if(path.length===2&&path[0]==="folders"&&folders.has(path[1].toUpperCase()))return method==="GET";
  if(path.length===2&&path[0]==="items"&&/^\d+$/.test(path[1]))return method==="GET";
  if(path.length===3&&path[0]==="items"&&/^\d+$/.test(path[1])&&path[2]==="attachments")return method==="GET";
  if(path.length===1&&path[0]==="search")return method==="GET";
  if(path.length===2&&path[0]==="attachments"&&/^\d+$/.test(path[1]))return method==="GET";
  if(path.length===1&&path[0]==="drafts")return method==="POST";
  if(path.length===2&&path[0]==="drafts"&&/^\d+$/.test(path[1]))return method==="PUT";
  if(path.length===3&&path[0]==="drafts"&&/^\d+$/.test(path[1])&&path[2]==="send")return method==="POST";
  if(path.length===3&&path[0]==="drafts"&&/^\d+$/.test(path[1])&&path[2]==="attachments")return method==="POST";
  if(path.length===4&&path[0]==="drafts"&&/^\d+$/.test(path[1])&&path[2]==="attachments"&&/^\d+$/.test(path[3]))return method==="DELETE";
  if(path.length===3&&path[0]==="items"&&/^\d+$/.test(path[1])&&itemActions.has(path[2]))return method==="POST";
  return false;
}

async function proxy(request:NextRequest,path:string[]){
  if(!allowed(path,request.method))return NextResponse.json({error:"not_found"},{status:404});
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),30000);
  try{
    const headers=new Headers({Accept:request.headers.get("accept")||"application/json"});
    for(const name of ["cookie","x-csrf-token","content-type"]){const value=request.headers.get(name);if(value)headers.set(name,value)}
    let body:BodyInit|undefined;
    if(!["GET","HEAD","DELETE"].includes(request.method)){
      const type=request.headers.get("content-type")||"";
      body=type.toLowerCase().startsWith("multipart/form-data")?await request.arrayBuffer():await request.text();
    }
    const upstream=await fetch(`${internalBase}/api/mail/${path.join("/")}${request.nextUrl.search}`,{method:request.method,headers,body,cache:"no-store",redirect:"manual",signal:controller.signal});
    const response=new NextResponse(upstream.body,{status:upstream.status});
    for(const name of ["content-type","content-disposition","content-length","cache-control","x-content-type-options","content-security-policy"]){const value=upstream.headers.get(name);if(value)response.headers.set(name,value)}
    return response;
  }catch{return NextResponse.json({error:"mail_backend_error"},{status:502})}finally{clearTimeout(timer)}
}

export async function GET(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function POST(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function PUT(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function DELETE(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
