# Set capabilities & set packages — implementation plan

Status: **plan, not yet implemented** (drafted 2026-10-17). No code has been
written against this plan.

Source of the capability list: hashicorp/go-set `hashset.go` / `set.go`
(MPL-2.0). What that library offers that pluto does not — verified by a full
API inventory of every pluto package (`go doc` + grep of non-test sources):

| hashicorp/go-set capability | pluto today |
|---|---|
| `Slice() []T` (materialize elements) | **missing everywhere** — all containers expose only `All()`/`Values()` as `iter.Seq`/`iter.Seq2`; the only slice-returning queries are the tries' `KeysWithPrefix`/`KeysThatMatch` |
| `String()` / `StringFunc()` (`fmt.Stringer`) | **missing everywhere** (only `bloom`, `stream.ID`, `g_lib.Point2D` implement Stringer) |
| `Equal` / `EqualSet` / `EqualSlice` | **missing everywhere** — no content-equality method exists on any container (only the free `g_lib.EqualSlice`) |
| `Copy()` | only `avl_tree`/`avl_tree_ts` `Copy(yy)` (destination form) and `hyperloglog`/`bloom_ts` `Clone()` |
| `InsertSlice` / `RemoveSlice` (batch) | **missing everywhere** |
| `ContainsSlice` / `ContainsAny` | **missing everywhere** |
| `RemoveFunc` (predicate bulk delete) | **missing everywhere** (nearest: `binary_tree.DeleteMatch`, which is custom-compare delete) |
| `Subset` / `ProperSubset` / superset / disjoint predicates | **missing everywhere** |
| `Union` / `Intersect` / `Difference` | **only** `avl_tree` + `avl_tree_ts`: `Union(yy, zz)`, `Intersect(yy, zz)`, `Minus(yy, zz)` (avl_tree/avl_tree.go:741-806) |
| `SymmetricDifference` | **missing everywhere** (new even for the avl family) |
| `Size()` / `Empty()` | already present house-style as `Len`/`Length`/`IsEmpty` — no change |
| `MarshalJSON` / `UnmarshalJSON` | **already present on every container** — no change |
| `Items() iter.Seq[T]` | **already present** as `All()`/`Values()` range-over-func iterators — no change |
| `From`/`FromSlice` constructors | missing (new set packages get them) |

Two things hashicorp has that pluto must **not** copy verbatim, per the design
contract: the `Collection[T]` interface (cross-container ops) and the
`Hasher[H]` method constraint. The pluto-idiomatic replacements are given in
§2.

---

## Part 1 — convenience capabilities for the existing data types

### 2. Translation rules (house contract wins)

Every addition is **additive** — no existing method changes name, signature,
or semantics.

1. **No interfaces.** Cross-container batch input is `[]T` or
   `iter.Seq[T]` (a function type, not a method constraint): the analog of
   hashicorp's `InsertSet(col Collection[T])` is
   `InsertIter(seq iter.Seq[T]) bool` — any pluto container's `All()`/
   `Values()` feeds it. Nothing ever requires `Hash()` on an element; hashing
   stays the table's own maphash-with-random-seed machinery.
2. **Naming map** (hashicorp → pluto):
   - `Size`/`Empty` → existing `Len`/`IsEmpty` (unchanged)
   - `Remove*` → existing `Delete*`
   - `RemoveFunc` → `DeleteFunc(fx func(T) bool) bool` (true = anything removed)
   - `Copy()` → `Clone() *X[T]` returning a fresh container (matches the
     `Hll.Clone`/`Bloom.Clone` precedent). avl's existing destination-form
     `Copy(yy)` is kept untouched; new code uses `Clone` only.
   - `Difference` → `Minus` (the established avl name)
   - `Subset(col)` (argument-first, reads backwards) → `IsSubsetOf(other)` /
     `IsSupersetOf(other)` / `IsProperSubsetOf` / `IsProperSupersetOf` /
     `IsDisjointWith` (receiver-first, reads correctly)
   - `Slice()` keeps the name `Slice` — it does not collide with anything,
     because pluto's `Values()` is an iterator, not a slice.
3. **Return-value conventions**: batch mutators return `bool` = "receiver
   changed" (`InsertSlice` true if any element was new; `DeleteSlice`/
   `DeleteFunc` true if anything was deleted), exactly like the existing
   single-element `Insert`/`Delete`.
