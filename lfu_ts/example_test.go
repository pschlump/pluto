/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/pschlump/pluto/lfu_ts"
)

// ExampleLfu tracks access frequency from many goroutines at once —
// the shared-table use the twin exists for.  A fixed clock and
// logFactor 0 keep the output deterministic.
func ExampleLfu() {
	minute := int64(10_000)
	l := lfu_ts.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(minute*60, 0)
	})
	var wg sync.WaitGroup
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < w+1; i++ {
				l.Touch(fmt.Sprintf("key-%d", w))
			}
		}(w)
	}
	wg.Wait()
	a, _ := l.Counter("key-0")
	b, _ := l.Counter("key-2")
	fmt.Println(a, b, l.Len())
	// Output: 6 8 3
}

// ExampleLfu_Lock demonstrates the compound surface the eviction scan
// wants: a held Lock plus the Nl* methods compare candidate frequencies
// from one instant and act — here evicting the coldest key atomically.
func ExampleLfu_Lock() {
	minute := int64(20_000)
	l := lfu_ts.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(minute*60, 0)
	})
	l.Touch("hot")  // 6
	l.Touch("hot")  // 7
	l.Touch("cold") // 6
	l.Touch("cold") // 7
	minute += 5     // five idle minutes: cold decays before the scan
	l.Touch("hot")

	l.Lock()
	coldest, coldestVal := "", uint8(255)
	for _, k := range []string{"hot", "cold"} {
		if v, ok := l.NlCounter(k); ok && v < coldestVal {
			coldest, coldestVal = k, v
		}
	}
	evicted := l.NlDelete(coldest)
	l.Unlock()

	_, hotOk := l.Counter("hot")
	fmt.Println("evicted:", coldest, evicted, "hot present:", hotOk, "len:", l.Len())
	// Output: evicted: cold true hot present: true len: 1
}
