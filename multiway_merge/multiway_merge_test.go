/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_merge

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// MmTest is the test element type.  Ordering is supplied to the *Func
// form as a plain function (cmpMmTest below) on the S field; N is
// satellite data the comparison ignores — it is what makes stability
// observable.
type MmTest struct {
	S string // the field the comparison function orders by
	N int    // satellite data the comparison function ignores
}

// cmpMmTest orders MmTest by its S field.
func cmpMmTest(a, b MmTest) int {
	return strings.Compare(a.S, b.S)
}

// expectPanic runs fn and fails the test unless it panics with the
// given message.
func expectPanic(t *testing.T, name, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if got, ok := r.(string); !ok || got != want {
			t.Errorf("%s panicked with %v, want %q", name, r, want)
		}
	}()
	fn()
}

// checkMerged verifies that got is sorted by cmpFn and contains
// exactly the elements of the concatenation of streams (multiset
// equality against the sorted-concatenation oracle).
func checkMerged[T any](t *testing.T, name string, got []T, fx func(a, b T) int, streams [][]T) {
	t.Helper()
	if !slices.IsSortedFunc(got, fx) {
		t.Errorf("%s: result is not sorted: %v", name, got)
	}
	var concat []T
	for _, s := range streams {
		concat = append(concat, s...)
	}
	oracle := slices.Clone(concat)
	slices.SortFunc(oracle, fx)
	if !slices.EqualFunc(got, oracle, func(a, b T) bool { return fx(a, b) == 0 }) {
		t.Errorf("%s: result is not a merge of the inputs: got %v, want %v", name, got, oracle)
	}
}

func TestMergeKInts(t *testing.T) {
	streams := [][]int{
		{1, 4, 7, 10},
		{2, 5, 8},
		{3, 6, 9, 12, 15},
	}
	got := MergeK(streams...)
	checkMerged(t, "MergeK of ints", got, func(a, b int) int { return a - b }, streams)
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15}
	if !slices.Equal(got, want) {
		t.Errorf("MergeK of ints: got %v, want %v", got, want)
	}
}

func TestMergeKStrings(t *testing.T) {
	streams := [][]string{
		{"apple", "fig", "pear"},
		{"apple", "date"},
		{"banana", "fig", "fig"},
	}
	got := MergeK(streams...)
	checkMerged(t, "MergeK of strings", got, strings.Compare, streams)
	want := []string{"apple", "apple", "banana", "date", "fig", "fig", "fig", "pear"}
	if !slices.Equal(got, want) {
		t.Errorf("MergeK of strings: got %v, want %v", got, want)
	}
}

func TestMergeKFunc(t *testing.T) {
	streams := [][]MmTest{
		{{S: "pear", N: 0}, {S: "pear", N: 1}},
		{{S: "apple", N: 2}, {S: "fig", N: 3}, {S: "fig", N: 4}},
	}
	got := MergeKFunc(cmpMmTest, streams...)
	checkMerged(t, "MergeKFunc of structs", got, cmpMmTest, streams)
}

// TestMergeKStableWitness pins the stability contract with a
// hand-traced merge: three streams whose heads are all equal, so the
// stream order — not heap order — decides the output.
func TestMergeKStableWitness(t *testing.T) {
	streams := [][]MmTest{
		{{S: "m", N: 10}, {S: "m", N: 11}, {S: "z", N: 12}},
		{{S: "a", N: 20}, {S: "m", N: 21}},
		{{S: "m", N: 30}, {S: "m", N: 31}},
	}
	got := MergeKFunc(cmpMmTest, streams...)
	want := []MmTest{
		{S: "a", N: 20},
		{S: "m", N: 10}, {S: "m", N: 11}, // stream 0's equals, in order
		{S: "m", N: 21},                  // then stream 1's
		{S: "m", N: 30}, {S: "m", N: 31}, // then stream 2's
		{S: "z", N: 12},
	}
	if !slices.Equal(got, want) {
		t.Errorf("stability witness: got %v, want %v", got, want)
	}
}

