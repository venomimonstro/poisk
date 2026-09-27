package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Check struct {
	Name string `json:"name"`
	Pass bool `json:"pass"`
	Detail string `json:"detail"`
}

type Report struct {
	Ready bool `json:"ready"`
	GitCommit string `json:"git_commit"`
	ReleaseVersion string `json:"release_version"`
	ExpectedSchema int64 `json:"expected_schema"`
	AppliedSchema int64 `json:"applied_schema"`
	CheckedAt time.Time `json:"checked_at"`
	Checks []Check `json:"checks"`
}

type Gate struct {
	DB *pgxpool.Pool
	ExpectedVersions []int64
	GitCommit string
	ReleaseVersion string
	InternetMail bool
	MailDomain string
	MailSelector string
	Now func() time.Time
}

func (g Gate) Evaluate(ctx context.Context) (Report, error) {
	if g.DB==nil{return Report{},errors.New("readiness database is not initialized")}
	if len(g.ExpectedVersions)==0{return Report{},errors.New("expected migration versions are required")}
	if !commitPattern.MatchString(g.GitCommit){return Report{},errors.New("git commit must be a lowercase 40-character SHA")}
	g.ReleaseVersion=strings.TrimSpace(g.ReleaseVersion);if g.ReleaseVersion==""{return Report{},errors.New("release version is required")}
	if g.InternetMail&&(strings.TrimSpace(g.MailDomain)==""||strings.TrimSpace(g.MailSelector)==""){return Report{},errors.New("mail domain and DKIM selector are required when Internet Mail is enabled")}
	now:=time.Now().UTC();if g.Now!=nil{now=g.Now().UTC()}
	expected:=append([]int64(nil),g.ExpectedVersions...);sort.Slice(expected,func(i,j int)bool{return expected[i]<expected[j]})
	report:=Report{Ready:true,GitCommit:g.GitCommit,ReleaseVersion:g.ReleaseVersion,ExpectedSchema:expected[len(expected)-1],CheckedAt:now}
	add:=func(name string,pass bool,detail string){report.Checks=append(report.Checks,Check{Name:name,Pass:pass,Detail:detail});if !pass{report.Ready=false}}

	applied,err:=g.appliedVersions(ctx);if err!=nil{return Report{},fmt.Errorf("read applied migrations: %w",err)}
	if len(applied)>0{report.AppliedSchema=applied[len(applied)-1]}
	missing,unexpected:=diffVersions(expected,applied)
	add("migrations_exact",len(missing)==0&&len(unexpected)==0,fmt.Sprintf("missing=%v unexpected=%v",missing,unexpected))
	releasePass,releaseDetail,err:=g.releaseCandidate(ctx,report.ExpectedSchema);if err!=nil{return Report{},err};add("release_manifest",releasePass,releaseDetail)

	for _,kind:=range []string{"BUILD_UNIT","INTEGRATION","FRESH_INSTALL","UPGRADE","BROWSER_SMOKE","SECURITY_REGRESSION","EDGE_TLS_PROXY"}{
		pass,detail,err:=g.evidence(ctx,kind,report.ExpectedSchema);if err!=nil{return Report{},err};add("evidence_"+kind,pass,detail)
	}

	qualityPass,qualityDetail,err:=g.quality(ctx,now,report.ExpectedSchema);if err!=nil{return Report{},err};add("quality_gate",qualityPass,qualityDetail)
	capacityPass,capacityDetail,err:=g.capacity(ctx,now,report.ExpectedSchema);if err!=nil{return Report{},err};add("capacity_1m",capacityPass,capacityDetail)
	backupPass,backupDetail,err:=g.recovery(ctx,"BACKUP",report.ExpectedSchema,now);if err!=nil{return Report{},err};add("recovery_backup",backupPass,backupDetail)
	restorePass,restoreDetail,err:=g.recovery(ctx,"RESTORE",report.ExpectedSchema,now);if err!=nil{return Report{},err};add("recovery_restore",restorePass,restoreDetail)
	pressurePass,pressureDetail,err:=g.resourcePressure(ctx,now);if err!=nil{return Report{},err};add("resource_pressure",pressurePass,pressureDetail)

	if g.InternetMail{
		mtaPass,mtaDetail,err:=g.evidence(ctx,"MTA_FLOW",report.ExpectedSchema);if err!=nil{return Report{},err};add("evidence_MTA_FLOW",mtaPass,mtaDetail)
		dnsPass,dnsDetail,err:=g.mailDNS(ctx,now,strings.ToLower(strings.TrimSpace(g.MailDomain)),strings.TrimSpace(g.MailSelector));if err!=nil{return Report{},err};add("mail_dns",dnsPass,dnsDetail)
	}
	return report,nil
}

