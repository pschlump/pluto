/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts_test

import (
	"encoding/json"
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

// ExampleLfu_MarshalJSON encodes the table as a JSON array of key/state
// objects — the raw Morris counter and the 16-bit last-access minute per
// key — in first-insert order (the ts twin's enumeration is
// deterministic, unlike the plain twin's bucket order).  A fixed clock
// keeps the example deterministic.
func ExampleLfu_MarshalJSON() {
	l := lfu_ts.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(1000*60, 0)
	})
	l.Touch("a") // counter 6 at minute 1000
	l.Touch("b")
	l.Touch("b") // counter 7

	b, err := json.Marshal(l)
	fmt.Println(string(b), err)
	// Output:
	// [{"key":"a","counter":6,"lastMin":1000},{"key":"b","counter":7,"lastMin":1000}] <nil>
}

// ExampleLfu_UnmarshalJSON replaces the contents of the table from a
// JSON array of key/state objects, restoring the exact counters and
// last-access minutes — here a key that was last touched 10 idle minutes
// ago reports its decay-adjusted counter.
func ExampleLfu_UnmarshalJSON() {
	l := lfu_ts.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(1000*60, 0)
	})
	if err := json.Unmarshal([]byte(`[{"key":"a","counter":42,"lastMin":990}]`), l); err != nil {
		fmt.Println("error:", err)
		return
	}
	v, ok := l.Counter("a") // 42 - 10 idle minutes at decayTime 1
	idle, _ := l.IdleMinutes("a")
	fmt.Println(v, ok, idle)
	// Output:
	// 32 true 10
}
