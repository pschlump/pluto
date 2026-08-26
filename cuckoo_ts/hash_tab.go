/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package cuckoo_ts implements a thread-safe generic cuckoo hash table.
// Every element is hashed once into a full 64-bit base hash (the stdlib
// hash/maphash with a per-table random seed, or a caller supplied hash
// function), and from that one base hash four candidate slot positions are
// derived: the table size is a power of two, and with mask = size-1 the
// candidates are
//
//	pos1 =  h        & mask
//	pos2 = (h >> 1)  & mask
//	pos3 = (h >> 2)  & mask
//	pos4 = (h >> 3)  & mask
//
// — the first hash is the base hash itself masked to the table, and each
// further hash is the base hash arithmetic-shifted right one more bit and
// masked again.  An element may live in any one of its four candidate slots.
// Search and delete look at exactly those four slots — O(1), no probing.  An
// insert that finds all four candidates occupied displaces the element in one
// of them (the "cuckoo" step) and re-places the displaced element in one of
// its own candidates, repeating until every element has a slot.
//
// The full 64-bit base hash is stored beside each element, so a resize
// re-derives every position from the stored hash without calling the hash
// function again, and searches can compare hashes before comparing elements.
//
// It is the thread-safe twin of github.com/pschlump/charon/cuckoo — the same
// API, guarded by a sync.RWMutex — with the addition of the Lock and Unlock
// pair, the Nl-prefixed (no-lock) methods for compound operations, and
// background resizes: in this package a resize is triggered either by a
// collision loop (an insert's displacement chain running past the kick limit
// — that resize runs synchronously, because the insert cannot return until
// its element is placed) or by the saturation — Len divided by the table
// size — passing a threshold.  A saturation above the grow threshold
// (default 0.85) or below the shrink threshold (default 0.10, only checked
// after a deletion; the table halves down to the minimum size of 256) starts a
// goroutine that takes the table's write lock and rebuilds the table on its
// own thread, re-checking the conditions after each rebuild and exiting when
// neither is due.  At most one background resizer runs per table; inserts
// and deletes that cross a threshold while one is in flight simply wake it.
// The table is always consistent — operations between trigger and rebuild
// run against the current (valid, merely saturated) table.
//
// Both thresholds are set at construction; a value <= 0 or NaN selects its
// default, and a shrink threshold >= the grow threshold selects both
// defaults.  The shrink threshold is clamped to at most half the grow
// threshold — growing halves the saturation, so a higher shrink threshold
// would make the background resizer oscillate (grow, shrink, grow, ...)
// forever.
//
// Tables of types that can be compared with == are created with NewHashTab,
// which hashes with a per-table random hash/maphash seed; tables of any other
// type — or with field-based equality — are created with NewHashTabFunc,
// which takes a caller supplied equality function and hash function.  The two
// functions must agree: whenever eq(a, b) is true, hash(a) and hash(b) must
// be equal, otherwise Search and Delete can miss elements.  Elements are
// stored and returned by value (T, not *T).
//
// Operations:
//
//	Insert — add a new element to the table, replacing any existing equal element.	O(1) average, O(n) worst
//	Delete — delete the element equal to `find`, if present.					    O(1) average
//	Search — return the stored element equal to `find`.						        O(1)
//	IsEmpty — Returns true if the table is empty.								    O(1)
//	Len / Length — Returns number of elements in the table.  0 length is empty.	    O(1)
//	Capacity / Saturation — Returns the table size and its load factor.			    O(1)
//	Truncate — Delete all the elements in the table.							    O(n)
//	Walk — Call a callback for each element in slot order.					        O(n)
//	Dump — Write a per-slot listing of the table for debugging.				        O(n)
//	All / Values — Range-over-func iterators over a snapshot.					    O(n)
//	Lock / Unlock + Nl* — compound multi-step operations.						    O(1) to lock
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
//
// One further, pathological panic exists (raised on the background resizer
// when a growth rebuild triggers it): if several distinct elements' four
// candidate positions coincide at every table size — at the extreme, more
// than four elements sharing one 64-bit hash — those elements compete for
// the same slots no matter how large the table grows.  It cannot be reached
// through NewHashTab for practical purposes: with the per-table random
// maphash seed, distinct elements essentially never share all 64 bits.
package cuckoo_ts

