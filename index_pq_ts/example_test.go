/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package index_pq_ts_test

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/index_pq_ts"
)

// Writers from many goroutines share one queue; each writer owns a
// disjoint stripe of the index space.
func Example() {
	q := index_pq_ts.NewIndexPQ[int](800)

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := w; k < 800; k += 8 {
				q.Insert(k, k) // value == index: drains in index order
			}
		}(w)
	}
	wg.Wait()
	fmt.Println(q.Len())

	// The first three pops are the three smallest values.
	for range 3 {
		k, v, _ := q.Pop()
		fmt.Println(k, v)
	}
	// Output:
	// 800
	// 0 0
	// 1 1
	// 2 2
}

// Lock + the Nl* methods make a multi-step operation atomic — here an
// atomic decrease-key-if-greater (a clamp of one index's value).
func ExampleIndexPQ_Lock() {
	q := index_pq_ts.NewIndexPQ[int](4)
	q.Insert(0, 30)
	q.Insert(1, 10)

	q.Lock()
	if q.NlContains(0) {
		if v, _ := q.NlValue(0); v > 5 {
			q.NlChange(0, 5) // clamp index 0 down to 5, atomically
		}
	}
	q.Unlock()

	k, v, _ := q.Peek()
	fmt.Println(k, v)
	// Output:
	// 0 5
}

// MarshalJSON encodes the queue as a JSON array of {"index":k,"value":v}
// pair objects in priority order, minimum value first.
func ExampleIndexPQ_MarshalJSON() {
	q := index_pq_ts.NewIndexPQ[int](4)
	q.Insert(0, 30)
	q.Insert(1, 10)
	q.Insert(2, 20)

	b, err := json.Marshal(q)
	fmt.Println(string(b), err)
	// Output:
	// [{"index":1,"value":10},{"index":2,"value":20},{"index":0,"value":30}] <nil>
}

// UnmarshalJSON replaces the contents of the queue from a JSON array of
// {"index":k,"value":v} pair objects; the pairs come back out in
// priority order.
func ExampleIndexPQ_UnmarshalJSON() {
	q := index_pq_ts.NewIndexPQ[string](4)
	if err := json.Unmarshal([]byte(`[{"index":0,"value":"c"},{"index":1,"value":"a"}]`), q); err != nil {
		fmt.Println("error:", err)
		return
	}
	for k, v := range q.All() {
		fmt.Println(k, v)
	}
	// Output:
	// 1 a
	// 0 c
}
