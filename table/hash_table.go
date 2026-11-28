/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"fmt"
	"iter"
)

// HashTable is a sparse row-major hash table. Its zero value is empty for reads;
// use a constructor before mutation.
type HashTable[R any, C any, V any] struct {
	rows      *hashIndex[R, *hashIndex[C, V]]
	rowEqual  func(R, R) bool
	rowHash   func(R) uint64
	colEqual  func(C, C) bool
	colHash   func(C) uint64
	cellCount int
}

// NewHashTable creates an empty sparse table using == and maphash for keys.
func NewHashTable[R comparable, C comparable, V any]() *HashTable[R, C, V] {
	rowEqual, rowHash := naturalFunctions[R]()
	colEqual, colHash := naturalFunctions[C]()
	return NewHashTableFunc[R, C, V](rowEqual, rowHash, colEqual, colHash)
}

// NewHashTableFunc creates an empty sparse table with custom key functions.
// Equal keys must have equal hashes.
func NewHashTableFunc[R any, C any, V any](rowEqual func(R, R) bool, rowHash func(R) uint64, colEqual func(C, C) bool, colHash func(C) uint64) *HashTable[R, C, V] {
	if rowEqual == nil || rowHash == nil || colEqual == nil || colHash == nil {
		panic("table: NewHashTableFunc requires non-nil equality and hash functions")
	}
	return &HashTable[R, C, V]{
		rows:     newHashIndex[R, *hashIndex[C, V]](rowEqual, rowHash),
		rowEqual: rowEqual, rowHash: rowHash, colEqual: colEqual, colHash: colHash,
	}
}

func (t *HashTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.rows == nil {
		panic(fmt.Sprintf("table: %s on a nil or zero-value HashTable; use a constructor", caller))
	}
}

// Put inserts or replaces a cell and returns its previous value. The row key
// is hashed once whether or not its row already exists.
// Complexity is expected amortized O(1).
func (t *HashTable[R, C, V]) Put(row R, column C, value V) (V, bool) {
	t.writeContract("Put")
	rows := t.rows
	rowHash := fixHash(rows.hash(row))
	position, found := rows.findOrAllocate(rowHash, row)
	var columns *hashIndex[C, V]
	if found {
		columns = rows.slots[position].value
	} else {
		columns = newHashIndex[C, V](t.colEqual, t.colHash)
		rows.insertAt(position, rowHash, row, columns)
	}
	old, replaced := columns.put(column, value)
	if !replaced {
		t.cellCount++
	}
	return old, replaced
}

