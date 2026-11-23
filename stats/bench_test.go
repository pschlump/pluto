package stats_test

import (
	"slices"
	"testing"

	"github.com/pschlump/pluto/stats"
)

const statsBenchmarkSize = 10000

var (
	statsValues []float64
	statsSink   []float64
	statsFloat  float64
)

func init() {
	statsValues = make([]float64, statsBenchmarkSize)
	for index := range statsValues {
		statsValues[index] = float64((index*7919)%1000) / 7
	}
}

func BenchmarkStatsAccumulator(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		var accumulator stats.Accumulator
		accumulator.AddAll(statsValues...)
		statsFloat = accumulator.Snapshot().PopulationVariance()
	}
}

func BenchmarkStatsPaired(b *testing.B) {
	y := make([]float64, len(statsValues))
	for index, x := range statsValues {
		y[index] = 3*x + float64(index%17)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		var accumulator stats.PairedAccumulator
		accumulator.AddAll(statsValues, y)
		statsFloat = accumulator.Snapshot().PearsonCorrelation()
	}
}

func BenchmarkStatsQuantiles(b *testing.B) {
	indexes := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	b.ReportAllocs()
	for range b.N {
		statsSink, _ = stats.Quantiles(statsValues, 100, indexes...)
	}
}

func BenchmarkStatsQuantilesSort(b *testing.B) {
	indexes := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	b.ReportAllocs()
	for range b.N {
		ordered := append([]float64(nil), statsValues...)
		slices.Sort(ordered)
		result := make([]float64, len(indexes))
		for position, index := range indexes {
			numerator := index * (len(ordered) - 1)
			quotient, remainder := numerator/100, numerator%100
			if remainder == 0 {
				result[position] = ordered[quotient]
			} else {
				result[position] = ordered[quotient] + (ordered[quotient+1]-ordered[quotient])*float64(remainder)/100
			}
		}
		statsSink = result
	}
}
