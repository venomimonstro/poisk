package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestHashReadinessArtifact(t *testing.T){
	dir:=t.TempDir();path:=filepath.Join(dir,"artifact.log");content:=[]byte("verified readiness artifact\n")
	if err:=os.WriteFile(path,content,0o600);err!=nil{t.Fatal(err)}
	ref,digest,err:=hashReadinessArtifact(path);if err!=nil{t.Fatal(err)}
	want:=sha256.Sum256(content);if ref!=path{t.Fatalf("ref=%q want=%q",ref,path)};if digest!=hex.EncodeToString(want[:]){t.Fatalf("digest=%s",digest)}
}

func TestHashReadinessArtifactRejectsEmptyAndSymlink(t *testing.T){
	dir:=t.TempDir();empty:=filepath.Join(dir,"empty.log");if err:=os.WriteFile(empty,nil,0o600);err!=nil{t.Fatal(err)}
	if _,_,err:=hashReadinessArtifact(empty);err==nil{t.Fatal("expected empty artifact rejection")}
	target:=filepath.Join(dir,"target.log");if err:=os.WriteFile(target,[]byte("ok"),0o600);err!=nil{t.Fatal(err)}
	link:=filepath.Join(dir,"link.log");if err:=os.Symlink(target,link);err!=nil{t.Skipf("symlink unavailable: %v",err)}
	if _,_,err:=hashReadinessArtifact(link);err==nil{t.Fatal("expected symlink rejection")}
}
