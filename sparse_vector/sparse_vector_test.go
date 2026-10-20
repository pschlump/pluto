package sparse_vector

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"strings"
	"testing"
)

// expectPanic runs fx and fails the test unless it panics; when want is
// non-empty the panic message must contain it.
func expectPanic(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if want != "" {
			if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
				t.Errorf("Unexpected panic message from %s: %v (expected it to contain %q)", name, r, want)
			}
		}
	}()
	fx()
}

// TestPutGetCanonical checks the basics and the canonical form: putting
// zero removes the entry, so zeros are never stored and NNz counts the
// nonzero dimensions.
func TestPutGetCanonical(t *testing.T) {
	v := NewSparseVector[float64](10)
	if v.Dimension() != 10 {
		t.Errorf("Dimension = %d, expected 10", v.Dimension())
	}
	if v.NNz() != 0 {
		t.Errorf("NNz = %d, expected 0 on a fresh vector", v.NNz())
	}
	if _, ok := v.Get(3); ok {
		t.Errorf("Get(3) reported ok on an empty vector")
	}

	if !v.Put(3, 0.50) || !v.Put(9, 0.75) || !v.Put(6, 0.11) {
		t.Errorf("Put of in-range values reported false")
	}
	if v.NNz() != 3 {
		t.Errorf("NNz = %d, expected 3", v.NNz())
	}
	if val, ok := v.Get(3); !ok || val != 0.50 {
		t.Errorf("Get(3) = %v, %v; expected 0.50, true", val, ok)
	}

	// Overwrite keeps one entry; putting zero removes it (the algs4
	// client's a.put(6, 0.00)).
	if !v.Put(6, 0.00) {
		t.Errorf("Put(6, 0) reported false")
	}
	if v.NNz() != 2 {
		t.Errorf("NNz = %d after putting zero, expected 2", v.NNz())
	}
	if _, ok := v.Get(6); ok {
		t.Errorf("Get(6) reported ok after a zero put removed the entry")
	}

	// Re-put after removal works.
	v.Put(6, -2)
	if val, ok := v.Get(6); !ok || val != -2 {
		t.Errorf("Get(6) = %v, %v after re-put; expected -2, true", val, ok)
	}
}

// TestPutOutOfRange checks that out-of-range puts report false and
// change nothing (the heap/fenwick_tree convention), and that a zero
// VALUE vector — dimension 0, no map — behaves the same way rather
// than panicking.
func TestPutOutOfRange(t *testing.T) {
	v := NewSparseVector[int](5)
	if v.Put(-1, 7) || v.Put(5, 7) {
		t.Errorf("Out-of-range Put reported true")
	}
	if v.NNz() != 0 {
		t.Errorf("NNz = %d after rejected puts, expected 0", v.NNz())
	}
	var zero SparseVector[int]
	if zero.Put(0, 7) {
		t.Errorf("Put on the zero value (dimension 0) reported true")
	}
	if _, ok := zero.Get(0); ok {
		t.Errorf("Get on the zero value reported ok")
	}
	if zero.Dimension() != 0 || zero.NNz() != 0 {
		t.Errorf("Zero value reports Dimension/NNz = %d/%d, expected 0/0", zero.Dimension(), zero.NNz())
	}
}

