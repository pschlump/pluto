package stats_test

import (
	"math"
	"testing"

	"github.com/pschlump/pluto/stats"
)

func closeEnough(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance*max(1, math.Abs(a), math.Abs(b))
}

func requireClose(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(want) {
		if !math.IsNaN(got) {
			t.Fatalf("got %v, want NaN", got)
		}
		return
	}
	if !closeEnough(got, want, tolerance) {
		t.Fatalf("got %.17g, want %.17g", got, want)
	}
}

func TestAccumulatorKnownData(t *testing.T) {
	var accumulator stats.Accumulator
	accumulator.AddAll(1, 2, 3, 4, 5)
	result := accumulator.Snapshot()
	if result.Count() != 5 {
		t.Fatalf("count = %d", result.Count())
	}
	requireClose(t, result.Sum(), 15, 0)
	requireClose(t, result.Mean(), 3, 0)
	requireClose(t, result.Min(), 1, 0)
	requireClose(t, result.Max(), 5, 0)
	requireClose(t, result.PopulationVariance(), 2, 0)
	requireClose(t, result.SampleVariance(), 2.5, 0)
	requireClose(t, result.PopulationStandardDeviation(), math.Sqrt(2), 0)
	requireClose(t, result.SampleStandardDeviation(), math.Sqrt(2.5), 0)
}

func TestAccumulatorShiftedAndMerge(t *testing.T) {
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
	merged := left.Snapshot()
	want := onePass.Snapshot()
	if merged.Count() != want.Count() {
		t.Fatalf("merged count = %d", merged.Count())
	}
	for name, pair := range map[string][2]float64{
		"mean": {merged.Mean(), want.Mean()}, "sum": {merged.Sum(), want.Sum()},
		"population": {merged.PopulationVariance(), want.PopulationVariance()},
		"sample":     {merged.SampleVariance(), want.SampleVariance()},
		"min":        {merged.Min(), want.Min()}, "max": {merged.Max(), want.Max()},
	} {
		t.Run(name, func(t *testing.T) { requireClose(t, pair[0], pair[1], 2e-7) })
	}
}

func TestEmptyAndNonFiniteStats(t *testing.T) {
	var accumulator stats.Accumulator
	empty := accumulator.Snapshot()
	if empty.Count() != 0 || empty.Sum() != 0 {
		t.Fatal("empty count/sum mismatch")
	}
	for _, value := range []float64{empty.Mean(), empty.Min(), empty.Max(), empty.PopulationVariance(), empty.SampleVariance()} {
		if !math.IsNaN(value) {
			t.Fatalf("empty statistic = %v, want NaN", value)
		}
	}
	accumulator.AddAll(math.Inf(1), 1, 2)
	if !math.IsInf(accumulator.Snapshot().Mean(), 1) || !math.IsNaN(accumulator.Snapshot().PopulationVariance()) {
		t.Fatal("positive infinity semantics mismatch")
	}
	accumulator.Add(math.Inf(-1))
	if !math.IsNaN(accumulator.Snapshot().Mean()) {
		t.Fatal("opposite infinities did not produce NaN")
	}
	var nan stats.Accumulator
	nan.AddAll(1, math.NaN())
	if !math.IsNaN(nan.Snapshot().Min()) || !math.IsNaN(nan.Snapshot().Max()) {
		t.Fatal("NaN min/max semantics mismatch")
	}
}
