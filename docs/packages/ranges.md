# ranges

## Range contract

`Range[T]` stores optional endpoints, open/closed bound types, and a comparator.
Natural constructors accept `cmp.Ordered`; every constructor has a Func form.
`Open`, `Closed`, `ClosedOpen`, and `OpenClosed` reject reversed bounds. `[a,a]`
is a singleton, `[a,a)` and `(a,a]` are empty, and public `(a,a)` construction is
invalid. Intersection of touching open endpoints can internally represent an
empty `(a,a)`, matching Guava behavior.

`LowerBound`/`UpperBound` return endpoint, bound type, and presence. `Contains`,
`ContainsAll`, `IsEmpty`, `Encloses`, `IsConnected`, `Intersection`, `Span`, and
`Equal` are O(1) comparator operations. Intersection panics for disconnected
ranges. Span includes gaps. Unbounded endpoints are explicit, never zero-value
sentinels.

## Discrete domains and contiguous sets

`SignedDomain` and `UnsignedDomain` cover standard integer widths. Next/Prev
return false at extrema; Distance rejects reversed inputs and reports an exact
unsigned distance, including MinInt64-to-MaxInt64. Custom domains bundle checked
functions and optional finite Min/Max.

`Canonical` uses Guava's closed-open convention: open lower endpoints advance,
and closed upper endpoints advance. At a maximum closed upper endpoint, the
closed bound remains because no successor exists. An exhausted open lower bound
becomes empty.

`ContiguousSet` uses domain extrema for unbounded inputs, remains lazy, and
supports `Contains`, `Rank`, exact `Len` with overflow reporting, `Range`, and
early-stoppable `All`. A 2^64-element unsigned domain reports size overflow.
Tests cover endpoint matrices, limits, custom comparators, canonicalization,
lazy early stop, and the pinned Guava differential golden.