4. **Panic contract unchanged.** All new methods are total functions with
   nil/zero-value tolerance for reads: a nil container Equals only another
   empty container, `String()` of nil is the empty rendering, `Slice()` of
   nil is nil/empty, `DeleteFunc` on nil returns false. The only panics that
   can surface are the *existing* insert-contract panics (e.g.
   `InsertSlice` reaching `Insert` on a nil/zero-value hash table — the same
   thing a caller loop would hit).
5. **Determinism.** All 12 hash packages hash with maphash under a random
   per-table seed (verified: e.g. hash_grow/hash_tab.go:95), so bucket order
   is nondeterministic across processes. Therefore:
   - `String()` for hash families **sorts the formatted element strings**
     (hashicorp's own trick: format each element, sort the strings) — output
     is run-to-run deterministic without ordering `T`.
   - `Slice()` for hash families is unordered (documented; matches
     hashicorp).
   - `Equal` is membership-based, never order-based.
   - Ordered structures need no sorting: their `String()`/`Slice()` walk
     in-order, which is already deterministic.
6. **`_ts` twins mirror everything** (same names, same signatures). Rules:
   - Batch operations take the write lock **once** for the whole batch (the
     `heap_sort_ts` `NlAppendHeap`/`NlHeapify` precedent) — a batch is
     atomic.
   - `Equal`/`Slice`/`String`/`Clone` snapshot under the read lock, then
     compute/encode unlocked (the JSON-marshal precedent).
   - Set-style relations snapshot each operand under its own read lock and
     never hold two locks at once (the `avl_tree_ts.Union` discipline,
     avl_tree_ts/avl_tree.go:919).
   - No new `Nl*` variants are required (the new methods are self-locking,
     not compounds); the existing `NlSearch` etc. remain the building blocks
     for caller-side compounds.
7. **Equality preconditions.** `Equal`, the subset predicates, and set
   algebra require both containers to use *consistent* equality/comparison
   functions. This is a documented precondition — it cannot be checked
   (functions are not comparable) and is the same rule the avl set ops
   already rely on.

### 3. The capability menu

Semantics each addition must implement (complexities for hash families are
O(1) average per element unless noted; ordered structures multiply by
O(log₂ n)):

| Method | Semantics |
|---|---|
| `Slice() []T` | fresh `[]T` of all elements. Ordered structures: ascending. Sequences: logical order (FIFO order, stack top-down — the order `All()` yields). Hash: unspecified order. |
| `String() string` | `fmt.Stringer`. Ordered: in-order, e.g. `[a b c]`. Hash: sorted formatted strings (rule 5). Sequences/heaps: logical/backing order. `StringFunc(f func(T) string) string` alongside, for caller formatting. |
| `Clone() *X[T]` | deep copy (fresh buckets/arrays/nodes — never share backing arrays with the receiver). Keeps the eq/cmp/hash functions. `_ts`: snapshot under read lock. |
| `Equal(o *X[T]) bool` | same element set (same `Len` + every element of `o` found by `Search`). Sequences: **order-sensitive** (documented — they are not sets). Heaps/PQs: same multiset (see §4.5). Map-shaped types (tries, interval semantics): compare **keys only** — `T` is `any` and value comparison is not expressible without boxing. |
| `Contains(x T) bool` | sugar for `Search(x)` found-flag, where the container lacks it (tries already have it). |
| `ContainsSlice(items []T) bool` | true iff **every** item is present (hashicorp `ContainsSlice`). |
| `ContainsAny(items []T) bool` | true iff at least one item is present. |
| `InsertSlice(items []T) bool` | insert each; true if any was new. |
| `InsertIter(seq iter.Seq[T]) bool` | the interface-free `InsertSet`. |
| `DeleteSlice(items []T) bool` | delete each; true if any was present. |
| `DeleteFunc(fx func(T) bool) bool` | delete every element satisfying `fx`. Implemented collect-then-delete (snapshot matches, then delete) — never delete in place while walking open-addressed/chained storage. |
| `Union(yy, zz)` / `Intersect(yy, zz)` / `Minus(yy, zz)` | avl's existing destination-form semantics: truncate receiver, rebuild as yy ∪ zz / yy ∩ zz / yy − zz; operands snapshotted (alias-safe); zero-value receiver adopts an operand's comparison. New packages copy this contract verbatim. |
| `SymmetricDifference(yy, zz)` | yy △ zz = (yy ∪ zz) − (yy ∩ zz); completes the algebra to a lattice; implement as Minus(Union(yy,zz), Intersect(yy,zz)) or a direct merge walk. |
| `IsSubsetOf(o) bool` etc. | receiver ⊆ o (elementwise via `Search`; proper = subset and `Len` strictly less). |
| `Keys() []string`, `Values() []T` | tries only: all keys in byte order / all values in key order (currently only prefix/glob queries return slices). |

`g_lib` addition (optional, avoids ~40 copies of the sorted-format loop):
`FormatSorted[T any](items []T, format func(T) string) string` — used by
every hash-family `String()`; ordered structures do not need it.

### 4. Applicability matrix — where each capability goes

Legend: ● = add, ○ = optional/deferred, — = does not apply.

#### 4.1 Hash tables (12 packages: hash_tab, hash_tab_ts, hash_tab_bt,
hash_tab_bt_ts, hash_tab_dll, hash_tab_dll_ts, hash_grow, hash_grow_ts,
cuckoo, cuckoo_ts, cuckoo_grow15_ts, sharded_hash_ts)

