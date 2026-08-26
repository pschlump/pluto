/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_grow_ts implements a thread-safe generic hash table using
// open addressing with linear probing.  The table automatically doubles in
// size when the load factor exceeds a configurable saturation threshold
// (default 0.5) and re-hashes every entry into the larger table.
//
// It is the thread-safe twin of github.com/pschlump/charon/hash_grow — the
// same API, guarded by a sync.RWMutex — with the addition of the Lock and
// Unlock pair and the Nl-prefixed (no-lock) methods for compound
// operations.
//
// Element data is never boxed into an interface and never unboxed with a
// type assertion.  Tables of types that can be compared with == are created
// with NewHashTab, which hashes with the stdlib hash/maphash using a
// per-table random seed; tables of any other type are created with
// NewHashTabFunc, which takes a caller supplied equality function and hash
// function.  The two functions must agree: equal elements must have equal
// hashes.
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
//	Truncate — Delete all the elements in the table.							O(n)
//	Walk — Call a callback for each element in bucket order.					O(n)
//	Dump — Write a per-bucket listing of the table for debugging.				O(n)
//	All / Values — Range-over-func iterators over a snapshot.					O(n)
//	Lock / Unlock + Nl* — compound multi-step operations.						O(1) to lock
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
package hash_grow_ts

import (
	"fmt"
	"hash/maphash"
	"io"
	"sync"
)

// HashTab is a generic, thread-safe hash table that doubles its underlying
// bucket array whenever the load factor passes the saturation threshold.
// Use NewHashTab for element types that support ==, or NewHashTabFunc for a
// caller supplied equality and hash function.  The zero value is an empty
// read-only table.
type HashTab[T any] struct {
	buckets             []T      // element storage by value; a slot is empty iff originalHash[i] == 0
	originalHash        []uint64 // the raw (un-reduced) hash per bucket; 0 marks an empty slot
	size                int      // number of buckets; doubled by each growth
	lock                sync.RWMutex
	length              int     // number of elements in the table
	saturationThreshold float64 // load factor that triggers growth of the table (default 0.5)

	// eq reports whether two elements are considered the same, and hash
	// returns a hash for an element.  Both are set by the constructors and
	// are the only things that know how to compare and hash T — T itself
	// never has to implement an interface.  They must agree: equal elements
	// must have equal hashes.
	eq   func(a, b T) bool
	hash func(a T) uint64
}

// -------------------------------------------------------------------------------------------------------

// NewHashTab creates a hash table with an initial size of n buckets (n must
// be at least 5) and the given saturation threshold.  A saturation less than
// or equal to 0 selects the default of 0.5.  Elements are compared with the
// == operator and hashed with the stdlib hash/maphash using a per-table
// random seed — no method has to be implemented on T, and no element is ever
// boxed into an interface.
// Complexity is O(n) for the bucket allocation.
func NewHashTab[T comparable](n int, saturation float64) *HashTab[T] {
	var seed = maphash.MakeSeed()
	return newHashTab(
		n, saturation,
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
		"NewHashTab",
	)
}

// NewHashTabFunc creates a hash table with an initial size of n buckets (n
// must be at least 5), the given saturation threshold (<= 0 selects the
// default of 0.5), a caller supplied equality function and a caller supplied
// hash function.  The two functions must agree: whenever eq(a, b) is true,
// hash(a) and hash(b) must be equal, otherwise Search and Delete can miss
// elements.
// Complexity is O(n) for the bucket allocation.
func NewHashTabFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, n int, saturation float64) *HashTab[T] {
	return newHashTab(n, saturation, eq, hash, "NewHashTabFunc")
}