import (
	"fmt"
	"hash/maphash"
	"io"
	"sync"
)

// Structural constants of the cuckoo scheme.
const (
	numPositions = 4   // candidate slots per element (one per derived hash)
	minTableSize = 256 // smallest table size; also the floor for shrinking
	maxKicks     = 64  // displacement limit before a resize is forced

	defaultGrowAt     = 0.85 // saturation above which the table grows
	defaultShrinkAt   = 0.10 // saturation below which the table shrinks
	maxResizeAttempts = 8    // resize escalations before the pathological panic
)

// slot is one table position: the element by value, its full 64-bit base
// hash, and the occupied flag (so a base hash of exactly 0 is a legal hash).
type slot[T any] struct {
	data T
	hash uint64
	used bool
}

// HashTab is a generic, thread-safe cuckoo hash table with four candidate
// positions per element.  Use NewHashTab for element types that support ==,
// or NewHashTabFunc for a caller supplied equality and hash function.  The
// zero value is an empty read-only table.
type HashTab[T any] struct {
	slots  []slot[T] // element storage by value; slot i is empty iff !slots[i].used
	size   int       // len(slots); always a power of two
	mask   uint64    // size - 1, used to derive the four positions from a hash
	lock   sync.RWMutex
	length int     // number of elements in the table
	evict  uint32  // rotating displacement start index, decorrelating inserts
	growAt float64 // saturation above which the table doubles (default 0.85)

	shrinkAt float64 // saturation below which the table halves (default 0.10)
	// resizing records that the background resize goroutine is in flight so
	// at most one runs per table.  It is guarded by the write lock: triggers
	// are detected while holding it, and the goroutine clears it while
	// holding it.
	resizing bool

	// eq reports whether two elements are considered the same, and hash
	// returns the 64-bit base hash for an element.  Both are set by the
	// constructors and are the only things that know how to compare and hash
	// T — T itself never has to implement an interface.  They must agree:
	// equal elements must have equal hashes.
	eq   func(a, b T) bool
	hash func(a T) uint64
}

// -------------------------------------------------------------------------------------------------------

// NewHashTab creates a cuckoo hash table sized to the next power of two at or
// above n (n must be at least 5; the minimum table size is 256) with the given
// grow and shrink saturation thresholds (<= 0 or NaN selects 0.85 and 0.10; a
// shrink threshold >= the grow threshold selects both defaults;
// the shrink threshold is clamped to at most half the grow threshold).  Elements
// are compared with the == operator and hashed with the stdlib hash/maphash
// using a per-table random seed — no method has to be implemented on T, and
// no element is ever boxed into an interface.
// Complexity is O(n) for the slot allocation.
func NewHashTab[T comparable](n int, growAt, shrinkAt float64) *HashTab[T] {
	var seed = maphash.MakeSeed()
	return newHashTab(
		n, growAt, shrinkAt,
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
		"NewHashTab",
	)
}

// NewHashTabFunc creates a cuckoo hash table sized to the next power of two
// at or above n (n must be at least 5; the minimum table size is 256), with the
// given grow and shrink saturation thresholds (<= 0 or NaN selects 0.85 and
// 0.10; a shrink threshold >= the grow threshold selects both defaults; the
// shrink threshold is clamped to at most half the grow threshold), a
// caller supplied equality function and a caller supplied hash function.  The
// two functions must agree: whenever eq(a, b) is true, hash(a) and hash(b)
// must be equal, otherwise Search and Delete can miss elements.
// Complexity is O(n) for the slot allocation.
func NewHashTabFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, n int, growAt, shrinkAt float64) *HashTab[T] {
	return newHashTab(n, growAt, shrinkAt, eq, hash, "NewHashTabFunc")
}

