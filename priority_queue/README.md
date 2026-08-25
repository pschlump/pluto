# priority_queue

A generic priority queue for Go (1.23+) backed by a binary min-heap
(`github.com/pschlump/pluto/heap`). Elements are stored as `*T` where `T`
implements `comparable.Comparable`; `Pop`/`Peek` always return the minimum
element according to `Compare`.

## Operations and complexity

| Operation        | Description                                              | Complexity   |
|------------------|----------------------------------------------------------|--------------|
| `NewPriorityQueue` | Create an empty queue                                  | O(1)         |
| `Len`            | Number of elements in the queue                          | O(1)         |
| `IsEmpty`        | True if the queue has no elements                        | O(1)         |
| `Peek`           | Return the minimum element without removing it (`nil` if empty) | O(1) |
| `Insert`         | Add an element                                           | O(log n)     |
| `Pop`            | Remove and return the minimum element (`nil` if empty)   | O(log n)     |
| `Search`         | Find an element by value; returns value, position, error | O(n)         |
| `UpdatePriority` | Replace the element at a position and re-heapify         | O(log n)     |
| `Delete`         | Remove the element at a position                         | O(log n)     |
| `Truncate`       | Remove all elements                                      | O(1)         |
| `All`            | Range-over-func iterator, minimum first, non-destructive | O(n log n)   |

`UpdatePriority` and `Delete` sift in place via `heap.Heap.Fix` /
`heap.Heap.Delete`.

## Example

```go
type Item struct {
	Name     string
	Priority int
}

func (a Item) Compare(x comparable.Comparable) int {
	b := x.(Item)
	return a.Priority - b.Priority
}

pq := priority_queue.NewPriorityQueue[Item]()
pq.Insert(&Item{Name: "banana", Priority: 3})
pq.Insert(&Item{Name: "apple", Priority: 2})

fmt.Println(pq.Peek().Name) // "apple" — the minimum

// Iterate in priority order without draining the queue (Go 1.23+):
for item := range pq.All() {
	fmt.Println(item.Name, item.Priority)
}

for !pq.IsEmpty() {
	item := pq.Pop() // ascending priority order
	_ = item
}
```

Positions used by `UpdatePriority`/`Delete` are those returned by `Search`;
any `Insert`, `Pop`, `UpdatePriority`, or `Delete` can invalidate previously
returned positions.

## Thread safety

`PriorityQueue` is **not** safe for concurrent use. If the queue is shared
between goroutines, protect every call with a `sync.Mutex` (or equivalent)
in the caller.
