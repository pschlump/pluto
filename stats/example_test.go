package stats_test

import (
	"fmt"

	"github.com/pschlump/pluto/stats"
)

func ExampleAccumulator() {
	var accumulator stats.Accumulator
	accumulator.AddAll(1, 2, 3, 4, 5)
	result := accumulator.Snapshot()
	fmt.Println(result.Count(), result.Mean(), result.PopulationVariance())
	// Output: 5 3 2
}

func ExampleQuantiles() {
	values, err := stats.Quantiles([]float64{0, 10, 20, 30}, 4, 1, 2, 3)
	fmt.Println(values, err)
	// Output: [7.5 15 22.5] <nil>
}
