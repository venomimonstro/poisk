package capacity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type SnapshotSummary struct{
	SnapshotID int64 `json:"snapshot_id"`
	RunID int64 `json:"run_id"`
	Label string `json:"label"`
	Mode string `json:"mode"`
	MeasuredDocuments int64 `json:"measured_documents"`
	MeasuredAt time.Time `json:"measured_at"`
	Workload map[string]WorkloadMetrics `json:"workload"`
	Projection Projection `json:"projection_10m"`
	Bottlenecks []Bottleneck `json:"bottlenecks"`
	Decision string `json:"decision,omitempty"`
}

func (r *Repository) RecentSnapshots(ctx context.Context,limit int)([]SnapshotSummary,error){
	if r==nil||r.db==nil||limit<1||limit>100{return nil,ErrInvalid}
	rows,err:=r.db.Query(ctx,`SELECT s.snapshot_id,s.run_id,r.label,r.mode,s.measured_documents,s.measured_at,s.workload,s.projection_10m,s.bottlenecks,COALESCE(d.choice,'')
FROM capacity_snapshots s JOIN capacity_benchmark_runs r ON r.run_id=s.run_id
LEFT JOIN capacity_adr_decisions d ON d.snapshot_id=s.snapshot_id
ORDER BY s.snapshot_id DESC LIMIT $1`,limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]SnapshotSummary,0,limit)
	for rows.Next(){var x SnapshotSummary;var workload,projection,bottlenecks []byte;if err:=rows.Scan(&x.SnapshotID,&x.RunID,&x.Label,&x.Mode,&x.MeasuredDocuments,&x.MeasuredAt,&workload,&projection,&bottlenecks,&x.Decision);err!=nil{return nil,err};if err:=json.Unmarshal(workload,&x.Workload);err!=nil{return nil,err};if err:=json.Unmarshal(projection,&x.Projection);err!=nil{return nil,err};if err:=json.Unmarshal(bottlenecks,&x.Bottlenecks);err!=nil{return nil,err};out=append(out,x)};return out,rows.Err()
}

func (r *Repository) CreateADR(ctx context.Context,snapshotID int64,choice,decidedBy,rationale string)(int64,string,error){
	choice=strings.ToUpper(strings.TrimSpace(choice));decidedBy=strings.TrimSpace(decidedBy);rationale=strings.TrimSpace(rationale)
	if r==nil||r.db==nil||snapshotID<=0||!validChoice(choice)||len(decidedBy)<2||len(decidedBy)>128||len(rationale)<20||len(rationale)>8000{return 0,"",ErrInvalid}
	var exists bool;if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM capacity_snapshots WHERE snapshot_id=$1)`,snapshotID).Scan(&exists);err!=nil{return 0,"",err};if !exists{return 0,"",pgx.ErrNoRows}
	var id int64;err:=r.db.QueryRow(ctx,`INSERT INTO capacity_adr_decisions(snapshot_id,choice,rationale,decided_by) VALUES($1,$2,$3,$4) RETURNING decision_id`,snapshotID,choice,rationale,decidedBy).Scan(&id);if err!=nil{return 0,"",err}
	markdown,err:=r.ADRMarkdown(ctx,snapshotID);return id,markdown,err
}

func (r *Repository) ADRMarkdown(ctx context.Context,snapshotID int64)(string,error){
	if r==nil||r.db==nil||snapshotID<=0{return "",ErrInvalid}
	var label,mode,choice,rationale,decidedBy string;var docs int64;var measured,decided time.Time;var workload,projection,bottlenecks []byte
	err:=r.db.QueryRow(ctx,`SELECT r.label,r.mode,s.measured_documents,s.measured_at,s.workload,s.projection_10m,s.bottlenecks,d.choice,d.rationale,d.decided_by,d.created_at
FROM capacity_snapshots s JOIN capacity_benchmark_runs r ON r.run_id=s.run_id JOIN capacity_adr_decisions d ON d.snapshot_id=s.snapshot_id WHERE s.snapshot_id=$1`,snapshotID).Scan(&label,&mode,&docs,&measured,&workload,&projection,&bottlenecks,&choice,&rationale,&decidedBy,&decided);if errors.Is(err,pgx.ErrNoRows){return "",pgx.ErrNoRows};if err!=nil{return "",err}
	var w map[string]WorkloadMetrics;var p Projection;var b []Bottleneck;if err=json.Unmarshal(workload,&w);err!=nil{return "",err};if err=json.Unmarshal(projection,&p);err!=nil{return "",err};if err=json.Unmarshal(bottlenecks,&b);err!=nil{return "",err}
	search:=w["search"]
	return fmt.Sprintf("# ADR — Capacity decision for snapshot %d\n\n- Benchmark: %s\n- Mode: %s\n- Measured at: %s\n- Measured indexed documents: %d\n- Search QPS: %.2f\n- Search P50/P95/P99: %.2f / %.2f / %.2f ms\n- Search error rate: %.4f\n- 10M model: %s, factor %.3f, projected DB+index bytes %d\n- Bottlenecks: %s\n\n## Decision\n%s\n\n## Rationale\n%s\n\nDecision recorded by %s at %s. This ADR records a human-approved architecture choice; the benchmark classifier does not apply infrastructure changes automatically.\n",snapshotID,label,mode,measured.UTC().Format(time.RFC3339),docs,search.QPS,search.P50MS,search.P95MS,search.P99MS,search.ErrorRate,p.Kind,p.ScaleFactor,p.ProjectedTotalBytes,bottleneckCodes(b),choice,rationale,decidedBy,decided.UTC().Format(time.RFC3339)),nil
}

func validChoice(v string)bool{return v=="STAY_SINGLE_NODE"||v=="MOVE_CRAWLER"||v=="SHARD_SEARCH"||v=="ADD_REPLICA"}
func bottleneckCodes(in []Bottleneck)string{if len(in)==0{return "none"};parts:=make([]string,0,len(in));for _,b:=range in{parts=append(parts,b.Code+":"+b.Severity)};return strings.Join(parts,", ")}
