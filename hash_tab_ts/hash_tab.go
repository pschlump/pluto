/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_tab_ts implements a thread-safe generic hash table using
// separate chaining: each of a fixed number of buckets owns a singly linked
// chain of elements stored by value.  The bucket count is fixed at
// construction — when the number of elements is not known up front, use the
// auto-growing hash_grow_ts package instead.  Every operation is guarded by
// a sync.RWMutex.
//
// It is the thread-safe twin of github.com/pschlump/charon/hash_tab — the
// same API — with the addition of the Lock/Unlock pair and the Nl-prefixed
// (no-lock) methods for compound operations.  Pluto has no hash_tab_ts; the
// twin takes the pure-generics rework that is charon/hash_tab and guards it
// with one table lock, following the pattern of hash_grow_ts and
// hash_tab_bt_ts.  Element data is never boxed into an interface and never
// unboxed with a type assertion: tables of types that can be compared with
// == (the builtin comparable constraint) are created with NewHashTab, which
// hashes with the stdlib hash/maphash — every table gets its own random
// seed, equal values always hash equal, and no method has to be implemented.
// Tables of any other type — or with field-based equality — are created
// with NewHashTabFunc, which takes a caller supplied equality function and
// hash function; the element type does not have to implement any interface.
// The two functions must agree: whenever eq(a, b) is true, hash(a) and
// hash(b) must be equal.
//
// Elements are stored and returned by value (T, not *T).
//
// Operations:
//
//	Insert — add a new element to the table, replacing any existing equal element.	O(1) average, O(n) worst
//	Delete — delete the element equal to `find`, if present.					O(1) average, O(n) worst
//	Search — return the stored element equal to `find`.						O(1) average, O(n) worst
//	IsEmpty — Returns true if the table is empty.								O(1)
//	Len / Length — Returns number of elements in the table.  0 length is empty.	O(1)
//	Truncate — Delete all the elements in the table.							O(k)
//	Walk — Call a callback for each element in bucket order.					O(n)
//	Dump — Write a per-bucket listing of the table for debugging.				O(n)
//	All / Values — Range-over-func iterators over a snapshot.					O(n)
//	Lock / Unlock + Nl* — compound multi-step operations.						O(1) to lock
//
// Walk, Dump, All and Values visit the buckets in bucket order and — within
// a bucket — from the most recently inserted element to the oldest.  Bucket
// order itself depends on the hash function and, for NewHashTab, on the
// per-table random seed, so the combined iteration order is nondeterministic
// and must not appear in fixed assertions.
//
// A nil *HashTab and the zero value both behave as an empty table for every
// read: searches report not-found, Delete returns false, and the iterators
// visit nothing.
//
// The package panics in exactly four situations, all programmer errors that
// cannot be handled where they occur — each message names the fix:
//
//	NewHashTabFunc with a nil equality or hash function — caught at construction.
//	NewHashTab/NewHashTabFunc with n < 5 — a smaller table has no headroom.
//	Insert on a nil table — a nil table cannot store an element.
//	Insert on a zero-value table — no equality/hash functions; the message names the constructors.
package hash_tab_ts

import (
	"fmt"
	"hash/maphash"
	"io"
	"sync"
)

// HashTab is a generic, thread-safe hash table with a fixed number of
// separately chained buckets.  Use NewHashTab for element types that support
// ==, or NewHashTabFunc for a caller supplied equality and hash function.
// The zero value is an empty read-only table.
type HashTab[T any] struct {
	buckets []*bucketNode[T] // one chain per bucket; touched only while the table lock is held
	size    int              // number of buckets; fixed at construction
	lock    sync.RWMutex
	length  int // number of elements in the table

	// eq reports whether two elements are considered the same, and hash
	// returns a hash for an element.  Both are set by the constructors and
	// are the only things that know how to compare and hash T — T itself
	// never has to implement an interface.  They must agree: equal elements
	// must have equal hashes.
	eq   func(a, b T) bool
	hash func(a T) uint64
}

// bucketNode is one link of a bucket chain.  Chains are singly linked and
// hold the elements by value; Insert pushes at the chain head, so a chain
// runs from the most recently inserted element to the oldest.
type bucketNode[T any] struct {
	data T
	hash uint64 // the raw (un-reduced) hash, kept for Dump
	next *bucketNode[T]
}

