/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package sharded_hash_ts implements a thread-safe generic hash table that
// shards its keyspace across a fixed power-of-two number of independently
// locked stripes.  One logical table, N internal tables, N sync.RWMutex —
// concurrent operations on keys that hash to different stripes never contend
// for the same lock, and Len is a lock-free atomic, so there is no global
// lock anywhere on the hot path.  It replaces the N independent tables plus
// caller-side routing and iteration glue that a shared keyspace otherwise
// needs (the Ultima keyspace is its reason for existing — see
// note/03-sharded-hash-table.md).
//
// Each stripe is a chained hash table with a power-of-two bucket-heads array
// indexed by hash & (heads-1) that doubles when the load factor passes a
// configurable threshold (default 0.75).  This is deliberately NOT the
// hash_grow/cuckoo open-addressing machinery, and the reason is the Scan
// contract: Scan promises, like Redis SCAN, that a full iteration returns
// every element present for the entire scan at least once.  That is only
// provable when an element never changes buckets except by the deterministic
// split of a doubling (bucket v becomes v or v+oldSize, decided by one hash
// bit).  hash_grow's backward-shift deletion moves elements between buckets
// on every delete, and its growth re-probes each element from its home slot
// depending on occupancy — either can move an element into an
// already-scanned bucket and strand it.  Chained buckets move nothing:
// Insert pushes a node at the chain head, Delete unlinks a node, and growth
// re-links every existing node at h & newMask.  The per-op speed of open
// addressing is traded for the scan guarantee; bench_test.go measures the
// trade against hash_grow_ts and cuckoo_ts.
//
// The rehash-safe scan cursor is Redis's reverse-binary iteration
// (note/redis/src/dict.c, dictScan): within a stripe the cursor visits
// buckets in reverse-binary order, the one ordering under which a mid-scan
// doubling still covers every unvisited bucket.  The cursor packs the stripe
// index in the high 24 bits and the reverse-binary bucket cursor in the low
// 40 bits — one cursor space across the whole table.
//
// Element data is never boxed into an interface and never unboxed with a
// type assertion.  Tables of types that can be compared with == (the builtin
// comparable constraint) are created with NewShardedHash, which hashes with
// the stdlib hash/maphash — one random seed for the whole table (both the
// stripe routing and the bucket placement use it), equal values always hash
// equal, and no method has to be implemented.  Tables of any other type — or
// with field-based identity — are created with NewShardedHashFunc, which
// takes a caller supplied equality function and hash function; the two must
// agree: whenever eq(a, b) is true, hash(a) and hash(b) must be equal.  A
// two-field struct with eq/hash reading only the key field is the map
// pattern: Insert replaces, Search returns the stored (key, value) pair.
//
// Elements are stored and returned by value (T, not *T).
//
// Operations:
//
//	Insert — add a new element, replacing any existing equal element.	O(1) average per stripe
//	Delete — delete the element equal to `find`, if present.	O(1) average per stripe
//	Search — return the stored element equal to `find`.		O(1) average per stripe
//	IsEmpty / Len / Length — lock-free via an atomic counter.	O(1)
//	Truncate — remove all elements from every stripe.			O(n)
//	Scan — up to `count` elements plus the next cursor.			O(count) amortized, one stripe lock at a time
//	LockKey + Nl* — atomic read-modify-write on one key's stripe.	O(1) to lock
//	StripeCount / StripeLen — per-stripe load for metrics.		O(1) / O(1)
//	Walk / All / Values — iterate stripe by stripe.				O(n)
//	Dump — per-stripe summary for debugging.					O(stripes)
//
// A nil *ShardedHash and the zero value both behave as an empty table for
// every read: searches report not-found, Delete returns false, Scan returns
// no elements and cursor 0, and the iterators visit nothing.
//
// The package panics in exactly four situations, all programmer errors that
// cannot be handled where they occur — each message names the fix:
//
//	NewShardedHashFunc with a nil equality or hash function — caught at construction.
//	NewShardedHash/NewShardedHashFunc with stripes < 1 — a stripe count is not a tuning hint.
//	Insert on a nil table — a nil table cannot store an element.
//	Insert on a zero-value table — no equality/hash functions; the message names the constructors.
//
// There is no plain (non-_ts) twin: the striping exists only for concurrent
// use.  Single-goroutine callers want hash_grow or hash_tab.
package sharded_hash_ts

