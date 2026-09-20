package maps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type mapStoreFake struct {
	manifests map[string]Manifest
	state State
	registered []string
	activated []string
	rollbackVersion string
}

func (s *mapStoreFake) Register(_ context.Context,m Manifest,_ string)error{
	if s.manifests==nil{s.manifests=map[string]Manifest{}}
	s.manifests[m.Version]=m;s.registered=append(s.registered,m.Version);return nil
}
func (s *mapStoreFake) Manifest(_ context.Context,version string)(Manifest,error){
	m,ok:=s.manifests[version];if !ok{return Manifest{},ErrMapNotFound};return m,nil
}
func (s *mapStoreFake) State(context.Context)(State,error){if s.state.ActiveVersion==""{return s.state,ErrNoActiveMap};return s.state,nil}
func (s *mapStoreFake) Activate(_ context.Context,version,_ string)error{s.activated=append(s.activated,version);s.state.PreviousVersion=s.state.ActiveVersion;s.state.ActiveVersion=version;return nil}
func (s *mapStoreFake) Rollback(_ context.Context,_ string)(string,error){
	if s.state.PreviousVersion==""{return "",ErrNoRollback}
	v:=s.state.PreviousVersion;s.state.PreviousVersion=s.state.ActiveVersion;s.state.ActiveVersion=v;s.rollbackVersion=v;return v,nil
}

func TestServiceRegisterValidatesFilesBeforeStore(t *testing.T){
	root,m:=writeMapFixture(t);store:=&mapStoreFake{}
	s:=&Service{Store:store,ArtifactRoot:root,PublicPrefix:"/maps"}
	if err:=s.Register(context.Background(),m,"test");err!=nil{t.Fatal(err)}
	if len(store.registered)!=1 || store.registered[0]!=m.Version{t.Fatalf("registered=%v",store.registered)}
	if err:=os.Truncate(filepath.Join(root,m.PMTilesPath),128);err!=nil{t.Fatal(err)}
	m.Version="broken-v2"
	if err:=s.Register(context.Background(),m,"test");err==nil{t.Fatal("truncated archive registered")}
	if len(store.registered)!=1{t.Fatalf("invalid registration reached store: %v",store.registered)}
}

func TestServiceActivateRevalidatesArtifact(t *testing.T){
	root,m:=writeMapFixture(t);store:=&mapStoreFake{manifests:map[string]Manifest{m.Version:m}}
	s:=&Service{Store:store,ArtifactRoot:root,PublicPrefix:"/maps"}
	if err:=s.Activate(context.Background(),m.Version,"test");err!=nil{t.Fatal(err)}
	if len(store.activated)!=1{t.Fatalf("activated=%v",store.activated)}
	if err:=os.Truncate(filepath.Join(root,m.PMTilesPath),128);err!=nil{t.Fatal(err)}
	if err:=s.Activate(context.Background(),m.Version,"test");err==nil{t.Fatal("corrupt artifact activated")}
	if len(store.activated)!=1{t.Fatalf("corrupt activation reached store: %v",store.activated)}
}

func TestServiceRollbackValidatesPreviousArtifact(t *testing.T){
	root,m:=writeMapFixture(t)
	previous:=m;previous.Version="previous"
	store:=&mapStoreFake{manifests:map[string]Manifest{"active":m,"previous":previous},state:State{ActiveVersion:"active",PreviousVersion:"previous"}}
	s:=&Service{Store:store,ArtifactRoot:root,PublicPrefix:"/maps"}
	v,err:=s.Rollback(context.Background(),"test");if err!=nil{t.Fatal(err)}
	if v!="previous" || store.rollbackVersion!="previous"{t.Fatalf("version=%q rollback=%q",v,store.rollbackVersion)}
}

func TestServiceRollbackRejectsMissingPrevious(t *testing.T){
	root,m:=writeMapFixture(t)
	store:=&mapStoreFake{manifests:map[string]Manifest{"active":m},state:State{ActiveVersion:"active"}}
	s:=&Service{Store:store,ArtifactRoot:root}
	if _,err:=s.Rollback(context.Background(),"test");!errors.Is(err,ErrNoRollback){t.Fatalf("err=%v",err)}
}

func TestActiveConfigUsesRuntimeIntegrityGate(t *testing.T){
	root,m:=writeMapFixture(t);store:=&mapStoreFake{manifests:map[string]Manifest{m.Version:m},state:State{ActiveVersion:m.Version}}
	s:=&Service{Store:store,ArtifactRoot:root,PublicPrefix:"/maps"}
	cfg,err:=s.ActiveConfig(context.Background());if err!=nil{t.Fatal(err)}
	if cfg.Version!=m.Version || cfg.PMTilesURL!="/maps/"+m.PMTilesPath || cfg.StyleURL!="/maps/"+m.StylePath{t.Fatalf("cfg=%+v",cfg)}
	if err:=os.Remove(filepath.Join(root,m.StylePath));err!=nil{t.Fatal(err)}
	if _,err:=s.ActiveConfig(context.Background());err==nil{t.Fatal("missing active style not detected")}
}
