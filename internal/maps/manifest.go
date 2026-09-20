package maps

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxStyleBytes int64 = 2 << 20
	pmtilesHeaderSize   = 127
	pmtilesRootWindow   = 16 << 10
)

var (
	ErrInvalidManifest = errors.New("invalid map manifest")
	versionPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	sourcePattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
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
	if m.PMTilesSize < pmtilesHeaderSize || m.StyleSize<=0 || m.StyleSize>maxStyleBytes { return ErrInvalidManifest }
	if m.Bounds[0] < -180 || m.Bounds[0] >= m.Bounds[2] || m.Bounds[2] > 180 { return ErrInvalidManifest }
	if m.Bounds[1] < -90 || m.Bounds[1] >= m.Bounds[3] || m.Bounds[3] > 90 { return ErrInvalidManifest }
	if m.MinZoom<0 || m.MaxZoom>24 || m.MinZoom>m.MaxZoom { return ErrInvalidManifest }
	if m.Center[0]<m.Bounds[0] || m.Center[0]>m.Bounds[2] || m.Center[1]<m.Bounds[1] || m.Center[1]>m.Bounds[3] || m.Center[2]<float64(m.MinZoom) || m.Center[2]>float64(m.MaxZoom) { return ErrInvalidManifest }
	if !sourcePattern.MatchString(strings.TrimSpace(m.SourceName)) { return ErrInvalidManifest }
	attribution:=strings.TrimSpace(m.AttributionHTML)
	if attribution=="" || len(attribution)>2000 || strings.ContainsAny(attribution,"<>\r\n") { return ErrInvalidManifest }
	return nil
}

func ValidateFiles(root string,m Manifest) error {
	if err:=m.Validate();err!=nil{return err}
	pmPath,err:=resolveUnderRoot(root,m.PMTilesPath);if err!=nil{return err}
	stylePath,err:=resolveUnderRoot(root,m.StylePath);if err!=nil{return err}
	if err:=verifyFile(pmPath,m.PMTilesSize,m.PMTilesSHA256,0);err!=nil{return fmt.Errorf("pmtiles artifact: %w",err)}
	if err:=validatePMTilesHeader(pmPath,m);err!=nil{return fmt.Errorf("pmtiles header: %w",err)}
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
	pmPath,err:=resolveUnderRoot(root,m.PMTilesPath);if err!=nil{return err}
	if err:=validatePMTilesHeader(pmPath,m);err!=nil{return err}
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

func validatePMTilesHeader(path string,m Manifest)error{
	f,err:=os.Open(path);if err!=nil{return err};defer f.Close()
	header:=make([]byte,pmtilesHeaderSize)
	if _,err:=io.ReadFull(f,header);err!=nil{return ErrInvalidManifest}
	if string(header[:7])!="PMTiles" || header[7]!=3{return ErrInvalidManifest}
	if header[99]!=1{return ErrInvalidManifest}
	if int(header[100])!=m.MinZoom || int(header[101])!=m.MaxZoom{return ErrInvalidManifest}

	bounds:=[4]float64{
		float64(int32(binary.LittleEndian.Uint32(header[102:106])))/1e7,
		float64(int32(binary.LittleEndian.Uint32(header[106:110])))/1e7,
		float64(int32(binary.LittleEndian.Uint32(header[110:114])))/1e7,
		float64(int32(binary.LittleEndian.Uint32(header[114:118])))/1e7,
	}
	for i:=range bounds{if math.Abs(bounds[i]-m.Bounds[i])>1e-7{return ErrInvalidManifest}}

	rootOffset:=binary.LittleEndian.Uint64(header[8:16])
	rootLength:=binary.LittleEndian.Uint64(header[16:24])
	tileOffset:=binary.LittleEndian.Uint64(header[56:64])
	tileLength:=binary.LittleEndian.Uint64(header[64:72])
	if rootOffset<pmtilesHeaderSize || rootLength==0 || rootOffset+rootLength<rootOffset || rootOffset+rootLength>pmtilesRootWindow{return ErrInvalidManifest}
	if !rangeInside(uint64(m.PMTilesSize),rootOffset,rootLength) || !rangeInside(uint64(m.PMTilesSize),tileOffset,tileLength){return ErrInvalidManifest}
	return nil
}

func rangeInside(size,offset,length uint64)bool{
	if length==0 || offset>size{return false}
	end:=offset+length
	return end>=offset && end<=size
}

func validateStyleFile(path string,m Manifest)error{
	f,err:=os.Open(path);if err!=nil{return err};defer f.Close()
	dec:=json.NewDecoder(io.LimitReader(f,maxStyleBytes+1))
	var style map[string]any
	if err:=dec.Decode(&style);err!=nil{return fmt.Errorf("decode map style: %w",err)}
	var extra any;if err:=dec.Decode(&extra);err!=io.EOF{return ErrInvalidManifest}
	if version,ok:=style["version"].(float64);!ok || version!=8{return ErrInvalidManifest}
	if _,hasImports:=style["imports"];hasImports{return ErrInvalidManifest}
	sources,ok:=style["sources"].(map[string]any);if !ok || len(sources)!=1{return ErrInvalidManifest}
	wanted:="pmtiles:///maps/"+m.PMTilesPath
	boundSource,ok:=sources[m.SourceName].(map[string]any);if !ok{return ErrInvalidManifest}
	if sourceType,ok:=boundSource["type"].(string);!ok || sourceType!="vector"{return ErrInvalidManifest}
	if u,ok:=boundSource["url"].(string);!ok || strings.TrimSpace(u)!=wanted{return ErrInvalidManifest}
	if _,hasTiles:=boundSource["tiles"];hasTiles{return ErrInvalidManifest}
	for _,key:=range []string{"sprite","glyphs"}{
		if value,ok:=style[key].(string);ok && !safeStyleAssetRef(value){return ErrInvalidManifest}
	}
	return nil
}

func safeStyleAssetRef(value string)bool{
	value=strings.TrimSpace(value)
	if value=="" || !strings.HasPrefix(value,"/maps/") || strings.Contains(value,"..") || strings.Contains(value,"\\") || strings.Contains(value,"://"){return false}
	return true
}
