# testgraphs notes (internal)

algs4 `GraphGenerator` (§4.1) and `DigraphGenerator` (§4.2) ported as
shared *test* helpers under `internal/` — not a public package, no root
README row, no `_ts` question.  The backlog's stated purpose: "porting a
few generators into shared *test* helpers would harden the randomized
graph tests."  The port went whole-hog (every algs4 generator) plus the
backlog's `Grid`, which algs4 does not have.

## Structure

- `testgraphs.go` — the types and the shared plumbing: `G{V; Edges
  [][2]int}` (undirected generators store mostly-sorted unordered
  pairs, directed ones ordered `v→w` pairs), `B{G; V1}` (contiguous
  bipartite sides), `Labeled{G; Label []int}` (Strong's SCC oracle),
  `requireRand`, `shuffled` (nil = identity), `uKey`, `addEdge` (the
  rejection-sampling step), and a hand-rolled `intHeap` (stdlib only;
  `container/heap` needs an interface — the project's whole point is
  not to have one).
- `graph.go` — undirected: `Simple`, `ErdosRenyi`, `Complete`, `Path`,
  `Cycle`, `Star`, `Wheel`, `BinaryTree`, `Tree` (Prüfer), `Bipartite`,
  `BipartiteProb`, `CompleteBipartite`, `EulerianCycle`,
  `EulerianPath`, `Grid`.
- `digraph.go` — directed, `Digraph`-suffixed where the name collides:
  `SimpleDigraph`, `ErdosRenyiDigraph`, `CompleteDigraph`,
  `PathDigraph`, `CycleDigraph`, `BinaryTreeDigraph`, `DAG`,
  `Tournament`, `RootedInDAG`/`RootedOutDAG` (+`Tree` forms),
  `EulerianCycleDigraph`, `EulerianPathDigraph`, `Strong`.

## The three design rules (why tests anywhere can use this)

1. **Edge lists, never pluto types.**  The package imports only
   `math/rand` and `fmt`, so even *in-package* tests (`package graph`,
   `package digraph`) import it without a cycle.  A test builds its
   graph in three lines: `g := NewGraph(gen.V); for _, e := range
   gen.Edges { g.AddEdge(e[0], e[1]) }`.
2. **Caller-supplied `*rand.Rand`** — tests pass a fixed seed and stay
   deterministic (the repo's seed-42 discipline).  Where algs4's only
   randomness is a label shuffle, `nil` means identity order, so
   `Path(nil, 4)`, `Star(nil, 8)`, `CycleDigraph(nil, v)` … are
   deterministic with no seed at all.  Generators that genuinely need
   randomness (rejection sampling, Prüfer, walks) panic on nil via
   `requireRand`.
3. **Documented deviations that upgrade the output to an oracle** —
   each is noted on the generator itself:
   - `Bipartite`/`BipartiteProb`/`CompleteBipartite` keep the sides
     contiguous (`[0,V1)` vs `[V1,V)`) instead of shuffling labels —
     the 2-coloring is free.
   - `DAG`, `RootedInDAG` (root `v-1`), `RootedOutDAG` (root `0`) use
     the identity order — `0..V-1` is itself a topological order, and
     the roots are the endpoints.
   - `Complete`/`CompleteBipartite` are built directly, not via
     `simple(v, 1.0)` — deterministic, consumes no randomness.
   - `Strong` pins all `c` labels (below).

## Panic contract

Impossible arguments panic with a message naming the generator and the
problem (`e > v·(v-1)/2`, `e < 0`, `v < 1`, `p` outside [0,1],
`Wheel v < 2`, `Strong`'s `1 <= c <= v`, `e <= 2(v-c)`, nil rand where
real randomness is required).  A wrong generator call is a test bug;
failing loudly beats a test that quietly tests nothing.  `internal/`
keeps this surface out of the public API entirely.

## Traps (each one bit during the port)

- **`Strong` label pinning** — algs4 draws every label uniformly, so a
  label can go unused: two "components" silently merge and the
  superimposed-trees phase overshoots the exact edge count (observed: 6
  edges when `e=5`).  The fix pins labels: the first `c` vertices of a
  shuffled order get `0..c-1`, the rest are random.  Now all `c` labels
  are used, the edge count is exactly `e`, and the SCC count is exactly
  `c` — which is what makes it an oracle.
- **`closedWalk` chaining** — chaining only `vertices[:e-1]` loses the
  `v_{e-2}→v_{e-1` edge; chain all `e` picks, then close with
  `[v_{e-1}, v_0]` (`e` edges total, even degree everywhere).
- **`walkEdges`' V** — the walk's vertex count is `max(picks)+1`, not
  the caller's `v`: untouched vertices are isolated, which is legal and
  exactly the `TestEulerianIsolatedVertices` shape.
- **Map iteration in consuming tests** — `DAG`/`Rooted*DAG` draw from
  the shared `rng`; iterating them through a `map[string]G` randomizes
  draw order and breaks test determinism.  Use a slice of cases
  (`digraph/generated_test.go` does).
- **v=2 corner for Strong** — one 2-vertex strong component needs both
  directed edges (`2 = 2(v-c)` with `c=1`) but `hi = v(v-1)/2 = 1`:
  impossible, hence the panic bounds; `v=2` forces `c=2`.  Consuming
  tests must skip `lo+1 > hi` shapes (`TestSCCStrongLabels` does).

## Oracles (this package's own tests + the consumers)

- Own tests (`testgraphs_test.go`): simple-form checks (no self loops,
  no parallel edges, edge count exact), same-seed determinism, shape
  checks (identity Path, 2-regular Cycle, Star hub, Grid edge count +
  `(i+j)%2` bipartite), Tree connectivity, Eulerian degree parity +
  walked-part connectivity, DAG `x<y`, Tournament one-edge-per-pair,
  rooted-DAG reachability (boolean closure), directed walk in/out
  balance, Strong's strongly-connected ⟺ equal-labels with exactly `c`
  distinct labels, and ~25 exact panic-message checks.
- Consumers (the hardening half of the backlog item):
  `graph/generated_test.go` — Eulerian queries on guaranteed-positive
  inputs (the randomized oracle tests usually see graphs with *no*
  walk, never exercising the walk builder); `digraph/generated_test.go`
  — the first *external* oracle for the three SCC algorithms
  (`Strong`'s labels; the three-way agreement test only proves they
  agree with each other), topological orders verified against every
  edge of generated DAGs, cycle inputs must report `HasOrder` false,
  directed Eulerian walks; `bipartite/generated_test.go` — matchings on
  known-sided graphs (every matching edge must cross the sides,
  `Cardinality <= min(v1,v2)`, `CompleteBipartite(v1,v2)` must hit
  exactly `min(v1,v2)`, all under `checkCertificate`'s König proof).

## Concurrency

Not applicable — test-only, no shared state, single-threaded by
construction.  There is deliberately no `_ts` anything here.
