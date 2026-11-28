# table / table_ts

## Public contract and complexity

`Cell[R,C,V]` contains `Row`, `Column`, and `Value`. `HashTable`, `TreeTable`,
and `ArrayTable` associate a value with each present row/column pair. `Put` and
`Delete` return the previous value and whether it was present. A stored zero
value is present. `Len` counts cells, `Clear` removes cells, and `Contains`,
`ContainsRow`, and `ContainsColumn` do not confuse absence with a zero value.

`Row(row)` and `Column(column)` return fresh `[]Cell` snapshots. `Rows()` and
`Columns()` yield a key and a fresh cell slice for each row or column. `Cells()`
is an early-stoppable cell iterator; `RowKeys()` and `ColumnKeys()` return key
snapshots. No returned slice is a live write-through view.

| Operation | HashTable | TreeTable | ArrayTable |
|---|---|---|---|
| Get / Contains cell | expected O(1) | O(log r + log c) | expected O(1) |
| Put / Delete | expected O(1) | O(log r + log c) | expected O(1) |
| Row | O(width) | O(log r + width) | O(columns) |
| Column | expected O(rows) | O(rows log c) | O(rows) |
| Rows / Columns | O(cells) | O(cells); Columns O(cells log c) worst | O(rows x columns) |
| Cells | O(cells) | O(cells), ordered | O(rows x columns) |
| Transpose | expected O(cells) | O(cells); O(cells log c) worst | O(rows x columns) |
| Clone | O(slots) | O(cells) | O(rows x columns) |

Hash lookup uses `==` and `maphash` with `NewHashTable`; `NewHashTableFunc`
accepts matching equality/hash functions for arbitrary key types. Hash iteration
order is unspecified. Tree construction uses natural ordering or explicit row
and column comparators. Cells are row-major ordered, rows and columns are
ordered, and mutations use AVL trees rather than sorted-slice scans.

Sparse tables delete empty rows. Their row and column key sets contain only
keys with at least one cell. Column operations scan the row-major index.

## Dense universes and indexed access

ArrayTable `Put` and `Delete` return `(previous, found, error)`;
out-of-universe mutation returns `ErrOutsideUniverse`. Sparse variants return
`(previous, found)`.

`NewArrayTable(rows, columns)` fixes complete row and column universes. Duplicate
keys collapse to their first occurrence, as with Guava's immutable-set universe
construction. `ContainsRow` and `ContainsColumn` report universe membership even
when the row or column has no present cells. `Rows`, `Columns`, `RowKeys`, and
`ColumnKeys` likewise retain the complete universes. Out-of-universe writes
return an error rather than silently expanding the matrix.

`GetAt`, `PutAt`, and `DeleteAt` use zero-based universe positions for direct
indexed access. `PutAt` and `DeleteAt` panic for invalid indexes; `GetAt` reports
not found. `PutAt` returns the previous cell value. A parallel `present` bitmap
distinguishes absent cells from stored zero values.

## Transpose, immutable snapshots, and JSON

`Transpose` returns an independent table with row and column types exchanged.
`Freeze` returns an independent `ImmutableTable` with no mutation API. Hash and
dense snapshots use hash lookup; ordered snapshots retain an AVL index and use
binary-searched row/column snapshots. All preserve source iteration order and are
safe for concurrent reads. Its `Transpose` and `Freeze` operations remain
read-only. Referenced payload values are not deep-copied.

All mutable tables marshal as arrays of cells with `row`, `column`, and `value`
fields. Unmarshal validates a complete replacement before mutation and retains
constructors' functions and dense universes. Immutable snapshots marshal but do
not implement mutation through JSON.

## Thread safety and verification

`table_ts` wraps the mutable variants with `sync.RWMutex`. Compound operations,
including transpose and JSON replacement, are atomic. Iterators and returned
rows/columns are snapshots; no lock is held while caller iterators or JSON code
run. Plain tables are not safe for concurrent mutation.

Fixed-seed models compare sparse tables with nested maps and dense tables with a
matrix. Tests cover empty-key cleanup, comparator order, fixed universes, zero
values, transpose, immutable independence, JSON, and concurrent snapshots. The
opt-in comparison harness runs pinned Guava table oracles and matched sparse,
ordered, dense, transpose, row/column, and memory benchmarks. Ordinary tests read
committed golden files only. Raw and summarized measurements are retained under
`docs/benchmarks/table/`; snapshot API costs are reported separately from Guava's
live-view row/column access.

## Live write-through views