// TestAlgs4Client is the classic client of algs4's SparseVector.java
// (§3.5), which the book prints as
//
//	a = (3, 0.5) (9, 0.75)
//	b = (3, 0.6) (4, 0.9)
//	a dot b = 0.3
//	a + b   = (3, 1.1) (4, 0.9)
//
// The 0.5·0.6 product is exactly the float64 0.3; the 0.5+0.6 sum
// rounds to exactly 1.1, so the output matches the book's.
func TestAlgs4Client(t *testing.T) {
	a := NewSparseVector[float64](10)
	a.Put(3, 0.50)
	a.Put(9, 0.75)
	a.Put(6, 0.11)
	a.Put(6, 0.00)
	b := NewSparseVector[float64](10)
	b.Put(3, 0.60)
	b.Put(4, 0.90)

	if got := a.String(); got != "(3, 0.5) (9, 0.75)" {
		t.Errorf("a = %q, expected %q", got, "(3, 0.5) (9, 0.75)")
	}
	if got := b.String(); got != "(3, 0.6) (4, 0.9)" {
		t.Errorf("b = %q, expected %q", got, "(3, 0.6) (4, 0.9)")
	}
	if prod := a.DotSparse(b); prod != 0.3 {
		t.Errorf("a dot b = %v, expected exactly 0.3", prod)
	}
	if got := a.Plus(b).String(); got != "(3, 1.1) (4, 0.9) (9, 0.75)" {
		t.Errorf("a + b = %q, expected %q", got, "(3, 1.1) (4, 0.9) (9, 0.75)")
	}
}

// TestDotDense checks the sparse·dense product by hand.
func TestDotDense(t *testing.T) {
	v := NewSparseVector[int](5)
	v.Put(1, 3)
	v.Put(3, -2)
	y := []int{10, 20, 30, 40, 50}
	if got := v.Dot(y); got != 3*20-2*40 {
		t.Errorf("Dot = %d, expected %d", got, 3*20-2*40)
	}

	empty := NewSparseVector[int](3)
	if got := empty.Dot([]int{1, 2, 3}); got != 0 {
		t.Errorf("Dot of the zero vector = %d, expected 0", got)
	}
}

// TestDotSparse checks the sparse·sparse product against the sparse·
// dense one on the same data, in both operand orders.
func TestDotSparse(t *testing.T) {
	a := NewSparseVector[float64](8)
	for i, w := range map[int]float64{0: 2, 3: -1.5, 7: 4} {
		a.Put(i, w)
	}
	b := NewSparseVector[float64](8)
	for i, w := range map[int]float64{1: 9, 3: 2, 7: 0.5} {
		b.Put(i, w)
	}
	want := a.Dot(b.Dense())
	if got := a.DotSparse(b); got != want {
		t.Errorf("a·b = %v, sparse·dense says %v", got, want)
	}
	if got := b.DotSparse(a); got != want {
		t.Errorf("b·a = %v, expected the same %v", got, want)
	}

	// Disjoint supports give exactly 0 with no cancellation needed.
	c := NewSparseVector[float64](8)
	c.Put(2, 5)
	if got := a.DotSparse(c); got != 0 {
		t.Errorf("Dot of disjoint supports = %v, expected 0", got)
	}
}

// TestScalePlusImmutable checks that the functional operations leave
// their operands untouched, and that cancelation prunes to canonical
// form (v + (-v) stores nothing).
func TestScalePlusImmutable(t *testing.T) {
	v := NewSparseVector[float64](6)
	v.Put(1, 2)
	v.Put(4, -3)

	s := v.Scale(10)
	if got, ok := s.Get(1); !ok || got != 20 {
		t.Errorf("Scale result Get(1) = %v, %v; expected 20, true", got, ok)
	}
	if got, ok := v.Get(1); !ok || got != 2 {
		t.Errorf("Operand changed by Scale: Get(1) = %v, %v; expected 2, true", got, ok)
	}
	if z := v.Scale(0); z.NNz() != 0 || z.Dimension() != 6 {
		t.Errorf("Scale(0) = nnz %d dim %d, expected 0/6", z.NNz(), z.Dimension())
	}

	p := v.Plus(v.Scale(-1))
	if p.NNz() != 0 {
		t.Errorf("v + (-v) has nnz %d, expected 0 (canonical cancelation)", p.NNz())
	}
	if v.NNz() != 2 {
		t.Errorf("Operand changed by Plus: nnz = %d, expected 2", v.NNz())
	}

	// A nil operand is the zero vector: Plus(nil) is a clone.
	c := v.Plus(nil)
	if c.NNz() != 2 || c.Dimension() != 6 {
		t.Errorf("Plus(nil) = nnz %d dim %d, expected 2/6", c.NNz(), c.Dimension())
	}
}