// -------------------------------------------------------------------------------------------------------

// NewHashTab creates a hash table with n buckets (n must be at least 5).
// Elements are compared with the == operator and hashed with the stdlib
// hash/maphash using a per-table random seed — no method has to be
// implemented on T, and no element is ever boxed into an interface.
// Complexity is O(n) for the bucket allocation.
func NewHashTab[T comparable](n int) *HashTab[T] {
	var seed = maphash.MakeSeed()
	return newHashTab(
		n,
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
		"NewHashTab",
	)
}

// NewHashTabFunc creates a hash table with n buckets (n must be at least
// 5), a caller supplied equality function and a caller supplied hash
// function.  The two functions must agree: whenever eq(a, b) is true,
// hash(a) and hash(b) must be equal, otherwise Search and Delete can miss
// elements.
// Complexity is O(n) for the bucket allocation.
func NewHashTabFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, n int) *HashTab[T] {
	return newHashTab(n, eq, hash, "NewHashTabFunc")
}

// newHashTab is the shared constructor body; `caller` names the public
// constructor in panic messages.
func newHashTab[T any](n int, eq func(a, b T) bool, hash func(a T) uint64, caller string) *HashTab[T] {
	if eq == nil {
		panic(fmt.Sprintf("hash_tab_ts: %s called with a nil equality function", caller))
	}
	if hash == nil {
		panic(fmt.Sprintf("hash_tab_ts: %s called with a nil hash function", caller))
	}
	if n < 5 {
		panic(fmt.Sprintf("hash_tab_ts: %s called with n = %d, the initial size must be at least 5", caller, n))
	}
	return &HashTab[T]{
		buckets: make([]*bucketNode[T], n),
		size:    n,
		length:  0,
		eq:      eq,
		hash:    hash,
	}
}

// IsEmpty will return true if the hash table is empty.
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	if tt == nil {
		return true
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlIsEmpty()
}

// nlIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (tt *HashTab[T]) nlIsEmpty() bool {
	return tt.length == 0
}

