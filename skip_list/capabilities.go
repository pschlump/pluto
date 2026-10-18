/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package skip_list

import (
	"fmt"
	"iter"
	"strings"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for ordered structures: materialization, rendering, deep
// copy, membership batches and predicate bulk delete.  Everything is
// additive — no existing method changed — and every new method is a total
// function on the read side: a nil list and the zero value behave as an
// empty list, exactly like the read operations in skip_list.go.  The only
// panics the new methods can surface are the insert-contract panics
// below, and only when a batch would actually store an element (a caller
// loop over the same items hits the identical panics).
//
// Ordering comes free: the level-0 lane is the sorted sequence, so Slice
// and String are ascending and deterministic with no sorting anywhere.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *SkipList[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("skip_list: %s called on a nil list", caller))
	}
	if tt.cmp == nil {
		panic(fmt.Sprintf("skip_list: %s called on a list with no comparison function (create the list with NewSkipList or NewSkipListFunc)", caller))
	}
}

// Contains reports whether the list holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(log₂ n) expected, O(n) worst case.
func (tt *SkipList[T]) Contains(find T) bool {
	_, found := tt.Search(find)
	return found
}

// Slice returns a fresh slice holding every element of the list in
// ascending order — the order All yields (the level-0 lane).  The slice
// is a copy: mutating it does not affect the list.  A nil or never-filled
// list yields nil; a drained list yields an empty slice — both of
// length 0.
// Complexity is O(n).
func (tt *SkipList[T]) Slice() []T {
	if tt == nil || tt.head == nil {
		return nil
	}
	items := make([]T, 0, tt.length)
	for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
		items = append(items, cur.data)
	}
	return items
}

// String renders the list as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets: "[a b c]", ascending.
// The level-0 walk is already deterministic, so no sorting is involved.
// A nil or empty list renders as "[]".  For a custom per-element
// rendering use StringFunc.
// Complexity is O(n).
func (tt *SkipList[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are walked in ascending order and the formatted
// strings joined inside square brackets, so the output is ascending and
// deterministic.  A nil format function renders as "[]".
// Complexity is O(n).
func (tt *SkipList[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range tt.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether tt and other hold the same set of elements: same
// length and every element of tt found in other by Search.  Membership is
// compared, never tower layout.  Two empty lists are equal, a nil list
// equals an empty list, and Equal is symmetric on any pair that satisfies
// its precondition.
//
// Precondition: both lists must use consistent comparison functions (a
// nil list trivially does).  Functions are not comparable, so this cannot
// be checked — two lists built by the same constructor over the same
// element type always satisfy it.
// Complexity is O(n log₂ n) expected.
func (tt *SkipList[T]) Equal(other *SkipList[T]) bool {
	if tt.Len() != other.Len() {
		return false
	}
	if tt.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, v := range tt.Slice() {
		if _, found := other.Search(v); !found {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the list: a fresh list with the same
// comparison function and the same elements, built by re-inserting them
// (so the clone's tower heights are freshly drawn from the global RNG —
// the observable ordering is identical, the internal layout is not).
// No backing memory is shared with the receiver, so mutating either list
// afterwards never affects the other.
// Complexity is O(n log₂ n) expected.
func (tt *SkipList[T]) Clone() *SkipList[T] {
	if tt == nil {
		return nil
	}
	dup := &SkipList[T]{cmp: tt.cmp}
	for _, v := range tt.Slice() {
		dup.Insert(v)
	}
	return dup
}

// ContainsSlice reports whether every element of items is present in the
// list (an empty items slice is vacuously contained).  For "at least one
// present" use ContainsAny.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) ContainsSlice(items []T) bool {
	for _, item := range items {
		if !tt.Contains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one element of items is present in
// the list.  An empty items slice contains nothing.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) ContainsAny(items []T) bool {
	for _, item := range items {
		if tt.Contains(item) {
			return true
		}
	}
	return false
}

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (Insert replaces an equal element and
// reports false, exactly as a caller loop does).
//
// It follows the insert contract: a batch that would store an element
// into a nil list or a zero-value list panics with the standard
// insert-family message; an empty batch stores nothing and is tolerated
// everywhere.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	tt.insertContractCheck("InsertSlice")
	return tt.insertAll(items)
}

// insertAll is the shared insert loop of the batch inserters; the caller
// has already applied the insert contract check.
func (tt *SkipList[T]) insertAll(items []T) bool {
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
// interface between.  The iterator is drained to a slice before the first
// insert, so feeding a list its own All() is safe.  It reports whether at
// least one element was new and follows the same insert contract as
// InsertSlice.  A nil iterator inserts nothing and returns false.
// Complexity is O(k·log₂ n) expected, k = number of yielded elements.
func (tt *SkipList[T]) InsertIter(seq iter.Seq[T]) bool {
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
// one was present.  Deleting is tolerated on a nil list (returns false),
// like Delete.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) DeleteSlice(items []T) bool {
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
// first (an ascending snapshot) and deleted afterwards — the list is
// never modified while it is being walked.  For contiguous predicate
// classes DeleteRange is the cheaper tool; DeleteFunc is for arbitrary
// predicates.  A nil list, an empty list or a nil fx deletes nothing and
// returns false.
// Complexity is O(n log₂ n) expected.
func (tt *SkipList[T]) DeleteFunc(fx func(T) bool) bool {
	if tt == nil || fx == nil || tt.length == 0 {
		return false
	}
	doomed := make([]T, 0)
	for _, v := range tt.Slice() {
		if fx(v) {
			doomed = append(doomed, v)
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
