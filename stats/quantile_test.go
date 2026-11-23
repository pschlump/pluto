package stats_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/pschlump/pluto/stats"
)

func sortedQuantiles(values []float64, scale int, indexes []int) []float64 {
	ordered := append([]float64(nil), values...)
	slices.Sort(ordered)
	result := make([]float64, len(indexes))
	for position, index := range indexes {
		numerator := index * (len(ordered) - 1)
		quotient, remainder := numerator/scale, numerator%scale
		if remainder == 0 {
			result[position] = ordered[quotient]
		} else {
			result[position] = ordered[quotient] + (ordered[quotient+1]-ordered[quotient])*float64(remainder)/float64(scale)
		}
	}
	return result
}

func TestQuantilesAgainstSortedOracle(t *testing.T) {
	random := rand.New(rand.NewPCG(1729, 8))
	for size := 1; size <= 150; size++ {
		values := make([]float64, size)
		for index := range values {
			values[index] = float64(random.IntN(21)-10) / 3
		}
		original := append([]float64(nil), values...)
		indexes := []int{0, 1, 2, 3, 4, 2, 0, 4}
		got, err := stats.Quantiles(values, 4, indexes...)
		if err != nil {
			t.Fatal(err)
		}
		want := sortedQuantiles(values, 4, indexes)
		for index := range got {
			requireClose(t, got[index], want[index], 1e-14)
		}
		if !reflect.DeepEqual(values, original) {
			t.Fatal("copying quantiles modified input")
		}
		got, err = stats.QuantilesInPlace(values, 4, indexes...)
		if err != nil {
			t.Fatal(err)
		}
		for index := range got {
			requireClose(t, got[index], want[index], 1e-14)
		}
	}
}

func TestQuantileBoundariesAndDuplicates(t *testing.T) {
	values := []float64{0, 10, 20, 30}
	got, err := stats.Quantiles(values, 4, 0, 1, 2, 3, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{0, 7.5, 15, 22.5, 30, 15}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	median, err := stats.Median(values)
	if err != nil || median != 15 {
		t.Fatalf("median = %v,%v", median, err)
	}
	percentile, err := stats.Percentile(values, 25)
	if err != nil || percentile != 7.5 {
		t.Fatalf("percentile = %v,%v", percentile, err)
	}
	for _, dataset := range [][]float64{{1}, {1, 1, 1}, {0, 1, 2, 3, 4, 5, 6, 7, 8, 9}} {
		if _, err := stats.QuantilesInPlace(dataset, 10, 0, 3, 10); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQuantileNaNAndInfinity(t *testing.T) {
	got, err := stats.Quantiles([]float64{1, math.NaN(), 2}, 4, 0, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range got {
		if !math.IsNaN(value) {
			t.Fatalf("NaN input produced %v", value)
		}
	}
	for _, test := range []struct {
		values []float64
		want   float64
	}{{[]float64{math.Inf(-1), 1}, math.Inf(-1)}, {[]float64{1, math.Inf(1)}, math.Inf(1)}, {[]float64{math.Inf(-1), math.Inf(1)}, math.NaN()}} {
		got, err := stats.Median(test.values)
		if err != nil {
			t.Fatal(err)
		}
		if math.IsNaN(test.want) {
			if !math.IsNaN(got) {
				t.Fatalf("got %v, want NaN", got)
			}
		} else if got != test.want {
			t.Fatalf("got %v, want %v", got, test.want)
		}
	}
}

func TestQuantileErrors(t *testing.T) {
	for _, test := range []struct {
		values  []float64
		scale   int
		indexes []int
		want    error
	}{{nil, 4, []int{1}, stats.ErrEmptyDataset}, {[]float64{1}, 0, []int{0}, stats.ErrScale}, {[]float64{1}, 4, nil, stats.ErrEmptyIndexes}, {[]float64{1}, 4, []int{-1}, stats.ErrIndex}, {[]float64{1}, 4, []int{5}, stats.ErrIndex}} {
		_, err := stats.Quantiles(test.values, test.scale, test.indexes...)
		if !errors.Is(err, test.want) {
			t.Errorf("error = %v, want %v", err, test.want)
		}
	}
}
