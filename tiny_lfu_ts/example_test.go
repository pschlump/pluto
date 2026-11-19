/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/pluto/tiny_lfu_ts"
)

func ExampleTinyLfu() {
	cache := tiny_lfu_ts.NewTinyLfu[string, int](100)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Put("answer", 42)
		}()
	}
	wg.Wait()
	value, ok := cache.Get("answer")
	fmt.Println(value, ok)
	// Output: 42 true
}

func ExampleTinyLfu_Lock() {
	cache := tiny_lfu_ts.NewTinyLfu[string, int](100)
	cache.Lock()
	if _, ok := cache.NlPeek("answer"); !ok {
		cache.NlPut("answer", 42)
	}
	value, ok := cache.NlPeek("answer")
	cache.Unlock()
	fmt.Println(value, ok)
	// Output: 42 true
}

func ExampleFrequencySketch() {
	sketch := tiny_lfu_ts.NewFrequencySketch[string](100)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				sketch.Increment("popular")
			}
		}()
	}
	wg.Wait()
	fmt.Println(sketch.Frequency("popular"))
	// Output: 15
}