// TestAxpy checks the in-place row operation by hand, including the
// canonical pruning of entries that cancel to zero.
func TestAxpy(t *testing.T) {
	y := NewSparseVector[int](5)
	y.Put(0, 1)
	y.Put(1, 2)
	y.Put(2, 3)
	x := NewSparseVector[int](5)
	x.Put(1, 1)
	x.Put(3, 5)

	y.Axpy(-2, x) // y = y - 2x = [1, 0, 3, -10, 0]
	want := []int{1, 0, 3, -10, 0}
	if got := y.Dense(); len(got) != len(want) {
		t.Fatalf("Dense length %d, expected %d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Dense[%d] = %d, expected %d", i, got[i], want[i])
			}
		}
	}
	if y.NNz() != 3 {
		t.Errorf("NNz = %d, expected 3 (the 1 that cancelled to 0 is pruned)", y.NNz())
	}
	if _, ok := y.Get(1); ok {
		t.Errorf("Get(1) ok after canceling to zero")
	}

	// A nil operand is the zero vector: no-op.
	before := y.Dense()
	y.Axpy(7, nil)
	if after := y.Dense(); len(after) != len(before) {
		t.Fatalf("Axpy(nil) changed the length")
	} else {
		for i := range before {
			if after[i] != before[i] {
				t.Errorf("Axpy(nil) changed Dense[%d] to %d", i, after[i])
			}
		}
	}
}

// TestEachDenseCloneString checks the remaining surface: Each in
// ascending order with early stop, Dense round-tripping, Clone
// independence, and the empty-vector String.
func TestEachDenseCloneString(t *testing.T) {
	v := NewSparseVector[float64](1000)
	for _, i := range []int{500, 3, 999, 17} {
		v.Put(i, float64(i))
	}

	var seen []int
	v.Each(func(i int, value float64) bool {
		seen = append(seen, i)
		if value != float64(i) {
			t.Errorf("Each(%d) value %v, expected %v", i, value, float64(i))
		}
		return false
	})
	if len(seen) != 4 || seen[0] != 3 || seen[1] != 17 || seen[2] != 500 || seen[3] != 999 {
		t.Errorf("Each visited %v, expected ascending [3 17 500 999]", seen)
	}

	stopped := 0
	v.Each(func(i int, value float64) bool {
		stopped++
		return true // stop on the first (lowest) index
	})
	if stopped != 1 {
		t.Errorf("Early-stop Each ran %d callbacks, expected 1", stopped)
	}

	d := v.Dense()
	if len(d) != 1000 || d[3] != 3 || d[999] != 999 || d[0] != 0 {
		t.Errorf("Dense is not the faithful materialization")
	}

	c := v.Clone()
	v.Put(3, 0)
	if val, ok := c.Get(3); !ok || val != 3 {
		t.Errorf("Clone is not independent: Get(3) = %v, %v", val, ok)
	}

	if got := NewSparseVector[float64](4).String(); got != "" {
		t.Errorf("String of the zero vector = %q, expected empty", got)
	}
}

// TestGenericInstantiations checks that the Numeric constraint accepts
// the predeclared types and types defined from them.
func TestGenericInstantiations(t *testing.T) {
	vi := NewSparseVector[int](3)
	vi.Put(2, 4)
	if got := vi.Dot([]int{1, 1, 2}); got != 8 {
		t.Errorf("int instantiation: Dot = %d, expected 8", got)
	}

	type myFloat float64 // a defined type: Numeric takes ~T, Number would not
	vm := NewSparseVector[myFloat](2)
	vm.Put(0, 1.5)
	if got := vm.Dot([]myFloat{2, 1}); got != 3 {
		t.Errorf("defined-type instantiation: Dot = %v, expected 3", got)
	}
}

