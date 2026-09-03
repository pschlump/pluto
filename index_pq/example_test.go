/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package index_pq_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pschlump/pluto/index_pq"
)

// An indexed priority queue over the indices 0..n-1: the client owns the
// indices, the queue orders the values.  Pops drain in ascending value
// order, and the index tells you which slot each value came from.
func Example() {
	q := index_pq.NewIndexPQ[int](4)
	q.Insert(0, 30)
	q.Insert(1, 10)
	q.Insert(2, 50)
	q.Insert(3, 20)

	for !q.IsEmpty() {
		k, v, _ := q.Pop()
		fmt.Printf("index %d value %d\n", k, v)
	}
	// Output:
	// index 1 value 10
	// index 3 value 20
	// index 0 value 30
	// index 2 value 50
}

// Naturally ordered values — integers, floats, strings — need no
// comparison function at all.
func ExampleNewIndexPQ() {
	q := index_pq.NewIndexPQ[string](3)
	q.Insert(0, "pear")
	q.Insert(1, "fig")
	q.Insert(2, "apple")

	fmt.Println(q.Len(), q.IsEmpty())
	k, v, _ := q.Peek()
	fmt.Println(k, v)

	if q.Contains(2) {
		fmt.Println("2 is in the queue")
	}
	if _, found := q.Value(0); found {
		q.Delete(0)
	}
	fmt.Println(q.Len())
	// Output:
	// 3 false
	// 2 apple
	// 2 is in the queue
	// 2
}

// Change is the decrease-key operation: replacing the value of an index
// already in the queue re-orders the heap in O(log n) — the operation
// Dijkstra's shortest-path algorithm is built on.
func ExampleIndexPQ_Change() {
	// Distances from the source in a tiny graph: index = vertex.
	dist := index_pq.NewIndexPQ[int](4)
	dist.Insert(0, 0) // source
	dist.Insert(1, 8) // tentative distances
	dist.Insert(2, 5)
	dist.Insert(3, 12)

	// Relax an edge: vertex 3 is now reachable in 7.
	dist.Change(3, 7)

	for !dist.IsEmpty() {
		vertex, d, _ := dist.Pop()
		fmt.Printf("vertex %d distance %d\n", vertex, d)
	}
	// Output:
	// vertex 0 distance 0
	// vertex 2 distance 5
	// vertex 3 distance 7
	// vertex 1 distance 8
}

// A reversed comparison function turns the min-first queue into a
// max-first queue.
func ExampleNewIndexPQFunc() {
	q := index_pq.NewIndexPQFunc(4, func(a, b int) int {
		return -index_pq.Compare(a, b) // reversed: maximum first
	})
	q.Insert(0, 3)
	q.Insert(1, 9)
	q.Insert(2, 1)
	q.Insert(3, 7)

	k, v, _ := q.Peek()
	fmt.Println(k, v)
	// Output:
	// 1 9
}

// All yields the (index, value) pairs in priority order, minimum first.
// It drains a private snapshot, so the queue is unchanged afterwards.
func ExampleIndexPQ_All() {
	q := index_pq.NewIndexPQ[int](4)
	q.Insert(0, 40)
	q.Insert(1, 10)
	q.Insert(2, 30)
	q.Insert(3, 20)

	var pairs []string
	for k, v := range q.All() {
		pairs = append(pairs, fmt.Sprintf("%d:%d", k, v))
	}
	fmt.Println(strings.Join(pairs, " "))
	fmt.Println("still", q.Len(), "in the queue")
	// Output:
	// 1:10 3:20 2:30 0:40
	// still 4 in the queue
}

// MarshalJSON encodes the queue as a JSON array of {"k":index,"v":value}
// objects in priority order, minimum value first.
func ExampleIndexPQ_MarshalJSON() {
	q := index_pq.NewIndexPQ[int](4)
	q.Insert(0, 30)
	q.Insert(1, 10)
	q.Insert(2, 50)
	q.Insert(3, 20)

	b, err := json.Marshal(q)
	fmt.Println(string(b), err)
	// Output:
	// [{"k":1,"v":10},{"k":3,"v":20},{"k":0,"v":30},{"k":2,"v":50}] <nil>
}

// UnmarshalJSON replaces the contents of the queue from a JSON array of
// {"k":index,"v":value} objects; the document order does not matter —
// the queue re-orders by value.
func ExampleIndexPQ_UnmarshalJSON() {
	q := index_pq.NewIndexPQ[string](3)
	if err := json.Unmarshal([]byte(`[{"k":0,"v":"pear"},{"k":1,"v":"fig"},{"k":2,"v":"apple"}]`), q); err != nil {
		fmt.Println("error:", err)
		return
	}
	for k, v := range q.All() {
		fmt.Println(k, v)
	}
	// Output:
	// 2 apple
	// 1 fig
	// 0 pear
}
