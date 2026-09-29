package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/capacity"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/quality"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
)

const maxQualityInputBytes int64 = 8 << 20

func readQualityInput(path string)([]byte,error){
	f,err:=os.Open(path);if err!=nil{return nil,err};defer f.Close()
	data,err:=io.ReadAll(io.LimitReader(f,maxQualityInputBytes+1));if err!=nil{return nil,err};if len(data)==0||int64(len(data))>maxQualityInputBytes{return nil,errors.New("quality input is empty or too large")};return data,nil
}
func qualityInputSHA256(data []byte)string{sum:=sha256.Sum256(data);return hex.EncodeToString(sum[:])}

func runQualityWithHistory(cfg config.Config,pool *pgxpool.Pool) error {
	commit,err:=capacityBenchmarkCommit();if err!=nil{return fmt.Errorf("quality release binding: %w",err)}
	releaseVersion,err:=capacityBenchmarkReleaseVersion();if err!=nil{return fmt.Errorf("quality candidate binding: %w",err)}
	if err:=requireReleaseBuildIdentity(commit,releaseVersion);err!=nil{return fmt.Errorf("quality candidate binding: %w",err)}
	databaseSchema,err:=capacity.NewRepository(pool).CurrentSchema(context.Background());if err!=nil{return fmt.Errorf("quality database schema: %w",err)}
	goldenPath:=os.Getenv("QUALITY_GOLDEN_PATH");if goldenPath==""{goldenPath="/app/docs/quality/golden.seed.json"}
	thresholdPath:=os.Getenv("QUALITY_THRESHOLDS_PATH");if thresholdPath==""{thresholdPath="/app/docs/quality/thresholds.json"}
	goldenRaw,err:=readQualityInput(goldenPath);if err!=nil{return fmt.Errorf("read golden set: %w",err)};thresholdRaw,err:=readQualityInput(thresholdPath);if err!=nil{return fmt.Errorf("read quality thresholds: %w",err)}
	goldenHash,thresholdHash:=qualityInputSHA256(goldenRaw),qualityInputSHA256(thresholdRaw)
	golden,err:=quality.LoadGolden(bytes.NewReader(goldenRaw));if err!=nil{return err};thresholds,err:=quality.LoadThresholds(bytes.NewReader(thresholdRaw));if err!=nil{return err}
	backend,err:=searchbackend.New(searchbackend.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort)});if err!=nil{return fmt.Errorf("create quality search backend: %w",err)}
	service:=&searchsvc.Service{Backend:backend,BackendConcurrency:make(chan struct{},cfg.BackendConcurrent)};ctx,cancel:=context.WithTimeout(context.Background(),10*time.Minute);defer cancel()
	report,err:=quality.EvaluateGolden(ctx,service,golden);if err!=nil{return err};gate:=quality.CheckGate(report.Summary,thresholds)
	if _,err:= (quality.HistoryRepository{DB:pool}).RecordReleaseBound(ctx,report,thresholds,gate,"app-quality",commit,releaseVersion,databaseSchema,goldenHash,thresholdHash);err!=nil{return fmt.Errorf("persist quality report: %w",err)}
	output:=struct{Report quality.Report `json:"report"`;Thresholds quality.Thresholds `json:"thresholds"`;Gate quality.GateResult `json:"gate"`;GitCommit string `json:"git_commit"`;ReleaseVersion string `json:"release_version"`;DatabaseSchema int64 `json:"database_schema"`;GoldenSHA256 string `json:"golden_sha256"`;ThresholdsSHA256 string `json:"thresholds_sha256"`}{report,thresholds,gate,commit,releaseVersion,databaseSchema,goldenHash,thresholdHash};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err:=enc.Encode(output);err!=nil{return fmt.Errorf("encode quality report: %w",err)};if !gate.Pass{return errors.New("search quality gate failed")};return nil
}
