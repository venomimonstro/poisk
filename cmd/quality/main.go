package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/venomimonstro/poisk/internal/quality"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
)

type output struct {
	Report quality.Report     `json:"report"`
	Gate   quality.GateResult `json:"gate"`
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	goldenPath := getenv("QUALITY_GOLDEN_PATH", "docs/quality/golden.seed.json")
	thresholdPath := getenv("QUALITY_THRESHOLDS_PATH", "docs/quality/thresholds.json")
	host := getenv("MANTICORE_HOST", "localhost")
	port, err := strconv.Atoi(getenv("MANTICORE_HTTP_PORT", "9308"))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid MANTICORE_HTTP_PORT")
	}

	goldenFile, err := os.Open(goldenPath)
	if err != nil { return fmt.Errorf("open golden set: %w", err) }
	defer goldenFile.Close()
	set, err := quality.LoadGolden(goldenFile)
	if err != nil { return err }

	thresholdFile, err := os.Open(thresholdPath)
	if err != nil { return fmt.Errorf("open quality thresholds: %w", err) }
	defer thresholdFile.Close()
	thresholds, err := quality.LoadThresholds(thresholdFile)
	if err != nil { return err }

	backend, err := searchbackend.New(searchbackend.Config{
		BaseURL: fmt.Sprintf("http://%s:%d", host, port),
		Timeout: 2 * time.Second,
		MaxResults: 50,
	})
	if err != nil { return fmt.Errorf("create quality search backend: %w", err) }
	service := &searchsvc.Service{Backend: backend}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	report, err := quality.EvaluateGolden(ctx, service, set)
	if err != nil { return err }

	aggregate := quality.Aggregate{
		Queries: report.Summary.JudgedQueries,
		NDCG10: report.Summary.NDCG10,
		MRR: report.Summary.MRR,
		Recall10: report.Summary.Recall10,
		ZeroResultRate: report.Summary.ZeroResultRate,
		Duplicate10: report.Summary.Duplicate10Rate,
	}
	gate := quality.CheckGate(aggregate, thresholds)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output{Report: report, Gate: gate}); err != nil {
		return fmt.Errorf("encode quality report: %w", err)
	}
	if !gate.Pass {
		return errors.New("search quality gate failed")
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" { return value }
	return fallback
}
