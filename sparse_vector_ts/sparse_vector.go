/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package sparse_vector_ts implements dimension-indexed sparse vectors
// of any numeric type, safe for concurrent use.  It is the
// thread-safe twin of github.com/pschlump/pluto/sparse_vector — the
// same API over a map of the nonzero entries, guarded by a
// sync.RWMutex — with the addition of the Lock/Unlock pair and the
// Nl-prefixed (no-lock) methods for compound operations.
//
// The stored form is canonical: Put(i, 0) removes the entry, so NNz is
// exactly the count of nonzero dimensions.  The panic and
// nil-tolerance contracts are identical to sparse_vector; the panic
// messages are prefixed sparse_vector_ts:.
//
// Concurrency model: Put and Axpy take the WRITE lock; Get, NNz, Dot,
// DotSparse, Scale, Plus, Clone, Dense, String and the read side of
// Each take the read lock.  Dimension takes no lock — the dimension is
// fixed at construction and never written afterwards.  The dimension
// in Dot/DotSparse/Plus/Axpy is checked the same way, so a mismatch
// panics without locking.  Each SNAPS the entries under the read lock
// and runs the callbacks outside it — a callback may call any method,
// including on the same vector.  The two-vector operations (DotSparse,
// Plus, Axpy) never hold two locks at once: the operand's entries are
// snapshotted under the operand's own read lock first, and the
// computation (or, for Axpy, the mutation under the receiver's write
// lock) runs on the snapshot afterwards — which also makes v.Axpy(α,
// v) legal.  Dot takes the caller's dense slice by reference: the
// caller must not mutate it concurrently.
//
// While the real Lock is held, call only the Nl-prefixed methods;
// calling a regular method will deadlock.  NlEach is the one live
// walk: it iterates under the held write lock, so its callbacks must
// NOT call methods that lock.  Axpy, DotSparse and Plus have no Nl
// forms — a second vector's lock cannot safely nest inside a held
// write lock, and their regular forms are already atomic.
//
// Run the tests with -race.
package sparse_vector_ts

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/pschlump/pluto/g_lib"
)

// SparseVector is a d-dimensional vector storing only its nonzero
// entries, keyed by dimension index in a hash map, safe for concurrent
// use.  See the package documentation for the locking model.
type SparseVector[T g_lib.Numeric] struct {
	d  int
	st map[int]T

	lock sync.RWMutex
}

// entry is one stored (index, value) pair — the unit of a snapshot.
type entry[T g_lib.Numeric] struct {
	i int
	v T
}

// NewSparseVector returns the zero vector of dimension d — indices
// 0..d-1, every entry zero, nothing stored.  It panics if d < 1.
// Complexity is O(1).
func NewSparseVector[T g_lib.Numeric](d int) *SparseVector[T] {
	if d < 1 {
		panic("sparse_vector_ts: NewSparseVector called with d < 1")
	}
	return &SparseVector[T]{d: d, st: make(map[int]T)}
}

// store keeps the canonical form: a zero value is deleted from the
// map, never stored.  The caller must hold the write lock (or the
// vector must be unshared).
func store[T g_lib.Numeric](st map[int]T, i int, value T) {
	if value == 0 {
		delete(st, i)
	} else {
		st[i] = value
	}
}

// nlSnapshot collects the stored entries without locking; the caller
// must hold at least the read lock.
func (v *SparseVector[T]) nlSnapshot() []entry[T] {
	es := make([]entry[T], 0, len(v.st))
	for i, val := range v.st {
		es = append(es, entry[T]{i, val})
	}
	return es
}

// snapshot collects the stored entries under the read lock.  No other
// lock is ever held here, so snapshots of two different vectors can be
// taken one after the other without deadlock.
func (v *SparseVector[T]) snapshot() []entry[T] {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.nlSnapshot()
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
		panic("sparse_vector_ts: Put called on a nil SparseVector")
	}
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.NlPut(i, value)
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
	v.lock.RLock()
	defer v.lock.RUnlock()
	rv, ok = v.st[i]
	return
}

// Dimension returns d, the number of dimensions (0..d-1).  A nil
// vector or the zero value reports 0.  Dimension takes no lock: d is
// fixed by the constructor and never written afterwards.
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
	v.lock.RLock()
	defer v.lock.RUnlock()
	return len(v.st)
}

