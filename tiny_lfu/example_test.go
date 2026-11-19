/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_test

import (
	"fmt"

	"github.com/pschlump/pluto/tiny_lfu"
)

func ExampleTinyLfu() {
	cache := tiny_lfu.NewTinyLfu[string, int](100)
	cache.Put("answer", 42)
	value, ok := cache.Get("answer")
	fmt.Println(value, ok, cache.Frequency("answer"))
	// Output: 42 true 2
}

func ExampleFrequencySketch() {
	sketch := tiny_lfu.NewFrequencySketch[string](100)
	sketch.Increment("popular")
	sketch.Increment("popular")
	fmt.Println(sketch.Frequency("popular"))
	// Output: 2
}
