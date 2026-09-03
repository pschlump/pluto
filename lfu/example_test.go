/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pschlump/pluto/lfu"
)

// ExampleLfu tracks access frequency for an eviction policy, the way
// Ultima backs allkeys-lfu.  A fixed clock and logFactor 0 keep the
// example deterministic (every increment lands, no decay periods
// elapse); real use passes lfu.DefaultLogFactor and lfu.DefaultDecayTime.
func ExampleLfu() {
	minute := int64(10_000)
	l := lfu.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(minute*60, 0)
	})

	for range 3 {
		l.Touch("hot") // 6, 7, 8
	}
	l.Touch("cold") // 6

	hot, _ := l.Counter("hot")
	cold, _ := l.Counter("cold")
	fmt.Println(hot, cold, l.Len())
	// Output: 8 6 2
}

// ExampleLfu_decay shows the time-based halflife: a key idle for
// minutes loses one counter step per decay period (the default is one
// minute), while a fresh key starts at InitVal = 5.
func ExampleLfu_decay() {
	minute := int64(2000)
	l := lfu.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(minute*60, 0)
	})

	l.Touch("k") // 6
	minute += 10 // ten idle minutes
	v, _ := l.Counter("k")
	fmt.Println(v)
	// Output: 0
}

// ExampleLfu_idle shows IdleMinutes — the same signal OBJECT IDLETIME
// reports (elapsed minutes since the last access, independent of decay).
func ExampleLfu_idle() {
	minute := int64(3000)
	l := lfu.NewLfuWithClock[int](10, 1, func() time.Time {
		return time.Unix(minute*60, 0)
	})

	l.Touch(42)
	minute += 7
	idle, ok := l.IdleMinutes(42)
	fmt.Println(idle, ok)
	// Output: 7 true
}

// ExampleMorrisCounter uses the standalone counter at Redis's default
// settings.  The value is probabilistic at a real logFactor; at 0 every
// increment is certain, which makes a stable example: 250 increments
// from InitVal saturate the 8-bit register.
func ExampleMorrisCounter() {
	c := lfu.NewMorrisCounter() // 5, the Redis fresh-key value
	for range 250 {
		c.Incr(0)
	}
	fmt.Println(c.Value())
	c.Decay(100)
	fmt.Println(c.Value())
	// Output: 255
	// 155
}

// ExampleLfu_MarshalJSON encodes the table as a JSON array of key/state
// objects — the raw Morris counter and the 16-bit last-access minute per
// key.  A fixed clock keeps the example deterministic; with more than
// one key the entry order is the backing table's bucket order, which
// varies from process to process.
func ExampleLfu_MarshalJSON() {
	l := lfu.NewLfuWithClock[string](0, 1, func() time.Time {
		return time.Unix(1000*60, 0)
	})
	l.Touch("a") // counter 6 at minute 1000

	b, err := json.Marshal(l)
	fmt.Println(string(b), err)
	// Output:
	// [{"key":"a","counter":6,"lastMin":1000}] <nil>
}

// ExampleLfu_UnmarshalJSON replaces the contents of the table from a
// JSON array of key/state objects, restoring the exact counters and
// last-access minutes — here a key that was last touched 10 idle minutes
// ago reports its decay-adjusted counter.
func ExampleLfu_UnmarshalJSON() {
	l := lfu.NewLfuWithClock[string](0, 1, func() time.Time {
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
