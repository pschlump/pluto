/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package b_tree

import (
	"fmt"
	"iter"
	"strings"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for ordered structures: materialization, rendering, deep
// copy, membership batches and predicate bulk delete.  Everything is
// additive — no existing method changed — and every new method is a total
// function on the read side: a nil tree and the zero value behave as an
// empty tree, exactly like the read operations in b_tree.go.  The only
// panics the new methods can surface are the insert-contract panics
// below, and only when a batch would actually store an element (a caller
// loop over the same items hits the identical panics).
//
// Ordering comes free: every walk is the in-order traversal, so Slice and
// String are ascending and deterministic with no sorting anywhere.

// insertContractCheck applies the insert panic contract for the batch
// inserters.  `caller` names the public method in the messages.
func (tt *BTree[T]) insertContractCheck(caller string) {
	if tt == nil {
		panic(fmt.Sprintf("b_tree: %s called on a nil tree", caller))
	}
	if tt.cmp == nil {
		panic(fmt.Sprintf("b_tree: %s called on a tree with no comparison function (create the tree with NewBTree or NewBTreeFunc)", caller))
	}
}

// Contains reports whether the tree holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(log n).
func (tt *BTree[T]) Contains(find T) bool {
	_, found := tt.Search(find)
	return found
}

// Slice returns a fresh slice holding every element of the tree in
// in-order (ascending) order — the order All yields.  The slice is a
// copy: mutating it does not affect the tree.  A nil tree yields nil; an
// empty tree yields an empty slice.
// Complexity is O(n).
func (tt *BTree[T]) Slice() []T {
	if tt == nil {
		return nil
	}
	items := make([]T, 0, tt.length)
	for _, v := range tt.All() { // index ignored: All is Seq2, the value is 2nd
		items = append(items, v)
	}
	return items
}

// String renders the tree as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets: "[a b c]", in-order
// (ascending).  The in-order walk is already deterministic, so no sorting
// is involved.  A nil or empty tree renders as "[]".  For a custom
// per-element rendering use StringFunc.
// Complexity is O(n).
func (tt *BTree[T]) String() string {
	return tt.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are walked in-order and the formatted strings
// joined inside square brackets, so the output is ascending and
// deterministic.  A nil format function renders as "[]".
// Complexity is O(n).
func (tt *BTree[T]) StringFunc(format func(T) string) string {
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
// different shapes (even of different orders) are equal.  Two empty
// trees are equal, a nil tree equals an empty tree, and Equal is
// symmetric on any pair that satisfies its precondition.
//
// Precondition: both trees must use consistent comparison functions (a
// nil tree trivially does).  Functions are not comparable, so this cannot
// be checked — two trees built by the same constructor over the same
// element type always satisfy it.
// Complexity is O(n log n).
func (tt *BTree[T]) Equal(other *BTree[T]) bool {
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

// Clone returns a deep copy of the tree: a fresh tree with the same
// order and comparison function and a fresh copy of every node, with the
// keys and children slices copied (never shared) so that later growth of
// either tree never overwrites the other's backing arrays.  No backing
// memory is shared with the receiver.
// Complexity is O(n).
func (tt *BTree[T]) Clone() *BTree[T] {
	if tt == nil {
		return nil
	}
	var copyNode func(n *BTreeNode[T]) *BTreeNode[T]
	copyNode = func(n *BTreeNode[T]) *BTreeNode[T] {
		if n == nil {
			return nil
		}
		dup := &BTreeNode[T]{
			keys: append([]T(nil), n.keys...),
		}
		if n.children != nil { // leaves keep nil children (the walk relies on it)
			dup.children = make([]*BTreeNode[T], len(n.children))
			for i, c := range n.children {
				dup.children[i] = copyNode(c)
			}
		}
		return dup
	}
	return &BTree[T]{
		root:   copyNode(tt.root),
		length: tt.length,
		order:  tt.order,
		cmp:    tt.cmp,
	}
}

// ContainsSlice reports whether every element of items is present in the
// tree (an empty items slice is vacuously contained).  For "at least one
// present" use ContainsAny.
// Complexity is O(k·log n), k = len(items).
func (tt *BTree[T]) ContainsSlice(items []T) bool {
	for _, item := range items {
		if !tt.Contains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one element of items is present in
// the tree.  An empty items slice contains nothing.
// Complexity is O(k·log n), k = len(items).
func (tt *BTree[T]) ContainsAny(items []T) bool {
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
// into a nil tree or a zero-value tree panics with the standard
// insert-family message; an empty batch stores nothing and is tolerated
// everywhere.
// Complexity is O(k·log n), k = len(items).
func (tt *BTree[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	tt.insertContractCheck("InsertSlice")
	return tt.insertAll(items)
}

// insertAll is the shared insert loop of the batch inserters; the caller
// has already applied the insert contract check.
func (tt *BTree[T]) insertAll(items []T) bool {
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
// insert, so feeding a tree a sequence derived from itself is safe (note
// that this package's All is an iter.Seq2, so adapt it — e.g. collect a
// Slice first).  It reports whether at least one element was new and
// follows the same insert contract as InsertSlice.  A nil iterator
// inserts nothing and returns false.
// Complexity is O(k·log n), k = number of yielded elements.
func (tt *BTree[T]) InsertIter(seq iter.Seq[T]) bool {
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
// one was present.  Deleting is tolerated on a nil tree (returns false),
// like Delete.
// Complexity is O(k·log n), k = len(items).
func (tt *BTree[T]) DeleteSlice(items []T) bool {
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
// first (an in-order snapshot) and deleted afterwards — the tree is never
// modified while it is being walked.  A nil tree, an empty tree or a nil
// fx deletes nothing and returns false.
// Complexity is O(n log n).
func (tt *BTree[T]) DeleteFunc(fx func(T) bool) bool {
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
