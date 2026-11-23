/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package skip_list

import (
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the largest element strictly less than key.
func (tt *SkipList[T]) Lower(key T) (result T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}
	cur := tt.head
	for level := tt.level - 1; level >= 0; level-- {
		for cur.forward[level] != nil && tt.cmp(cur.forward[level].data, key) < 0 {
			cur = cur.forward[level]
		}
	}
	if cur != tt.head {
		return cur.data, true
	}
	return
}

// Higher returns the smallest element strictly greater than key.
func (tt *SkipList[T]) Higher(key T) (result T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}
	cur := tt.head
	for level := tt.level - 1; level >= 0; level-- {
		for cur.forward[level] != nil && tt.cmp(cur.forward[level].data, key) <= 0 {
			cur = cur.forward[level]
		}
	}
	if cur.forward[0] != nil {
		return cur.forward[0].data, true
	}
	return
}

func (tt *SkipList[T]) firstBound(value T, inclusive bool) (*SkipListNode[T], int) {
	rank := 0
	cur := tt.head
	for level := tt.level - 1; level >= 0; level-- {
		for cur.forward[level] != nil {
			order := tt.cmp(cur.forward[level].data, value)
			if order < 0 || order == 0 && !inclusive {
				rank += cur.span[level]
				cur = cur.forward[level]
			} else {
				break
			}
		}
	}
	return cur.forward[0], rank
}

// RangeOf iterates over an open/closed/unbounded range with global ranks.
// Complexity is O(log n + results) expected.
func (tt *SkipList[T]) RangeOf(value ranges.Range[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if tt == nil || tt.IsEmpty() || value.IsEmpty() {
			return
		}
		lower, lowerBound, hasLower := value.LowerBound()
		upper, upperBound, hasUpper := value.UpperBound()
		cur, rank := tt.head.forward[0], 0
		if hasLower {
			cur, rank = tt.firstBound(lower, lowerBound == ranges.ClosedBound)
		}
		for cur != nil {
			if hasUpper {
				order := tt.cmp(cur.data, upper)
				if order > 0 || order == 0 && upperBound == ranges.OpenBound {
					return
				}
			}
			if !yield(rank, cur.data) {
				return
			}
			rank++
			cur = cur.forward[0]
		}
	}
}
