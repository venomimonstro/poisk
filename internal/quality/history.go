package quality

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HistoryRepository struct{ DB *pgxpool.Pool }

type RunRecord struct {
	ID int64 `json:"run_id"`
	SchemaVersion int `json:"schema_version"`
	Summary Aggregate `json:"summary"`
	Gate GateResult `json:"gate"`
	Thresholds Thresholds `json:"thresholds"`
	Source string `json:"source"`
	CompletedAt time.Time `json:"completed_at"`
}

func (r HistoryRepository) Record(ctx context.Context,report Report,thresholds Thresholds,gate GateResult,source string)(RunRecord,error){
	if r.DB==nil{return RunRecord{},errors.New("quality history database unavailable")}
	source=strings.TrimSpace(source);if source==""{source="quality"};if len(source)>80{return RunRecord{},errors.New("invalid quality source")}
	failuresRaw,err:=json.Marshal(gate.Failures);if err!=nil{return RunRecord{},err};thresholdsRaw,err:=json.Marshal(thresholds);if err!=nil{return RunRecord{},err};reportRaw,err:=json.Marshal(report);if err!=nil{return RunRecord{},err}
	var out RunRecord
	err=r.DB.QueryRow(ctx,`INSERT INTO quality_runs(schema_version,queries,ndcg_10,mrr,recall_10,zero_result_rate,duplicate_10,gate_pass,gate_failures,thresholds,report,source)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12)
RETURNING run_id,completed_at`,report.SchemaVersion,report.Summary.Queries,report.Summary.NDCG10,report.Summary.MRR,report.Summary.Recall10,report.Summary.ZeroResultRate,report.Summary.Duplicate10,gate.Pass,string(failuresRaw),string(thresholdsRaw),string(reportRaw),source).Scan(&out.ID,&out.CompletedAt)
	if err!=nil{return RunRecord{},err};out.SchemaVersion=report.SchemaVersion;out.Summary=report.Summary;out.Gate=gate;out.Thresholds=thresholds;out.Source=source;return out,nil
}

func (r HistoryRepository) Latest(ctx context.Context)(*RunRecord,error){
	if r.DB==nil{return nil,errors.New("quality history database unavailable")}
	var out RunRecord;var failuresRaw,thresholdsRaw []byte
	err:=r.DB.QueryRow(ctx,`SELECT run_id,schema_version,queries,ndcg_10,mrr,recall_10,zero_result_rate,duplicate_10,gate_pass,gate_failures,thresholds,source,completed_at FROM quality_runs ORDER BY completed_at DESC,run_id DESC LIMIT 1`).Scan(&out.ID,&out.SchemaVersion,&out.Summary.Queries,&out.Summary.NDCG10,&out.Summary.MRR,&out.Summary.Recall10,&out.Summary.ZeroResultRate,&out.Summary.Duplicate10,&out.Gate.Pass,&failuresRaw,&thresholdsRaw,&out.Source,&out.CompletedAt)
	if errors.Is(err,pgx.ErrNoRows){return nil,nil};if err!=nil{return nil,err};if err=json.Unmarshal(failuresRaw,&out.Gate.Failures);err!=nil{return nil,err};if err=json.Unmarshal(thresholdsRaw,&out.Thresholds);err!=nil{return nil,err};return &out,nil
}
