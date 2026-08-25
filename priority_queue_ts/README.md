# priority_queue_ts

A **thread-safe** generic priority queue for Go (1.23+), the locking twin of
[`priority_queue`](../priority_queue/). It is backed by the thread-safe
min-heap in [`heap_ts`](../heap_ts/) and has the exact same API as
`priority_queue`. Elements are stored as `*T` where `T` implements
`comparable.Comparable`; `Pop`/`Peek` always return the minimum element
according to `Compare`.

## Differences from `priority_queue`

- Every operation is guarded by the `sync.RWMutex` inside `heap_ts`.
- `All()` builds its private copy from a snapshot taken under the write lock
  — non-destructive, safe to call queue methods from inside the loop body;
  changes made after the snapshot are not visible to the iteration.
- `Lock`/`Unlock` expose the write lock for atomic batching (e.g. a
  Search-then-UpdatePriority sequence). While the lock is held, call only
  the `Nl`-prefixed no-lock methods (`NlInsert`, `NlPop`, `NlPeek`,
  `NlSearch`, `NlUpdatePriority`, `NlDelete`, `NlLen`, `NlIsEmpty`); calling
  a regular method will deadlock. The plain `priority_queue` package
  provides no-op `Lock`/`Unlock` and pass-through `Nl*` aliases so the same
  code compiles against either package.
- Pointers returned by `Peek`/`Search`/`Pop` alias data stored in the queue
  — treat them as read-only. Positions returned by `Search` may be
  invalidated by any concurrent mutation; use `Lock`/`Unlock` when a
  position must stay valid across two calls.

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

## Example

```go
pq := priority_queue_ts.NewPriorityQueue[Item]()
pq.Insert(&Item{Name: "banana", Priority: 3})
pq.Insert(&Item{Name: "apple", Priority: 2})

fmt.Println(pq.Peek().Name) // "apple" — the minimum

// Iterate in priority order without draining the queue (Go 1.23+):
for item := range pq.All() {
	fmt.Println(item.Name, item.Priority)
}

// Atomic search-then-update:
pq.Lock()
if _, pos, err := pq.NlSearch(&Item{Priority: 3}); err == nil {
	pq.NlUpdatePriority(pos, &Item{Name: "banana", Priority: 1})
}
pq.Unlock()
```

Tests include a goroutine concurrency test; run `go test -race` (or
`make race` at the repo root) after any change.
