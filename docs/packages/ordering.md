# ordering

`Ordering[T]` wraps a typed three-way comparison function. `Natural` uses
`cmp.Compare`; `FromFunc`, `Reverse`, `Then`, `ByKey`, `Lexicographical`, and
`AllEqual` compose comparators without boxing elements.

`Min`, `Max`, `IsOrdered`, and `IsStrictlyOrdered` consume `iter.Seq` inputs.
Empty extrema return `(zero, false)`. Equal extrema retain the first value.
Ordered checks stop at the first violation.

`SortedCopy` uses stable sorting and leaves its input unchanged.
`LeastOf`/`GreatestOf` maintain a bounded worst-first heap, using O(n log k)
time and O(k) auxiliary space rather than sorting all n values. Results are
sorted least-first or greatest-first respectively. k=0 returns an empty slice,
k>n returns all values, and negative k panics.

Ordering values contain no mutable package state. Concurrent use is safe when
caller-supplied comparison and key functions are themselves safe, so no `_ts`
twin is provided.