// newHashTab is the shared constructor body; `caller` names the public
// constructor in panic messages.
func newHashTab[T any](n int, saturation float64, eq func(a, b T) bool, hash func(a T) uint64, caller string) *HashTab[T] {
	if eq == nil {
		panic(fmt.Sprintf("hash_grow_ts: %s called with a nil equality function", caller))
	}
	if hash == nil {
		panic(fmt.Sprintf("hash_grow_ts: %s called with a nil hash function", caller))
	}
	if n < 5 {
		panic(fmt.Sprintf("hash_grow_ts: %s called with n = %d, the initial size must be at least 5", caller, n))
	}
	if !(saturation > 0) { // also catches NaN
		saturation = 0.5
	}
	return &HashTab[T]{
		buckets:             make([]T, n),
		originalHash:        make([]uint64, n),
		size:                n,
		length:              0,
		saturationThreshold: saturation,
		eq:                  eq,
		hash:                hash,
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

// Truncate removes all data from the table.  All bucket and hash slots are
// cleared so the garbage collector can reclaim the stored elements.  The
// size and the equality/hash functions are kept, so the table remains
// usable and can simply be refilled.
// Complexity is O(n).
func (tt *HashTab[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	clear(tt.buckets)      // zero values of T, releasing references for GC
	clear(tt.originalHash) // re-mark every slot empty
	tt.length = 0
}

// nextIndex returns the next position in the table, wrapping at the end.
func (tt *HashTab[T]) nextIndex(xx int) (rv int) {
	rv = xx + 1
	if rv >= tt.size {
		rv = 0
	}
	return
}

// hashOf returns the raw hash of `a` with the reserved value 0 remapped to
// 1 — 0 is the marker for an empty slot in originalHash.
func (tt *HashTab[T]) hashOf(a T) uint64 {
	rv := tt.hash(a)
	if rv == 0 {
		rv = 1
	}
	return rv
}

// Insert will add a new item to the table.  If it is a duplicate of an
// existing item the new item will replace the existing one and false is
// returned; true is returned when a new element was added.  When the load
// factor passes the saturation threshold the table is doubled and every
// entry is re-hashed.
// Complexity is O(1) average, O(n) worst case; growth is amortized O(1).
func (tt *HashTab[T]) Insert(item T) bool {
	if tt == nil {
		panic("hash_grow_ts: Insert called on a nil table")
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
		panic("hash_grow_ts: Insert called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
	}

	rh := tt.hashOf(item)
	var placed, added bool
	for {
		placed, added = tt.insertNewItem(rh, item)
		if placed {
			break
		}
		// The table was completely full — only reachable when a saturation
		// of 1.0 or more has deferred growth until now.  Growth halves the
		// load factor, so the retry always places the item.
		tt.grow()
	}

	// Grow (double) the table when the load factor passes the threshold.
	if float64(tt.length)/float64(tt.size) > tt.saturationThreshold {
		tt.grow()
	}
	return added
}

// insertNewItem places `item` with raw hash `rh` into the table, replacing
// an equal item if one is found along the probe chain.  It returns whether
// the item was placed and whether it was added (true) or replaced an equal
// item (false).  A table created by the constructors always has room, so
// "not placed" can only be reached on a table whose growth was deferred by a
// saturation of 1.0 or more until it became completely full.
func (tt *HashTab[T]) insertNewItem(rh uint64, item T) (placed, added bool) {
	hh := int(rh % uint64(tt.size))
	for probes := 0; probes < tt.size; probes++ {
		if tt.originalHash[hh] == 0 { // empty slot
			tt.buckets[hh] = item
			tt.originalHash[hh] = rh
			tt.length++
			return true, true
		}
		if tt.eq(tt.buckets[hh], item) { // equal key present: replace it
			tt.buckets[hh] = item
			tt.originalHash[hh] = rh
			return true, false
		}
		hh = tt.nextIndex(hh) // collision: linear probe onward
	}
	return false, false // full cycle with no empty slot and no equal item
}

// grow doubles the table and re-hashes every entry into the new bucket
// array.  The stored raw hashes are reused — nothing is re-hashed with the
// element hash function.
func (tt *HashTab[T]) grow() {
	oldBuckets, oldOriginal := tt.buckets, tt.originalHash
	tt.size = tt.size * 2
	tt.length = 0
	tt.buckets = make([]T, tt.size)
	tt.originalHash = make([]uint64, tt.size)
	for i := range oldBuckets {
		if oldOriginal[i] != 0 {
			tt.insertNewItem(oldOriginal[i], oldBuckets[i])
		}
	}
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

// Search will walk the probe chain from the home bucket looking for `find`
// and return the stored element equal to it.  If it is not found the zero
// value of T and false are returned.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Search(find T) (rv T, found bool) {
	if tt == nil {
		return // a nil table iterates as an empty one
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
	h := int(tt.hashOf(find) % uint64(tt.size))
	for probes := 0; probes < tt.size; probes++ {
		if tt.originalHash[h] == 0 {
			return // an empty slot ends the probe chain: not found
		}
		if tt.eq(tt.buckets[h], find) {
			return tt.buckets[h], true
		}
		h = tt.nextIndex(h)
	}
	return // full cycle (a completely full table): not found
}

// Delete an element from the table.  The element equal to `find` is located
// with the same probe walk Search uses, then removed with a backward-shift
// deletion that keeps probe chains contiguous.  Returns true if the element
// was found and removed.
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
// It is atomic with the search (the write lock is held across it), so a
// Delete-then-Search race cannot resurrect an element.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) NlDelete(find T) (found bool) {
	if tt.nlIsEmpty() || tt.eq == nil {
		return false
	}
	rh := tt.hashOf(find)
	h := int(rh % uint64(tt.size))

	for probes := 0; probes < tt.size; probes++ {
		if tt.originalHash[h] == 0 {
			return false
		}
		if tt.eq(tt.buckets[h], find) {
			tt.originalHash[h] = 0 // found: clear the slot (the T zero value remains, hidden by the empty marker)
			var zero T
			tt.buckets[h] = zero // release the reference for GC
			tt.length--          // one less element
			found = true

			// Backward-shift deletion: fill the gap at h with any following
			// element whose probe sequence passes through the gap.  This keeps
			// probe chains contiguous so Search can stop at the first truly
			// empty (originalHash == 0) slot.
			//
			// Before:    A1 A2 b A3 b b c A4 __
			// Delete '2nd' A
			// Before:    A1 __ b A3 b b c A4 __
			// Move Up:   A1 A3 b A4 b b c __ __
			gap := h
			for hf := h; ; {
				hf = tt.nextIndex(hf)
				if tt.originalHash[hf] == 0 {
					break // the scan reaches an empty slot (or the gap itself after a full wrap)
				}
				home := int(tt.originalHash[hf] % uint64(tt.size))
				if !inProbeRange(gap, hf, home) {
					tt.buckets[gap] = tt.buckets[hf]
					tt.originalHash[gap] = tt.originalHash[hf]
					tt.buckets[hf] = zero
					tt.originalHash[hf] = 0
					gap = hf
				}
			}
			return
		}
		h = tt.nextIndex(h)
	}
	return false // full cycle: not present
}

// inProbeRange reports whether `home` is in the cyclic half-open interval
// (gap, hf] of table positions.  An element at `hf` whose home position is in
// this range never probed through `gap` and therefore must not be moved into
// the gap.
func inProbeRange(gap, hf, home int) bool {
	if gap < hf {
		return home > gap && home <= hf
	}
	return home > gap || home <= hf
}

// ApplyFunction is the callback type for Walk.  It is called with the bucket
// position and the element stored there.  Returning false stops the walk (the
// same convention as the tree packages; note dll/sll are the opposite).
type ApplyFunction[T any] func(pos int, data T) bool

// Walk calls `fx` for each element in the table, in bucket order, until all
// elements have been visited or `fx` returns false.  It returns true if the
// walk ran to completion.
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
		if tt.originalHash[ii] != 0 {
			if !fx(ii, tt.buckets[ii]) {
				return false
			}
		}
	}
	return
}

// Dump will print out the hash table, including empty buckets, to `fo` —
// the element count and modulo size on the first line, then one line per
// bucket.  The hash values shown are the per-table random-seeded raw hashes,
// so the output varies from process to process; use it for debugging, not
// for golden files.  The read lock is held for the whole dump.
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
		if tt.originalHash[i] == 0 {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] empty\n", i)
			continue
		}
		_, _ = fmt.Fprintf(fo, "bucket [%04d] h=%d h%%size=%d = %v\n", i, tt.originalHash[i], tt.originalHash[i]%uint64(tt.size), tt.buckets[i])
	}
}
