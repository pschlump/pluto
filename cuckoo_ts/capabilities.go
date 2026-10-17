/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo_ts

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/g_lib"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation): materialization, rendering, deep copy, membership batches
// and predicate bulk delete.  Everything is additive and mirrors cuckoo's
// capabilities.go method for method.  Lock discipline: a batch mutator
// takes the write lock once for the whole batch (a batch is atomic — an
// interleaved reader never sees part of one), and the
// compute-after-snapshot methods (Slice/String/Equal/DeleteFunc's match
// pass) snapshot under the read lock and run unlocked afterwards, so a
// caller format or predicate function may safely call back into the
// table.  No new Nl methods: the new methods are self-locking, not
// compounds.
//
// The background resizer interleaves safely by construction: it needs the
// write lock for each rebuild, so it runs between — never during — a
// locked batch.  A batch whose inserts cross the grow threshold (or whose
// deletes cross the shrink threshold) leaves the resizer goroutine to do
// its rebuilds after the batch's lock is released.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *HashTab[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("cuckoo_ts: %s called on a nil table (a nil table cannot store an element)", caller))
	}
	if tt.eq == nil || tt.hash == nil {
		panic(fmt.Sprintf("cuckoo_ts: %s called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)", caller))
	}
}

// Contains reports whether the table holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Contains(find T) bool {
	_, found := tt.Search(find)
	return found
}

// Slice returns a fresh slice holding every element of the table.  The
// elements are copied under the read lock (the slot array is stable
// under it — rebuilds take the write lock), and the result is a plain
// value slice: mutating it does not affect the table.  The order is slot
// order — the order All and Values yield — which depends on the
// per-table hash seed and the table's resize history, and is therefore
// unspecified; sort the result when a fixed order is needed.  A nil
// table yields nil; an empty table yields an empty slice.
// Complexity is O(n).
func (tt *HashTab[T]) Slice() []T {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	items := make([]T, 0, tt.length)
	for i := range tt.slots {
		if tt.slots[i].used {
			items = append(items, tt.slots[i].data)
		}
	}
	tt.lock.RUnlock()
	return items
}

