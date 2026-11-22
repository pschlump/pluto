/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"encoding/json"
	"iter"
	"sort"
)

type immutableRow[R any, C any, V any] struct {
	key   R
	cells []Cell[R, C, V]
}

type immutableColumn[R any, C any, V any] struct {
	key   C
	cells []Cell[R, C, V]
}

type snapshotSource[R any, C any, V any] interface {
	Cells() iter.Seq[Cell[R, C, V]]
	Rows() iter.Seq2[R, []Cell[R, C, V]]
	Columns() iter.Seq2[C, []Cell[R, C, V]]
}

// ImmutableTable is an independent read-only table snapshot. It is safe for
// concurrent reads; payload values themselves are not deeply immutable.
type ImmutableTable[R any, C any, V any] struct {
	index      *HashTable[R, C, V]
	ordered    *TreeTable[R, C, V]
	cells      []Cell[R, C, V]
	rows       []immutableRow[R, C, V]
	columns    []immutableColumn[R, C, V]
	rowPos     *hashIndex[R, int]
	colPos     *hashIndex[C, int]
	rowCompare func(R, R) int
	colCompare func(C, C) int
}

func freeze[R any, C any, V any](source snapshotSource[R, C, V], rowEqual func(R, R) bool, rowHash func(R) uint64, colEqual func(C, C) bool, colHash func(C) uint64) *ImmutableTable[R, C, V] {
	result := &ImmutableTable[R, C, V]{}
	ordered, isOrdered := source.(*TreeTable[R, C, V])
	if isOrdered {
		result.ordered = ordered.Clone()
		result.rowCompare = ordered.rowCompare
		result.colCompare = ordered.colCompare
	} else {
		result.index = NewHashTableFunc[R, C, V](rowEqual, rowHash, colEqual, colHash)
		result.rowPos = newHashIndex[R, int](rowEqual, rowHash)
		result.colPos = newHashIndex[C, int](colEqual, colHash)
	}
	result.cells = collectCells(source.Cells())
	for _, cell := range result.cells {
		if result.index != nil {
			result.index.Put(cell.Row, cell.Column, cell.Value)
		}
	}
	for row, cells := range source.Rows() {
		if result.rowPos != nil {
			result.rowPos.put(row, len(result.rows))
		}
		result.rows = append(result.rows, immutableRow[R, C, V]{key: row, cells: copyCells(cells)})
	}
	for column, cells := range source.Columns() {
		if result.colPos != nil {
			result.colPos.put(column, len(result.columns))
		}
		result.columns = append(result.columns, immutableColumn[R, C, V]{key: column, cells: copyCells(cells)})
	}
	return result
}

// Freeze returns an immutable, independent copy of a hash table.
func (t *HashTable[R, C, V]) Freeze() *ImmutableTable[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	return freeze[R, C, V](t, t.rowEqual, t.rowHash, t.colEqual, t.colHash)
}

// Freeze returns an immutable copy retaining ordered lookup and iteration.
func (t *TreeTable[R, C, V]) Freeze() *ImmutableTable[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	return freeze[R, C, V](t, nil, nil, nil, nil)
}

// Freeze returns an immutable copy retaining complete dense universes.
func (t *ArrayTable[R, C, V]) Freeze() *ImmutableTable[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	return freeze[R, C, V](t, t.rowEqual, t.rowHash, t.colEqual, t.colHash)
}

// Freeze returns the receiver because immutable snapshots can be shared safely.
func (t *ImmutableTable[R, C, V]) Freeze() *ImmutableTable[R, C, V] { return t }

func (t *ImmutableTable[R, C, V]) Get(row R, column C) (V, bool) {
	if t == nil {
		var zero V
		return zero, false
	}
	if t.ordered != nil {
		return t.ordered.Get(row, column)
	}
	return t.index.Get(row, column)
}

func (t *ImmutableTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

func (t *ImmutableTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil {
		return false
	}
	if t.ordered != nil {
		return t.ordered.ContainsRow(row)
	}
	_, found := t.rowPos.get(row)
	return found
}

func (t *ImmutableTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil {
		return false
	}
	if t.ordered != nil {
		return t.ordered.ContainsColumn(column)
	}
	_, found := t.colPos.get(column)
	return found
}

func (t *ImmutableTable[R, C, V]) Len() int {
	if t == nil {
		return 0
	}
	return len(t.cells)
}

func (t *ImmutableTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

func (t *ImmutableTable[R, C, V]) Cells() iter.Seq[Cell[R, C, V]] {
	if t == nil {
		return cellSequence([]Cell[R, C, V](nil))
	}
	return cellSequence(t.cells)
}

func (t *ImmutableTable[R, C, V]) Row(row R) []Cell[R, C, V] {
	if t == nil {
		return nil
	}
	position, found := t.rowPosition(row)
	if !found {
		return []Cell[R, C, V]{}
	}
	return copyCells(t.rows[position].cells)
}

func (t *ImmutableTable[R, C, V]) rowPosition(row R) (int, bool) {
	if t.ordered != nil {
		position := sort.Search(len(t.rows), func(index int) bool {
			return t.rowCompare(t.rows[index].key, row) >= 0
		})
		return position, position < len(t.rows) && t.rowCompare(t.rows[position].key, row) == 0
	}
	return t.rowPos.get(row)
}

func (t *ImmutableTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	if t == nil {
		return nil
	}
	position, found := t.columnPosition(column)
	if !found {
		return []Cell[R, C, V]{}
	}
	return copyCells(t.columns[position].cells)
}

func (t *ImmutableTable[R, C, V]) columnPosition(column C) (int, bool) {
	if t.ordered != nil {
		position := sort.Search(len(t.columns), func(index int) bool {
			return t.colCompare(t.columns[index].key, column) >= 0
		})
		return position, position < len(t.columns) && t.colCompare(t.columns[position].key, column) == 0
	}
	return t.colPos.get(column)
}

func (t *ImmutableTable[R, C, V]) Rows() iter.Seq2[R, []Cell[R, C, V]] {
	rows := []immutableRow[R, C, V]{}
	if t != nil {
		rows = t.rows
	}
	return func(yield func(R, []Cell[R, C, V]) bool) {
		for _, row := range rows {
			if !yield(row.key, copyCells(row.cells)) {
				return
			}
		}
	}
}

func (t *ImmutableTable[R, C, V]) Columns() iter.Seq2[C, []Cell[R, C, V]] {
	columns := []immutableColumn[R, C, V]{}
	if t != nil {
		columns = t.columns
	}
	return func(yield func(C, []Cell[R, C, V]) bool) {
		for _, column := range columns {
			if !yield(column.key, copyCells(column.cells)) {
				return
			}
		}
	}
}

func (t *ImmutableTable[R, C, V]) RowKeys() iter.Seq[R] {
	return func(yield func(R) bool) {
		for row := range t.Rows() {
			if !yield(row) {
				return
			}
		}
	}
}

func (t *ImmutableTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	return func(yield func(C) bool) {
		for column := range t.Columns() {
			if !yield(column) {
				return
			}
		}
	}
}

// Transpose returns an independent immutable transposed snapshot.
func (t *ImmutableTable[R, C, V]) Transpose() *ImmutableTable[C, R, V] {
	if t == nil {
		return nil
	}
	if t.ordered != nil {
		return t.ordered.Transpose().Freeze()
	}
	return t.index.Transpose().Freeze()
}

func (t *ImmutableTable[R, C, V]) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(t.cells)
}
