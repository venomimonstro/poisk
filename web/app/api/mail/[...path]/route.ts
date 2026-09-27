import { NextRequest, NextResponse } from "next/server";

const internalBase=(process.env.API_INTERNAL_BASE_URL||"http://localhost:8080").replace(/\/$/,"");
const folders=new Set(["INBOX","SENT","DRAFTS","TRASH","SPAM"]);
const itemActions=new Set(["read","star","trash","restore","spam"]);
const maxJSONBody=1<<20;
const maxMultipartBody=(25<<20)+(1<<20);

function allowed(path:string[],method:string){
  if(path.length===1&&path[0]==="me")return method==="GET";
  if(path.length===2&&path[0]==="folders"&&folders.has(path[1].toUpperCase()))return method==="GET";
  if(path.length===2&&path[0]==="items"&&/^\d+$/.test(path[1]))return method==="GET";
  if(path.length===3&&path[0]==="items"&&/^\d+$/.test(path[1])&&path[2]==="attachments")return method==="GET";
  if(path.length===1&&path[0]==="search")return method==="GET";
  if(path.length===2&&path[0]==="attachments"&&/^\d+$/.test(path[1]))return method==="GET";
  if(path.length===1&&path[0]==="drafts")return method==="POST";
  if(path.length===2&&path[0]==="drafts"&&/^\d+$/.test(path[1]))return method==="GET"||method==="PUT";
  if(path.length===3&&path[0]==="drafts"&&/^\d+$/.test(path[1])&&path[2]==="send")return method==="POST";
  if(path.length===3&&path[0]==="drafts"&&/^\d+$/.test(path[1])&&path[2]==="attachments")return method==="POST";
  if(path.length===4&&path[0]==="drafts"&&/^\d+$/.test(path[1])&&path[2]==="attachments"&&/^\d+$/.test(path[3]))return method==="DELETE";
  if(path.length===3&&path[0]==="items"&&/^\d+$/.test(path[1])&&itemActions.has(path[2]))return method==="POST";
  if(path.length===2&&path[0]==="internet"&&path[1]==="address")return method==="GET"||method==="PUT";
  if(path.length===4&&path[0]==="internet"&&path[1]==="messages"&&/^\d+$/.test(path[2])&&path[3]==="deliveries")return method==="GET";
  return false;
}

async function readLimitedBody(request:NextRequest,max:number):Promise<Uint8Array>{
  const declared=Number(request.headers.get("content-length")||0);if(Number.isFinite(declared)&&declared>max)throw new Error("body_too_large");
  if(!request.body)return new Uint8Array();
  const reader=request.body.getReader();const chunks:Uint8Array[]=[];let total=0;
  try{for(;;){const{done,value}=await reader.read();if(done)break;if(!value)continue;total+=value.byteLength;if(total>max)throw new Error("body_too_large");chunks.push(value)}}finally{reader.releaseLock()}
  const body=new Uint8Array(total);let offset=0;for(const chunk of chunks){body.set(chunk,offset);offset+=chunk.byteLength}return body;
}

async function proxy(request:NextRequest,path:string[]){
  if(!allowed(path,request.method))return NextResponse.json({error:"not_found"},{status:404});
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),30000);
  try{
    const headers=new Headers({Accept:request.headers.get("accept")||"application/json"});
    for(const name of ["cookie","x-csrf-token","content-type"]){const value=request.headers.get(name);if(value)headers.set(name,value)}
    let body:BodyInit|undefined;
    if(!["GET","HEAD","DELETE"].includes(request.method)){
      const type=(request.headers.get("content-type")||"").toLowerCase();const max=type.startsWith("multipart/form-data")?maxMultipartBody:maxJSONBody;
      body=await readLimitedBody(request,max);
    }
    const upstream=await fetch(`${internalBase}/api/mail/${path.join("/")}${request.nextUrl.search}`,{method:request.method,headers,body,cache:"no-store",redirect:"manual",signal:controller.signal});
    const response=new NextResponse(upstream.body,{status:upstream.status});
    for(const name of ["content-type","content-disposition","content-length","cache-control","x-content-type-options","content-security-policy"]){const value=upstream.headers.get(name);if(value)response.headers.set(name,value)}
    return response;
  }catch(error){if(error instanceof Error&&error.message==="body_too_large")return NextResponse.json({error:"body_too_large"},{status:413});return NextResponse.json({error:"mail_backend_error"},{status:502})}finally{clearTimeout(timer)}
}

export async function GET(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function POST(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function PUT(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
export async function DELETE(request:NextRequest,context:{params:Promise<{path:string[]}>}){return proxy(request,(await context.params).path||[])}
