package fenwick_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Constructors
// -------------------------------------------------------------------------------------------------------

func TestNewFenwickTree(t *testing.T) {
	const n = 10
	ft := NewFenwickTree[int](n)
	if ft == nil {
		t.Fatalf("NewFenwickTree returned nil.")
	}
	if ft.Len() != n {
		t.Errorf("Expected Len()=%d, got %d", n, ft.Len())
	}
	if ft.IsEmpty() {
		t.Errorf("Expected IsEmpty()=false on a fresh tree.")
	}
	for i := range n {
		if v, ok := ft.Value(i); !ok || v != 0 {
			t.Errorf("Expected Value(%d)=(0, true) on a fresh tree, got (%d, %v)", i, v, ok)
		}
		if s := ft.Sum(i); s != 0 {
			t.Errorf("Expected Sum(%d)=0 on a fresh tree, got %d", i, s)
		}
	}
}

func TestNewFenwickTreeFrom(t *testing.T) {
	data := []int{3, 1, 4, 1, 5, 9, 2, 6}
	ft := NewFenwickTreeFrom(data)
	if ft.Len() != len(data) {
		t.Fatalf("Expected Len()=%d, got %d", len(data), ft.Len())
	}
	prefix := 0
	for i, want := range data {
		prefix += want
		if v, ok := ft.Value(i); !ok || v != want {
			t.Errorf("Expected Value(%d)=%d, got (%d, %v)", i, want, v, ok)
		}
		if s := ft.Sum(i); s != prefix {
			t.Errorf("Expected Sum(%d)=%d, got %d", i, prefix, s)
		}
	}
}

// TestNewFenwickTreePanics verifies the documented constructor panics:
// n < 1 and an empty data slice.
func TestNewFenwickTreePanics(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewFenwickTree(%d) to panic, it did not.", n)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "fenwick_tree_ts: NewFenwickTree") {
					t.Errorf("Unexpected panic message for NewFenwickTree(%d): %v", n, r)
				}
			}()
			NewFenwickTree[int](n)
		}()
	}
	for _, data := range [][]int{nil, {}} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewFenwickTreeFrom(%v) to panic, it did not.", data)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "fenwick_tree_ts: NewFenwickTreeFrom") {
					t.Errorf("Unexpected panic message for NewFenwickTreeFrom: %v", r)
				}
			}()
			NewFenwickTreeFrom(data)
		}()
	}
}

// -------------------------------------------------------------------------------------------------------
// Add, Set, Sum, RangeSum, Value
// -------------------------------------------------------------------------------------------------------

func TestAddSumRangeSum(t *testing.T) {
	const n = 8
	ft := NewFenwickTree[int](n)

	if !ft.Add(2, 10) {
		t.Errorf("Expected Add(2, 10) to return true.")
	}
	if !ft.Add(5, 7) {
		t.Errorf("Expected Add(5, 7) to return true.")
	}
	if s := ft.Sum(1); s != 0 {
		t.Errorf("Expected Sum(1)=0, got %d", s)
	}
	if s := ft.Sum(2); s != 10 {
		t.Errorf("Expected Sum(2)=10, got %d", s)
	}
	if s := ft.Sum(n - 1); s != 17 {
		t.Errorf("Expected Sum(%d)=17, got %d", n-1, s)
	}

	if s, ok := ft.RangeSum(2, 5); !ok || s != 17 {
		t.Errorf("Expected RangeSum(2,5)=(17, true), got (%d, %v)", s, ok)
	}
	if s, ok := ft.RangeSum(3, 4); !ok || s != 0 {
		t.Errorf("Expected RangeSum(3,4)=(0, true), got (%d, %v)", s, ok)
	}

	// Sum(-1) is the empty prefix: zero, no panic.
	if s := ft.Sum(-1); s != 0 {
		t.Errorf("Expected Sum(-1)=0, got %d", s)
	}
}

func TestSetAndValue(t *testing.T) {
	ft := NewFenwickTreeFrom([]int{1, 2, 3, 4, 5})

	if !ft.Set(2, 30) {
		t.Errorf("Expected Set(2, 30) to return true.")
	}
	if v, ok := ft.Value(2); !ok || v != 30 {
		t.Errorf("Expected Value(2)=30 after Set, got (%d, %v)", v, ok)
	}
	if s := ft.Sum(4); s != 42 {
		t.Errorf("Expected Sum(4)=42 after Set, got %d", s)
	}
}

// -------------------------------------------------------------------------------------------------------
// Out-of-range indices report, never panic
// -------------------------------------------------------------------------------------------------------

