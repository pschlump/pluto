# evicting_queue / evicting_queue_ts

## Public contract and complexity

`Queue[T]` is a fixed-capacity FIFO ring buffer. `NewQueue(capacity)` panics for
negative capacity. Capacity zero is valid: every write succeeds and retains
nothing. A full `Push` evicts the oldest value. `Pop` and `Peek` return
`(value, true)` or `(zero, false)` when empty. `Len` counts retained values and
`RemainingCapacity` is capacity minus length.

| Operation | Complexity |
|---|---|
| Push / Pop / Peek | O(1) |
| Len / Capacity / RemainingCapacity | O(1) |
| PushAll of m values | O(min(m, capacity)) when m >= capacity; otherwise O(m) |
| Slice / All / Clone / Clear | O(n) |
| Equal | O(n), plus element comparison |

`PushAll` appends in argument order and retains exactly the newest values that
fit. For an input at least as large as capacity, it directly copies the final
`capacity` values. `Clear`, pop, eviction, and replacement release obsolete
slots so referenced payloads can be collected.

`Slice` returns a fresh oldest-to-newest slice. Plain `All` walks in that order,
stops immediately when the consumer stops, and must not overlap mutation.
`Clone` preserves capacity and contents without sharing storage. `Equal` is
sequence equality, ignores capacity, and uses `==`; dynamic element values must
be comparable. Nil and zero-value queues support empty reads, but constructors
are required before nonempty writes.

## JSON and thread safety

JSON is an array in logical order. Unmarshal decodes the entire array before
mutation, clears prior contents, and applies normal eviction, so oversized input
retains its newest values. Decode failure leaves the original queue unchanged.

`evicting_queue_ts` wraps the same implementation with `sync.RWMutex`. Bulk push
and JSON replacement are atomic. `All`, `Slice`, `Clone`, and JSON encoding use
snapshots; caller iteration and JSON methods execute outside internal locks.
`Equal` takes independent snapshots and cannot deadlock with an opposite-way
comparison.

Seeded tests compare zero/one/general capacities against a slice model through
push, pop, bulk push, clear, and wraparound. Tests also cover stale slots, clone
independence, JSON, early stop, concurrent operations, and opposite equality.
The pinned Guava oracle covers capacity 0, 1, and 3. Matched Go/JMH benchmarks
compare the ring with Guava EvictingQueue and manual Pluto dqueue/ArrayDeque
eviction; raw results are under `docs/benchmarks/evicting_queue/`.
