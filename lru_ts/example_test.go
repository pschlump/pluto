/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package lru_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/pluto/lru_ts"
)

func ExampleLru() {
	c := lru_ts.NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // a hit marks "a" most-recently-used
	c.Put("c", 3) // so "b" is the eviction victim, not "a"
	first := true
	for k, v := range c.All() { // most to least recently used
		if !first {
			fmt.Print(" ")
		}
		first = false
		fmt.Printf("%s=%d", k, v)
	}
	fmt.Println()
	if _, ok := c.Get("b"); !ok {
		fmt.Println("b was evicted")
	}
	// Output:
	// c=3 a=1
	// b was evicted
}

// Peek looks without changing the recency order; Get re-marks.  In the
// twin that split is also the lock split: Peek runs under the read
// lock, Get under the write lock.
func ExampleLru_Peek() {
	c := lru_ts.NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Peek("a")   // no reorder: "a" stays least-recently-used
	c.Put("c", 3) // so "a" is evicted
	_, aOK := c.Get("a")
	fmt.Println("a cached:", aOK)
	// Output:
	// a cached: false
}

// NewLruFunc takes a veto callback: an entry the callback rejects is
// skipped during eviction, and when nothing is evictable the cache
// temporarily exceeds its capacity — the soft cap.  The callback runs
// with the write lock held; it must not call back into the cache.
func ExampleNewLruFunc() {
	c := lru_ts.NewLruFunc(2, func(k string, _ int) bool {
		return k != "pinned" // "pinned" vetoes its own eviction
	})
	c.Put("pinned", 0)
	c.Put("x", 1)
	c.Put("y", 2) // "pinned" is LRU but vetoed: "x" is evicted instead
	c.Put("z", 3) // now "pinned" and "y" — "y" goes
	first := true
	for k := range c.All() {
		if !first {
			fmt.Print(" ")
		}
		first = false
		fmt.Print(k)
	}
	fmt.Println()
	// Output:
	// z pinned
}

// ExampleLru_Lock demonstrates the compound surface: a held Lock plus
// the Nl* methods make a batch of puts and the eviction-to-capacity
// they trigger one atomic operation — no reader observes the middle.
func ExampleLru_Lock() {
	c := lru_ts.NewLru[int, int](4)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				c.Put(w*100+i, i)
			}
		}(w)
	}
	wg.Wait()

	c.Lock()
	for i := 100; i < 110; i++ {
		c.NlPut(i, i)
	}
	n := c.NlLen()
	c.Unlock()

	fmt.Println("len after batch:", n)
	// Output:
	// len after batch: 4
}