func (g Gate) appliedVersions(ctx context.Context)([]int64,error){
	rows,err:=g.DB.Query(ctx,`SELECT version FROM schema_migrations ORDER BY version`);if err!=nil{return nil,err};defer rows.Close();out:=[]int64{};for rows.Next(){var v int64;if err:=rows.Scan(&v);err!=nil{return nil,err};out=append(out,v)};return out,rows.Err()
}

func diffVersions(expected,applied []int64)(missing,unexpected []int64){
	e:=map[int64]struct{}{};a:=map[int64]struct{}{};for _,v:=range expected{e[v]=struct{}{}};for _,v:=range applied{a[v]=struct{}{}}
	for _,v:=range expected{if _,ok:=a[v];!ok{missing=append(missing,v)}};for _,v:=range applied{if _,ok:=e[v];!ok{unexpected=append(unexpected,v)}};return
}

func (g Gate) releaseCandidate(ctx context.Context,schema int64)(bool,string,error){
	var build,status string;var required int64;var preflight *time.Time
	err:=g.DB.QueryRow(ctx,`SELECT build_sha,status,required_schema_version,preflight_at FROM app_releases WHERE version=$1`,g.ReleaseVersion).Scan(&build,&status,&required,&preflight)
	if errors.Is(err,pgx.ErrNoRows){return false,"release manifest not found",nil};if err!=nil{return false,"",err}
	build=strings.ToLower(strings.TrimSpace(build));allowedStatus:=status=="STAGED"||status=="ACTIVE"||status=="PREVIOUS"
	pass:=build==g.GitCommit&&required==schema&&preflight!=nil&&allowedStatus
	return pass,fmt.Sprintf("version=%s build_sha=%s expected_sha=%s required_schema=%d expected_schema=%d status=%s preflight=%v",g.ReleaseVersion,build,g.GitCommit,required,schema,status,preflight!=nil),nil
}

func (g Gate) evidence(ctx context.Context,kind string,schema int64)(bool,string,error){
	var status,commit,artifact string;var recordedSchema int64;var completed time.Time
	err:=g.DB.QueryRow(ctx,`SELECT status,git_commit,database_schema,artifact_ref,completed_at FROM commercial_readiness_evidence WHERE evidence_type=$1 AND git_commit=$2 AND database_schema=$3 ORDER BY completed_at DESC,evidence_id DESC LIMIT 1`,kind,g.GitCommit,schema).Scan(&status,&commit,&recordedSchema,&artifact,&completed)
	if errors.Is(err,pgx.ErrNoRows){return false,"missing PASS evidence for current commit/schema",nil};if err!=nil{return false,"",err}
	return status=="PASS",fmt.Sprintf("status=%s commit=%s schema=%d artifact=%s completed_at=%s",status,commit,recordedSchema,artifact,completed.UTC().Format(time.RFC3339)),nil
}

func (g Gate) quality(ctx context.Context,now time.Time,schema int64)(bool,string,error){
	var pass bool;var completed time.Time;var failuresRaw []byte;var commit string;var recordedSchema int64
	err:=g.DB.QueryRow(ctx,`SELECT gate_pass,completed_at,gate_failures,git_commit,database_schema FROM quality_runs WHERE git_commit=$1 AND database_schema=$2 ORDER BY completed_at DESC,run_id DESC LIMIT 1`,g.GitCommit,schema).Scan(&pass,&completed,&failuresRaw,&commit,&recordedSchema)
	if errors.Is(err,pgx.ErrNoRows){return false,"no quality run for current commit/schema",nil};if err!=nil{return false,"",err}
	var failures []string;_ = json.Unmarshal(failuresRaw,&failures);fresh:=now.Sub(completed.UTC())<=7*24*time.Hour&&completed.Before(now.Add(5*time.Minute))
	return pass&&fresh,fmt.Sprintf("pass=%v commit=%s schema=%d fresh_7d=%v completed_at=%s failures=%v",pass,commit,recordedSchema,fresh,completed.UTC().Format(time.RFC3339),failures),nil
}

