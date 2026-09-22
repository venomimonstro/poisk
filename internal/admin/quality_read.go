package admin

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type QualityRunSummary struct {
	RunID int64 `json:"run_id"`
	SchemaVersion int `json:"schema_version"`
	Queries int `json:"queries"`
	NDCG10 float64 `json:"ndcg_10"`
	MRR float64 `json:"mrr"`
	Recall10 float64 `json:"recall_10"`
	ZeroResultRate float64 `json:"zero_result_rate"`
	Duplicate10 float64 `json:"duplicate_10"`
	GatePass bool `json:"gate_pass"`
	Failures []string `json:"failures"`
	Source string `json:"source"`
	CompletedAt time.Time `json:"completed_at"`
}

func (s Service) LatestQualityRun(ctx context.Context,session Session)(*QualityRunSummary,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	var out QualityRunSummary;var failures []byte
	err:=s.Store.db.QueryRow(ctx,`SELECT run_id,schema_version,queries,ndcg_10,mrr,recall_10,zero_result_rate,duplicate_10,gate_pass,gate_failures,source,completed_at FROM quality_runs ORDER BY completed_at DESC,run_id DESC LIMIT 1`).Scan(&out.RunID,&out.SchemaVersion,&out.Queries,&out.NDCG10,&out.MRR,&out.Recall10,&out.ZeroResultRate,&out.Duplicate10,&out.GatePass,&failures,&out.Source,&out.CompletedAt)
	if errors.Is(err,pgx.ErrNoRows){return nil,nil};if err!=nil{return nil,err}
	if len(failures)>0{if err:=json.Unmarshal(failures,&out.Failures);err!=nil{return nil,err}};return &out,nil
}
