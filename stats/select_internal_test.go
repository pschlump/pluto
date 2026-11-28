/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stats

import (
	"cmp"
	"math"
	"math/bits"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// referenceQuantilesInPlace is the pre-2026-11-27 implementation, kept
// verbatim (selection functions renamed with a reference prefix) as the
// differential oracle: the introselect must reproduce its results bit for bit.
func referenceQuantilesInPlace(values []float64, scale int, indexes ...int) ([]float64, error) {
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
	referenceSelectAll(values, required, 0, len(values)-1)
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

func referenceSelectAll(values []float64, targets []int, low, high int) {
	if len(targets) == 0 || low >= high {
		return
	}
	pivotIndex := referenceMedianOfMedians(values, low, high)
	pivot := values[pivotIndex]
	lower, upper := referencePartition(values, low, high, pivot)
	leftCount := 0
	for leftCount < len(targets) && targets[leftCount] < lower {
		leftCount++
	}
	middleEnd := leftCount
	for middleEnd < len(targets) && targets[middleEnd] <= upper {
		middleEnd++
	}
	referenceSelectAll(values, targets[:leftCount], low, lower-1)
	referenceSelectAll(values, targets[middleEnd:], upper+1, high)
}

func referencePartition(values []float64, low, high int, pivot float64) (int, int) {
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

func referenceMedianOfMedians(values []float64, low, high int) int {
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
	return referenceMedianOfMedians(values, low, low+medians-1)
}

// sortOracle computes the same Type-7 results from a full sort.
func sortOracle(values []float64, scale int, indexes []int) []float64 {
	result := make([]float64, len(indexes))
	if slices.ContainsFunc(values, math.IsNaN) {
		for position := range result {
			result[position] = math.NaN()
		}
		return result
	}
	ordered := slices.Clone(values)
	slices.Sort(ordered)
	for position, index := range indexes {
		high, low := bits.Mul64(uint64(index), uint64(len(ordered)-1))
		q, r := bits.Div64(high, low, uint64(scale))
		if r == 0 {
			result[position] = ordered[q]
		} else {
			result[position] = interpolate(ordered[q], ordered[q+1], int(r), scale)
		}
	}
	return result
}

// sameBits reports bitwise equality, treating every NaN as equal.
func sameBits(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// shapedValues builds n values of the named shape. None mixes -0 and +0
// (the sign of a zero order statistic is unspecified when both are present).
func shapedValues(random *rand.Rand, shape string, n int) []float64 {
	values := make([]float64, n)
	for i := range values {
		switch shape {
		case "random":
			values[i] = random.NormFloat64() * 1e3
		case "fewDistinct":
			values[i] = float64(random.IntN(5))
		case "allEqual":
			values[i] = 7.25
		case "sorted":
			values[i] = float64(i)
		case "reverse":
			values[i] = float64(n - i)
		case "organPipe":
			values[i] = float64(min(i, n-1-i))
		case "sawtooth":
			values[i] = float64(i % 17)
		case "benchmark":
			values[i] = float64((i*7919)%1000) / 7
		case "infinities":
			switch random.IntN(6) {
			case 0:
				values[i] = math.Inf(1)
			case 1:
				values[i] = math.Inf(-1)
			default:
				values[i] = float64(random.IntN(9) - 4)
			}
		case "nan":
			values[i] = float64(random.IntN(9))
			if i == n/2 {
				values[i] = math.NaN()
			}
		}
	}
	return values
}

var testShapes = []string{"random", "fewDistinct", "allEqual", "sorted", "reverse", "organPipe", "sawtooth", "benchmark", "infinities", "nan"}

func randomIndexSet(random *rand.Rand, scale int) []int {
	switch random.IntN(4) {
	case 0: // every index, ascending
		indexes := make([]int, scale+1)
		for i := range indexes {
			indexes[i] = i
		}
		return indexes
	case 1: // a single index
		return []int{random.IntN(scale + 1)}
	default: // random order with duplicates
		indexes := make([]int, 1+random.IntN(12))
		for i := range indexes {
			indexes[i] = random.IntN(scale + 1)
		}
		return indexes
	}
}

func TestQuantilesDifferentialAgainstReferenceAndSort(t *testing.T) {
	random := rand.New(rand.NewPCG(20261127, 1))
	sizes := []int{127, 128, 129, 130, 255, 256, 1000, 4099}
	for n := 1; n <= 70; n++ {
		sizes = append(sizes, n)
	}
	scales := []int{1, 2, 3, 4, 7, 10, 100, 997, 1000}
	for _, n := range sizes {
		for _, shape := range testShapes {
			values := shapedValues(random, shape, n)
			for range 4 {
				scale := scales[random.IntN(len(scales))]
				indexes := randomIndexSet(random, scale)
				got, err := QuantilesInPlace(slices.Clone(values), scale, indexes...)
				if err != nil {
					t.Fatal(err)
				}
				reference, err := referenceQuantilesInPlace(slices.Clone(values), scale, indexes...)
				if err != nil {
					t.Fatal(err)
				}
				oracle := sortOracle(values, scale, indexes)
				for i := range got {
					if !sameBits(got[i], reference[i]) || !sameBits(got[i], oracle[i]) {
						t.Fatalf("n=%d shape=%s scale=%d indexes=%v: position %d got %v, reference %v, sort %v",
							n, shape, scale, indexes, i, got[i], reference[i], oracle[i])
					}
				}
			}
		}
	}
}

// TestSelectAllPlacesTargets checks selectAll directly at several depth
// budgets, including 0 (median-of-medians from the first level), so the
// fallback path is exercised on its own, and checks the result is a
// permutation of the input.
func TestSelectAllPlacesTargets(t *testing.T) {
	random := rand.New(rand.NewPCG(42, 7))
	for _, n := range []int{2, 3, 16, 17, 18, 40, 129, 600, 3001} {
		for _, shape := range testShapes {
			if shape == "nan" {
				continue
			}
			values := shapedValues(random, shape, n)
			sorted := slices.Clone(values)
			slices.Sort(sorted)
			for _, depth := range []int{0, 1, 2, selectDepth(n)} {
				for trial := range 3 {
					targets := make([]int, 1+random.IntN(8))
					for i := range targets {
						targets[i] = random.IntN(n)
					}
					if trial == 0 {
						targets = []int{0, n - 1}
					}
					slices.Sort(targets)
					targets = slices.Compact(targets)
					work := slices.Clone(values)
					selectAll(work, targets, 0, n-1, depth)
					for _, target := range targets {
						if !sameBits(work[target], sorted[target]) {
							t.Fatalf("n=%d shape=%s depth=%d target %d: got %v want %v",
								n, shape, depth, target, work[target], sorted[target])
						}
					}
					slices.Sort(work)
					for i := range work {
						if !sameBits(work[i], sorted[i]) {
							t.Fatalf("n=%d shape=%s depth=%d: result is not a permutation", n, shape, depth)
						}
					}
				}
			}
		}
	}
}

// checkPartition verifies the partition contract on values[low..high]:
// <= pivot | == pivot | >= pivot, with both outer parts shorter than the
// range. strict additionally demands a non-empty middle and strict outer
// parts (the three-way partition's contract).
func checkPartition(t *testing.T, name string, values []float64, low, high, lower, upper int, pivot float64, strict bool) {
	t.Helper()
	if lower < low || upper > high || upper < lower-1 || lower-low >= high-low+1 || high-upper >= high-low+1 {
		t.Fatalf("%s: bounds lower=%d upper=%d for range %d..%d", name, lower, upper, low, high)
	}
	if strict && upper < lower {
		t.Fatalf("%s: empty equal block", name)
	}
	for i := low; i <= high; i++ {
		v := values[i]
		switch {
		case i < lower && (v > pivot || strict && v == pivot),
			i >= lower && i <= upper && v != pivot,
			i > upper && (v < pivot || strict && v == pivot):
			t.Fatalf("%s: values[%d]=%v breaks the contract around pivot %v (lower=%d upper=%d): %v",
				name, i, v, pivot, lower, upper, values[low:high+1])
		}
	}
}

// TestPartitionContracts drives partition and partition3 directly, including
// the inputs that force partition onto its three-way fallback (pivot is the
// maximum and the scans meet at the last slot).
func TestPartitionContracts(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 14))
	cases := [][]float64{{1, 2, 3}, {3, 3}, {1, 3}, {2, 1, 3}, {5}, {1, 1, 1, 2}}
	for range 2000 {
		values := make([]float64, 1+random.IntN(40))
		distinct := 1 + random.IntN(8)
		for i := range values {
			values[i] = float64(random.IntN(distinct))
		}
		cases = append(cases, values)
	}
	fallbacks := 0
	for _, values := range cases {
		high := len(values) - 1
		for _, pivotAt := range []int{0, high / 2, high, random.IntN(len(values))} {
			pivot := values[pivotAt]
			work := slices.Clone(values)
			lower, upper := partition(work, 0, high, pivot)
			if upper >= lower {
				fallbacks++
			}
			checkPartition(t, "partition", work, 0, high, lower, upper, pivot, false)
			work = slices.Clone(values)
			lower, upper = partition3(work, 0, high, pivot)
			checkPartition(t, "partition3", work, 0, high, lower, upper, pivot, true)
		}
	}
	if fallbacks == 0 {
		t.Fatal("no case exercised the three-way fallback")
	}
}

// TestQuantilesSignedZeros: with both -0 and +0 present, which zero a
// selection lands on is unspecified (it was in the original too), but every
// result is numerically equal to the sort oracle.
func TestQuantilesSignedZeros(t *testing.T) {
	random := rand.New(rand.NewPCG(5, 5))
	for n := 1; n <= 300; n++ {
		values := make([]float64, n)
		for i := range values {
			switch random.IntN(3) {
			case 0:
				values[i] = math.Copysign(0, -1)
			case 1:
				values[i] = 0
			default:
				values[i] = float64(random.IntN(5) - 2)
			}
		}
		indexes := randomIndexSet(random, 100)
		got, err := QuantilesInPlace(slices.Clone(values), 100, indexes...)
		if err != nil {
			t.Fatal(err)
		}
		oracle := sortOracle(values, 100, indexes)
		for i := range got {
			if got[i] != oracle[i] {
				t.Fatalf("n=%d: got %v want %v", n, got[i], oracle[i])
			}
		}
	}
}

// TestQuantilesAdversarialShapesStayFast is a generous wall-clock guard: a
// quadratic selection would need minutes for these inputs; the introselect
// needs milliseconds.
func TestQuantilesAdversarialShapesStayFast(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	const n = 200000
	random := rand.New(rand.NewPCG(9, 9))
	indexes := make([]int, 101)
	for i := range indexes {
		indexes[i] = i
	}
	start := time.Now()
	for _, shape := range []string{"sorted", "reverse", "organPipe", "sawtooth", "fewDistinct", "allEqual"} {
		values := shapedValues(random, shape, n)
		if _, err := QuantilesInPlace(values, 100, indexes...); err != nil {
			t.Fatal(err)
		}
		values = shapedValues(random, shape, n)
		selectAll(values, []int{0, n / 3, n / 2, n - 1}, 0, n-1, 0) // fallback-only path
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("adversarial shapes took %v", elapsed)
	}
}

// TestQuantilesErrorParityWithReference pins the check order: a NaN input
// returns NaN results before indexes are validated.
func TestQuantilesErrorParityWithReference(t *testing.T) {
	for _, test := range []struct {
		values  []float64
		scale   int
		indexes []int
	}{
		{nil, 4, []int{1}},
		{[]float64{1}, 0, []int{0}},
		{[]float64{1}, 4, nil},
		{[]float64{1, 2}, 4, []int{-1}},
		{[]float64{1, 2}, 4, []int{5}},
		{[]float64{1, math.NaN()}, 4, []int{5}},
		{[]float64{1, math.NaN()}, 0, []int{1}},
	} {
		got, gotErr := QuantilesInPlace(slices.Clone(test.values), test.scale, test.indexes...)
		want, wantErr := referenceQuantilesInPlace(slices.Clone(test.values), test.scale, test.indexes...)
		if gotErr != wantErr || len(got) != len(want) {
			t.Fatalf("%v: got %v,%v want %v,%v", test, got, gotErr, want, wantErr)
		}
		for i := range got {
			if !sameBits(got[i], want[i]) {
				t.Fatalf("%v: got %v want %v", test, got, want)
			}
		}
	}
}

// TestAccumulatorExtremaMatchMathMinMax pins that the builtin min/max used
// by Add and Merge agree bit for bit with math.Min/math.Max on signed zeros,
// infinities and NaN, and that finite matches IsNaN/IsInf.
func TestAccumulatorExtremaMatchMathMinMax(t *testing.T) {
	specials := []float64{math.Copysign(0, -1), 0, 1, -1, math.Inf(1), math.Inf(-1), math.NaN(),
		math.SmallestNonzeroFloat64, math.MaxFloat64, -math.MaxFloat64}
	for _, a := range specials {
		if finite(a) != (!math.IsNaN(a) && !math.IsInf(a, 0)) {
			t.Fatalf("finite(%v) mismatch", a)
		}
		for _, b := range specials {
			var accumulator Accumulator
			accumulator.Add(a)
			accumulator.Add(b)
			if !sameBits(accumulator.min, math.Min(a, b)) || !sameBits(accumulator.max, math.Max(a, b)) {
				t.Fatalf("Add(%v,%v): min %v max %v", a, b, accumulator.min, accumulator.max)
			}
			var left, right Accumulator
			left.Add(a)
			right.Add(b)
			left.Merge(right.Snapshot())
			if !sameBits(left.min, math.Min(a, b)) || !sameBits(left.max, math.Max(a, b)) {
				t.Fatalf("Merge(%v,%v): min %v max %v", a, b, left.min, left.max)
			}
		}
	}
}
