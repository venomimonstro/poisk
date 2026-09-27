package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeRecoveryFixture(t *testing.T,schema int64)string{
	t.Helper();dir:=t.TempDir();files:=map[string][]byte{
		"postgres.dump":[]byte("postgres dump bytes"),
		"postgres.list":[]byte("restore listing"),
		"mail-db-blobs.list":[]byte{},
		"mail-blobs.list":[]byte{},
		"mail-blobs.tar.gz":[]byte("empty-ish archive fixture"),
		"docker-compose.yml":[]byte("services: {}\n"),
		"nginx-default.conf":[]byte("server {}\n"),
	}
	names:=make([]string,0,len(files));for name,data:=range files{if err:=os.WriteFile(filepath.Join(dir,name),data,0o600);err!=nil{t.Fatal(err)};names=append(names,name)};sort.Strings(names)
	var sums strings.Builder;for _,name:=range names{sum:=sha256.Sum256(files[name]);fmt.Fprintf(&sums,"%s  %s\n",hex.EncodeToString(sum[:]),name)}
	manifest:=fmt.Sprintf("backup_format=poisk-v2\ncreated_at=20260927T120000Z\ndatabase=poisk\ndatabase_schema=%d\nsecrets_included=false\nmail_blobs_included=true\nquiesced=true\n",schema)
	if err:=os.WriteFile(filepath.Join(dir,"SHA256SUMS"),[]byte(sums.String()),0o600);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(dir,"MANIFEST"),[]byte(manifest),0o600);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(dir,"COMPLETE"),[]byte("ok\n"),0o600);err!=nil{t.Fatal(err)}
	return dir
}

func TestInspectBackupArtifact(t *testing.T){
	dir:=writeRecoveryFixture(t,71);artifact,err:=inspectBackupArtifact(dir,71);if err!=nil{t.Fatal(err)}
	if artifact.Schema!=71||artifact.Bytes<=0||len(artifact.SHA256)!=64||artifact.Ref!=dir{t.Fatalf("artifact=%+v",artifact)}
}

func TestInspectBackupArtifactRejectsTampering(t *testing.T){
	dir:=writeRecoveryFixture(t,71);if err:=os.WriteFile(filepath.Join(dir,"postgres.dump"),[]byte("tampered"),0o600);err!=nil{t.Fatal(err)}
	if _,err:=inspectBackupArtifact(dir,71);err==nil{t.Fatal("expected checksum mismatch")}
}

func TestInspectBackupArtifactRejectsWrongSchema(t *testing.T){
	dir:=writeRecoveryFixture(t,70);if _,err:=inspectBackupArtifact(dir,71);err==nil{t.Fatal("expected schema mismatch")}
}
