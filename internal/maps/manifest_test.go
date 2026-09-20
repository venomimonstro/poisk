package maps

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writeMapFixture(t *testing.T) (string, Manifest) {
	t.Helper()
	root:=t.TempDir()
	if err:=os.MkdirAll(filepath.Join(root,"tiles"),0o755);err!=nil{t.Fatal(err)}
	if err:=os.MkdirAll(filepath.Join(root,"styles"),0o755);err!=nil{t.Fatal(err)}

	pm:=make([]byte,256)
	copy(pm[:7],[]byte("PMTiles"));pm[7]=3
	binary.LittleEndian.PutUint64(pm[8:16],127)
	binary.LittleEndian.PutUint64(pm[16:24],1)
	binary.LittleEndian.PutUint64(pm[56:64],128)
	binary.LittleEndian.PutUint64(pm[64:72],1)
	pm[96]=1;pm[97]=1;pm[98]=1;pm[99]=1;pm[100]=0;pm[101]=14
	putE7:=func(offset int,v float64){binary.LittleEndian.PutUint32(pm[offset:offset+4],uint32(int32(v*1e7)))}
	putE7(102,-180);putE7(106,-85);putE7(110,180);putE7(114,85)
	pm[118]=4;putE7(119,37.62);putE7(123,55.75)
	pmPath:=filepath.Join(root,"tiles","ru-v1.pmtiles")
	if err:=os.WriteFile(pmPath,pm,0o644);err!=nil{t.Fatal(err)}

	style:=[]byte(`{"version":8}`)
	style=[]byte(`{"version":8,"sources":{"osm":{"type":"vector","url":"pmtiles:///maps/tiles/ru-v1.pmtiles"}},"layers":[]}`)
	// Convert the source literal into actual JSON instead of an escaped fixture.
	for i:=0;i<len(style);i++{if style[i]=='\\' && i+1<len(style) && style[i+1]=='"'{style=append(style[:i],style[i+1:]...);i--}}
	stylePath:=filepath.Join(root,"styles","ru-v1.json")
	if err:=os.WriteFile(stylePath,style,0o644);err!=nil{t.Fatal(err)}
	pmHash:=sha256.Sum256(pm);styleHash:=sha256.Sum256(style)
	m:=Manifest{
		Version:"ru-v1",PMTilesPath:"tiles/ru-v1.pmtiles",StylePath:"styles/ru-v1.json",
		PMTilesSHA256:hex.EncodeToString(pmHash[:]),StyleSHA256:hex.EncodeToString(styleHash[:]),
		PMTilesSize:int64(len(pm)),StyleSize:int64(len(style)),Bounds:[4]float64{-180,-85,180,85},
		MinZoom:0,MaxZoom:14,Center:[3]float64{37.62,55.75,8},SourceName:"osm",AttributionHTML:"© OpenStreetMap contributors",
	}
	return root,m
}

func TestValidateFilesAcceptsMatchingPMTilesAndStyle(t *testing.T){
	root,m:=writeMapFixture(t)
	if err:=ValidateFiles(root,m);err!=nil{t.Fatalf("ValidateFiles() error=%v",err)}
}

func TestValidateFilesRejectsCorruptPMTilesHeader(t *testing.T){
	root,m:=writeMapFixture(t)
	path:=filepath.Join(root,"tiles","ru-v1.pmtiles")
	data,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
	copy(data[:7],[]byte("NOTTILE"))
	if err:=os.WriteFile(path,data,0o644);err!=nil{t.Fatal(err)}
	h:=sha256.Sum256(data);m.PMTilesSHA256=hex.EncodeToString(h[:])
	if err:=ValidateFiles(root,m);err==nil{t.Fatal("corrupt PMTiles header accepted")}
}

func TestValidateFilesRejectsHeaderManifestMismatch(t *testing.T){
	root,m:=writeMapFixture(t)
	m.MaxZoom=13
	if err:=ValidateFiles(root,m);err==nil{t.Fatal("zoom mismatch accepted")}
}

func TestValidateFilesRequiresConfiguredStyleSource(t *testing.T){
	root,m:=writeMapFixture(t)
	m.SourceName="missing"
	if err:=ValidateFiles(root,m);err==nil{t.Fatal("missing configured source accepted")}
}

func TestManifestRejectsTraversalAndRemoteStyleDependencies(t *testing.T){
	root,m:=writeMapFixture(t)
	bad:=m;bad.PMTilesPath="../secret.pmtiles"
	if err:=bad.Validate();err==nil{t.Fatal("path traversal accepted")}

	style:=[]byte(`{"version":8,"sources":{"osm":{"type":"vector","url":"pmtiles:///maps/tiles/ru-v1.pmtiles"}},"glyphs":"https://external.test/{fontstack}/{range}.pbf","layers":[]}`)
	for i:=0;i<len(style);i++{if style[i]=='\\' && i+1<len(style) && style[i+1]=='"'{style=append(style[:i],style[i+1:]...);i--}}
	stylePath:=filepath.Join(root,"styles","ru-v1.json")
	if err:=os.WriteFile(stylePath,style,0o644);err!=nil{t.Fatal(err)}
	h:=sha256.Sum256(style);m.StyleSHA256=hex.EncodeToString(h[:]);m.StyleSize=int64(len(style))
	if err:=ValidateFiles(root,m);err==nil{t.Fatal("remote style dependency accepted")}
}