// TestMergeKTwoRuns is the k=2 special case: MergeK(a, b) must agree
// exactly with merge_sort's stable two-run merge, which is the
// slices.SortStableFunc oracle on the concatenation.
func TestMergeKTwoRuns(t *testing.T) {
	a := []MmTest{{S: "b", N: 0}, {S: "d", N: 1}, {S: "d", N: 2}}
	b := []MmTest{{S: "b", N: 3}, {S: "c", N: 4}, {S: "d", N: 5}}
	got := MergeKFunc(cmpMmTest, a, b)
	oracle := append(slices.Clone(a), b...)
	slices.SortStableFunc(oracle, cmpMmTest)
	if !slices.Equal(got, oracle) {
		t.Errorf("two-run merge: got %v, want the stable merge %v", got, oracle)
	}
}

// TestMergeKEdges covers the tolerated degenerate inputs: no streams,
// all-empty streams, nil streams among the inputs, a single stream
// (result is a copy), and streams of one element.
func TestMergeKEdges(t *testing.T) {
	if got := MergeK[int](); got == nil || len(got) != 0 {
		t.Errorf("MergeK with no streams: got %v, want empty non-nil", got)
	}
	if got := MergeK([]int{}, nil, []int{}); got == nil || len(got) != 0 {
		t.Errorf("MergeK of empty streams: got %v, want empty non-nil", got)
	}

	one := []int{2, 4, 6}
	got := MergeK(one, nil, []int{})
	if want := []int{2, 4, 6}; !slices.Equal(got, want) {
		t.Errorf("MergeK of one stream: got %v, want %v", got, want)
	}
	if one[0] != 2 || &one[0] == &got[0] {
		t.Errorf("MergeK of one stream: input %v modified or result aliases it", one)
	}

	got = MergeK([]int{5}, []int{1}, []int{3}, []int{1})
	if want := []int{1, 1, 3, 5}; !slices.Equal(got, want) {
		t.Errorf("MergeK of singleton streams: got %v, want %v", got, want)
	}
}

// TestMergeKInputNotModified pins the purity contract: the input
// streams are read, never written.
func TestMergeKInputNotModified(t *testing.T) {
	streams := [][]int{{1, 3, 5}, {2, 4}, {0, 6}}
	before := make([][]int, len(streams))
	for i, s := range streams {
		before[i] = slices.Clone(s)
	}
	_ = MergeK(streams...)
	for i := range streams {
		if !slices.Equal(streams[i], before[i]) {
			t.Errorf("input stream %d modified: got %v, want %v", i, streams[i], before[i])
		}
	}
}

// TestMergeKDescending merges with a reversed comparison — the
// descending merge, the same trick that turns the heap family into
// max-first.
func TestMergeKDescending(t *testing.T) {
	streams := [][]int{{9, 5, 1}, {10, 2}, {7, 3}}
	rev := func(a, b int) int { return b - a }
	got := MergeKFunc(rev, streams...)
	want := []int{10, 9, 7, 5, 3, 2, 1}
	if !slices.Equal(got, want) {
		t.Errorf("descending merge: got %v, want %v", got, want)
	}
}

func TestMergeKFuncNilPanics(t *testing.T) {
	expectPanic(t, "MergeKFunc with a nil comparison function",
		"multiway_merge: MergeKFunc called with a nil comparison function",
		func() { MergeKFunc[int](nil, []int{1}) })
}

// BenchmarkMergeK merges k pre-sorted streams of ints totaling 100k
// elements — the external-sort run merge.  The streams are built once
// at a fixed PCG seed; k varies because log k is the whole story.
// BenchmarkSlicesSort is the baseline the pattern exists to beat: a
// full sort of the same 100k elements.
func benchStreams(k int) [][]int {
	rng := rand.New(rand.NewPCG(42, 23))
	streams := make([][]int, k)
	for i := range streams {
		s := make([]int, 100_000/k)
		for j := range s {
			s[j] = rng.IntN(1_000_000)
		}
		slices.Sort(s)
		streams[i] = s
	}
	return streams
}

func BenchmarkMergeK(b *testing.B) {
	for _, k := range []int{2, 16, 256} {
		streams := benchStreams(k)
		b.Run("k="+strconv.Itoa(k), func(b *testing.B) {
			for b.Loop() {
				_ = MergeK(streams...)
			}
		})
	}
}

func BenchmarkSlicesSort(b *testing.B) {
	var concat []int
	for _, s := range benchStreams(16) {
		concat = append(concat, s...)
	}
	b.ResetTimer()
	for b.Loop() {
		data := slices.Clone(concat)
		slices.Sort(data)
	}
}
