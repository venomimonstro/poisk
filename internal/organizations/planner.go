package organizations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type Plan struct {
	BatchID int64
	StagingID int64
	Action string
	TargetPlaceID *int64
	MatchRule string
	Confidence int
	ReasonCode string
	PlanHash string
}

type PlannerStore interface {
	RowsForPlanning(context.Context,int64,int64,int)([]StagedRow,error)
	LinkedCandidate(context.Context,string,string)(Candidate,bool,error)
	StrongCandidates(context.Context,StagedRow)([]Candidate,error)
	SavePlan(context.Context,StagedRow,Plan,[]Candidate) error
	CompletePlanning(context.Context,int64) error
}

type Planner struct{ Store PlannerStore; PageSize int }

func (p Planner) PlanBatch(ctx context.Context,batchID int64) error {
	if p.Store==nil||batchID<=0{return ErrInvalidBatch}
	pageSize:=p.PageSize;if pageSize<=0||pageSize>500{pageSize=100}
	var after int64
	for{
		rows,err:=p.Store.RowsForPlanning(ctx,batchID,after,pageSize);if err!=nil{return err}
		if len(rows)==0{break}
		for _,row:=range rows{
			plan,review,err:=p.planRow(ctx,row);if err!=nil{return err}
			if err:=p.Store.SavePlan(ctx,row,plan,review);err!=nil{return err}
			after=row.ID
		}
	}
	return p.Store.CompletePlanning(ctx,batchID)
}

func (p Planner) planRow(ctx context.Context,row StagedRow)(Plan,[]Candidate,error){
	linked,ok,err:=p.Store.LinkedCandidate(ctx,row.SourceKey,row.SourceRecordID);if err!=nil{return Plan{},nil,err}
	if ok{
		action:="UPDATE";rule:="SOURCE_IDENTITY";confidence:=100
		if linked.SourcePayloadHash==row.PayloadHash{action="NOOP"}
		plan:=newPlan(row,action,&linked.PlaceID,rule,confidence,"")
		return plan,nil,nil
	}
	candidates,err:=p.Store.StrongCandidates(ctx,row);if err!=nil{return Plan{},nil,err}
	if len(candidates)==0{return newPlan(row,"CREATE",nil,"NO_STRONG_MATCH",100,""),nil,nil}
	if len(candidates)>1{return newPlan(row,"REVIEW",nil,"MULTIPLE_STRONG_MATCHES",0,"AMBIGUOUS_MATCH"),candidates,nil}
	candidate:=candidates[0]
	rule,confidence:=strongRule(row,candidate)
	if rule==""{return Plan{},nil,errors.New("candidate returned without strong match")}
	return newPlan(row,"UPDATE",&candidate.PlaceID,rule,confidence,""),nil,nil
}

func strongRule(row StagedRow,c Candidate)(string,int){
	if row.NormalizedName==""||row.NormalizedName!=c.NormalizedName{return "",0}
	if row.Phone!=""&&row.Phone==c.Phone{return "NAME_PHONE",100}
	if row.Website!=""&&row.Website==c.Website{return "NAME_WEBSITE",98}
	if row.NormalizedAddress!=""&&row.NormalizedAddress==c.NormalizedAddress{return "NAME_ADDRESS",95}
	return "",0
}

func newPlan(row StagedRow,action string,target *int64,rule string,confidence int,reason string)Plan{
	payload:=struct{
		StagingID int64 `json:"staging_id"`
		PayloadHash string `json:"payload_hash"`
		Action string `json:"action"`
		Target *int64 `json:"target,omitempty"`
		Rule string `json:"rule"`
		Confidence int `json:"confidence"`
		Reason string `json:"reason,omitempty"`
	}{row.ID,row.PayloadHash,action,target,rule,confidence,reason}
	encoded,_:=json.Marshal(payload);sum:=sha256.Sum256(encoded)
	return Plan{BatchID:row.BatchID,StagingID:row.ID,Action:action,TargetPlaceID:target,MatchRule:rule,Confidence:confidence,ReasonCode:reason,PlanHash:hex.EncodeToString(sum[:])}
}

func (r *Repository) SavePlan(ctx context.Context,row StagedRow,plan Plan,review []Candidate)error{
	if r==nil||r.db==nil{return errors.New("organization repository is not initialized")}
	if plan.BatchID!=row.BatchID||plan.StagingID!=row.ID||plan.PlanHash==""{return ErrInvalidBatch}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var planID int64
	err=tx.QueryRow(ctx,`INSERT INTO organization_import_plans(batch_id,staging_id,action,target_place_id,match_rule,confidence,reason_code,plan_hash)
VALUES($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),$8)
ON CONFLICT(batch_id,staging_id) DO UPDATE SET plan_hash=organization_import_plans.plan_hash
WHERE organization_import_plans.plan_hash=EXCLUDED.plan_hash
RETURNING plan_id`,plan.BatchID,plan.StagingID,plan.Action,plan.TargetPlaceID,plan.MatchRule,plan.Confidence,plan.ReasonCode,plan.PlanHash).Scan(&planID)
	if errors.Is(err,pgxErrNoRows()){return ErrImportConflict}
	if err!=nil{return fmt.Errorf("save organization import plan: %w",err)}
	if plan.Action=="REVIEW"{
		for _,candidate:=range review{
			rule,score:=strongRule(row,candidate);if rule==""{rule="AMBIGUOUS"}
			_,err=tx.Exec(ctx,`INSERT INTO organization_merge_review(batch_id,staging_id,candidate_place_id,reason_code,score)
VALUES($1,$2,$3,$4,$5) ON CONFLICT(staging_id,candidate_place_id) DO NOTHING`,row.BatchID,row.ID,candidate.PlaceID,rule,score)
			if err!=nil{return err}
		}
	}
	tag,err:=tx.Exec(ctx,`UPDATE organization_staging_rows SET state='PLANNED',updated_at=now() WHERE staging_id=$1 AND state IN ('VALID','PLANNED')`,row.ID)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrImportConflict}
	return tx.Commit(ctx)
}

// pgxErrNoRows is kept behind a helper so planner.go stays focused on planning semantics.
func pgxErrNoRows() error { return fmt.Errorf("%w", ErrImportNotFound) }
