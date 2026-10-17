/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sharded_hash_ts

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/g_lib"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation): materialization, rendering, deep copy, membership batches
// and predicate bulk delete.  Everything is additive — no existing method
// changed — and every new method is a total function on the read side: a
// nil table and the zero value behave as an empty table, exactly like the
// read operations in sharded_hash.go.  The only panics the new methods can
// surface are the insert-contract panics below, and only when a batch
// would actually store an element (a caller loop over the same items hits
// the identical panics).
//
// Sharding discipline (the package's own rule, from Scan/Truncate): there
// is no global lock, so no whole-table operation is atomic across stripes.
// The batch methods group their items by stripe and take the stripes one
// at a time in stripe-index order — the LockKey multi-key ordering rule —
// so each stripe's share of a batch is atomic under one hold of that
// stripe's lock, concurrent batches cannot deadlock against each other,
// and an interleaved single-key operation never observes a stripe's share
// half-applied.  Only ever one stripe lock is held at a time.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *ShardedHash[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("sharded_hash_ts: %s called on a nil table (a nil table cannot store an element)", caller))
	}
	if tt.eq == nil || tt.hash == nil {
		panic(fmt.Sprintf("sharded_hash_ts: %s called on a table with no equality/hash functions (create the table with NewShardedHash or NewShardedHashFunc)", caller))
	}
}

// stripeIndexOf returns the stripe index a raw hash routes to — the index
// routeHash dereferences.
func (tt *ShardedHash[T]) stripeIndexOf(hh uint64) int {
	return int((hh * fibMult) >> tt.stripeShift)
}

// groupByStripe routes each item to its stripe and returns the items
// grouped per stripe index.  Call it on a non-nil constructed table only.
func (tt *ShardedHash[T]) groupByStripe(items []T) [][]T {
	groups := make([][]T, len(tt.stripes))
	for _, item := range items {
		i := tt.stripeIndexOf(tt.hash(item))
		groups[i] = append(groups[i], item)
	}
	return groups
}

// Contains reports whether the table holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(1) average, O(n) worst case.
func (tt *ShardedHash[T]) Contains(find T) bool {
	_, found := tt.Search(find)
	return found
}

// Slice returns a fresh slice holding every element of the table.  The
// elements are copied stripe by stripe, each stripe under one hold of its
// read lock — the same per-stripe point-in-time view All and Values
// snapshot: each stripe's share is consistent, stripes are not mutually
// consistent (inherent to sharding; concurrent writers can interleave
// between stripes).  The order is stripe order, bucket order within each
// stripe; both depend on the hash seed and per-stripe growth history, so
// sort the result when a fixed order is needed.  The slice is a copy:
// mutating it does not affect the table.  A nil table yields nil; an
// empty table yields an empty slice.
// Complexity is O(n).
func (tt *ShardedHash[T]) Slice() []T {
	if tt == nil {
		return nil
	}
	items := make([]T, 0, tt.Len())
	for _, s := range tt.stripes {
		s.lock.RLock()
		for _, head := range s.tab.heads {
			for n := head; n != nil; n = n.next {
				items = append(items, n.data)
			}
		}
		s.lock.RUnlock()
	}
	return items
}

