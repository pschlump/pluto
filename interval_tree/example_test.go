package interval_tree_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/interval_tree"
)

// ExampleIntervalTree books a calendar: a room cannot hold two meetings
// that overlap, so each candidate is checked with AnyOverlap (one
// O(log n) descent) before it is added.
func ExampleIntervalTree() {
	room := interval_tree.NewIntervalTree[int, string]()
	book := func(name string, start, end int) bool {
		if _, _, clash := room.AnyOverlap(interval_tree.Interval[int]{Lo: start, Hi: end - 1}); clash {
			return false // some meeting already occupies part of the slot
		}
		room.Insert(interval_tree.Interval[int]{Lo: start, Hi: end - 1}, name)
		return true
	}

	for _, m := range []struct {
		name       string
		start, end int
	}{
		{"standup", 9, 10},
		{"design", 11, 13},
		{"1:1", 13, 14},
	} {
		fmt.Println(book(m.name, m.start, m.end), m.name)
	}
	// Overlaps standup; refused.
	fmt.Println(book("retro", 9, 11), "retro")

	// Output:
	// true standup
	// true design
	// true 1:1
	// false retro
}

// ExampleIntervalTree_Stab asks "which meetings are running at 12:00?"
// — the stabbing query.
func ExampleIntervalTree_Stab() {
	busy := interval_tree.NewIntervalTree[int, string]()
	busy.Insert(interval_tree.Interval[int]{Lo: 9, Hi: 10}, "standup")
	busy.Insert(interval_tree.Interval[int]{Lo: 11, Hi: 13}, "design")
	busy.Insert(interval_tree.Interval[int]{Lo: 13, Hi: 14}, "1:1")

	for _, meeting := range busy.Stab(12) {
		fmt.Println(meeting)
	}
	// Output:
	// design
}

// ExampleIntervalTree_Overlaps finds every interval intersecting a
// window, ascending by start.
func ExampleIntervalTree_Overlaps() {
	ip := interval_tree.NewIntervalTree[int, string]()
	ip.Insert(interval_tree.Interval[int]{Lo: 10, Hi: 19}, "printer")
	ip.Insert(interval_tree.Interval[int]{Lo: 40, Hi: 47}, "voip-phones")
	ip.Insert(interval_tree.Interval[int]{Lo: 25, Hi: 30}, "thermostat")
	ip.Insert(interval_tree.Interval[int]{Lo: 17, Hi: 22}, "ap")

	for _, device := range ip.Overlaps(interval_tree.Interval[int]{Lo: 15, Hi: 20}) {
		fmt.Println(device)
	}
	// Output:
	// printer
	// ap
}

// ExampleIntervalTree_Dump shows the debugging dump.
func ExampleIntervalTree_Dump() {
	tt := interval_tree.NewIntervalTree[int, string]()
	tt.Insert(interval_tree.Interval[int]{Lo: 15, Hi: 18}, "a")
	tt.Insert(interval_tree.Interval[int]{Lo: 17, Hi: 19}, "b")
	tt.Insert(interval_tree.Interval[int]{Lo: 30, Hi: 31}, "c")
	var sb strings.Builder
	tt.Dump(&sb)
	fmt.Print(sb.String())
	// Output:
	// IntervalTree length=3 depth=2
	//   15-18 max=18 a(R)
	// 17-19 max=31 b(B)
	//   30-31 max=31 c(R)
}
