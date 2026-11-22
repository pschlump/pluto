/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"cmp"
	"fmt"
	"iter"

	"github.com/pschlump/pluto/avl_tree"
)

type treeEntry[K any, V any] struct {
	key   K
	value V
}

type treeIndex[K any, V any] struct {
	entries *avl_tree.AvlTree[treeEntry[K, V]]
}

func newTreeIndex[K any, V any](compare func(K, K) int) *treeIndex[K, V] {
	return &treeIndex[K, V]{
		entries: avl_tree.NewAvlTreeFunc(func(a, b treeEntry[K, V]) int { return compare(a.key, b.key) }),
	}
}

func (index *treeIndex[K, V]) get(key K) (V, bool) {
	entry, found := index.entries.Search(treeEntry[K, V]{key: key})
	return entry.value, found
}

func (index *treeIndex[K, V]) put(key K, value V) (V, bool) {
	old, found := index.get(key)
	index.entries.Insert(treeEntry[K, V]{key: key, value: value})
	return old, found
}

func (index *treeIndex[K, V]) delete(key K) (V, bool) {
	old, found := index.get(key)
	if found {
		index.entries.Delete(treeEntry[K, V]{key: key})
	}
	return old, found
}

func (index *treeIndex[K, V]) len() int { return index.entries.Len() }

func (index *treeIndex[K, V]) all() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for entry := range index.entries.All() {
			if !yield(entry.key, entry.value) {
				return
			}
		}
	}
}

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

func treeRowCells[R any, C any, V any](row R, columns *treeIndex[C, V]) []Cell[R, C, V] {
	cells := make([]Cell[R, C, V], 0, columns.len())
	for column, value := range columns.all() {
		cells = append(cells, Cell[R, C, V]{Row: row, Column: column, Value: value})
	}
	return cells
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
	groups := newTreeIndex[C, []Cell[R, C, V]](func(a, b C) int {
		if t == nil || t.colCompare == nil {
			return 0
		}
		return t.colCompare(a, b)
	})
	if t != nil && t.rows != nil {
		for cell := range t.Cells() {
			cells, _ := groups.get(cell.Column)
			groups.put(cell.Column, append(cells, cell))
		}
	}
	type group struct {
		column C
		cells  []Cell[R, C, V]
	}
	snapshot := []group{}
	if t != nil && t.rows != nil {
		for column, cells := range groups.all() {
			snapshot = append(snapshot, group{column: column, cells: cells})
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

func (t *TreeTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	cells := []Cell[R, C, V]{}
	if t == nil || t.rows == nil {
		return nil
	}
	for row, columns := range t.rows.all() {
		if value, found := columns.get(column); found {
			cells = append(cells, Cell[R, C, V]{Row: row, Column: column, Value: value})
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
	for cell := range t.Cells() {
		result.Put(cell.Column, cell.Row, cell.Value)
	}
	return result
}

func (t *TreeTable[R, C, V]) Clone() *TreeTable[R, C, V] {
	if t == nil {
		return nil
	}
	if t.rows == nil {
		return &TreeTable[R, C, V]{}
	}
	result := NewTreeTableFunc[R, C, V](t.rowCompare, t.colCompare)
	for cell := range t.Cells() {
		result.Put(cell.Row, cell.Column, cell.Value)
	}
	return result
}
