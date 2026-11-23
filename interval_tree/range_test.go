/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package interval_tree

import (
	"errors"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeAdapters(t *testing.T) {
	tree := NewIntervalTree[int, string]()
	if err := tree.InsertRange(ranges.Closed(1, 3), "a"); err != nil {
		t.Fatal(err)
	}
	if err := tree.InsertRange(ranges.Open(1, 3), "bad"); !errors.Is(err, ErrUnsupportedRange) {
		t.Fatal(err)
	}
	if err := tree.InsertRange(ranges.AtLeast(1), "bad"); !errors.Is(err, ErrUnsupportedRange) {
		t.Fatal(err)
	}
	sequence, err := tree.OverlapsRange(ranges.Closed(3, 4))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for interval, value := range sequence {
		if interval.Lo != 1 || interval.Hi != 3 || value != "a" {
			t.Fatal(interval, value)
		}
		count++
	}
	if count != 1 {
		t.Fatal(count)
	}
}
