/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package binary_search_st_ts is the thread-safe twin of
// binary_search_st: the same ordered symbol table surface — two
// parallel arrays kept sorted by key, binary-search reads, shift
// writes — behind a sync.RWMutex.  It is the tree_set_ts wrapper-twin
// pattern: the twin holds a plain binary_search_st.BinarySearchST by
// value plus the one mutex, every public method is a nil-guard, a lock
// and a call straight into the plain package, and the Nl* variants are
// the same calls for callers that already hold the lock.
//
// The public API, the panic contract and every observable behavior are
// binary_search_st's; read that package's documentation for the
// operation semantics.  What this package adds is the locking:
//
//	Reads (Get, Contains, FindMin, FindMax, Floor, Ceil, Rank,
//	AtIndex, CountRange, Len, IsEmpty, Keys, the JSON marshal) take
//	the read lock for the whole call.
//
//	Writes (Put, Delete, DeleteAtHead, DeleteAtTail, DeleteRange,
//	Truncate, the JSON unmarshal) take the write lock once for the
//	whole operation.
//
//	Iterators (All, Backward, Range) take a snapshot under the read
//	lock and yield the copy, so the table may be freely mutated — even
//	from inside the loop body.
//
// Callback discipline: StringFunc's format function and DeleteFunc's
// predicate run with NO lock held, on a snapshot — both may safely
//
//	call back into the table.  PutIter drains the iterator (and takes
//
// any table snapshot it may come from) BEFORE the write lock is
//
//	acquired, so feeding a table its own All() is safe.
//
//	Compound Nl* surface: NlPut, NlGet, NlDelete, NlContains, NlLen
//	and NlIsEmpty operate without taking the lock, for the
//	race-free-compound pattern — hold Lock across a multi-step
//	sequence and call the Nl* forms inside.  The riders, batch and
//	iterator methods are self-locking and have no Nl form.
//
// As with binary_search_st, a nil *BinarySearchST and the zero value
// behave as an empty table for every read, and the package panics in
// exactly the same two situations, with twin-named messages: a nil
// comparison function at construction (NewBinarySearchSTFunc), and
// Put — or any operation that would store an entry, PutIter and a
// storing UnmarshalJSON included — on a nil table or a zero-value
// table.  The messages name this package's constructors.
package binary_search_st_ts

import (
	"cmp"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/binary_search_st"
)

// BinarySearchST is a thread-safe ordered symbol table from keys of
// type K to values of type V: the binary_search_st.BinarySearchST
// surface behind a sync.RWMutex.  Use NewBinarySearchST for naturally
// ordered key types (numbers, strings) or NewBinarySearchSTFunc for a
// caller supplied comparison function.  The zero value is an empty
// table (reads as empty; Put panics).
type BinarySearchST[K, V any] struct {
	inner binary_search_st.BinarySearchST[K, V]
	lock  sync.RWMutex

	// cmp is the twin's own copy of the comparison function, the same
	// one inner was built with (binary_search_st does not export its
	// comparator).  It is written only by the constructors, and read
	// by the write-contract check and by Clone — both under the write
	// or read lock respectively.  Invariant: cmp != nil exactly when
	// the twin was built by a constructor, which is exactly when inner
	// can store.
	cmp func(a, b K) int
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by
// NewBinarySearchST (the plain package's Compare) and is handy for
// building custom comparison functions.
func Compare[T cmp.Ordered](a, b T) int {
	return binary_search_st.Compare[T](a, b)
}

// -------------------------------------------------------------------------------------------------------

// NewBinarySearchST creates a new, empty, thread-safe symbol table for
// any naturally ordered key type (all integers, floats and strings —
// cmp.Ordered).  Ordering uses the built-in < and > operators of K; no
// interface and no boxing is involved.
// Complexity is O(1).
func NewBinarySearchST[K cmp.Ordered, V any]() *BinarySearchST[K, V] {
	return newST[K, V](binary_search_st.Compare[K])
}

// NewBinarySearchSTFunc creates a new, empty, thread-safe symbol table
// that orders keys with the caller supplied comparison function fx —
// for example a struct ordered by one of its fields.  fx must return a
// negative value if a sorts before b, 0 if the two are equal and a
// positive value if a sorts after b, and must order keys consistently.
// It panics if fx is nil.
// Complexity is O(1).
func NewBinarySearchSTFunc[K, V any](fx func(a, b K) int) *BinarySearchST[K, V] {
	if fx == nil {
		panic("binary_search_st_ts: NewBinarySearchSTFunc called with a nil comparison function")
	}
	return newST[K, V](fx)
}

// newST builds the twin around a plain table carrying the same
// comparison function.
func newST[K, V any](fx func(a, b K) int) *BinarySearchST[K, V] {
	return &BinarySearchST[K, V]{
		inner: *binary_search_st.NewBinarySearchSTFunc[K, V](fx),
		cmp:   fx,
	}
}

// writeContractCheck applies the Put panic contract for the operations
// that store entries, with this package's name.  `caller` names the
// public operation in the message.
func (st *BinarySearchST[K, V]) writeContractCheck(caller string) {
	if st == nil {
		panic(fmt.Sprintf("binary_search_st_ts: %s called on a nil symbol table (create the table with NewBinarySearchST or NewBinarySearchSTFunc)", caller))
	}
	if st.cmp == nil {
		panic(fmt.Sprintf("binary_search_st_ts: %s called on a zero-value symbol table with no comparison function (create the table with NewBinarySearchST or NewBinarySearchSTFunc)", caller))
	}
}

// IsEmpty returns true if the table has no entries.  A nil table and a
// zero-value table are both empty.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) IsEmpty() bool {
	return st.Len() == 0
}

