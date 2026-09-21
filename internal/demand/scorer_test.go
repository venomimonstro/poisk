package demand

import "testing"

func TestScoreBoundaries(t *testing.T){
	cases:=[]Signals{
		{},
		{IndependentBuckets:999,Hits:999,AverageResults:999,AverageQuality:999,AverageFreshness:999,AverageSpam:999},
		{IndependentBuckets:-1,Hits:-1,AverageResults:-1,AverageQuality:-1,AverageFreshness:-1,AverageSpam:-1},
	}
	for _,in:=range cases{got:=Score(in);for name,v:=range map[string]int{"demand":got.Demand,"coverage":got.Coverage,"quality":got.Quality,"freshness":got.Freshness,"spam":got.Spam,"gap":got.Gap}{if v<0||v>100{t.Fatalf("%s=%d out of range for %+v",name,v,in)}}}
}

func TestQualifiesRequiresIndependentDemandAndWeakResults(t *testing.T){
	weak:=Score(Signals{IndependentBuckets:6,Hits:12,AverageResults:2,AverageQuality:35,AverageFreshness:40,AverageSpam:25})
	if !Qualifies(weak,6){t.Fatalf("weak high-demand result should qualify: %+v",weak)}
	if Qualifies(weak,2){t.Fatal("two buckets must not qualify")}
	strong:=Score(Signals{IndependentBuckets:8,Hits:20,AverageResults:9,AverageQuality:90,AverageFreshness:90,AverageSpam:5})
	if Qualifies(strong,8){t.Fatalf("healthy coverage must not create a gap: %+v",strong)}
}

func TestSpamCanOnlyIncreaseWeakness(t *testing.T){
	clean:=Score(Signals{IndependentBuckets:6,Hits:12,AverageResults:4,AverageQuality:55,AverageFreshness:60,AverageSpam:0})
	spam:=Score(Signals{IndependentBuckets:6,Hits:12,AverageResults:4,AverageQuality:55,AverageFreshness:60,AverageSpam:100})
	if spam.Gap<clean.Gap{t.Fatalf("spam reduced gap: clean=%+v spam=%+v",clean,spam)}
}
