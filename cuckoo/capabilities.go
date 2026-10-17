/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo

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
// read operations in hash_tab.go.  The only panics the new methods can
// surface are the insert-contract panics below (plus Insert's own
// pathological placement panic, which a caller loop over the same items
// would hit identically), and the insert contract fires only when a
// batch would actually store an element.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *HashTab[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("cuckoo: %s called on a nil table (a nil table cannot store an element)", caller))
	}
	if tt.eq == nil || tt.hash == nil {
		panic(fmt.Sprintf("cuckoo: %s called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)", caller))
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
// order is slot order — the order All and Values yield — which depends on
// the per-table hash seed and the table's resize history, and is
// therefore unspecified; sort the result when a fixed order is needed.
// The slice is a copy: mutating it does not affect the table.  A nil
// table yields nil; an empty table yields an empty slice.
// Complexity is O(n).
func (tt *HashTab[T]) Slice() []T {
	if tt == nil {
		return nil
	}
	items := make([]T, 0, tt.length)
	for i := range tt.slots {
		if tt.slots[i].used {
			items = append(items, tt.slots[i].data)
		}
	}
	return items
}

// String renders the table as its elements formatted with fmt.Sprint,
// the formatted strings sorted ascending and joined with single spaces
// inside square brackets: "[a b c]".  Sorting the formatted strings —
// not the elements — is what makes the output deterministic: slot order
// depends on the per-table random hash seed and differs from run to
// run, but two tables holding the same elements always render the same
// string.  A nil or empty table renders as "[]".  For a custom
// per-element rendering use StringFunc.
// Complexity is O(n log n).
func (tt *HashTab[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are formatted, the strings sorted ascending and
// joined inside square brackets, so the output is deterministic whatever
// order the slots happen to be in.  A nil format function renders as
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
	for _, v := range tt.Slice() {
		if !other.Contains(v) {
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
// A nil table clones to nil; a zero-value table clones to a zero-value
// table.
// Complexity is O(n).
func (tt *HashTab[T]) Clone() *HashTab[T] {
	if tt == nil {
		return nil
	}
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
// table (an empty items slice is vacuously contained).  For "at least
// one present" use ContainsAny.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *HashTab[T]) ContainsSlice(items []T) bool {
	for _, item := range items {
		if !tt.Contains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one element of items is present
// in the table.  An empty items slice contains nothing.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (tt *HashTab[T]) ContainsAny(items []T) bool {
	for _, item := range items {
		if tt.Contains(item) {
			return true
		}
	}
	return false
}

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (Insert replaces an equal element and
// reports false, exactly as a caller loop does).  A batch that crosses
// the grow threshold grows the table mid-batch, exactly as a caller loop
// of Insert does.
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
	return tt.insertAll(items)
}

// insertAll is the shared insert loop of the batch inserters; the caller
// has already applied the insert contract check.
func (tt *HashTab[T]) insertAll(items []T) bool {
	changed := false
	for _, item := range items {
		if tt.Insert(item) {
			changed = true
		}
	}
	return changed
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between.  The iterator is drained to a slice before the
// first insert, so feeding a table its own Values() is safe.  It reports
// whether at least one element was new and follows the same insert
// contract as InsertSlice.  A nil iterator inserts nothing and returns
// false.
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
	return tt.insertAll(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present.  A batch that crosses the shrink threshold shrinks
// the table mid-batch (best effort, exactly as a caller loop of Delete).
// Deleting is tolerated on a nil table (returns false), like Delete.
// Complexity is O(k) average, O(n) per triggered shrink, k = len(items).
func (tt *HashTab[T]) DeleteSlice(items []T) bool {
	changed := false
	for _, item := range items {
		if tt.Delete(item) {
			changed = true
		}
	}
	return changed
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The matching elements are collected
// first and deleted afterwards — mandatory here, not just tidiness:
// Delete's shrink rebuilds the table and repositions every element, so
// deleting during a slot walk would visit rebuilt slots twice and skip
// others.  A nil table, an empty table or a nil fx deletes nothing and
// returns false.
// Complexity is O(n) average, O(n) per triggered shrink.
func (tt *HashTab[T]) DeleteFunc(fx func(T) bool) bool {
	if tt == nil || fx == nil || tt.length == 0 {
		return false
	}
	var doomed []T
	for i := range tt.slots {
		if tt.slots[i].used && fx(tt.slots[i].data) {
			doomed = append(doomed, tt.slots[i].data)
		}
	}
	changed := false
	for _, item := range doomed {
		if tt.Delete(item) {
			changed = true
		}
	}
	return changed
}