import (
	"fmt"
	"hash/maphash"
	"io"
	"math/bits"
	"sync"
	"sync/atomic"
)

// Cursor packing: the low slotBits bits of a Scan cursor are the
// reverse-binary bucket cursor within a stripe, the next stripeBits bits are
// the stripe index.  Both are sized far beyond practical use (2^40 buckets
// per stripe, 2^24 stripes) and enforced at construction/growth.
const (
	slotBits   = 40
	slotMask   = uint64(1)<<slotBits - 1
	maxStripes = 1 << 24 // the stripe field of a cursor
	maxHeads   = uint64(1) << slotBits
)

// fibMult is the golden-ratio (Fibonacci hashing) multiplier: stripe routing
// takes the top bits of hash*fibMult, which spreads any caller hash — even
// one with poor high bits, like plain FNV — evenly over the stripes and
// decorrelates the routing bits from the low bits the bucket mask uses.
const fibMult = uint64(0x9E3779B97F4A7C15)

// defaultLoadFactor is the per-stripe growth threshold used when the
// constructor's loadFactor is <= 0 or NaN.  0.75 is the classic chained-table
// setting; chains stay short while memory overhead stays modest.
const defaultLoadFactor = 0.75

// defaultInitialHeads is the per-stripe bucket count used when the
// constructor's initialCapacity is < 8 (including 0).
const defaultInitialHeads = 16

// node is one element of a stripe's chain.  The raw hash is cached so growth
// re-links nodes without re-hashing, and so chain walks can compare hashes
// before calling the equality function.  A hash of exactly 0 is legal here —
// chain membership needs no empty-slot marker (unlike hash_grow).
type node[T any] struct {
	data T
	h    uint64
	next *node[T]
}

// stripeTab is the lock-free chained table owned by one stripe.  heads is
// always a power of two; heads grow by doubling under the stripe's write
// lock, re-linking every node at h & newMask (the deterministic split Scan's
// guarantee depends on — nodes are never re-hashed and never move except by
// that split).
type stripeTab[T any] struct {
	heads     []*node[T]
	live      int     // number of nodes chained in heads
	loadFac   float64 // grow when live passes loadFac * len(heads)
	threshold int     // cached int(loadFac * len(heads))
}

// newStripeTab builds a fresh table with `nHeads` (already normalized to a
// power of two) buckets.
func newStripeTab[T any](nHeads int, loadFactor float64) *stripeTab[T] {
	tb := &stripeTab[T]{heads: make([]*node[T], nHeads), loadFac: loadFactor}
	tb.threshold = tb.computeThreshold()
	return tb
}

// computeThreshold returns the live count at which the table doubles.
func (tb *stripeTab[T]) computeThreshold() int {
	t := int(tb.loadFac * float64(len(tb.heads)))
	if t < 1 {
		t = 1
	}
	return t
}

// find returns the node equal to `find` in the chain at h & mask, or nil.
// The raw hash is compared before eq — equal elements always have equal
// hashes (the constructor contract), so a hash mismatch rules out equality
// without calling eq.
func (tb *stripeTab[T]) find(hh uint64, find T, eq func(a, b T) bool) *node[T] {
	for n := tb.heads[hh&uint64(len(tb.heads)-1)]; n != nil; n = n.next {
		if n.h == hh && eq(n.data, find) {
			return n
		}
	}
	return nil
}

// insert adds `item` with raw hash `hh`, replacing the data of an equal
// element if one is chained (returning false) or chaining a new node at the
// bucket head (returning true).  Doubling the table when the threshold
// passes is amortized O(1).
func (tb *stripeTab[T]) insert(hh uint64, item T, eq func(a, b T) bool) (added bool) {
	b := hh & uint64(len(tb.heads)-1)
	for n := tb.heads[b]; n != nil; n = n.next {
		if n.h == hh && eq(n.data, item) {
			n.data = item // equal element: replace in place, no structural change
			return false
		}
	}
	tb.heads[b] = &node[T]{data: item, h: hh, next: tb.heads[b]}
	tb.live++
	if tb.live > tb.threshold {
		tb.grow()
	}
	return true
}

