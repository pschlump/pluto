/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package sparse_vector implements dimension-indexed sparse vectors of
// any numeric type (Sedgwick & Wayne, algs4 §3.5, SparseVector.java) —
// a d-dimensional vector stored as a hash map from dimension index to
// the nonzero entries, with the dot and axpy operations that sparse
// Gaussian elimination and simplex are built from.
//
// The stored form is canonical: Put(i, 0) removes the entry, so a zero
// is never stored and NNz is exactly the count of nonzero dimensions.
// Entries are values in a plain map[int]T — no node objects, no
// boxing.
//
// Basic operations:
//
//	Put — Assigns value to dimension i (zero removes).	O(1) avg
//	Get — Returns the entry at i and whether one is set.	O(1) avg
//	NNz — Number of stored (nonzero) entries.				O(1)
//	Dimension — The dimension d, fixed at construction.	O(1)
//	Each — Visits the entries in ascending index order.	O(nnz log nnz)
//	Dot — Sparse · dense product.							O(nnz)
//	DotSparse — Sparse · sparse product.					O(min nnz) avg
//	Scale — Returns α·v as a new vector.					O(nnz)
//	Plus — Returns v + y as a new vector.					O(nnz v + nnz y)
//	Axpy — v += α·x, in place (the row operation).			O(nnz x) avg
//	Clone — Independent copy.								O(nnz)
//	Dense — Materializes the full []T of length d.			O(d)
//
// The package has exactly seven panics:
//
//	NewSparseVector(d) with d < 1 — a vector over no dimensions is
//	meaningless.
//	Put on a nil *SparseVector, and Axpy on a nil *SparseVector — the
//	writes with no sane answer.
//	Dot with a dense operand whose length is not d, and DotSparse /
//	Plus / Axpy with an operand of a different dimension — the
//	operations are not defined on mismatched dimensions.
//
// Everything else reports rather than panics (the heap and
// fenwick_tree convention): Put returns false for an out-of-range
// index, Get reports ok=false, and a nil vector or the zero value
// behaves as the zero vector of dimension 0 for every read — Dot of an
// empty dense operand is 0, a nil operand in DotSparse/Axpy is the
// zero vector, Plus with a nil operand clones the receiver, Each
// visits nothing.
//
// The package is not safe for concurrent use — see the thread-safe
// twin sparse_vector_ts.
package sparse_vector

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/pschlump/pluto/g_lib"
)

// SparseVector is a d-dimensional vector storing only its nonzero
// entries, keyed by dimension index in a hash map.
type SparseVector[T g_lib.Numeric] struct {
	d  int
	st map[int]T
}

// NewSparseVector returns the zero vector of dimension d — indices
// 0..d-1, every entry zero, nothing stored.  It panics if d < 1.
// Complexity is O(1).
func NewSparseVector[T g_lib.Numeric](d int) *SparseVector[T] {
	if d < 1 {
		panic("sparse_vector: NewSparseVector called with d < 1")
	}
	return &SparseVector[T]{d: d, st: make(map[int]T)}
}

// store keeps the canonical form: a zero value is deleted from the
// map, never stored.
func store[T g_lib.Numeric](st map[int]T, i int, value T) {
	if value == 0 {
		delete(st, i)
	} else {
		st[i] = value
	}
}

// Put assigns value to dimension i and returns true.  Assigning zero
// REMOVES the entry, keeping the stored form canonical.  It returns
// false — changing nothing — if i is out of range.
//
// It panics on a nil *SparseVector: a nil vector cannot record an
// entry.
// Complexity is O(1) on average.
func (v *SparseVector[T]) Put(i int, value T) bool {
	if v == nil {
		panic("sparse_vector: Put called on a nil SparseVector")
	}
	if i < 0 || i >= v.d {
		return false
	}
	store(v.st, i, value)
	return true
}

// Get returns the entry at dimension i and whether one is stored.
// ok is false — and the value the zero value of T — when i is out of
// range or nothing is stored there; zeros are never stored, so ok is
// exactly "dimension i is nonzero".  A nil vector or the zero value
// reports (0, false) for every index.
// Complexity is O(1) on average.
func (v *SparseVector[T]) Get(i int) (rv T, ok bool) {
	if v == nil || i < 0 || i >= v.d {
		return rv, false
	}
	rv, ok = v.st[i]
	return
}

// Dimension returns d, the number of dimensions (0..d-1) fixed at
// construction.  A nil vector or the zero value reports 0.
// Complexity is O(1).
func (v *SparseVector[T]) Dimension() int {
	if v == nil {
		return 0
	}
	return v.d
}

// NNz returns the number of stored entries — the count of nonzero
// dimensions.  A nil vector or the zero value reports 0.
// Complexity is O(1).
func (v *SparseVector[T]) NNz() int {
	if v == nil {
		return 0
	}
	return len(v.st)
}

// Each visits the stored entries in ascending dimension order; fx
// returning true stops the walk.  A nil vector visits nothing.
// Complexity is O(nnz log nnz).
func (v *SparseVector[T]) Each(fx func(i int, value T) bool) {
	if v == nil {
		return
	}
	for _, i := range slices.Sorted(maps.Keys(v.st)) {
		if fx(i, v.st[i]) {
			return
		}
	}
}

