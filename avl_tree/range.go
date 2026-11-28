/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package avl_tree

import (
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the largest element strictly less than key.
func (tt *AvlTree[T]) Lower(key T) (T, bool) {
	var result T
	found := false
	if tt == nil {
		return result, false
	}
	for node := tt.root; node != nil; {
		if tt.compare(node.data, key) < 0 {
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
	for node := tt.root; node != nil; {
		if tt.compare(node.data, key) > 0 {
			result, found = node.data, true
			node = node.left
		} else {
			node = node.right
		}
	}
	return result, found
}

// RangeOf iterates in ascending order over an open/closed/unbounded range. The
// initial descent costs O(log n), and the complete walk costs O(log n + results).
// The tree must not be modified during iteration.
func (tt *AvlTree[T]) RangeOf(value ranges.Range[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt == nil || tt.root == nil || value.IsEmpty() {
			return
		}
		lower, lowerBound, hasLower := value.LowerBound()
		upper, upperBound, hasUpper := value.UpperBound()
		var stack [maxAvlHeight]*AvlTreeElement[T] // fixed size: no allocation
		sp := 0
		node := tt.root
		for node != nil {
			include := !hasLower || tt.compare(node.data, lower) > 0 || lowerBound == ranges.ClosedBound && tt.compare(node.data, lower) == 0
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
				order := tt.compare(node.data, upper)
				if order > 0 || order == 0 && upperBound == ranges.OpenBound {
					return
				}
			}
			if !yield(node.data) {
				return
			}
			for node = node.right; node != nil; node = node.left {
				stack[sp] = node
				sp++
			}
		}
	}
}
