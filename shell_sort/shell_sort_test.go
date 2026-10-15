package shell_sort

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"cmp"
	"slices"
	"strings"
	"testing"
)

// SsTest is the test element type.  Ordering is supplied to SortFunc
// as a plain function (cmpSsTest below) on the S field; N is satellite
// data the comparison ignores.
type SsTest struct {
	S string // the field the comparison function orders by
	N int    // satellite data the comparison function ignores
}

// cmpSsTest orders SsTest by its S field.
func cmpSsTest(a, b SsTest) int {
	return strings.Compare(a.S, b.S)
}

// expectPanic runs fn and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fn()
}

// checkSorted verifies that got is in ascending order and is a
// permutation of original (multiset equality against the slices.Sort
// oracle).
func checkSorted[T cmp.Ordered](t *testing.T, name string, got, original []T) {
	t.Helper()
	if !slices.IsSorted(got) {
		t.Errorf("%s: result is not sorted: %v", name, got)
	}
	oracle := slices.Clone(original)
	slices.Sort(oracle)
	if !slices.Equal(got, oracle) {
		t.Errorf("%s: result is not a permutation of the input: got %v, want %v", name, got, oracle)
	}
}

func TestSortInts(t *testing.T) {
	original := []int{5, 2, 1, 8, 3, 4, 2, 9, 5}
	data := slices.Clone(original)
	Sort(data)
	checkSorted(t, "Sort of ints", data, original)
}

func TestSortStrings(t *testing.T) {
	original := []string{"pear", "apple", "fig", "apple", "date"}
	data := slices.Clone(original)
	Sort(data)
	checkSorted(t, "Sort of strings", data, original)
}

