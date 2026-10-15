/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the STABILITY ORACLE — random struct slices with
// duplicate keys and unique satellite tags, compared EXACTLY against
// the slices.SortStableFunc oracle (equal output, not merely equal
// multisets, is what stability means) — plus the permutation oracle
// for plain values and the brute-force oracle for Inversions.

package merge_sort

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// checkStableOracle sorts data with the given sort function and
// verifies the result equals the slices.SortStableFunc oracle on the
// same input EXACTLY — sortedness, permutation, and the relative
// order of equal-keyed elements.
func checkStableOracle(t *testing.T, name string, original []MsTest, sort func([]MsTest, func(a, b MsTest) int)) {
	t.Helper()
	data := slices.Clone(original)
	sort(data, cmpMsTest)

	if !slices.IsSortedFunc(data, cmpMsTest) {
		t.Fatalf("%s: result is not sorted by S", name)
	}
	oracle := slices.Clone(original)
	slices.SortStableFunc(oracle, cmpMsTest)
	if !slices.Equal(data, oracle) {
		t.Fatalf("%s: STABILITY VIOLATION (or mis-sort): got %+v, stable oracle wants %+v", name, data, oracle)
	}
}

// TestRandomizedStabilityOracle is the package's defining
// verification: keys drawn from a small range (duplicates everywhere)
// with unique satellite tags, so every out-of-order tag pair among
// equals is visible.  Both sort paths are checked against the
// stdlib's stable sort at a fixed PCG seed.
func TestRandomizedStabilityOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for range 1000 {
		size := rng.IntN(400)
		original := make([]MsTest, size)
		for i := range original {
			// Key from a tiny range (heavy duplicates); tag unique.
			original[i] = MsTest{S: string(rune('a' + rng.IntN(6))), N: i}
		}

		checkStableOracle(t, "SortFunc", original, SortFunc[MsTest])
		checkStableOracle(t, "SortBottomUpFunc", original, SortBottomUpFunc[MsTest])
	}
}

// TestStabilityAcrossCutoff focuses the oracle on sizes at and below
// the insertion-sort cutoff (7), where the top-down path never
// reaches a merge — the cutoff's strictly-greater shift must be as
// stable as the merge's take-left rule.
func TestStabilityAcrossCutoff(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for size := 0; size <= insertionCutoff+2; size++ {
		for range 200 {
			original := make([]MsTest, size)
			for i := range original {
				original[i] = MsTest{S: string(rune('a' + rng.IntN(3))), N: i}
			}
			checkStableOracle(t, "cutoff size", original, SortFunc[MsTest])
			checkStableOracle(t, "cutoff size", original, SortBottomUpFunc[MsTest])
		}
	}
}

// TestStabilityAtMergeBoundaries targets inputs where equal keys span
// the mid points the top-down recursion and the bottom-up widths
// choose — exactly the elements a stability bug would shuffle across
// a merge boundary.
func TestStabilityAtMergeBoundaries(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for range 1000 {
		size := 8 + rng.IntN(120)
		original := make([]MsTest, size)
		for i := range original {
			original[i] = MsTest{S: string(rune('a' + (i % (rng.IntN(4) + 1)))), N: i}
		}
		checkStableOracle(t, "boundary", original, SortFunc[MsTest])
		checkStableOracle(t, "boundary", original, SortBottomUpFunc[MsTest])
	}
}

// TestRandomizedInts sorts random int slices of sizes 0..2000 (values
// from a small range so duplicates are common) and cross-checks
// against the slices.Sort oracle: sorted and a permutation of the
// input.  Fixed PCG seed for reproducibility.
func TestRandomizedInts(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for size := range 2001 {
		original := make([]int, size)
		for i := range original {
			original[i] = rng.IntN(100)
		}
		data := slices.Clone(original)
		Sort(data)

		if !slices.IsSorted(data) {
			t.Fatalf("size %d: result is not sorted", size)
		}
		oracle := slices.Clone(original)
		slices.Sort(oracle)
		if !slices.Equal(data, oracle) {
			t.Fatalf("size %d: result is not a permutation of the input", size)
		}

		// The bottom-up variant must produce the same permutation.
		data = slices.Clone(original)
		SortBottomUp(data)
		oracle = slices.Clone(original)
		slices.Sort(oracle)
		if !slices.Equal(data, oracle) {
			t.Fatalf("size %d: bottom-up result is not a permutation of the input", size)
		}
	}
}

