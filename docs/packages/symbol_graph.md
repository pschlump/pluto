# symbol_graph notes

String vertex names over the integer-vertex `graph`/`digraph` packages —
algs4 §4.1 `SymbolGraph` + §4.2 `SymbolDigraph`, both types in one
package (the "query objects in the same package" precedent).  One new
idea, zero new algorithms: a name→index map built in algs4's two passes,
then the plain graph packages do everything.

## Structure

- `SymbolGraph{st map[string]int; keys []string; g *graph.Graph}` — and
  the digraph twin structurally identical.  `st` and `keys` are inverse
  maps; `g` is the live built structure.
- `firstPass` assigns indices 0,1,2,… at first appearance across all
  fields of all lines; `secondPass` connects field 0 of each line to
  every other field of that line (`addEdge` is the underlying package's
  `AddEdge`, whose out-of-range `false` is unreachable — indices come
  from `st`).  Splitting happens twice (once per pass) — lines are short
  and the algs4 shape does the same; do not "optimize" into one pass
  without preserving first-appearance index order.
- `fields` maps a blank line to no fields: `strings.Split("", d)` is
  `[""]`, which would otherwise name an empty-string vertex.
- `readLines` (bufio.Scanner, buffer raised to 1 MiB) exists because the
  two passes need the input twice; an `io.Reader` cannot be rewound.

## Line semantics (documented contract, pinned by TestLineShapes)

- `"A B A"` — field 0 connects to each other field: edge A–B plus the
  self loop A–A (twice in `Degree(A)`, once in `E` — the graph
  package's accounting).
- `"A B"` twice — parallel edges preserved.
- `"solo"` — a vertex with no edges.  `""` — nothing.
- Adjacent delimiters (`"X,,Y"` on `","`) produce empty-string names;
  Java's `split` does the same.  Not sanitized — the delimiter is the
  caller's responsibility.
- The digraph form: field 0 points AT each other field (never the
  reverse) — `InDegree` of a pure head field is 0.

## Panic / error contract

One panic: empty delimiter at construction (either constructor) —
`strings.Split` with `""` would silently split into single characters.
The reader constructors return errors instead (nil reader, read
failure, >1 MiB line).  Methods on nil `*SymbolGraph`/`*SymbolDigraph`
read as an empty graph.  `Index` returns `(int, bool)` and `Name`
`(string, bool)` — never −1 sentinels.

## Facade decisions

- The mapping is frozen at construction; `Graph()`/`Digraph()` hand out
  the live internal structure (as algs4 does) so query objects build on
  it directly and caller-added edges cannot invalidate indices.
- No V()/E()/Adj delegates — algs4's SymbolGraph exposes exactly
  contains/index/name/graph, and thin is the character the backlog
  asks for; unwrap for the rest.
- No JSON layer: the wrapped graph has one, and the mapping is not
  interesting on the wire.
- No `_ts` twin, ever (gauss/maxflow precedent — constructor-built,
  mapping immutable): concurrent reads are safe exactly as far as the
  caller does not mutate, which is the underlying package's own
  contract.  Recorded in twins.md's exception list.

## Test-oracle expectations

- `TestKnownGraph` pins first-appearance order and the field-0-only
  edge rule (`Degree(MCO) == 1` on the routes fixture).
- `TestRandomizedModel`: seed 42, 20 rounds of random lines against a
  reference model (map for the mapping pass, a directly built
  `graph.NewGraph` for the edge pass) comparing V, E, every Degree and
  every sorted adjacency list.
- `TestDegreesOfSeparation` runs the algs4 client end to end: BFS over
  `sg.Graph()`, path translated back to names, cross-checked against
  `DistTo` and `HasEdge`.
- The routes fixture needs the ATL–DEN bridge line or JFK and LAX land
  in different components and the BFS test fails with "no path" — that
  is a fixture bug, not a code bug.

## Benchmark reality

1000 lines × 4 fields over 501 names: build 280 µs / ~340 KB / ~4.4k
allocs; reader form similar plus the scan.  `Index`/`Contains` are
single map ops (~7–11 ns).  Nothing here will ever be the bottleneck of
a client.
