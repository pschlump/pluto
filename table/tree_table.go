/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"cmp"
	"fmt"
	"iter"
)

// TreeTable is a sparse table with comparator-ordered rows and columns.
type TreeTable[R any, C any, V any] struct {
	rows       *treeIndex[R, *treeIndex[C, V]]
	rowCompare func(R, R) int
	colCompare func(C, C) int
	cellCount  int
}

// NewTreeTable creates an empty table using natural row and column ordering.
func NewTreeTable[R cmp.Ordered, C cmp.Ordered, V any]() *TreeTable[R, C, V] {
	return NewTreeTableFunc[R, C, V](cmp.Compare[R], cmp.Compare[C])
}

// NewTreeTableFunc creates an empty table with custom row/column comparators.
func NewTreeTableFunc[R any, C any, V any](rowCompare func(R, R) int, colCompare func(C, C) int) *TreeTable[R, C, V] {
	if rowCompare == nil || colCompare == nil {
		panic("table: NewTreeTableFunc requires non-nil comparison functions")
	}
	return &TreeTable[R, C, V]{
		rows:       newTreeIndex[R, *treeIndex[C, V]](rowCompare),
		rowCompare: rowCompare, colCompare: colCompare,
	}
}

func (t *TreeTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.rows == nil {
		panic(fmt.Sprintf("table: %s on a nil or zero-value TreeTable; use a constructor", caller))
	}
}

func (t *TreeTable[R, C, V]) Put(row R, column C, value V) (V, bool) {
	t.writeContract("Put")
	columns, found := t.rows.get(row)
	if !found {
		columns = newTreeIndex[C, V](t.colCompare)
		t.rows.put(row, columns)
	}
	old, replaced := columns.put(column, value)
	if !replaced {
		t.cellCount++
	}
	return old, replaced
}

func (t *TreeTable[R, C, V]) Get(row R, column C) (V, bool) {
	if t == nil || t.rows == nil {
		var zero V
		return zero, false
	}
	columns, found := t.rows.get(row)
	if !found {
		var zero V
		return zero, false
	}
	return columns.get(column)
}

func (t *TreeTable[R, C, V]) Delete(row R, column C) (V, bool) {
	if t == nil || t.rows == nil {
		var zero V
		return zero, false
	}
	columns, found := t.rows.get(row)
	if !found {
		var zero V
		return zero, false
	}
	old, deleted := columns.delete(column)
	if deleted {
		t.cellCount--
		if columns.len() == 0 {
			t.rows.delete(row)
		}
	}
	return old, deleted
}