These are sets natively — the fullest treatment.

| Slice | String(+Func)      | Equal | Clone | Contains | ContainsSlice/Any | InsertSlice/InsertIter | DeleteSlice | DeleteFunc |
|-------|--------------------|-------|-------|----------|-------------------|------------------------|-------------|------------|
| ●     | ● (sorted strings) | ●     | ●     | ●        | ●                 | ●                      | ●           | ●          |

- Set algebra (Union/Intersect/Minus/SymmetricDifference) and subset
  predicates are **not** added to these 12 packages — they live once in the
  `hash_set` facade (Part 2), not duplicated 12×(plus twins). `Equal` is the
  one multi-container method that stays on the tables (fundamental, and the
  test suites want it anyway).
- Traps: `DeleteFunc` on the open-addressed tables (hash_grow/cuckoo
  families) must collect-then-delete (rule 3); mass deletion may cross
  cuckoo's shrink threshold — delete after the snapshot, shrink is safe then.
  `hash_tab_dll`'s element-handle `Search` is irrelevant here — all new
  methods go through `Values()`/`Search`, never through live handles.
  `sharded_hash_ts` batch ops lock stripe-by-stripe (its `LockKey` world);
  a whole-table batch is not atomic across stripes — document, matching its
  existing `Scan` caveats.
- `hash_tab_bt` uses a 3-way `cmp` in its Func constructor (not eq) — its
  `Equal` still uses `Search`; no constructor changes.

#### 4.2 Ordered structures (12 packages: binary_tree, binary_tree_ts,
avl_tree, avl_tree_ts, rb_tree, rb_tree_ts, splay_tree, b_tree, skip_list,
skip_list_ts, interval_tree, interval_tree_ts)

| Slice (in-order) | String(+Func) | Equal | Clone | Contains | ContainsSlice/Any | InsertSlice/InsertIter | DeleteSlice | DeleteFunc | SymmetricDifference                                 |
|------------------|---------------|-------|-------|----------|-------------------|------------------------|-------------|------------|-----------------------------------------------------|
| ●                | ●             | ●     | ●     | ●        | ●                 | ●                      | ●           | ●          | ○ (avl family only, completes the existing quartet) |

- Full set algebra stays avl-only (+ the new `tree_set`, Part 2). The other
  trees keep map semantics; their callers who want algebra use `tree_set`.
  (`b_tree` and `splay_tree` have no `_ts` twin — nothing changes there.)
- `splay_tree`: `Clone`/`Equal`/`Slice`/`Contains` all splay (every access
  mutates — legal, amortized as always; note in doc comments).
- `interval_tree`: `Equal` compares stored intervals via exact-match
  `Search`; the overlap-query machinery is not involved.
- Riders for Part 2 (avl enablers, independently useful): add
  `Floor(key)`/`Ceil(key)` to avl_tree + avl_tree_ts, symmetric with the
  existing `FindMin`/`FindMax` (skip_list already has them — the signatures
  to copy).

#### 4.3 String-keyed tries (6 packages: trie, trie_ts, tst, tst_ts,
patricia_trie, patricia_trie_ts)

The tries **are** pluto's string sets (this resolves backlog
`additional-algorithms-to-implment.md` §5 "Set facades — sugar only": no
`TrieSet` package is built; instead the tries get set ergonomics directly).

| Keys | Values | String (byte order) | Equal (keys only) | Clone | ContainsSlice/Any (keys) | DeleteSlice (keys) |
|------|--------|---------------------|-------------------|-------|--------------------------|--------------------|
| ●    | ●      | ●                   | ●                 | ●     | ●                        | ●                  |

