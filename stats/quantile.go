/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stats

import (
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
// Complexity is O(n log k) expected for k distinct required ranks (O(n) for a
// fixed index count), plus one O(n) copy; adversarial inputs fall back to a
// guaranteed-progress pivot and stay sub-quadratic.
func Quantiles(values []float64, scale int, indexes ...int) ([]float64, error) {
	return QuantilesInPlace(append([]float64(nil), values...), scale, indexes...)
}

// QuantilesInPlace computes Type-7 quantiles in one deterministic selection
// pass and may arbitrarily reorder values. Any NaN makes every result NaN.
// Complexity is that of Quantiles without the copy.
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
	selectAll(values, required, 0, len(values)-1, selectDepth(len(values)))
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

// selectInsertionCutoff is the range length at or below which selectAll
// simply insertion-sorts the range, which places every target at once.
const selectInsertionCutoff = 16

// selectAll rearranges values[low..high] so that every position in targets
// (sorted, distinct, all inside low..high) holds the value a full sort would
// put there. It is an introselect in the spirit of Guava's
// Quantiles.selectAllInPlace: deterministic median-of-three / ninther pivots,
// a Hoare partition (three-way when Hoare cannot make progress), and descent
// only into the partitions that still contain targets. depth is the remaining
// budget of cheap-pivot levels; once it is exhausted every further level uses
// the repeated median-of-five pivot (the original implementation's rule),
// which keeps at least about n^0.68 elements on each side, so inputs built to
// defeat median-of-three (or sorted, reversed, organ-pipe, or duplicate-heavy
// data) cannot drive the selection quadratic.
func selectAll(values []float64, targets []int, low, high, depth int) {
	for len(targets) > 0 && low < high {
		if high-low < selectInsertionCutoff {
			insertionSort(values, low, high)
			return
		}
		var pivot float64
		if depth <= 0 {
			pivot = values[medianOfMedians(values, low, high)]
		} else {
			depth--
			pivot = choosePivot(values, low, high)
		}
		lower, upper := partition(values, low, high, pivot)
		leftCount := 0
		for leftCount < len(targets) && targets[leftCount] < lower {
			leftCount++
		}
		middleEnd := leftCount
		for middleEnd < len(targets) && targets[middleEnd] <= upper {
			middleEnd++
		}
		left, right := targets[:leftCount], targets[middleEnd:]
		// Recurse into the side with fewer targets and loop on the other,
		// keeping the recursion shallow.
		if len(left) < len(right) {
			selectAll(values, left, low, lower-1, depth)
			targets, low = right, upper+1
		} else {
			selectAll(values, right, upper+1, high, depth)
			targets, high = left, lower-1
		}
	}
}

// selectDepth is the cheap-pivot level budget for a range of n values.
func selectDepth(n int) int {
	return 2 * bits.Len(uint(n))
}

// choosePivot returns a pivot value from values[low..high]: the median of
// three for short ranges and Tukey's ninther for long ones. Values are not
// moved. The input never contains NaN, so plain comparisons are total.
func choosePivot(values []float64, low, high int) float64 {
	mid := low + (high-low)/2
	if high-low < 128 {
		return median3(values[low], values[mid], values[high])
	}
	step := (high - low) / 8
	return median3(
		median3(values[low], values[low+step], values[low+2*step]),
		median3(values[mid-step], values[mid], values[mid+step]),
		median3(values[high-2*step], values[high-step], values[high]),
	)
}

func median3(a, b, c float64) float64 {
	if b < a {
		a, b = b, a
	}
	if c < b {
		b = c
		if b < a {
			b = a
		}
	}
	return b
}

// insertionSort sorts values[low..high] with plain < comparisons.
func insertionSort(values []float64, low, high int) {
	for i := low + 1; i <= high; i++ {
		value := values[i]
		j := i
		for j > low && value < values[j-1] {
			values[j] = values[j-1]
			j--
		}
		values[j] = value
	}
}

// partition splits values[low..high] around pivot, which must be a value
// present in the range, and returns (lower, upper) such that
// values[low..lower-1] <= pivot, values[lower..upper] == pivot (possibly an
// empty block, upper == lower-1), and values[upper+1..high] >= pivot, with
// both outer parts strictly shorter than the range. The fast path is a Hoare
// partition: elements equal to pivot stop both scans and are swapped, which
// splits runs of duplicates evenly instead of degrading. The input never
// contains NaN, so plain < and > comparisons are total (and agree with
// cmp.Compare, the comparison the original implementation used).
func partition(values []float64, low, high int, pivot float64) (int, int) {
	i, j := low, high
	for {
		// Both scans are bounded without index checks: the pivot value is in
		// the range, and after each swap the swapped pair acts as sentinels.
		for values[i] < pivot {
			i++
		}
		for values[j] > pivot {
			j--
		}
		if i >= j {
			break
		}
		values[i], values[j] = values[j], values[i]
		i++
		j--
	}
	// Now values[low..j] <= pivot <= values[j+1..high] and j >= low. The
	// split makes no progress only when j == high (the pivot is the maximum
	// and the scans met at the last slot); the three-way partition then
	// isolates a non-empty equal block.
	if j >= high {
		return partition3(values, low, high, pivot)
	}
	return j + 1, j
}

// partition3 three-way partitions values[low..high] around pivot and returns
// the bounds of the non-empty block equal to pivot.
func partition3(values []float64, low, high int, pivot float64) (int, int) {
	lower, current, upper := low, low, high
	for current <= upper {
		value := values[current]
		if value < pivot {
			values[lower], values[current] = value, values[lower]
			lower++
			current++
		} else if value > pivot {
			values[current], values[upper] = values[upper], value
			upper--
		} else {
			current++
		}
	}
	return lower, upper
}

// medianOfMedians returns the index of a repeated median-of-five pivot in
// values[low..high]; it reorders the range.
func medianOfMedians(values []float64, low, high int) int {
	count := high - low + 1
	if count <= 5 {
		insertionSort(values, low, high)
		return low + count/2
	}
	medians := 0
	for group := low; group <= high; group += 5 {
		end := min(group+4, high)
		insertionSort(values, group, end)
		median := group + (end-group)/2
		values[low+medians], values[median] = values[median], values[low+medians]
		medians++
	}
	return medianOfMedians(values, low, low+medians-1)
}
