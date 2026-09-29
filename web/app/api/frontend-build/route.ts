import { NextResponse } from "next/server";

const gitCommit=process.env.NEXT_PUBLIC_BUILD_SHA||"dev";
const releaseVersion=process.env.NEXT_PUBLIC_RELEASE_VERSION||"dev";

export async function GET(){
  return NextResponse.json(
    {git_commit:gitCommit,release_version:releaseVersion},
    {status:200,headers:{"Cache-Control":"no-store","X-Content-Type-Options":"nosniff"}},
  );
}
