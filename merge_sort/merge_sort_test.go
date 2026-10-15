package merge_sort

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

// MsTest is the test element type.  Ordering is supplied to the *Func
// forms as a plain function (cmpMsTest below) on the S field; N is
// satellite data the comparison ignores — it is what makes stability
// observable.
type MsTest struct {
	S string // the field the comparison function orders by
	N int    // satellite data the comparison function ignores
}

// cmpMsTest orders MsTest by its S field.
func cmpMsTest(a, b MsTest) int {
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

	data = slices.Clone(original)
	SortBottomUp(data)
	checkSorted(t, "SortBottomUp of ints", data, original)
}

func TestSortStrings(t *testing.T) {
	original := []string{"pear", "apple", "fig", "apple", "date"}
	data := slices.Clone(original)
	Sort(data)
	checkSorted(t, "Sort of strings", data, original)

	data = slices.Clone(original)
	SortBottomUp(data)
	checkSorted(t, "SortBottomUp of strings", data, original)
}

func TestSortFunc(t *testing.T) {
	original := []MsTest{
		{S: "pear", N: 7}, {S: "apple", N: 2}, {S: "fig", N: 5},
		{S: "apple", N: 99},
	}
	data := slices.Clone(original)
	SortFunc(data, cmpMsTest)

	if !slices.IsSortedFunc(data, cmpMsTest) {
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

// TestStableWitness pins the package's defining property with a small
// input where an unstable sort would reorder equal keys: the two
// 1-keyed and three 2-keyed elements must come out in their original
// relative order (N ascending), through BOTH the top-down and
// bottom-up paths and every merge/skip decision they make.
func TestStableWitness(t *testing.T) {
	original := []MsTest{
		{S: "2", N: 1}, {S: "1", N: 2}, {S: "2", N: 3}, {S: "1", N: 4}, {S: "2", N: 5},
	}
	want := []MsTest{
		{S: "1", N: 2}, {S: "1", N: 4}, {S: "2", N: 1}, {S: "2", N: 3}, {S: "2", N: 5},
	}

	for name, sort := range map[string]func([]MsTest, func(a, b MsTest) int){
		"SortFunc":         SortFunc[MsTest],
		"SortBottomUpFunc": SortBottomUpFunc[MsTest],
	} {
		data := slices.Clone(original)
		sort(data, cmpMsTest)
		if !slices.Equal(data, want) {
			t.Errorf("%s: equal keys were reordered (unstable): got %+v, want %+v", name, data, want)
		}
	}
}

// TestSortFuncNilCmpPanics verifies the package's one panic class: a
// nil comparison function, in each *Func form, with the message
// naming the function.
func TestSortFuncNilCmpPanics(t *testing.T) {
	check := func(name string, fn func()) {
		t.Helper()
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s(data, nil) to panic.", name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, name) {
					t.Errorf("%s panic message does not name the function: %v", name, r)
				}
			}()
			fn()
		}()
	}

	check("SortFunc", func() { SortFunc([]int{1}, nil) })
	check("SortBottomUpFunc", func() { SortBottomUpFunc([]int{1}, nil) })
	check("InversionsFunc", func() { InversionsFunc([]int{1}, nil) })
	expectPanic(t, "SortFunc(nil, nil)", func() { SortFunc[int](nil, nil) })
}

