package demand

import "math"

type Signals struct {
	IndependentBuckets int
	Hits int
	AverageResults float64
	AverageQuality float64
	AverageFreshness float64
	AverageSpam float64
}

type Scores struct {
	Demand int `json:"demand"`
	Coverage int `json:"coverage"`
	Quality int `json:"quality"`
	Freshness int `json:"freshness"`
	Spam int `json:"spam"`
	Gap int `json:"gap"`
}

func Score(s Signals) Scores {
	buckets:=clampFloat(float64(s.IndependentBuckets),0,144)
	hits:=clampFloat(float64(s.Hits),0,432)
	demand:=clampInt(int(math.Round(buckets*7+math.Min(hits,24)*1.5)),0,100)
	coverage:=clampInt(int(math.Round(clampFloat(s.AverageResults,0,10)*10)),0,100)
	quality:=clampInt(int(math.Round(clampFloat(s.AverageQuality,0,100))),0,100)
	freshness:=clampInt(int(math.Round(clampFloat(s.AverageFreshness,0,100))),0,100)
	spam:=clampInt(int(math.Round(clampFloat(s.AverageSpam,0,100))),0,100)
	weakness:=100-(coverage*35+quality*35+freshness*20+(100-spam)*10)/100
	gap:=clampInt(int(math.Round(float64(demand*weakness)/100)),0,100)
	return Scores{Demand:demand,Coverage:coverage,Quality:quality,Freshness:freshness,Spam:spam,Gap:gap}
}

func Qualifies(scores Scores,independentBuckets int)bool{
	return independentBuckets>=3&&scores.Demand>=30&&scores.Coverage<=60&&scores.Quality<=65&&scores.Gap>=30
}

func clampInt(v,min,max int)int{if v<min{return min};if v>max{return max};return v}
func clampFloat(v,min,max float64)float64{if v<min{return min};if v>max{return max};return v}