// String renders the table as its elements formatted with fmt.Sprint,
// the formatted strings sorted ascending and joined with single spaces
// inside square brackets: "[a b c]".  Sorting the formatted strings —
// not the elements — is what makes the output deterministic: stripe and
// bucket order depend on the per-table random hash seed and differ from
// run to run, but two tables holding the same elements always render
// the same string.  A nil or empty table renders as "[]".  For a custom
// per-element rendering use StringFunc.
// Complexity is O(n log n).
func (tt *ShardedHash[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are formatted, the strings sorted ascending and
// joined inside square brackets, so the output is deterministic whatever
// order the stripes happen to be in.  The elements are snapshotted (per
// stripe) and the format function runs with no stripe lock held, so it
// may safely call back into the table.  A nil format function renders as
// "[]".
// Complexity is O(n log n).
func (tt *ShardedHash[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	return g_lib.FormatSorted(tt.Slice(), format)
}

// Equal reports whether tt and other hold the same set of elements: same
// length and every element of tt found in other by Search.  Membership
// is compared, never iteration order.  Two empty tables are equal, a nil
// table equals an empty table, and Equal is symmetric on any pair that
// satisfies its precondition.
//
// At most one stripe lock (of one table) is held at any moment, so Equal
// trivially cannot deadlock — not even when two goroutines compare the
// same pair of tables in opposite directions.
//
// Precondition: both tables must use consistent equality and hash
// functions (a nil table trivially does).  Functions are not comparable,
// so this cannot be checked — two tables built by the same constructor
// over the same element type always satisfy it.
// Complexity is O(n) average.
func (tt *ShardedHash[T]) Equal(other *ShardedHash[T]) bool {
	if tt.Len() != other.Len() {
		return false
	}
	if tt.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, v := range tt.Slice() {
		if !other.Contains(v) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the table: the same stripe count, stripe
// routing, initial capacity and load factor, the same equality/hash
// functions, and a fresh stripe for each source stripe whose chained
// table is copied bucket by bucket and node by node — same bucket count,
// same chain order, same growth state.  No node is shared with the
// receiver, so mutating either table afterwards never affects the other.
// Each source stripe is read-locked only while it is copied.  A nil table
// clones to nil; a zero-value table clones to a zero-value table.
// Complexity is O(n).
func (tt *ShardedHash[T]) Clone() *ShardedHash[T] {
	if tt == nil {
		return nil
	}
	dup := &ShardedHash[T]{
		eq:           tt.eq,
		hash:         tt.hash,
		stripes:      make([]*stripe[T], len(tt.stripes)),
		stripeShift:  tt.stripeShift,
		initialHeads: tt.initialHeads,
		loadFac:      tt.loadFac,
	}
	dup.count.Store(tt.count.Load())
	for i, s := range tt.stripes {
		dupStripes := &stripe[T]{}
		s.lock.RLock()
		tab := &stripeTab[T]{
			heads:     make([]*node[T], len(s.tab.heads)),
			live:      s.tab.live,
			loadFac:   s.tab.loadFac,
			threshold: s.tab.threshold,
		}
		for b, head := range s.tab.heads {
			var tail *node[T]
			for n := head; n != nil; n = n.next {
				copyNode := &node[T]{data: n.data, h: n.h}
				if tail == nil {
					tab.heads[b] = copyNode
				} else {
					tail.next = copyNode
				}
				tail = copyNode
			}
		}
		s.lock.RUnlock()
		dupStripes.tab = tab
		dup.stripes[i] = dupStripes
	}
	return dup
}

// ContainsSlice reports whether every element of items is present in the
// table (an empty items slice is vacuously contained).  The items are
// grouped by stripe and each stripe's share is checked under one hold of
// that stripe's read lock — per-stripe consistent, not atomic across
// stripes (the sharding rule above).  For "at least one present" use
// ContainsAny.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *ShardedHash[T]) ContainsSlice(items []T) bool {
	if len(items) == 0 {
		return true // an empty batch is vacuously contained
	}
	if tt == nil || tt.eq == nil {
		return false // a nil/zero-value table contains nothing
	}
	for i, group := range tt.groupByStripe(items) {
		if len(group) == 0 {
			continue
		}
		s := tt.stripes[i]
		s.lock.RLock()
		for _, item := range group {
			hh := tt.hash(item)
			if s.tab.find(hh, item, tt.eq) == nil {
				s.lock.RUnlock()
				return false
			}
		}
		s.lock.RUnlock()
	}
	return true
}

// ContainsAny reports whether at least one element of items is present
// in the table, checked per stripe under one hold of that stripe's read
// lock.  An empty items slice contains nothing.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *ShardedHash[T]) ContainsAny(items []T) bool {
	if tt == nil || tt.eq == nil || len(items) == 0 {
		return false
	}
	for i, group := range tt.groupByStripe(items) {
		if len(group) == 0 {
			continue
		}
		s := tt.stripes[i]
		s.lock.RLock()
		for _, item := range group {
			hh := tt.hash(item)
			if s.tab.find(hh, item, tt.eq) != nil {
				s.lock.RUnlock()
				return true
			}
		}
		s.lock.RUnlock()
	}
	return false
}

// InsertSlice inserts each element of items and reports whether at least
// one element was new (an equal element is replaced and reports false,
// exactly as a caller loop of Insert does).  The items are grouped by
// stripe and inserted stripe by stripe in stripe-index order, each
// stripe's whole share under one hold of that stripe's write lock — so a
// stripe's share is atomic and concurrent batches cannot deadlock, but
// the whole-table batch is NOT atomic across stripes (an interleaved
// reader may observe some stripes' shares applied and others not — the
// same caveat Scan and Truncate document).
//
// It follows the insert contract: a batch that would store an element
// into a nil table or a zero-value table panics with the standard
// insert-family message; an empty batch stores nothing and is tolerated
// everywhere.
// Complexity is O(k) average amortized, O(n) worst case per element, k = len(items).
func (tt *ShardedHash[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	tt.insertContractCheck("InsertSlice")
	return tt.insertGrouped(items)
}

// insertGrouped is the shared grouped insert loop of the batch inserters;
// the caller has already applied the insert contract check.
func (tt *ShardedHash[T]) insertGrouped(items []T) bool {
	changed := false
	for i, group := range tt.groupByStripe(items) {
		if len(group) == 0 {
			continue
		}
		s := tt.stripes[i]
		s.lock.Lock()
		for _, item := range group {
			if s.tab.insert(tt.hash(item), item, tt.eq) {
				tt.count.Add(1) // under the stripe lock, like Insert
				changed = true
			}
		}
		s.lock.Unlock()
	}
	return changed
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between.  The iterator is drained (and any table snapshot it
// comes from taken) before the first stripe is locked, so feeding a
// table its own Values() is safe and no stripe lock is held across the
// caller's iteration.  It reports whether at least one element was new
// and follows the same insert contract and sharding discipline as
// InsertSlice.  A nil iterator inserts nothing and returns false.
// Complexity is O(k) average amortized, k = number of yielded elements.
func (tt *ShardedHash[T]) InsertIter(seq iter.Seq[T]) bool {
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
	tt.insertContractCheck("InsertIter")
	return tt.insertGrouped(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present, with the same group-by-stripe discipline as
// InsertSlice (each stripe's share atomic under that stripe's write
// lock; the whole-table batch not atomic across stripes).  Deleting is
// tolerated on a nil table (returns false), like Delete.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *ShardedHash[T]) DeleteSlice(items []T) bool {
	if tt == nil || tt.eq == nil || len(items) == 0 {
		return false
	}
	changed := false
	for i, group := range tt.groupByStripe(items) {
		if len(group) == 0 {
			continue
		}
		s := tt.stripes[i]
		s.lock.Lock()
		for _, item := range group {
			if s.tab.remove(tt.hash(item), item, tt.eq) {
				tt.count.Add(-1) // under the stripe lock, like Delete
				changed = true
			}
		}
		s.lock.Unlock()
	}
	return changed
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The elements are snapshotted (per
// stripe, as Slice does), fx runs on the snapshot with no stripe lock
// held (so fx may safely call back into the table), and the matched
// elements are then deleted through DeleteSlice's group-by-stripe
// discipline — collect-then-delete, the table is never modified while it
// is being walked.  A nil table, an empty table or a nil fx deletes
// nothing and returns false.
// Complexity is O(n) average.
func (tt *ShardedHash[T]) DeleteFunc(fx func(T) bool) bool {
	if tt == nil || fx == nil {
		return false
	}
	doomed := make([]T, 0)
	for _, v := range tt.Slice() { // snapshot, per stripe
		if fx(v) { // caller predicate, no stripe lock held
			doomed = append(doomed, v)
		}
	}
	if len(doomed) == 0 {
		return false
	}
	return tt.DeleteSlice(doomed)
}
