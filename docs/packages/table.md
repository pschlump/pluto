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
| Get / Contains cell | expected O(1) | O(log r + log c) | O(1) |
| Put / Delete | expected O(1) | O(log r + log c) | O(1) |
| Row | O(width) | O(log r + width) | O(columns) |
| Column | O(cells) | O(cells) | O(rows) |
| Rows / Columns | O(cells) | O(cells log c) for columns | O(rows x columns) |
| Cells | O(cells) | O(cells), ordered | O(rows x columns) |
| Transpose | O(cells) | O(cells log cells) | O(rows x columns) |

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