func TestOutOfRange(t *testing.T) {
	const n = 5
	ft := NewFenwickTree[int](n)
	ft.Add(0, 4)

	bad := []int{-1, -100, n, n + 1, 1000}
	for _, i := range bad {
		if ft.Add(i, 1) {
			t.Errorf("Expected Add(%d, 1) to return false.", i)
		}
		if ft.Set(i, 1) {
			t.Errorf("Expected Set(%d, 1) to return false.", i)
		}
		if v, ok := ft.Value(i); ok || v != 0 {
			t.Errorf("Expected Value(%d) to return (0, false), got (%d, %v)", i, v, ok)
		}
		if s := ft.Sum(i); s != 0 {
			t.Errorf("Expected Sum(%d) to return 0, got %d", i, s)
		}
	}

	for _, r := range [][2]int{{2, 1}, {-1, 2}, {0, n}, {n, n}, {-5, -2}} {
		if s, ok := ft.RangeSum(r[0], r[1]); ok || s != 0 {
			t.Errorf("Expected RangeSum(%d,%d) to return (0, false), got (%d, %v)", r[0], r[1], s, ok)
		}
	}

	if s := ft.Sum(n - 1); s != 4 {
		t.Errorf("Out-of-range operations disturbed the tree: expected Sum(%d)=4, got %d", n-1, s)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value tolerance
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil *FenwickTree behaves as an
// empty tree for every read — and that the nil guard comes before the
// lock acquisition (a nil-receiver lock would itself panic).
func TestNilTolerated(t *testing.T) {
	var nilFT *FenwickTree[int]

	if s := nilFT.Sum(0); s != 0 {
		t.Errorf("Expected Sum on a nil tree to return 0, got %d", s)
	}
	if s, ok := nilFT.RangeSum(0, 0); ok || s != 0 {
		t.Errorf("Expected RangeSum on a nil tree to return (0, false).")
	}
	if v, ok := nilFT.Value(0); ok || v != 0 {
		t.Errorf("Expected Value on a nil tree to return (0, false).")
	}
	if nilFT.Len() != 0 {
		t.Errorf("Expected Len()=0 on a nil tree, got %d", nilFT.Len())
	}
	if !nilFT.IsEmpty() {
		t.Errorf("Expected IsEmpty()=true on a nil tree.")
	}
	// Lock/Unlock are no-ops on a nil receiver.
	nilFT.Lock()
	nilFT.Unlock()
}

// TestNilAddSetPanics verifies the package's two method panics: Add and
// Set on a nil *FenwickTree.
func TestNilAddSetPanics(t *testing.T) {
	var nilFT *FenwickTree[int]
	for _, call := range []struct {
		name string
		fn   func()
	}{
		{"Add", func() { nilFT.Add(0, 1) }},
		{"Set", func() { nilFT.Set(0, 1) }},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on a nil tree to panic, it did not.", call.name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "fenwick_tree_ts: "+call.name) {
					t.Errorf("Unexpected panic message for nil %s: %v", call.name, r)
				}
			}()
			call.fn()
		}()
	}
}

// TestZeroValue verifies that the zero value behaves as an empty tree:
// every index is out of range, and even Add/Set report false (there are
// no slots to update) instead of panicking.
func TestZeroValue(t *testing.T) {
	var ft FenwickTree[int]

	if ft.Len() != 0 || !ft.IsEmpty() {
		t.Errorf("Expected zero value to report Len()=0 and IsEmpty()=true.")
	}
	if s := ft.Sum(0); s != 0 {
		t.Errorf("Expected Sum on the zero value to return 0, got %d", s)
	}
	if _, ok := ft.RangeSum(0, 0); ok {
		t.Errorf("Expected RangeSum on the zero value to return ok=false.")
	}
	if _, ok := ft.Value(0); ok {
		t.Errorf("Expected Value on the zero value to return ok=false.")
	}
	if ft.Add(0, 1) {
		t.Errorf("Expected Add on the zero value to return false (no slots).")
	}
	if ft.Set(0, 1) {
		t.Errorf("Expected Set on the zero value to return false (no slots).")
	}
}

// -------------------------------------------------------------------------------------------------------
// Lock + Nl* compound operations
// -------------------------------------------------------------------------------------------------------

// TestLockNlCompound verifies that a group of Nl-prefixed operations
// under the exposed write lock behaves atomically and matches the
// locked equivalents.
func TestLockNlCompound(t *testing.T) {
	ft := NewFenwickTreeFrom([]int{1, 2, 3, 4, 5})

	ft.Lock()
	if ft.NlLen() != 5 {
		t.Errorf("Expected NlLen()=5, got %d", ft.NlLen())
	}
	v, ok := ft.NlValue(2)
	if !ok || v != 3 {
		t.Errorf("Expected NlValue(2)=(3, true), got (%d, %v)", v, ok)
	}
	// Atomic read-modify-write: double slot 2.
	if !ft.NlSet(2, 2*v) {
		t.Errorf("Expected NlSet(2, 6) to return true.")
	}
	if s := ft.NlSum(2); s != 9 {
		t.Errorf("Expected NlSum(2)=9 after doubling slot 2, got %d", s)
	}
	if !ft.NlAdd(4, 10) {
		t.Errorf("Expected NlAdd(4, 10) to return true.")
	}
	s, ok := ft.NlRangeSum(0, 4)
	ft.Unlock()

	if !ok || s != 28 {
		t.Errorf("Expected NlRangeSum(0,4)=(28, true), got (%d, %v)", s, ok)
	}
	if got := ft.Sum(4); got != 28 {
		t.Errorf("Expected Sum(4)=28 after the compound update, got %d", got)
	}
	if v, _ := ft.Value(2); v != 6 {
		t.Errorf("Expected Value(2)=6 after the compound update, got %d", v)
	}

	// Out-of-range Nl operations report, they do not panic.
	ft.Lock()
	if ft.NlAdd(5, 1) || ft.NlSet(-1, 1) {
		t.Errorf("Expected out-of-range NlAdd/NlSet to return false.")
	}
	if _, ok := ft.NlValue(5); ok {
		t.Errorf("Expected NlValue(5) to return ok=false.")
	}
	if _, ok := ft.NlRangeSum(3, 2); ok {
		t.Errorf("Expected NlRangeSum(3,2) to return ok=false.")
	}
	if s := ft.NlSum(-1); s != 0 {
		t.Errorf("Expected NlSum(-1)=0, got %d", s)
	}
	ft.Unlock()
}
