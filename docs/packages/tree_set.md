# tree_set / tree_set_ts

The algs4 SET analog: a sorted set of unique elements over
`avl_tree.AvlTree[T]`.  Phase 7 of the set-capabilities plan.  The
plain package is a thin facade (the `priority_queue`-over-`heap`
composition precedent); the twin is the kd_tree_ts wrapper pattern.

## Structure

`tree_set.TreeSet[T any]` holds `inner *avl_tree.AvlTree[T]` plus
`cmp func(a, b T) int` — the facade's own copy of the comparator
(tree_set cannot read the wrapped tree's).  Invariant: `inner != nil`
iff `cmp != nil`; the two states are the zero value `{nil, nil}` and
constructor-built `{tree, cmp}`.  Two things read the own copy:
`Range` (bounds comparisons) and the algebra's adoption (below).  Most
reads delegate through the unexported `innerOf()` helper, which returns
nil for a nil set / zero value — every avl_tree operation is
nil-tolerant on a nil tree, so delegation needs no per-method guards.
`ApplyFunction` is a type alias of avl_tree's.

Why avl_tree and not skip_list: the plan (§6) picked avl_tree because
its set-op machinery (snapshot-then-rebuild Union/Intersect/Minus/
SymmetricDifference, adoptCmp, capabilities, JSON) is already proven —
the facade is delegation, not re-implementation.  skip_list remains the
recorded fallback only.

## Facade decisions

- **Panic interception is the whole reason the facade is not pure
  delegation**: avl_tree's contract messages name avl constructors.
  `insertContractCheck` produces the priority_queue-style combined
  message — `tree_set: Insert called on a nil or zero-value set (create
  the set with NewTreeSet or NewTreeSetFunc)` — for `Insert`,
  `InsertSlice`, `InsertIter` and `UnmarshalJSON` (only when they would
  actually store; empty batches are tolerated).  Plus the constructor
  panics `NewTreeSetFunc(nil)` / `FromSliceFunc(nil, …)`.  That is the
  entire panic surface.
- **Algebra**: facade-level `adoptCmp(yy, zz)` syncs the own copy (a
  zero-value set adopts yy's ordering, then zz's), then delegate to
  `inner.Union/...` with `innerOf()` operands (nil operands are the
  empty set inside avl's nil-tolerant ops).  Do NOT truncate the
  receiver before delegating — avl's ops snapshot their operands
  themselves before resetting the destination, which is what makes
  `s.Union(s, s)` work; truncating at the facade would destroy an
  aliasing operand first.  When no set in sight has a comparator the
  receiver stays/returns to the zero value.
- **JSON is facade-level, not delegated**: `UnmarshalJSON` cannot
  delegate (avl's messages again), so it mirrors avl's decode-then-
  rebuild: decode error leaves the set untouched; `[]` clears
  everywhere (Truncate keeps cmp); elements stored only after a
  successful decode.
- **Deliberately NOT exposed** from avl: `Copy` (destination-form — the
  §2 rule is Clone-only), `Reverse` (breaks the ordering invariant the
  set's identity rests on), `Depth`/`Dump` (tree internals, not set
  vocabulary), `Front`-style iterators (`All`/`Backward`/`Range`
  cover iteration).
- **No no-op `Lock`/`Unlock` on the plain package** — unlike
  heap/priority_queue.  Reason: the twin holds the plain by VALUE
  (`inner tree_set.TreeSet[T]`), and a type with Lock/Unlock methods is
  a sync.Locker to `go vet`'s copylocks, which would reject every
  `t.inner = *out` install in the twin.  avl_tree and kd_tree (the
  other wrapper bases) have none either; heap/priority_queue composites
  hold their base by pointer, so they can afford the no-ops.

## Range — the one non-delegated read

`Range(lo, hi T) iter.Seq[T]`: lo ≤ x ≤ hi, ascending, inclusive, empty
when lo > hi.  Implemented as a filtered walk of `inner.All()` with an
early stop past hi, using the facade's own `cmp` for the bound
comparisons.  Deliberately diverges from skip_list.Range on both
counts: it is `iter.Seq[T]` not `iter.Seq2[int, T]` (a set element has
no rank in iteration), and it is O(n) worst case, not skip_list's
O(log₂ n + m) positioned descent — avl_tree offers no positioned-start
descent for a facade to reuse, and the plan records that price rather
than pay a skip_list base for one rider.

## tree_set_ts — the cross-set comparator problem

The twin cannot delegate algebra to the plain package: the operands
must be read under THEIR locks, and plain `Union` reads its operands
unlocked (a race).  It also cannot hold two operand locks while
delegating (deadlock in opposite directions).  Solution — the twin's
own `cmp` copy plus snapshot-then-rebuild:

1. `yy.snapshot()` / `zz.snapshot()` return `(elements, comparator)`
   read together under that operand's own read lock, then released.
   Elements and cmp MUST come from one lock hold: cmp is written by the
   adoption step, so reading it unlocked is a race.
2. Under the receiver's write lock: adopt (own `adoptCmp`), rebuild
   locally on plain sets (`FromSliceFunc(t.cmp, a)` + plain algebra),
   `t.inner = *out` — one value copy, legal because the plain package
   is not a Locker (above).

Never two locks at once — that is the whole deadlock-freedom argument,
and `TestTwinNoDeadlockOppositeDirections` exercises it in both
directions.  The documented cost: an algebra op is atomic per set, not
across sets.

Everything else follows the standard twin discipline: read lock for
whole reads; ONE write lock for whole batches (atomicity:
`TestTwinBatchAtomicity` observes Len ∈ {0, n} only); snapshot
iterators (`All`/`Backward`/`Range` — mutate from inside the loop,
legal); live walks under the read lock (callback must not call back);
`StringFunc`/`DeleteFunc` callbacks run unlocked on snapshots;
`InsertIter` drains before the write lock (own-All feeding is safe).

`Nl*` surface: `NlInsert`, `NlDelete`, `NlSearch`, `NlContains`,
`NlLen`, `NlIsEmpty` — direct inner calls.  A store through `NlInsert`
on a zero-value twin lands in the PLAIN package's contract check, so
its message carries the `tree_set:` prefix — same asymmetry as
kd_tree_ts's NaN-through-NlInsert, documented in the twin README.

Write-contract checks are ordered for race-freedom: the nil-receiver
check panics before any lock (a nil `t.lock` would deref); the
zero-value check reads `t.cmp` under the write lock it is about to
violate.  The twin's zero-value detection is `t.cmp == nil` — sound
because the invariant "built ⇔ cmp set" holds at the twin level too.

## Contracts to preserve when changing

- `Insert` returns true iff new; a duplicate REPLACES the stored value
  (satellite data included — `Union` keeps zz's element on a
  cross-operand duplicate, the avl rule).
- Destination-form algebra: the receiver is REPLACED by yy ∪/∩/−/△ zz
  (not merged into); a filled receiver `Union(nil, nil)` becomes empty.
- The empty set is a subset of every set, disjoint with every set;
  `IsProperSubsetOf` requires strict size inequality; `IsSupersetOf` is
  literally `other.IsSubsetOf(t)`.
- `Equal` is membership, never shape; symmetric under the
  consistent-comparator precondition (functions are not comparable —
  the precondition is documented, not checked, everywhere it applies).
- `DeleteAtHead`/`DeleteAtTail` return only `bool` (avl's signature —
  to learn WHICH element went, read `Index(0)`/`Index(Len-1)` first, as
  the drain-order test does).
- `Slice`: nil set → nil; empty built set → empty non-nil; always a
  fresh copy, ascending.

## Test-oracle expectations

- The model battery uses `map[string]bool` with FIXED seeds 42 AND 7
  (800 steps), checking Len/Slice/Contains against the model after
  every step.
- The twin's parity test runs the same seeded op sequence against the
  plain package and asserts identical observable state after each step.
- The comparative benchmark (`tree_set/bench_test.go`, seed 42,
  1k/100k string keys): TreeSet vs hash_set vs `map[string]bool` on
  insert-all/hit-miss-lookup/delete-half.  Recorded reality on this
  machine at n=100k: map lookup ~1.2ms, HashSet ~1.6ms, TreeSet ~12ms
  (~10× the map); map insert ~1.4ms, HashSet ~4.0ms, TreeSet ~24ms,
  allocating one node (~480 B) per element — the honest constants that
  justify the set being bought for ORDER (Floor/Ceil/Range/rank
  riders), not raw lookup speed.  `hash_set/bench_test.go` runs the
  mirror three-way over int keys with the same conclusion.
- avl_tree's own note applies underneath: ties poison mirror oracles —
  the satellite-replacement tests here use distinct payloads.
