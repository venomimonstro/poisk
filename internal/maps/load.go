package maps

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

const maxManifestBytes int64 = 64 << 10

func LoadManifest(root,relative string)(Manifest,error){
	if !safeRelativePath(relative,".json") || !strings.HasPrefix(relative,"manifests/"){return Manifest{},ErrInvalidManifest}
	path,err:=resolveUnderRoot(root,relative);if err!=nil{return Manifest{},err}
	f,err:=os.Open(path);if err!=nil{return Manifest{},err};defer f.Close()
	dec:=json.NewDecoder(io.LimitReader(f,maxManifestBytes+1));dec.DisallowUnknownFields()
	var m Manifest
	if err:=dec.Decode(&m);err!=nil{return Manifest{},err}
	var extra any;if err:=dec.Decode(&extra);err!=io.EOF{return Manifest{},errors.New("map manifest has trailing content")}
	if err:=m.Validate();err!=nil{return Manifest{},err}
	return m,nil
}
