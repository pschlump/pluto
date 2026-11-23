# set_view

`SetView[T]` provides live lazy `Union`, `Intersection`, `Difference`, and
`SymmetricDifference` over the common `Len`/`Contains`/`All` surface of Pluto's
plain and thread-safe sets. Views have no mutation API and reflect later operand
changes. Contains uses the defining logical expression. All suppresses duplicate
values through operand Contains, uses no auxiliary set, and propagates early
termination. Len counts a fresh iteration. Plain operands must not be mutated
concurrently; `_ts` operands snapshot their individual iterators, but a
multi-operand view is not an atomic cross-set snapshot.
