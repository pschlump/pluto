/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_set_ts is the thread-safe twin of hash_set: a
// HashSet[T] with the identical public surface, guarding a plain
// hash_set.HashSet value with a single sync.RWMutex (the kd_tree_ts
// wrapper-twin pattern — no locks inside the borrowed structure, one
// lock around it).  See hash_set for the set semantics themselves: the
// element type implements no interface, values are stored and returned
// by value, and equality and hashing go through the one function pair
// the constructors store.
//
// Concurrency model:
//
//   - Every method self-locks.  Reads take the read lock, single-step
//     writes take the write lock, and multi-step operations (the
//     batches, the algebra, JSON unmarshal) hold the write lock for the
//     whole compound, so concurrent readers observe them atomically.
//   - Lock and Unlock expose the write lock for caller-built compounds;
//     both are no-ops on a nil set.  Calling a regular (locking) method
//     while the lock is held deadlocks — use the Nl-prefixed forms
//     inside a compound: NlInsert, NlDelete (write lock held), NlSearch,
//     NlLen, NlIsEmpty (read or write lock held).  There are no other
//     Nl methods; every other surface is self-locking.
//   - Snapshot methods (Slice, String, StringFunc, Equal, Clone, All,
//     the relations, the algebra operands) copy what they need under
//     the read lock and then work on the copy with no lock held — so
//     StringFunc's format callback may safely call back into the set.
//     Walk is the one exception: it holds the read lock for the whole
//     walk, so its callback must NOT call back into the set (documented
//     deadlock hazard, the hash_grow_ts Walk contract).
//   - Two-set operations (Equal, the relations, the algebra) never hold
//     two locks at once: each set is touched under its own lock and
//     released before the other is touched (snapshots first, compute
//     unlocked, swap the receiver's contents under one write lock at the
//     end).  Two goroutines may therefore compare or combine the same
//     pair of sets in opposite directions without deadlock.
//
// A nil *HashSet behaves as an empty set for every read (the kd_tree_ts
// nil-guard-before-lock-acquisition pattern), and the insert contract
// matches hash_set: the storing operations panic on a nil set, and on a
// zero-value set with no equality/hash functions — with hash_set_ts
// spelled messages naming this package's constructors.  The constructor
// nil-function panics match too.
package hash_set_ts

import (
	"fmt"
	"hash/maphash"
	"iter"
	"sync"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/hash_set"
)

