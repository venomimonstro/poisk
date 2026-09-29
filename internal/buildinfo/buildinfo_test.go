package buildinfo

import "testing"

func TestValidateCandidateRejectsDevBuild(t *testing.T){
	oldCommit,oldRelease:=GitCommit,ReleaseVersion;defer func(){GitCommit,ReleaseVersion=oldCommit,oldRelease}()
	GitCommit="dev";ReleaseVersion="dev"
	if err:=ValidateCandidate("0123456789012345678901234567890123456789","v1");err==nil{t.Fatal("dev build unexpectedly accepted")}
}

func TestValidateCandidateRequiresExactBuildIdentity(t *testing.T){
	oldCommit,oldRelease:=GitCommit,ReleaseVersion;defer func(){GitCommit,ReleaseVersion=oldCommit,oldRelease}()
	GitCommit="0123456789abcdef0123456789abcdef01234567";ReleaseVersion="v1.2.3"
	if err:=ValidateCandidate(GitCommit,ReleaseVersion);err!=nil{t.Fatalf("exact candidate rejected: %v",err)}
	if err:=ValidateCandidate("1123456789abcdef0123456789abcdef01234567",ReleaseVersion);err==nil{t.Fatal("wrong commit accepted")}
	if err:=ValidateCandidate(GitCommit,"v1.2.4");err==nil{t.Fatal("wrong release accepted")}
}
