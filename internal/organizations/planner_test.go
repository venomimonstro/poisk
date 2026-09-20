package organizations

import (
	"context"
	"testing"
)

type plannerStoreFake struct{
	rows []StagedRow
	linked Candidate
	hasLinked bool
	candidates []Candidate
	saved []Plan
	reviews [][]Candidate
	completed int
}
func (s *plannerStoreFake) RowsForPlanning(_ context.Context,_ int64,after int64,_ int)([]StagedRow,error){
	var out []StagedRow;for _,row:=range s.rows{if row.ID>after{out=append(out,row)}};return out,nil
}
func (s *plannerStoreFake) LinkedCandidate(context.Context,string,string)(Candidate,bool,error){return s.linked,s.hasLinked,nil}
func (s *plannerStoreFake) StrongCandidates(context.Context,StagedRow)([]Candidate,error){return s.candidates,nil}
func (s *plannerStoreFake) SavePlan(_ context.Context,_ StagedRow,p Plan,review []Candidate)error{s.saved=append(s.saved,p);s.reviews=append(s.reviews,review);return nil}
func (s *plannerStoreFake) CompletePlanning(context.Context,int64)error{s.completed++;return nil}

func stagedFixture()StagedRow{return StagedRow{ID:1,BatchID:7,SourceKey:"catalog",SourceRecordID:"x1",NormalizedName:"кафе ромашка",Phone:"+79991234567",Website:"https://example.com/",NormalizedAddress:"москва тверская 1",PayloadHash:"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}

func TestPlannerUsesSourceIdentityForNoop(t *testing.T){
	row:=stagedFixture();store:=&plannerStoreFake{rows:[]StagedRow{row},hasLinked:true,linked:Candidate{PlaceID:9,SourcePayloadHash:row.PayloadHash}}
	if err:=(Planner{Store:store}).PlanBatch(context.Background(),row.BatchID);err!=nil{t.Fatal(err)}
	if len(store.saved)!=1||store.saved[0].Action!="NOOP"||store.saved[0].TargetPlaceID==nil||*store.saved[0].TargetPlaceID!=9{t.Fatalf("plans=%+v",store.saved)}
}

func TestPlannerCreatesWhenNoStrongCandidate(t *testing.T){
	row:=stagedFixture();store:=&plannerStoreFake{rows:[]StagedRow{row}}
	if err:=(Planner{Store:store}).PlanBatch(context.Background(),row.BatchID);err!=nil{t.Fatal(err)}
	if len(store.saved)!=1||store.saved[0].Action!="CREATE"||store.saved[0].TargetPlaceID!=nil{t.Fatalf("plan=%+v",store.saved)}
}

func TestPlannerMergesSingleStrongNamePhoneCandidate(t *testing.T){
	row:=stagedFixture();store:=&plannerStoreFake{rows:[]StagedRow{row},candidates:[]Candidate{{PlaceID:10,NormalizedName:row.NormalizedName,Phone:row.Phone}}}
	if err:=(Planner{Store:store}).PlanBatch(context.Background(),row.BatchID);err!=nil{t.Fatal(err)}
	p:=store.saved[0]
	if p.Action!="UPDATE"||p.MatchRule!="NAME_PHONE"||p.Confidence!=100||p.TargetPlaceID==nil||*p.TargetPlaceID!=10{t.Fatalf("plan=%+v",p)}
}

func TestPlannerSendsMultipleStrongCandidatesToReview(t *testing.T){
	row:=stagedFixture();store:=&plannerStoreFake{rows:[]StagedRow{row},candidates:[]Candidate{
		{PlaceID:10,NormalizedName:row.NormalizedName,Phone:row.Phone},
		{PlaceID:11,NormalizedName:row.NormalizedName,Website:row.Website},
	}}
	if err:=(Planner{Store:store}).PlanBatch(context.Background(),row.BatchID);err!=nil{t.Fatal(err)}
	p:=store.saved[0]
	if p.Action!="REVIEW"||p.ReasonCode!="AMBIGUOUS_MATCH"||len(store.reviews[0])!=2{t.Fatalf("plan=%+v reviews=%+v",p,store.reviews)}
}

func TestPlanHashIsStableForSameInput(t *testing.T){
	row:=stagedFixture();id:=int64(3)
	a:=newPlan(row,"UPDATE",&id,"NAME_PHONE",100,"")
	b:=newPlan(row,"UPDATE",&id,"NAME_PHONE",100,"")
	if a.PlanHash==""||a.PlanHash!=b.PlanHash{t.Fatalf("hashes %q %q",a.PlanHash,b.PlanHash)}
}
