/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"fmt"
	"iter"
)

// ArrayTable is a dense fixed-universe table. Absence is tracked separately
// from values, so the zero V is an ordinary present value.
type ArrayTable[R any, C any, V any] struct {
	rowKeys   []R
	colKeys   []C
	rows      *hashIndex[R, int]
	columns   *hashIndex[C, int]
	rowEqual  func(R, R) bool
	rowHash   func(R) uint64
	colEqual  func(C, C) bool
	colHash   func(C) uint64
	values    []V
	present   []bool
	cellCount int
}

// NewArrayTable creates a dense table over the Cartesian product of rows and
// columns. Duplicate universe keys collapse to their first occurrence.
func NewArrayTable[R comparable, C comparable, V any](rows []R, columns []C) *ArrayTable[R, C, V] {
	rowEqual, rowHash := naturalFunctions[R]()
	colEqual, colHash := naturalFunctions[C]()
	return NewArrayTableFunc[R, C, V](rows, columns, rowEqual, rowHash, colEqual, colHash)
}

// NewArrayTableFunc creates a dense table with custom universe key functions.
func NewArrayTableFunc[R any, C any, V any](rows []R, columns []C, rowEqual func(R, R) bool, rowHash func(R) uint64, colEqual func(C, C) bool, colHash func(C) uint64) *ArrayTable[R, C, V] {
	if rowEqual == nil || rowHash == nil || colEqual == nil || colHash == nil {
		panic("table: NewArrayTableFunc requires non-nil equality and hash functions")
	}
	t := &ArrayTable[R, C, V]{
		rows:     newHashIndex[R, int](rowEqual, rowHash),
		columns:  newHashIndex[C, int](colEqual, colHash),
		rowEqual: rowEqual, rowHash: rowHash, colEqual: colEqual, colHash: colHash,
	}
	for _, row := range rows {
		if _, found := t.rows.get(row); found {
			continue
		}
		t.rows.put(row, len(t.rowKeys))
		t.rowKeys = append(t.rowKeys, row)
	}
	for _, column := range columns {
		if _, found := t.columns.get(column); found {
			continue
		}
		t.columns.put(column, len(t.colKeys))
		t.colKeys = append(t.colKeys, column)
	}
	if len(t.rowKeys) != 0 && len(t.colKeys) > int(^uint(0)>>1)/len(t.rowKeys) {
		panic("table: ArrayTable universe size overflow")
	}
	t.values = make([]V, len(t.rowKeys)*len(t.colKeys))
	t.present = make([]bool, len(t.values))
	return t
}

func (t *ArrayTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.rows == nil {
		panic(fmt.Sprintf("table: %s on a nil or zero-value ArrayTable; use a constructor", caller))
	}
}

func (t *ArrayTable[R, C, V]) position(rowIndex, columnIndex int) (int, bool) {
	if t == nil || rowIndex < 0 || rowIndex >= len(t.rowKeys) || columnIndex < 0 || columnIndex >= len(t.colKeys) {
		return 0, false
	}
	return rowIndex*len(t.colKeys) + columnIndex, true
}

func (t *ArrayTable[R, C, V]) indexes(row R, column C) (int, int, bool) {
	if t == nil || t.rows == nil {
		return 0, 0, false
	}
	rowIndex, rowFound := t.rows.get(row)
	columnIndex, columnFound := t.columns.get(column)
	return rowIndex, columnIndex, rowFound && columnFound
}

func (t *ArrayTable[R, C, V]) Put(row R, column C, value V) (V, bool, error) {
	t.writeContract("Put")
	rowIndex, columnIndex, found := t.indexes(row, column)
	if !found {
		var zero V
		return zero, false, ErrOutsideUniverse
	}
	old, replaced := t.PutAt(rowIndex, columnIndex, value)
	return old, replaced, nil
}

func (t *ArrayTable[R, C, V]) Get(row R, column C) (V, bool) {
	rowIndex, columnIndex, found := t.indexes(row, column)
	if !found {
		var zero V
		return zero, false
	}
	return t.GetAt(rowIndex, columnIndex)
}

// GetAt returns the value at zero-based universe indexes.
func (t *ArrayTable[R, C, V]) GetAt(rowIndex, columnIndex int) (V, bool) {
	position, valid := t.position(rowIndex, columnIndex)
	if !valid || !t.present[position] {
		var zero V
		return zero, false
	}
	return t.values[position], true
}

func (t *ArrayTable[R, C, V]) Delete(row R, column C) (V, bool, error) {
	if t == nil || t.rows == nil {
		var zero V
		return zero, false, nil
	}
	rowIndex, columnIndex, found := t.indexes(row, column)
	if !found {
		var zero V
		return zero, false, ErrOutsideUniverse
	}
	old, deleted := t.DeleteAt(rowIndex, columnIndex)
	return old, deleted, nil
}

// PutAt stores at zero-based universe indexes and panics for invalid indexes.
func (t *ArrayTable[R, C, V]) PutAt(rowIndex, columnIndex int, value V) (V, bool) {
	position, valid := t.position(rowIndex, columnIndex)
	if !valid {
		panic("table: PutAt index outside ArrayTable universe")
	}
	old, replaced := t.values[position], t.present[position]
	t.values[position] = value
	t.present[position] = true
	if !replaced {
		t.cellCount++
	}
	return old, replaced
}