// String renders the table as its elements formatted with fmt.Sprint,
// the formatted strings sorted ascending and joined with single spaces
// inside square brackets: "[a b c]".  Sorting the formatted strings —
// not the elements — is what makes the output deterministic: slot order
// depends on the per-table random hash seed and differs from run to
// run, but two tables holding the same elements always render the same
// string.  The elements are snapshotted under the read lock and the
// formatting runs without the lock.  A nil or empty table renders as
// "[]".  For a custom per-element rendering use StringFunc.
// Complexity is O(n log n).
func (tt *HashTab[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are formatted, the strings sorted ascending and
// joined inside square brackets, so the output is deterministic whatever
// order the slots happen to be in.  The elements are snapshotted under
// the read lock and the format function runs without the lock, so it may
// safely call back into the table.  A nil format function renders as
// "[]".
// Complexity is O(n log n).
func (tt *HashTab[T]) StringFunc(format func(T) string) string {
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
// Each table is touched under its own lock only, never two locks at
// once (the avl_tree_ts set-operation discipline), so Equal never
// deadlocks — not even when two goroutines compare the same pair of
// tables in opposite directions.
//
// Precondition: both tables must use consistent equality and hash
// functions (a nil table trivially does).  Functions are not comparable,
// so this cannot be checked — two tables built by the same constructor
// over the same element type always satisfy it.
// Complexity is O(n) average.
func (tt *HashTab[T]) Equal(other *HashTab[T]) bool {
	if tt.Len() != other.Len() {
		return false
	}
	if tt.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, v := range tt.Slice() { // tt's lock, released before other is touched
		if !other.Contains(v) { // other's lock, one element at a time
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the table: a fresh slot array with every
// occupied slot's element and hash copied by value, the same size,
// thresholds and resize counters, and the same equality/hash functions.
// Elements are stored by value, so no backing memory is shared with the
// receiver — mutating either table afterwards never affects the other.
// The clone does not inherit the receiver's in-flight background resize
// (none runs on it); its length and size are copied consistently under
// one hold of the read lock, and its own later mutations spawn its own
// resizer when a threshold is crossed.  The read lock is held for the
// whole copy, so an in-flight resizer rebuild waits behind it.  A nil
// table clones to nil; a zero-value table clones to a zero-value table.
// Complexity is O(n).
func (tt *HashTab[T]) Clone() *HashTab[T] {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	dup := &HashTab[T]{
		size:       tt.size,
		mask:       tt.mask,
		length:     tt.length,
		growAt:     tt.growAt,
		shrinkAt:   tt.shrinkAt,
		growLen:    tt.growLen,
		shrinkLen:  tt.shrinkLen,
		grows:      tt.grows,
		shrinks:    tt.shrinks,
		forced:     tt.forced,
		generation: tt.generation,
		eq:         tt.eq,
		hash:       tt.hash,
	}
	if tt.slots != nil {
		dup.slots = make([]slot[T], len(tt.slots))
		copy(dup.slots, tt.slots)
	}
	return dup
}

// ContainsSlice reports whether every element of items is present in the
// table (an empty items slice is vacuously contained).  The whole batch
// runs under one hold of the read lock, so the answer describes one
// consistent snapshot.  For "at least one present" use ContainsAny.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *HashTab[T]) ContainsSlice(items []T) bool {
	if len(items) == 0 {
		return true // an empty batch is vacuously contained
	}
	if tt == nil {
		return false // a nil table contains nothing
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for _, item := range items {
		if !tt.nlContains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one element of items is present
// in the table.  The whole batch runs under one hold of the read lock.
// An empty items slice contains nothing.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *HashTab[T]) ContainsAny(items []T) bool {
	if tt == nil || len(items) == 0 {
		return false
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for _, item := range items {
		if tt.nlContains(item) {
			return true
		}
	}
	return false
}

// nlContains is Contains without locking; the caller must hold the lock.
func (tt *HashTab[T]) nlContains(find T) bool {
	_, found := tt.NlSearch(find)
	return found
}

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (NlInsert replaces an equal element and
// reports false, exactly as a caller loop of Insert does).  The whole
// batch runs under one hold of the write lock, so it is atomic: an
// interleaved reader sees none or all of it, never part.  A batch that
// crosses the grow threshold starts the background resizer, whose
// rebuilds run after the batch releases the lock.
//
// It follows the insert contract: a batch that would store an element
// into a nil table or a zero-value table panics with the standard
// insert-family message; an empty batch stores nothing and is tolerated
// everywhere.
// Complexity is O(k) average amortized, O(n) worst case per element, k = len(items).
func (tt *HashTab[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	tt.insertContractCheck("InsertSlice")
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.insertAll(items)
}

// insertAll is the shared insert loop of the batch inserters; the caller
// holds the write lock and has already applied the insert contract
// check.
func (tt *HashTab[T]) insertAll(items []T) bool {
	changed := false
	for _, item := range items {
		if tt.NlInsert(item) {
			changed = true
		}
	}
	return changed
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between.  The iterator is drained (and the table snapshot it
// may come from taken) before the write lock is acquired, so feeding a
// table its own Values() is safe and the lock is never held across the
// caller's iteration.  It reports whether at least one element was new
// and follows the same insert contract as InsertSlice.  A nil iterator
// inserts nothing and returns false.
// Complexity is O(k) average amortized, k = number of yielded elements.
func (tt *HashTab[T]) InsertIter(seq iter.Seq[T]) bool {
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.insertAll(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present.  The whole batch runs under one hold of the write
// lock, so it is atomic.  A batch that crosses the shrink threshold
// starts the background resizer, whose rebuild runs after the batch
// releases the lock.  Deleting is tolerated on a nil table (returns
// false), like Delete.
// Complexity is O(k) average, O(n) per background rebuild, k = len(items).
func (tt *HashTab[T]) DeleteSlice(items []T) bool {
	if tt == nil {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	changed := false
	for _, item := range items {
		if tt.NlDelete(item) {
			changed = true
		}
	}
	return changed
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The elements are snapshotted under the
// read lock, fx runs on the snapshot without any lock held (so fx may
// safely call back into the table), and the matched elements are then
// deleted under one hold of the write lock — collect-then-delete,
// mandatory here as in cuckoo: a delete that crosses the shrink
// threshold rebuilds the table and repositions every element, so the
// table must never be modified while it is being walked.  A nil table,
// an empty table or a nil fx deletes nothing and returns false.
// Complexity is O(n) average, O(n) per background rebuild.
func (tt *HashTab[T]) DeleteFunc(fx func(T) bool) bool {
	if tt == nil || fx == nil {
		return false
	}
	doomed := make([]T, 0)
	for _, v := range tt.Slice() { // snapshot under the read lock
		if fx(v) { // caller predicate, no lock held
			doomed = append(doomed, v)
		}
	}
	if len(doomed) == 0 {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	changed := false
	for _, item := range doomed {
		if tt.NlDelete(item) {
			changed = true
		}
	}
	return changed
}
