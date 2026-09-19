package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func LoadThresholds(r io.Reader) (Thresholds, error) {
	if r == nil { return Thresholds{}, errors.New("threshold reader is nil") }
	decoder := json.NewDecoder(io.LimitReader(r, 64<<10))
	decoder.DisallowUnknownFields()
	var out Thresholds
	if err := decoder.Decode(&out); err != nil { return Thresholds{}, fmt.Errorf("decode quality thresholds: %w", err) }
	if err := out.Validate(); err != nil { return Thresholds{}, err }
	return out, nil
}

func (t Thresholds) Validate() error {
	if t.MinQueries < 1 || t.MinQueries > 100000 {
		return errors.New("quality threshold min_queries must be between 1 and 100000")
	}
	for name, value := range map[string]float64{
		"min_ndcg_10": t.MinNDCG10,
		"min_mrr": t.MinMRR,
		"min_recall_10": t.MinRecall10,
		"max_zero_result": t.MaxZeroResult,
		"max_duplicate_10": t.MaxDuplicate10,
	} {
		if value < 0 || value > 1 { return fmt.Errorf("quality threshold %s must be between 0 and 1", name) }
	}
	return nil
}