// remove unlinks the node equal to `find` from the chain at h & mask.
// Unlinking moves no other node — the property Scan's guarantee needs.
func (tb *stripeTab[T]) remove(hh uint64, find T, eq func(a, b T) bool) (found bool) {
	b := hh & uint64(len(tb.heads)-1)
	var prev *node[T]
	for n := tb.heads[b]; n != nil; prev, n = n, n.next {
		if n.h == hh && eq(n.data, find) {
			if prev == nil {
				tb.heads[b] = n.next
			} else {
				prev.next = n.next
			}
			var zero T
			n.data = zero // release the reference for GC
			n.next = nil
			tb.live--
			return true
		}
	}
	return false
}

// grow doubles heads and re-links every existing node at h & newMask.  A
// node from old bucket v lands in exactly v or v+len(oldHeads), decided by
// the next hash bit alone — never re-hashed, never moved by occupancy.  That
// determinism is what makes a mid-scan doubling safe (see Scan).
func (tb *stripeTab[T]) grow() {
	if uint64(len(tb.heads)) >= maxHeads {
		panic("sharded_hash_ts: stripe grew past 2^40 buckets — beyond the largest cursor-addressable table")
	}
	old := tb.heads
	tb.heads = make([]*node[T], 2*len(old))
	m := uint64(len(tb.heads) - 1)
	for _, head := range old {
		for n := head; n != nil; {
			next := n.next
			n.next = tb.heads[n.h&m]
			tb.heads[n.h&m] = n
			n = next
		}
	}
	tb.threshold = tb.computeThreshold()
}

// -------------------------------------------------------------------------------------------------------

// ShardedHash is a generic, thread-safe, striped hash table.  Use
// NewShardedHash for element types that support ==, or NewShardedHashFunc
// for a caller supplied equality and hash function.  The zero value is an
// empty read-only table.  The stripe count is fixed for the life of the
// table — elements never migrate stripes.
type ShardedHash[T any] struct {
	// eq and hash are set by the constructors and are the only things that
	// know how to compare and hash T.  They must agree: equal elements must
	// have equal hashes.
	eq   func(a, b T) bool
	hash func(a T) uint64

	stripes      []*stripe[T]
	stripeShift  uint    // route = (hash * fibMult) >> stripeShift selects the stripe
	initialHeads int     // per-stripe bucket count a fresh/emptied stripe starts with
	loadFac      float64 // per-stripe growth threshold
	count        atomic.Int64
}

// stripe is one independently locked shard: one mutex and one chained table.
// All public methods touch exactly one stripe per key, so no code path ever
// holds two stripe locks at once.
type stripe[T any] struct {
	lock sync.RWMutex
	tab  *stripeTab[T]
}

// NewShardedHash creates a sharded table with `stripes` stripes (rounded up
// to a power of two, at least 1), `initialCapacity` buckets per stripe
// (rounded up to a power of two, at least 8; <= 0 selects 16), and the
// per-stripe growth threshold `loadFactor` (<= 0 or NaN selects 0.75).
// Elements are compared with the == operator and hashed with the stdlib
// hash/maphash using one random seed for the whole table — no method has to
// be implemented on T, and no element is ever boxed into an interface.
// Complexity is O(stripes + stripes*initialCapacity) for the allocation.
func NewShardedHash[T comparable](stripes, initialCapacity int, loadFactor float64) *ShardedHash[T] {
	var seed = maphash.MakeSeed()
	return newShardedHash(
		stripes, initialCapacity, loadFactor,
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
		"NewShardedHash",
	)
}

// NewShardedHashFunc creates a sharded table with a caller supplied equality
// function and hash function, `stripes` stripes (rounded up to a power of
// two), `initialCapacity` buckets per stripe (rounded up to a power of two),
// and the per-stripe growth threshold `loadFactor` (<= 0 or NaN selects
// 0.75).  The two functions must agree: whenever eq(a, b) is true, hash(a)
// and hash(b) must be equal, otherwise Search and Delete can look in the
// wrong stripe or the wrong bucket.
// Complexity is O(stripes + stripes*initialCapacity) for the allocation.
func NewShardedHashFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, stripes, initialCapacity int, loadFactor float64) *ShardedHash[T] {
	return newShardedHash(stripes, initialCapacity, loadFactor, eq, hash, "NewShardedHashFunc")
}

