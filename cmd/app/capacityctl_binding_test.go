package main

import (
	"os"
	"testing"
)

func TestCapacityBenchmarkCommitRequiresExactSHA(t *testing.T){
	old,had:=os.LookupEnv("READINESS_GIT_SHA")
	t.Cleanup(func(){if had{_ = os.Setenv("READINESS_GIT_SHA",old)}else{_ = os.Unsetenv("READINESS_GIT_SHA")}})
	for _,value:=range []string{"","abc","012345678901234567890123456789012345678g"}{
		_ = os.Setenv("READINESS_GIT_SHA",value)
		if _,err:=capacityBenchmarkCommit();err==nil{t.Fatalf("expected invalid commit %q to fail",value)}
	}
	const valid="0123456789abcdef0123456789abcdef01234567"
	_ = os.Setenv("READINESS_GIT_SHA",valid)
	got,err:=capacityBenchmarkCommit();if err!=nil{t.Fatal(err)}
	if got!=valid{t.Fatalf("commit=%q",got)}
}

func TestCapacityBenchmarkReleaseVersionRequired(t *testing.T){
	old,had:=os.LookupEnv("RELEASE_VERSION")
	t.Cleanup(func(){if had{_ = os.Setenv("RELEASE_VERSION",old)}else{_ = os.Unsetenv("RELEASE_VERSION")}})
	for _,value:=range []string{"","dev",string(make([]byte,129)),"candidate\nnext"}{
		_ = os.Setenv("RELEASE_VERSION",value)
		if _,err:=capacityBenchmarkReleaseVersion();err==nil{t.Fatalf("expected invalid release %q to fail",value)}
	}
	const valid="2026.09.27-rc1"
	_ = os.Setenv("RELEASE_VERSION",valid)
	got,err:=capacityBenchmarkReleaseVersion();if err!=nil{t.Fatal(err)}
	if got!=valid{t.Fatalf("release=%q",got)}
}
