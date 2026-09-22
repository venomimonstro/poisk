package admin

import (
	"context"
	"errors"
	"time"
)

type AnswerMetricsSummary struct {
	Requests int64 `json:"requests"`
	Available int64 `json:"available"`
	FallbackLowConfidence int64 `json:"fallback_low_confidence"`
	FallbackInsufficientSources int64 `json:"fallback_insufficient_sources"`
	FallbackOther int64 `json:"fallback_other"`
	Errors int64 `json:"errors"`
	AverageConfidence float64 `json:"average_confidence"`
}

func (s Service) AnswerMetrics(ctx context.Context,session Session,hours int)(AnswerMetricsSummary,error){
	if s.Store==nil||s.Store.db==nil{return AnswerMetricsSummary{},errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return AnswerMetricsSummary{},err};if hours<=0{hours=24};if hours>24*30{hours=24*30}
	since:=time.Now().UTC().Add(-time.Duration(hours)*time.Hour).Truncate(time.Hour);var out AnswerMetricsSummary;var confidence float64
	err:=s.Store.db.QueryRow(ctx,`SELECT COALESCE(sum(requests),0),COALESCE(sum(available),0),COALESCE(sum(fallback_low_confidence),0),COALESCE(sum(fallback_insufficient_sources),0),COALESCE(sum(fallback_other),0),COALESCE(sum(errors),0),COALESCE(sum(confidence_sum),0) FROM answer_metrics_hourly WHERE bucket_start >= $1`,since).Scan(&out.Requests,&out.Available,&out.FallbackLowConfidence,&out.FallbackInsufficientSources,&out.FallbackOther,&out.Errors,&confidence);if err!=nil{return AnswerMetricsSummary{},err};if out.Requests>0{out.AverageConfidence=confidence/float64(out.Requests)};return out,nil
}