// Dot returns the inner product of v with the dense vector y.
// It panics if len(y) is not Dimension() — the product is not defined
// on mismatched dimensions.  A nil v is the zero vector, so Dot of an
// empty dense operand on a nil receiver is 0.
// Complexity is O(nnz) — proportional to the stored entries, not d.
func (v *SparseVector[T]) Dot(y []T) T {
	var sum T
	if v == nil {
		if len(y) != 0 {
			panic(fmt.Sprintf("sparse_vector: Dot called with a dense operand of length %d, want dimension %d", len(y), 0))
		}
		return sum
	}
	if len(y) != v.d {
		panic(fmt.Sprintf("sparse_vector: Dot called with a dense operand of length %d, want dimension %d", len(y), v.d))
	}
	for i, val := range v.st {
		sum += val * y[i]
	}
	return sum
}

// DotSparse returns the inner product of two sparse vectors, iterating
// the sparser operand and probing the denser one.  A nil operand is
// the zero vector: the product is 0.  It panics if the dimensions
// differ.
// Complexity is O(min(nnz(v), nnz(y))) on average.
func (v *SparseVector[T]) DotSparse(y *SparseVector[T]) T {
	var sum T
	if v == nil || y == nil {
		return sum
	}
	if v.d != y.d {
		panic(fmt.Sprintf("sparse_vector: DotSparse called with a dimension-%d operand, want %d", y.d, v.d))
	}
	small, big := v.st, y.st
	if len(small) > len(big) {
		small, big = big, small
	}
	for i, val := range small {
		if bv, ok := big[i]; ok {
			sum += val * bv
		}
	}
	return sum
}

// Scale returns α·v as a NEW vector; v is unchanged.  Scaling by zero
// returns the zero vector of the same dimension — canonical, nothing
// stored.  A nil receiver scales to nil.
// Complexity is O(nnz).
func (v *SparseVector[T]) Scale(alpha T) *SparseVector[T] {
	if v == nil {
		return nil
	}
	r := NewSparseVector[T](v.d)
	for i, val := range v.st {
		if p := alpha * val; p != 0 {
			r.st[i] = p
		}
	}
	return r
}

// Plus returns v + y as a NEW vector; neither operand is changed.  A
// nil operand is the zero vector, so Plus(nil) is a clone of the
// receiver and Plus on a nil receiver is a clone of y.  It panics if
// the dimensions differ.
// Complexity is O(nnz(v) + nnz(y)) on average.
func (v *SparseVector[T]) Plus(y *SparseVector[T]) *SparseVector[T] {
	if v == nil {
		return y.Clone()
	}
	if y == nil {
		return v.Clone()
	}
	if v.d != y.d {
		panic(fmt.Sprintf("sparse_vector: Plus called with a dimension-%d operand, want %d", y.d, v.d))
	}
	r := NewSparseVector[T](v.d)
	maps.Copy(r.st, v.st)
	for i, val := range y.st {
		store(r.st, i, r.st[i]+val)
	}
	return r
}

// Axpy adds α·x to v IN PLACE — the y ← y + α·x of BLAS, the row
// operation sparse Gaussian elimination applies to eliminate an entry.
// A nil x is the zero vector and the call is a no-op.  It panics on a
// nil receiver (a nil vector cannot record the update) or if the
// dimensions differ.
// Complexity is O(nnz(x)) on average.
func (v *SparseVector[T]) Axpy(alpha T, x *SparseVector[T]) {
	if v == nil {
		panic("sparse_vector: Axpy called on a nil SparseVector")
	}
	if x == nil {
		return
	}
	if v.d != x.d {
		panic(fmt.Sprintf("sparse_vector: Axpy called with a dimension-%d operand, want %d", x.d, v.d))
	}
	for i, xi := range x.st {
		store(v.st, i, v.st[i]+alpha*xi)
	}
}

// Clone returns an independent copy of v.  A nil vector clones to nil.
// Complexity is O(nnz).
func (v *SparseVector[T]) Clone() *SparseVector[T] {
	if v == nil {
		return nil
	}
	r := NewSparseVector[T](v.d)
	maps.Copy(r.st, v.st)
	return r
}

// Dense materializes v as the full []T of length Dimension().  A nil
// vector returns nil.
// Complexity is O(d).
func (v *SparseVector[T]) Dense() []T {
	if v == nil {
		return nil
	}
	y := make([]T, v.d)
	for i, val := range v.st {
		y[i] = val
	}
	return y
}

// String renders the stored entries as "(index, value)" pairs in
// ascending index order — algs4's format without its trailing space.
// A nil vector or a zero vector renders as the empty string.
// Complexity is O(nnz log nnz).
func (v *SparseVector[T]) String() string {
	if v == nil || len(v.st) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v.st))
	for _, i := range slices.Sorted(maps.Keys(v.st)) {
		parts = append(parts, fmt.Sprintf("(%d, %v)", i, v.st[i]))
	}
	return strings.Join(parts, " ")
}

// Lock is a no-op in this package; it exists for API parity with the
// thread-safe twin sparse_vector_ts, where Lock takes the write lock.
func (v *SparseVector[T]) Lock() {}

// Unlock is a no-op in this package; see Lock.
func (v *SparseVector[T]) Unlock() {}
