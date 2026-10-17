# Additional algorithms to implement — algs4 gap analysis

Date: 2026-10-17
Sources: <https://algs4.cs.princeton.edu/code/> (the full textbook-program index),
plus §6.3 Geometric Intersections (<https://algs4.cs.princeton.edu/93intersection/>, "chapter under construction" but `IntervalST.java` is published there).

Method: every Java program in the algs4 catalog was mapped against pluto's packages
(`ls` of the repo + CLAUDE.md). Items pluto already has were recorded once in the
coverage table and dropped; what remains was scored on four axes:

1. **Classic-ness** — is it a load-bearing classic of the textbook/course?
2. **Library fit** — is it a *data structure / stateless algorithm* per pluto's charter (type parameters, value semantics, no interface boxing, the two-constructor ordering convention, documented panic surface), not a client or I/O harness?
3. **Marginal value** — does it add a *capability* pluto lacks (stability, spatial search, interval stabbing, regexp matching, decrease-key in o(log n) amortized…) or merely a variant?
4. **Effort** — including the house bar: `README.adoc` + root table, `thorough_test.go` model test at fixed seed, invariant checker, `-race` for `_ts` twins.

Note the mapping runs both ways: pluto also contains major structures algs4 lacks
(`skip_list`, `splay_tree`, `b_tree_disk_ts`, `cuckoo`, `bloom`, `hyperloglog`,
`geo`, `bitmap`, `lru`/`lfu`, `stream`, `quicklist`, `sharded_hash_ts`) — those are
Ultima-driven and not reconsidered here.

---

## 1. Already covered (for the record)

| algs4 program(s) | pluto package |
|---|---|
| Stack, Queue (linked + resizing array) | `stack`, `queue`, `stack_sll_ts`, `queue_dll`, `dqueue` |
| Bag | `bag` (+ `_ts`) |
| UF / WeightedQuickUnionUF | `union_find` (+ `_ts`) |
| Quick / Quick3way / QuickX ideas | `quick_sort` (3-way, median-of-3, cutoff) |
| Shell | `shell_sort` (3x+1 gaps) |
| Heap (heapsort), MaxPQ/MinPQ | `heap_sort` (+ `_ts`), `heap` (+ `_ts`) |
| IndexMinPQ / IndexMaxPQ | `index_pq` (+ `_ts`) |
| BinomialMinPQ | `binomial_queue` |
| BST | `binary_tree` (+ `_ts`), `splay_tree`, `b_tree` |
| AVLTreeST (§9) | `avl_tree` (+ `_ts`) |
| RedBlackBST | `rb_tree` (+ `_ts`) |
| SeparateChainingHashST | `hash_tab`, `hash_tab_dll`, `hash_tab_bt` (+ `_ts` each), `sharded_hash_ts` |
| LinearProbingHashST | `hash_grow` (+ `_ts`), `cuckoo` (+ `_ts`) |
| TrieST, TST, PatriciaST/SET | `trie`, `tst`, `patricia_trie` (+ `_ts` each) |
| KMP, BoyerMoore, RabinKarp | `substring` |
| LZW | `lzw` |
| RunLength | `run_length` |
| SuffixArray (+ LRS client) | `suffix_array` |
| BTree | `b_tree`, `b_tree_disk_ts` |
| Graph, DFS/BFS paths, CC | `graph` (+ `_ts`) |
| Digraph, DirectedDFS, paths, DirectedCycle, DepthFirstOrder, Topological, KosarajuSCC | `digraph` (+ `_ts`) |
| EdgeWeightedGraph, LazyPrim/Prim/Kruskal | `mst` (+ `_ts`) over `dijkstra` |
| DijkstraSP/Undirected/AllPairs, AcyclicSP/LP, BellmanFordSP, EdgeWeightedDirectedCycle (internal) | `dijkstra`, `bellman_ford` (+ `_ts` each) |
| FordFulkerson, FlowNetwork, FlowEdge | `maxflow` |
| FenwickTree, SegmentTree (§9) | `fenwick_tree`, `segment_tree` (+ `_ts` each) |

---

## 2. Out of scope (and why)

* **I/O, drawing, timing harnesses** — `StdIn`/`StdOut`/`In`/`Out`/`Draw`/`Picture`/`Stopwatch`/`DoublingTest`…: not algorithms; Go stdlib covers them.
* **Case-study clients** — percolation, `ThreeSum` analysis clients, `CollisionSystem`/`Particle` (§6.1 event-driven simulation), N-body, `Arbitrage`, `CPM`, `GREP`, `KWIK`: clients of the library, not library material.
* **Numeric support classes** (`Complex`, `Polynomial`, `Vector`, `Matrix`, `Interval1D/2D`) — only pulled in as *dependencies* of a chosen algorithm (see kd_tree, FFT below); Go's `complex128` covers `Complex`.
* **Exercise trace variants** (`TraceMerge`, `NonrecursiveDFS`, `DirectedCycleX`, `TopologicalX`, `SuffixArrayX`, `LazyDijkstraSP`) — teaching visualizations / redundant variants.

---

## 3. Tier 1 — build next

Ordered by priority; effort is *S* (<½ day), *M* (1–2 days), *L* (3+ days) at house
quality (README + model tests + invariants).

### 3.1 `merge_sort` — M — **DONE 2026-10-15** (`merge_sort/`)

* **algs4:** `Merge`, `MergeBU`, `MergeX` (§2.2), `Inversions`, `KendallTau` rider.
* **Why first:** the most conspicuous gap in the library. Two of the three classic sorts exist (`quick_sort`, `heap_sort`); what is missing is not just another sort but the **stability** property, and the merge-based riders (inversion counting) that only a merge sort gives.
* **API sketch** (stateless functions, `quick_sort` precedent → **no `_ts` twin**):

  ```go
  func Sort[T cmp.Ordered](a []T)                          // stable top-down; aux slice, insertion cutoff
  func SortFunc[T any](a []T, cmp func(a, b T) int)        // the only panic: nil cmp
  func SortBottomUp[T cmp.Ordered](a []T) / SortBottomUpFunc(...)
  func Inversions[T cmp.Ordered](a []T) int64              // counts via merge, returns permuted copy optional
  ```
* **Verification:** `slices.SortStableFunc` oracle (sortedness + *stability* — verify with satellite-data keys), seeds 42/7, adversarial shapes (all-equal, sorted, reverse); `Inversions` vs brute-force O(n²) oracle.
* **Shipped:** all of the sketch (Sort/SortFunc with MergeX cutoff-7 + ordered skip, SortBottomUp/SortBottomUpFunc, Inversions/InversionsFunc returning int64 over a private copy). The `KendallTau` rider was **not** built — it is a two-line composition of `InversionsFunc` over a rank permutation, left for when a caller needs it.

### 3.2 `bag` — S — **DONE 2026-10-15** (`bag/`, `bag_ts/`)

* **algs4:** `Bag`, `LinkedBag`, `ResizingArrayBag` (§1.3).
* **Why:** last missing §1.3 container; quick win. Constraint-free, constructor-free (the `queue` contract): zero value usable, single panic = `Add` on nil.
* **API:** `Add(T)`, `Len`, `IsEmpty`, `All() iter.Seq2[int, T]` (insertion order), `Backward`. Singly linked (stable memory, the `queue_dll` rationale).
* **Twin:** `bag_ts` (the `queue_ts` pattern: nil guards before lock, snapshot iterators, race trio). Trivial.

### 3.3 `interval_tree` — M — **DONE 2026-10-15** (`interval_tree/`, `interval_tree_ts/`)

* **algs4:** `IntervalST.java` (§6.3 intersections; textbook exercise 3.3.43 "interval search tree"), companions `Interval1D/2D`, sweep-line clients.
* **Why:** a classic *augmented* balanced BST (each node stores max-hi of its subtree) answering **stabbing/overlap queries** in O(log n) — a capability pluto has nothing like. Enormously practical (calendar/IP-range/bandwidth problems; the booksite's 70k IP-range dataset is a ready-made perf fixture).
* **API sketch** (generic bounds — pluto generalizes Sedgwick's ints):

  ```go
  type Interval[T cmp.Ordered] struct{ Lo, Hi T }   // Lo <= Hi enforced at insert (error/panic choice per house)
  type IntervalTree[T any] struct{ ... }            // balanced via pluto rb-tree invariants, maxHi augmentation
  func NewIntervalTree[T cmp.Ordered]() *IntervalTree[T]
  func NewIntervalTreeFunc[T any](cmp func(a, b T) int) *IntervalTreeFunc[T]
  Insert(iv Interval[T], val T) bool   // replace-on-equal, bool like the trees
  Delete(iv) bool
  AnyOverlaps(iv) (Interval[T], T, bool)
  OverlapsAll(iv) iter.Seq2[Interval[T], T]  // snapshot, ts convention
  Stab(point T) iter.Seq2[Interval[T], T]
  ```
* **Twin:** `interval_tree_ts` (mutable container → RWMutex twin).
* **Verification:** algs4 exercise semantics + randomized model vs O(n) scan oracle (seed 42); invariant checker: BST order by Lo, maxHi == max over subtree, balance.
* **As built:** two type params `IntervalTree[K, V any]` (the sketch's `val T` was a typo — satellite value, the lru map pattern), a *self-contained* CLRS red-black tree (not composed on rb_tree — the augmentation lives in the nodes), closed intervals (touching overlaps, point intervals legal).  Names deviate from the sketch: `AnyOverlap` (not AnyOverlaps) and `Overlaps` (not OverlapsAll); `Stab` kept.  Inverted `Hi < Lo` is the 4th Insert panic (b_tree-shaped contract).  The ts twin exports real `Lock`/`Unlock` + `Nl*` (skip_list_ts surface) — the canonical compound is check-overlap-then-book.  Same-seed model test (PCG 42,7) in both twins.

### 3.4 `kd_tree` — M/L — **DONE 2026-10-16** (`kd_tree/`, `kd_tree_ts/`)

* **algs4:** `KdTree.java`, `PointSET.java` (§5+ "Nearest neighbor search"; §1.2 `Point2D`, `RectHV`).
* **Why:** THE multidimensional search structure — orthogonal range search and nearest-neighbor in 2D+, a whole query family nothing in pluto approximates. Also the gateway to the §9 geometry batch (Tier 2) by introducing `Point2D`/`RectHV` support types.
* **Decision to make up front:** faithful 2D port (Sedgwick's `Point2D`/`RectHV`, alternating x/y split, nearest pruned by oriented distance) vs generic k-d (`[]float64` points, k fixed at construction). Recommendation: **2D faithful first** (`geo`/`bitmap` non-generic precedent), note the generalization in the README.
* **API sketch:** `NewKdTree()`, `Insert(Point2D) bool`, `Contains`, `Nearest(p) (Point2D, bool)`, `NearestK(p, k)`, `Range(rect) []Point2D` / iterator, `Len`, `IsEmpty`. Zero value usable; panic = `Insert` on nil only.
* **Twin:** `kd_tree_ts`.
* **Verification:** brute-force `PointSET` oracle (implement both — algs4 does), circle-test client fixture, seeds 42/7; invariants: partition by depth, subtree point containment, size field.
* **As built:** all of the sketch, 2D-faithful per the recommendation. `Point2D`/`RectHV` live in `g_lib/point2d.go` (comparable value types — `Point2D` is a legal map key — so Tier 2's geometry batch reuses them). Constructor-free (the tries' precedent — the splitting rule is inherent in the coordinates; `NewKdTree()` convenience only), nodes carry **no stored RectHV** (`splitRects` derives each subtree rect from the ancestor splits during the descent), root rect = the whole plane ±Inf (not algs4's unit square). `NearestK` keeps its k best in a bounded max-heap composed from `pluto/heap` (reversed comparator on squared distance); output deterministic by (d², X, Y); ties among exactly equidistant points unspecified beyond the distance sequence (mathematical ties can differ by an ulp — both structures compute identical d² so their orders agree; the model tests compare distance sequences). Panic contract: 2 per structure — `Insert` on nil, `Insert` with a NaN coordinate (the interval_tree reject-at-write precedent; ±Inf legal, saturating d²). `PointSET` shipped as `PointSet` (map-backed brute force) — the test oracle and the small-input fallback. No delete (as in algs4 — rebuild); unbalanced (diagonal inserts legitimately chain, asserted). `kd_tree_ts` is the wrapper twin (stream_ts pattern — plain value + one RWMutex, nothing stored to adopt/snapshot) with eager-snapshot `All`, `Nl*` mirror, and check-spacing-then-insert as the canonical compound; `PointSet` deliberately has no twin. Tests: PCG 42,7 4000-step model vs `PointSet`, invariants checker, tie/degenerate-chain/depth/edge-rect suites, twin race trio + plain-vs-twin model. Benchmarks (100k points, M4 Max): Nearest 925 ns vs brute 466 µs (~500×), Range 777 ns vs 664 µs (~850×), NearestK 3.4 µs vs 10.4 ms (~3,000×).

### 3.5 `nfa` (regular expressions) — M

* **algs4:** `NFA.java` (§5.4), `GREP` client.
* **Why:** §5.4 is missing wholesale, and Sedgwick's NFA construction (Thompson NFA via digraph + stack, simulated by multi-set-of-states reachable set) is a beloved classic that fits pluto's *constructor-style immutable query object* pattern exactly (`substring` precedent: build once, search many, zero panics).
* **API sketch:**

  ```go
  func NewNFA(pattern string) (*NFA, error)  // supports ( ) | * . + per NFA.java; syntax error -> error (algs4 throws)
  func (n *NFA) Recognizes(text string) bool
  ```
  Internally builds a plain `pluto/digraph` for the ε-transitions (intra-pluto composition) — or a private adjacency like `bellman_ford`'s internal DFS; pick by benchmark.
* **Twin:** none (immutable — `substring`/`suffix_array` precedent).
* **Verification:** property test vs Go `regexp` oracle on the supported subset (seeds 42/7, adversarial patterns like `(a|aa)*b` + `a{n}` blowup-free), algs4 propositional-dynamic-logic examples.

### 3.6 `string_sort` (LSD / MSD / Quick3string) — M — **DONE 2026-10-16** (`string_sort/`)

* **algs4:** `LSD`, `MSD`, `InplaceMSD`, `Quick3string`, `AmericanFlag(X)`, `Alphabet` (§5.1).
* **Why:** §5.1 missing wholesale; key-indexed/radix character sorts are the classic answer when comparison sorts are the bottleneck and the Redis-lineage library handles byte strings natively throughout.
* **Shape:** one stateless package `string_sort` (`geo`/`bitmap` precedent — pure functions, **no twin**):

  ```go
  func LSDSort(a []string, w int)                 // fixed-width keys, w passes of key-indexed counting
  func MSDSort(a []string)                        // recursive, insertion cutoff at small subarrays
  func Quick3String(a []string)                   // 3-way radix quicksort, in place
  ```
  (internal shared `insertion` cutoff helper; `Alphabet`-style extended-ASCII assumption, 256 buckets, documented).
* **Verification:** `slices.Sort` oracle over random fixed-width and variable-width keys, equal-prefix-heavy inputs (the MSD worst case), seeds 42/7; benchmarks vs `slices.Sort` and `quick_sort`.
* **As built:** all of the sketch. Ordering is byte-wise lexicographic — exactly Go's native string `<` — so `slices.Sort` is the exact oracle (strings are values, the sorted sequence of a multiset is unique; `slices.Equal` checks sortedness+permutation in one) and binary-safe/UTF-8 keys work unchanged (`Alphabet`/`InplaceMSD`/American-flag documented out of scope).  Panic contract: exactly 2, both `LSDSort` (`w < 0`, key shorter than w — the only function with a precondition).  `Quick3String` carries the two `quick_sort` deviations (median-of-3 pivot byte, recurse-smaller-side/loop-larger — a library cannot pre-shuffle); MSD skips recursion into empty buckets (no-op elimination); `less(v,w,d)` is `v[d:] < w[d:]`.  Thorough tests at PCG 42,7: fixed/variable-width, equal-prefix-heavy, prefix chains, alphabet-256, 100k sorted/reverse/all-equal adversarial, and 2000 keys sharing a 2000-byte prefix (recursion depth vs Go stack growth).  Benchmarks (100k keys, M4 Max): MSD 2.5 ms vs `slices.Sort` 9.4 ms on random 16-byte keys; `Quick3String` most robust on long shared prefixes (beats `slices.Sort` there); LSD 4.6 ms on random16 but 25.3 ms on 64-byte keys — every w pass is a full counting sort.

### 3.7 `huffman` + `run_length` — M total — **DONE 2026-10-16** (`huffman/`, `run_length/`)

* **algs4:** `Huffman.java`, `RunLength.java` (§5.5); `LZW` already done.
* **Why:** completes the §5.5 compression trio. Both are stateless `Compress/Expand` pairs (`lzw` precedent, **no twin**); the binary bit-I/O already exists in `lzw/bits.go` (unexported — either duplicate small or lift into `g_lib` as an internal shared helper; decide once, note in README).
* **API:** `func Compress(data []byte) []byte`, `func Expand(blob []byte) []byte` (nil on malformed, `lzw` convention). Huffman = canonical code, 8-bit symbols, trie serialized in the header; RLE = algs4's 8-bit count/bitmap-style runs (simplest member, do first).
* **Verification:** round-trip property tests at seeds 42/7 across entropy regimes; hand-traced vectors for tiny inputs; malformed-input sweeps.
* **As built (huffman, 2026-10-16):** the sketch's "canonical code" read as the standard frequency-built code — the trie IS serialized in the header (the sketch's concrete instruction), and canonical length-table encoding was declined: it saves ~60 header bytes but caps code lengths at what the table can hold, while the trie form has no length limit (85 Fibonacci-weighted symbols legitimately produce an 84-bit codeword, which is why codewords are strings, not integers — algs4 uses `String` too).  The merge composes `pluto/heap` with a `(freq, seq)` tie-break, so output is deterministic: recompressing an expansion reproduces the blob byte for byte (asserted every round-trip iteration).  Count is 64 bits, not algs4's 32-bit int (Go slices exceed 4 GiB).  Malformed hardening beyond lzw's truncation sweep: `readTrie` caps at 511 nodes (a full binary tree over 256 symbols; algs4 recurses to stack exhaustion on hostile streams) and `Expand` rejects counts no slice could hold or that exceed the bits remaining under an internal root (≥1 bit/symbol there — provably never rejects honest blobs); the single-leaf zero-bits-per-symbol case is the documented, algs4-shared decompression-bomb remainder.  bits.go duplicated from lzw (the "duplicate small" option): g_lib's charter is stateless pure functions, a bit writer is stateful — a third consumer would trigger lifting into `internal/`.  Tests: hand-traced byte-exact vectors (`AB`/`AAA`/`AAB`/`X`, derived on paper), the seed-42/7 six-regime round-trip + determinism + parsed-brie invariants (full binary, ≤256 leaves, Kraft Σ 2^-depth = 1), scan-based no-heap optimality oracle on the weighted path length (incl. all-equal-frequency tables), every-strict-prefix truncation sweep, trie-cap and garbage sweeps, deep-chain expand, concurrent round-trip.  Measured (M4 Max, 1 MiB): 0.33 B/byte runs / 0.55 text / ~1.0 uniform / 10 bytes single; compress 70–671 MB/s, expand 22–1276 MB/s.

### 3.8 Graph/digraph query completions — S each — **DONE 2026-10-16**

Small constructor-style query objects added to the existing packages (and mirrored in the `_ts` twins) — the pattern is already established there:

* `graph.Cycle` — undirected cycle detection (self-loop/parallel-edge aware, algs4 `Cycle.java`).
* `graph.TwoColor` (a.k.a. `Bipartite`) — 2-coloring / bipartiteness, `IsBipartite`, `Color(v)`.
* `digraph.TransitiveClosure` — Warshall reachability matrix, `Reachable(v, w)`; document the O(V²) memory.
* (rider) `suffix_array.LCS(s, t)` — algs4 `LongestCommonSubstring.java` (§6.3): build suffix array over `s#t`, scan LCP of adjacent suffixes from opposite halves — S, rides on existing machinery.

* **As built:** all four plus twins. `Cycle`/`TwoColor` in `graph` (+ `graph_ts`): dedicated self-loop and parallel-edge scans BEFORE the DFS (the parent check cannot see either), cycle reported with the first vertex repeated at the end; `TwoColor.Color(v)` returns `(bool, bool)` — meaningful only when bipartite. Both are source-less queries tolerating nil/empty graphs (the `NewCC` precedent). `TransitiveClosure` in `digraph` (+ `digraph_ts`): algs4's one-DFS-per-vertex into a V×V boolean matrix (O(V·(V+E)), the sparse-friendly alternative to literal Warshall O(V³)), reused `marked` scratch cleared between passes, every vertex reaches itself. `LCS` in `suffix_array`: dynamic separator (first byte absent from both inputs) instead of a literal `#`, returning `("", false)` only when all 256 byte values are in use. Verification: seed-42 model tests against structurally different oracles — incremental-union has-cycle + closed-walk certificate, BFS-coloring + proper-coloring certificate, boolean-matrix Warshall, brute-force LCS length + substring check.

### 3.9 `fib_heap` — M/L — **DONE 2026-10-16** (`fib_heap/`)

* **algs4:** `FibonacciMinPQ`, `IndexFibonacciMinPQ` (§9 Beyond).
* **Why:** completes the priority-queue zoo — binary ✅, binomial ✅, **Fibonacci ❌** — with the textbook's amortized O(1) `Insert`/`DecreaseKey` and O(log n) extract-min (lazy root list + cascading cuts; degree bound D(n) ≤ log_φ n).
* **API:** mirror `index_pq` (the index variant is the useful one — decrease-key is the point): `NewIndexFibonacciPQ[T cmp.Ordered](n)` / `…Func`, `Insert(k, v)`, `Change`/`DecreaseKey`, `Pop`, `Peek`, `All` in priority order.
* **Twin:** follow `binomial_queue` — **no `_ts` twin** initially; document.
* **Verification:** model test vs `index_pq`/`heap` (seed 42, every return value cross-checked), invariant checker (heap order, degree bound, mark rules); benchmarks against `heap` showing where decrease-key wins (dense-graph Dijkstra driver).
* **As built:** all of the above. One `extract` routine is the removal machinery for `Pop`, `Delete` AND the increase-direction `Change` (the CLRS delete-then-insert trick — no subtree-dissolve code); `DecreaseKey` refuses a non-decrease (false, no change) and is nil/zero-tolerant like `Delete`; consolidate reuses scratch slices (Pop is 0-alloc steady state); the invariant checker verifies the Fibonacci size lemma (degree-d ⇒ ≥ F(d+2) nodes), which is the log_φ degree bound itself. The mirror model test draws tie-free values — after a tie-pop the two structures legally hold different index sets and every presence op diverges (ties covered by a separate map-oracle test). Benchmark honesty: `DecreaseKey` 5.7 ns vs `index_pq.Change` 31.5 ns, but the dense Dijkstra driver still loses to the binary heap (1.22× at V=400 → 1.11× at V=1500 — asymptotics favor fib, constants don't; documented in README).

### 3.10 `multiway_pq` (d-ary heap) — M — **DONE 2026-10-16** (`multiway_pq/`)

* **algs4:** `MultiwayMinPQ`, `IndexMultiwayMinPQ` (§9).
* **Why:** the practical PQ variant algs4 ships for large heaps: d children ⇒ shallower tree, cheaper `DecreaseKey`/sift-down at dearer `Insert` — the right structure behind dense Dijkstra/large schedulers; also `Multiway.java`'s k-way sorted-stream merge rides naturally here (`MergeK` helper or an example).
* **API:** `NewDHeap[T cmp.Ordered](d int)` / `NewDHeapFunc(d, cmp)`; d ≥ 2 panic; otherwise mirror `heap`'s surface.
* **Twin:** optional `d_heap_ts` later; ship plain first, benchmark d ∈ {2,4,8,16}.
* **As-built:** `DHeap[T]` mirrors `heap`'s ENTIRE surface file-for-file (incl. `iter.go` `All` and `json.go` — unmarshal bulk-rebuilds in O(n) instead of heap's O(n log n) push loop) with `d` fixed at construction and `D()` exposing it; panic contract = heap's 3 + `d < 2`. Randomized model test runs d ∈ {2,3,4,8,16} against a multiset model AND pluto `heap` value-for-value (tie-safe: two heaps over one multiset pop the same minimum VALUE). Benchmarked d ∈ {2,4,8,16} with worst-case sift paths (leaf-slot decreases, root increases — a random slot is a leaf on average and measures nothing): decrease 12.8→7.7 ns (d=2→16), Pop 71.9→124.3 ns, bulk build 20.5→10.8 µs (FASTER with d); d=2 tracks `heap` within noise. Multiway.java merge shipped as a checked example (`MergeK` remains item 23).

---

## 4. Tier 2 — strong candidates

| # | Item | algs4 ref | Notes / shape | Effort |
|---|---|---|---|---|
| 11 | `shell_sort` | `Shell.java` (§2.1) | **DONE 2026-10-15** (`shell_sort/`) — 3x+1 gaps exactly as algs4, hoisted-value inner loop; the 3x+1-vs-Knuth bench comparison was not done | S |
| 12 | `stable_pq` | `StableMinPQ` (§2.4 ex.) | Stable priority queue (tie → FIFO); pairs with `merge_sort` as its PQ cousin | S |
| 13 | `bipartite` matching | `BipartiteMatching`, `HopcroftKarp` (§6.4) | Augmenting-path matching + O(E√V) Hopcroft–Karp over `graph`; compose `maxflow` like algs4 or direct augmenting paths (bench once, pick) | M |
| 14 | `assignment_problem` | `AssignmentProblem` (§6.4) | Hungarian/successive-shortest-paths n×n assignment | M/L |
| 15 | `floyd_warshall` | `FloydWarshall` (§4.4) | All-pairs SP, negatives OK (detects negative cycles); V×V matrix query object, own small pkg | S/M |
| 16 | `boruvka_mst` | `BoruvkaMST` (§4.3) | Completes the MST quartet (lazy/eager Prim, Kruskal done); UF-driven | S |
| 17 | `global_mincut` | `GlobalMincut` (§6.4) | Global min cut over `maxflow` (n−1 maxflows) or Stoer–Wagner; pick after benchmark | M |
| 18 | SCC + Eulerian batch | `TarjanSCC`, `GabowSCC`, `EulerianCycle/Path`, directed Eulerian (§4.1/4.2) | Additions to `digraph`/`graph` + twins; Tarjan/Gabow are the linear SCCs algs4 teaches beside Kosaraju; Eulerian is the classic multigraph walk | S each |
| 19 | Geometry batch | `GrahamScan`, `ClosestPair`, `FarthestPair` (§9) | `Point2D`/`RectHV` now in `g_lib` (introduced by kd_tree); polar-order comparator; closest-pair divide-and-conquer; convex hull returns ccw `[]Point2D` | M |
| 20 | `fft` | `FFT.java` (§9) | Radix-2 FFT over `[]complex128`; stateless functions (`geo` precedent), no twin; verify against DFT oracle + round-trip | M |
| 21 | `gauss` | `GaussianElimination`, `GaussJordanElimination` (§9) | Needs a minimal matrix story (`[][]float64` is enough); pivoting + singular detection | M |
| 22 | `sparse_vector` | `SparseVector` (§3.5) | Dimension-indexed sparse dot/axpy used by gauss/simplex; hash-map backed | S |
| 23 | `multiway` merge | `Multiway.java` (§2.4) | `MergeK` over sorted slices via `index_pq` — helper in `index_pq`/`priority_queue` or `g_lib` | S |

---

## 5. Tier 3 — completeness / opportunistic

* `binary_search_st` (`BinarySearchST`, §3.1) — sorted-array symbol table with `Rank`/`Floor`/`Ceiling`; overlaps `skip_list` capability-wise, but it is *the* array-ST classic and trivially cheap.
* `symbol_graph` (`SymbolGraph`, §4.1) — string→int vertex facade over `graph`/`digraph`; thin, but every graph client wants it.
* Set facades — `trie`/`tst`/`patricia` already return keys; a `TrieSet`-style shell is sugar only.
* `suffix_array_x` — 3-way-quicksort suffix construction variant (impl detail; skip unless construction time becomes a benchmark concern).
* Linear programming — `LinearProgramming`, `TwoPersonZeroSumGame` (§6.5/6.6): heavyweight numeric, pull in only if a numeric-algorithms track is ever wanted.
* `GraphGenerator`/`DigraphGenerator` — not public packages, but porting a few generators (random, grid, bipartite, tinyDG-style) into shared *test* helpers would harden the randomized graph tests.

---

## 6. Suggested order of work

1. **Quick wins batch:** ~~`bag` + `bag_ts`, `shell_sort`, graph additions (Cycle/TwoColor/TransitiveClosure), `run_length`~~ — all done (bag/bag_ts, shell_sort and run_length 2026-10-15; the graph additions + the `suffix_array.LCS` rider 2026-10-16).
2. ~~**`merge_sort`** (+ `Inversions`) — closes the stability gap.~~ — done (2026-10-15), with the stability oracle against `slices.SortStableFunc`.
3. **`interval_tree` + `_ts`** — first genuinely new query family.
4. ~~**`string_sort`**~~ — **DONE 2026-10-16** (`string_sort/`, backlog §3.6): LSD → MSD → Quick3string.
5. **`nfa`** — regexp query object.
6. ~~**`kd_tree` + `_ts`**~~ — **DONE 2026-10-16** (`kd_tree/`, `kd_tree_ts/`, backlog §3.4): introduced `Point2D`/`RectHV` in `g_lib`, unlocking Tier 2 geometry.
7. **`huffman`** — completes compression.
8. **`fib_heap`** — **DONE 2026-10-16** (`fib_heap/`, backlog §3.9); then **`multiway_pq`** — **DONE 2026-10-16** (`multiway_pq/`, backlog §3.10). PQ zoo complete.
9. Tier 2 in table order as appetite allows (SCC/Eulerian batch and `boruvka_mst` are the cheapest next stops).

Rough totals: Tier 1 ≈ 7–10 working days; Tier 2 adds ≈ 8–12.
