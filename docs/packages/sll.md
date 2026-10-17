# sll notes

`Sll[T any]` is the lean sibling of dll: stack ops (`Push`/`Pop`/`Peek` at the head), O(1) tail insertion (`InsertAfterTail`), value `Search`/`Delete`, `Reverse`, the cursor `SllIter` (Front/Current, forward only — no `Rear`/`Prev`), and `IterateOver` (`iter.Seq2[int, T]`; the same single-variable-yields-the-index trap as dll).  Error API: `ErrEmptySll`, `ErrNotFound`, with fixed precedence — `Delete` on an empty list returns `ErrNotFound`, `DeleteFound` returns `ErrEmptySll`.  Unlike dll, `DeleteFound` is a **value-based re-search** (O(n) — it removes the first element equal to the found element's data), not a direct splice.  `checkInvariants` in thorough_test.go verifies head/tail/count consistency — call it after structural changes.

`sll_ts` adds the `lock sync.RWMutex` and the lock-free `deleteLocked`; its cursor `SllIter` walks the live list under per-call read locks (like `dll_ts`'s `DllIter`) while `IterateOver` uses an eager snapshot.

