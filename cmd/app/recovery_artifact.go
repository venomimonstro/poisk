package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type backupArtifact struct{Ref string;SHA256 string;Bytes int64;Schema int64}

var recoveryDigestPattern=regexp.MustCompile(`^[0-9a-f]{64}$`)
var recoveryChecksumFiles=map[string]struct{}{
	"postgres.dump":{},"postgres.list":{},"mail-db-blobs.list":{},"mail-blobs.list":{},"mail-blobs.tar.gz":{},"docker-compose.yml":{},"nginx-default.conf":{},
}

func inspectBackupArtifact(dir string,expectedSchema int64)(backupArtifact,error){
	if expectedSchema<=0{return backupArtifact{},errors.New("expected schema must be positive")};dir=filepath.Clean(strings.TrimSpace(dir));if dir=="."||dir==""||len(dir)>240{return backupArtifact{},errors.New("invalid backup directory")}
	info,err:=os.Lstat(dir);if err!=nil{return backupArtifact{},err};if info.Mode()&os.ModeSymlink!=0||!info.IsDir(){return backupArtifact{},errors.New("backup artifact must be a real directory")}
	manifest,err:=readSmallRecoveryFile(filepath.Join(dir,"MANIFEST"),64<<10);if err!=nil{return backupArtifact{},err};sums,err:=readSmallRecoveryFile(filepath.Join(dir,"SHA256SUMS"),1<<20);if err!=nil{return backupArtifact{},err};complete,err:=readSmallRecoveryFile(filepath.Join(dir,"COMPLETE"),64);if err!=nil{return backupArtifact{},err};if strings.TrimSpace(string(complete))!="ok"{return backupArtifact{},errors.New("backup COMPLETE marker is invalid")}
	meta,err:=parseRecoveryManifest(string(manifest));if err!=nil{return backupArtifact{},err};if meta["backup_format"]!="poisk-v2"||meta["quiesced"]!="true"||meta["secrets_included"]!="false"||meta["mail_blobs_included"]!="true"{return backupArtifact{},errors.New("backup manifest invariants failed")}
	schema,err:=strconv.ParseInt(meta["database_schema"],10,64);if err!=nil||schema!=expectedSchema{return backupArtifact{},fmt.Errorf("backup schema mismatch: artifact=%s expected=%d",meta["database_schema"],expectedSchema)}
	seen:=map[string]struct{}{};var total int64
	for lineNo,line:=range strings.Split(strings.TrimSpace(string(sums)),"\n"){
		fields:=strings.Fields(line);if len(fields)!=2||!recoveryDigestPattern.MatchString(strings.ToLower(fields[0])){return backupArtifact{},fmt.Errorf("invalid SHA256SUMS line %d",lineNo+1)}
		name:=strings.TrimPrefix(fields[1],"*");resolved,base,err:=resolveRecoveryChecksumPath(dir,name);if err!=nil{return backupArtifact{},fmt.Errorf("invalid SHA256SUMS line %d: %w",lineNo+1,err)};if _,allowed:=recoveryChecksumFiles[base];!allowed{return backupArtifact{},fmt.Errorf("unexpected checksummed file %s",base)};if _,dup:=seen[base];dup{return backupArtifact{},fmt.Errorf("duplicate checksum for %s",base)}
		digest,size,err:=hashRecoveryFile(resolved);if err!=nil{return backupArtifact{},err};if digest!=strings.ToLower(fields[0]){return backupArtifact{},fmt.Errorf("checksum mismatch for %s",base)};seen[base]=struct{}{};total+=size
	}
	for name:=range recoveryChecksumFiles{if _,ok:=seen[name];!ok{return backupArtifact{},fmt.Errorf("checksum missing for %s",name)}}
	if total<=0{return backupArtifact{},errors.New("backup artifact is empty")}
	h:=sha256.New();_,_=h.Write(manifest);_,_=h.Write([]byte{0});_,_=h.Write(sums)
	return backupArtifact{Ref:dir,SHA256:hex.EncodeToString(h.Sum(nil)),Bytes:total,Schema:schema},nil
}

func readSmallRecoveryFile(path string,max int64)([]byte,error){info,err:=os.Lstat(path);if err!=nil{return nil,err};if info.Mode()&os.ModeSymlink!=0||!info.Mode().IsRegular()||info.Size()<=0||info.Size()>max{return nil,fmt.Errorf("invalid recovery metadata file %s",filepath.Base(path))};return os.ReadFile(path)}
func parseRecoveryManifest(raw string)(map[string]string,error){out:=map[string]string{};for i,line:=range strings.Split(strings.TrimSpace(raw),"\n"){parts:=strings.SplitN(line,"=",2);if len(parts)!=2||strings.TrimSpace(parts[0])==""{return nil,fmt.Errorf("invalid MANIFEST line %d",i+1)};key:=strings.TrimSpace(parts[0]);value:=strings.TrimSpace(parts[1]);if _,dup:=out[key];dup{return nil,fmt.Errorf("duplicate MANIFEST key %s",key)};if strings.IndexFunc(key+value,func(r rune)bool{return r<0x20||r==0x7f})>=0{return nil,errors.New("control character in MANIFEST")};out[key]=value};return out,nil}
func resolveRecoveryChecksumPath(dir,name string)(string,string,error){clean:=filepath.Clean(name);base:=filepath.Base(clean);if base=="."||base==".."||strings.Contains(base,"/")||strings.Contains(base,"\\"){return "","",errors.New("invalid checksum filename")};var resolved string;if filepath.IsAbs(clean){resolved=clean}else{resolved=filepath.Join(dir,clean)};absDir,err:=filepath.Abs(dir);if err!=nil{return "","",err};absResolved,err:=filepath.Abs(resolved);if err!=nil{return "","",err};rel,err:=filepath.Rel(absDir,absResolved);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return "","",errors.New("checksum path escapes backup directory")};return absResolved,base,nil}
func hashRecoveryFile(path string)(string,int64,error){info,err:=os.Lstat(path);if err!=nil{return "",0,err};if info.Mode()&os.ModeSymlink!=0||!info.Mode().IsRegular(){return "",0,errors.New("checksummed backup entry is not a regular file")};f,err:=os.Open(path);if err!=nil{return "",0,err};defer f.Close();opened,err:=f.Stat();if err!=nil{return "",0,err};if !os.SameFile(info,opened){return "",0,errors.New("checksummed file changed before hashing")};h:=sha256.New();n,err:=io.Copy(h,f);if err!=nil{return "",0,err};if n!=opened.Size(){return "",0,errors.New("checksummed file changed while hashing")};return hex.EncodeToString(h.Sum(nil)),opened.Size(),nil}
