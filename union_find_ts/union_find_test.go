package union_find_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Constructor and singletons
// -------------------------------------------------------------------------------------------------------

func TestNewUnionFind(t *testing.T) {
	const n = 10
	uf := NewUnionFind(n)
	if uf == nil {
		t.Fatalf("NewUnionFind returned nil.")
	}
	if uf.Len() != n {
		t.Errorf("Expected Len()=%d, got %d", n, uf.Len())
	}
	if uf.Count() != n {
		t.Errorf("Expected Count()=%d for a fresh union-find, got %d", n, uf.Count())
	}
	// Every element starts in its own singleton set: its own root, not
	// connected to anything else.
	for i := range n {
		root, ok := uf.Find(i)
		if !ok {
			t.Fatalf("Find(%d) on a fresh union-find returned ok=false", i)
		}
		if root != i {
			t.Errorf("Expected Find(%d)=%d on a fresh union-find, got %d", i, i, root)
		}
		if uf.Connected(i, (i+1)%n) {
			t.Errorf("Expected %d and %d to be disconnected on a fresh union-find", i, (i+1)%n)
		}
	}
}

// TestNewUnionFindPanics verifies the documented panic for n < 1.
func TestNewUnionFindPanics(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewUnionFind(%d) to panic, it did not.", n)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewUnionFind") {
					t.Errorf("Unexpected panic message for NewUnionFind(%d): %v", n, r)
				}
			}()
			NewUnionFind(n)
		}()
	}
}

// -------------------------------------------------------------------------------------------------------
// Union, Connected, transitivity, Count
// -------------------------------------------------------------------------------------------------------

func TestUnionMerge(t *testing.T) {
	uf := NewUnionFind(6)

	if !uf.Union(0, 1) {
		t.Errorf("Expected first Union(0,1) to return true.")
	}
	if uf.Count() != 5 {
		t.Errorf("Expected Count()=5 after one merge, got %d", uf.Count())
	}
	if !uf.Connected(0, 1) || !uf.Connected(1, 0) {
		t.Errorf("Expected 0 and 1 to be connected after Union(0,1).")
	}

	// A duplicate union merges nothing and reports false.
	if uf.Union(0, 1) || uf.Union(1, 0) {
		t.Errorf("Expected duplicate Union(0,1) to return false.")
	}
	if uf.Count() != 5 {
		t.Errorf("Expected Count() to stay 5 after a duplicate union, got %d", uf.Count())
	}

	// Transitivity: 0-1 and 1-2 put 0 and 2 in the same set.
	if !uf.Union(1, 2) {
		t.Errorf("Expected Union(1,2) to return true.")
	}
	if !uf.Connected(0, 2) {
		t.Errorf("Expected 0 and 2 to be connected through 1.")
	}
	if uf.Union(0, 2) {
		t.Errorf("Expected Union(0,2) to return false (already one set).")
	}

	// Find must agree on the root for every member of a set.
	r0, _ := uf.Find(0)
	r1, _ := uf.Find(1)
	r2, _ := uf.Find(2)
	if r0 != r1 || r1 != r2 {
		t.Errorf("Expected one root for {0,1,2}, got %d, %d, %d", r0, r1, r2)
	}
	r3, _ := uf.Find(3)
	if r3 == r0 {
		t.Errorf("Expected element 3 to have a different root than {0,1,2}.")
	}

	// Merge everything into one set.
	uf.Union(2, 3)
	uf.Union(3, 4)
	uf.Union(4, 5)
	if uf.Count() != 1 {
		t.Errorf("Expected Count()=1 after merging all elements, got %d", uf.Count())
	}
	for p := range 6 {
		for q := range 6 {
			if !uf.Connected(p, q) {
				t.Errorf("Expected %d and %d to be connected in the single remaining set", p, q)
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Out-of-range indices report, never panic
// -------------------------------------------------------------------------------------------------------

func TestOutOfRange(t *testing.T) {
	const n = 5
	uf := NewUnionFind(n)
	uf.Union(0, 1)

	bad := []int{-1, -100, n, n + 1, 1000}
	for _, p := range bad {
		if root, ok := uf.Find(p); ok || root != 0 {
			t.Errorf("Expected Find(%d) to return (0, false), got (%d, %v)", p, root, ok)
		}
		if uf.Union(p, 0) || uf.Union(0, p) || uf.Union(p, p) {
			t.Errorf("Expected Union with out-of-range %d to return false.", p)
		}
		if uf.Connected(p, 0) || uf.Connected(0, p) {
			t.Errorf("Expected Connected with out-of-range %d to return false.", p)
		}
	}
	if uf.Count() != n-1 {
		t.Errorf("Out-of-range unions changed Count(): expected %d, got %d", n-1, uf.Count())
	}
	if !uf.Connected(0, 1) {
		t.Errorf("Out-of-range operations disturbed the existing structure.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value tolerance
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil *UnionFind behaves as an empty
// union-find for every read.
func TestNilTolerated(t *testing.T) {
	var nilUF *UnionFind

	if root, ok := nilUF.Find(0); ok || root != 0 {
		t.Errorf("Expected Find on a nil union-find to return (0, false).")
	}
	if nilUF.Connected(0, 1) {
		t.Errorf("Expected Connected on a nil union-find to return false.")
	}
	if nilUF.Count() != 0 {
		t.Errorf("Expected Count()=0 on a nil union-find, got %d", nilUF.Count())
	}
	if nilUF.Len() != 0 {
		t.Errorf("Expected Len()=0 on a nil union-find, got %d", nilUF.Len())
	}
}

// TestNilUnionPanics verifies the package's only method panic: Union on
// a nil *UnionFind.
func TestNilUnionPanics(t *testing.T) {
	var nilUF *UnionFind
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected Union on a nil union-find to panic, it did not.")
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "Union") {
			t.Errorf("Unexpected panic message: %v", r)
		}
	}()
	nilUF.Union(0, 1)
}

// TestZeroValue verifies that the zero value behaves as an empty
// union-find: every index is out of range, and even Union reports false
// (there are no elements to merge) instead of panicking.
func TestZeroValue(t *testing.T) {
	var uf UnionFind

	if uf.Len() != 0 || uf.Count() != 0 {
		t.Errorf("Expected zero value to report Len()=0 and Count()=0.")
	}
	if _, ok := uf.Find(0); ok {
		t.Errorf("Expected Find on the zero value to return ok=false.")
	}
	if uf.Connected(0, 1) {
		t.Errorf("Expected Connected on the zero value to return false.")
	}
	if uf.Union(0, 1) {
		t.Errorf("Expected Union on the zero value to return false (no elements).")
	}
}
