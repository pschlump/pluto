/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package interval_tree_ts

import (
	"errors"
	"testing"

	"github.com/pschlump/pluto/interval_tree"
	"github.com/pschlump/pluto/ranges"
)

func TestRangeAdapters(t *testing.T) {
	tree := NewIntervalTree[int, string]()
	if err := tree.InsertRange(ranges.Closed(1, 3), "a"); err != nil {
		t.Fatal(err)
	}
	if err := tree.InsertRange(ranges.Open(1, 3), "bad"); !errors.Is(err, interval_tree.ErrUnsupportedRange) {
		t.Fatal(err)
	}
	sequence, err := tree.OverlapsRange(ranges.Closed(3, 4))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range sequence {
		count++
	}
	if count != 1 {
		t.Fatal(count)
	}
}
