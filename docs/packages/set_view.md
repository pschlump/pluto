# set_view

`SetView[T]` provides live lazy `Union`, `Intersection`, `Difference`, and
`SymmetricDifference` over the common `Len`/`Contains`/`All` surface of Pluto's
plain and thread-safe sets. Views have no mutation API and reflect later operand
changes. Contains uses the defining logical expression. All suppresses duplicate
values through operand Contains, uses no auxiliary set, and propagates early
termination. Len counts a fresh iteration. Plain operands must not be mutated
concurrently; `_ts` operands snapshot their individual iterators, but a
multi-operand view is not an atomic cross-set snapshot.

`MutableView` adds explicit write-through behavior. `Wrap` exposes the whole
mutable set; `Filter` exposes only values accepted by a predicate. Insert writes
to the parent and returns `ErrOutsideView` for rejected values. Hidden values
cannot be deleted through a filtered view, and Truncate removes only visible
values. Parent mutations are visible to subsequent reads. The interface accepts
both `hash_set` and `hash_set_ts`; plain sets require external synchronization,
while twin-backed views lock each underlying operation. Algebraic SetView values
remain read-only because mutation of a union/intersection/difference would be
ambiguous.
