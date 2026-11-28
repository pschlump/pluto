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
//
// The universe (rowKeys, colKeys, rowIndex, colIndex and the key functions) is
// immutable after construction. Clone, Transpose, Replace and UnmarshalJSON
// share it with the new table instead of rebuilding it; nothing may write to
// rowKeys or colKeys, and RowKeys/ColumnKeys hand out copies.
type ArrayTable[R any, C any, V any] struct {
	rowKeys   []R
	colKeys   []C
	rowIndex  func(R) (int, bool) // universe position of a row key; nil only in the zero value
	colIndex  func(C) (int, bool)
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
//
// Universe lookups use Go's built-in map, which hashes comparable keys faster
// than the maphash-based functions NewArrayTableFunc would be given.
// Complexity is O(rows + columns + rows x columns).
func NewArrayTable[R comparable, C comparable, V any](rows []R, columns []C) *ArrayTable[R, C, V] {
	rowEqual, rowHash := naturalFunctions[R]()
	colEqual, colHash := naturalFunctions[C]()
	t := &ArrayTable[R, C, V]{rowEqual: rowEqual, rowHash: rowHash, colEqual: colEqual, colHash: colHash}
	t.rowKeys, t.rowIndex = mapUniverse(rows)
	t.colKeys, t.colIndex = mapUniverse(columns)
	t.allocateMatrix()
	return t
}

// NewArrayTableFunc creates a dense table with custom universe key functions.
// Complexity is O(rows + columns + rows x columns) expected.
func NewArrayTableFunc[R any, C any, V any](rows []R, columns []C, rowEqual func(R, R) bool, rowHash func(R) uint64, colEqual func(C, C) bool, colHash func(C) uint64) *ArrayTable[R, C, V] {
	if rowEqual == nil || rowHash == nil || colEqual == nil || colHash == nil {
		panic("table: NewArrayTableFunc requires non-nil equality and hash functions")
	}
	t := &ArrayTable[R, C, V]{rowEqual: rowEqual, rowHash: rowHash, colEqual: colEqual, colHash: colHash}
	t.rowKeys, t.rowIndex = funcUniverse(rows, rowEqual, rowHash)
	t.colKeys, t.colIndex = funcUniverse(columns, colEqual, colHash)
	t.allocateMatrix()
	return t
}

// mapUniverse deduplicates keys (first occurrence wins) and returns the
// distinct keys with a position lookup over a built-in map.
func mapUniverse[K comparable](keys []K) ([]K, func(K) (int, bool)) {
	positions := make(map[K]int, len(keys))
	distinct := make([]K, 0, len(keys))
	for _, key := range keys {
		if _, found := positions[key]; found {
			continue
		}
		positions[key] = len(distinct)
		distinct = append(distinct, key)
	}
	return distinct, func(key K) (int, bool) {
		position, found := positions[key]
		return position, found
	}
}

// funcUniverse is mapUniverse for caller-supplied equality and hash functions.
func funcUniverse[K any](keys []K, equal func(K, K) bool, hash func(K) uint64) ([]K, func(K) (int, bool)) {
	positions := newHashIndex[K, int](equal, hash)
	distinct := make([]K, 0, len(keys))
	for _, key := range keys {
		keyHash := fixHash(hash(key))
		slot, found := positions.findOrAllocate(keyHash, key)
		if found {
			continue
		}
		positions.insertAt(slot, keyHash, key, len(distinct))
		distinct = append(distinct, key)
	}
	return distinct, positions.get
}

// allocateMatrix sizes values and present for the universe.
func (t *ArrayTable[R, C, V]) allocateMatrix() {
	if len(t.rowKeys) != 0 && len(t.colKeys) > int(^uint(0)>>1)/len(t.rowKeys) {
		panic("table: ArrayTable universe size overflow")
	}
	t.values = make([]V, len(t.rowKeys)*len(t.colKeys))
	t.present = make([]bool, len(t.values))
}

// sameUniverse returns an empty table sharing t's immutable universe.
func (t *ArrayTable[R, C, V]) sameUniverse() *ArrayTable[R, C, V] {
	result := &ArrayTable[R, C, V]{
		rowKeys: t.rowKeys, colKeys: t.colKeys, rowIndex: t.rowIndex, colIndex: t.colIndex,
		rowEqual: t.rowEqual, rowHash: t.rowHash, colEqual: t.colEqual, colHash: t.colHash,
	}
	result.allocateMatrix()
	return result
}

func (t *ArrayTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.rowIndex == nil {
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
	if t == nil || t.rowIndex == nil {
		return 0, 0, false
	}
	rowIndex, rowFound := t.rowIndex(row)
	if !rowFound {
		return 0, 0, false
	}
	columnIndex, columnFound := t.colIndex(column)
	return rowIndex, columnIndex, columnFound
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
	if t == nil || t.rowIndex == nil {
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
	if t == nil || t.rowIndex == nil {
		return false
	}
	_, found := t.rowIndex(row)
	return found
}

// ContainsColumn reports fixed-universe membership, even for an empty column.
func (t *ArrayTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.colIndex == nil {
		return false
	}
	_, found := t.colIndex(column)
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
	if t == nil || t.rowIndex == nil {
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
		if t == nil || t.rowIndex == nil {
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
	if t == nil || t.rowIndex == nil {
		return nil
	}
	rowIndex, found := t.rowIndex(row)
	if !found {
		return []Cell[R, C, V]{}
	}
	return t.rowAt(rowIndex)
}

// rowAt copies one row: a count over the row's contiguous present segment
// sizes the result exactly, so the copy is one allocation.
func (t *ArrayTable[R, C, V]) rowAt(rowIndex int) []Cell[R, C, V] {
	width := len(t.colKeys)
	start := rowIndex * width
	present := t.present[start : start+width]
	values := t.values[start : start+width]
	count := 0
	for _, isPresent := range present {
		if isPresent {
			count++
		}
	}
	cells := make([]Cell[R, C, V], count)
	row := t.rowKeys[rowIndex]
	next := 0
	for columnIndex, isPresent := range present {
		if isPresent {
			cells[next] = Cell[R, C, V]{Row: row, Column: t.colKeys[columnIndex], Value: values[columnIndex]}
			next++
		}
	}
	return cells
}

func (t *ArrayTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	if t == nil || t.colIndex == nil {
		return nil
	}
	columnIndex, found := t.colIndex(column)
	if !found {
		return []Cell[R, C, V]{}
	}
	return t.columnAt(columnIndex)
}

// columnAt copies one column: a strided count of present cells sizes the
// result exactly, so the copy is one allocation.
func (t *ArrayTable[R, C, V]) columnAt(columnIndex int) []Cell[R, C, V] {
	width := len(t.colKeys)
	count := 0
	for position := columnIndex; position < len(t.present); position += width {
		if t.present[position] {
			count++
		}
	}
	cells := make([]Cell[R, C, V], count)
	column := t.colKeys[columnIndex]
	next := 0
	for rowIndex, position := 0, columnIndex; position < len(t.present); rowIndex, position = rowIndex+1, position+width {
		if t.present[position] {
			cells[next] = Cell[R, C, V]{Row: t.rowKeys[rowIndex], Column: column, Value: t.values[position]}
			next++
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
	if t != nil && t.rowIndex != nil {
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
	if t != nil && t.colIndex != nil {
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
	if t == nil || t.rowIndex == nil {
		return &ArrayTable[C, R, V]{}
	}
	// The transposed universe is the same immutable universe with its axes
	// swapped, so it is shared; only the matrix is rebuilt, column-major.
	result := &ArrayTable[C, R, V]{
		rowKeys: t.colKeys, colKeys: t.rowKeys, rowIndex: t.colIndex, colIndex: t.rowIndex,
		rowEqual: t.colEqual, rowHash: t.colHash, colEqual: t.rowEqual, colHash: t.rowHash,
	}
	result.allocateMatrix()
	rows, columns := len(t.rowKeys), len(t.colKeys)
	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		source := rowIndex * columns
		for columnIndex := 0; columnIndex < columns; columnIndex++ {
			if t.present[source+columnIndex] {
				target := columnIndex*rows + rowIndex
				result.values[target] = t.values[source+columnIndex]
				result.present[target] = true
			}
		}
	}
	result.cellCount = t.cellCount
	return result
}

// Clone returns an independent dense table sharing the immutable universe.
// Complexity is O(rows x columns).
func (t *ArrayTable[R, C, V]) Clone() *ArrayTable[R, C, V] {
	if t == nil {
		return nil
	}
	if t.rowIndex == nil {
		return &ArrayTable[R, C, V]{}
	}
	result := t.sameUniverse()
	copy(result.values, t.values)
	copy(result.present, t.present)
	result.cellCount = t.cellCount
	return result
}
