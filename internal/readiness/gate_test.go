package readiness

import (
	"context"
	"testing"
)

func TestDiffVersionsDetectsMissingAndUnexpected(t *testing.T){
	missing,unexpected:=diffVersions([]int64{1,2,4,5},[]int64{1,3,4,5})
	if len(missing)!=1||missing[0]!=2{t.Fatalf("missing=%v",missing)}
	if len(unexpected)!=1||unexpected[0]!=3{t.Fatalf("unexpected=%v",unexpected)}
}

func TestGateRejectsInvalidCommitBeforeDatabaseAccess(t *testing.T){
	_,err:=(Gate{ExpectedVersions:[]int64{1},GitCommit:"not-a-commit"}).Evaluate(context.Background())
	if err==nil{t.Fatal("expected invalid readiness configuration")}
}

func TestEvidenceKindSetIsClosed(t *testing.T){
	for _,kind:=range []string{
		"BUILD_UNIT","INTEGRATION","FRESH_INSTALL","UPGRADE","BROWSER_SMOKE",
		"SECURITY_REGRESSION","EDGE_TLS_PROXY","MTA_FLOW",
	}{
		if _,ok:=evidenceKinds[kind];!ok{t.Fatalf("missing kind %s",kind)}
	}
	if _,ok:=evidenceKinds["ARBITRARY"];ok{t.Fatal("unexpected arbitrary evidence kind")}
}