// newShardedHash is the shared constructor body; `caller` names the public
// constructor in panic messages.
func newShardedHash[T any](stripes, initialCapacity int, loadFactor float64, eq func(a, b T) bool, hash func(a T) uint64, caller string) *ShardedHash[T] {
	if eq == nil {
		panic(fmt.Sprintf("sharded_hash_ts: %s called with a nil equality function", caller))
	}
	if hash == nil {
		panic(fmt.Sprintf("sharded_hash_ts: %s called with a nil hash function", caller))
	}
	if stripes < 1 {
		panic(fmt.Sprintf("sharded_hash_ts: %s called with stripes = %d, the stripe count must be at least 1", caller, stripes))
	}
	nStripes := roundUpPow2(stripes)
	if nStripes > maxStripes {
		panic(fmt.Sprintf("sharded_hash_ts: %s called with stripes = %d, above the cursor-addressable maximum of %d", caller, stripes, maxStripes))
	}
	if !(loadFactor > 0) { // also catches NaN
		loadFactor = defaultLoadFactor
	}
	if initialCapacity < defaultInitialHeads {
		initialCapacity = defaultInitialHeads
	}
	initialCapacity = roundUpPow2(initialCapacity)

	sh := &ShardedHash[T]{
		eq:           eq,
		hash:         hash,
		stripes:      make([]*stripe[T], nStripes),
		stripeShift:  uint(64 - (bits.Len(uint(nStripes)) - 1)),
		initialHeads: initialCapacity,
		loadFac:      loadFactor,
	}
	for i := range sh.stripes {
		sh.stripes[i] = &stripe[T]{tab: newStripeTab[T](initialCapacity, loadFactor)}
	}
	return sh
}

// roundUpPow2 returns x rounded up to the next power of two (x itself when
// already a power of two; 1 stays 1).
func roundUpPow2(x int) int {
	if x <= 1 {
		return 1
	}
	return 1 << uint(bits.Len(uint(x-1)))
}

// routeHash maps a raw element hash to its stripe.  The top bits of
// hash*fibMult (Fibonacci hashing) spread the keyspace evenly over the
// stripes no matter how the caller's hash is distributed, and are
// independent of the low bits the bucket mask uses.
func (tt *ShardedHash[T]) routeHash(hh uint64) *stripe[T] {
	return tt.stripes[(hh*fibMult)>>tt.stripeShift]
}

// Insert will add a new item to the table.  If it is an equal duplicate of
// an existing item the new item replaces the stored one and false is
// returned; true is returned when a new element was added.  Only the one
// stripe the item hashes to is locked.
// Complexity is O(1) average, O(n) worst case; growth is amortized O(1).
func (tt *ShardedHash[T]) Insert(item T) bool {
	if tt == nil {
		panic("sharded_hash_ts: Insert called on a nil table")
	}
	if tt.eq == nil || tt.hash == nil {
		panic("sharded_hash_ts: Insert called on a table with no equality/hash functions (create the table with NewShardedHash or NewShardedHashFunc)")
	}
	hh := tt.hash(item)
	s := tt.routeHash(hh)
	s.lock.Lock()
	added := s.tab.insert(hh, item, tt.eq)
	if added {
		tt.count.Add(1) // under the stripe lock, so Truncate's subtraction can never interleave with the add
	}
	s.lock.Unlock()
	return added
}

// NlInsert is Insert without locking; call it only while holding the stripe
// lock for `item` (the one LockKey(item) takes).  It panics on a table with
// no equality/hash functions (a zero-value table), naming the constructors.
func (tt *ShardedHash[T]) NlInsert(item T) bool {
	if tt.eq == nil || tt.hash == nil {
		panic("sharded_hash_ts: Insert called on a table with no equality/hash functions (create the table with NewShardedHash or NewShardedHashFunc)")
	}
	hh := tt.hash(item)
	s := tt.routeHash(hh)
	added := s.tab.insert(hh, item, tt.eq)
	if added {
		tt.count.Add(1)
	}
	return added
}