func TestSortFunc(t *testing.T) {
	original := []SsTest{
		{S: "pear", N: 7}, {S: "apple", N: 2}, {S: "fig", N: 5},
		{S: "apple", N: 99},
	}
	data := slices.Clone(original)
	SortFunc(data, cmpSsTest)

	if !slices.IsSortedFunc(data, cmpSsTest) {
		t.Errorf("SortFunc result is not sorted: %v", data)
	}
	// Satellite data rides along: every (S, N) pair of the input must
	// appear exactly once in the output.
	used := make([]bool, len(original))
	for _, got := range data {
		found := false
		for i, want := range original {
			if !used[i] && got == want {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			t.Errorf("element %+v in sorted output does not appear in the input", got)
		}
	}
}

// TestSortFuncNilCmpPanics verifies the package's one panic: SortFunc
// with a nil comparison function, and that the message names SortFunc.
func TestSortFuncNilCmpPanics(t *testing.T) {
	expectPanic(t, "SortFunc(data, nil)", func() { SortFunc([]int{1}, nil) })
	expectPanic(t, "SortFunc(nil, nil)", func() { SortFunc[int](nil, nil) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected SortFunc(data, nil) to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "SortFunc") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		SortFunc([]int{1, 2}, nil)
	}()
}

// TestNilEmptySingle verifies that nil, empty and single-element
// slices are no-ops for both Sort and SortFunc.
func TestNilEmptySingle(t *testing.T) {
	var nilData []int
	Sort(nilData)
	if nilData != nil {
		t.Errorf("Sort must leave a nil slice nil, got %v", nilData)
	}
	SortFunc(nilData, cmp.Compare[int])

	empty := []int{}
	Sort(empty)
	if len(empty) != 0 {
		t.Errorf("Sort of empty slice: expected 0 elements, got %d", len(empty))
	}
	SortFunc(empty, cmp.Compare[int])

	one := []SsTest{{S: "only", N: 1}}
	SortFunc(one, cmpSsTest)
	if one[0] != (SsTest{S: "only", N: 1}) {
		t.Errorf("SortFunc of single element changed it: %v", one[0])
	}
}

// TestGapBoundarySizes sorts slices at and around the sizes where the
// 3x+1 gap schedule changes (n/3 crosses 4, 13, 40, 121, 364, 1093):
// the largest gap must not run off the slice and the schedule must
// always end at h = 1.
func TestGapBoundarySizes(t *testing.T) {
	// h takes values 1,4,13,40,121,364,1093,3280,... as long as h < n/3.
	// n/3 crossing h means n crossing 3h: 3, 12, 39, 120, 363, 1092, ...
	for _, n := range []int{1, 2, 3, 4, 12, 13, 39, 40, 120, 121, 363, 364, 1092, 1093, 1094} {
		original := make([]int, n)
		for i := range original {
			original[i] = (i * 7919) % (3*n + 7) // deterministic scatter, duplicates common
		}
		data := slices.Clone(original)
		Sort(data)
		checkSorted(t, "gap boundary", data, original)
	}
}

// TestAdversarialShapes runs classic problem shapes through Sort:
// all-equal, sorted, reverse-sorted, sawtooth, organ-pipe, two distinct
// values, and sorted-with-one-swap.  Shellsort has no adversarial input
// the way quicksort does — the gap schedule is input-independent — but
// these shapes exercise the boundary logic of the gapped insertion.
func TestAdversarialShapes(t *testing.T) {
	const n = 513

	allEqual := make([]int, n) // all zeros

	sorted := make([]int, n)
	for i := range sorted {
		sorted[i] = i
	}

	reverse := make([]int, n)
	for i := range reverse {
		reverse[i] = n - i
	}

	sawtooth := make([]int, n)
	for i := range sawtooth {
		sawtooth[i] = i % 17
	}

	organPipe := make([]int, n)
	for i := range organPipe {
		if 2*i < n {
			organPipe[i] = i
		} else {
			organPipe[i] = n - i
		}
	}

	twoValues := make([]int, n)
	for i := range twoValues {
		twoValues[i] = i % 2
	}

	oneSwap := make([]int, n)
	for i := range oneSwap {
		oneSwap[i] = i
	}
	oneSwap[n/4], oneSwap[3*n/4] = oneSwap[3*n/4], oneSwap[n/4]

	for name, original := range map[string][]int{
		"all-equal":            allEqual,
		"sorted":               sorted,
		"reverse-sorted":       reverse,
		"sawtooth":             sawtooth,
		"organ-pipe":           organPipe,
		"two-distinct-values":  twoValues,
		"sorted-with-one-swap": oneSwap,
	} {
		data := slices.Clone(original)
		Sort(data)
		checkSorted(t, name, data, original)
	}
}

// TestLargeAdversarial verifies that 1,000,000 already-sorted and
// 250,000 reverse-sorted ints sort correctly.  Shellsort is fully
// iterative — there is no recursion, hence no stack-depth story at any
// size; the test pins correctness at sizes far past the last gap
// transition the small tests reach (1093).
func TestLargeAdversarial(t *testing.T) {
	const n = 1_000_000

	sorted := make([]int, n)
	for i := range sorted {
		sorted[i] = i
	}
	Sort(sorted)
	if !slices.IsSorted(sorted) {
		t.Errorf("Sort of 1e6 already-sorted ints is not sorted")
	}

	const m = 250_000
	reverse := make([]int, m)
	for i := range reverse {
		reverse[i] = m - i
	}
	Sort(reverse)
	for i, v := range reverse {
		if v != i+1 {
			t.Fatalf("Sort of 250k reverse-sorted ints: at subscript %d expected %d got %d", i, i+1, v)
		}
	}
}

// TestSortDescendingComparator verifies that a reversed comparison
// function yields descending order.
func TestSortDescendingComparator(t *testing.T) {
	original := []int{5, 2, 1, 8, 3, 4, 2, 9, 5}
	data := slices.Clone(original)
	SortFunc(data, func(a, b int) int { return b - a })
	if !slices.IsSortedFunc(data, func(a, b int) int { return b - a }) {
		t.Errorf("SortFunc with reversed comparison is not descending: %v", data)
	}
	oracle := slices.Clone(original)
	slices.SortFunc(oracle, func(a, b int) int { return b - a })
	if !slices.Equal(data, oracle) {
		t.Errorf("descending result differs from oracle: got %v, want %v", data, oracle)
	}
}

func BenchmarkSort1K(b *testing.B) {
	base := make([]int, 1000)
	rng := uint32(42)
	for i := range base {
		rng = rng*1664525 + 1013904223
		base[i] = int(rng >> 8)
	}
	data := make([]int, len(base))
	b.ResetTimer()
	for range b.N {
		copy(data, base)
		Sort(data)
	}
}

func BenchmarkSort100K(b *testing.B) {
	base := make([]int, 100_000)
	rng := uint32(42)
	for i := range base {
		rng = rng*1664525 + 1013904223
		base[i] = int(rng >> 8)
	}
	data := make([]int, len(base))
	b.ResetTimer()
	for range b.N {
		copy(data, base)
		Sort(data)
	}
}

// BenchmarkSortManyDuplicates: many duplicate keys help every pass of
// the gapped insertion (equal elements stop the inner loop at one
// compare).
func BenchmarkSortManyDuplicates(b *testing.B) {
	base := make([]int, 100_000)
	rng := uint32(42)
	for i := range base {
		rng = rng*1664525 + 1013904223
		base[i] = int(rng>>8) % 8
	}
	data := make([]int, len(base))
	b.ResetTimer()
	for range b.N {
		copy(data, base)
		Sort(data)
	}
}

// BenchmarkSortSorted: the near-best case — each of the ~log₃ n passes
// is a linear scan that moves nothing.
func BenchmarkSortSorted(b *testing.B) {
	base := make([]int, 100_000)
	for i := range base {
		base[i] = i
	}
	data := make([]int, len(base))
	b.ResetTimer()
	for range b.N {
		copy(data, base)
		Sort(data)
	}
}
