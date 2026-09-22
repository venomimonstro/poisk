package answer

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricsRepository struct{ DB *pgxpool.Pool }

type MetricsSnapshot struct {
	Requests int64 `json:"requests"`
	Available int64 `json:"available"`
	FallbackLowConfidence int64 `json:"fallback_low_confidence"`
	FallbackInsufficientSources int64 `json:"fallback_insufficient_sources"`
	FallbackOther int64 `json:"fallback_other"`
	Errors int64 `json:"errors"`
	AverageConfidence float64 `json:"average_confidence"`
}

func (r MetricsRepository) Record(ctx context.Context,resp Response,failed bool,now time.Time) error {
	if r.DB==nil{return nil};if now.IsZero(){now=time.Now().UTC()};bucket:=now.UTC().Truncate(time.Hour)
	available:=0;low:=0;insufficient:=0;other:=0;errs:=0;confidence:=resp.Confidence
	if failed{errs=1;confidence=0}else if resp.Available{available=1}else{switch strings.TrimSpace(resp.FallbackReason){case "low_confidence":low=1;case "insufficient_independent_sources":insufficient=1;default:other=1}}
	_,err:=r.DB.Exec(ctx,`INSERT INTO answer_metrics_hourly(bucket_start,requests,available,fallback_low_confidence,fallback_insufficient_sources,fallback_other,errors,confidence_sum)
VALUES($1,1,$2,$3,$4,$5,$6,$7)
ON CONFLICT(bucket_start) DO UPDATE SET requests=answer_metrics_hourly.requests+1,available=answer_metrics_hourly.available+EXCLUDED.available,fallback_low_confidence=answer_metrics_hourly.fallback_low_confidence+EXCLUDED.fallback_low_confidence,fallback_insufficient_sources=answer_metrics_hourly.fallback_insufficient_sources+EXCLUDED.fallback_insufficient_sources,fallback_other=answer_metrics_hourly.fallback_other+EXCLUDED.fallback_other,errors=answer_metrics_hourly.errors+EXCLUDED.errors,confidence_sum=answer_metrics_hourly.confidence_sum+EXCLUDED.confidence_sum,updated_at=now()`,bucket,available,low,insufficient,other,errs,confidence)
	return err
}

func (r MetricsRepository) Snapshot(ctx context.Context,since time.Time)(MetricsSnapshot,error){
	if since.IsZero(){since=time.Now().UTC().Add(-24*time.Hour)};var out MetricsSnapshot;var sum float64
	err:=r.DB.QueryRow(ctx,`SELECT COALESCE(sum(requests),0),COALESCE(sum(available),0),COALESCE(sum(fallback_low_confidence),0),COALESCE(sum(fallback_insufficient_sources),0),COALESCE(sum(fallback_other),0),COALESCE(sum(errors),0),COALESCE(sum(confidence_sum),0) FROM answer_metrics_hourly WHERE bucket_start >= $1`,since.UTC().Truncate(time.Hour)).Scan(&out.Requests,&out.Available,&out.FallbackLowConfidence,&out.FallbackInsufficientSources,&out.FallbackOther,&out.Errors,&sum);if err!=nil{return MetricsSnapshot{},err};if out.Requests>0{out.AverageConfidence=sum/float64(out.Requests)};return out,nil
}
