/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set

import (
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the largest element strictly less than key.
func (ts *TreeSet[T]) Lower(key T) (T, bool) { return ts.innerOf().Lower(key) }

// Higher returns the smallest element strictly greater than key.
func (ts *TreeSet[T]) Higher(key T) (T, bool) { return ts.innerOf().Higher(key) }

// RangeOf iterates over an open/closed/unbounded range in O(log n + results).
func (ts *TreeSet[T]) RangeOf(value ranges.Range[T]) iter.Seq[T] {
	return ts.innerOf().RangeOf(value)
}
