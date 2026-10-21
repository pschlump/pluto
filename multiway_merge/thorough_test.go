/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the STABILITY ORACLE — random sorted streams with
// duplicate-heavy keys and unique satellite tags, compared EXACTLY
// (not multiset-equal) against the slices.SortStableFunc oracle on
// the concatenation, which is the unique sorted stable permutation —
// plus the run-merge oracle (k sorted chunks of shuffled data merge
// to the full stable sort, the external-sort use) and the
// merge_sort agreement check.

package multiway_merge

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/merge_sort"
)

// checkStableOracle merges streams with MergeKFunc and verifies the
// result equals the slices.SortStableFunc oracle on the streams'
// concatenation EXACTLY — sortedness, permutation, and the relative
// order of equal-keyed elements (equal output, not merely equal
// multisets, is what stability means; both agree because a stable
// sort of the concatenation orders equals by stream-major position,
// which is the stream-order tie rule of the merge).
func checkStableOracle(t *testing.T, name string, streams [][]MmTest) {
	t.Helper()
	got := MergeKFunc(cmpMmTest, streams...)
	if !slices.IsSortedFunc(got, cmpMmTest) {
		t.Fatalf("%s: result is not sorted by S", name)
	}
	var concat []MmTest
	for _, s := range streams {
		concat = append(concat, s...)
	}
	slices.SortStableFunc(concat, cmpMmTest)
	if !slices.Equal(got, concat) {
		t.Fatalf("%s: STABILITY VIOLATION (or mis-merge): got %+v, stable oracle wants %+v", name, got, concat)
	}
}

// TestRandomizedStabilityOracle is the package's defining
// verification: random numbers of streams of random sizes, keys drawn
// from a small range so equal values collide across streams, and
// unique satellite tags so every out-of-order tag pair among equals
// is visible.  Empty streams are sprinkled in — they must contribute
// nothing.  Fixed PCG seed; both entry points are checked.
func TestRandomizedStabilityOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	mkStreams := func() [][]MmTest {
		k := rng.IntN(17) // 0..16 streams, the degenerate counts included
		streams := make([][]MmTest, k)
		for i := range streams {
			n := rng.IntN(50)
			if rng.IntN(8) == 0 {
				continue // an empty stream among the inputs
			}
			s := make([]MmTest, n)
			for j := range s {
				// Key from a tiny range (heavy cross-stream duplicates);
				// the tag is unique: it encodes stream and position.
				s[j] = MmTest{S: string(rune('a' + rng.IntN(4))), N: i*1000 + j}
			}
			slices.SortStableFunc(s, cmpMmTest) // a sorted input stream
			streams[i] = s
		}
		return streams
	}

	for range 1000 {
		checkStableOracle(t, "MergeKFunc", mkStreams())
	}

	// The cmp.Ordered entry point takes the same ride on plain ints —
	// a separate fixed case set with cross-stream duplicates.
	for range 1000 {
		k := rng.IntN(9)
		streams := make([][]int, k)
		for i := range streams {
			s := make([]int, rng.IntN(40))
			for j := range s {
				s[j] = rng.IntN(5)
			}
			slices.Sort(s)
			streams[i] = s
		}
		got := MergeK(streams...)
		if !slices.IsSorted(got) {
			t.Fatalf("MergeK: result is not sorted: %v", got)
		}
		var concat []int
		for _, s := range streams {
			concat = append(concat, s...)
		}
		slices.Sort(concat)
		if !slices.Equal(got, concat) {
			t.Fatalf("MergeK: result is not a merge of the inputs")
		}
	}
}

// TestRunMergeOracle is the external-sort shape: shuffle a deck, cut
// it into k chunks, sort each chunk (the runs), and merge — the
// result must be the full stable sort of the deck.  Every element
// carries a unique tag, so this is an exact permutation check.
func TestRunMergeOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 23))
	for range 200 {
		n := rng.IntN(500)
		deck := make([]MmTest, n)
		for i := range deck {
			deck[i] = MmTest{S: string(rune('a' + rng.IntN(6))), N: i}
		}
		rng.Shuffle(n, func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

		k := 1 + rng.IntN(12)
		chunk := (n + k - 1) / k
		var runs [][]MmTest
		for lo := 0; lo < n; lo += chunk {
			run := slices.Clone(deck[lo:min(lo+chunk, n)])
			slices.SortStableFunc(run, cmpMmTest)
			runs = append(runs, run)
		}

		got := MergeKFunc(cmpMmTest, runs...)
		want := slices.Clone(deck)
		merge_sort.SortFunc(want, cmpMmTest)
		if !slices.Equal(got, want) {
			t.Fatalf("run merge: k=%d n=%d: got %+v, want %+v", len(runs), n, got, want)
		}
	}
}

// TestAgreesWithMergeSort ties the two packages together: for random
// two-run inputs the k-way merge must equal merge_sort's own stable
// sort of the concatenation, element for element.
func TestAgreesWithMergeSort(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 31))
	for range 500 {
		mk := func() []MmTest {
			s := make([]MmTest, rng.IntN(60))
			for i := range s {
				s[i] = MmTest{S: string(rune('a' + rng.IntN(5))), N: i}
			}
			slices.SortStableFunc(s, cmpMmTest)
			return s
		}
		a, b := mk(), mk()
		got := MergeKFunc(cmpMmTest, a, b)
		want := append(slices.Clone(a), b...)
		merge_sort.SortFunc(want, cmpMmTest)
		if !slices.Equal(got, want) {
			t.Fatalf("two-run agreement: got %+v, merge_sort wants %+v", got, want)
		}
	}
}