- `Keys()` = all keys in byte order (today only `KeysWithPrefix`/
  `KeysThatMatch` return slices). `Values()` returns values in key order.
- `Equal` compares **key sets only** — values are `T any` and comparing them
  is not expressible without boxing; document loudly (an `EqualKeys` alias
  is NOT added; the doc comment carries it).
- tst's empty-key rejection and trie's root-stored empty key mean `Equal`
  between a trie and a tst is meaningful only for non-empty key sets — they
  are different packages with separate `Equal`s anyway; note in docs.
- Batch insert is deliberately not added (a map-shaped batch is
  `InsertMap(map[string]T)` territory — callers loop; keep the surface
  small).
- Clone must deep-copy nodes (R-way arrays in `trie` are big — 2 KiB/node;
  clone walks, it does not memcpy).

#### 4.4 Sequences (16 packages: queue, queue_ts, queue_dll, stack, stack_ts,
stack_sll_ts, dqueue, dqueue_ts, quicklist, quicklist_ts, bag, bag_ts, dll,
dll_ts, sll, sll_ts)

Minimal treatment — sequences are not sets; O(n) membership scans and
subset predicates would be footguns.

| Slice (logical order) | String | Equal (ORDER-SENSITIVE) | Clone |
|-----------------------|--------|-------------------------|-------|
| ●                     | ●      | ●                       | ●     |

- `Equal` is documented as order-sensitive (a queue [a b] ≠ [b a]) — the one
  family where equality is sequence equality, not set equality.
- `bag` is add-only: no `DeleteFunc`/`DeleteSlice` (nothing to delete with);
  `Slice` is insertion order (its `All()` order — deliberately kept).
- `dll`/`sll` are error-based APIs; the four additions are total functions
  needing no errors — they fit without disturbing the error contract.
- `quicklist` `Slice` must respect compression (it materializes like `At`/
  iterators do — the ts twin write-locks `At`-shaped reads, so `Slice`/
  `String` take the same discipline: write lock in the twin).

#### 4.5 Heaps & priority queues (10 packages: heap, heap_ts, priority_queue,
priority_queue_ts, index_pq, index_pq_ts, fib_heap, multiway_pq, stable_pq,
binomial_queue)

| Slice (backing/breadth order) | String (same order) | Equal (multiset) | Clone |
|-------------------------------|---------------------|------------------|-------|
| ●                             | ●                   | ●                | ●     |

- Order semantics follow the JSON precedent (note/implement-json.md): a
  heap's only stable non-destructive observable order is its backing array /
  `All()` order — `Slice` and `String` use it, `priority_queue` uses sorted
  order (its `All()` is sorted).
- `Equal` = same length + same element sequence sorted by the comparator
  (non-destructive). Ties: two heaps over one multiset may hold different
  internal arrangements — Equal compares the *sorted value sequence*, so
  ties are safe (the `multiway_pq` tie-safe model-test precedent).
  `stable_pq`: compare key sequences, ignore FIFO stamps (they differ
  between equal multisiset insertions).
- `heap_sort` is excluded — it is functions over caller slices, not a
  container.

#### 4.6 Explicitly excluded

- `b_tree_disk_ts` (disk store — different beast), `stream`/`stream_ts`
  (Redis stream semantics), `bitmap`/`geo`/`crc`/sorts/compression packages
  (functions or byte-domain, already have their own surface),
  `graph`/`digraph` (+twins), `kd_tree`, `suffix_array`, `union_find`,
  `segment_tree`/`fenwick_tree` (array-indexed, wrap caller storage),
  `bloom`/`hyperloglog` (+twins — approximate: exact `Equal`/subset is
  meaningless; they already have `Clone`/`Merge` where sensible).
- `lru`/`lfu` (+twins): ○ `String()` only, in `All()` order — deferred to
  the last phase; everything else is eviction-order-sensitive and murky.

---

## Part 2 — the set packages

Two new structure families, composed on existing pluto packages (the
`priority_queue`-over-`heap` composition precedent), giving pluto the
algs4 `SET`/`HashSET` pair with Go generics and no interfaces.

### 5. `hash_set` + `hash_set_ts` — HashSet[T]

- **Base**: `hash_grow.HashTab[T]` (auto-growing open addressing — the
  general-purpose, map-like engine; fixed-size `hash_tab` and the exotic
  tables are deliberately not the base).