// HashSet is a thread-safe set of T values with no duplicates: the
// plain hash_set.HashSet the package wraps, plus one sync.RWMutex.  Use
// NewHashSet for element types that support ==, or NewHashSetFunc for a
// caller supplied equality and hash function.  The zero value is an
// empty read-only set.
type HashSet[T any] struct {
	inner hash_set.HashSet[T]
	lock  sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// NewHashSet creates a new, empty set for any element type comparable
// with ==.  Elements are compared with the == operator and hashed with
// the stdlib hash/maphash using a per-set random seed — no method has
// to be implemented on T, and no element is ever boxed into an
// interface.
// Complexity is O(1) for the set; O(capacity) for the bucket allocation.
func NewHashSet[T comparable]() *HashSet[T] {
	var seed = maphash.MakeSeed()
	return newHashSet(
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
	)
}

// NewHashSetFunc creates a new, empty set that identifies elements with
// the caller supplied equality function and hashes them with the caller
// supplied hash function.  The two functions must agree: whenever
// eq(a, b) is true, hash(a) and hash(b) must be equal.  It panics if eq
// or hash is nil.
// Complexity is O(1) for the set; O(capacity) for the bucket allocation.
func NewHashSetFunc[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashSet[T] {
	if eq == nil {
		panic("hash_set_ts: NewHashSetFunc called with a nil equality function")
	}
	if hash == nil {
		panic("hash_set_ts: NewHashSetFunc called with a nil hash function")
	}
	return newHashSet(eq, hash)
}

// FromSlice creates a new set initialized with the elements of items.
// Equal elements collapse the way repeated Insert calls do; a nil or
// empty slice yields an empty set.
// Complexity is O(k) average, k = len(items).
func FromSlice[T comparable](items []T) *HashSet[T] {
	ss := NewHashSet[T]()
	ss.InsertSlice(items)
	return ss
}

// FromSliceFunc is FromSlice for a caller supplied equality and hash
// function (which must agree — see NewHashSetFunc).  It panics if eq or
// hash is nil.
// Complexity is O(k) average, k = len(items).
func FromSliceFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, items []T) *HashSet[T] {
	if eq == nil {
		panic("hash_set_ts: FromSliceFunc called with a nil equality function")
	}
	if hash == nil {
		panic("hash_set_ts: FromSliceFunc called with a nil hash function")
	}
	ss := newHashSet(eq, hash)
	ss.InsertSlice(items)
	return ss
}

// newHashSet is the shared constructor body; the public constructors
// have already validated the pair.
func newHashSet[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashSet[T] {
	return &HashSet[T]{
		inner: *hash_set.NewHashSetFunc(eq, hash),
	}
}

// Lock acquires the set's write lock, for caller-built read-modify-write
// compounds built from the Nl-prefixed methods.  Unlock releases it.
// Both are no-ops on a nil set.  Calling a regular (locking) method of
// this set while the lock is held deadlocks.
func (t *HashSet[T]) Lock() {
	if t == nil {
		return
	}
	t.lock.Lock()
}

// Unlock releases the write lock acquired by Lock.  It is a no-op on a
// nil set.
func (t *HashSet[T]) Unlock() {
	if t == nil {
		return
	}
	t.lock.Unlock()
}

// pairOf reads the inner set's equality/hash pair under t's read lock.
// The pair is written only by the constructors (and swapped whole by
// the algebra under the write lock), so a locked read is race-free.  A
// nil set has no pair.
func (t *HashSet[T]) pairOf() (eq func(a, b T) bool, hash func(a T) uint64) {
	if t == nil {
		return nil, nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Eq(), t.inner.Hash()
}

// insertContract applies the insert panic contract for the storing
// operations, called with the write lock held: a nil set cannot store
// an element, and a zero-value set has no equality/hash functions.
func (t *HashSet[T]) insertContract(caller string) {
	if t.inner.Eq() == nil || t.inner.Hash() == nil {
		panic(fmt.Sprintf("hash_set_ts: %s called on a zero-value set (create the set with NewHashSet or NewHashSetFunc)", caller))
	}
}

// -------------------------------------------------------------------------------------------------------
// Core.

// Insert adds item to the set.  If an equal element is already present
// the new item replaces it and false is returned; true is returned when
// a new element was added.
//
// Insert panics on a nil set or on a zero-value set (no equality/hash
// functions) — the hash_set insert contract with hash_set_ts spelled
// messages.  Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) Insert(item T) bool {
	if t == nil {
		panic("hash_set_ts: Insert called on a nil set (a nil set cannot store an element)")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.insertContract("Insert")
	return t.inner.Insert(item)
}

// Delete removes the element equal to find, if present, and reports
// whether it was found.  Deleting from a nil or zero-value set returns
// false.
// Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) Delete(find T) bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.Delete(find)
}

// Search returns the stored element equal to find, or the zero value of
// T and false when it is not present.
// Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) Search(find T) (rv T, found bool) {
	if t == nil {
		return
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Search(find)
}

// Contains reports whether the set holds an element equal to find.
// Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) Contains(find T) bool {
	_, found := t.Search(find)
	return found
}

// Len returns the number of elements in the set.
// Complexity is O(1).
func (t *HashSet[T]) Len() int {
	if t == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Len()
}

// IsEmpty returns true if the set has no elements.
// Complexity is O(1).
func (t *HashSet[T]) IsEmpty() bool {
	return t.Len() == 0
}

// Truncate removes all elements from the set.  The capacity and the
// equality/hash functions are kept, so the set remains usable.
// Complexity is O(n).
func (t *HashSet[T]) Truncate() {
	if t == nil {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.inner.Truncate()
}

// Eq returns the set's stored equality function, or nil for a nil or
// zero-value set.
// Complexity is O(1).
func (t *HashSet[T]) Eq() func(a, b T) bool {
	if t == nil {
		return nil
	}
	eq, _ := t.pairOf()
	return eq
}

// Hash returns the set's stored hash function; see Eq.
// Complexity is O(1).
func (t *HashSet[T]) Hash() func(a T) uint64 {
	if t == nil {
		return nil
	}
	_, hash := t.pairOf()
	return hash
}

// NlInsert is Insert for callers holding the write lock (Lock).  It
// applies the same insert contract as Insert; the contract panic is
// raised by the wrapped plain set, so its message spells hash_set (the
// package that owns the contract) — the fix it names, the constructors,
// is the same pair this package exposes.
// Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) NlInsert(item T) bool {
	return t.inner.Insert(item)
}

// NlDelete is Delete for callers holding the write lock (Lock).
// Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) NlDelete(find T) bool {
	return t.inner.Delete(find)
}

