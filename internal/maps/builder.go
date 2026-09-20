package maps

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

type pmtilesInfo struct {
	Bounds     [4]float64
	MinZoom    int
	MaxZoom    int
	Center     [3]float64
	TileType   byte
	RootOffset uint64
	RootLength uint64
	TileOffset uint64
	TileLength uint64
}

func readPMTilesInfo(path string,size int64)(pmtilesInfo,error){
	if size<pmtilesHeaderSize{return pmtilesInfo{},ErrInvalidManifest}
	f,err:=os.Open(path);if err!=nil{return pmtilesInfo{},err};defer f.Close()
	header:=make([]byte,pmtilesHeaderSize)
	if _,err:=io.ReadFull(f,header);err!=nil{return pmtilesInfo{},ErrInvalidManifest}
	if string(header[:7])!="PMTiles" || header[7]!=3{return pmtilesInfo{},ErrInvalidManifest}
	readE7:=func(start int)float64{return float64(int32(binary.LittleEndian.Uint32(header[start:start+4])))/1e7}
	info:=pmtilesInfo{
		Bounds:[4]float64{readE7(102),readE7(106),readE7(110),readE7(114)},
		MinZoom:int(header[100]),MaxZoom:int(header[101]),Center:[3]float64{readE7(119),readE7(123),float64(header[118])},
		TileType:header[99],RootOffset:binary.LittleEndian.Uint64(header[8:16]),RootLength:binary.LittleEndian.Uint64(header[16:24]),
		TileOffset:binary.LittleEndian.Uint64(header[56:64]),TileLength:binary.LittleEndian.Uint64(header[64:72]),
	}
	if info.TileType!=1{return pmtilesInfo{},ErrInvalidManifest}
	if info.MinZoom<0 || info.MaxZoom>24 || info.MinZoom>info.MaxZoom{return pmtilesInfo{},ErrInvalidManifest}
	if info.RootOffset<pmtilesHeaderSize || info.RootLength==0 || info.RootOffset+info.RootLength<info.RootOffset || info.RootOffset+info.RootLength>pmtilesRootWindow{return pmtilesInfo{},ErrInvalidManifest}
	if !rangeInside(uint64(size),info.RootOffset,info.RootLength) || !rangeInside(uint64(size),info.TileOffset,info.TileLength){return pmtilesInfo{},ErrInvalidManifest}
	return info,nil
}

func BuildManifest(root,version,pmtilesPath,stylePath,sourceName,attribution string)(Manifest,error){
	if !safeRelativePath(pmtilesPath,".pmtiles") || !safeRelativePath(stylePath,".json"){return Manifest{},ErrInvalidManifest}
	pmFull,err:=resolveUnderRoot(root,pmtilesPath);if err!=nil{return Manifest{},err}
	styleFull,err:=resolveUnderRoot(root,stylePath);if err!=nil{return Manifest{},err}
	pmStat,err:=os.Stat(pmFull);if err!=nil{return Manifest{},err}
	styleStat,err:=os.Stat(styleFull);if err!=nil{return Manifest{},err}
	if !pmStat.Mode().IsRegular() || !styleStat.Mode().IsRegular() || styleStat.Size()<=0 || styleStat.Size()>maxStyleBytes{return Manifest{},ErrInvalidManifest}
	info,err:=readPMTilesInfo(pmFull,pmStat.Size());if err!=nil{return Manifest{},err}
	pmSHA,err:=sha256File(pmFull);if err!=nil{return Manifest{},err}
	styleSHA,err:=sha256File(styleFull);if err!=nil{return Manifest{},err}
	m:=Manifest{
		Version:version,PMTilesPath:pmtilesPath,StylePath:stylePath,PMTilesSHA256:pmSHA,StyleSHA256:styleSHA,
		PMTilesSize:pmStat.Size(),StyleSize:styleStat.Size(),Bounds:info.Bounds,MinZoom:info.MinZoom,MaxZoom:info.MaxZoom,Center:info.Center,
		SourceName:sourceName,AttributionHTML:attribution,
	}
	if err:=ValidateFiles(root,m);err!=nil{return Manifest{},err}
	return m,nil
}

func sha256File(path string)(string,error){
	f,err:=os.Open(path);if err!=nil{return "",err};defer f.Close()
	h:=sha256.New();if _,err:=io.Copy(h,f);err!=nil{return "",err}
	value:=hex.EncodeToString(h.Sum(nil));if value==""{return "",errors.New("empty sha256")}
	return value,nil
}
