# hash_set / hash_set_ts

`HashSet[T]` — the algs4 `HashSET` analog — a set of unique elements
with the full set surface (core, batches, algebra, relations,
conveniences, JSON), implemented as a **facade over
`hash_grow.HashTab[T]`** the way `priority_queue` composes `heap`: the
set is the table plus the surface, not a re-implementation.  The twin
wraps the plain set the kd_tree_ts way: an `inner hash_set.HashSet[T]`
plain **value** plus one `sync.RWMutex`.

## Why hash_grow as the base

The set-capabilities plan (§5) picked hash_grow over the other hash
families deliberately: `hash_tab` is fixed-size (a set must grow),
`hash_tab_bt`/`hash_tab_dll` are bucket experiments that exist for
their own package-family reasons, `cuckoo` has power-of-two capacity
constraints, and `sharded_hash_ts` is lock-shaped (its internals are
chains because of the SCAN cursor contract).  `hash_grow` — open
addressing, auto-double at saturation, backward-shift deletion — is the
general-purpose map-like engine.  A set never iterates in a promised
order, so the one thing open addressing is bad at (stable ordered
iteration) does not matter.

## Structure

```go
type HashSet[T any] struct {
    tab  *hash_grow.HashTab[T]
    eq   func(a, b T) bool   // the same pair the table stores,
    hash func(a T) uint64    // kept at set level for Clone/adoption
}
```

Constructors: `NewHashSet[T comparable]()` (eq `a == b`, hash
`maphash.Comparable(seed, a)` under a per-set `maphash.MakeSeed()` —
the wrapper builds the closures once and passes the SAME pair to
`hash_grow.NewHashTabFunc`, so wrapper and inner table share identical
functions) and `NewHashSetFunc(eq, hash)` (nil either → panic naming
the constructor).  `FromSlice` / `FromSliceFunc` are the convenience
forms (duplicates collapse; empty slice never panics).  Initial table:
capacity 16, saturation 0 → hash_grow's 0.5 default; the table doubles
on its own, so no options machinery is exposed.  `Eq()` / `Hash()`
expose the stored pair (nil for nil/zero-value) — the twin reads them
to apply the insert contract and to adopt at the wrapper level; that is
why they exist.

## Panic contract (intercepted at the facade)

Exactly hash_grow's, re-spelled so hash_grow's constructor names never
leak:

- `NewHashSetFunc` / `FromSliceFunc` with nil eq or nil hash — separate
  messages per function, at construction.
- A storing operation on a **nil set** → `hash_set: <Method> called on
  a nil set (a nil set cannot store an element)`.
- A storing operation on a **zero-value set** → `... called on a set
  with no equality/hash functions (create the set with NewHashSet or
  NewHashSetFunc)`.

"Storing" = `Insert`, `InsertSlice`/`InsertIter` **when they have a
non-empty batch** (empty batch → false, no contract, tolerable on nil
and zero-value), and `UnmarshalJSON` **when the decoded array has
elements**.  Everything else is a total function: nil and zero value
read as empty.  The zero-value marker is `eq == nil` (⟺ `hash == nil`
⟺ `tab == nil` — they are only ever set together); `insertContractCheck`
is the one choke point.

## JSON

Array-of-elements, bucket order (UNSPECIFIED — per-set random seed).
`UnmarshalJSON` decodes first; a decode error leaves the set untouched;
then (if there are elements) the insert contract, then Truncate +
re-insert — **contents replaced, eq/hash KEPT** (the dll precedent).
`json.Marshal` of a nil *pointer* renders `"null"` (stdlib rule, before
the method runs); `nilSet.MarshalJSON()` called directly returns `[]`.
The twin holds one write lock for the whole unmarshal.

## The algebra (avl_tree contract, verbatim in spirit)

`Union / Intersect / Minus / SymmetricDifference(yy, zz)` are
**destination form**: receiver truncated and rebuilt as the result.
Operands are snapshotted FIRST (`a, b := yy.Slice(), zz.Slice()` —
Slice is nil-tolerant), so either may alias the receiver:
`s.Union(s, s)`, `s.Minus(s, s)` (empties), `s.SymmetricDifference(s,
s)` (empties) are all legal and tested.  Adoption: `resetForAlgebra`
keeps the receiver's own pair when it has one, otherwise adopts yy's,
then zz's; **if none is adoptable the receiver stays an empty
zero-value set** (tab nil, functions nil) — the avl rule.  A nil
receiver is a no-op (`if tt == nil { return }`, avl again).

