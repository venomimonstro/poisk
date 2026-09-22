package datahub

import "testing"

func TestStableSegmentIsDeterministicAndBounded(t *testing.T){
	cases:=map[string]string{
		"Moscow":"moscow",
		"home appliances":"home-appliances",
		"a/b?c":"abc",
	}
	for in,want:=range cases{if got:=StableSegment(in);got!=want{t.Fatalf("StableSegment(%q)=%q want=%q",in,got,want)}}
	long:="очень-длинный-ключ-с-кириллицей"
	a:=StableSegment(long);b:=StableSegment(long);if a==""||a!=b||len(a)>190{t.Fatalf("unstable fallback %q %q",a,b)}
}

func TestIdentityStablePaths(t *testing.T){
	slug,path,err:=Identity(PageCityCategory,"moscow","hearing-aids",0,0);if err!=nil{t.Fatal(err)}
	if slug!="moscow~hearing-aids"||path!="/data/city/moscow/hearing-aids"{t.Fatalf("slug=%q path=%q",slug,path)}
	slug,path,err=Identity(PageOrganization,"","",42,0);if err!=nil||slug!="org-42"||path!="/data/organization/org-42"{t.Fatalf("org slug=%q path=%q err=%v",slug,path,err)}
}

func TestThinPageGate(t *testing.T){
	thin:=Evaluate(PageCityCategory,Evidence{Organizations:2,WithAddress:2,AverageQuality:90,DistinctSources:2})
	if thin.Publish||thin.Reason!="insufficient_organizations"{t.Fatalf("thin=%+v",thin)}
	ok:=Evaluate(PageCityCategory,Evidence{Organizations:8,WithWebsite:5,WithAddress:7,AverageQuality:70,DistinctSources:4})
	if !ok.Publish||ok.Score<45{t.Fatalf("ok=%+v",ok)}
}

func TestCityRequiresMoreEvidence(t *testing.T){
	gate:=Evaluate(PageCity,Evidence{Organizations:9,WithWebsite:7,WithAddress:8,AverageQuality:80,DistinctSources:5})
	if gate.Publish{t.Fatalf("city should remain draft: %+v",gate)}
}