// Each visits the stored entries in ascending dimension order; fx
// returning true stops the walk.  The entries are SNAPPED under the
// read lock and the callbacks run outside it, so a callback may call
// any method — including Put on this same vector, whose effect the
// walk will not see.  A nil vector visits nothing.
// Complexity is O(nnz log nnz).
func (v *SparseVector[T]) Each(fx func(i int, value T) bool) {
	if v == nil {
		return
	}
	es := v.snapshot()
	slices.SortFunc(es, func(a, b entry[T]) int { return a.i - b.i })
	for _, e := range es {
		if fx(e.i, e.v) {
			return
		}
	}
}

// Dot returns the inner product of v with the dense vector y, which
// the caller must not mutate concurrently.  It panics if len(y) is not
// Dimension() — the product is not defined on mismatched dimensions
// (the check reads only the immutable dimension, so it needs no lock).
// A nil v is the zero vector, so Dot of an empty dense operand on a
// nil receiver is 0.
// Complexity is O(nnz) — proportional to the stored entries, not d.
func (v *SparseVector[T]) Dot(y []T) T {
	var sum T
	if v == nil {
		if len(y) != 0 {
			panic(fmt.Sprintf("sparse_vector_ts: Dot called with a dense operand of length %d, want dimension %d", len(y), 0))
		}
		return sum
	}
	if len(y) != v.d {
		panic(fmt.Sprintf("sparse_vector_ts: Dot called with a dense operand of length %d, want dimension %d", len(y), v.d))
	}
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.NlDot(y)
}

