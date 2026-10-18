/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package interval_tree

import (
	"fmt"
	"strings"
)

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) in the map-shaped, reduced form the tries use: the tree
// maps Interval[K] to V, so the batch inserters are deliberately absent
// (a map-shaped batch insert would need key/value pairs — callers loop;
// the surface stays small).  What is here: materialization, rendering,
// deep copy, exact-match membership batches and predicate bulk delete.
// Everything is additive — no existing method changed — and every new
// method is a total function on the read side: a nil tree and the zero
// value behave as an empty tree.  None of the new methods panics.
//
// Membership here is EXACT-MATCH (the found flag of Search): Contains
// answers "is this precise interval stored".  The overlap machinery
// (AnyOverlap, Overlaps, Stab) is a different query family and is not
// involved.  Ordering comes free: the in-order walk is ascending
// (Lo, Hi), so Slice and String are deterministic with no sorting.

// Contains reports whether the tree holds an interval equal to iv — the
// found flag of the exact-match Search.  It is NOT an overlap test: an
// interval that merely overlaps stored ones is not contained; use
// AnyOverlap or Overlaps for that.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Contains(iv Interval[K]) bool {
	_, found := tt.Search(iv)
	return found
}

// Slice returns a fresh slice holding every stored interval in ascending
// (Lo, Hi) order — the order All yields.  The values are not included;
// range All() for the pairs.  The slice is a copy: mutating it does not
// affect the tree.  A nil or empty tree yields nil.
// Complexity is O(n).
func (tt *IntervalTree[K, V]) Slice() []Interval[K] {
	if tt == nil || tt.length == 0 {
		return nil
	}
	items := make([]Interval[K], 0, tt.length)
	for iv := range tt.All() {
		items = append(items, iv)
	}
	return items
}

// String renders the tree as its intervals formatted "Lo..Hi" and joined
// with single spaces inside square brackets, ascending: "[1..5 2..7]".
// The in-order walk is already deterministic, so no sorting is involved.
// A nil or empty tree renders as "[]".  For a custom per-interval
// rendering use StringFunc.
// Complexity is O(n).
func (tt *IntervalTree[K, V]) String() string {
	return tt.StringFunc(func(iv Interval[K]) string {
		return fmt.Sprintf("%v..%v", iv.Lo, iv.Hi)
	})
}

// StringFunc is String with a caller supplied per-interval format
// function; the intervals are walked in ascending (Lo, Hi) order and the
// formatted strings joined inside square brackets, so the output is
// ascending and deterministic.  A nil format function renders as "[]".
// Complexity is O(n).
func (tt *IntervalTree[K, V]) StringFunc(format func(Interval[K]) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, iv := range tt.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(iv))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether tt and other hold the same interval KEYS: same
// length and every interval of tt found in other by exact-match Search.
//
// The values are NOT compared — V is any, and comparing two arbitrary
// values is not expressible without boxing them into an interface.  Two
// trees with identical intervals but different satellite values are
// Equal; this is the map-shaped rule the tries share.  Two empty trees
// are equal, a nil tree equals an empty tree, and Equal is symmetric on
// any pair that satisfies its precondition.
//
// Precondition: both trees must use consistent endpoint comparison
// functions (a nil tree trivially does).  Functions are not comparable,
// so this cannot be checked — two trees built by the same constructor
// over the same key type always satisfy it.
// Complexity is O(n log₂ n).
func (tt *IntervalTree[K, V]) Equal(other *IntervalTree[K, V]) bool {
	if tt.Len() != other.Len() {
		return false
	}
	if tt.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, iv := range tt.Slice() {
		if _, found := other.Search(iv); !found {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the tree: a fresh tree with the same
// endpoint comparison function and a fresh copy of every node (values,
// colors, the maxHi augmentation and parent links included), preserving
// the receiver's shape.  No backing memory is shared with the receiver,
// so mutating either tree afterwards never affects the other.
// Complexity is O(n).
func (tt *IntervalTree[K, V]) Clone() *IntervalTree[K, V] {
	if tt == nil {
		return nil
	}
	var copyNode func(n, parent *itNode[K, V]) *itNode[K, V]
	copyNode = func(n, parent *itNode[K, V]) *itNode[K, V] {
		if n == nil {
			return nil
		}
		dup := &itNode[K, V]{
			iv:     n.iv,
			val:    n.val,
			maxHi:  n.maxHi,
			red:    n.red,
			parent: parent,
		}
		dup.left = copyNode(n.left, dup)
		dup.right = copyNode(n.right, dup)
		return dup
	}
	return &IntervalTree[K, V]{
		root:   copyNode(tt.root, nil),
		length: tt.length,
		cmp:    tt.cmp,
	}
}

// ContainsSlice reports whether every interval of items is stored
// (exact-match; an empty items slice is vacuously contained).  For "at
// least one stored" use ContainsAny.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *IntervalTree[K, V]) ContainsSlice(items []Interval[K]) bool {
	for _, item := range items {
		if !tt.Contains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one interval of items is stored
// (exact-match).  An empty items slice contains nothing.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *IntervalTree[K, V]) ContainsAny(items []Interval[K]) bool {
	for _, item := range items {
		if tt.Contains(item) {
			return true
		}
	}
	return false
}

// DeleteSlice deletes each interval of items (exact-match, the values
// are irrelevant to the match) and reports whether at least one was
// present.  Deleting is tolerated on a nil tree (returns false), like
// Delete.
// Complexity is O(k·log₂ n), k = len(items).
func (tt *IntervalTree[K, V]) DeleteSlice(items []Interval[K]) bool {
	changed := false
	for _, item := range items {
		if tt.Delete(item) {
			changed = true
		}
	}
	return changed
}

// DeleteFunc deletes every entry whose interval AND value satisfy fx and
// reports whether anything was deleted.  The predicate sees both the
// interval and its satellite value.  The matching entries are collected
// first (an ascending snapshot) and deleted afterwards — the tree is
// never modified while it is being walked.  A nil tree, an empty tree or
// a nil fx deletes nothing and returns false.
// Complexity is O(n log₂ n).
func (tt *IntervalTree[K, V]) DeleteFunc(fx func(Interval[K], V) bool) bool {
	if tt == nil || fx == nil || tt.length == 0 {
		return false
	}
	type pair struct {
		iv  Interval[K]
		val V
	}
	doomed := make([]pair, 0)
	for iv, val := range tt.All() {
		if fx(iv, val) {
			doomed = append(doomed, pair{iv, val})
		}
	}
	if len(doomed) == 0 {
		return false
	}
	changed := false
	for _, d := range doomed {
		if tt.Delete(d.iv) {
			changed = true
		}
	}
	return changed
}