Implementation shape (hash_grow has no algebra, so it lives at the
wrapper): after adoption, build the receiver's fresh table sized by
`algebraCapacity(n) = 2n+9` (≥5) — starts under saturation, no
immediate rehash — plus private probe tables:

- Union: insert a then b.
- Intersect/Minus: probe table over b; insert a's elements
  (present / absent).
- SymmetricDifference: probe tables over BOTH a and b; insert each
  side's not-in-other.

Expected O(n+m).  All of it, Equal, and the relations carry the shared
**consistent-functions precondition** (eq true ⇒ hashes equal) —
documented, unchecked (functions are not comparable).

## Determinism traps (the per-set random seed)

Every set hashes under its own `maphash.MakeSeed()`, so bucket order
differs between any two sets and between processes:

- `Slice()` is UNSPECIFIED order — sort it when order matters.
- `String()`/`StringFunc()` sort the **formatted strings**
  (`g_lib.FormatSorted`), so two separately constructed sets over the
  same elements render identical strings — that is what makes the
  Example `// Output:` blocks stable.
- `Equal`/relations/algebra compare **membership**, never order.
- Never assert a fixed slot order, JSON array order, `All()` order, or
  `Walk` order in tests — sort first.

## The twin's lock discipline

`hash_set_ts.HashSet[T]{inner hash_set.HashSet[T]; lock sync.RWMutex}`.
Nil guards run BEFORE lock acquisition (kd_tree_ts).  The Nl surface is
exactly `NlInsert`/`NlDelete`/`NlSearch`/`NlLen`/`NlIsEmpty` — direct
inner calls (so `NlInsert`'s contract panic carries the plain package's
message; documented).  Batch mutators take ONE write lock for the whole
compound; `InsertIter` drains the iterator BEFORE the lock; DeleteFunc
snapshots under RLock, runs fx **unlocked** (fx may call back in —
tested), deletes doomed under one write lock (collect-then-delete is
mandatory under hash_grow's backward-shift deletion).
`StringFunc`/`Equal`/`Clone`/`All`/relations snapshot then compute
unlocked; **`Walk` holds the read lock for the whole walk** — its
callback must NOT call back in (the one documented deadlock hazard).
Two-set operations never hold two locks: each operand snapshotted under
its own lock, released; the algebra computes on local
`hash_set.FromSliceFunc` sets with no lock held and swaps the receiver
via one `t.inner = fresh` write-lock assignment — atomic to readers,
deadlock-free in opposite directions (tested by
`TestHashSetTSNoDeadlockOppositeDirections`).  The insert-contract
panics are re-spelled `hash_set_ts` at the twin (`insertContract`
checks `inner.Eq() == nil` under the held lock).

## Tests (§8 of the plan)

- Randomized model vs `map[T]bool`, **fixed seeds 42 AND 7** (800 steps
  each: insert/delete/search/contains/InsertSlice/DeleteSlice with
  return-value checks every step).
- Algebra property battery: commutativity of Union/Intersect/SymDiff,
  `s△t = (s∪t)−(s∩t)` law, alias forms (`x.Union(x,x)` etc.), zero-value
  adoption, nil-receiver no-ops, map-computed references.
- Subset laws: `s⊆s`, proper-of-self false, `∅⊆s`, `s⊆s∪t`, `s∩t⊆s`;
  disjointness of two empties.
- DeleteFunc on 1000 elements deleting half, then FULL membership
  re-verification (collect-then-delete must not lose neighbors).
- Clone deep-copy in both directions; Equal symmetric/reflexive; String
  determinism across separately seeded sets; FromSlice dedup.
- JSON round trip keeping a case-insensitive eq/hash pair (verified
  post-unmarshal via case-mismatched Search/Delete).
- Panic contract with message-content checks (constructor nils, nil
  set, zero-value set, per-method names).
- Twin: the race trio, plain-vs-twin 500-step parity script, batch
  atomicity (compound reader via Lock+Nl* must see all-or-nothing),
  no-deadlock opposite-direction Equal/algebra, unlocked-callback
  proofs (DeleteFunc and StringFunc calling back in).

## Benchmarks

`hash_set/bench_test.go`: HashSet vs tree_set vs the `map[T]bool`
driver at n = 1k and 100k over int keys (insert-all + mixed
lookup/delete phases) — the honest constants tradition: the runtime
map (SWISS, incremental growth, specialized hashing) wins
insert-heavy work and that is documented in the file, not hidden; the
mixed phase is close.  The tree_set arm (one node allocation per
element, ~3–8× HashSet's constants across both phases at n=100k) is
the price of its ordered riders.  `tree_set/bench_test.go` runs the
mirror three-way over string keys.
