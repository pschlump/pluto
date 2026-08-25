# heap_ts

A **thread-safe** generic min-heap for Go (1.23+), the locking twin of
[`heap`](../heap/). It has the exact same API as `heap`; every operation is
guarded by an internal `sync.RWMutex`. Pop always removes and returns the
minimum element (per `comparable.Comparable.Compare`).

## Differences from `heap`

- Every method takes the internal `sync.RWMutex` (read lock for readers).
- `All()` iterates a **snapshot** taken under the read lock — it is safe to
  call heap methods from inside the loop body; changes made after the
  snapshot are not visible to the iteration.
- `Lock`/`Unlock` expose the write lock for atomic batching. While the lock
  is held, call only the `Nl`-prefixed no-lock methods (`NlPush`, `NlPop`,
  `NlDelete`, `NlFix`, `NlLen`, `NlGetValue`); calling a regular method will
  deadlock. The plain `heap` package provides no-op `Lock`/`Unlock` and
  pass-through `Nl*` aliases so the same code compiles against either
  package.
- Pointers returned by `Peek`/`GetValue`/`Search` alias data stored in the
  heap — treat them as read-only. Indexes returned by `Search` may be
  invalidated by any concurrent mutation.

## Operations and complexity

| Operation    | Description                                                  | Complexity |
|--------------|--------------------------------------------------------------|------------|
| `NewHeap`    | Create an empty heap                                         | O(1)       |
| `Push`       | Add an element                                               | O(log n)   |
| `Pop`        | Remove and return the minimum element (`nil` if empty)       | O(log n)   |
| `Peek`       | Return the minimum element without removing it               | O(1)       |
| `Delete`     | Remove and return the element at an index                    | O(log n)   |
| `Fix`        | Replace the element at an index and restore heap order       | O(log n)   |
| `SetValue`   | Alias for `Fix`                                              | O(log n)   |
| `GetValue`   | Return the element at an index                               | O(1)       |
| `Search`     | Linear scan by value; returns value, index                   | O(n)       |
| `Len`/`Length` | Number of elements                                         | O(1)       |
| `Truncate`   | Remove all elements                                          | O(1)       |
| `AppendHeap` | Bulk append (breaks heap order; rebuild with `Heapify`)      | O(m)       |
| `Heapify`    | Restore heap order of the sub-tree rooted at an index        | O(log n)   |
| `All`        | Range-over-func iterator over a snapshot (internal order)    | O(n)       |

## Example

```go
h := heap_ts.NewHeap[Item]()
h.Push(&Item{Priority: 3})
h.Push(&Item{Priority: 1})
fmt.Println(h.Pop().Priority) // 1 — the minimum

// Atomic batch:
h.Lock()
h.NlPush(&Item{Priority: 2})
_ = h.NlPop()
h.Unlock()
```

Tests include a goroutine concurrency test; run `go test -race` (or
`make race` at the repo root) after any change.