// Search will return the stored element equal to `find`.  If it is not found
// the zero value of T and false are returned.  Only the one stripe the probe
// hashes to is read-locked.
// Complexity is O(1) average, O(n) worst case.
func (tt *ShardedHash[T]) Search(find T) (rv T, found bool) {
	if tt == nil || tt.eq == nil || tt.hash == nil {
		return // nil table, zero value: not found
	}
	hh := tt.hash(find)
	s := tt.routeHash(hh)
	s.lock.RLock()
	if n := s.tab.find(hh, find, tt.eq); n != nil {
		rv, found = n.data, true
	}
	s.lock.RUnlock()
	return
}

// NlSearch is Search without locking; call it only while holding the stripe
// lock for `find`.
// Complexity is O(1) average, O(n) worst case.
func (tt *ShardedHash[T]) NlSearch(find T) (rv T, found bool) {
	if tt.eq == nil || tt.hash == nil {
		return
	}
	hh := tt.hash(find)
	s := tt.routeHash(hh)
	if n := s.tab.find(hh, find, tt.eq); n != nil {
		rv, found = n.data, true
	}
	return
}

// Delete an element from the table.  Returns true if the element was found
// and removed.  Only the one stripe the probe hashes to is locked; the
// search and the unlink are atomic under it.
// Complexity is O(1) average, O(n) worst case.
func (tt *ShardedHash[T]) Delete(find T) (found bool) {
	if tt == nil || tt.eq == nil || tt.hash == nil {
		return false
	}
	hh := tt.hash(find)
	s := tt.routeHash(hh)
	s.lock.Lock()
	found = s.tab.remove(hh, find, tt.eq)
	if found {
		tt.count.Add(-1)
	}
	s.lock.Unlock()
	return
}

// NlDelete is Delete without locking; call it only while holding the stripe
// lock for `find`.
// Complexity is O(1) average, O(n) worst case.
func (tt *ShardedHash[T]) NlDelete(find T) (found bool) {
	if tt.eq == nil || tt.hash == nil {
		return false
	}
	hh := tt.hash(find)
	s := tt.routeHash(hh)
	found = s.tab.remove(hh, find, tt.eq)
	if found {
		tt.count.Add(-1)
	}
	return
}

// IsEmpty will return true if the table is empty.
// Complexity is O(1) — a lock-free atomic load.
func (tt *ShardedHash[T]) IsEmpty() bool {
	return tt == nil || tt.count.Load() == 0
}

// NlIsEmpty is IsEmpty without locking (it never locks — the count is
// atomic); it is provided for compound-section symmetry with the other _ts
// packages.
func (tt *ShardedHash[T]) NlIsEmpty() bool {
	return tt == nil || tt.count.Load() == 0
}

// Len returns the number of elements in the table.
// Complexity is O(1) — a lock-free atomic load, exact between operations.
func (tt *ShardedHash[T]) Len() int {
	if tt == nil {
		return 0
	}
	return int(tt.count.Load())
}

// Length returns the number of elements in the table.
// Complexity is O(1).
func (tt *ShardedHash[T]) Length() int {
	return tt.Len()
}

// NlLen is Len without locking (it never locks — the count is atomic); safe
// inside a LockKey section, for any key.
func (tt *ShardedHash[T]) NlLen() int {
	if tt == nil {
		return 0
	}
	return int(tt.count.Load())
}

// Truncate removes all data from every stripe, re-creating each stripe's
// table at the configured initial capacity.  Stripes are cleared one at a
// time — concurrent operations on untouched stripes proceed while a stripe
// is being cleared.  A Scan in flight stays valid: the guarantee only covers
// elements present for the entire scan, and Truncate removes them.
// Complexity is O(n + stripes).
func (tt *ShardedHash[T]) Truncate() {
	if tt == nil {
		return
	}
	for _, s := range tt.stripes {
		s.lock.Lock()
		if n := s.tab.live; n > 0 {
			s.tab = newStripeTab[T](tt.initialHeads, tt.loadFac)
			tt.count.Add(int64(-n))
		}
		s.lock.Unlock()
	}
}

