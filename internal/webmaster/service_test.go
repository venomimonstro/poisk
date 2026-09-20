package webmaster

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
)

type serviceStore struct {
	owned       Site
	ownedErr    error
	queueCalls  int
	lastUserID  int64
	lastSiteID  int64
	lastURL     string
	lastOp      string
}

func (s *serviceStore) CreateUser(context.Context,string,string)(User,error){ return User{},errors.New("unused") }
func (s *serviceStore) UserByEmail(context.Context,string)(User,error){ return User{},errors.New("unused") }
func (s *serviceStore) CreateSession(context.Context,int64,[32]byte,time.Time) error { return errors.New("unused") }
func (s *serviceStore) UserBySession(context.Context,[32]byte)(User,error){ return User{},errors.New("unused") }
func (s *serviceStore) DeleteSession(context.Context,int64,[32]byte) error { return errors.New("unused") }
func (s *serviceStore) AddSite(context.Context,int64,SiteOrigin)(Site,error){ return Site{},errors.New("unused") }
func (s *serviceStore) ListSites(context.Context,int64)([]Site,error){ return nil,errors.New("unused") }
func (s *serviceStore) OwnedSite(_ context.Context,userID,siteID int64,_ bool)(Site,error){ s.lastUserID=userID; s.lastSiteID=siteID; return s.owned,s.ownedErr }
func (s *serviceStore) CreateVerification(context.Context,int64,int64,string,[32]byte,string,time.Time)(Verification,error){ return Verification{},errors.New("unused") }
func (s *serviceStore) MarkVerified(context.Context,int64,int64,string,[32]byte) error { return errors.New("unused") }
func (s *serviceStore) SubmitSitemap(context.Context,int64,int64,string)(int64,error){ return 0,errors.New("unused") }
func (s *serviceStore) QueueURLRequest(_ context.Context,userID,siteID int64,url,op string)(int64,error){ s.queueCalls++; s.lastUserID=userID; s.lastSiteID=siteID; s.lastURL=url; s.lastOp=op; return 7,nil }
func (s *serviceStore) URLStatus(context.Context,int64,int64,string)(URLStatus,error){ return URLStatus{},errors.New("unused") }
func (s *serviceStore) Metrics(context.Context,int64,int64,time.Time,time.Time)(Metrics,error){ return Metrics{},errors.New("unused") }

type serviceResolver map[string][]netip.Addr
func (r serviceResolver) LookupNetIP(_ context.Context,_ string,host string)([]netip.Addr,error){ ips,ok:=r[host]; if !ok{return nil,errors.New("not found")}; return ips,nil }

func publicValidator() crawlersecurity.Validator {
	v:=crawlersecurity.NewValidator()
	v.Resolver=serviceResolver{"example.com":{netip.MustParseAddr("1.1.1.1")},"other.test":{netip.MustParseAddr("8.8.8.8")}}
	return v
}

func TestSubmitURLRejectsForeignSiteBeforeQueue(t *testing.T){
	store:=&serviceStore{ownedErr:ErrNotFound}
	s:=&Service{Store:store,Validator:publicValidator()}
	_,err:=s.SubmitURL(context.Background(),42,99,"https://example.com/a","REINDEX")
	if !errors.Is(err,ErrNotFound){ t.Fatalf("err=%v",err) }
	if store.queueCalls!=0 { t.Fatalf("queue calls=%d",store.queueCalls) }
}

func TestSubmitURLRejectsCrossHost(t *testing.T){
	store:=&serviceStore{owned:Site{ID:5,UserID:42,Host:"example.com",Origin:"https://example.com",Status:"VERIFIED"}}
	s:=&Service{Store:store,Validator:publicValidator()}
	_,err:=s.SubmitURL(context.Background(),42,5,"https://other.test/a","SUBMIT")
	if !errors.Is(err,ErrInvalidSiteOrigin){ t.Fatalf("err=%v",err) }
	if store.queueCalls!=0 { t.Fatalf("queue calls=%d",store.queueCalls) }
}

func TestSubmitURLQueuesNormalizedOwnedURL(t *testing.T){
	store:=&serviceStore{owned:Site{ID:5,UserID:42,Host:"example.com",Origin:"https://example.com",Status:"VERIFIED"}}
	s:=&Service{Store:store,Validator:publicValidator()}
	id,err:=s.SubmitURL(context.Background(),42,5,"https://example.com/a?utm_source=x","reindex")
	if err!=nil { t.Fatal(err) }
	if id!=7 || store.queueCalls!=1 || store.lastURL!="https://example.com/a" || store.lastOp!="REINDEX" { t.Fatalf("store=%+v id=%d",store,id) }
}

func TestMetaVerificationInstructionIsLiteralHTML(t *testing.T){
	got:=verificationInstruction(VerificationMeta,"abc")
	want:=`<meta name="poisk-verification" content="abc">`
	want=strings.ReplaceAll(want,`\"`,`"`)
	if got!=want { t.Fatalf("instruction=%q want=%q",got,want) }
}
