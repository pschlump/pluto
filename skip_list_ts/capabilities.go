/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package skip_list_ts

import (
	"fmt"
	"iter"
	"strings"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for ordered structures: materialization, rendering, deep
// copy, membership batches and predicate bulk delete.  Everything is
// additive and mirrors skip_list's capabilities.go method for method.
// Lock discipline: a batch mutator takes the write lock once for the
// whole batch (a batch is atomic — an interleaved reader never sees part
// of one), and the compute-after-snapshot methods (Slice/String/Equal/
// DeleteFunc's match pass) snapshot under the read lock and run unlocked
// afterwards, so a caller format or predicate function may safely call
// back into the list.  No new Nl methods: the new methods are
// self-locking, not compounds.
//
// Ordering comes free: the level-0 lane is the sorted sequence, so Slice
// and String are ascending and deterministic with no sorting anywhere.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *SkipList[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("skip_list_ts: %s called on a nil list", caller))
	}
	if tt.cmp == nil {
		panic(fmt.Sprintf("skip_list_ts: %s called on a list with no comparison function (create the list with NewSkipList or NewSkipListFunc)", caller))
	}
}

// Contains reports whether the list holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Contains(find T) bool {
	_, found := tt.Search(find)
	return found
}

// Slice returns a fresh slice holding every element of the list in
// ascending order — the order All yields (the level-0 lane).  The
// snapshot is taken under the read lock, so this is safe to call
// concurrently with any list operation.  The slice is a copy: mutating it
// does not affect the list.  An empty or nil list yields nil.
// Complexity is O(n).
func (tt *SkipList[T]) Slice() []T {
	return tt.toSlice()
}

// String renders the list as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets: "[a b c]", ascending.
// The elements are snapshotted under the read lock and the formatting
// runs without the lock.  A nil or empty list renders as "[]".  For a
// custom per-element rendering use StringFunc.
// Complexity is O(n).
func (tt *SkipList[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are walked in ascending order and the formatted
// strings joined inside square brackets, so the output is ascending and
// deterministic.  The elements are snapshotted under the read lock and
// the format function runs without the lock, so it may safely call back
// into the list.  A nil format function renders as "[]".
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
// Each list is touched under its own lock only, never two locks at once,
// so Equal never deadlocks — not even when two goroutines compare the
// same pair of lists in opposite directions.
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
	for _, v := range tt.Slice() { // tt's lock, released before other is touched
		if _, found := other.Search(v); !found { // other's lock, one element at a time
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the list: a fresh list with the same
// comparison function and the same elements, built by re-inserting them
// (so the clone's tower heights are freshly drawn from the global RNG —
// the observable ordering is identical, the internal layout is not).  No
// backing memory is shared with the receiver, so mutating either list
// afterwards never affects the other.  The elements are snapshotted under
// the read lock (cmp is immutable after construction), and the
// re-insertion runs on the local clone without any lock held.
// Complexity is O(n log₂ n) expected.
func (tt *SkipList[T]) Clone() *SkipList[T] {
	if tt == nil {
		return nil
	}
	items := tt.toSlice() // snapshot under the read lock
	cmp := tt.cmp
	dup := &SkipList[T]{cmp: cmp}
	for _, v := range items {
		dup.Insert(v) // dup is local: its own lock, no contention
	}
	return dup
}

// ContainsSlice reports whether every element of items is present in the
// list (an empty items slice is vacuously contained).  The whole batch
// runs under one hold of the read lock, so the answer describes one
// consistent snapshot.  For "at least one present" use ContainsAny.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) ContainsSlice(items []T) bool {
	if len(items) == 0 {
		return true // an empty batch is vacuously contained
	}
	if tt == nil {
		return false // a nil list contains nothing
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

// ContainsAny reports whether at least one element of items is present in
// the list.  The whole batch runs under one hold of the read lock.  An
// empty items slice contains nothing.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) ContainsAny(items []T) bool {
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
func (tt *SkipList[T]) nlContains(find T) bool {
	if tt.isEmpty() {
		return false
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, find) < 0 {
			cur = cur.forward[i]
		}
	}
	next := cur.forward[0]
	return next != nil && tt.cmp(next.data, find) == 0
}

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (insertLocked replaces an equal element
// and reports false, exactly as a caller loop of Insert does).  The whole
// batch runs under one hold of the write lock, so it is atomic: an
// interleaved reader sees none or all of it, never part.
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.insertAll(items)
}

// insertAll is the shared insert loop of the batch inserters; the caller
// holds the write lock and has already applied the insert contract
// check.
func (tt *SkipList[T]) insertAll(items []T) bool {
	changed := false
	for _, item := range items {
		if tt.insertLocked(item) {
			changed = true
		}
	}
	return changed
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between.  The iterator is drained (and the list snapshot it
// may come from taken) before the write lock is acquired, so feeding a
// list its own All() is safe and the lock is never held across the
// caller's iteration.  It reports whether at least one element was new
// and follows the same insert contract as InsertSlice.  A nil iterator
// inserts nothing and returns false.
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.insertAll(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present.  The whole batch runs under one hold of the write
// lock, so it is atomic.  Deleting is tolerated on a nil list (returns
// false), like Delete.
// Complexity is O(k·log₂ n) expected, k = len(items).
func (tt *SkipList[T]) DeleteSlice(items []T) bool {
	if tt == nil {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	changed := false
	for _, item := range items {
		if !tt.isEmpty() && tt.deleteLocked(item) {
			changed = true
		}
	}
	return changed
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The elements are snapshotted under the
// read lock, fx runs on the snapshot without any lock held (so fx may
// safely call back into the list), and the matched elements are then
// deleted under one hold of the write lock — collect-then-delete, the
// list is never modified while it is being walked.  For contiguous
// predicate classes NlDeleteRange under a held Lock is the cheaper tool;
// DeleteFunc is for arbitrary predicates.  A nil list, an empty list or a
// nil fx deletes nothing and returns false.
// Complexity is O(n log₂ n) expected.
func (tt *SkipList[T]) DeleteFunc(fx func(T) bool) bool {
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
		if !tt.isEmpty() && tt.deleteLocked(item) {
			changed = true
		}
	}
	return changed
}