// LockKey takes the write lock of the stripe that `key` hashes to and
// returns the function that releases it.  Between LockKey and the returned
// unlock, the Nl-prefixed methods for keys of the SAME stripe run atomically
// — a search-then-replace or search-then-delete no other goroutine can
// interleave with:
//
//	unlock := h.LockKey(key)
//
//	if v, found := h.NlSearch(key); found {
//		use(v)
//		h.NlDelete(key)
//	}
//
// unlock()
//
// Calling a locking public method (Insert, Delete, Truncate, Scan, ...) on
// any key of the same stripe while the lock is held deadlocks, and the Nl
// methods are race-free only for keys that route to the held stripe — a key
// of another stripe is not protected by the held lock.  Clients that lock
// multiple keys at once MUST take them in stripe-index order (the order
// StripeCount numbers them in) or they can deadlock against another client
// holding them in the opposite order.  Locking a nil or zero-value table
// returns a no-op unlock.
func (tt *ShardedHash[T]) LockKey(key T) (unlock func()) {
	if tt == nil || tt.eq == nil || tt.hash == nil || len(tt.stripes) == 0 {
		return func() {} // a nil/zero-value table has no stripe to lock
	}
	s := tt.routeHash(tt.hash(key))
	s.lock.Lock()
	return s.lock.Unlock
}

// StripeCount returns the number of stripes (a power of two, fixed at
// construction; 0 for a nil/zero-value table).
// Complexity is O(1).
func (tt *ShardedHash[T]) StripeCount() int {
	if tt == nil {
		return 0
	}
	return len(tt.stripes)
}

// StripeLen returns the number of elements currently in stripe i (0 for an
// out-of-range index — the heap convention, not a panic).  Per-stripe load
// for metrics and rebalancing decisions.
// Complexity is O(1) — one stripe read lock.
func (tt *ShardedHash[T]) StripeLen(i int) int {
	if tt == nil || i < 0 || i >= len(tt.stripes) {
		return 0
	}
	s := tt.stripes[i]
	s.lock.RLock()
	n := s.tab.live
	s.lock.RUnlock()
	return n
}

// ApplyFunction is the callback type for Walk.  It is called with a running
// position (0 for the first element visited, counting up) and the element
// stored there.  Returning false stops the walk (the tree-package
// convention; note dll/sll are the opposite).
type ApplyFunction[T any] func(pos int, data T) bool

// Walk calls `fx` for each element in the table, stripe by stripe in stripe
// order and bucket order within each stripe, until all elements have been
// visited or `fx` returns false.  It returns true if the walk ran to
// completion.
//
// Each stripe's read lock is held only while that stripe is walked.  fx must
// not call methods on the same table — a write to the stripe being walked
// deadlocks (use All or Values, which iterate a snapshot, when the loop body
// needs to touch the table).
// Complexity is O(n).
func (tt *ShardedHash[T]) Walk(fx ApplyFunction[T]) (b bool) {
	b = true
	if tt == nil {
		return
	}
	pos := 0
	for _, s := range tt.stripes {
		s.lock.RLock()
		for _, head := range s.tab.heads {
			for n := head; n != nil; n = n.next {
				if !fx(pos, n.data) {
					s.lock.RUnlock()
					return false
				}
				pos++
			}
		}
		s.lock.RUnlock()
	}
	return
}

// Dump writes a per-stripe summary of the table to `fo` — the stripe count
// and element total on the first line, then one line per stripe with its
// element count and bucket count.  Unlike hash_grow's per-bucket dump (a
// sharded table's buckets can number in the millions), no hash values or
// elements are printed; the per-stripe live/heads pair is the debugging and
// metrics view.  Each stripe is read-locked only while its line is computed.
// Complexity is O(stripes).
func (tt *ShardedHash[T]) Dump(fo io.Writer) {
	if tt == nil {
		_, _ = fmt.Fprintf(fo, "sharded_hash_ts: nil table\n")
		return
	}
	_, _ = fmt.Fprintf(fo, "sharded_hash_ts: %d stripes, %d elements\n", len(tt.stripes), tt.count.Load())
	for i, s := range tt.stripes {
		s.lock.RLock()
		live, heads := s.tab.live, len(s.tab.heads)
		s.lock.RUnlock()
		_, _ = fmt.Fprintf(fo, "stripe [%06d] live=%d heads=%d\n", i, live, heads)
	}
}
