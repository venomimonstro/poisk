package readiness

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CandidateReport struct {
	Valid bool `json:"valid"`
	GitCommit string `json:"git_commit"`
	ReleaseVersion string `json:"release_version"`
	ExpectedSchema int64 `json:"expected_schema"`
	AppliedSchema int64 `json:"applied_schema"`
	Checks []Check `json:"checks"`
}

func (g Gate) Candidate(ctx context.Context)(CandidateReport,error){
	if g.DB==nil{return CandidateReport{},errors.New("readiness database is not initialized")}
	if len(g.ExpectedVersions)==0{return CandidateReport{},errors.New("expected migration versions are required")}
	if !commitPattern.MatchString(g.GitCommit){return CandidateReport{},errors.New("git commit must be a lowercase 40-character SHA")}
	g.ReleaseVersion=strings.TrimSpace(g.ReleaseVersion);if len(g.ReleaseVersion)<1||len(g.ReleaseVersion)>128||strings.IndexFunc(g.ReleaseVersion,func(r rune)bool{return r<0x20||r==0x7f})>=0{return CandidateReport{},errors.New("release version is invalid")}
	expected:=append([]int64(nil),g.ExpectedVersions...);sort.Slice(expected,func(i,j int)bool{return expected[i]<expected[j]})
	out:=CandidateReport{Valid:true,GitCommit:g.GitCommit,ReleaseVersion:g.ReleaseVersion,ExpectedSchema:expected[len(expected)-1]}
	add:=func(name string,pass bool,detail string){out.Checks=append(out.Checks,Check{Name:name,Pass:pass,Detail:detail});if !pass{out.Valid=false}}
	applied,err:=g.appliedVersions(ctx);if err!=nil{return CandidateReport{},fmt.Errorf("read applied migrations: %w",err)};if len(applied)>0{out.AppliedSchema=applied[len(applied)-1]}
	missing,unexpected:=diffVersions(expected,applied);add("migrations_exact",len(missing)==0&&len(unexpected)==0,fmt.Sprintf("missing=%v unexpected=%v",missing,unexpected))
	releasePass,releaseDetail,_,err:=g.releaseCandidate(ctx,out.ExpectedSchema);if err!=nil{return CandidateReport{},err};add("release_manifest",releasePass,releaseDetail)
	return out,nil
}

var _ *pgxpool.Pool
