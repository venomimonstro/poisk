package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var evidenceSHA256Pattern=regexp.MustCompile(`^[0-9a-f]{64}$`)
var evidenceKinds=map[string]struct{}{
	"BUILD_UNIT":{},"INTEGRATION":{},"FRESH_INSTALL":{},"UPGRADE":{},"BROWSER_SMOKE":{},
	"SECURITY_REGRESSION":{},"EDGE_TLS_PROXY":{},"MTA_FLOW":{},
}
var sensitiveEvidenceKeys=map[string]struct{}{
	"password":{},"passwd":{},"token":{},"access_token":{},"refresh_token":{},"authorization":{},"cookie":{},
	"secret":{},"client_secret":{},"private_key":{},"api_key":{},"apikey":{},"access_key":{},"session":{},"session_token":{},
}
var sensitiveEvidenceSuffixes=[]string{"_password","_passwd","_token","_secret","_private_key","_api_key","_access_key"}

type EvidenceInput struct{
	Type string
	Status string
	GitCommit string
	ReleaseVersion string
	DatabaseSchema int64
	ArtifactRef string
	ArtifactSHA256 string
	Details map[string]any
	Actor string
	CompletedAt time.Time
}

func RecordEvidence(ctx context.Context,db *pgxpool.Pool,in EvidenceInput)(int64,error){
	if db==nil{return 0,errors.New("readiness database is not initialized")}
	in.Type=strings.ToUpper(strings.TrimSpace(in.Type));in.Status=strings.ToUpper(strings.TrimSpace(in.Status));in.GitCommit=strings.ToLower(strings.TrimSpace(in.GitCommit));in.ReleaseVersion=strings.TrimSpace(in.ReleaseVersion);in.ArtifactRef=strings.TrimSpace(in.ArtifactRef);in.ArtifactSHA256=strings.ToLower(strings.TrimSpace(in.ArtifactSHA256));in.Actor=strings.TrimSpace(in.Actor)
	if _,ok:=evidenceKinds[in.Type];!ok{return 0,errors.New("invalid evidence type")};if in.Status!="PASS"&&in.Status!="FAIL"{return 0,errors.New("invalid evidence status")};if !commitPattern.MatchString(in.GitCommit){return 0,errors.New("invalid git commit")};if len(in.ReleaseVersion)<1||len(in.ReleaseVersion)>128||hasControl(in.ReleaseVersion){return 0,errors.New("invalid release version")};if in.DatabaseSchema<=0{return 0,errors.New("invalid database schema")};if len(in.ArtifactRef)<1||len(in.ArtifactRef)>240||hasControl(in.ArtifactRef){return 0,errors.New("invalid artifact ref")};if !evidenceSHA256Pattern.MatchString(in.ArtifactSHA256){return 0,errors.New("invalid artifact sha256")};if len(in.Actor)<1||len(in.Actor)>120||hasControl(in.Actor){return 0,errors.New("invalid actor")};if in.CompletedAt.IsZero(){in.CompletedAt=time.Now().UTC()};if in.CompletedAt.After(time.Now().UTC().Add(5*time.Minute)){return 0,errors.New("completed_at is in the future")};if in.Details==nil{in.Details=map[string]any{}}
	if containsSensitiveEvidenceKey(in.Details){return 0,errors.New("evidence details contain a sensitive key")}
	details,err:=json.Marshal(in.Details);if err!=nil{return 0,err};if len(details)>16<<10{return 0,errors.New("evidence details are too large")}
	var id int64;err=db.QueryRow(ctx,`INSERT INTO commercial_readiness_evidence(evidence_type,status,git_commit,release_version,database_schema,artifact_ref,artifact_sha256,details,actor,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10) RETURNING evidence_id`,in.Type,in.Status,in.GitCommit,in.ReleaseVersion,in.DatabaseSchema,in.ArtifactRef,in.ArtifactSHA256,string(details),in.Actor,in.CompletedAt.UTC()).Scan(&id);return id,err
}

func hasControl(v string)bool{return strings.IndexFunc(v,func(r rune)bool{return r<0x20||r==0x7f})>=0}

func containsSensitiveEvidenceKey(v any)bool{
	switch value:=v.(type){
	case map[string]any:
		for key,item:=range value{
			normalized:=strings.ToLower(strings.TrimSpace(key));normalized=strings.ReplaceAll(normalized,"-","_");normalized=strings.ReplaceAll(normalized," ","_")
			if isSensitiveEvidenceKey(normalized){return true}
			if containsSensitiveEvidenceKey(item){return true}
		}
	case []any:
		for _,item:=range value{if containsSensitiveEvidenceKey(item){return true}}
	}
	return false
}

func isSensitiveEvidenceKey(key string)bool{
	if _,blocked:=sensitiveEvidenceKeys[key];blocked{return true}
	for _,suffix:=range sensitiveEvidenceSuffixes{if strings.HasSuffix(key,suffix){return true}}
	return false
}
