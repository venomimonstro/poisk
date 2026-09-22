package datahub

import (
	"context"
	"time"
)

type BuildState struct{Job string `json:"job"`;Cursor string `json:"cursor"`;LastRunAt *time.Time `json:"last_run_at,omitempty"`;LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`}
type Status struct{Published int64 `json:"published"`;Draft int64 `json:"draft"`;Suppressed int64 `json:"suppressed"`;Versions int64 `json:"versions"`;Trends int64 `json:"trends"`;Builds []BuildState `json:"builds"`}

func (r *Repository) Status(ctx context.Context)(Status,error){
	if r==nil||r.db==nil{return Status{},ErrInvalid};var out Status
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FILTER(WHERE state='PUBLISHED'),count(*) FILTER(WHERE state='DRAFT'),count(*) FILTER(WHERE state='SUPPRESSED') FROM datahub_pages`).Scan(&out.Published,&out.Draft,&out.Suppressed);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM datahub_page_versions`).Scan(&out.Versions);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM datahub_trends_daily WHERE day>=CURRENT_DATE-interval '7 days'`).Scan(&out.Trends);err!=nil{return out,err}
	rows,err:=r.db.Query(ctx,`SELECT job_key,cursor_text,last_run_at,last_completed_at FROM datahub_build_state ORDER BY job_key`);if err!=nil{return out,err};defer rows.Close();for rows.Next(){var s BuildState;if err:=rows.Scan(&s.Job,&s.Cursor,&s.LastRunAt,&s.LastCompletedAt);err!=nil{return out,err};out.Builds=append(out.Builds,s)};return out,rows.Err()
}
