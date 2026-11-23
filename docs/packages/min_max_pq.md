# min_max_pq / min_max_pq_ts

## Public contract and complexity

`MinMaxPriorityQueue[T]` is a single-array min-max heap. Natural constructors
use `cmp.Ordered`; `NewMinMaxPriorityQueueFunc` and
`NewBoundedMinMaxPriorityQueueFunc` accept a three-way comparator and arbitrary
element types. The zero value and nil queue support empty reads, while nonempty
writes require construction. Empty peeks and pops return `(zero, false)`.

| Operation | Complexity |
|---|---|
| PeekMin / PeekMax | O(1) |
| Push / PopMin / PopMax | O(log n) |
| Len / IsEmpty / MaximumSize | O(1) |
| PushAll of m values | O(m log n) |
| Slice / All / Clone / Clear | O(n) |
| Equal | O(n log n) |

The root is the minimum. For two or three elements the maximum is at index 1 or
2; otherwise it is the greater of those indices. Alternating array levels enforce
minimum and maximum grandparent ordering. Removal restores both parent and
descendant constraints and clears the vacated slot.

Bounded constructors require a positive maximum. When full, insertion retains
the least `maximumSize` values by evicting the greatest old or incoming value.
An incoming value equal to the current maximum is discarded. `MaximumSize`
returns `(bound, true)` or `(0, false)` when unbounded. Duplicate values remain
distinct occurrences.

`Slice` and `All` use internal breadth-first heap order, not sorted order. Drain
with `PopMin` or `PopMax` for ordered output. `Clone` copies the exact layout,
comparator, and bound. `Equal` compares sorted copies using the receiver's
comparator, so layouts and capacities do not matter; nonempty operands must use
consistent comparators.

## JSON and thread safety

JSON is an array in internal heap order. Unmarshal decodes first, builds a valid
replacement with the retained comparator and bound, then installs it. Bounded
JSON input therefore retains its least values. Invalid JSON leaves the original
unchanged.

`min_max_pq_ts` wraps the heap with `sync.RWMutex`. `PushAll` and JSON replacement
are atomic. Iterators, slices, clones, equality, and JSON encoding use snapshots;
caller iterator and JSON methods run outside the lock. Comparators execute under
the operation lock and must not re-enter the same queue. `Equal` snapshots each
operand separately, avoiding nested locks.

Fixed-seed models compare bounded and unbounded operations against sorted
multisets, including duplicate and equal-maximum inserts. A level-aware invariant
checker runs after randomized mutations. Tests cover custom comparison,
non-comparable payloads, both removal ends, clone independence, JSON, early stop,
and concurrent snapshots. Committed Guava goldens cover bounded and unbounded
traces. Matched Go/JMH benchmarks include Guava MinMaxPriorityQueue, a lazy
two-heap alternative, and one-ended normal-heap baselines; raw results are under
`docs/benchmarks/min_max_pq/`. One-ended workloads generally favor a normal heap.
