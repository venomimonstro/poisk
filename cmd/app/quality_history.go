package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/quality"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
)

func runQualityWithHistory(cfg config.Config,pool *pgxpool.Pool) error {
	goldenPath:=os.Getenv("QUALITY_GOLDEN_PATH");if goldenPath==""{goldenPath="/app/docs/quality/golden.seed.json"}
	thresholdPath:=os.Getenv("QUALITY_THRESHOLDS_PATH");if thresholdPath==""{thresholdPath="/app/docs/quality/thresholds.json"}
	goldenFile,err:=os.Open(goldenPath);if err!=nil{return fmt.Errorf("open golden set: %w",err)};defer goldenFile.Close();golden,err:=quality.LoadGolden(goldenFile);if err!=nil{return err}
	thresholdFile,err:=os.Open(thresholdPath);if err!=nil{return fmt.Errorf("open quality thresholds: %w",err)};defer thresholdFile.Close();thresholds,err:=quality.LoadThresholds(thresholdFile);if err!=nil{return err}
	backend,err:=searchbackend.New(searchbackend.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort)});if err!=nil{return fmt.Errorf("create quality search backend: %w",err)}
	service:=&searchsvc.Service{Backend:backend,BackendConcurrency:make(chan struct{},cfg.BackendConcurrent)};ctx,cancel:=context.WithTimeout(context.Background(),10*time.Minute);defer cancel()
	report,err:=quality.EvaluateGolden(ctx,service,golden);if err!=nil{return err};gate:=quality.CheckGate(report.Summary,thresholds)
	if _,err:= (quality.HistoryRepository{DB:pool}).Record(ctx,report,thresholds,gate,"app-quality");err!=nil{return fmt.Errorf("persist quality report: %w",err)}
	output:=struct{Report quality.Report `json:"report"`;Thresholds quality.Thresholds `json:"thresholds"`;Gate quality.GateResult `json:"gate"`}{report,thresholds,gate};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err:=enc.Encode(output);err!=nil{return fmt.Errorf("encode quality report: %w",err)};if !gate.Pass{return errors.New("search quality gate failed")};return nil
}