// DotSparse returns the inner product of two sparse vectors, iterating
// the sparser snapshot and probing a map built from the denser one.  A
// nil operand is the zero vector: the product is 0.  It panics if the
// dimensions differ.  Both operands are snapshotted (an O(nnz) copy
// each), one lock at a time, so no lock is ever held across two
// vectors — v.DotSparse(v) is legal.
// Complexity is O(nnz(v) + nnz(y)) — the snapshot cost dominates the
// min-side iteration of the single-threaded twin.
func (v *SparseVector[T]) DotSparse(y *SparseVector[T]) T {
	var sum T
	if v == nil || y == nil {
		return sum
	}
	if v.d != y.d {
		panic(fmt.Sprintf("sparse_vector_ts: DotSparse called with a dimension-%d operand, want %d", y.d, v.d))
	}
	a, b := v.snapshot(), y.snapshot()
	small, big := a, b
	if len(small) > len(big) {
		small, big = big, small
	}
	m := make(map[int]T, len(big))
	for _, e := range big {
		m[e.i] = e.v
	}
	for _, e := range small {
		if bv, ok := m[e.i]; ok {
			sum += e.v * bv
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
	v.lock.RLock()
	defer v.lock.RUnlock()
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
// the dimensions differ.  Both operands are snapshotted, one lock at a
// time.
// Complexity is O(nnz(v) + nnz(y)) on average.
func (v *SparseVector[T]) Plus(y *SparseVector[T]) *SparseVector[T] {
	if v == nil {
		return y.Clone()
	}
	if y == nil {
		return v.Clone()
	}
	if v.d != y.d {
		panic(fmt.Sprintf("sparse_vector_ts: Plus called with a dimension-%d operand, want %d", y.d, v.d))
	}
	a, b := v.snapshot(), y.snapshot()
	r := NewSparseVector[T](v.d)
	for _, e := range a {
		r.st[e.i] = e.v
	}
	for _, e := range b {
		store(r.st, e.i, r.st[e.i]+e.v)
	}
	return r
}

// Axpy adds α·x to v IN PLACE — the y ← y + α·x of BLAS, the row
// operation sparse Gaussian elimination applies to eliminate an entry.
// A nil x is the zero vector and the call is a no-op.  It panics on a
// nil receiver (a nil vector cannot record the update) or if the
// dimensions differ.  The operand is snapshotted under its own read
// lock BEFORE the receiver's write lock is taken, so no two locks are
// ever held at once and v.Axpy(α, v) is legal.
// Complexity is O(nnz(x)) on average, plus the O(nnz(x)) snapshot.
func (v *SparseVector[T]) Axpy(alpha T, x *SparseVector[T]) {
	if v == nil {
		panic("sparse_vector_ts: Axpy called on a nil SparseVector")
	}
	if x == nil {
		return
	}
	if v.d != x.d {
		panic(fmt.Sprintf("sparse_vector_ts: Axpy called with a dimension-%d operand, want %d", x.d, v.d))
	}
	xs := x.snapshot()
	v.lock.Lock()
	defer v.lock.Unlock()
	for _, e := range xs {
		store(v.st, e.i, v.st[e.i]+alpha*e.v)
	}
}

// Clone returns an independent copy of v.  A nil vector clones to nil.
// Complexity is O(nnz).
func (v *SparseVector[T]) Clone() *SparseVector[T] {
	if v == nil {
		return nil
	}
	v.lock.RLock()
	defer v.lock.RUnlock()
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
	v.lock.RLock()
	defer v.lock.RUnlock()
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
	if v == nil {
		return ""
	}
	v.lock.RLock()
	es := v.nlSnapshot()
	v.lock.RUnlock()
	if len(es) == 0 {
		return ""
	}
	slices.SortFunc(es, func(a, b entry[T]) int { return a.i - b.i })
	parts := make([]string, 0, len(es))
	for _, e := range es {
		parts = append(parts, fmt.Sprintf("(%d, %v)", e.i, e.v))
	}
	return strings.Join(parts, " ")
}

// -------------------------------------------------------------------------------------------------------
// Exposed write lock and the Nl (no-lock) methods for compound
// operations.  While the lock is held only call the Nl-prefixed
// methods; calling a regular method will deadlock.
// -------------------------------------------------------------------------------------------------------

// Lock takes the vector's write lock, allowing a group of operations
// to be performed atomically.  While the lock is held only call the
// Nl-prefixed (no-lock) methods; calling a regular method will
// deadlock.  Pair every Lock with a corresponding Unlock.  Locking a
// nil vector is a no-op.
func (v *SparseVector[T]) Lock() {
	if v == nil {
		return
	}
	v.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil
// vector is a no-op.
func (v *SparseVector[T]) Unlock() {
	if v == nil {
		return
	}
	v.lock.Unlock()
}

// NlDimension is Dimension without locking; it reads only the
// immutable dimension, so it is safe under Lock or anywhere else.
func (v *SparseVector[T]) NlDimension() int {
	if v == nil {
		return 0
	}
	return v.d
}

// NlNNz is NNz without locking; call it only while holding Lock.
func (v *SparseVector[T]) NlNNz() int {
	if v == nil {
		return 0
	}
	return len(v.st)
}

// NlGet is Get without locking; call it only while holding Lock.  It
// reports (0, false) for an out-of-range index or a missing entry.
func (v *SparseVector[T]) NlGet(i int) (rv T, ok bool) {
	if v == nil || i < 0 || i >= v.d {
		return rv, false
	}
	rv, ok = v.st[i]
	return
}

// NlPut is Put without locking; call it only while holding Lock.  It
// reports false for an out-of-range index; assigning zero removes the
// entry.
func (v *SparseVector[T]) NlPut(i int, value T) bool {
	if i < 0 || i >= v.d {
		return false
	}
	store(v.st, i, value)
	return true
}

// NlEach is Each without locking; call it only while holding Lock.
// Unlike the public Each this is a LIVE walk under the held lock, so
// its callbacks must not call methods that lock (any regular method)
// — deadlock.  A nil vector visits nothing.
func (v *SparseVector[T]) NlEach(fx func(i int, value T) bool) {
	if v == nil {
		return
	}
	for _, i := range slices.Sorted(maps.Keys(v.st)) {
		if fx(i, v.st[i]) {
			return
		}
	}
}

// NlDot is Dot without locking; call it only while holding Lock.  The
// dense operand is the caller's slice, referenced without copying, and
// a length mismatch panics exactly as in Dot.
func (v *SparseVector[T]) NlDot(y []T) T {
	var sum T
	if v == nil {
		return sum
	}
	if len(y) != v.d {
		panic(fmt.Sprintf("sparse_vector_ts: NlDot called with a dense operand of length %d, want dimension %d", len(y), v.d))
	}
	for i, val := range v.st {
		sum += val * y[i]
	}
	return sum
}