// TestRandomizedStrings sorts random strings built from a small
// alphabet (so duplicate keys occur) and cross-checks against
// slices.Sort.
func TestRandomizedStrings(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const letters = "abcdef"

	for round := range 500 {
		size := rng.IntN(400)
		original := make([]string, size)
		for i := range original {
			var sb strings.Builder
			for range rng.IntN(12) {
				sb.WriteByte(letters[rng.IntN(len(letters))])
			}
			original[i] = sb.String()
		}
		data := slices.Clone(original)
		SortFunc(data, strings.Compare)

		if !slices.IsSorted(data) {
			t.Fatalf("round %d: result is not sorted", round)
		}
		oracle := slices.Clone(original)
		slices.Sort(oracle)
		if !slices.Equal(data, oracle) {
			t.Fatalf("round %d: result is not a permutation of the input", round)
		}
	}
}

// TestRandomizedManyDistinctValues is the counterpoint to
// TestRandomizedInts: values from the full int range, so duplicates
// are rare and every merge compares unequal elements.
func TestRandomizedManyDistinctValues(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for round := range 300 {
		size := rng.IntN(2001)
		original := make([]int, size)
		for i := range original {
			original[i] = int(rng.Int64())
		}
		data := slices.Clone(original)
		Sort(data)

		if !slices.IsSorted(data) {
			t.Fatalf("round %d: result is not sorted", round)
		}
		oracle := slices.Clone(original)
		slices.Sort(oracle)
		if !slices.Equal(data, oracle) {
			t.Fatalf("round %d: result is not a permutation of the input", round)
		}
	}
}

// bruteForceInversions is the O(n²) reference for the inversion
// count: count every pair i < j with data[i] > data[j].
func bruteForceInversions(data []int) int64 {
	var n int64
	for i := range data {
		for j := i + 1; j < len(data); j++ {
			if data[i] > data[j] {
				n++
			}
		}
	}
	return n
}

// TestInversionsKnownVectors checks hand-derived counts.
func TestInversionsKnownVectors(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want int64
	}{
		{"sorted", []int{1, 2, 3, 4, 5}, 0},
		{"reverse", []int{5, 4, 3, 2, 1}, 10}, // 5·4/2
		{"all-equal", []int{7, 7, 7, 7}, 0},
		{"one adjacent swap", []int{1, 3, 2, 4}, 1},
		{"minimum at front", []int{4, 1, 2, 3}, 3},
		{"maximum at back", []int{2, 3, 1, 4}, 2},
		{"empty", nil, 0},
		{"single", []int{42}, 0},
	}
	for _, tc := range cases {
		if got := Inversions(tc.in); got != tc.want {
			t.Errorf("%s: Inversions(%v) = %d, want %d", tc.name, tc.in, got, tc.want)
		}
		if got := InversionsFunc(tc.in, cmp.Compare[int]); got != tc.want {
			t.Errorf("%s: InversionsFunc = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestInversionsBruteForce cross-checks the O(n log n) count against
// the O(n²) double loop over random inputs at a fixed PCG seed.
func TestInversionsBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for size := 0; size <= 400; size++ {
		data := make([]int, size)
		for i := range data {
			data[i] = rng.IntN(50) // duplicates common
		}
		want := bruteForceInversions(data)
		if got := Inversions(data); got != want {
			t.Fatalf("size %d: Inversions = %d, brute force says %d", size, got, want)
		}
	}
}

// TestInversionsBig pins the int64 return past the 32-bit range:
// 100,000 reverse-sorted ints have 4,999,950,000 inversions.
func TestInversionsBig(t *testing.T) {
	const n = 100_000
	data := make([]int, n)
	for i := range data {
		data[i] = n - i
	}
	want := int64(n) * int64(n-1) / 2
	if got := Inversions(data); got != want {
		t.Errorf("Inversions of 100k reverse-sorted = %d, want %d", got, want)
	}
}

// TestInversionsInputNotModified verifies the purity contract:
// Inversions sorts only a private copy.
func TestInversionsInputNotModified(t *testing.T) {
	data := []int{5, 2, 1, 8, 3}
	orig := slices.Clone(data)
	if got := Inversions(data); got != 5 {
		t.Errorf("Inversions = %d, want 5", got)
	}
	if !slices.Equal(data, orig) {
		t.Errorf("Inversions modified its input: %v -> %v", orig, data)
	}
}

// TestInversionsZeroAfterSort: a sorted slice has no inversions —
// including one that was just produced by this package.
func TestInversionsZeroAfterSort(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for round := range 200 {
		data := make([]int, rng.IntN(500))
		for i := range data {
			data[i] = rng.IntN(1000)
		}
		Sort(data)
		if got := Inversions(data); got != 0 {
			t.Fatalf("round %d: Inversions of a sorted slice = %d, want 0", round, got)
		}
	}
}
