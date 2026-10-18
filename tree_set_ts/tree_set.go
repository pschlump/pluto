/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package tree_set_ts is the thread-safe twin of tree_set: the same
// TreeSet surface — a sorted set of unique elements over an avl_tree,
// the algs4 SET analog with Go generics and no interfaces — behind a
// sync.RWMutex.  It is the kd_tree_ts wrapper-twin pattern: the twin
// holds a plain tree_set.TreeSet by value plus the one mutex, every
// public method is a nil-guard, a lock and a call straight into the
// plain package, and the Nl* variants are the same calls for callers
// that already hold the lock.
//
// The public API, the panic contract and every observable behavior are
// tree_set's; read that package's documentation for the operation
// semantics.  What this package adds is the locking:
//
//	Reads (Search, Contains, Floor, Ceil, FindMin, FindMax, Index,
//	Len, IsEmpty, Slice, ContainsSlice, ContainsAny, Equal, the
//	relations, the JSON marshal) take the read lock for the whole
//	call.
//
//	Writes (Insert, InsertSlice, InsertIter, Delete, DeleteSlice,
//	DeleteFunc, DeleteAtHead, DeleteAtTail, Truncate, the set
//	algebra, UnmarshalJSON) take the write lock once for the whole
//	operation — a batch is atomic: concurrent readers see it all done
//	or not at all.
//
//	Iterators (All, Backward, Range) take a snapshot under the read
//	lock and yield the copy, so the set may be freely mutated — even
//	from inside the loop body.  The walks (WalkInOrder and friends)
//
// instead hold the read lock for the whole traversal of the live set:
// a walk callback must not call back into the set, or it self-deadlocks
// (use Slice or All when the callback must touch the set).
//
// Callback discipline: StringFunc's format function and DeleteFunc's
// predicate run with NO lock held, on a snapshot — both may safely call
// back into the set.  InsertIter drains the iterator (and takes any
// tree snapshot it may come from) BEFORE the write lock is acquired, so
// feeding a set its own All() is safe.
//
//	Cross-set operations never hold two locks at once.  Equal and the
//	relations snapshot the receiver under its own read lock and then
//	touch the other set one self-locking probe at a time; the set
//	algebra snapshots each operand under that operand's own read lock
//	(released), rebuilds the result locally and installs it under the
//	receiver's write lock.  Two goroutines may therefore compare or
//	combine the same pair of sets in opposite directions without
//	deadlock.  The price is inherent: an algebra operation is atomic
//	per set, not across sets — the operand snapshot is not a fence
//	against concurrent mutation of the operand between its snapshot
//	and the install.
//
//	Compound Nl* surface: NlInsert, NlDelete, NlSearch, NlContains,
//	NlLen and NlIsEmpty operate without taking the lock, for the
//	race-free-compound pattern — hold Lock (or RLock for the read
//	side) across a multi-step sequence and call the Nl* forms inside.
//	The convenience, batch, algebra, relation and iterator methods are
//	self-locking and have no Nl form.
//
// As with tree_set, a nil *TreeSet and the zero value behave as an
// empty set for every read, and the package panics in exactly the same
// two situations, with twin-named messages: a nil comparison function
// at construction (NewTreeSetFunc/FromSliceFunc), and Insert — or any
// operation that would store an element, the batch inserters and
// UnmarshalJSON included — on a nil set or a zero-value set.  The
// messages name this package's constructors.
package tree_set_ts

import (
	"cmp"
	"fmt"
	"iter"
	"strings"
	"sync"

	"github.com/pschlump/pluto/tree_set"
)

// TreeSet is a thread-safe sorted set of unique elements: the
// tree_set.TreeSet surface behind a sync.RWMutex.  Use NewTreeSet for
// naturally ordered element types or NewTreeSetFunc for a caller
// supplied comparison function.  The zero value is an empty set for
// reads; Insert on it panics — create sets with the constructors.
type TreeSet[T any] struct {
	inner tree_set.TreeSet[T]
	lock  sync.RWMutex

	// cmp is the twin's own copy of the comparison function, the same
	// one inner was built with (tree_set does not export its
	// comparator).  It is written only by the constructors and by the
	// set algebra's adoption step, both under the write lock, and read
	// under a lock everywhere — including by another set's algebra
	// snapshot, which reads it under this set's read lock.  Invariant:
	// cmp != nil exactly when the twin was built by a constructor (or
	// adopted an ordering), which is exactly when inner can store.
	cmp func(a, b T) int
}

