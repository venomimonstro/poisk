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
