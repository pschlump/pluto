/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package interval_tree_ts

import (
	"iter"

	"github.com/pschlump/pluto/interval_tree"
	"github.com/pschlump/pluto/ranges"
)

// InsertRange inserts a closed bounded range using the plain adapter.
func (tt *IntervalTree[K, V]) InsertRange(value ranges.Range[K], item V) error {
	interval, err := interval_tree.IntervalFromRange(value)
	if err != nil {
		return err
	}
	tt.Insert(Interval[K]{Lo: interval.Lo, Hi: interval.Hi}, item)
	return nil
}

// OverlapsRange returns a snapshot overlap iterator for a closed bounded range.
func (tt *IntervalTree[K, V]) OverlapsRange(value ranges.Range[K]) (iter.Seq2[Interval[K], V], error) {
	interval, err := interval_tree.IntervalFromRange(value)
	if err != nil {
		return nil, err
	}
	return tt.Overlaps(Interval[K]{Lo: interval.Lo, Hi: interval.Hi}), nil
}
