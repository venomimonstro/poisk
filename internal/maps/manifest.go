package maps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxStyleBytes int64 = 2 << 20

var (
	ErrInvalidManifest = errors.New("invalid map manifest")
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

type Manifest struct {
	Version         string     `json:"version"`
	PMTilesPath     string     `json:"pmtiles_path"`
	StylePath       string     `json:"style_path"`
	PMTilesSHA256   string     `json:"pmtiles_sha256"`
	StyleSHA256     string     `json:"style_sha256"`
	PMTilesSize     int64      `json:"pmtiles_size"`
	StyleSize       int64      `json:"style_size"`
	Bounds          [4]float64 `json:"bounds"`
	MinZoom         int        `json:"min_zoom"`
	MaxZoom         int        `json:"max_zoom"`
	Center          [3]float64 `json:"center"`
	SourceName      string     `json:"source_name"`
	AttributionHTML string     `json:"attribution_html"`
}

func (m Manifest) Validate() error {
	if !versionPattern.MatchString(m.Version) { return ErrInvalidManifest }
	if !safeRelativePath(m.PMTilesPath,".pmtiles") || !safeRelativePath(m.StylePath,".json") { return ErrInvalidManifest }
	if !validSHA(m.PMTilesSHA256) || !validSHA(m.StyleSHA256) { return ErrInvalidManifest }
	if m.PMTilesSize<=0 || m.StyleSize<=0 || m.StyleSize>maxStyleBytes { return ErrInvalidManifest }
	if m.Bounds[0] < -180 || m.Bounds[0] >= m.Bounds[2] || m.Bounds[2] > 180 { return ErrInvalidManifest }
	if m.Bounds[1] < -90 || m.Bounds[1] >= m.Bounds[3] || m.Bounds[3] > 90 { return ErrInvalidManifest }
	if m.MinZoom<0 || m.MaxZoom>24 || m.MinZoom>m.MaxZoom { return ErrInvalidManifest }
	if m.Center[0]<m.Bounds[0] || m.Center[0]>m.Bounds[2] || m.Center[1]<m.Bounds[1] || m.Center[1]>m.Bounds[3] || m.Center[2]<float64(m.MinZoom) || m.Center[2]>float64(m.MaxZoom) { return ErrInvalidManifest }
	if strings.TrimSpace(m.SourceName)=="" || len(m.SourceName)>200 || strings.TrimSpace(m.AttributionHTML)=="" || len(m.AttributionHTML)>2000 { return ErrInvalidManifest }
	return nil
}

func ValidateFiles(root string,m Manifest) error {
	if err:=m.Validate();err!=nil{return err}
	pmPath,err:=resolveUnderRoot(root,m.PMTilesPath);if err!=nil{return err}
	stylePath,err:=resolveUnderRoot(root,m.StylePath);if err!=nil{return err}
	if err:=verifyFile(pmPath,m.PMTilesSize,m.PMTilesSHA256,0);err!=nil{return fmt.Errorf("pmtiles artifact: %w",err)}
	if err:=verifyFile(stylePath,m.StyleSize,m.StyleSHA256,maxStyleBytes);err!=nil{return fmt.Errorf("map style: %w",err)}
	if err:=validateStyleFile(stylePath,m);err!=nil{return err}
	return nil
}

func RuntimeFilesPresent(root string,m Manifest) error {
	if err:=m.Validate();err!=nil{return err}
	for _,item:=range []struct{path string;size int64}{{m.PMTilesPath,m.PMTilesSize},{m.StylePath,m.StyleSize}}{
		full,err:=resolveUnderRoot(root,item.path);if err!=nil{return err}
		info,err:=os.Stat(full);if err!=nil{return err}
		if !info.Mode().IsRegular() || info.Size()!=item.size{return ErrInvalidManifest}
	}
	stylePath,err:=resolveUnderRoot(root,m.StylePath);if err!=nil{return err}
	return validateStyleFile(stylePath,m)
}

func safeRelativePath(path,suffix string) bool {
	if path=="" || filepath.IsAbs(path) || strings.Contains(path,"\\") || strings.Contains(path,"..") || !strings.HasSuffix(strings.ToLower(path),suffix){return false}
	clean:=filepath.ToSlash(filepath.Clean(path))
	return clean==path && clean!="." && !strings.HasPrefix(clean,"/")
}

func validSHA(value string) bool {
	if len(value)!=64 || strings.ToLower(value)!=value{return false}
	_,err:=hex.DecodeString(value);return err==nil
}

func resolveUnderRoot(root,relative string)(string,error){
	if strings.TrimSpace(root)=="" || relative=="" || filepath.IsAbs(relative) || strings.Contains(relative,"\\") || strings.Contains(relative,".."){return "",ErrInvalidManifest}
	rootAbs,err:=filepath.Abs(root);if err!=nil{return "",err}
	full,err:=filepath.Abs(filepath.Join(rootAbs,filepath.FromSlash(relative)));if err!=nil{return "",err}
	rel,err:=filepath.Rel(rootAbs,full);if err!=nil || rel==".." || strings.HasPrefix(rel,".."+string(filepath.Separator)){return "",ErrInvalidManifest}
	return full,nil
}

func verifyFile(path string,wantSize int64,wantSHA string,maxSize int64)error{
	info,err:=os.Stat(path);if err!=nil{return err}
	if !info.Mode().IsRegular() || info.Size()!=wantSize || (maxSize>0 && info.Size()>maxSize){return ErrInvalidManifest}
	f,err:=os.Open(path);if err!=nil{return err};defer f.Close()
	h:=sha256.New();if _,err:=io.Copy(h,f);err!=nil{return err}
	if hex.EncodeToString(h.Sum(nil))!=wantSHA{return ErrInvalidManifest}
	return nil
}

func validateStyleFile(path string,m Manifest)error{
	f,err:=os.Open(path);if err!=nil{return err};defer f.Close()
	dec:=json.NewDecoder(io.LimitReader(f,maxStyleBytes+1))
	var style map[string]any
	if err:=dec.Decode(&style);err!=nil{return fmt.Errorf("decode map style: %w",err)}
	var extra any;if err:=dec.Decode(&extra);err!=io.EOF{return ErrInvalidManifest}
	if version,ok:=style["version"].(float64);!ok || version!=8{return ErrInvalidManifest}
	if _,hasImports:=style["imports"];hasImports{return ErrInvalidManifest}
	sources,ok:=style["sources"].(map[string]any);if !ok || len(sources)==0{return ErrInvalidManifest}
	wanted:="pmtiles:///maps/"+m.PMTilesPath
	bound:=false
	for _,raw:=range sources{
		source,ok:=raw.(map[string]any);if !ok{return ErrInvalidManifest}
		if u,ok:=source["url"].(string);ok{
			u=strings.TrimSpace(u)
			if u==wanted{bound=true;continue}
			lower:=strings.ToLower(u)
			if strings.HasPrefix(lower,"http://") || strings.HasPrefix(lower,"https://") || strings.HasPrefix(lower,"pmtiles://"){return ErrInvalidManifest}
		}
		if tiles,ok:=source["tiles"].([]any);ok{for _,tile:=range tiles{if s,ok:=tile.(string);ok{lower:=strings.ToLower(strings.TrimSpace(s));if strings.HasPrefix(lower,"http://")||strings.HasPrefix(lower,"https://"){return ErrInvalidManifest}}}}
	}
	if !bound{return ErrInvalidManifest}
	for _,key:=range []string{"sprite","glyphs"}{if value,ok:=style[key].(string);ok{lower:=strings.ToLower(strings.TrimSpace(value));if strings.HasPrefix(lower,"http://")||strings.HasPrefix(lower,"https://"){return ErrInvalidManifest}}}
	return nil
}
