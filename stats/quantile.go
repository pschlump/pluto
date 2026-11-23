/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stats

import (
	"cmp"
	"errors"
	"math"
	"math/bits"
	"slices"
)

var (
	ErrEmptyDataset = errors.New("stats: empty dataset")
	ErrEmptyIndexes = errors.New("stats: no quantile indexes")
	ErrScale        = errors.New("stats: quantile scale must be positive")
	ErrIndex        = errors.New("stats: quantile index outside 0..scale")
)

// Quantile computes one Type-7 quantile without modifying values.
func Quantile(values []float64, scale, index int) (float64, error) {
	result, err := Quantiles(values, scale, index)
	if err != nil {
		return 0, err
	}
	return result[0], nil
}

// QuantileInPlace computes one Type-7 quantile and may reorder values.
func QuantileInPlace(values []float64, scale, index int) (float64, error) {
	result, err := QuantilesInPlace(values, scale, index)
	if err != nil {
		return 0, err
	}
	return result[0], nil
}

// Median computes the Type-7 median without modifying values.
func Median(values []float64) (float64, error) {
	return Quantile(values, 2, 1)
}

// Percentile computes percentile 0..100 without modifying values.
func Percentile(values []float64, percentile int) (float64, error) {
	return Quantile(values, 100, percentile)
}

// Quantiles computes Type-7 indexes in one deterministic selection pass.
// Results preserve index order and duplicates, and values is not modified.
func Quantiles(values []float64, scale int, indexes ...int) ([]float64, error) {
	return QuantilesInPlace(append([]float64(nil), values...), scale, indexes...)
}

// QuantilesInPlace computes Type-7 quantiles in one deterministic selection
// pass and may arbitrarily reorder values. Any NaN makes every result NaN.
func QuantilesInPlace(values []float64, scale int, indexes ...int) ([]float64, error) {
	if len(values) == 0 {
		return nil, ErrEmptyDataset
	}
	if scale <= 0 {
		return nil, ErrScale
	}
	if len(indexes) == 0 {
		return nil, ErrEmptyIndexes
	}
	for _, value := range values {
		if math.IsNaN(value) {
			result := make([]float64, len(indexes))
			for index := range result {
				result[index] = math.NaN()
			}
			return result, nil
		}
	}
	quotients := make([]int, len(indexes))
	remainders := make([]int, len(indexes))
	required := make([]int, 0, len(indexes)*2)
	for position, index := range indexes {
		if index < 0 || index > scale {
			return nil, ErrIndex
		}
		high, low := bits.Mul64(uint64(index), uint64(len(values)-1))
		quotient, remainder := bits.Div64(high, low, uint64(scale))
		quotients[position], remainders[position] = int(quotient), int(remainder)
		required = append(required, int(quotient))
		if remainder != 0 {
			required = append(required, int(quotient)+1)
		}
	}
	slices.Sort(required)
	required = slices.Compact(required)
	selectAll(values, required, 0, len(values)-1)
	result := make([]float64, len(indexes))
	for position := range indexes {
		quotient, remainder := quotients[position], remainders[position]
		if remainder == 0 {
			result[position] = values[quotient]
		} else {
			result[position] = interpolate(values[quotient], values[quotient+1], remainder, scale)
		}
	}
	return result, nil
}

func interpolate(lower, upper float64, remainder, scale int) float64 {
	if math.IsInf(lower, -1) {
		if math.IsInf(upper, 1) {
			return math.NaN()
		}
		return lower
	}
	if math.IsInf(upper, 1) {
		return upper
	}
	return lower + (upper-lower)*float64(remainder)/float64(scale)
}

func selectAll(values []float64, targets []int, low, high int) {
	if len(targets) == 0 || low >= high {
		return
	}
	pivotIndex := medianOfMedians(values, low, high)
	pivot := values[pivotIndex]
	lower, upper := partition(values, low, high, pivot)
	leftCount := 0
	for leftCount < len(targets) && targets[leftCount] < lower {
		leftCount++
	}
	middleEnd := leftCount
	for middleEnd < len(targets) && targets[middleEnd] <= upper {
		middleEnd++
	}
	selectAll(values, targets[:leftCount], low, lower-1)
	selectAll(values, targets[middleEnd:], upper+1, high)
}

func partition(values []float64, low, high int, pivot float64) (int, int) {
	lower, current, upper := low, low, high
	for current <= upper {
		comparison := cmp.Compare(values[current], pivot)
		if comparison < 0 {
			values[lower], values[current] = values[current], values[lower]
			lower++
			current++
		} else if comparison > 0 {
			values[current], values[upper] = values[upper], values[current]
			upper--
		} else {
			current++
		}
	}
	return lower, upper
}

func medianOfMedians(values []float64, low, high int) int {
	count := high - low + 1
	if count <= 5 {
		slices.Sort(values[low : high+1])
		return low + count/2
	}
	medians := 0
	for group := low; group <= high; group += 5 {
		end := min(group+4, high)
		slices.Sort(values[group : end+1])
		median := group + (end-group)/2
		values[low+medians], values[median] = values[median], values[low+medians]
		medians++
	}
	return medianOfMedians(values, low, low+medians-1)
}