// TestNilEmptySingle verifies that nil, empty and single-element
// slices are no-ops for every sort function and count as 0 for
// Inversions.
func TestNilEmptySingle(t *testing.T) {
	var nilData []int
	Sort(nilData)
	SortBottomUp(nilData)
	SortFunc(nilData, cmp.Compare[int])
	SortBottomUpFunc(nilData, cmp.Compare[int])
	if nilData != nil {
		t.Errorf("Sort must leave a nil slice nil, got %v", nilData)
	}
	if got := Inversions(nilData); got != 0 {
		t.Errorf("Inversions(nil) = %d, want 0", got)
	}

	empty := []int{}
	Sort(empty)
	SortBottomUp(empty)
	if len(empty) != 0 {
		t.Errorf("Sort of empty slice: expected 0 elements, got %d", len(empty))
	}
	if got := Inversions(empty); got != 0 {
		t.Errorf("Inversions(empty) = %d, want 0", got)
	}

	one := []MsTest{{S: "only", N: 1}}
	SortFunc(one, cmpMsTest)
	SortBottomUpFunc(one, cmpMsTest)
	if one[0] != (MsTest{S: "only", N: 1}) {
		t.Errorf("SortFunc of single element changed it: %v", one[0])
	}
	if got := InversionsFunc(one, cmpMsTest); got != 0 {
		t.Errorf("Inversions of single element = %d, want 0", got)
	}
}

// TestBottomUpMatchesTopDown: both variants are stable with the same
// take-left-on-tie rule, so for identical input they must produce the
// identical permutation — not merely two sorted permutations.
func TestBottomUpMatchesTopDown(t *testing.T) {
	original := []int{5, 2, 1, 8, 3, 4, 2, 9, 5, 0, 7, 7, 7, 1, 1, 6, 3}
	td := slices.Clone(original)
	Sort(td)
	bu := slices.Clone(original)
	SortBottomUp(bu)
	if !slices.Equal(td, bu) {
		t.Errorf("top-down and bottom-up differ: %v vs %v", td, bu)
	}
}

// TestAdversarialShapes runs the classic problem shapes through both
// variants.
func TestAdversarialShapes(t *testing.T) {
	const n = 513 // odd, past several merge widths, exercises hi=min(...)

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
		checkSorted(t, name+" (top-down)", data, original)

		data = slices.Clone(original)
		SortBottomUp(data)
		checkSorted(t, name+" (bottom-up)", data, original)
	}
}

// TestLargeAdversarial verifies 1,000,000 already-sorted and 250,000
// reverse-sorted ints through both variants (the already-ordered skip
// makes sorted input nearly free for the top-down sort; the bottom-up
// variant merges it all the same — both must be right).
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
	SortBottomUp(reverse)
	for i, v := range reverse {
		if v != i+1 {
			t.Fatalf("SortBottomUp of 250k reverse-sorted ints: at subscript %d expected %d got %d", i, i+1, v)
		}
	}
}

// TestSortDescendingComparator verifies that a reversed comparison
// function yields descending order.
func TestSortDescendingComparator(t *testing.T) {
	original := []int{5, 2, 1, 8, 3, 4, 2, 9, 5}
	rev := func(a, b int) int { return b - a }

	data := slices.Clone(original)
	SortFunc(data, rev)
	oracle := slices.Clone(original)
	slices.SortFunc(oracle, rev)
	if !slices.Equal(data, oracle) {
		t.Errorf("SortFunc descending differs from oracle: got %v, want %v", data, oracle)
	}

	data = slices.Clone(original)
	SortBottomUpFunc(data, rev)
	if !slices.Equal(data, oracle) {
		t.Errorf("SortBottomUpFunc descending differs from oracle: got %v, want %v", data, oracle)
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

func BenchmarkSortBottomUp100K(b *testing.B) {
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
		SortBottomUp(data)
	}
}

// BenchmarkSortSorted is the top-down sort's best case: the
// already-ordered skip makes each level of the recursion a linear
// scan that never merges.
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

// BenchmarkSortManyDuplicates: many equal keys are every merge's tie
// case — the take-left rule copies them in order with one compare
// each.
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

func BenchmarkInversions100K(b *testing.B) {
	base := make([]int, 100_000)
	rng := uint32(42)
	for i := range base {
		rng = rng*1664525 + 1013904223
		base[i] = int(rng >> 8)
	}
	b.ResetTimer()
	for range b.N {
		_ = Inversions(base)
	}
}