// Len returns the number of entries in the table.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) Len() int {
	if st == nil {
		return 0
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Len()
}

// Length returns the number of entries in the table.  It is an alias
// for Len.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) Length() int {
	return st.Len()
}

// Truncate removes all entries from the table, releasing the backing
// arrays so the GC can reclaim them.  The comparison function is kept,
// so the table stays usable after truncating.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) Truncate() {
	if st == nil {
		return
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	st.inner.Truncate()
}

// Put stores val under key, replacing the value of an equal key.  It
// panics on a nil table or on a zero-value table (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty table.
// Complexity is O(log₂ n) to find the position plus O(n) for the
// insert shift.
func (st *BinarySearchST[K, V]) Put(key K, val V) {
	st.writeContractCheck("Put")
	st.lock.Lock()
	defer st.lock.Unlock()
	st.inner.Put(key, val)
}

// Get returns the value stored under key and whether it was found.
// Get on a nil table or a table without the key reports false.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Get(key K) (val V, found bool) {
	if st == nil {
		return
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Get(key)
}

// Delete removes the entry stored under key and reports whether it was
// found.
// Complexity is O(log₂ n) to find the entry plus O(n) for the delete
// shift.
func (st *BinarySearchST[K, V]) Delete(key K) (found bool) {
	if st == nil {
		return false
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	return st.inner.Delete(key)
}

// Contains reports whether key is present in the table.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Contains(key K) bool {
	if st == nil {
		return false
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Contains(key)
}

// FindMin returns the smallest key and its value.  It reports false on
// an empty table (a nil table and a zero-value table included).
// Complexity is O(1).
func (st *BinarySearchST[K, V]) FindMin() (key K, val V, found bool) {
	if st == nil {
		return
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.FindMin()
}

// FindMax returns the largest key and its value.  It reports false on
// an empty table (a nil table and a zero-value table included).
// Complexity is O(1).
func (st *BinarySearchST[K, V]) FindMax() (key K, val V, found bool) {
	if st == nil {
		return
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.FindMax()
}

// DeleteAtHead removes the entry with the smallest key and reports
// whether anything was removed.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) DeleteAtHead() (found bool) {
	if st == nil {
		return false
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	return st.inner.DeleteAtHead()
}

// DeleteAtTail removes the entry with the largest key and reports
// whether anything was removed.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) DeleteAtTail() (found bool) {
	if st == nil {
		return false
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	return st.inner.DeleteAtTail()
}

// -------------------------------------------------------------------------------------------------------
// Exposed write lock and the Nl (no-lock) methods for compound
// operations.  While the lock is held only call the Nl-prefixed
// methods; calling a regular method will deadlock.
// -------------------------------------------------------------------------------------------------------

// Lock takes the table's write lock, allowing a group of operations to
// be performed atomically.  While the lock is held only call the
// Nl-prefixed (no-lock) methods; calling a regular method will
// deadlock.  Pair every Lock with a corresponding Unlock.  Locking a
// nil table is a no-op.
func (st *BinarySearchST[K, V]) Lock() {
	if st == nil {
		return
	}
	st.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil table
// is a no-op.
func (st *BinarySearchST[K, V]) Unlock() {
	if st == nil {
		return
	}
	st.lock.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (st *BinarySearchST[K, V]) NlLen() int {
	if st == nil {
		return 0
	}
	return st.inner.Len()
}

// NlIsEmpty is IsEmpty without locking; call it only while holding
// Lock.
func (st *BinarySearchST[K, V]) NlIsEmpty() bool {
	return st.NlLen() == 0
}

// NlPut is Put without locking; call it only while holding Lock.  It
// follows the Put contract of the inner table: on a zero-value twin
// the plain package's own panic fires (carrying the binary_search_st:
// prefix).
func (st *BinarySearchST[K, V]) NlPut(key K, val V) {
	st.inner.Put(key, val)
}

// NlGet is Get without locking; call it only while holding Lock.
func (st *BinarySearchST[K, V]) NlGet(key K) (val V, found bool) {
	if st == nil {
		return
	}
	return st.inner.Get(key)
}

// NlDelete is Delete without locking; call it only while holding Lock.
func (st *BinarySearchST[K, V]) NlDelete(key K) (found bool) {
	if st == nil {
		return false
	}
	return st.inner.Delete(key)
}

// NlContains is Contains without locking; call it only while holding
// Lock.
func (st *BinarySearchST[K, V]) NlContains(key K) bool {
	if st == nil {
		return false
	}
	return st.inner.Contains(key)
}