func (t *TreeTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

func (t *TreeTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil || t.rows == nil {
		return false
	}
	_, found := t.rows.get(row)
	return found
}

func (t *TreeTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.rows == nil {
		return false
	}
	for _, columns := range t.rows.all() {
		if _, found := columns.get(column); found {
			return true
		}
	}
	return false
}

func (t *TreeTable[R, C, V]) Len() int {
	if t == nil {
		return 0
	}
	return t.cellCount
}

func (t *TreeTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

func (t *TreeTable[R, C, V]) Clear() {
	if t == nil || t.rows == nil {
		return
	}
	t.rows = newTreeIndex[R, *treeIndex[C, V]](t.rowCompare)
	t.cellCount = 0
}

// Cells iterates in ascending row-major order. Do not mutate while iterating.
func (t *TreeTable[R, C, V]) Cells() iter.Seq[Cell[R, C, V]] {
	return func(yield func(Cell[R, C, V]) bool) {
		if t == nil || t.rows == nil {
			return
		}
		for row, columns := range t.rows.all() {
			for column, value := range columns.all() {
				if !yield(Cell[R, C, V]{Row: row, Column: column, Value: value}) {
					return
				}
			}
		}
	}
}

// treeRowCells copies one row in column order into an exactly sized slice
// (one allocation, no iterator closure).
func treeRowCells[R any, C any, V any](row R, columns *treeIndex[C, V]) []Cell[R, C, V] {
	return appendRowCells(make([]Cell[R, C, V], 0, columns.count), row, columns.root)
}

func (t *TreeTable[R, C, V]) Row(row R) []Cell[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	columns, found := t.rows.get(row)
	if !found {
		return []Cell[R, C, V]{}
	}
	return treeRowCells(row, columns)
}

func (t *TreeTable[R, C, V]) Rows() iter.Seq2[R, []Cell[R, C, V]] {
	type group struct {
		row   R
		cells []Cell[R, C, V]
	}
	groups := []group{}
	if t != nil && t.rows != nil {
		for row, columns := range t.rows.all() {
			groups = append(groups, group{row: row, cells: treeRowCells(row, columns)})
		}
	}
	return func(yield func(R, []Cell[R, C, V]) bool) {
		for _, group := range groups {
			if !yield(group.row, copyCells(group.cells)) {
				return
			}
		}
	}
}

func (t *TreeTable[R, C, V]) Columns() iter.Seq2[C, []Cell[R, C, V]] {
	type group struct {
		column C
		cells  []Cell[R, C, V]
	}
	snapshot := []group{}
	if t != nil && t.rows != nil && t.cellCount > 0 {
		groups := t.columnGroups()
		cells := make([]Cell[R, C, V], len(groups.rows))
		for index := range cells {
			cells[index] = Cell[R, C, V]{Row: groups.rows[index], Column: groups.columns[groups.owner[index]], Value: groups.values[index]}
		}
		snapshot = make([]group, len(groups.columns))
		for index, column := range groups.columns {
			snapshot[index] = group{column: column, cells: cells[groups.offsets[index]:groups.offsets[index+1]:groups.offsets[index+1]]}
		}
	}
	return func(yield func(C, []Cell[R, C, V]) bool) {
		for _, group := range snapshot {
			if !yield(group.column, copyCells(group.cells)) {
				return
			}
		}
	}
}

// treeColumnGroups is the column-major regrouping of a TreeTable: the distinct
// columns in ascending order, and the cells' rows and values laid out column
// by column, each column's run in ascending row order. Column k owns
// rows[offsets[k]:offsets[k+1]]; owner maps a flat position to its column.
type treeColumnGroups[R any, C any, V any] struct {
	columns []C
	offsets []int
	owner   []int
	rows    []R
	values  []V
}

// columnGroups regroups the cells column-major in two passes over the
// row-major index.
//
// Pass one assigns every cell a column bucket. Each row's columns arrive in
// ascending order, so they are first matched by a merge cursor against the
// previous row's (column, bucket) list - one comparison per cell when rows
// share their columns - and only a column the previous row lacked is looked up
// (or added) in a temporary column index. The cursor advances at most the
// previous row's width per row, so the pass is O(cells) for regular tables and
// O(cells log columns) at worst. Pass two counting-sorts the cells into their
// buckets' runs; walking rows in ascending order keeps every run row-sorted.
func (t *TreeTable[R, C, V]) columnGroups() treeColumnGroups[R, C, V] {
	type guess struct {
		column C
		bucket int
	}
	buckets := newTreeIndex[C, int](t.colCompare)
	assigned := make([]int, 0, t.cellCount)
	counts := []int{}
	var previous, current []guess
	for _, columns := range t.rows.all() {
		current = current[:0]
		cursor := 0
		for column := range columns.all() {
			bucket := -1
			for cursor < len(previous) {
				c := t.colCompare(previous[cursor].column, column)
				if c < 0 {
					cursor++
					continue
				}
				if c == 0 {
					bucket = previous[cursor].bucket
					cursor++
				}
				break
			}
			if bucket < 0 {
				var found bool
				if bucket, found = buckets.get(column); !found {
					bucket = len(counts)
					buckets.put(column, bucket)
					counts = append(counts, 0)
				}
			}
			counts[bucket]++
			assigned = append(assigned, bucket)
			current = append(current, guess{column: column, bucket: bucket})
		}
		previous, current = current, previous
	}

	groups := treeColumnGroups[R, C, V]{
		columns: make([]C, 0, len(counts)),
		offsets: make([]int, len(counts)+1),
		owner:   make([]int, len(assigned)),
		rows:    make([]R, len(assigned)),
		values:  make([]V, len(assigned)),
	}
	next := make([]int, len(counts)) // bucket -> next flat position
	for column, bucket := range buckets.all() {
		next[bucket] = groups.offsets[len(groups.columns)]
		groups.offsets[len(groups.columns)+1] = next[bucket] + counts[bucket]
		counts[bucket] = len(groups.columns) // reuse: bucket -> sorted column ordinal
		groups.columns = append(groups.columns, column)
	}
	cell := 0
	for row, columns := range t.rows.all() {
		for _, value := range columns.all() {
			bucket := assigned[cell]
			cell++
			position := next[bucket]
			next[bucket]++
			groups.rows[position], groups.values[position] = row, value
			groups.owner[position] = counts[bucket]
		}
	}
	return groups
}

// Column returns a fresh snapshot in ascending row order.
// Complexity is O(rows log width).
func (t *TreeTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	cells := make([]Cell[R, C, V], 0, columnCapacityHint(t.rows.count, t.cellCount))
	for row, columns := range t.rows.all() {
		if node := columns.node(column); node != nil {
			cells = append(cells, Cell[R, C, V]{Row: row, Column: node.key, Value: node.value})
		}
	}
	return cells
}

func (t *TreeTable[R, C, V]) RowKeys() iter.Seq[R] {
	return func(yield func(R) bool) {
		for row := range t.Rows() {
			if !yield(row) {
				return
			}
		}
	}
}

func (t *TreeTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	return func(yield func(C) bool) {
		for column := range t.Columns() {
			if !yield(column) {
				return
			}
		}
	}
}

func (t *TreeTable[R, C, V]) Transpose() *TreeTable[C, R, V] {
	if t == nil || t.rows == nil {
		return &TreeTable[C, R, V]{}
	}
	result := NewTreeTableFunc[C, R, V](t.colCompare, t.rowCompare)
	if t.cellCount == 0 {
		return result
	}
	// Regroup column-major, then build every transposed row (and the row
	// index itself) directly from its sorted run: perfectly balanced trees,
	// one node slab per row, no comparisons and no rotations.
	groups := t.columnGroups()
	inner := make([]treeIndex[R, V], len(groups.columns))
	rowIndexes := make([]*treeIndex[R, V], len(groups.columns))
	for index := range groups.columns {
		from, to := groups.offsets[index], groups.offsets[index+1]
		inner[index] = buildTreeIndex(t.rowCompare, groups.rows[from:to], groups.values[from:to])
		rowIndexes[index] = &inner[index]
	}
	outer := buildTreeIndex(t.colCompare, groups.columns, rowIndexes)
	result.rows = &outer
	result.cellCount = t.cellCount
	return result
}

// Clone returns an independent ordered table with the same comparators.
// Each row is copied structurally into one node slab; nothing is compared.
// Complexity is O(cells).
func (t *TreeTable[R, C, V]) Clone() *TreeTable[R, C, V] {
	if t == nil {
		return nil
	}
	if t.rows == nil {
		return &TreeTable[R, C, V]{}
	}
	result := NewTreeTableFunc[R, C, V](t.rowCompare, t.colCompare)
	result.rows = t.rows.clone()
	var stack [maxTreeHeight]*treeNode[R, *treeIndex[C, V]]
	depth := 0
	if result.rows.root != nil {
		stack[0] = result.rows.root
		depth = 1
	}
	for depth > 0 {
		depth--
		node := stack[depth]
		node.value = node.value.clone()
		if node.left != nil {
			stack[depth] = node.left
			depth++
		}
		if node.right != nil {
			stack[depth] = node.right
			depth++
		}
	}
	result.cellCount = t.cellCount
	return result
}
