package interval_tree_ts_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/interval_tree_ts"
)

// ExampleIntervalTree books a shared calendar from many goroutines: the
// overlap check and the insert run as ONE atomic compound under
// Lock/NlAnyOverlap/NlInsert — the package's canonical compound.  Run
// under -race in the test suite, the example doubles as a smoke test.
func ExampleIntervalTree() {
	room := interval_tree_ts.NewIntervalTree[int, string]()

	book := func(name string, start, end int) bool {
		room.Lock() // hold the lock across check + insert: atomic
		defer room.Unlock()
		slot := interval_tree_ts.Interval[int]{Lo: start, Hi: end}
		if _, _, clash := room.NlAnyOverlap(slot); clash {
			return false
		}
		room.NlInsert(slot, name)
		return true
	}

	fmt.Println(book("standup", 9, 10), "standup")
	fmt.Println(book("design", 11, 13), "design")

	// A second worker trying the morning: refused without ever racing
	// the first booking.
	fmt.Println(book("retro", 9, 11), "retro")

	// Output:
	// true standup
	// true design
	// false retro
}

// ExampleIntervalTree_Stab is the stabbing query over the guarded tree
// (an eager snapshot — safe to mutate from inside the loop).
func ExampleIntervalTree_Stab() {
	busy := interval_tree_ts.NewIntervalTree[int, string]()
	busy.Insert(interval_tree_ts.Interval[int]{Lo: 9, Hi: 10}, "standup")
	busy.Insert(interval_tree_ts.Interval[int]{Lo: 11, Hi: 13}, "design")
	busy.Insert(interval_tree_ts.Interval[int]{Lo: 13, Hi: 14}, "1:1")

	for _, meeting := range busy.Stab(13) {
		fmt.Println(meeting)
	}
	// Output:
	// design
	// 1:1
}
