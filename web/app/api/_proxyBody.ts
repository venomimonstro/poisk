import { NextRequest } from "next/server";

export class ProxyBodyTooLarge extends Error {
  constructor(){super("body_too_large")}
}

export async function readLimitedProxyBody(request:NextRequest,maxBytes:number):Promise<Uint8Array>{
  const declared=Number(request.headers.get("content-length")||0);
  if(Number.isFinite(declared)&&declared>maxBytes)throw new ProxyBodyTooLarge();
  if(!request.body)return new Uint8Array();
  const reader=request.body.getReader();const chunks:Uint8Array[]=[];let total=0;
  try{
    for(;;){
      const{done,value}=await reader.read();if(done)break;if(!value)continue;
      total+=value.byteLength;if(total>maxBytes)throw new ProxyBodyTooLarge();chunks.push(value);
    }
  }finally{reader.releaseLock()}
  const out=new Uint8Array(total);let offset=0;for(const chunk of chunks){out.set(chunk,offset);offset+=chunk.byteLength}return out;
}