// ApplyFunction is the callback type of the Walk* functions, an alias of
// tree_set's (and avl_tree's) so a callback written for any of the three
// packages works with all of them.
type ApplyFunction[T any] = tree_set.ApplyFunction[T]

// -------------------------------------------------------------------------------------------------------

// NewTreeSet creates a new, empty, thread-safe set for any naturally
// ordered element type (all integers, floats and strings —
// cmp.Ordered), ordered by the built-in < and > operators of T — the
// same comparator the plain package's NewTreeSet stores.
// Complexity is O(1).
func NewTreeSet[T cmp.Ordered]() *TreeSet[T] {
	return newTreeSet[T](cmp.Compare[T])
}

// NewTreeSetFunc creates a new, empty, thread-safe set that orders
// elements with the caller supplied comparison function fx — for
// example a struct ordered by one of its fields.  fx must order
// elements consistently.  It panics if fx is nil.
// Complexity is O(1).
func NewTreeSetFunc[T any](fx func(a, b T) int) *TreeSet[T] {
	if fx == nil {
		panic("tree_set_ts: NewTreeSetFunc called with a nil comparison function")
	}
	return newTreeSet[T](fx)
}

// FromSlice creates a new, thread-safe set holding the elements of
// items, in the natural ordering of T; duplicates collapse exactly as
// inserting them in order would.
// Complexity is O(k·log₂ k), k = len(items).
func FromSlice[T cmp.Ordered](items []T) *TreeSet[T] {
	set := NewTreeSet[T]()
	set.InsertSlice(items)
	return set
}

// FromSliceFunc is FromSlice with a caller supplied comparison function
// (the NewTreeSetFunc form).  It panics if fx is nil.
// Complexity is O(k·log₂ k), k = len(items).
func FromSliceFunc[T any](fx func(a, b T) int, items []T) *TreeSet[T] {
	if fx == nil {
		panic("tree_set_ts: FromSliceFunc called with a nil comparison function")
	}
	set := NewTreeSetFunc[T](fx)
	set.InsertSlice(items)
	return set
}

func newTreeSet[T any](cmp func(a, b T) int) *TreeSet[T] {
	inner := tree_set.NewTreeSetFunc[T](cmp)
	return &TreeSet[T]{inner: *inner, cmp: cmp}
}

// Lock takes the set's exclusive lock, for multi-step write sequences
// run through the Nl* methods.  A nil set is tolerated (the call is a
// no-op): a nil set has no state to guard.  Every Lock must be paired
// with an Unlock.
func (t *TreeSet[T]) Lock() {
	if t == nil {
		return
	}
	t.lock.Lock()
}

// Unlock releases the exclusive lock.  A nil set is tolerated.
func (t *TreeSet[T]) Unlock() {
	if t == nil {
		return
	}
	t.lock.Unlock()
}

// writeContractCheck applies the insert panic contract for operations
// that store elements: a nil set or a zero-value set has no comparison
// function, so it cannot store.  Call it BEFORE taking the lock for a
// nil receiver (t.lock would dereference nil) and under the lock for
// the zero-value check.  `caller` names the public method.
func (t *TreeSet[T]) writeContractCheck(caller string) {
	if t == nil || t.cmp == nil {
		panic(fmt.Sprintf("tree_set_ts: %s called on a nil or zero-value set (create the set with NewTreeSet or NewTreeSetFunc)", caller))
	}
}