// TestAxpyAlias checks v.Axpy(α, v): the loop ranges v.st but only
// ever overwrites or deletes the CURRENT key, never inserts a new one,
// so self-elimination is well defined.
func TestAxpyAlias(t *testing.T) {
	v := NewSparseVector[int](4)
	v.Put(0, 1)
	v.Put(2, 3)
	v.Put(3, -2)
	v.Axpy(2, v) // v = 3v = (0,3) (2,9) (3,-6)
	want := []int{3, 0, 9, -6}
	if got := v.Dense(); len(got) != len(want) {
		t.Fatalf("Dense length %d, expected %d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Dense[%d] = %d, expected %d", i, got[i], want[i])
			}
		}
	}

	// α = -1 self-eliminates to the zero vector, all at once.
	v.Axpy(-1, v)
	if v.NNz() != 0 {
		t.Errorf("v - v has nnz %d, expected 0", v.NNz())
	}
}

// TestPanics checks the whole panic contract: constructor, the two nil
// writes, and the four dimension mismatches.
func TestPanics(t *testing.T) {
	var nilVec *SparseVector[float64]

	expectPanic(t, "NewSparseVector(0)", "d < 1", func() { NewSparseVector[float64](0) })
	expectPanic(t, "NewSparseVector(-5)", "d < 1", func() { NewSparseVector[float64](-5) })

	expectPanic(t, "Put on nil", "sparse_vector: Put", func() { nilVec.Put(0, 1) })
	expectPanic(t, "Axpy on nil", "sparse_vector: Axpy", func() { nilVec.Axpy(1, nilVec) })

	v := NewSparseVector[float64](3)
	w := NewSparseVector[float64](4)
	expectPanic(t, "Dot length mismatch", "sparse_vector: Dot", func() { v.Dot([]float64{1, 2, 3, 4}) })
	expectPanic(t, "DotSparse dimension mismatch", "sparse_vector: DotSparse", func() { v.DotSparse(w) })
	expectPanic(t, "Plus dimension mismatch", "sparse_vector: Plus", func() { v.Plus(w) })
	expectPanic(t, "Axpy dimension mismatch", "sparse_vector: Axpy", func() { v.Axpy(1, w) })
}

// TestNilTolerated verifies that a nil vector answers every read with
// the zero-vector result.
func TestNilTolerated(t *testing.T) {
	var v *SparseVector[float64]

	if val, ok := v.Get(0); ok || val != 0 {
		t.Errorf("nil Get = %v, %v; expected 0, false", val, ok)
	}
	if v.Dimension() != 0 || v.NNz() != 0 {
		t.Errorf("nil Dimension/NNz = %d/%d, expected 0/0", v.Dimension(), v.NNz())
	}
	if got := v.Dot(nil); got != 0 {
		t.Errorf("nil Dot(nil) = %v, expected 0", got)
	}
	if got := v.DotSparse(nil); got != 0 {
		t.Errorf("nil DotSparse(nil) = %v, expected 0", got)
	}
	if got := v.DotSparse(NewSparseVector[float64](1)); got != 0 {
		t.Errorf("nil DotSparse(non-nil) = %v, expected 0", got)
	}
	if v.Scale(2) != nil || v.Clone() != nil || v.Dense() != nil {
		t.Errorf("nil Scale/Clone/Dense should return nil")
	}
	if r := v.Plus(nil); r != nil {
		t.Errorf("nil Plus(nil) should return nil")
	}
	// nil Plus(y) is a clone of y.
	y := NewSparseVector[float64](2)
	y.Put(1, 5)
	if r := v.Plus(y); r.NNz() != 1 || r.Dimension() != 2 {
		t.Errorf("nil Plus(y) = nnz %d dim %d, expected 1/2", r.NNz(), r.Dimension())
	}
	visited := false
	v.Each(func(i int, value float64) bool { visited = true; return false })
	if visited {
		t.Errorf("nil Each visited an entry")
	}
	if got := v.String(); got != "" {
		t.Errorf("nil String = %q, expected empty", got)
	}
}