// NlSearch is Search for callers holding either lock: the read lock for
// read-only compounds, the write lock inside read-modify-write
// compounds.
// Complexity is O(1) average, O(n) worst case.
func (t *HashSet[T]) NlSearch(find T) (rv T, found bool) {
	return t.inner.Search(find)
}

// NlLen is Len for callers holding either lock.
// Complexity is O(1).
func (t *HashSet[T]) NlLen() int {
	return t.inner.Len()
}

// NlIsEmpty is IsEmpty for callers holding either lock.
// Complexity is O(1).
func (t *HashSet[T]) NlIsEmpty() bool {
	return t.inner.Len() == 0
}

// -------------------------------------------------------------------------------------------------------
// Batch operations — one write lock for the whole compound.

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new.  The whole batch runs under one write
// lock, so concurrent readers observe it atomically (all inserted or
// none).  An empty batch stores nothing, returns false and never fires
// the insert contract.  A batch that would store an element into a nil
// or zero-value set panics with the standard insert-family message.
// Complexity is O(k) average, k = len(items).
func (t *HashSet[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	if t == nil {
		panic("hash_set_ts: InsertSlice called on a nil set (a nil set cannot store an element)")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.insertContract("InsertSlice")
	return t.inner.InsertSlice(items)
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator.
// The iterator is drained BEFORE the write lock is taken, so feeding a
// set its own All() is safe and the caller's iterator runs without any
// lock held (it may call back into either set).  A nil iterator inserts
// nothing and returns false.
// Complexity is O(k) average, k = number of yielded elements.
func (t *HashSet[T]) InsertIter(seq iter.Seq[T]) bool {
	if seq == nil {
		return false
	}
	items := make([]T, 0)
	for item := range seq {
		items = append(items, item)
	}
	if len(items) == 0 {
		return false
	}
	if t == nil {
		panic("hash_set_ts: InsertIter called on a nil set (a nil set cannot store an element)")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.insertContract("InsertIter")
	return t.inner.InsertSlice(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present.  The whole batch runs under one write lock.
// Complexity is O(k) average, k = len(items).
func (t *HashSet[T]) DeleteSlice(items []T) bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.DeleteSlice(items)
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The elements are snapshotted under the
// read lock, fx runs on the snapshot with NO lock held (it may safely
// call back into the set), and the doomed elements are then deleted
// under one write lock — the collect-then-delete the underlying table
// mandates under its backward-shift deletion, wrapped with the
// hash_grow_ts lock discipline.
// Complexity is O(n) average.
func (t *HashSet[T]) DeleteFunc(fx func(T) bool) bool {
	if t == nil || fx == nil {
		return false
	}
	t.lock.RLock()
	items := t.inner.Slice() // snapshot: the read lock is released before fx runs
	t.lock.RUnlock()
	doomed := make([]T, 0)
	for _, v := range items {
		if fx(v) {
			doomed = append(doomed, v)
		}
	}
	if len(doomed) == 0 {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.DeleteSlice(doomed)
}

// ContainsSlice reports whether every element of items is present in the
// set (an empty items slice is vacuously contained).  The whole scan
// runs under one read lock, so the answer is a consistent snapshot.
// Complexity is O(k) average, k = len(items).
func (t *HashSet[T]) ContainsSlice(items []T) bool {
	if t == nil {
		return len(items) == 0 // vacuous containment, nothing to probe
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	for _, item := range items {
		if !t.inner.Contains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one element of items is present
// in the set, under one read lock.
// Complexity is O(k) average, k = len(items).
func (t *HashSet[T]) ContainsAny(items []T) bool {
	if t == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	for _, item := range items {
		if t.inner.Contains(item) {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------------------------------------------------
// Set-convenience capabilities — snapshot, then compute with no lock held.

// Slice returns a fresh slice holding every element of the set, copied
// under the read lock; the UNSPECIFIED slot order caveat of the plain
// package applies.  A nil set yields nil; an empty set yields an empty
// slice.
// Complexity is O(n).
func (t *HashSet[T]) Slice() []T {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Slice()
}

// String renders the set as its elements formatted with fmt.Sprint, the
// formatted strings sorted ascending inside square brackets: "[a b c]".
// Deterministic across sets and runs, exactly as the plain package's
// String.  A nil or empty set renders as "[]".
// Complexity is O(n log n).
func (t *HashSet[T]) String() string {
	return t.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function.  The elements are snapshotted under the read lock and the
// format function runs with NO lock held, so it may safely call back
// into the set.  A nil format function renders as "[]".
// Complexity is O(n log n).
func (t *HashSet[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	return g_lib.FormatSorted(t.Slice(), format)
}

// Equal reports whether t and other hold the same set of elements: same
// length and every element of t found in other.  The receiver is
// snapshotted under its own read lock and other is then touched one
// Contains call at a time — never two locks at once, so Equal never
// deadlocks, not even when two goroutines compare the same pair in
// opposite directions.  Membership is compared, never iteration order.
//
// Precondition: both sets must use consistent equality/hash functions
// (a nil set trivially does).
// Complexity is O(n) average.
func (t *HashSet[T]) Equal(other *HashSet[T]) bool {
	if t.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if t.Len() == 0 {
		return true // both empty (either may be nil)
	}
	items := t.Slice() // snapshot taken, t's lock released
	for _, v := range items {
		if !other.Contains(v) { // one lock at a time, on other
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the set: a fresh thread-safe set with the
// same equality/hash functions and fresh backing storage.  The inner
// set is cloned under the read lock.  A nil set clones to nil; a
// zero-value set clones to a fresh zero-value set.
// Complexity is O(n).
func (t *HashSet[T]) Clone() *HashSet[T] {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	dup := t.inner.Clone() // deep copy of the plain set
	t.lock.RUnlock()
	return &HashSet[T]{inner: *dup}
}

// -------------------------------------------------------------------------------------------------------
// The set algebra — snapshot each operand under its own lock, compute on
// local sets with no lock held, swap the receiver under one write lock.

// adoptPair picks the equality/hash pair an algebra rebuild uses: the
// receiver's own pair when it has one, otherwise yy's, otherwise zz's.
// Each read takes that one set's read lock and releases it before the
// next — never two locks at once.
func (t *HashSet[T]) adoptPair(yy, zz *HashSet[T]) (eq func(a, b T) bool, hash func(a T) uint64) {
	eq, hash = t.pairOf()
	if eq != nil && hash != nil {
		return eq, hash
	}
	if eq, hash = yy.pairOf(); eq != nil && hash != nil {
		return eq, hash
	}
	return zz.pairOf()
}

// swapIn installs fresh as the receiver's contents under one write lock
// — the atomic commit step every algebra method ends with.
func (t *HashSet[T]) swapIn(fresh hash_set.HashSet[T]) {
	t.lock.Lock()
	t.inner = fresh
	t.lock.Unlock()
}

// Union is a set union, t = yy ∪ zz: the receiver's contents are
// replaced with every element of yy and zz.  The operands are
// snapshotted each under its own read lock (released before anything
// else is touched — never two locks at once), the result is built on a
// local set with no lock held, and the receiver is swapped under one
// write lock, so concurrent readers observe the whole result at once
// and either operand may alias the receiver (t.Union(t, t) is legal).
// A zero-value t adopts yy's equality/hash pair (then zz's); a nil t is
// a no-op, and nil operands contribute nothing.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (t *HashSet[T]) Union(yy, zz *HashSet[T]) {
	if t == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias t
	eq, hash := t.adoptPair(yy, zz)
	if eq == nil || hash == nil {
		t.swapIn(hash_set.HashSet[T]{}) // nothing to adopt: empty result
		return
	}
	fresh := *hash_set.FromSliceFunc(eq, hash, a)
	fresh.InsertSlice(b)
	t.swapIn(fresh)
}

// Intersect is a set intersection, t = yy ∩ zz.  Same lock discipline
// as Union: operand snapshots under their own locks, local rebuild with
// no lock held, one write-lock swap.  Either operand may alias t; a
// zero-value t adopts yy's pair (then zz's); a nil t is a no-op.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (t *HashSet[T]) Intersect(yy, zz *HashSet[T]) {
	if t == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias t
	eq, hash := t.adoptPair(yy, zz)
	if eq == nil || hash == nil {
		t.swapIn(hash_set.HashSet[T]{})
		return
	}
	inZZ := hash_set.FromSliceFunc(eq, hash, b)
	keep := make([]T, 0, len(a))
	for _, v := range a {
		if inZZ.Contains(v) {
			keep = append(keep, v)
		}
	}
	t.swapIn(*hash_set.FromSliceFunc(eq, hash, keep))
}

// Minus is a set minus, t = yy − zz: the elements of yy not in zz.
// Same lock discipline as Union.  Either operand may alias t
// (t.Minus(t, t) empties t); a zero-value t adopts yy's pair (then
// zz's); a nil t is a no-op.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (t *HashSet[T]) Minus(yy, zz *HashSet[T]) {
	if t == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias t
	eq, hash := t.adoptPair(yy, zz)
	if eq == nil || hash == nil {
		t.swapIn(hash_set.HashSet[T]{})
		return
	}
	inZZ := hash_set.FromSliceFunc(eq, hash, b)
	keep := make([]T, 0, len(a))
	for _, v := range a {
		if !inZZ.Contains(v) {
			keep = append(keep, v)
		}
	}
	t.swapIn(*hash_set.FromSliceFunc(eq, hash, keep))
}

// SymmetricDifference is a set symmetric difference, t = yy △ zz — the
// elements in exactly one of yy and zz.  Same lock discipline as Union.
// Either operand may alias t; a zero-value t adopts yy's pair (then
// zz's); a nil t is a no-op.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (t *HashSet[T]) SymmetricDifference(yy, zz *HashSet[T]) {
	if t == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias t
	eq, hash := t.adoptPair(yy, zz)
	if eq == nil || hash == nil {
		t.swapIn(hash_set.HashSet[T]{})
		return
	}
	inYY := hash_set.FromSliceFunc(eq, hash, a)
	inZZ := hash_set.FromSliceFunc(eq, hash, b)
	keep := make([]T, 0, len(a)+len(b))
	for _, v := range a {
		if !inZZ.Contains(v) {
			keep = append(keep, v)
		}
	}
	for _, v := range b {
		if !inYY.Contains(v) {
			keep = append(keep, v)
		}
	}
	t.swapIn(*hash_set.FromSliceFunc(eq, hash, keep))
}

// -------------------------------------------------------------------------------------------------------
// The subset/disjoint relations — both sides snapshotted, never two
// locks at once, computed with no lock held.

// relationParts snapshots both sides of a relation: the receiver's and
// other's elements, each under its own read lock, released before the
// computation starts.
func (t *HashSet[T]) relationParts(other *HashSet[T]) (a, b []T) {
	return t.Slice(), other.Slice()
}

// IsSubsetOf reports whether every element of t is present in other.
// The empty set is a subset of everything (including a nil set).
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(n) expected, n = |t|.
func (t *HashSet[T]) IsSubsetOf(other *HashSet[T]) bool {
	if t == nil {
		return true
	}
	a, b := t.relationParts(other)
	if len(a) > len(b) {
		return false // a longer set cannot be a subset
	}
	if len(a) == 0 {
		return true // the empty set is a subset of everything
	}
	eq, hash := t.pairOf() // a is non-empty: the pair exists
	probe := hash_set.FromSliceFunc(eq, hash, b)
	for _, v := range a {
		if !probe.Contains(v) {
			return false
		}
	}
	return true
}

// IsProperSubsetOf reports whether t is a subset of other AND strictly
// shorter.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(n) expected, n = |t|.
func (t *HashSet[T]) IsProperSubsetOf(other *HashSet[T]) bool {
	return t.Len() < other.Len() && t.IsSubsetOf(other)
}

// IsSupersetOf reports whether every element of other is present in t —
// other.IsSubsetOf(t).
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(m) expected, m = |other|.
func (t *HashSet[T]) IsSupersetOf(other *HashSet[T]) bool {
	return other.IsSubsetOf(t)
}

// IsProperSupersetOf reports whether t is a superset of other AND
// strictly longer.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(m) expected, m = |other|.
func (t *HashSet[T]) IsProperSupersetOf(other *HashSet[T]) bool {
	return other.IsProperSubsetOf(t)
}

// IsDisjointWith reports whether t and other share no element.  Two
// empty sets are disjoint (vacuously), as is any pair where either side
// is empty or nil.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(n) expected, n = |t|.
func (t *HashSet[T]) IsDisjointWith(other *HashSet[T]) bool {
	if t == nil || t.IsEmpty() {
		return true
	}
	a, b := t.relationParts(other)
	if len(b) == 0 {
		return true
	}
	eq, hash := t.pairOf() // a is non-empty: the pair exists
	probe := hash_set.FromSliceFunc(eq, hash, b)
	for _, v := range a {
		if probe.Contains(v) {
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------------------------------------
// Iteration.

// ApplyFunction is the callback type for Walk, an alias of the plain
// package's (the bucket position and the element stored there;
// returning false stops the walk).
type ApplyFunction[T any] = hash_set.ApplyFunction[T]

// All returns an iterator (Go 1.23 range-over-func) over the elements
// of the set.  The elements are SNAPSHOTTED under the read lock and
// yielded after the lock is released — the wrapper-twin snapshot
// iterator, safe against concurrent mutation, at the cost of the copy.
// The order is unspecified (per-set hash seed).
//
//	for item := range t.All() { ... }
//
// Complexity is O(n) for the snapshot, O(1) per yield.
func (t *HashSet[T]) All() iter.Seq[T] {
	if t == nil {
		return func(func(T) bool) {}
	}
	items := t.Slice() // snapshot taken, lock released
	return func(yield func(T) bool) {
		for _, v := range items {
			if !yield(v) {
				return
			}
		}
	}
}

// Walk calls fx for each element of the set until all elements have
// been visited or fx returns false; it returns true when the walk ran
// to completion.  The READ LOCK IS HELD FOR THE WHOLE WALK: fx must not
// call back into this set (a locking method would deadlock on the held
// read lock versus a waiting writer) — snapshot with Slice or All and
// walk the copy when the callback needs the set.  The order is
// unspecified (per-set hash seed).
// Complexity is O(n).
func (t *HashSet[T]) Walk(fx ApplyFunction[T]) (b bool) {
	if t == nil {
		return true
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Walk(fx)
}