// newHashTab is the shared constructor body; `caller` names the public
// constructor in panic messages.
func newHashTab[T any](n int, growAt, shrinkAt float64, eq func(a, b T) bool, hash func(a T) uint64, caller string) *HashTab[T] {
	if eq == nil {
		panic(fmt.Sprintf("cuckoo_ts: %s called with a nil equality function", caller))
	}
	if hash == nil {
		panic(fmt.Sprintf("cuckoo_ts: %s called with a nil hash function", caller))
	}
	if n < 5 {
		panic(fmt.Sprintf("cuckoo_ts: %s called with n = %d, the initial size must be at least 5", caller, n))
	}
	if !(growAt > 0) { // also catches NaN
		growAt = defaultGrowAt
	}
	if !(shrinkAt > 0) {
		shrinkAt = defaultShrinkAt
	}
	if shrinkAt >= growAt { // an inverted band has no sane meaning: both defaults
		growAt, shrinkAt = defaultGrowAt, defaultShrinkAt
	}
	if shrinkAt > growAt/2 {
		// Hysteresis: growing halves the saturation, so a shrink threshold
		// above half the grow threshold would make a resize oscillate
		// (grow, shrink, grow, ...) forever.
		shrinkAt = growAt / 2
	}
	size := nextPowerOfTwo(n)
	return &HashTab[T]{
		slots:    make([]slot[T], size),
		size:     size,
		mask:     uint64(size - 1),
		length:   0,
		evict:    0,
		growAt:   growAt,
		shrinkAt: shrinkAt,
		eq:       eq,
		hash:     hash,
	}
}

// nextPowerOfTwo returns the smallest power of two that is at least n, but
// never smaller than minTableSize.
func nextPowerOfTwo(n int) int {
	p := minTableSize
	for p < n {
		p <<= 1
	}
	return p
}

// posOf returns the candidate position derived by shifting `h` right `i` bits
// and masking to the table — i = 0 is the first hash (the base hash itself),
// i = 1..3 the second through fourth hashes, each shifted one bit further.
func posOf(h uint64, i int, mask uint64) int {
	return int((h >> uint(i)) & mask)
}

// savedSlot records the original content of a position a displacement chain
// is about to overwrite, so a chain that runs out of kicks can be rolled
// back and leave the array exactly as it found it.
type savedSlot[T any] struct {
	pos int
	old slot[T]
}

// placeInto places (item, h) into the given slot array using the standard
// cuckoo displacement: if any of the four candidate positions is empty the
// item lands there; otherwise the element in position startIdx is displaced
// and the process repeats for it, for at most maxKicks displacements.  It
// returns false when the kick limit is exceeded without finding a slot — the
// collision loop that forces a resize — with the array rolled back to its
// original content (a failed chain must not leave the item half-placed or
// drop a displaced element).  The array's element count is not tracked here;
// the caller adjusts it.
func placeInto[T any](slots []slot[T], mask uint64, h uint64, item T, startIdx int) bool {
	cur, curH := item, h
	idx := startIdx & (numPositions - 1)

	// The buffer of first-writes; the chain touches at most one new position
	// per displacement, so maxKicks entries suffice.  Most inserts never
	// displace and never touch it.
	var saved [maxKicks]savedSlot[T]
	nSaved := 0
	save := func(p int) {
		for i := 0; i < nSaved; i++ {
			if saved[i].pos == p {
				return // only the first write to a position needs restoring
			}
		}
		saved[nSaved] = savedSlot[T]{pos: p, old: slots[p]}
		nSaved++
	}
	rollback := func() {
		for i := 0; i < nSaved; i++ {
			slots[saved[i].pos] = saved[i].old
		}
	}

	for kicks := 0; kicks <= maxKicks; kicks++ {
		for i := 0; i < numPositions; i++ { // any empty candidate takes the element
			p := posOf(curH, i, mask)
			if !slots[p].used {
				slots[p] = slot[T]{data: cur, hash: curH, used: true}
				return true
			}
		}
		if kicks == maxKicks {
			rollback() // collision loop: undo the chain, report the failure
			return false
		}
		p := posOf(curH, idx, mask) // displace and carry the evicted element on
		save(p)
		displaced := slots[p]
		slots[p] = slot[T]{data: cur, hash: curH, used: true}
		cur, curH = displaced.data, displaced.hash
		idx = (idx + 1) & (numPositions - 1)
	}
	rollback()
	return false
}

