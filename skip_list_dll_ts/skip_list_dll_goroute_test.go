package skip_list_dll_ts_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/skip_list_dll_ts"
)

type TestDemo struct {
	S string
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestDemo)(nil)

func (aa TestDemo) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestDemo); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*TestDemo); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return 0
}

// TestSkipListGoroutines hammers a shared list with concurrent inserters,
// searchers, deleters and iterators.  It is meant to be run under the race
// detector: `go test -race`.
func TestSkipListGoroutines(t *testing.T) {

	var List1 skip_list_dll_ts.SkipList[TestDemo]

	var wg sync.WaitGroup

	// Writers: insert disjoint key ranges.
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				List1.Insert(TestDemo{S: fmt.Sprintf("%d-%06d", w, i)})
			}
		}(w)
	}

	// Readers: search, scan min/max, and iterate in both directions.  The
	// iterator bodies only break, so they never call back into the list
	// while the read lock is held.
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				List1.Search(TestDemo{S: fmt.Sprintf("%d-%06d", r, i)})
				List1.FindMin()
				List1.FindMax()
				List1.Length()
				for range List1.All() {
					break
				}
				for range List1.Backward() {
					break
				}
			}
		}(r)
	}

	// Deleters.
	for d := 0; d < 4; d++ {
		wg.Add(1)
		go func(d int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				List1.Delete(TestDemo{S: fmt.Sprintf("%d-%06d", d, i)})
			}
		}(d)
	}

	wg.Wait()

	// The list must still be in a consistent, ordered state in both
	// directions.
	prev := ""
	for v := range List1.All() {
		if prev != "" && v.S < prev {
			t.Fatalf("List out of order after concurrent use: %q before %q", v.S, prev)
		}
		prev = v.S
	}
	prev = ""
	for v := range List1.Backward() {
		if prev != "" && v.S > prev {
			t.Fatalf("List out of order after concurrent use: %q before %q", v.S, prev)
		}
		prev = v.S
	}
	if List1.Length() < 0 {
		t.Errorf("Length should never be negative, got %d", List1.Length())
	}
}
