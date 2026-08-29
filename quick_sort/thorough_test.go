package quick_sort

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// TestRandomizedInts sorts random int slices of sizes 0..2000
// (values drawn from a small range so duplicates are common) and
// cross-checks against the slices.Sort oracle: the result must be
// sorted and a permutation of the input.  Fixed PCG seed for
// reproducibility.
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

// TestRandomizedStructs sorts a struct slice with SortFunc ordered by
// the S field, with duplicate keys, and verifies both the sorted order
// and that the satellite N data rides along: the multiset of (S, N)
// pairs in the output must equal the input's.
func TestRandomizedStructs(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for round := range 500 {
		size := rng.IntN(300)
		original := make([]QsTest, size)
		for i := range original {
			original[i] = QsTest{S: string(rune('a' + rng.IntN(8))), N: rng.IntN(1_000_000)}
		}
		data := slices.Clone(original)
		SortFunc(data, cmpQsTest)

		if !slices.IsSortedFunc(data, cmpQsTest) {
			t.Fatalf("round %d: result is not sorted by S", round)
		}

		// Multiset equality of the whole struct, satellite data included.
		oracle := slices.Clone(original)
		slices.SortFunc(oracle, func(a, b QsTest) int {
			if c := cmpQsTest(a, b); c != 0 {
				return c
			}
			return a.N - b.N
		})
		byN := slices.Clone(data)
		slices.SortFunc(byN, func(a, b QsTest) int {
			if c := cmpQsTest(a, b); c != 0 {
				return c
			}
			return a.N - b.N
		})
		if !slices.Equal(byN, oracle) {
			t.Fatalf("round %d: satellite data lost — output pairs differ from input pairs", round)
		}
	}
}

// TestRandomizedManyDistinctValues is the counterpoint to
// TestRandomizedInts: values from the full int range, so duplicates
// are rare and partitioning does the heavy lifting.
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
