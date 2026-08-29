package segment_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Constructors
// -------------------------------------------------------------------------------------------------------

func TestNewSegmentTree(t *testing.T) {
	data := []int{3, 1, 4, 1, 5, 9, 2, 6}
	st := NewSegmentTree(data)
	if st == nil {
		t.Fatalf("NewSegmentTree returned nil.")
	}
	if st.Len() != len(data) {
		t.Fatalf("Expected Len()=%d, got %d", len(data), st.Len())
	}
	if st.IsEmpty() {
		t.Errorf("Expected IsEmpty()=false on a fresh tree.")
	}
	// Every point value must round-trip, and the full-range query must
	// be the total sum.
	for i, want := range data {
		if v, ok := st.Value(i); !ok || v != want {
			t.Errorf("Expected Value(%d)=%d, got (%d, %v)", i, want, v, ok)
		}
	}
	if s, ok := st.Query(0, len(data)-1); !ok || s != 31 {
		t.Errorf("Expected Query(0,%d)=(31, true), got (%d, %v)", len(data)-1, s, ok)
	}
}

func TestNewSegmentTreeFunc(t *testing.T) {
	data := []int{3, 1, 4, 1, 5, 9, 2, 6}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	st := NewSegmentTreeFunc(data, min, math.MaxInt)
	if st.Len() != len(data) {
		t.Fatalf("Expected Len()=%d, got %d", len(data), st.Len())
	}
	if v, ok := st.Query(0, len(data)-1); !ok || v != 1 {
		t.Errorf("Expected range min=(1, true), got (%d, %v)", v, ok)
	}
	if v, ok := st.Query(4, 7); !ok || v != 2 {
		t.Errorf("Expected range min over [4,7]=(2, true), got (%d, %v)", v, ok)
	}
}

// TestNewSegmentTreePanics verifies the documented constructor panics:
// empty data (both constructors) and a nil combine function.
func TestNewSegmentTreePanics(t *testing.T) {
	for _, data := range [][]int{nil, {}} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewSegmentTree(%v) to panic, it did not.", data)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSegmentTree") {
					t.Errorf("Unexpected panic message for NewSegmentTree: %v", r)
				}
			}()
			NewSegmentTree(data)
		}()
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewSegmentTreeFunc(%v, ...) to panic, it did not.", data)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSegmentTreeFunc") {
					t.Errorf("Unexpected panic message for NewSegmentTreeFunc: %v", r)
				}
			}()
			NewSegmentTreeFunc(data, func(a, b int) int { return a + b }, 0)
		}()
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected NewSegmentTreeFunc with a nil combine to panic, it did not.")
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "nil combine") {
			t.Errorf("Unexpected panic message for nil combine: %v", r)
		}
	}()
	NewSegmentTreeFunc([]int{1, 2}, nil, 0)
}

// -------------------------------------------------------------------------------------------------------
// Query, Update, Value
// -------------------------------------------------------------------------------------------------------

func TestQueryUpdate(t *testing.T) {
	st := NewSegmentTree([]int{1, 2, 3, 4, 5})

	if s, ok := st.Query(0, 0); !ok || s != 1 {
		t.Errorf("Expected Query(0,0)=(1, true), got (%d, %v)", s, ok)
	}
	if s, ok := st.Query(1, 3); !ok || s != 9 {
		t.Errorf("Expected Query(1,3)=(9, true), got (%d, %v)", s, ok)
	}
	if s, ok := st.Query(0, 4); !ok || s != 15 {
		t.Errorf("Expected Query(0,4)=(15, true), got (%d, %v)", s, ok)
	}

	if !st.Update(2, 30) {
		t.Errorf("Expected Update(2, 30) to return true.")
	}
	if v, ok := st.Value(2); !ok || v != 30 {
		t.Errorf("Expected Value(2)=30 after Update, got (%d, %v)", v, ok)
	}
	if s, ok := st.Query(0, 4); !ok || s != 42 {
		t.Errorf("Expected Query(0,4)=(42, true) after Update, got (%d, %v)", s, ok)
	}
	if s, ok := st.Query(3, 4); !ok || s != 9 {
		t.Errorf("Expected Query(3,4)=(9, true) after Update, got (%d, %v)", s, ok)
	}
}