// Get returns a cell value and whether the cell is present.
func (t *HashTable[R, C, V]) Get(row R, column C) (V, bool) {
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

// Delete removes a cell and its empty sparse row, returning the former value.
func (t *HashTable[R, C, V]) Delete(row R, column C) (V, bool) {
	if t == nil || t.rows == nil {
		var zero V
		return zero, false
	}
	position, found := t.rows.lookup(row)
	if !found {
		var zero V
		return zero, false
	}
	columns := t.rows.slots[position].value
	old, deleted := columns.delete(column)
	if deleted {
		t.cellCount--
		if columns.count == 0 {
			t.rows.removeAt(position)
		}
	}
	return old, deleted
}

func (t *HashTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

// ContainsRow reports whether a sparse row has at least one cell.
func (t *HashTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil || t.rows == nil {
		return false
	}
	_, found := t.rows.get(row)
	return found
}

// ContainsColumn reports whether any sparse row contains column.
func (t *HashTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.rows == nil {
		return false
	}
	columnHash := fixHash(t.colHash(column))
	for position := range t.rows.slots {
		slot := &t.rows.slots[position]
		if slot.hash != 0 {
			if _, found := slot.value.find(columnHash, column); found {
				return true
			}
		}
	}
	return false
}

func (t *HashTable[R, C, V]) Len() int {
	if t == nil {
		return 0
	}
	return t.cellCount
}

func (t *HashTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

// Clear removes all cells and retains the key functions.
func (t *HashTable[R, C, V]) Clear() {
	if t == nil || t.rows == nil {
		return
	}
	t.rows.clear()
	t.cellCount = 0
}

// Cells iterates in unspecified hash order. Do not mutate while iterating.
func (t *HashTable[R, C, V]) Cells() iter.Seq[Cell[R, C, V]] {
	return func(yield func(Cell[R, C, V]) bool) {
		if t == nil || t.rows == nil {
			return
		}
		for rowPosition := range t.rows.slots {
			rowSlot := &t.rows.slots[rowPosition]
			if rowSlot.hash == 0 {
				continue
			}
			for position := range rowSlot.value.slots {
				slot := &rowSlot.value.slots[position]
				if slot.hash != 0 && !yield(Cell[R, C, V]{Row: rowSlot.key, Column: slot.key, Value: slot.value}) {
					return
				}
			}
		}
	}
}

// hashRowCells copies one row into an exactly sized slice (one allocation).
func hashRowCells[R any, C any, V any](row R, columns *hashIndex[C, V]) []Cell[R, C, V] {
	cells := make([]Cell[R, C, V], columns.count)
	next := 0
	for position := range columns.slots {
		slot := &columns.slots[position]
		if slot.hash != 0 {
			cells[next] = Cell[R, C, V]{Row: row, Column: slot.key, Value: slot.value}
			next++
		}
	}
	return cells
}

// Row returns a fresh snapshot in unspecified column order, allocated once at
// its exact length. Complexity is expected O(width).
func (t *HashTable[R, C, V]) Row(row R) []Cell[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	columns, found := t.rows.get(row)
	if !found {
		return []Cell[R, C, V]{}
	}
	return hashRowCells(row, columns)
}

// Column returns a fresh snapshot in unspecified row order. The column key is
// hashed once and probed in every row. Complexity is expected O(rows).
func (t *HashTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	cells := make([]Cell[R, C, V], 0, columnCapacityHint(t.rows.count, t.cellCount))
	if t.rows.count == 0 {
		return cells
	}
	columnHash := fixHash(t.colHash(column))
	for rowPosition := range t.rows.slots {
		rowSlot := &t.rows.slots[rowPosition]
		if rowSlot.hash == 0 {
			continue
		}
		if position, found := rowSlot.value.find(columnHash, column); found {
			slot := &rowSlot.value.slots[position]
			cells = append(cells, Cell[R, C, V]{Row: rowSlot.key, Column: slot.key, Value: slot.value})
		}
	}
	return cells
}

// Rows returns a snapshot iterator over non-empty sparse rows.
func (t *HashTable[R, C, V]) Rows() iter.Seq2[R, []Cell[R, C, V]] {
	type group struct {
		row   R
		cells []Cell[R, C, V]
	}
	groups := []group{}
	if t != nil && t.rows != nil {
		for row, columns := range t.rows.all() {
			groups = append(groups, group{row: row, cells: hashRowCells(row, columns)})
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

// Columns returns a snapshot iterator over non-empty sparse columns.
func (t *HashTable[R, C, V]) Columns() iter.Seq2[C, []Cell[R, C, V]] {
	groups := newHashIndex[C, []Cell[R, C, V]](t.colEqualOrNil(), t.colHashOrNil())
	if t != nil && t.rows != nil {
		// Column hashes are cached in every row's slots, so grouping
		// re-hashes nothing.
		for rowPosition := range t.rows.slots {
			rowSlot := &t.rows.slots[rowPosition]
			if rowSlot.hash == 0 {
				continue
			}
			for position := range rowSlot.value.slots {
				slot := &rowSlot.value.slots[position]
				if slot.hash == 0 {
					continue
				}
				cell := Cell[R, C, V]{Row: rowSlot.key, Column: slot.key, Value: slot.value}
				group, found := groups.findOrAllocate(slot.hash, slot.key)
				if found {
					groups.slots[group].value = append(groups.slots[group].value, cell)
				} else {
					groups.insertAt(group, slot.hash, slot.key, []Cell[R, C, V]{cell})
				}
			}
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

func (t *HashTable[R, C, V]) colEqualOrNil() func(C, C) bool {
	if t == nil || t.colEqual == nil {
		return func(C, C) bool { return false }
	}
	return t.colEqual
}

func (t *HashTable[R, C, V]) colHashOrNil() func(C) uint64 {
	if t == nil || t.colHash == nil {
		return func(C) uint64 { return 0 }
	}
	return t.colHash
}

func (t *HashTable[R, C, V]) RowKeys() iter.Seq[R] {
	return func(yield func(R) bool) {
		for row := range t.Rows() {
			if !yield(row) {
				return
			}
		}
	}
}

func (t *HashTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	return func(yield func(C) bool) {
		for column := range t.Columns() {
			if !yield(column) {
				return
			}
		}
	}
}

// Transpose returns an independent table with rows and columns exchanged.
func (t *HashTable[R, C, V]) Transpose() *HashTable[C, R, V] {
	if t == nil || t.rows == nil {
		return &HashTable[C, R, V]{}
	}
	result := NewHashTableFunc[C, R, V](t.colEqual, t.colHash, t.rowEqual, t.rowHash)
	transposeHashRows(t.rows, result.rows)
	result.cellCount = t.cellCount
	return result
}

// transposeHashRows fills the empty destination index with the transpose of
// source. Both hash functions' results are cached in the source slots (a column
// hash in every row index, a row hash in the outer index), so no key is hashed:
// the destination row is located by its cached column hash, and the source row
// is appended to it with insertUnique because source rows are distinct.
//
// A first pass counts each destination row's width so every destination row
// index is allocated once at its final size.
func transposeHashRows[R any, C any, V any](source *hashIndex[R, *hashIndex[C, V]], destination *hashIndex[C, *hashIndex[R, V]]) {
	for rowPosition := range source.slots {
		rowSlot := &source.slots[rowPosition]
		if rowSlot.hash == 0 {
			continue
		}
		for position := range rowSlot.value.slots {
			slot := &rowSlot.value.slots[position]
			if slot.hash == 0 {
				continue
			}
			target, found := destination.findOrAllocate(slot.hash, slot.key)
			if !found {
				target = destination.insertAt(target, slot.hash, slot.key, newHashIndex[R, V](source.equal, source.hash))
			}
			destination.slots[target].value.count++ // width tally; reset below
		}
	}
	for position := range destination.slots {
		slot := &destination.slots[position]
		if slot.hash != 0 {
			width := slot.value.count
			slot.value.count = 0
			slot.value.reserve(width)
		}
	}
	for rowPosition := range source.slots {
		rowSlot := &source.slots[rowPosition]
		if rowSlot.hash == 0 {
			continue
		}
		for position := range rowSlot.value.slots {
			slot := &rowSlot.value.slots[position]
			if slot.hash == 0 {
				continue
			}
			target, _ := destination.find(slot.hash, slot.key)
			destination.slots[target].value.insertUnique(rowSlot.hash, rowSlot.key, slot.value)
		}
	}
}

// Clone returns an independent sparse table with the same key functions.
func (t *HashTable[R, C, V]) Clone() *HashTable[R, C, V] {
	if t == nil {
		return nil
	}
	if t.rows == nil {
		return &HashTable[R, C, V]{}
	}
	// Copy both slot levels verbatim: same layout, no re-hashing.
	result := &HashTable[R, C, V]{
		rows:     t.rows.clone(),
		rowEqual: t.rowEqual, rowHash: t.rowHash, colEqual: t.colEqual, colHash: t.colHash,
		cellCount: t.cellCount,
	}
	for position := range result.rows.slots {
		slot := &result.rows.slots[position]
		if slot.hash != 0 {
			slot.value = slot.value.clone()
		}
	}
	return result
}
