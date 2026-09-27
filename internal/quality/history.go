package quality

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var qualityCommitPattern=regexp.MustCompile(`^[0-9a-f]{40}$`)
var qualitySHA256Pattern=regexp.MustCompile(`^[0-9a-f]{64}$`)

type HistoryRepository struct{ DB *pgxpool.Pool }

type RunRecord struct {
	ID int64 `json:"run_id"`
	SchemaVersion int `json:"schema_version"`
	GitCommit string `json:"git_commit,omitempty"`
	DatabaseSchema int64 `json:"database_schema,omitempty"`
	GoldenSHA256 string `json:"golden_sha256,omitempty"`
	ThresholdsSHA256 string `json:"thresholds_sha256,omitempty"`
	Summary Aggregate `json:"summary"`
	Gate GateResult `json:"gate"`
	Thresholds Thresholds `json:"thresholds"`
	Source string `json:"source"`
	CompletedAt time.Time `json:"completed_at"`
}

func (r HistoryRepository) Record(ctx context.Context,report Report,thresholds Thresholds,gate GateResult,source string)(RunRecord,error){
	return r.record(ctx,report,thresholds,gate,source,"",0,"","")
}

func (r HistoryRepository) RecordBound(ctx context.Context,report Report,thresholds Thresholds,gate GateResult,source,gitCommit string,databaseSchema int64)(RunRecord,error){
	return r.record(ctx,report,thresholds,gate,source,gitCommit,databaseSchema,"","")
}

func (r HistoryRepository) RecordReleaseBound(ctx context.Context,report Report,thresholds Thresholds,gate GateResult,source,gitCommit string,databaseSchema int64,goldenSHA256,thresholdsSHA256 string)(RunRecord,error){
	goldenSHA256=strings.ToLower(strings.TrimSpace(goldenSHA256));thresholdsSHA256=strings.ToLower(strings.TrimSpace(thresholdsSHA256))
	if !qualitySHA256Pattern.MatchString(goldenSHA256)||!qualitySHA256Pattern.MatchString(thresholdsSHA256){return RunRecord{},errors.New("invalid quality input binding")}
	return r.record(ctx,report,thresholds,gate,source,gitCommit,databaseSchema,goldenSHA256,thresholdsSHA256)
}

func (r HistoryRepository) record(ctx context.Context,report Report,thresholds Thresholds,gate GateResult,source,gitCommit string,databaseSchema int64,goldenSHA256,thresholdsSHA256 string)(RunRecord,error){
	if r.DB==nil{return RunRecord{},errors.New("quality history database unavailable")}
	source=strings.TrimSpace(source);if source==""{source="quality"};if len(source)>80{return RunRecord{},errors.New("invalid quality source")}
	gitCommit=strings.ToLower(strings.TrimSpace(gitCommit));if gitCommit!=""{if !qualityCommitPattern.MatchString(gitCommit)||databaseSchema<=0{return RunRecord{},errors.New("invalid quality release binding")}}else if databaseSchema!=0{return RunRecord{},errors.New("invalid quality release binding")}
	goldenSHA256=strings.ToLower(strings.TrimSpace(goldenSHA256));thresholdsSHA256=strings.ToLower(strings.TrimSpace(thresholdsSHA256));if (goldenSHA256=="")!=(thresholdsSHA256==""){return RunRecord{},errors.New("invalid quality input binding")};if goldenSHA256!=""&&(!qualitySHA256Pattern.MatchString(goldenSHA256)||!qualitySHA256Pattern.MatchString(thresholdsSHA256)){return RunRecord{},errors.New("invalid quality input binding")}
	failuresRaw,err:=json.Marshal(gate.Failures);if err!=nil{return RunRecord{},err};thresholdsRaw,err:=json.Marshal(thresholds);if err!=nil{return RunRecord{},err};reportRaw,err:=json.Marshal(report);if err!=nil{return RunRecord{},err}
	var out RunRecord
	err=r.DB.QueryRow(ctx,`INSERT INTO quality_runs(schema_version,queries,ndcg_10,mrr,recall_10,zero_result_rate,duplicate_10,gate_pass,gate_failures,thresholds,report,source,git_commit,database_schema,golden_sha256,thresholds_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12,NULLIF($13,''),NULLIF($14,0),NULLIF($15,''),NULLIF($16,''))
RETURNING run_id,completed_at`,report.SchemaVersion,report.Summary.Queries,report.Summary.NDCG10,report.Summary.MRR,report.Summary.Recall10,report.Summary.ZeroResultRate,report.Summary.Duplicate10,gate.Pass,string(failuresRaw),string(thresholdsRaw),string(reportRaw),source,gitCommit,databaseSchema,goldenSHA256,thresholdsSHA256).Scan(&out.ID,&out.CompletedAt)
	if err!=nil{return RunRecord{},err};out.SchemaVersion=report.SchemaVersion;out.GitCommit=gitCommit;out.DatabaseSchema=databaseSchema;out.GoldenSHA256=goldenSHA256;out.ThresholdsSHA256=thresholdsSHA256;out.Summary=report.Summary;out.Gate=gate;out.Thresholds=thresholds;out.Source=source;return out,nil
}

func (r HistoryRepository) Latest(ctx context.Context)(*RunRecord,error){
	if r.DB==nil{return nil,errors.New("quality history database unavailable")}
	var out RunRecord;var failuresRaw,thresholdsRaw []byte;var commit,goldenHash,thresholdsHash *string;var databaseSchema *int64
	err:=r.DB.QueryRow(ctx,`SELECT run_id,schema_version,queries,ndcg_10,mrr,recall_10,zero_result_rate,duplicate_10,gate_pass,gate_failures,thresholds,source,completed_at,git_commit,database_schema,golden_sha256,thresholds_sha256 FROM quality_runs ORDER BY completed_at DESC,run_id DESC LIMIT 1`).Scan(&out.ID,&out.SchemaVersion,&out.Summary.Queries,&out.Summary.NDCG10,&out.Summary.MRR,&out.Summary.Recall10,&out.Summary.ZeroResultRate,&out.Summary.Duplicate10,&out.Gate.Pass,&failuresRaw,&thresholdsRaw,&out.Source,&out.CompletedAt,&commit,&databaseSchema,&goldenHash,&thresholdsHash)
	if errors.Is(err,pgx.ErrNoRows){return nil,nil};if err!=nil{return nil,err};if commit!=nil{out.GitCommit=*commit};if databaseSchema!=nil{out.DatabaseSchema=*databaseSchema};if goldenHash!=nil{out.GoldenSHA256=*goldenHash};if thresholdsHash!=nil{out.ThresholdsSHA256=*thresholdsHash};if err=json.Unmarshal(failuresRaw,&out.Gate.Failures);err!=nil{return nil,err};if err=json.Unmarshal(thresholdsRaw,&out.Thresholds);err!=nil{return nil,err};return &out,nil
}