// DeleteAt removes at zero-based universe indexes and clears the value slot.
func (t *ArrayTable[R, C, V]) DeleteAt(rowIndex, columnIndex int) (V, bool) {
	position, valid := t.position(rowIndex, columnIndex)
	if !valid {
		panic("table: DeleteAt index outside ArrayTable universe")
	}
	old, deleted := t.values[position], t.present[position]
	if deleted {
		var zero V
		t.values[position] = zero
		t.present[position] = false
		t.cellCount--
	}
	return old, deleted
}

func (t *ArrayTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

// ContainsRow reports fixed-universe membership, even for an empty row.
func (t *ArrayTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil || t.rows == nil {
		return false
	}
	_, found := t.rows.get(row)
	return found
}

// ContainsColumn reports fixed-universe membership, even for an empty column.
func (t *ArrayTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.columns == nil {
		return false
	}
	_, found := t.columns.get(column)
	return found
}

func (t *ArrayTable[R, C, V]) Len() int {
	if t == nil {
		return 0
	}
	return t.cellCount
}

func (t *ArrayTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

func (t *ArrayTable[R, C, V]) Clear() {
	if t == nil || t.rows == nil {
		return
	}
	var zero V
	for index := range t.values {
		t.values[index] = zero
		t.present[index] = false
	}
	t.cellCount = 0
}

func (t *ArrayTable[R, C, V]) Cells() iter.Seq[Cell[R, C, V]] {
	return func(yield func(Cell[R, C, V]) bool) {
		if t == nil || t.rows == nil {
			return
		}
		for rowIndex, row := range t.rowKeys {
			for columnIndex, column := range t.colKeys {
				position := rowIndex*len(t.colKeys) + columnIndex
				if t.present[position] && !yield(Cell[R, C, V]{Row: row, Column: column, Value: t.values[position]}) {
					return
				}
			}
		}
	}
}

func (t *ArrayTable[R, C, V]) Row(row R) []Cell[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	rowIndex, found := t.rows.get(row)
	if !found {
		return []Cell[R, C, V]{}
	}
	return t.rowAt(rowIndex)
}

func (t *ArrayTable[R, C, V]) rowAt(rowIndex int) []Cell[R, C, V] {
	cells := []Cell[R, C, V]{}
	for columnIndex, column := range t.colKeys {
		if value, found := t.GetAt(rowIndex, columnIndex); found {
			cells = append(cells, Cell[R, C, V]{Row: t.rowKeys[rowIndex], Column: column, Value: value})
		}
	}
	return cells
}

func (t *ArrayTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	if t == nil || t.columns == nil {
		return nil
	}
	columnIndex, found := t.columns.get(column)
	if !found {
		return []Cell[R, C, V]{}
	}
	return t.columnAt(columnIndex)
}

func (t *ArrayTable[R, C, V]) columnAt(columnIndex int) []Cell[R, C, V] {
	cells := []Cell[R, C, V]{}
	for rowIndex, row := range t.rowKeys {
		if value, found := t.GetAt(rowIndex, columnIndex); found {
			cells = append(cells, Cell[R, C, V]{Row: row, Column: t.colKeys[columnIndex], Value: value})
		}
	}
	return cells
}

// Rows includes every universe row, including rows with no present cells.
func (t *ArrayTable[R, C, V]) Rows() iter.Seq2[R, []Cell[R, C, V]] {
	type group struct {
		row   R
		cells []Cell[R, C, V]
	}
	groups := []group{}
	if t != nil && t.rows != nil {
		for rowIndex, row := range t.rowKeys {
			groups = append(groups, group{row: row, cells: t.rowAt(rowIndex)})
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

// Columns includes every universe column, including columns with no cells.
func (t *ArrayTable[R, C, V]) Columns() iter.Seq2[C, []Cell[R, C, V]] {
	type group struct {
		column C
		cells  []Cell[R, C, V]
	}
	groups := []group{}
	if t != nil && t.columns != nil {
		for columnIndex, column := range t.colKeys {
			groups = append(groups, group{column: column, cells: t.columnAt(columnIndex)})
		}
	}
	return func(yield func(C, []Cell[R, C, V]) bool) {
		for _, group := range groups {
			if !yield(group.column, copyCells(group.cells)) {
				return
			}
		}
	}
}

func (t *ArrayTable[R, C, V]) RowKeys() iter.Seq[R] {
	keys := append([]R(nil), t.rowKeys...)
	return func(yield func(R) bool) {
		for _, key := range keys {
			if !yield(key) {
				return
			}
		}
	}
}

func (t *ArrayTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	keys := append([]C(nil), t.colKeys...)
	return func(yield func(C) bool) {
		for _, key := range keys {
			if !yield(key) {
				return
			}
		}
	}
}

func (t *ArrayTable[R, C, V]) Transpose() *ArrayTable[C, R, V] {
	if t == nil || t.rows == nil {
		return &ArrayTable[C, R, V]{}
	}
	result := NewArrayTableFunc[C, R, V](t.colKeys, t.rowKeys, t.colEqual, t.colHash, t.rowEqual, t.rowHash)
	for cell := range t.Cells() {
		_, _, _ = result.Put(cell.Column, cell.Row, cell.Value)
	}
	return result
}

func (t *ArrayTable[R, C, V]) Clone() *ArrayTable[R, C, V] {
	if t == nil {
		return nil
	}
	if t.rows == nil {
		return &ArrayTable[R, C, V]{}
	}
	result := NewArrayTableFunc[R, C, V](t.rowKeys, t.colKeys, t.rowEqual, t.rowHash, t.colEqual, t.colHash)
	copy(result.values, t.values)
	copy(result.present, t.present)
	result.cellCount = t.cellCount
	return result
}
