/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package avl_tree_ts

import (
	"iter"
	"slices"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the largest element strictly less than key.
func (tt *AvlTree[T]) Lower(key T) (T, bool) {
	var result T
	found := false
	if tt == nil {
		return result, false
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for node := tt.root; node != nil; {
		if tt.cmp(node.data, key) < 0 {
			result, found = node.data, true
			node = node.right
		} else {
			node = node.left
		}
	}
	return result, found
}

// Higher returns the smallest element strictly greater than key.
func (tt *AvlTree[T]) Higher(key T) (T, bool) {
	var result T
	found := false
	if tt == nil {
		return result, false
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for node := tt.root; node != nil; {
		if tt.cmp(node.data, key) > 0 {
			result, found = node.data, true
			node = node.left
		} else {
			node = node.right
		}
	}
	return result, found
}

// RangeOf returns a snapshot iterator over an open/closed/unbounded range.
// Snapshot construction is O(log n + results).
func (tt *AvlTree[T]) RangeOf(value ranges.Range[T]) iter.Seq[T] {
	if tt == nil {
		return slices.Values([]T(nil))
	}
	tt.lock.RLock()
	values := []T{}
	if tt.root != nil && !value.IsEmpty() {
		lower, lowerBound, hasLower := value.LowerBound()
		upper, upperBound, hasUpper := value.UpperBound()
		stack := []*AvlTreeElement[T]{}
		node := tt.root
		for node != nil {
			include := !hasLower || tt.cmp(node.data, lower) > 0 || lowerBound == ranges.ClosedBound && tt.cmp(node.data, lower) == 0
			if include {
				stack = append(stack, node)
				node = node.left
			} else {
				node = node.right
			}
		}
		for len(stack) > 0 {
			node = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if hasUpper {
				order := tt.cmp(node.data, upper)
				if order > 0 || order == 0 && upperBound == ranges.OpenBound {
					break
				}
			}
			values = append(values, node.data)
			node = node.right
			for node != nil {
				stack = append(stack, node)
				node = node.left
			}
		}
	}
	tt.lock.RUnlock()
	return slices.Values(values)
}
