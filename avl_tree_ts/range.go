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
		var stack [maxAvlHeight]*AvlTreeElement[T] // fixed size: no allocation
		sp := 0
		node := tt.root
		for node != nil {
			include := !hasLower || tt.cmp(node.data, lower) > 0 || lowerBound == ranges.ClosedBound && tt.cmp(node.data, lower) == 0
			if include {
				stack[sp] = node
				sp++
				node = node.left
			} else {
				node = node.right
			}
		}
		for sp > 0 {
			sp--
			node = stack[sp]
			if hasUpper {
				order := tt.cmp(node.data, upper)
				if order > 0 || order == 0 && upperBound == ranges.OpenBound {
					break
				}
			}
			values = append(values, node.data)
			for node = node.right; node != nil; node = node.left {
				stack[sp] = node
				sp++
			}
		}
	}
	tt.lock.RUnlock()
	return slices.Values(values)
}