// saturation returns the load factor: length divided by table size.  It is 0
// for a zero-value table.  The caller must hold the lock (either kind).
func (tt *HashTab[T]) saturation() float64 {
	if tt.size == 0 {
		return 0
	}
	return float64(tt.length) / float64(tt.size)
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
// operations (for example an atomic NlSearch followed by NlDelete).  Calling
// a locking public method while holding Lock deadlocks, so inside the
// critical section use only the Nl methods.  A threshold-triggered resize
// runs on a background goroutine that also needs this write lock — do not
// hold Lock indefinitely.  Locking a nil table is a no-op.
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

// Truncate removes all data from the table.  Every slot is cleared so the
// garbage collector can reclaim the stored elements.  The table size and the
// equality/hash functions are kept, so the table remains usable and can
// simply be refilled.  A background resizer, if one is in flight, wakes,
// finds nothing to do, and exits.
// Complexity is O(n).
func (tt *HashTab[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	clear(tt.slots) // zero values of T and zero hashes, releasing references for GC
	tt.length = 0
}

// Insert will add a new item to the table.  If it is a duplicate of an
// existing item the new item will replace the existing one and false is
// returned; true is returned when a new element was added.  When all four
// candidate slots are occupied the insert displaces existing elements (the
// cuckoo step); a displacement chain that exceeds the kick limit forces a
// synchronous resize, and an addition that passes the grow threshold starts
// the background resize goroutine.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Insert(item T) bool {
	if tt == nil {
		panic("cuckoo_ts: Insert called on a nil table")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlInsert(item)
}

// NlInsert is Insert without locking; call it only while holding Lock.
// It panics on a table with no equality/hash functions (a zero-value table),
// naming the constructors.
func (tt *HashTab[T]) NlInsert(item T) bool {
	if tt.eq == nil || tt.hash == nil {
		panic("cuckoo_ts: Insert called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
	}
	return tt.insertItem(tt.hash(item), item)
}

// insertItem places `item` with base hash `h`, replacing an equal element if
// one already occupies a candidate slot, escalating through synchronous
// resizes when the kick limit is exceeded.  It returns true when a new
// element was added and false when an equal element was replaced.
func (tt *HashTab[T]) insertItem(h uint64, item T) bool {
	for attempt := 0; ; attempt++ {
		// An equal element may already occupy one of the four candidate
		// slots — replace it in place; the element count does not change.
		for i := 0; i < numPositions; i++ {
			p := posOf(h, i, tt.mask)
			if s := &tt.slots[p]; s.used && tt.eq(s.data, item) {
				s.data = item
				s.hash = h
				return false
			}
		}
		// The rotating start index decorrelates the displacement chains of
		// successive inserts.
		if placeInto(tt.slots, tt.mask, h, item, int(tt.evict)) {
			tt.evict++
			tt.length++
			// Unlike the plain package, the threshold-triggered resize runs
			// on the background goroutine — this insert returns against the
			// current table, which stays valid (merely saturated) until the
			// rebuild lands.
			if tt.saturation() > tt.growAt {
				tt.spawnResize()
			}
			return true
		}
		// Collision loop: more slots are the only way out, and this insert
		// cannot return until its element is placed — so this resize is
		// synchronous even in the thread-safe twin.
		if attempt >= maxResizeAttempts {
			panic(fmt.Sprintf(
				"cuckoo_ts: Insert could not place an element after %d resizes — the hash function produces elements whose candidate positions coincide at every table size (for example more than %d elements sharing one hash value); use a hash function with fewer collisions",
				maxResizeAttempts, numPositions))
		}
		tt.resizeTo(tt.size * 2)
	}
}

// tryRebuild rebuilds the table into a fresh slot array of newSize (a power
// of two), re-deriving every position from the stored base hashes — the hash
// function is not called again.  It returns false without touching the
// current table when some element cannot be placed (only possible when the
// stored hashes collide pathologically).  The caller must hold the write
// lock.
func (tt *HashTab[T]) tryRebuild(newSize int) bool {
	newSlots := make([]slot[T], newSize)
	newMask := uint64(newSize - 1)
	for i := range tt.slots {
		if !tt.slots[i].used {
			continue
		}
		s := tt.slots[i]
		// The top hash bits pick the first displacement position; the low
		// bits are already spoken for by the candidate positions themselves.
		if !placeInto(newSlots, newMask, s.hash, s.data, int(s.hash>>60)) {
			return false // the partial rebuild is discarded; the old table stands
		}
	}
	tt.slots = newSlots
	tt.size = newSize
	tt.mask = newMask
	return true
}

// resizeTo rebuilds the table at newSize, doubling on each failed rebuild —
// the synchronous resize path (a collision loop inside an insert, or a
// background growth).  Panics when even the escalated rebuilds cannot
// separate the elements (see the pathological panic in the package comment).
// The caller must hold the write lock.
func (tt *HashTab[T]) resizeTo(newSize int) {
	for attempt := 0; ; attempt++ {
		if tt.tryRebuild(newSize << uint(attempt)) {
			return
		}
		if attempt+1 >= maxResizeAttempts {
			panic(fmt.Sprintf(
				"cuckoo_ts: resize to %d failed after %d attempts — the hash function produces elements whose candidate positions coincide at every table size; use a hash function with fewer collisions",
				newSize, maxResizeAttempts))
		}
	}
}

// desiredSize reports whether a threshold-triggered resize is currently due
// and the size to rebuild at: double when the saturation is above the grow
// threshold, half when it is below the shrink threshold and the table is
// above the minimum size.  An empty table needs nothing (Truncate does not
// shrink).  The caller must hold the write lock.
func (tt *HashTab[T]) desiredSize() (newSize int, needed bool) {
	if tt.length == 0 || tt.eq == nil || tt.hash == nil {
		return 0, false
	}
	sat := tt.saturation()
	if sat > tt.growAt {
		return tt.size * 2, true
	}
	if sat < tt.shrinkAt && tt.size > minTableSize {
		return tt.size / 2, true
	}
	return 0, false
}

// spawnResize starts the background resize goroutine when one is not already
// in flight.  Call it while holding the write lock (the trigger was detected
// under it); the goroutine takes the write lock itself for each rebuild, so
// its work runs between — never during — other operations.
func (tt *HashTab[T]) spawnResize() {
	if tt.resizing {
		return // the running goroutine re-checks the conditions after each rebuild
	}
	tt.resizing = true
	go tt.backgroundResize()
}

// backgroundResize is the body of the background resizer: repeatedly take
// the write lock, re-evaluate the thresholds against the current table, and
// rebuild at the indicated size — growth escalates (see resizeTo), a shrink
// is best effort (keep the current size when the rebuild cannot place every
// element).  It exits once neither threshold is due, clearing the resizing
// flag so a later trigger can start a fresh goroutine.
func (tt *HashTab[T]) backgroundResize() {
	for {
		tt.lock.Lock()
		newSize, needed := tt.desiredSize()
		if !needed {
			tt.resizing = false
			tt.lock.Unlock()
			return
		}
		if newSize > tt.size {
			tt.resizeTo(newSize)
		} else {
			tt.tryRebuild(newSize)
		}
		tt.lock.Unlock()
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

// Capacity returns the current table size — always a power of two, at least
// minTableSize for a constructed table.  A nil table and the zero value
// report 0.  Right after a threshold is crossed the reported capacity may
// still be the old one — the background resizer has not rebuilt yet.
// Complexity is O(1).
func (tt *HashTab[T]) Capacity() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.size
}

// Saturation returns the load factor: Len divided by Capacity.  It crosses
// the grow threshold (default 0.85) just before the background resizer
// doubles the table and the shrink threshold (default 0.10) just before it
// halves the table.  A nil table and the zero value report 0.
// Complexity is O(1).
func (tt *HashTab[T]) Saturation() float64 {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.saturation()
}

// Search looks for `find` in its four candidate slots and returns the stored
// element equal to it.  If it is not found the zero value of T and false are
// returned.
// Complexity is O(1) average — exactly four slots are examined; O(1) worst
// case for the element comparison itself.
func (tt *HashTab[T]) Search(find T) (rv T, found bool) {
	if tt == nil {
		return // a nil table searches as an empty one
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlSearch(find)
}

// NlSearch is Search without locking; call it only while holding Lock.
// Complexity is O(1) average — exactly four slots are examined.
func (tt *HashTab[T]) NlSearch(find T) (rv T, found bool) {
	if tt.nlIsEmpty() || tt.eq == nil {
		return // empty or zero-value table: not found
	}
	h := tt.hash(find)
	for i := 0; i < numPositions; i++ {
		p := posOf(h, i, tt.mask)
		if s := tt.slots[p]; s.used && s.hash == h && tt.eq(s.data, find) {
			return s.data, true // equal elements hash equal, so the pre-check is safe
		}
	}
	return
}

// Delete an element from the table.  The element equal to `find` is located
// in one of its four candidate slots and the slot is cleared — unlike linear
// probing there are no probe chains to repair.  When the deletion takes the
// saturation below the shrink threshold the background resizer halves the
// table (down to the minimum table size).  Returns true if the element was
// found and removed.
// Complexity is O(1) average; the triggered shrink is O(n) on the background
// goroutine.
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
func (tt *HashTab[T]) NlDelete(find T) (found bool) {
	if tt.nlIsEmpty() || tt.eq == nil {
		return false
	}
	h := tt.hash(find)
	for i := 0; i < numPositions; i++ {
		p := posOf(h, i, tt.mask)
		if s := &tt.slots[p]; s.used && s.hash == h && tt.eq(s.data, find) {
			var zero T
			s.data = zero // release the reference for GC
			s.hash = 0
			s.used = false
			tt.length--
			// Unlike the plain package the shrink runs on the background
			// goroutine.
			if tt.length > 0 && tt.size > minTableSize && tt.saturation() < tt.shrinkAt {
				tt.spawnResize()
			}
			return true
		}
	}
	return false
}

// ApplyFunction is the callback type for Walk.  It is called with the slot
// position and the element stored there.  Returning false stops the walk (the
// same convention as the tree packages; note dll/sll are the opposite).
type ApplyFunction[T any] func(pos int, data T) bool

// Walk calls `fx` for each element in the table, in slot order, until all
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
	for ii := range tt.slots {
		if tt.slots[ii].used {
			if !fx(ii, tt.slots[ii].data) {
				return false
			}
		}
	}
	return
}

// Dump will print out the hash table, including empty slots, to `fo` — the
// element count, table size and saturation on the first line, then one line
// per slot.  The hash values shown are the per-table random-seeded base
// hashes, so with NewHashTab the output varies from process to process; use
// it for debugging, not for golden files.  The read lock is held for the
// whole dump.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	if tt == nil {
		_, _ = fmt.Fprintf(fo, "Elements: 0, table size:0\n")
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	_, _ = fmt.Fprintf(fo, "Elements: %d, table size:%d, saturation:%.4f\n", tt.length, tt.size, tt.saturation())
	for i := range tt.slots {
		if !tt.slots[i].used {
			_, _ = fmt.Fprintf(fo, "slot [%04d] empty\n", i)
			continue
		}
		_, _ = fmt.Fprintf(fo, "slot [%04d] h=%d = %v\n", i, tt.slots[i].hash, tt.slots[i].data)
	}
}