- **Type**: `HashSet[T any]` wrapping an unexported `hash_grow.HashTab[T]`
    plus the stored eq/hash pair (mirroring how the table stores them, so
    `Clone` and operand-adoption work).
- **Constructors** (house two-form):
  - `NewHashSet[T comparable]() *HashSet[T]` — `==` + maphash random seed
  - `NewHashSetFunc[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashSet[T]`
  - `FromSlice[T comparable](items []T) *HashSet[T]` / `FromSliceFunc(eq, hash, items)`
    (the hashicorp `From` analog). Capacity defaults internal; no options
    machinery in v1.
- **API** = the §3 menu in full:
  - core: `Insert`/`Delete`/`Search`/`Contains`/`Len`/`IsEmpty`/`Truncate`
  - batch: `InsertSlice`/`InsertIter`/`DeleteSlice`/`DeleteFunc`/
    `ContainsSlice`/`ContainsAny`
  - conveniences: `Slice`/`String`/`StringFunc`/`Clone`/`Equal`
  - **algebra** (avl contract verbatim — destination form, snapshotted
    operands, alias-safe, zero-value receiver adopts an operand's eq+hash):
    `Union(yy, zz)`, `Intersect(yy, zz)`, `Minus(yy, zz)`,
    `SymmetricDifference(yy, zz)`
  - relations: `IsSubsetOf`/`IsProperSubsetOf`/`IsSupersetOf`/
    `IsProperSupersetOf`/`IsDisjointWith`
  - iteration: `All() iter.Seq[T]`, `Walk(fx ApplyFunction[T])`
  - JSON: array-of-elements marshal/unmarshal — unmarshal replaces contents
    and keeps the eq/hash functions (the dll JSON precedent)
- **hash_set_ts**: wrapper twin (kd_tree_ts pattern — plain value + one
  RWMutex). Algebra snapshots each operand under its own read lock, never
  two locks at once (avl_ts discipline). Batch ops atomic under one write
  lock. Full API parity + `Lock`/`Unlock` + `Nl*` for the single-element
  core (`NlInsert`/`NlDelete`/`NlSearch`/`NlLen`/`NlIsEmpty`).
- **Complexities**: algebra is O(n+m) expected (hash) — document per method.

### 6. `tree_set` + `tree_set_ts` — TreeSet[T]

- **Base**: `avl_tree.AvlTree[T]`. Rationale: the set-op machinery already
  exists there and is proven (snapshot operands, comparator adoption, twin
  lock discipline — avl_tree/avl_tree.go:741-806), so the facade's algebra is
  thin delegation. The alternative base — `skip_list`, which already has
  `Floor`/`Ceil`/`Rank`/`Range` — was considered; it is recorded as the
  fallback if rank queries ever matter more than reuse (a skip-list sorted
  set would be the Redis-zset shape and could be a later package; it is not
  this one).
- **Type**: `TreeSet[T any]` wrapping `avl_tree.AvlTree[T]`.
- **Constructors**: `NewTreeSet[T cmp.Ordered]()` / `NewTreeSetFunc[T any](fx
    func(a, b T) int)` / `FromSlice` / `FromSliceFunc`.
- **API**: the §3 menu plus the sorted-set surface avl gives for free —
  `FindMin`/`FindMax`/`Floor`/`Ceil` (the §4.2 rider)/`Index`/`Range`
  (sub-range iteration via the tree walk)/`DeleteAtHead`/`DeleteAtTail`.
  `Slice`/`String` are ascending and deterministic without any sorting.
- **tree_set_ts**: wrapper twin as above; snapshot iterators; the avl_ts
  algebra lock pattern carries over unchanged.

### 7. What is deliberately NOT built

- No `Collection[T]`-style interface, no `Hasher` — cross-set ops are
  same-concrete-type; slices and `iter.Seq` bridge everything else.
- No `TrieSet`/string-set package — the tries are it (§4.3).
- No b-tree set, no disk-backed set, no sorted-set-over-skip-list in v1.
- No mutable-during-iteration guarantees added anywhere; no ordering
  guarantees added to hash families; no `reflect.DeepEqual` anywhere.

---

## 8. Tests (house patterns)

Per package touched, per capability:

- Fixed-vector tests + a randomized model test against reference models
  (`map[T]bool` / `g_lib.ToBoolMap` / `g_lib.EqualSlice`),
  fixed seeds 42 and 7.
- `String` determinism: two separately-constructed containers with the same
  elements produce identical strings (this is what the sorted-format rule
  buys; without it the random seed breaks it).
- `Clone` deep-copy assertion: mutate the clone, receiver unchanged (and
  vice versa) — catches shared-backing-array bugs.
- `Equal` symmetric/reflective properties; order-sensitivity test for
  sequences; multiset-tie test for heaps.
- `DeleteFunc` on open-addressed tables: large table, delete ~half, then
  full membership re-verification (catches probe-chain/tombstone damage).
- Twins: the race trio (concurrent read/write/iterate under `-race`),
  plain-vs-twin behavioral parity, batch-atomicity test (writer interleaved
  with `InsertSlice` sees all-or-nothing).
- Set packages additionally: algebra property tests vs map-computed
  reference results (union/intersection/difference/symdiff/subset laws —
  commutativity where applicable, alias-safety `s.Union(s, s)`), adoption
  tests for zero-value receivers, JSON round-trip keeping eq/cmp.
- Checked examples (`example_test.go`) for every new exported method — they
  appear on pkg.go.dev.
- No benchmark obligations for the convenience methods; the two set packages
  get one comparative benchmark each (hash_set vs tree_set vs a plain
  `map[T]bool` driver at n = 1k/100k) to document the constant-factor
  reality, in the honest-benchmarking tradition of fib_heap/multiway_pq.

## 9. Documentation updates required by the change

For every package touched: its `README.adoc` (new methods in the operations
table with complexities, updated Panics and Concurrency sections) and
`docs/packages/<name>.md` (same-change rule). Plus:

- root `README.adoc`: package-index rows for `hash_set`, `hash_set_ts`,
  `tree_set`, `tree_set_ts`.
- `docs/packages/hash_set.md`, `docs/packages/tree_set.md` (new — structure,
  base-choice rationale, algebra contracts, adoption rules, traps).
- `docs/packages/twins.md`: the two new twin pairs in the mapping table.
- `CLAUDE.md` package index: two new one-line entries.
- `docs/additional-algorithms-to-implment.md` §5: annotate the "Set facades
  — sugar only" line with the resolution (tries got set ergonomics; generic
  sets are the new packages).