// TestSingleElement verifies the n == 1 corner case (size == 1, the
// root is the only leaf).
func TestSingleElement(t *testing.T) {
	st := NewSegmentTree([]int{7})
	if st.Len() != 1 {
		t.Fatalf("Expected Len()=1, got %d", st.Len())
	}
	if s, ok := st.Query(0, 0); !ok || s != 7 {
		t.Errorf("Expected Query(0,0)=(7, true), got (%d, %v)", s, ok)
	}
	if !st.Update(0, -3) {
		t.Errorf("Expected Update(0, -3) to return true.")
	}
	if s, ok := st.Query(0, 0); !ok || s != -3 {
		t.Errorf("Expected Query(0,0)=(-3, true) after Update, got (%d, %v)", s, ok)
	}
}

// -------------------------------------------------------------------------------------------------------
// Out-of-range indices report, never panic
// -------------------------------------------------------------------------------------------------------

func TestOutOfRange(t *testing.T) {
	const n = 5
	st := NewSegmentTree([]int{1, 2, 3, 4, 5})

	bad := []int{-1, -100, n, n + 1, 1000}
	for _, i := range bad {
		if st.Update(i, 1) {
			t.Errorf("Expected Update(%d, 1) to return false.", i)
		}
		if v, ok := st.Value(i); ok || v != 0 {
			t.Errorf("Expected Value(%d) to return (0, false), got (%d, %v)", i, v, ok)
		}
	}

	// Bad ranges: inverted, out of range on either end.
	for _, r := range [][2]int{{2, 1}, {-1, 2}, {0, n}, {n, n}, {-5, -2}} {
		if s, ok := st.Query(r[0], r[1]); ok || s != 0 {
			t.Errorf("Expected Query(%d,%d) to return (0, false), got (%d, %v)", r[0], r[1], s, ok)
		}
	}

	// The failed operations changed nothing.
	if s, ok := st.Query(0, n-1); !ok || s != 15 {
		t.Errorf("Out-of-range operations disturbed the tree: expected total 15, got (%d, %v)", s, ok)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value tolerance
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil *SegmentTree behaves as an
// empty tree for every read.
func TestNilTolerated(t *testing.T) {
	var nilST *SegmentTree[int]

	if s, ok := nilST.Query(0, 0); ok || s != 0 {
		t.Errorf("Expected Query on a nil tree to return (0, false).")
	}
	if v, ok := nilST.Value(0); ok || v != 0 {
		t.Errorf("Expected Value on a nil tree to return (0, false).")
	}
	if nilST.Len() != 0 {
		t.Errorf("Expected Len()=0 on a nil tree, got %d", nilST.Len())
	}
	if !nilST.IsEmpty() {
		t.Errorf("Expected IsEmpty()=true on a nil tree.")
	}
	// Lock/Unlock are no-ops even on a nil receiver.
	nilST.Lock()
	nilST.Unlock()
}

// TestNilUpdatePanics verifies the package's only method panic: Update
// on a nil *SegmentTree.
func TestNilUpdatePanics(t *testing.T) {
	var nilST *SegmentTree[int]
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected Update on a nil tree to panic, it did not.")
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "Update") {
			t.Errorf("Unexpected panic message: %v", r)
		}
	}()
	nilST.Update(0, 1)
}

// TestZeroValue verifies that the zero value behaves as an empty tree:
// every index is out of range, and even Update reports false (there
// are no slots to update) instead of panicking.
func TestZeroValue(t *testing.T) {
	var st SegmentTree[int]

	if st.Len() != 0 || !st.IsEmpty() {
		t.Errorf("Expected zero value to report Len()=0 and IsEmpty()=true.")
	}
	if _, ok := st.Query(0, 0); ok {
		t.Errorf("Expected Query on the zero value to return ok=false.")
	}
	if _, ok := st.Value(0); ok {
		t.Errorf("Expected Value on the zero value to return ok=false.")
	}
	if st.Update(0, 1) {
		t.Errorf("Expected Update on the zero value to return false (no slots).")
	}
}
