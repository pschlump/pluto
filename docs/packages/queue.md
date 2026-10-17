# queue notes

`Queue[T any]` is the constraint-free container: no ordering, no equality, no constructors — the zero value is fully usable (including `Push`), so the only panic is `Push`/`Enqueue` on a nil queue.  Slice-backed with a moving window: `popHead` zeroes the vacated slot, advances `data = data[1:]`, and nils the slice on full drain (tested by `TestPopReleasesBackingArray`); long grow/drain cycles reallocate — point callers needing stable O(1) memory at `queue_dll` (or `dll`'s `Enqueue`/`Pop`).  `Dequeue`/`Peek` return `(T, error)`.  Error API: `ErrEmptyQueue`.

`queue_ts` adds the `lock sync.RWMutex` and `noLock*` internals; its `All`/`Backward` copy the slice under the read lock when called.

