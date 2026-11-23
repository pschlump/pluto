package stats_test

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/stats"
)

type guavaStats struct {
	Count                       uint64 `json:"count"`
	Sum                         string `json:"sum"`
	Mean                        string `json:"mean"`
	Min                         string `json:"min"`
	Max                         string `json:"max"`
	PopulationVariance          string `json:"populationVariance"`
	SampleVariance              string `json:"sampleVariance"`
	PopulationStandardDeviation string `json:"populationStandardDeviation"`
	SampleStandardDeviation     string `json:"sampleStandardDeviation"`
}

type guavaStatsRoot struct {
	Basic      guavaStats        `json:"basic"`
	Merged     guavaStats        `json:"merged"`
	OnePass    guavaStats        `json:"onePass"`
	Paired     map[string]string `json:"paired"`
	Horizontal struct {
		IsHorizontal bool   `json:"isHorizontal"`
		Transform99  string `json:"transform99"`
	} `json:"horizontal"`
	Quantiles struct {
		Scale   int      `json:"scale"`
		Indexes []int    `json:"indexes"`
		Values  []string `json:"values"`
	} `json:"quantiles"`
}

func parseGuavaFloat(t *testing.T, value string) float64 {
	t.Helper()
	switch value {
	case "Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func compareGuavaStats(t *testing.T, got stats.Stats, want guavaStats) {
	t.Helper()
	if got.Count() != want.Count {
		t.Errorf("count = %d, want %d", got.Count(), want.Count)
	}
	pairs := [][2]float64{
		{got.Sum(), parseGuavaFloat(t, want.Sum)}, {got.Mean(), parseGuavaFloat(t, want.Mean)},
		{got.Min(), parseGuavaFloat(t, want.Min)}, {got.Max(), parseGuavaFloat(t, want.Max)},
		{got.PopulationVariance(), parseGuavaFloat(t, want.PopulationVariance)},
		{got.SampleVariance(), parseGuavaFloat(t, want.SampleVariance)},
		{got.PopulationStandardDeviation(), parseGuavaFloat(t, want.PopulationStandardDeviation)},
		{got.SampleStandardDeviation(), parseGuavaFloat(t, want.SampleStandardDeviation)},
	}
	for _, pair := range pairs {
		requireClose(t, pair[0], pair[1], 2e-7)
	}
}

func TestGuavaStatsVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_stats.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector guavaStatsRoot
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	var basic stats.Accumulator
	basic.AddAll(1, 2, 3, 4, 5)
	compareGuavaStats(t, basic.Snapshot(), vector.Basic)
	var onePass, left, right stats.Accumulator
	for index := 0; index < 1000; index++ {
		value := 1e12 + float64((index*7919)%101)
		onePass.Add(value)
		if index%2 == 0 {
			left.Add(value)
		} else {
			right.Add(value)
		}
	}
	left.Merge(right.Snapshot())
	compareGuavaStats(t, onePass.Snapshot(), vector.OnePass)
	compareGuavaStats(t, left.Snapshot(), vector.Merged)

	var paired stats.PairedAccumulator
	for x := 0.0; x < 10; x++ {
		paired.Add(x, 3*x+2)
	}
	fit := paired.Snapshot().LeastSquaresFit()
	slope, _ := fit.Slope()
	transform4, _ := fit.Transform(4)
	inverse14, _ := fit.Inverse().Transform(14)
	for name, pair := range map[string][2]float64{
		"correlation": {paired.Snapshot().PearsonCorrelation(), parseGuavaFloat(t, vector.Paired["correlation"])},
		"slope":       {slope, parseGuavaFloat(t, vector.Paired["slope"])},
		"transform4":  {transform4, parseGuavaFloat(t, vector.Paired["transform4"])},
		"inverse14":   {inverse14, parseGuavaFloat(t, vector.Paired["inverse14"])},
	} {
		t.Run(name, func(t *testing.T) { requireClose(t, pair[0], pair[1], 1e-13) })
	}
	var horizontal stats.PairedAccumulator
	horizontal.AddAll([]float64{1, 2, 3}, []float64{7, 7, 7})
	horizontalFit := horizontal.Snapshot().LeastSquaresFit()
	transform99, _ := horizontalFit.Transform(99)
	if horizontalFit.IsHorizontal() != vector.Horizontal.IsHorizontal {
		t.Fatal("horizontal vector mismatch")
	}
	requireClose(t, transform99, parseGuavaFloat(t, vector.Horizontal.Transform99), 0)
	quantiles, err := stats.Quantiles([]float64{0, 10, 20, 30}, vector.Quantiles.Scale, vector.Quantiles.Indexes...)
	if err != nil {
		t.Fatal(err)
	}
	for index, text := range vector.Quantiles.Values {
		requireClose(t, quantiles[index], parseGuavaFloat(t, text), 0)
	}
}