// snapshot returns the set's elements and its comparison function, both
// read under one hold of the read lock — the cross-set operand read.
// A nil set snapshots as empty with no ordering.
func (t *TreeSet[T]) snapshot() (items []T, compare func(a, b T) int) {
	if t == nil {
		return nil, nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Slice(), t.cmp
}

// -------------------------------------------------------------------------------------------------------
// Core operations.

// Insert adds item to the set; a duplicate replaces it and returns
// false, true when the element was new.  The whole operation is under
// one hold of the write lock, so it is atomic to concurrent readers.
// It panics on a nil set or on a zero-value set (no comparison
// function), with a message naming this package's constructors — the
// same insert contract as the plain package.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) Insert(item T) bool {
	if t == nil {
		t.writeContractCheck("Insert") // always panics on nil
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.cmp == nil {
		t.writeContractCheck("Insert") // zero-value: same contract, under the lock
	}
	return t.inner.Insert(item)
}

// NlInsert is Insert without the locking, for callers holding the set's
// exclusive lock — the race-free compound pattern.  The nil and
// zero-value contract is the plain package's (the message names
// tree_set's constructors, because that is where the call lands).
// Complexity is O(log₂ n).
func (t *TreeSet[T]) NlInsert(item T) bool {
	return t.inner.Insert(item)
}

// Delete removes the element equal to find and reports whether one was
// present.  As with Search, `find` only needs the fields that the
// comparison function reads.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) Delete(find T) bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.Delete(find)
}

// NlDelete is Delete without the locking, for callers holding the set's
// exclusive lock.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) NlDelete(find T) bool {
	if t == nil {
		return false
	}
	return t.inner.Delete(find)
}

// Search returns the stored element equal to find, or false when the
// set does not hold one.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) Search(find T) (item T, found bool) {
	if t == nil {
		return item, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Search(find)
}

// NlSearch is Search without the locking, for callers holding the set's
// lock (read or write).
// Complexity is O(log₂ n).
func (t *TreeSet[T]) NlSearch(find T) (item T, found bool) {
	if t == nil {
		return item, false
	}
	return t.inner.Search(find)
}

// Contains reports whether the set holds an element equal to find.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) Contains(find T) bool {
	if t == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Contains(find)
}

// NlContains is Contains without the locking, for callers holding the
// set's lock (read or write).
// Complexity is O(log₂ n).
func (t *TreeSet[T]) NlContains(find T) bool {
	if t == nil {
		return false
	}
	return t.inner.Contains(find)
}

// Len returns the number of elements in the set.
// Complexity is O(1).
func (t *TreeSet[T]) Len() int {
	if t == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Len()
}

// NlLen is Len without the locking, for callers holding the set's lock
// (read or write).
// Complexity is O(1).
func (t *TreeSet[T]) NlLen() int {
	if t == nil {
		return 0
	}
	return t.inner.Len()
}

// Length is an alias for Len; it returns the number of elements in the
// set.
// Complexity is O(1).
func (t *TreeSet[T]) Length() int {
	return t.Len()
}

// IsEmpty returns true if the set has no elements.
// Complexity is O(1).
func (t *TreeSet[T]) IsEmpty() bool {
	return t.Len() == 0
}

// NlIsEmpty is IsEmpty without the locking, for callers holding the
// set's lock (read or write).
// Complexity is O(1).
func (t *TreeSet[T]) NlIsEmpty() bool {
	return t.NlLen() == 0
}

// Truncate removes all elements, keeping the comparison function.  A
// nil set or a zero-value set is left as it is.
// Complexity is O(1).
func (t *TreeSet[T]) Truncate() {
	if t == nil {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.inner.Truncate()
}

// -------------------------------------------------------------------------------------------------------
// Sorted-set riders.

// FindMin returns the smallest element of the set (by the comparator's
// order), or false if the set is empty.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) FindMin() (item T, found bool) {
	if t == nil {
		return item, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.FindMin()
}

// FindMax returns the largest element of the set (by the comparator's
// order), or false if the set is empty.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) FindMax() (item T, found bool) {
	if t == nil {
		return item, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.FindMax()
}

// Floor returns the largest element that compares <= key, or found=false
// when key is less than every element.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) Floor(key T) (rv T, found bool) {
	if t == nil {
		return rv, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Floor(key)
}

// Ceil returns the smallest element that compares >= key, or found=false
// when key is greater than every element.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) Ceil(key T) (rv T, found bool) {
	if t == nil {
		return rv, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Ceil(key)
}

// DeleteAtHead removes the smallest element of the set, returning false
// if the set is empty.  Deleting is tolerated on a nil set and a
// zero-value set (it returns false) — only storing panics.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) DeleteAtHead() bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.DeleteAtHead()
}

