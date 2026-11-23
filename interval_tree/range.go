/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package interval_tree

import (
	"errors"
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// ErrUnsupportedRange reports an open or unbounded range that cannot be
// represented by this package's closed bounded Interval.
var ErrUnsupportedRange = errors.New("interval_tree: range must be closed and bounded")

// IntervalFromRange converts a fully closed bounded range.
func IntervalFromRange[K any](value ranges.Range[K]) (Interval[K], error) {
	lower, lowerBound, hasLower := value.LowerBound()
	upper, upperBound, hasUpper := value.UpperBound()
	if !hasLower || !hasUpper || lowerBound != ranges.ClosedBound || upperBound != ranges.ClosedBound {
		return Interval[K]{}, ErrUnsupportedRange
	}
	return Interval[K]{Lo: lower, Hi: upper}, nil
}

// InsertRange converts and inserts a closed bounded range.
func (tt *IntervalTree[K, V]) InsertRange(value ranges.Range[K], item V) error {
	interval, err := IntervalFromRange(value)
	if err != nil {
		return err
	}
	tt.Insert(interval, item)
	return nil
}

// OverlapsRange converts a query and returns the same overlap iterator as
// Overlaps. This is an adapter, not RangeSet or RangeMap behavior.
func (tt *IntervalTree[K, V]) OverlapsRange(value ranges.Range[K]) (iter.Seq2[Interval[K], V], error) {
	interval, err := IntervalFromRange(value)
	if err != nil {
		return nil, err
	}
	return tt.Overlaps(interval), nil
}
