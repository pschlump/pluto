/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package skip_list_ts

import (
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the largest element strictly less than key.
func (tt *SkipList[T]) Lower(key T) (result T, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
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
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
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

// RangeOf returns a snapshot with global ranks over an open/closed/unbounded range.
func (tt *SkipList[T]) RangeOf(value ranges.Range[T]) iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {}
	}
	tt.lock.RLock()
	items := []rangeEntry[T]{}
	if !tt.isEmpty() && !value.IsEmpty() {
		lower, lowerBound, hasLower := value.LowerBound()
		upper, upperBound, hasUpper := value.UpperBound()
		rank := 0
		cur := tt.head
		for level := tt.level - 1; level >= 0; level-- {
			for cur.forward[level] != nil {
				if !hasLower {
					break
				}
				order := tt.cmp(cur.forward[level].data, lower)
				if order < 0 || order == 0 && lowerBound == ranges.OpenBound {
					rank += cur.span[level]
					cur = cur.forward[level]
				} else {
					break
				}
			}
		}
		for node := cur.forward[0]; node != nil; node = node.forward[0] {
			if hasUpper {
				order := tt.cmp(node.data, upper)
				if order > 0 || order == 0 && upperBound == ranges.OpenBound {
					break
				}
			}
			items = append(items, rangeEntry[T]{rank: rank, data: node.data})
			rank++
		}
	}
	tt.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for _, item := range items {
			if !yield(item.rank, item.data) {
				return
			}
		}
	}
}