## 10. Rollout order and effort

Each phase is independently shippable; within a phase the work parcels
cleanly per package (the note/implement-json.md agent-per-package wave
pattern).

| Phase | Scope                                                                           | Packages | Effort                |
|-------|---------------------------------------------------------------------------------|----------|-----------------------|
| 0     | `g_lib.FormatSorted` helper                                                     | 1        | XS                    |
| 1     | Hash families — full §4.1 menu                                                  | 12       | M (2–3 days parallel) |
| 2     | Ordered structures — §4.2 menu + avl `SymmetricDifference` + avl `Floor`/`Ceil` | 12       | M/L                   |
| 3     | Tries — §4.3 menu                                                               | 6        | S/M                   |
| 4     | Sequences — §4.4 minimal menu                                                   | 16       | S                     |
| 5     | Heaps/PQs — §4.5 menu                                                           | 10       | S                     |
| 6     | `hash_set` + `hash_set_ts`                                                      | 2        | M                     |
| 7     | `tree_set` + `tree_set_ts`                                                      | 2        | M                     |
| 8     | Cache `String()` riders + §9 doc cross-updates                                  | 4+docs   | XS                    |

Gates per phase: `make build test vet race lint`, `gofmt -l .` empty,
`go mod tidy` leaves no root go.sum. Phases 6–7 land only after 1–2 (the
facades reuse `Slice`/`Equal`/algebra pieces and the avl riders).

## 11. Risks / traps checklist

- Random-seed nondeterminism leaking into `String`/JSON tests (mitigated:
  sorted-format rule; never assert raw hash-JSON byte-equality across two
  constructions).
- `DeleteFunc` in-place iteration corruption on open addressing / chain
  relinking — collect-then-delete, always.
- Twin batch atomicity — one lock hold per batch, no per-element
  lock/unlock loops (deadlock/misordering).
- avl algebra alias-safety — keep the snapshot-operands rule when porting;
  `s.Union(s, s)` is in the property tests for exactly this.
- `Clone` sharing backing arrays (hash buckets, quicklist segments, trie
  node arrays) — the deep-copy mutation test is mandatory.
- Comparator/eq consistency precondition left undocumented — every Equal/
  algebra doc comment must state it.