func (g Gate) capacity(ctx context.Context,now time.Time,schema int64)(bool,string,error){
	var runID,snapshotID,measured int64;var completed time.Time;var hasADR bool;var commit,recordedSchema string
	err:=g.DB.QueryRow(ctx,`SELECT r.run_id,s.snapshot_id,s.measured_documents,r.completed_at,
EXISTS(SELECT 1 FROM capacity_adr_decisions a WHERE a.snapshot_id=s.snapshot_id),
COALESCE(r.config->>'git_commit',''),COALESCE(r.config->>'database_schema','')
FROM capacity_benchmark_runs r JOIN capacity_snapshots s ON s.run_id=r.run_id
WHERE r.status='COMPLETED' AND r.mode='ISOLATED_1M' AND s.measured_documents>=1000000
AND r.config->>'git_commit'=$1 AND r.config->>'database_schema'=$2
ORDER BY r.completed_at DESC,r.run_id DESC LIMIT 1`,g.GitCommit,fmt.Sprint(schema)).Scan(&runID,&snapshotID,&measured,&completed,&hasADR,&commit,&recordedSchema)
	if errors.Is(err,pgx.ErrNoRows){return false,"no completed ISOLATED_1M snapshot for current commit/schema with >=1,000,000 documents",nil};if err!=nil{return false,"",err}
	fresh:=now.Sub(completed.UTC())<=30*24*time.Hour&&completed.Before(now.Add(5*time.Minute));return hasADR&&fresh,fmt.Sprintf("run_id=%d snapshot_id=%d measured=%d commit=%s schema=%s adr=%v fresh_30d=%v completed_at=%s",runID,snapshotID,measured,commit,recordedSchema,hasADR,fresh,completed.UTC().Format(time.RFC3339)),nil
}

func (g Gate) recovery(ctx context.Context,kind string,schema int64,now time.Time)(bool,string,error){
	var status string;var recordedSchema int64;var completed time.Time;var artifact *string
	err:=g.DB.QueryRow(ctx,`SELECT status,database_schema,completed_at,artifact_ref FROM recovery_drills WHERE drill_type=$1 ORDER BY completed_at DESC,drill_id DESC LIMIT 1`,kind).Scan(&status,&recordedSchema,&completed,&artifact)
	if errors.Is(err,pgx.ErrNoRows){return false,"no recovery drill",nil};if err!=nil{return false,"",err}
	fresh:=now.Sub(completed.UTC())<=30*24*time.Hour&&completed.Before(now.Add(5*time.Minute))
	return status=="PASS"&&recordedSchema==schema&&fresh,fmt.Sprintf("status=%s schema=%d expected_schema=%d fresh_30d=%v artifact=%v completed_at=%s",status,recordedSchema,schema,fresh,artifact,completed.UTC().Format(time.RFC3339)),nil
}

func (g Gate) resourcePressure(ctx context.Context,now time.Time)(bool,string,error){
	var state string;var updated time.Time
	err:=g.DB.QueryRow(ctx,`SELECT COALESCE(value->>'state',''),updated_at FROM system_settings WHERE key='resource_pressure'`).Scan(&state,&updated)
	if errors.Is(err,pgx.ErrNoRows){return false,"resource pressure state missing",nil};if err!=nil{return false,"",err}
	fresh:=now.Sub(updated.UTC())<=2*time.Minute&&updated.Before(now.Add(5*time.Minute));return fresh&&state!="CRITICAL"&&state!="",fmt.Sprintf("state=%s fresh_2m=%v updated_at=%s",state,fresh,updated.UTC().Format(time.RFC3339)),nil
}

func (g Gate) mailDNS(ctx context.Context,now time.Time,domain,selector string)(bool,string,error){
	var ready,drift bool;var checked time.Time;var reasons []string
	err:=g.DB.QueryRow(ctx,`SELECT ready,drift,reasons,checked_at FROM mail_dns_readiness_snapshots WHERE domain=$1 AND selector=$2 ORDER BY checked_at DESC,snapshot_id DESC LIMIT 1`,domain,selector).Scan(&ready,&drift,&reasons,&checked)
	if errors.Is(err,pgx.ErrNoRows){return false,"no mail DNS readiness snapshot for configured domain/selector",nil};if err!=nil{return false,"",err}
	fresh:=now.Sub(checked.UTC())<=30*time.Minute&&checked.Before(now.Add(5*time.Minute));return ready&&!drift&&fresh,fmt.Sprintf("domain=%s selector=%s ready=%v drift=%v fresh_30m=%v reasons=%v checked_at=%s",domain,selector,ready,drift,fresh,reasons,checked.UTC().Format(time.RFC3339)),nil
}
