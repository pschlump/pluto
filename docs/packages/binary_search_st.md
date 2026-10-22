# binary_search_st notes

Ordered symbol table (key → value) on two parallel arrays kept sorted by
key — algs4 §3.1 `BinarySearchST`.  First `[K, V]` two-type-parameter
container in the repo ordered by K; the comparator contract generalizes
the usual single-`T` form as `NewBinarySearchST[K cmp.Ordered, V any]` /
`NewBinarySearchSTFunc[K, V any](fx func(a, b K) int)`.

## Structure

- `keys []K` + `vals []V`, always equal length, always sorted by `cmp`.
  No sentinel, no capacity bookkeeping — plain `slices.Insert` /
  `slices.Delete` shifts (both zero the vacated slots; `DeleteFunc`
  compacts in place and `clear`s the tail, the queue precedent).
- `search(key)` is the single private binary search: the insertion
  point, i.e. the index of the first key `>= key`.  Every public lookup
  (`Get`, `Floor`, `Ceil`, `Rank`, `span`) is `search` plus at most one
  neighbor check and one `cmp` equality probe.  Hand-rolled loop — the
  repo has no shared binary-search helper, and `slices.BinarySearchFunc`
  would pay a closure per step.
- `span(lo, hi)` returns the half-open `[a, b)` of keys in the closed
  range; `CountRange`/`DeleteRange`/`Range` all share it.  All three
  guard `lo`-after-`hi` via `cmp(lo, hi) <= 0` first — without the
  guard `span` can return `b < a` and `DeleteRange` would splice
  backwards.

## API decisions

- **Put/Get/Delete/Contains, not Insert/Search** — the map verb school.
  Precedent: `lru` is the only other K→V package and speaks Put/Get;
  algs4's ST is put/get.  The ordered riders keep the skip_list names
  (`Rank`/`AtIndex`/`Floor`/`Ceil`/`CountRange`/`DeleteRange`) because
  the backlog bullet names `Rank`/`Floor`/`Ceiling` and calls skip_list
  the capability twin.  `Rank` uses skip_list semantics — position of a
  *present* key, `(0, false)` when absent — NOT algs4's
  count-of-keys-less-than (which would always succeed).
- Lookups return `(K, V, bool)` pairs (`FindMin`/`FindMax`/`Floor`/
  `Ceil`/`AtIndex`); `Get` returns `(V, bool)`.  No `Equal`: keys have
  the stored comparator but values have no equality function, and a
  cmp+eq pair of functions would be a new constructor shape (lru also
  has no Equal).  Compare via `Keys` + `Get`.
- JSON is the index_pq wire form — `[{"k":…,"v":…}]` in ascending key
  order; unmarshal validates the whole document before storing, follows
  the Put last-pair-wins convention for duplicate keys, and tolerates
  `[]`/`null` everywhere.

## Panic contract

Exactly two non-constructor situations (the binary_tree insert
contract): `NewBinarySearchSTFunc(nil)`, and `Put` — directly or via
`PutIter`/`UnmarshalJSON` — on a nil table or a zero-value table (no
comparison function).  Note `PutIter` contract-checks *up front* (an
empty seq on a nil table still panics — it is a storing operation, like
`Insert`); `UnmarshalJSON` checks only when pairs would actually be
stored (the index_pq precedent).  Everything else is nil/zero tolerant.
`cmp == nil` implies the arrays are empty, so reads never reach a nil
comparator.

## Traps

- `Put` in ascending order is the worst case (every insert shifts the
  whole tail); benchmark `Put` at random positions, not sorted.
- Iterators (`All`/`Backward`/`Range`) capture the slice headers at
  iteration start — reallocation is invisible, but an in-place
  `slices.Insert` shift during iteration is unspecified.  Documented as
  "the contract does not cover mutating while iterating".
- `_ts` twin: `binary_search_st_ts` (added 2026-10-22), the wrapper
  twin this file predicted — a plain value + RWMutex + the twin's own
  cmp copy (the write-contract check and Clone read it), snapshot
  iterators (`All`/`Backward`/`Range` copy under the read lock — a
  live shift under iteration would be data race soup), `PutIter`
  drained before the write lock, `DeleteFunc` rebuilt atomically
  (snapshot + predicate unlocked + Truncate + ascending re-Put,
  O(n log₂ n) — the plain in-place compaction cannot run a caller
  callback under the lock), JSON decode outside the lock, and the
  tree_set_ts Nl* surface (`NlPut`/`NlGet`/`NlDelete`/`NlContains`/
  `NlLen`/`NlIsEmpty`); twin-named panic messages; a store through
  `NlPut` on a zero-value twin reaches the plain prefix (tree_set_ts
  precedent).

## Test-oracle expectations

- `TestRandomizedModel`: fixed seed 42, 800 steps against a
  `map[string]int` reference; every 40 steps the sorted model keys are
  the oracle for `Keys`/`All`, and `slices.BinarySearch` on them is the
  oracle for `Floor`/`Ceil`/`Rank`/`AtIndex`.
- `stKey{S string; N int}` ordered by S only is the TestTreeNode shape
  (satellite data replaced on duplicate keys); the `[]string`-key test
  proves keys never touch `==` (slice keys are not comparable at all).

## Benchmark reality

Apple M4 Max, 1000 entries: `Get` hit 118 ns vs 128 ns for skip_list
`Search` (both allocation-free O(log₂ n) — the array's locality is a
small consistent win; misses and `Floor` similar).  Writes are the
cost: a random-position `Put`+`Delete` pair ~810 ns vs ~360 ns for the
skip_list pair, and the shift grows linearly with n.  Read-mostly
tables, or the teaching shape — the README says so.