// DeleteAtTail removes the largest element of the set, returning false
// if the set is empty.
// Complexity is O(log₂ n).
func (t *TreeSet[T]) DeleteAtTail() bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.DeleteAtTail()
}

// Index returns the `pos`-th element of the set in ascending order
// (0-based), or false if pos is out of range.  It never panics.
// Complexity is O(n).
func (t *TreeSet[T]) Index(pos int) (item T, found bool) {
	if t == nil {
		return item, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Index(pos)
}

// -------------------------------------------------------------------------------------------------------
// Conveniences.

// Slice returns a fresh slice holding every element of the set in
// ascending order, taken under the read lock — a consistent snapshot
// safe to read from any goroutine.  The slice is a copy: it aliases
// none of the set's storage.  A nil set yields nil; an empty set yields
// an empty (non-nil) slice.
// Complexity is O(n).
func (t *TreeSet[T]) Slice() []T {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Slice()
}

// String renders the set as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in ascending order:
// "[a b c]".  The elements are snapshotted under the read lock and the
// formatting runs without it.  A nil or empty set renders as "[]".
// Complexity is O(n).
func (t *TreeSet[T]) String() string {
	return t.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function.  The elements are snapshotted under the read lock and the
// format function runs without it, so it may safely call back into the
// set.  A nil format function renders as "[]".
// Complexity is O(n).
func (t *TreeSet[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range t.Slice() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether the receiver and other hold the same set of
// elements: same length and every element of the receiver found in
// other.  Membership is compared, never shape; two empty sets are
// equal and a nil set equals an empty set.
//
// The receiver is snapshotted under its own read lock and other is
// touched one self-locking Contains call at a time — never two locks at
// once — so Equal never deadlocks, not even when two goroutines compare
// the same pair of sets in opposite directions.
//
// Precondition: both sets must use consistent comparison functions (a
// nil set trivially does); functions are not comparable, so this cannot
// be checked.
// Complexity is O(n log₂ n).
func (t *TreeSet[T]) Equal(other *TreeSet[T]) bool {
	if t.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if t.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, v := range t.Slice() { // snapshot, lock released
		if !other.Contains(v) { // one probe at a time, other's lock only
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the set: a fresh set with the same
// comparison function and a fresh copy of every node.  The copy runs
// under one hold of the read lock, so the clone is a consistent
// snapshot — and no backing memory is shared: mutating either set
// afterwards never affects the other.  A nil set clones to nil; a
// zero-value set clones to a fresh zero value.
// Complexity is O(n).
func (t *TreeSet[T]) Clone() *TreeSet[T] {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.cmp == nil {
		return &TreeSet[T]{} // a zero-value set clones to a fresh zero value
	}
	return &TreeSet[T]{inner: *t.inner.Clone(), cmp: t.cmp}
}

// -------------------------------------------------------------------------------------------------------
// Batch operations — one hold of the appropriate lock for the whole
// batch: atomic to concurrent observers.

// InsertSlice inserts each element of items and reports whether at
// least one was new.  The whole batch runs under one hold of the write
// lock.  It follows the insert contract: a batch that would store into
// a nil set or a zero-value set panics; an empty batch is tolerated
// everywhere.
// Complexity is O(k·log₂ n), k = len(items).
func (t *TreeSet[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	if t == nil {
		t.writeContractCheck("InsertSlice") // reports the nil-set contract and panics
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.cmp == nil {
		t.writeContractCheck("InsertSlice") // the zero-value contract, checked under the lock
	}
	return t.inner.InsertSlice(items)
}

// InsertIter is InsertSlice over a range-over-func iterator; every
// pluto container's All() feeds it directly.  The iterator is drained
// (and the tree snapshot it may come from taken) BEFORE the write lock
// is acquired — the lock is never held across caller iteration, so
// feeding a set its own All() is safe.  It reports whether at least one
// element was new and follows the insert contract of InsertSlice.
// Complexity is O(k·log₂ n), k = number of yielded elements.
func (t *TreeSet[T]) InsertIter(seq iter.Seq[T]) bool {
	if seq == nil {
		return false
	}
	var items []T
	for item := range seq { // drained before any lock: caller iteration is never under the lock
		items = append(items, item)
	}
	if len(items) == 0 {
		return false
	}
	if t == nil {
		t.writeContractCheck("InsertIter")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.cmp == nil {
		t.writeContractCheck("InsertIter")
	}
	return t.inner.InsertSlice(items)
}

// DeleteSlice deletes each element of items, under one hold of the
// write lock, and reports whether at least one was present.
// Complexity is O(k·log₂ n), k = len(items).
func (t *TreeSet[T]) DeleteSlice(items []T) bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.inner.DeleteSlice(items)
}

// DeleteFunc deletes every element for which fx returns true and
// reports whether anything was deleted.  The elements are snapshotted
// under the read lock, fx runs on the snapshot WITHOUT any lock held —
// it may safely call back into the set — and the matched elements are
// then deleted under one hold of the write lock, so the whole bulk
// delete is atomic to concurrent readers.
// Complexity is O(n log₂ n).
func (t *TreeSet[T]) DeleteFunc(fx func(T) bool) bool {
	if t == nil || fx == nil {
		return false
	}
	t.lock.RLock()
	snapshot := t.inner.Slice()
	t.lock.RUnlock()
	var doomed []T
	for _, v := range snapshot { // fx runs with no lock held
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

// ContainsSlice reports whether every element of items is present (an
// empty items slice is vacuously contained).  The whole batch runs
// under the read lock.
// Complexity is O(k·log₂ n), k = len(items).
func (t *TreeSet[T]) ContainsSlice(items []T) bool {
	if t == nil {
		return len(items) == 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.ContainsSlice(items)
}

// ContainsAny reports whether at least one element of items is present.
// The whole batch runs under the read lock.
// Complexity is O(k·log₂ n), k = len(items).
func (t *TreeSet[T]) ContainsAny(items []T) bool {
	if t == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.ContainsAny(items)
}

// -------------------------------------------------------------------------------------------------------
// Set algebra — cross-set discipline: snapshot each operand under that
// operand's own read lock (released), rebuild locally, install under
// the receiver's write lock.  Never two locks at once.

// Union is a set union, receiver = yy union zz; if an element is in
// both yy and zz the one from zz is kept.  Operands are snapshotted
// under their own read locks first, so either may alias the receiver.
// The result is ordered by the receiver's comparison function; a
// zero-value receiver adopts yy's (then zz's).
//
// Precondition: the sets involved must use consistent comparison
// functions; two sets built by the same constructor over the same
// element type always satisfy it.  Atomic per set, not across sets:
// an operand may change between its snapshot and the install.
// Complexity is O((n+m)·log₂(n+m)).
func (t *TreeSet[T]) Union(yy, zz *TreeSet[T]) {
	if t == nil {
		return
	}
	a, yCmp := yy.snapshot() // each operand under its own lock, then released
	b, zCmp := zz.snapshot()
	t.lock.Lock()
	defer t.lock.Unlock()
	t.adoptCmp(yCmp, zCmp)
	if t.cmp == nil {
		t.inner = tree_set.TreeSet[T]{} // no ordering anywhere: the empty result
		return
	}
	y, z := tree_set.FromSliceFunc(t.cmp, a), tree_set.FromSliceFunc(t.cmp, b)
	out := tree_set.NewTreeSetFunc[T](t.cmp)
	out.Union(y, z)
	t.inner = *out
}

// Intersect is a set intersection, receiver = yy intersect zz.  The
// operand-snapshot discipline is Union's; either operand may alias the
// receiver, and a zero-value receiver adopts an operand's ordering.
//
// Precondition: as for Union — consistent comparison functions; atomic
// per set, not across sets.
// Complexity is O((n+m)·log₂(n+m)).
func (t *TreeSet[T]) Intersect(yy, zz *TreeSet[T]) {
	if t == nil {
		return
	}
	a, yCmp := yy.snapshot()
	b, zCmp := zz.snapshot()
	t.lock.Lock()
	defer t.lock.Unlock()
	t.adoptCmp(yCmp, zCmp)
	if t.cmp == nil {
		t.inner = tree_set.TreeSet[T]{}
		return
	}
	y, z := tree_set.FromSliceFunc(t.cmp, a), tree_set.FromSliceFunc(t.cmp, b)
	out := tree_set.NewTreeSetFunc[T](t.cmp)
	out.Intersect(y, z)
	t.inner = *out
}

// Minus is a set minus, receiver = yy - zz.  The operand-snapshot
// discipline is Union's; either operand may alias the receiver, and a
// zero-value receiver adopts an operand's ordering.
//
// Precondition: as for Union — consistent comparison functions; atomic
// per set, not across sets.
// Complexity is O((n+m)·log₂(n+m)).
func (t *TreeSet[T]) Minus(yy, zz *TreeSet[T]) {
	if t == nil {
		return
	}
	a, yCmp := yy.snapshot()
	b, zCmp := zz.snapshot()
	t.lock.Lock()
	defer t.lock.Unlock()
	t.adoptCmp(yCmp, zCmp)
	if t.cmp == nil {
		t.inner = tree_set.TreeSet[T]{}
		return
	}
	y, z := tree_set.FromSliceFunc(t.cmp, a), tree_set.FromSliceFunc(t.cmp, b)
	out := tree_set.NewTreeSetFunc[T](t.cmp)
	out.Minus(y, z)
	t.inner = *out
}

// SymmetricDifference is a set symmetric difference, receiver = yy △ zz
// = the elements in exactly one of yy and zz.  The operand-snapshot
// discipline is Union's; either operand may alias the receiver, and a
// zero-value receiver adopts an operand's ordering.
//
// Precondition: as for Union — consistent comparison functions; atomic
// per set, not across sets.
// Complexity is O((n+m)·log₂(n+m)).
func (t *TreeSet[T]) SymmetricDifference(yy, zz *TreeSet[T]) {
	if t == nil {
		return
	}
	a, yCmp := yy.snapshot()
	b, zCmp := zz.snapshot()
	t.lock.Lock()
	defer t.lock.Unlock()
	t.adoptCmp(yCmp, zCmp)
	if t.cmp == nil {
		t.inner = tree_set.TreeSet[T]{}
		return
	}
	y, z := tree_set.FromSliceFunc(t.cmp, a), tree_set.FromSliceFunc(t.cmp, b)
	out := tree_set.NewTreeSetFunc[T](t.cmp)
	out.SymmetricDifference(y, z)
	t.inner = *out
}

// adoptCmp gives a zero-value receiver a comparison function, adopting
// yy's (then zz's), both read under the operand's own read lock by the
// caller.  Called under the receiver's write lock.
func (t *TreeSet[T]) adoptCmp(yCmp, zCmp func(a, b T) int) {
	if t.cmp == nil {
		t.cmp = yCmp
	}
	if t.cmp == nil {
		t.cmp = zCmp
	}
}

// -------------------------------------------------------------------------------------------------------
// Relations — the Equal discipline: snapshot the receiver under its own
// read lock, probe the other set one self-locking call at a time.

// IsSubsetOf reports whether every element of the receiver is also in
// other (the receiver ⊆ other).  The empty set is a subset of every
// set.  The receiver is snapshotted under its own read lock and other
// is probed one Contains call at a time — never two locks at once.
//
// Precondition: both sets must use consistent comparison functions (a
// nil set trivially does).
// Complexity is O(n·log₂ m).
func (t *TreeSet[T]) IsSubsetOf(other *TreeSet[T]) bool {
	if t.Len() > other.Len() {
		return false
	}
	for _, v := range t.Slice() { // snapshot, lock released
		if !other.Contains(v) { // one probe at a time, other's lock only
			return false
		}
	}
	return true
}

// IsProperSubsetOf reports whether the receiver ⊆ other and the two are
// not equal (the receiver's size is strictly less).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(n·log₂ m).
func (t *TreeSet[T]) IsProperSubsetOf(other *TreeSet[T]) bool {
	return t.Len() < other.Len() && t.IsSubsetOf(other)
}

// IsSupersetOf reports whether every element of other is also in the
// receiver (the receiver ⊇ other).  It is other.IsSubsetOf(t).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(m·log₂ n).
func (t *TreeSet[T]) IsSupersetOf(other *TreeSet[T]) bool {
	return other.IsSubsetOf(t)
}

// IsProperSupersetOf reports whether the receiver ⊃ other strictly (the
// receiver's size is strictly greater).  It is
// other.IsProperSubsetOf(t).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(m·log₂ n).
func (t *TreeSet[T]) IsProperSupersetOf(other *TreeSet[T]) bool {
	return other.IsProperSubsetOf(t)
}

// IsDisjointWith reports whether the receiver and other hold no common
// element.  Two empty sets ARE disjoint.  The lock discipline is
// IsSubsetOf's.
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(n·log₂ m).
func (t *TreeSet[T]) IsDisjointWith(other *TreeSet[T]) bool {
	for _, v := range t.Slice() { // snapshot, lock released
		if other.Contains(v) { // one probe at a time
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------------------------------------
// Iteration — snapshot iterators; live walks.

// All returns a range-over-func iterator that visits every element of
// the set in ascending order.  Unlike the plain package's All, this one
// yields a SNAPSHOT taken under the read lock: the set may be freely
// mutated — even from inside the loop body — and the iteration is
// unaffected.
// Complexity is O(n) for the snapshot.
func (t *TreeSet[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range t.Slice() { // snapshot under the read lock, lock released
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that visits every element
// of the set in descending order — a snapshot, like All: the set may be
// mutated while it is consumed.
// Complexity is O(n) for the snapshot.
func (t *TreeSet[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		snapshot := t.Slice()
		for i := len(snapshot) - 1; i >= 0; i-- {
			if !yield(snapshot[i]) {
				return
			}
		}
	}
}

// Range returns an iterator over the elements x with lo <= x <= hi, in
// ascending sequence; lo > hi iterates as empty.  Like All it yields a
// snapshot taken under the read lock — the bounds and the elements are
// captured atomically, and the set may be mutated while it is consumed.
// The bounds comparisons use the set's comparison function; as in the
// plain package the walk starts at the smallest element and stops once
// past hi (O(n) worst case, not skip_list's positioned descent).
// Complexity is O(n).
func (t *TreeSet[T]) Range(lo, hi T) iter.Seq[T] {
	return func(yield func(T) bool) {
		if t == nil {
			return
		}
		t.lock.RLock()
		compare, snapshot := t.cmp, t.inner.Slice()
		t.lock.RUnlock()
		if compare == nil || compare(lo, hi) > 0 {
			return
		}
		for _, v := range snapshot {
			if compare(v, hi) > 0 {
				return // past hi: nothing later can be in range
			}
			if compare(v, lo) >= 0 {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// WalkInOrder visits every element in ascending order; returning false
// from fx stops the walk.  The read lock is held for the WHOLE
// traversal of the live set, so the walk is atomic to concurrent
// writers — and fx must not call back into the set (it would
// self-deadlock); use All when the callback must touch the set.
// Complexity is O(n).
func (t *TreeSet[T]) WalkInOrder(fx ApplyFunction[T]) {
	if t == nil {
		return
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	t.inner.WalkInOrder(fx)
}

// WalkPreOrder visits every element in pre-order; returning false from
// fx stops the walk.  The read lock is held for the whole traversal —
// fx must not call back into the set (WalkInOrder's caveat).
// Complexity is O(n).
func (t *TreeSet[T]) WalkPreOrder(fx ApplyFunction[T]) {
	if t == nil {
		return
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	t.inner.WalkPreOrder(fx)
}

// WalkPostOrder visits every element in post-order; returning false
// from fx stops the walk.  The read lock is held for the whole
// traversal — fx must not call back into the set (WalkInOrder's
// caveat).
// Complexity is O(n).
func (t *TreeSet[T]) WalkPostOrder(fx ApplyFunction[T]) {
	if t == nil {
		return
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	t.inner.WalkPostOrder(fx)
}