`RowView(row)`, `ColumnView(column)`, and `CellView(row,column)` are explicit
live views for HashTable, TreeTable, ArrayTable, and their `_ts` twins. Get/Put/
Delete consult and mutate the parent. Row/column Clear deletes currently present
cells. Dense views return `ErrOutsideUniverse` for out-of-universe writes.
Existing Row, Column, Rows, Columns, and Cells snapshot APIs are unchanged.

Each view `Cells` or `All` call takes a fresh iteration snapshot. Length and
iteration inherit the parent row/column complexity. `_ts` views lock each parent
operation independently; no lock is held while yielding and multi-step view
sequences are not transactions. Tests cover both mutation directions, row/column
cleanup, dense universe errors, ordered views, and concurrent row mutation.

## Private indexes and performance invariants

The table package does not use `hash_grow` or `avl_tree`; it owns two private
indexes tuned for this package (see the 2026-11-27 section of
`docs/benchmarks/table/README.md` for the measurements behind them).

`hashIndex` (`table/hash_index.go`) backs HashTable, the custom-function
ArrayTable universe and ImmutableTable positions. Linear probing over a
power-of-two array of `{hash, key, value}` slots, maximum load 3/4, home slot
by Fibonacci multiply of the cached hash, backward-shift deletion (no
tombstones; the vacated slot is zeroed). Invariants editors must keep:

- A slot's cached `hash` is `fixHash(callerHash(key))`; 0 means empty, so a
  caller hash of exactly 0 is stored as 1. Every path that probes with a
  precomputed hash (Column, ContainsColumn, Columns, Transpose) must pass the
  fixed hash.
- Every write hashes its key once: `findOrAllocate` then `insertAt` (which may
  grow and relocates the target slot - always use its returned position).
- An equal key replaces the whole entry, key included (the hash_grow `Insert`
  semantics callers relied on).
- `insertUnique` skips equality and is only for keys known to be absent
  (Transpose destination rows: source rows are distinct).
- Transpose reuses cached hashes: the destination row is found by the source
  slot's column hash, the source row is inserted under the outer slot's row
  hash. Its first pass tallies widths in `count` and then `reserve`s each
  destination row, so no destination row grows. Clone copies slot arrays
  verbatim.
- The slot array never shrinks; `clear` releases it.

`treeIndex` (`table/tree_index.go`) backs TreeTable: an AVL tree with key and
value in the node. `put`/`delete` descend once, record link addresses in a
fixed `[maxTreeHeight]` stack array and rebalance bottom-up; a replacement
updates the node in place (key and value) and allocates nothing. Traversals
use method recursion or an explicit stack - never a recursive closure, which
heap-allocates. Unlinked nodes are zeroed. `checkTreeIndex` in
`perf_model_test.go` validates order, cached heights, balance and count.

Bulk paths:

- `buildTreeIndex` builds a perfectly balanced tree from strictly ascending
  entries into one node slab; `clone` copies a tree's shape into one slab. A
  node later deleted from a slab stays allocated until the whole slab is
  unreachable; slabs are per row (and one for the row index), so the extra
  retention is bounded by one row's size at build time.
- `columnGroups` regroups a TreeTable column-major for Transpose and Columns:
  pass one assigns buckets by a merge cursor against the previous row's sorted
  (column, bucket) list, falling back to a temporary column index only for
  columns the previous row lacked; pass two counting-sorts. Walking rows in
  ascending order is what makes every column run row-sorted - keep it.
  `TestTransposeWork` pins the comparison budget (a cursor that stops skipping
  or matching still produces correct output, so only the count catches it).

ArrayTable: `NewArrayTable` keeps its universe in built-in maps behind
`rowIndex`/`colIndex` position closures (about twice as fast as maphash
through `hashIndex`); `NewArrayTableFunc` closes over a `hashIndex`. The
universe (key slices, lookups, key functions) is immutable after construction
and is shared by Clone, Transpose (axes swapped), Replace and UnmarshalJSON,
also across `table_ts` twins - never write to `rowKeys`/`colKeys` (RowKeys and
ColumnKeys copy). `rowIndex == nil` is the zero-value marker. Transpose is a
direct matrix transpose.

Snapshot copies allocate once: Row sizes its slice exactly (row count, or a
present-cell count for ArrayTable), Column preallocates
`columnCapacityHint(rows, cells)` = `min(rows, cells, 1024)` cells (so a sparse
column in a huge table does not reserve O(rows) memory) or counts exactly for
ArrayTable. `TestSnapshotAllocations` pins one allocation per Row/Column and
zero for Get and replacing Put. `table_ts` presizes its `Cells` snapshot with
`Len` under the read lock.

Rejected (measured, do not retry without new evidence): struct-of-arrays hash
slots (no mixed gain, slower Transpose, more allocations); a `<`-then-`==`
natural comparator instead of `cmp.Compare` (slower for string keys).
