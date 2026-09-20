package maps

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Store interface {
	Register(context.Context,Manifest,string) error
	Manifest(context.Context,string)(Manifest,error)
	State(context.Context)(State,error)
	Activate(context.Context,string,string) error
	Rollback(context.Context,string)(string,error)
}

type Service struct {
	Store      Store
	ArtifactRoot string
	PublicPrefix string
}

type Config struct {
	Version         string     `json:"version"`
	PMTilesURL      string     `json:"pmtiles_url"`
	StyleURL        string     `json:"style_url"`
	PMTilesSHA256   string     `json:"pmtiles_sha256"`
	PMTilesSize     int64      `json:"pmtiles_size"`
	Bounds          [4]float64 `json:"bounds"`
	MinZoom         int        `json:"min_zoom"`
	MaxZoom         int        `json:"max_zoom"`
	Center          [3]float64 `json:"center"`
	SourceName      string     `json:"source_name"`
	AttributionHTML string     `json:"attribution_html"`
}

func (s *Service) Register(ctx context.Context,m Manifest,actor string)error{
	if s==nil||s.Store==nil{return errors.New("map service is not initialized")}
	if err:=ValidateFiles(s.ArtifactRoot,m);err!=nil{return err}
	return s.Store.Register(ctx,m,actor)
}

func (s *Service) Activate(ctx context.Context,version,actor string)error{
	if s==nil||s.Store==nil{return errors.New("map service is not initialized")}
	m,err:=s.Store.Manifest(ctx,version);if err!=nil{return err}
	if err:=ValidateFiles(s.ArtifactRoot,m);err!=nil{return fmt.Errorf("map activation integrity check: %w",err)}
	return s.Store.Activate(ctx,version,actor)
}

func (s *Service) Rollback(ctx context.Context,actor string)(string,error){
	if s==nil||s.Store==nil{return "",errors.New("map service is not initialized")}
	state,err:=s.Store.State(ctx);if err!=nil{return "",err}
	if state.PreviousVersion==""{return "",ErrNoRollback}
	m,err:=s.Store.Manifest(ctx,state.PreviousVersion);if err!=nil{return "",ErrNoRollback}
	if err:=ValidateFiles(s.ArtifactRoot,m);err!=nil{return "",fmt.Errorf("map rollback integrity check: %w",err)}
	return s.Store.Rollback(ctx,actor)
}

func (s *Service) ActiveConfig(ctx context.Context)(Config,error){
	if s==nil||s.Store==nil{return Config{},errors.New("map service is not initialized")}
	state,err:=s.Store.State(ctx);if err!=nil{return Config{},err}
	m,err:=s.Store.Manifest(ctx,state.ActiveVersion);if err!=nil{return Config{},err}
	if err:=RuntimeFilesPresent(s.ArtifactRoot,m);err!=nil{return Config{},fmt.Errorf("active map unavailable: %w",err)}
	prefix:=strings.TrimRight(strings.TrimSpace(s.PublicPrefix),"/")
	if prefix==""{prefix="/maps"}
	return Config{
		Version:m.Version,
		PMTilesURL:prefix+"/"+m.PMTilesPath,
		StyleURL:prefix+"/"+m.StylePath,
		PMTilesSHA256:m.PMTilesSHA256,
		PMTilesSize:m.PMTilesSize,
		Bounds:m.Bounds,MinZoom:m.MinZoom,MaxZoom:m.MaxZoom,Center:m.Center,
		SourceName:m.SourceName,AttributionHTML:m.AttributionHTML,
	},nil
}
