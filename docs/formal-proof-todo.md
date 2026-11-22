# Formal proof of correctness: research and implementation plan

Status: proposed research plan; no package is formally verified yet.

Date: 2026-11-22

## 1. Executive summary

Formal verification is feasible for a substantial part of Pluto, but there is
no drop-in Go equivalent of Rust's Kani that can consume this entire module.
The practical route is a combination of:

1. **go2lean** for an initial Go/SSA-to-Lean pilot and bounded counterexample
   search on generic, slice-based code;
2. **Gobra** for annotated, heap-manipulating Go and eventually selected
   concurrent code, after a spike confirms support for the required Go subset;
3. **Dafny** models for difficult mathematical algorithms, with differential
   tests connecting the verified model to the Go implementation;
4. **Goose plus Perennial/Rocq** only for the small number of concurrent or
   crash-safe components whose assurance requirements justify an interactive
   proof; and
5. **TLA+/PlusCal** for lock, cache, and disk protocols where a state-machine
   model is more valuable than a direct proof of every Go statement.

A useful first milestone is a **12–16 person-week pilot**, not a repository-wide
annotation exercise. It should verify a small slice sort, union-find, a binary
heap, and one pointer-based container, while establishing a repeatable CI and
proof-maintenance workflow.

A high-value sequential core—linear containers, union-find, heaps, sorting,
ordered arrays, Fenwick/segment trees, substring search, and the basic graph
traversals—is approximately **24–48 person-months** with one experienced
verification engineer and a library maintainer available for specification
questions. Verifying essentially the whole repository, including the `_ts`
variants, codecs, numerical error bounds, probabilistic structures, and disk
formats, is more plausibly **150–400 person-months**. These are planning ranges,
not quotations; the pilot is intended to replace them with measurements.

The strongest existing assets are the model-based randomized tests, structural
invariant checkers, independent oracles, and optimality/correctness certificates.
They should become the specifications and proof lemmas rather than being
replaced by a parallel, less precise contract language.

## 2. What “formally validated” must mean

A successful proof is always relative to a specification, a semantics, and a
trusted computing base. “This package is correct” is not a sufficiently precise
claim. Each verification record should state all of the following:

- **Implementation coverage:** the exact Go revision, files, exported methods,
  generic instantiations, and dependencies covered.
- **Safety:** absence of out-of-bounds access, nil dereference, unexpected panic,
  integer overflow where relevant, data races, and invalid pointer ownership.
- **Functional correctness:** refinement of an abstract model, such as a
  sequence, finite map, multiset, graph relation, or mathematical vector.
- **Termination:** partial correctness alone does not rule out an infinite loop.
  Total correctness requires a decreasing measure or another termination proof.
- **Bounds:** whether a result is exhaustive for all inputs or only for bounded
  sizes, bounded loop unrolling, bounded schedules, or bounded arithmetic.
- **Assumptions:** comparator laws, hash-function behavior, no caller mutation,
  random-source properties, IEEE-754 behavior, allocator behavior, and operating
  system or disk models.
- **Trusted base:** the verifier, SMT solver or proof kernel, Go-to-model
  translator, standard-library models, Go compiler, and runtime.

The following claims should remain separate:

- A **bounded model-checking pass** is not an unbounded proof.
- A **Dafny or TLA+ model** is not a proof of the Go code unless a checked
  refinement or a well-tested equivalence argument connects them.
- **Race-detector and fuzzing success** is evidence, not a proof.
- **Round-trip testing a codec** does not by itself prove compatibility with its
  external format or bounded behavior on hostile input.
- **An asymptotic complexity statement** needs a cost model and its own proof;
  functional correctness does not establish it.
- **Probabilistic accuracy** for Bloom filters or HyperLogLog needs a
  probabilistic argument, not merely deterministic functional verification.

## 3. Repository baseline

At the time of this plan the repository contains approximately:

- 136 package directories with top-level Go files;
- 102,000 production Go lines; and
- 198,000 test Go lines.

There are no proof artifacts or verification-tool dependencies in the module.
The tests already provide several useful specification patterns:

- sequence and map models for lists, queues, hash tables, and tries;
- sorted-slice models and permutation checks for heaps and sorting;
- structural checks for AVL, red-black, B-tree, interval, and skip-list
  invariants;
- independent brute-force oracles for suffix arrays, geometry, matching, and
  graph algorithms;
- LP-duality certificates for `assignment_problem`;
- capacity, conservation, and min-cut certificates for `maxflow`; and
- solution and infeasibility certificates for `gauss`.

A certificate checked at run time is not yet a static correctness proof: a
buggy implementation could compute an invalid answer and reject it, or the
certificate checker could be wrong. It is nevertheless an excellent basis for
a theorem and an independently testable specification.

## 4. Why Kani does not transfer directly

[Kani](https://model-checking.github.io/kani/) is a Rust model checker that
uses proof harnesses and CBMC to check safety and correctness properties. Its
architecture consumes Rust compiler MIR and tracks nightly Rust compiler
interfaces. It is therefore not a Go verification frontend. The issue is not
simply that someone “modified the Rust compiler”; it is that Kani's analysis is
built around Rust's compiler intermediate representation and Rust semantics.

Go's exported AST, type checker, and SSA packages make a separate frontend
possible without forking the Go compiler, but one still has to model slices,
interfaces, maps, defer, goroutines, allocation, overflow, and the memory model.
That frontend and its semantics become part of the trusted base.

## 5. Candidate systems and recommended roles

### 5.1 go2lean: first direct-source pilot, with strict proof gates

[go2lean](https://github.com/fooexec/go2lean) translates Go through `go/ssa`
into an executable Lean 4 model. It supports generic functions instantiated at
concrete types, ordinary Go postconditions, bounded search, replay of candidate
counterexamples as real Go tests, and a `prove` workflow for loop invariants.

This makes it a strong candidate for `shell_sort`, `merge_sort`, `union_find`,
and other slice-oriented pilots. Its bounded search is also useful before
investing in a proof.

Its own [limitations](https://github.com/fooexec/go2lean/blob/main/docs/limitations.md)
are material: search exhaustion is not a proof; termination is not covered by
its functional theorems; spare slice capacity is outside the proof model; and
proof support for structs, pointers, maps, and methods is limited. Model gaps
and under-approximations must be reviewed. Therefore:

- use it first for concrete generic instantiations and simple data types;
- record every `assume`, fuel, slice bound, and unmodeled dependency;
- require Lean kernel-checked certificates, not only a successful search;
- add a separate termination argument where total correctness is required; and
- do not select it as the pointer/concurrency proof engine without another
  capability spike.

Estimated initial setup and pilot: **2–4 weeks**, overlapping the general
specification work below.

### 5.2 Gobra: primary candidate for heap ownership and concurrency

[Gobra](https://github.com/viperproject/gobra) is an automated modular verifier
for annotated Go, built on Viper and SMT backends. Its
[tutorial](https://github.com/viperproject/gobra/blob/master/docs/tutorial.md)
documents pre/postconditions, loop invariants, termination measures, recursive
predicates, mathematical sequences/sets/multisets, slice permissions, mutexes,
channels, and goroutines. It can check memory safety, crash safety, data-race
freedom, and functional contracts.

This is the best conceptual fit for linked lists, trees, graph objects, and
selected `_ts` wrappers. Recursive separation-logic predicates can describe a
tree or list while abstracting it as a sequence or map.

Risks:

- annotated `.gobra` source is not identical to ordinary `.go` source, so a
  generated or synchronized proof copy creates a maintenance obligation;
- Go 1.27 language and standard-library coverage cannot be assumed;
- generic type parameters, `cmp.Ordered`, range-over-func iterators, and
  function-valued comparators need a concrete support spike; and
- SMT quantifiers and recursive predicates can require substantial proof
  guidance and may become brittle across verifier releases.

If generics are unsupported for a target, verify a mechanically generated
non-generic specialization such as `int` and keep the generated file, template,
hash, and differential tests under version control. That proves the specialized
code, not every possible instantiation of the generic source.

Estimated setup and capability spike: **3–6 weeks**. A pointer-based pilot is
likely to take another **4–8 weeks**.

### 5.3 Dafny: executable reference models and difficult proofs

[Dafny](https://dafny.org/latest/DafnyRef/DafnyRef) provides contracts,
inductive datatypes, sets, sequences, multisets, termination proofs, and
automated verification. It is appropriate for proving the algorithm first:
quicksort partitioning, red-black rotations, KMP construction, shortest paths,
matching, or a codec's bitstream relation.

For Pluto, a Dafny proof alone establishes the correctness of the model. The
connection to Go must include:

- translation or code-generation rules that are reviewed and pinned;
- differential tests over the existing fixed-seed and adversarial corpora;
- explicit checks for integer width, overflow, float rounding, aliasing, and
  mutation differences; and
- trace-level comparisons at important loop boundaries where practical.

This route is lower risk for a hard algorithm but weaker as a direct claim
about the shipped Go source. Budget **1–3 weeks** to establish the model/test
workflow, then the algorithm-specific proof cost.

### 5.4 Goose and Perennial/Rocq: high-assurance concurrent and storage cores

[Goose](https://github.com/goose-lang/goose) translates a carefully selected Go
subset to a Rocq/Coq model for verification with Perennial and Iris. It is
particularly relevant to concurrent and crash-safe storage.

It is not a low-cost whole-module solution. Goose intentionally supports only a
subset of Go, including restrictions on ordinary assignment, and the translator
and semantics are trusted. Generic Pluto containers would need adaptation or a
specialized implementation. Use this route only when an interactive proof is
justified—for example, a crash-atomic on-disk B-tree protocol or a concurrency
core that cannot be handled by Gobra.

Expect **2–3 months** to build expertise and prove a small component; realistic
storage proofs are multi-person-month projects.

### 5.5 TLA+/PlusCal: designs and protocols, not Go statement verification

TLA+/PlusCal with TLC is suitable for:

- lock acquisition and `Nl*` calling protocols;
- snapshot-iterator semantics;
- multi-lock operations and lock ordering;
- LRU/LFU/TinyLFU admission and eviction state machines;
- sharded-table resizing and migration; and
- disk write, free-list, recovery, and crash protocols.

TLC normally explores finite configurations. A passing finite model is bounded
verification; TLAPS or an inductive invariant is needed for a general theorem.
A refinement map and model-based test traces are needed to connect the model to
Go.

Budget **1–3 weeks per small protocol**, and **1–3 months** for a disk/recovery
protocol, excluding implementation-level refinement.

### 5.6 Other tools

VeGo is an interesting recent comment-annotation/SSA approach, but its published
concurrency support is a roadmap item and it does not yet provide the mature
heap and concurrency ecosystem needed here. It is reasonable to monitor, not to
make the foundation of this plan.

`go vet`, Staticcheck, fuzzing, property tests, and the race detector remain
valuable pre-proof quality gates, but are not counted as formal proof systems.

## 6. Verification targets and effort by family

The ranges below assume an engineer already familiar with the selected tool.
They cover the core sequential algorithm and its internal representation. They
exclude JSON, examples, rendering, convenience methods, and `_ts` wrappers
unless explicitly named. Shared abstract libraries and proofs should reduce the
cost of later packages; difficult specifications or tool gaps can increase it.

| Packages or family | Properties to prove | Preferred route | Typical effort |
|---|---|---|---|
| `stack`, `queue`, `bag`, `dqueue`, `queue_dll` | Sequence abstraction, order, capacity/index safety, no lost elements | go2lean for slices; Gobra for linked forms | 2–5 weeks each |
| `sll`, `dll`, `quicklist`, `stream` | Ownership, acyclic list shape, links, sequence/map abstraction, mutation safety | Gobra | 4–10 weeks each |
| `union_find` | Parent forest, representative consistency, partition preservation, rank bounds, termination of path halving | go2lean or Dafny, then Gobra if needed | 4–8 weeks |
| `heap`, `priority_queue`, `multiway_pq`, `stable_pq` | Heap order, multiset preservation, min result, index maps, stable tie order | go2lean/Dafny | 4–10 weeks each |
| `index_pq`, `fib_heap`, `binomial_queue` | Heap order plus inverse index map, linking/cutting, degrees/marks, amortized state | Gobra plus Dafny model | 8–24 weeks each; Fibonacci heap may take longer |
| `shell_sort`, `heap_sort`, `merge_sort`, `quick_sort` | Sortedness, permutation preservation, stability where promised, termination, index safety | go2lean/Dafny | 4–10 weeks each; quicksort 8–14 |
| `multiway_merge`, `string_sort` | Sorted output, input multiset, stream stability, radix invariants, termination | Dafny or go2lean | 5–12 weeks each |
| `binary_search_st`, `tree_set`, `hash_set` | Finite-map/set abstraction, ordering/rank laws, duplicate replacement, batch operations | go2lean for arrays; Gobra for composed structures | 5–12 weeks each |
| `fenwick_tree`, `segment_tree`, `sparse_vector` | Indexed map abstraction, prefix/range combination, canonical nonzero representation | go2lean/Dafny | 4–8 weeks each |
| `bitmap`, `crc` | Bit-vector semantics, index rules, polynomial/table equivalence | go2lean or Dafny with bit vectors | 3–8 weeks each |
| `binary_tree`, `splay_tree` | BST order, parent/child ownership, node multiset, rotations and deletion | Gobra | 8–18 weeks each |
| `avl_tree`, `rb_tree`, `interval_tree` | BST plus cached height/color/max endpoint, rotations, deletion, rank where present | Gobra with Dafny support | 10–24 weeks each |
| `b_tree` | Node occupancy, sorted keys/children, recursive ownership, split/merge/rebalancing | Gobra/Dafny | 16–32 weeks |
| `skip_list` | Ordered level lists, spans/ranks, tower bounds, probabilistic-height assumptions | Gobra | 12–24 weeks |
| `trie`, `tst`, `patricia_trie` | String-keyed finite-map abstraction, path compression, pruning, prefix queries | Gobra/Dafny | 8–18 weeks each |
| `substring` | Leftmost-match semantics, DFA/skip-table invariants, rolling-hash verification | Dafny or go2lean | KMP 5–9; Rabin–Karp 3–6; Boyer–Moore 8–16 weeks |
| `suffix_array`, `string_sort` | Suffix permutation/order, Kasai LCP, separator correctness for LCS | Dafny | 10–20 weeks for suffix array |
| `graph`, `digraph`, `symbol_graph` | Adjacency multiset, reachability, shortest BFS paths, components, cycles, SCC, Euler certificates | Gobra for containers; Dafny for algorithms | 2–8 weeks per query after the container proof |
| `dijkstra`, `bellman_ford`, `floyd_warshall` | Path validity and minimality, relaxation invariants, negative-cycle meaning | Dafny plus verified dependencies | 8–18 weeks per algorithm family |
| `mst`, `global_mincut`, `bipartite` | Spanning/cut/matching certificates, minimality/maximality, component changes | Dafny | 10–24 weeks each |
| `maxflow`, `assignment_problem` | Conservation/capacity/min-cut or primal-dual certificates and construction correctness | Dafny; Gobra for the implementation | 16–36 weeks each |
| `lzw`, `huffman`, `run_length`, `compress_z` | Bitstream grammar, encoder/decoder relation, table lockstep, malformed-input bounds | Dafny model plus direct safety proof | 8–20 weeks each |
| `deflate`, `gzip`, `zlib`, `bzip2`, `lz4`, `xz`, `lzma`, `zstd`, `ppmd`, `ppmd8` | Format conformance, bit/Huffman/BWT/state-machine correctness, hostile-input bounds | Layered Dafny models and implementation proofs | 3–12 months per substantial codec |
| `fft`, `gauss`, `geometry`, `g_lib`, `kd_tree`, `geo` | Exact arithmetic relation, then a separate IEEE-754 rounding/error and termination analysis | Dafny/Rocq plus a floating-point framework | 2–6 months each for basic bounds; robust float proofs 6–18 months |
| `bloom`, `hyperloglog` | Deterministic state transitions and no false negatives for Bloom; probabilistic error bounds separately | Gobra/Dafny plus probability theory | 4–10 weeks deterministic; 3–12 months probabilistic |
| `lru`, `lfu`, `tiny_lfu`, caches and shards | Map contents plus recency/frequency policy, admission/eviction, capacity, concurrent linearizability | TLA+ then Gobra/Goose | 2–6 months per family |
| `b_tree_disk_ts`, `database_disk_ts`, `hash_index_disk_ts`, `lru_disk_ts` | In-memory correctness, encoding, free lists, isolation, crash atomicity and recovery | TLA+ and Goose/Perennial | 6–18 months each |
| `_ts` twins | Underlying invariant, lock ownership, race freedom, linearizability, iterator snapshot semantics, absence of deadlock | Gobra; TLA+ for protocols; Goose for exceptional cases | Add 50–150% to each sequential proof |

The rows are not additive estimates: proving common comparator, multiset, list,
tree, graph, and iterator libraries once creates substantial reuse. Conversely,
proving all convenience APIs and every generic instantiation would add work not
represented here.

### Special cases

- **Comparator-based generic code:** require antisymmetry, totality, and
  transitivity, and require equality under the comparator to agree with the
  abstract key relation. A comparison function that subtracts integers can
  overflow; it cannot silently be assumed to implement a lawful order.
- **Hash tables:** prove map refinement independently of hash quality. Treat the
  hash function as deterministic for equal keys and bounded in cost. Prove the
  actual Cuckoo displacement/rebuild termination only under explicit cycle or
  fallback assumptions.
- **Randomized skip lists and Bloom/HyperLogLog:** separate the deterministic
  transition theorem from distributional and expected-complexity claims.
- **Float algorithms:** proving equality over mathematical reals is not a proof
  about `float64`. NaN, infinities, signed zero, overflow, FMA contraction, and
  rounding must be explicit. The documented geometry failures show why this
  matters.
- **Iterators and JSON:** verify them as separate layers. Range-over-func,
  reflection, generic unmarshalling, and live-versus-snapshot behavior add
  substantial language and specification complexity.
- **Concurrent wrappers:** proving that a mutex is present is not enough. Specify
  atomicity for compound methods, the lock caller owns during `Nl*` calls,
  callback reentrancy, two-object lock ordering, and snapshot consistency.

## 7. Recommended phases and acceptance criteria

### Phase 0: specification and tool spike — 3–4 weeks

- [ ] Pin tool versions and create an isolated `verification/` hierarchy; do not
      add verifier runtime dependencies to Pluto's production `go.mod`.
- [ ] Define reusable comparators, sequences, multisets, finite maps, and panic
      conventions.
- [ ] Run go2lean on concrete `int` instantiations of `shell_sort` and `heap`.
- [ ] Run Gobra on a small linked-list package and on a generic/comparator spike.
- [ ] Write one Dafny model and differential-test it against Go.
- [ ] Record verification time, annotation lines, proof lines, unsupported
      constructs, trusted dependencies, and maintenance friction.

**Gate:** select no more than two direct-source routes. If generics require a
large bespoke rewrite, use generated specializations or Dafny models rather
than forking production algorithms into an unrelated verified implementation.

### Phase 1: first useful proofs — 9–12 additional weeks

- [ ] `shell_sort`: sortedness, permutation, termination, and bounds.
- [ ] `union_find`: partition refinement, representative consistency, and
      termination of `Find`.
- [ ] `heap`: multiset refinement and heap order for push/pop/fix/delete.
- [ ] `merge_sort`: sortedness, permutation, stability, and termination.
- [ ] One pointer container (`sll` or `binary_tree`) if the Gobra spike passed.

**Gate:** CI reruns all proofs on the exact source revision; counterexamples
become regression tests; each package has a machine-readable proof manifest;
and no proof uses an undocumented `trust`/`assume` escape hatch.

### Phase 2: reusable sequential core — 4–8 months

- [ ] Linear containers and their iterators' basic sequence contracts.
- [ ] Remaining heaps and sorting algorithms, including `index_pq`.
- [ ] `binary_search_st`, Fenwick and segment trees, and sparse vectors.
- [ ] KMP and Rabin–Karp, then suffix array.
- [ ] Basic graph/digraph containers, DFS/BFS/components, and topological sort.
- [ ] Publish a proof coverage matrix and proof maintenance instructions.

### Phase 3: high-value complex algorithms — 9–18 months

- [ ] AVL and red-black trees, then interval and B-trees.
- [ ] Tries and ordered/ranked skip-list operations.
- [ ] Dijkstra, Bellman–Ford, Floyd–Warshall, and MST certificates.
- [ ] Max-flow/min-cut, bipartite matching, and assignment duality.
- [ ] Huffman and LZW bitstream/table proofs, including malformed input.

### Phase 4: specialized assurance — multi-year, only by requirement

- [ ] Linearizability and deadlock properties for selected `_ts` packages.
- [ ] Crash safety and recovery for disk-backed structures.
- [ ] Full external codec conformance and hostile-input resource bounds.
- [ ] Floating-point accuracy and robust geometry.
- [ ] Probabilistic error bounds for Bloom and HyperLogLog.

### Ongoing maintenance

Reserve **15–30% of the original proof effort per year** while the verified
code is actively changing. Any code change must rerun its proofs. A failed
proof cannot be resolved by weakening the contract without reviewer approval.
Pin verifier releases and schedule explicit upgrade windows; do not mix proof
migration with feature work.

## 8. Sedgewick/algs4 audit found in this repository

The audit below distinguishes an actual defect or inconsistent documentation
from a narrower contract, robustness improvement, or performance rewrite. It
uses the repository's source/tests and the current upstream
[algs4 source](https://algs4.cs.princeton.edu/code/). The upstream code can
change, so links and access dates matter.

### 8.1 Confirmed defects or broken edge cases

| Package | Finding and Pluto behavior | Classification |
|---|---|---|
| `maxflow` | Upstream `FlowNetwork.edges()` retains an adjacency entry only when `e.to() != v`. A self-loop is stored twice in its own adjacency list and both copies fail that filter, so enumeration omits it. Pluto's result `Edges()` includes every edge. See [FlowNetwork.java](https://algs4.cs.princeton.edu/64maxflow/FlowNetwork.java.html) and `maxflow/README.adoc`. | Confirmed upstream enumeration defect; does not change the maximum-flow value because self-loops cannot augment an s–t flow. |
| `substring` | Upstream KMP allocates `dfa[R][0]` for an empty pattern and then indexes `pat.charAt(0)`. Pluto defines the empty pattern to match at zero. See [KMP.java](https://algs4.cs.princeton.edu/53substring/KMP.java.html). | Confirmed empty-input crash; empty-pattern semantics are a deliberate Pluto extension. |
| `substring` | Upstream `RabinKarp(char[], int)` unconditionally throws `UnsupportedOperationException`. Pluto exposes only the string/byte-oriented constructor. See [RabinKarp.java](https://algs4.cs.princeton.edu/53substring/RabinKarp.java.html). | Confirmed unimplemented upstream overload, not a defect in the string algorithm. |
| `lzw` | Upstream `expand()` indexes `st[codeword]` and dereferences `s.charAt(0)` without validating hostile or out-of-range codewords. Pluto reports malformed input as `nil`. See [LZW.java](https://algs4.cs.princeton.edu/55compression/LZW.java.html). | Confirmed malformed-input crash; valid-stream round trips are a separate property. |
| `huffman` | Upstream empty compression builds an empty `MinPQ` and then calls `delMin()`. Pluto maps empty input to empty output. Upstream recursive `readTrie()` also trusts the input shape; Pluto caps a byte alphabet's tree at 511 nodes. See [Huffman.java](https://algs4.cs.princeton.edu/55compression/Huffman.java.html). | Confirmed empty-input failure and malformed-input robustness gap. |
| `geometry` | Upstream Graham scan's inner loop calls `hull.peek()` without a base-vertex guard. Nearly collinear floating-point input can pop below the base and throw. Pluto stops at the base and documents that its result may then be approximate. See [GrahamScan.java](https://raw.githubusercontent.com/kevin-wayne/algs4/master/src/main/java/edu/princeton/cs/algs4/GrahamScan.java). | Confirmed robustness gap by code inspection; the local test pins no panic, not mathematical hull correctness for noisy floats. |

### 8.2 Documentation mismatch and deliberate contract changes

These should not be reported as algorithmic correctness bugs:

- **`assignment_problem`:** the current upstream Javadoc says edge weights are
  non-negative, while the implementation shifts by `minWeight` and therefore
  supports negative finite weights. Pluto supports and tests negative weights.
  This is an upstream documentation inconsistency. Pluto also replaces the
  per-round residual graph and general graph Dijkstra with a dense matrix scan;
  the local benchmark reports 2.6–2.9× less time and about 1300× less memory at
  `n=200` for that Go comparison. See
  [AssignmentProblem.java](https://algs4.cs.princeton.edu/65reductions/AssignmentProblem.java.html).
- **`fft`:** upstream linear convolution requires equal power-of-two input
  lengths and returns a padded result. Pluto accepts arbitrary lengths, chooses
  sufficient padding, and trims to `len(x)+len(y)-1`. This is a generalized
  contract, not evidence that upstream convolution is wrong for its documented
  inputs.
- **`quick_sort`:** Pluto replaces the reference's public pre-shuffle with
  median-of-three, insertion cutoff, and recurse-smaller/iterate-larger. These
  are deterministic robustness changes. The repository explicitly found no
  correctness bug in `Quick3way` partitioning.
- **`merge_sort` and `shell_sort`:** changes are established `MergeX`
  optimizations, allocation/copy choices, and equivalent loop forms, not defect
  fixes.
- **`multiway_merge`:** Pluto uses `Change` rather than delete-plus-insert and
  adds stream ID as a tie-break for stability. Upstream `Multiway.java` still
  produces sorted output; stability and constant-factor work are the changes.
- **`string_sort`:** byte-wise ordering covers arbitrary Go strings, with
  deterministic pivot/stack improvements. These are representation and
  robustness differences.
- **`bag`:** insertion-order iteration differs from `LinkedBag`'s prepend order
  but agrees with the bag abstraction and `ResizingArrayBag`.
- **`trie`/`tst` and `patricia_trie`:** deletion/pruning, nil/zero behavior,
  byte strings, generic values, and iterators are Pluto API extensions.
- **`kd_tree`:** arbitrary coordinates, derived rectangles, and `NearestK` are
  extensions or representation changes; no reference defect was established.
- **`geometry.FarthestPair`:** Pluto checks additional parallel-edge tie pairs
  and tests against brute force. The present evidence justifies calling this a
  robustness difference. It should not be promoted to a confirmed upstream
  defect without a saved Java counterexample.

### 8.3 A correction to an older local Rabin–Karp note

The older local note described upstream Rabin–Karp as unverified Monte Carlo.
The current booksite and GitHub source call `check(txt, offset)` on every hash
hit and identify the implementation as Las Vegas. The current upstream code and
Pluto therefore both prevent a hash collision from becoming a false positive.
The remaining differences are Pluto's fixed `2^31-1` prime, deterministic
output, raw-byte strings, `-1` miss result, and empty-pattern handling.

## 9. Boxing, interface dispatch, and performance evidence

### 9.1 Direct repository evidence

Git commit **`4b0e608` (“Move away from interfaces”)** added a now-removed
`bench_compare` module specifically to measure interface boxing/unboxing on a
binary tree. It compared three implementations on identical deterministic
workloads:

1. the old interface-based tree, whose struct element was converted to a
   `Comparable` interface and recovered with a type assertion;
2. the same struct shape with a plain typed comparison function; and
3. plain string keys using built-in ordering.

The benchmark covered build/insert, successful search, delete-all, and full
iteration at 1,000 and 10,000 elements. Its design explains why the ordering
workloads are the interesting ones: interface conversion and dispatch occur per
comparison, while iteration is a control with no comparisons. The struct with a
string field was selected because its interface conversion requires storage
beyond a single interface data word and can allocate per comparison.

The benchmark was deleted in commit `1c892cd` when stale “sibling library”
references were removed. **No benchmark result file is present in Git history**,
so this document records the verified experimental design but intentionally does
not invent a speedup or allocation count. Rebuild a current standalone benchmark
before publishing numerical claims.

The current generic design is documented in the root README: ordered structures
use built-in `<`/`>` for `cmp.Ordered` or a typed `func(a, b T) int`; elements
are stored and returned by value. The most directly affected operations are
comparison-heavy tree insertion/search/deletion, heap sifts, priority queues,
and sorting. Iteration alone does not exercise comparator boxing.

### 9.2 Java/algs4 cases where the same issue arises

- `Quick3way.sort(Comparable[])` and the other object-based sorting APIs compare
  through `Comparable`. Primitive values must first be represented by wrapper
  objects such as `Integer[]`; they cannot be stored directly in that array.
- `BinarySearchST<Key extends Comparable<Key>, Value>` stores `Comparable[]` and
  `Object[]`. Primitive numeric keys/values require wrappers and interface
  dispatch; Pluto stores typed key/value slices.
- `LZW` uses `TST<Integer>`, so codeword values are wrapper objects in Java;
  Pluto uses `Tst[int]`.
- `SymbolGraph` uses `ST<String,Integer>` for vertex indices; Java's index
  values are boxed. Pluto uses its native map/index representation.
- Heaps and indexed priority queues similarly compare object references through
  `Comparable`, while Pluto's ordered constructors use typed operators or a
  typed comparator.

These observations establish the mechanism and the affected operations, not a
universal claim that every Go implementation is faster. JIT optimizations,
escape analysis, wrapper caches, element size, comparator cost, allocations for
nodes, and workload shape matter. Go generic values can also allocate when
captured or converted elsewhere. Performance claims require matched benchmarks;
they are not properties that a correctness proof establishes.

## 10. Per-package reference-difference documentation

The package READMEs are the canonical user-facing location for these details.
Existing sections already cover `quick_sort`, `merge_sort`, `shell_sort`,
`multiway_merge`, `string_sort`, `substring`, `lzw`, `huffman`, `fft`,
`geometry`, `maxflow`, `kd_tree`, and `interval_tree`. This audit adds or
corrects the binary-tree boxing history, Rabin–Karp's current upstream status,
the assignment-problem summary, and the binary-search symbol-table comparison.

Future ports should use a consistent README section named **“Deviations from
the algs4 reference”** and label each item as one of:

- confirmed reference defect;
- malformed-input hardening;
- API/semantic extension;
- representation or performance change; or
- numerical robustness change.

Every claimed defect should link the exact upstream file and a local regression
test or saved counterexample. If upstream later fixes the issue, retain the
historical note with versions/dates instead of silently rewriting history.

## 11. Sources and further reading

- [Kani documentation](https://model-checking.github.io/kani/)
- [go2lean repository and proof/search distinction](https://github.com/fooexec/go2lean)
- [go2lean limitations](https://github.com/fooexec/go2lean/blob/main/docs/limitations.md)
- [Gobra repository](https://github.com/viperproject/gobra)
- [Gobra tutorial](https://github.com/viperproject/gobra/blob/master/docs/tutorial.md)
- [Gobra paper](https://arxiv.org/abs/2105.13840)
- [Goose repository](https://github.com/goose-lang/goose)
- [Perennial](https://www.chajed.io/perennial/)
- [Dafny reference manual](https://dafny.org/latest/DafnyRef/DafnyRef)
- [algs4 source index](https://algs4.cs.princeton.edu/code/)
- Local historical benchmark: `git show 4b0e608:bench_compare/README.adoc` and
  `git show 4b0e608:bench_compare/bench_test.go`.
