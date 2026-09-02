/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/pluto/hyperloglog_ts"
)

// ExampleHll counts distinct elements from many goroutines at once —
// the shared-sketch use the twin exists for.  The estimate is
// deterministic (the hash is a frozen constant), so the exact value is
// a stable example output.
func ExampleHll() {
	h := hyperloglog_ts.NewHll()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 12_500; i++ {
				h.Add([]byte(fmt.Sprintf("visitor:%d:%d", w, i)))
			}
		}(w)
	}
	wg.Wait()
	fmt.Println(h.Count())
	// Output: 99773
}

// ExampleHll_Lock demonstrates the compound surface: a held Lock plus
// the Nl* methods make an add-batch-then-count sequence atomic.
func ExampleHll_Lock() {
	h := hyperloglog_ts.NewHll()
	h.Lock()
	for i := 0; i < 5_000; i++ {
		h.NlAdd([]byte(fmt.Sprintf("item-%d", i)))
	}
	n := h.NlCount()
	h.Unlock()
	fmt.Println(n >= 4_900 && n <= 5_100)
	// Output: true
}