// Lock takes the table's write lock for a compound sequence of Nl-prefixed
// operations (for example an atomic NlSearch followed by NlDelete).
// Calling a locking public method while holding Lock deadlocks, so inside
// the critical section use only the Nl methods.  Locking a nil table is a
// no-op.
func (tt *HashTab[T]) Lock() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil table is a
// no-op.
func (tt *HashTab[T]) Unlock() {
	if tt == nil {
		return
	}
	tt.lock.Unlock()
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
// Complexity is O(1).
func (tt *HashTab[T]) NlIsEmpty() bool {
	return tt.nlIsEmpty()
}

// NlLen is Len without locking; call it only while holding Lock.
// Complexity is O(1).
func (tt *HashTab[T]) NlLen() int {
	return tt.length
}

// Truncate removes all data from the table.  Every bucket is set to nil so
// each whole chain becomes garbage and the collector can reclaim the stored
// elements.  The size and the equality/hash functions are kept, so the
// table remains usable and can simply be refilled.
// Complexity is O(k), k = number of buckets.
func (tt *HashTab[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	clear(tt.buckets) // nil chains, releasing the elements for GC
	tt.length = 0
}

// hashOf returns the raw hash of `a`.  Unlike hash_grow_ts there is no
// reserved zero value to remap: an empty bucket is a nil chain, not a hash
// marker, so a hash of 0 is just another hash.
func (tt *HashTab[T]) hashOf(a T) uint64 {
	return tt.hash(a)
}

// bucketOf returns the bucket index for the raw hash `h`.
func (tt *HashTab[T]) bucketOf(h uint64) int {
	return int(h % uint64(tt.size))
}

// Insert will add a new item to the table.  If an equal item is already in
// the bucket chain it is replaced by the new one and false is returned;
// true is returned when a new element was added.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Insert(item T) bool {
	if tt == nil {
		panic("hash_tab_ts: Insert called on a nil table")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlInsert(item)
}

// NlInsert is Insert without locking; call it only while holding Lock.
// It panics on a table with no equality/hash functions (a zero-value
// table), naming the constructors.
func (tt *HashTab[T]) NlInsert(item T) bool {
	if tt.eq == nil || tt.hash == nil {
		panic("hash_tab_ts: Insert called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
	}
	h := tt.hashOf(item)
	b := tt.bucketOf(h)
	for node := tt.buckets[b]; node != nil; node = node.next {
		if tt.eq(node.data, item) { // equal element present: replace it
			node.data = item
			node.hash = h
			return false
		}
	}
	tt.buckets[b] = &bucketNode[T]{data: item, hash: h, next: tt.buckets[b]} // push at the chain head
	tt.length++
	return true
}

// Search will walk the bucket chain for `find` and return the stored
// element equal to it.  If it is not found the zero value of T and false
// are returned.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Search(find T) (rv T, found bool) {
	if tt == nil {
		return // a nil table reads as an empty one
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlSearch(find)
}

// NlSearch is Search without locking; call it only while holding Lock.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) NlSearch(find T) (rv T, found bool) {
	if tt.nlIsEmpty() || tt.eq == nil {
		return // empty or zero-value table: not found
	}
	for node := tt.buckets[tt.bucketOf(tt.hashOf(find))]; node != nil; node = node.next {
		if tt.eq(node.data, find) {
			return node.data, true
		}
	}
	return
}

// Delete an element from the table.  The element equal to `find` is located
// with the same chain walk Search uses, then unlinked from its chain in a
// single pass.  The write lock is held across the whole operation, so the
// search-and-delete is atomic.  Returns true if the element was found and
// removed.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Delete(find T) (found bool) {
	if tt == nil {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDelete(find)
}

// NlDelete is Delete without locking; call it only while holding Lock.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) NlDelete(find T) (found bool) {
	if tt.nlIsEmpty() || tt.eq == nil {
		return false
	}
	b := tt.bucketOf(tt.hashOf(find))
	var prev *bucketNode[T]
	for node := tt.buckets[b]; node != nil; node = node.next {
		if tt.eq(node.data, find) {
			if prev == nil {
				tt.buckets[b] = node.next // unlink the chain head
			} else {
				prev.next = node.next
			}
			tt.length--
			return true
		}
		prev = node
	}
	return false
}

// Len returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Length returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Length() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// ApplyFunction is the callback type for Walk.  It is called with the
// bucket position and the element stored there.  Returning false stops the
// walk (the same convention as hash_tab and the tree packages; note dll/sll
// are the opposite).
type ApplyFunction[T any] func(pos int, data T) bool

// Walk calls `fx` for each element in the table, in bucket order and —
// within a bucket — from the most recently inserted element to the oldest,
// until all elements have been visited or `fx` returns false.  It returns
// true if the walk ran to completion.
//
// The read lock is held for the whole walk: fx must not call methods on the
// same table, or the call can deadlock (use All or Values, which iterate a
// snapshot, when the loop body needs to touch the table).
// Complexity is O(n).
func (tt *HashTab[T]) Walk(fx ApplyFunction[T]) (b bool) {
	b = true
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.nlIsEmpty() {
		return
	}
	for ii := range tt.buckets {
		for node := tt.buckets[ii]; node != nil; node = node.next {
			if !fx(ii, node.data) {
				return false
			}
		}
	}
	return
}

// Dump will print out the hash table, including empty buckets, to `fo` —
// the element count and modulo size on the first line, then one line per
// element (a bucket with a chain of several elements prints one line per
// element).  The hash values shown are the per-table random-seeded raw
// hashes, so the output varies from process to process; use it for
// debugging, not for golden files.  The read lock is held for the whole
// dump.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	if tt == nil {
		_, _ = fmt.Fprintf(fo, "Elements: 0, mod size:0\n")
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	_, _ = fmt.Fprintf(fo, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i := range tt.buckets {
		if tt.buckets[i] == nil {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] empty\n", i)
			continue
		}
		for node := tt.buckets[i]; node != nil; node = node.next {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] h=%d h%%size=%d = %v\n", i, node.hash, node.hash%uint64(tt.size), node.data)
		}
	}
}
