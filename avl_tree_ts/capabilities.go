/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

import (
	"fmt"
	"iter"
	"strings"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for ordered structures: materialization, rendering, deep
// copy, membership batches and predicate bulk delete.  Everything is
// additive and mirrors avl_tree's capabilities.go method for method.
// Lock discipline: a batch mutator takes the write lock once for the
// whole batch (a batch is atomic — an interleaved reader never sees part
// of one), and the compute-after-snapshot methods (Slice/String/Equal/
// DeleteFunc's match pass) snapshot under the read lock and run unlocked
// afterwards, so a caller format or predicate function may safely call
// back into the tree.  No new Nl methods: the new methods are
// self-locking, not compounds.
//
// Ordering comes free: every walk is the in-order traversal, so Slice and
// String are ascending and deterministic with no sorting anywhere.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *AvlTree[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("avl_tree_ts: %s called on a nil tree (a nil tree cannot store an element)", caller))
	}
	if tt.cmp == nil {
		panic(fmt.Sprintf("avl_tree_ts: %s called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)", caller))
	}
}

// Contains reports whether the tree holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Contains(find T) bool {
	_, found := tt.Search(find)
	return found
}

// Slice returns a fresh slice holding every element of the tree in
// in-order (ascending) order — the order All yields.  The snapshot is
// taken under the read lock, so this is safe to call concurrently with
// any tree operation.  The slice is a copy: mutating it does not affect
// the tree.  A nil tree yields nil; an empty tree yields an empty slice.
// Complexity is O(n).
func (tt *AvlTree[T]) Slice() []T {
	if tt == nil {
		return nil
	}
	items, _ := tt.snapshot()
	return items
}

// String renders the tree as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets: "[a b c]", in-order
// (ascending).  The in-order walk is already deterministic, so no sorting
// is involved.  The elements are snapshotted under the read lock and the
// formatting runs without the lock.  A nil or empty tree renders as "[]".
// For a custom per-element rendering use StringFunc.
// Complexity is O(n).
func (tt *AvlTree[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are walked in-order and the formatted strings
// joined inside square brackets, so the output is ascending and
// deterministic.  The elements are snapshotted under the read lock and
// the format function runs without the lock, so it may safely call back
// into the tree.  A nil format function renders as "[]".
// Complexity is O(n).
func (tt *AvlTree[T]) StringFunc(format func(T) string) string {
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
// compared, never shape — two trees holding the same elements in
// different shapes are equal.  Two empty trees are equal, a nil tree
// equals an empty tree, and Equal is symmetric on any pair that satisfies
// its precondition.
//
// Each tree is touched under its own lock only, never two locks at once
// (the set-operation discipline above), so Equal never deadlocks — not
// even when two goroutines compare the same pair of trees in opposite
// directions.
//
// Precondition: both trees must use consistent comparison functions (a
// nil tree trivially does).  Functions are not comparable, so this cannot
// be checked — two trees built by the same constructor over the same
// element type always satisfy it.
// Complexity is O(n log₂ n).
func (tt *AvlTree[T]) Equal(other *AvlTree[T]) bool {
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

// Clone returns a deep copy of the tree: a fresh tree with the same
// comparison function and a fresh copy of every node (heights included),
// preserving the receiver's shape.  No backing memory is shared with the
// receiver, so mutating either tree afterwards never affects the other.
// This is the returning form — the existing destination-form Copy(yy)
// is unchanged; new code prefers Clone.  The read lock is held for the
// whole copy.  A nil tree clones to nil; a zero-value tree clones to a
// zero-value tree.
// Complexity is O(n).
func (tt *AvlTree[T]) Clone() *AvlTree[T] {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var copyNode func(n *AvlTreeElement[T]) *AvlTreeElement[T]
	copyNode = func(n *AvlTreeElement[T]) *AvlTreeElement[T] {
		if n == nil {
			return nil
		}
		return &AvlTreeElement[T]{
			data:   n.data,
			height: n.height,
			left:   copyNode(n.left),
			right:  copyNode(n.right),
		}
	}
	return &AvlTree[T]{
		root:   copyNode(tt.root),
		length: tt.length,
		cmp:    tt.cmp,
	}
}

// ContainsSlice reports whether every element of items is present in the
// tree (an empty items slice is vacuously contained).  The whole batch
// runs under one hold of the read lock, so the answer describes one
// consistent snapshot.  For "at least one present" use ContainsAny.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *AvlTree[T]) ContainsSlice(items []T) bool {
	if len(items) == 0 {
		return true // an empty batch is vacuously contained
	}
	if tt == nil {
		return false // a nil tree contains nothing
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
// the tree.  The whole batch runs under one hold of the read lock.  An
// empty items slice contains nothing.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *AvlTree[T]) ContainsAny(items []T) bool {
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
func (tt *AvlTree[T]) nlContains(find T) bool {
	cur := tt.root
	for cur != nil {
		c := tt.compare(find, cur.data)
		if c == 0 {
			return true
		}
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	return false
}

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (nlInsert replaces an equal element and
// reports false, exactly as a caller loop of Insert does).  The whole
// batch runs under one hold of the write lock, so it is atomic: an
// interleaved reader sees none or all of it, never part.
//
// It follows the insert contract: a batch that would store an element
// into a nil tree or a zero-value tree panics with the standard
// insert-family message; an empty batch stores nothing and is tolerated
// everywhere.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *AvlTree[T]) InsertSlice(items []T) bool {
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
func (tt *AvlTree[T]) insertAll(items []T) bool {
	changed := false
	for _, item := range items {
		if tt.nlInsert(item) {
			changed = true
		}
	}
	return changed
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between.  The iterator is drained (and the tree snapshot it
// may come from taken) before the write lock is acquired, so feeding a
// tree its own All() is safe and the lock is never held across the
// caller's iteration.  It reports whether at least one element was new
// and follows the same insert contract as InsertSlice.  A nil iterator
// inserts nothing and returns false.
// Complexity is O(k·log₂ n), k = number of yielded elements.
func (tt *AvlTree[T]) InsertIter(seq iter.Seq[T]) bool {
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
// lock, so it is atomic.  Deleting is tolerated on a nil tree (returns
// false), like Delete.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *AvlTree[T]) DeleteSlice(items []T) bool {
	if tt == nil {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	changed := false
	for _, item := range items {
		if tt.nlDelete(item) {
			changed = true
		}
	}
	return changed
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The elements are snapshotted under the
// read lock, fx runs on the snapshot without any lock held (so fx may
// safely call back into the tree), and the matched elements are then
// deleted under one hold of the write lock — collect-then-delete, the
// tree is never modified while it is being walked.  A nil tree, an empty
// tree or a nil fx deletes nothing and returns false.
// Complexity is O(n log₂ n).
func (tt *AvlTree[T]) DeleteFunc(fx func(T) bool) bool {
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
		if tt.nlDelete(item) {
			changed = true
		}
	}
	return changed
}
