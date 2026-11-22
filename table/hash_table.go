/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/hash_grow"
)

type hashEntry[K any, V any] struct {
	key   K
	value V
}

type hashIndex[K any, V any] struct {
	entries *hash_grow.HashTab[hashEntry[K, V]]
}

func newHashIndex[K any, V any](equal func(K, K) bool, hash func(K) uint64) *hashIndex[K, V] {
	return &hashIndex[K, V]{
		entries: hash_grow.NewHashTabFunc(
			func(a, b hashEntry[K, V]) bool { return equal(a.key, b.key) },
			func(entry hashEntry[K, V]) uint64 { return hash(entry.key) },
			16, 0,
		),
	}
}

func (index *hashIndex[K, V]) get(key K) (V, bool) {
	entry, found := index.entries.Search(hashEntry[K, V]{key: key})
	return entry.value, found
}

func (index *hashIndex[K, V]) put(key K, value V) (V, bool) {
	old, found := index.get(key)
	index.entries.Insert(hashEntry[K, V]{key: key, value: value})
	return old, found
}

func (index *hashIndex[K, V]) delete(key K) (V, bool) {
	old, found := index.get(key)
	if found {
		index.entries.Delete(hashEntry[K, V]{key: key})
	}
	return old, found
}

func (index *hashIndex[K, V]) len() int { return index.entries.Len() }

func (index *hashIndex[K, V]) clear() { index.entries.Truncate() }

func (index *hashIndex[K, V]) all() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for entry := range index.entries.Values() {
			if !yield(entry.key, entry.value) {
				return
			}
		}
	}
}

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

// Put inserts or replaces a cell and returns its previous value.
func (t *HashTable[R, C, V]) Put(row R, column C, value V) (V, bool) {
	t.writeContract("Put")
	columns, found := t.rows.get(row)
	if !found {
		columns = newHashIndex[C, V](t.colEqual, t.colHash)
		t.rows.put(row, columns)
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
	for _, columns := range t.rows.all() {
		if _, found := columns.get(column); found {
			return true
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
		for row, columns := range t.rows.all() {
			for column, value := range columns.all() {
				if !yield(Cell[R, C, V]{Row: row, Column: column, Value: value}) {
					return
				}
			}
		}
	}
}

func hashRowCells[R any, C any, V any](row R, columns *hashIndex[C, V]) []Cell[R, C, V] {
	cells := make([]Cell[R, C, V], 0, columns.len())
	for column, value := range columns.all() {
		cells = append(cells, Cell[R, C, V]{Row: row, Column: column, Value: value})
	}
	return cells
}

// Row returns a fresh snapshot in unspecified column order.
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

// Column returns a fresh snapshot in unspecified row order.
func (t *HashTable[R, C, V]) Column(column C) []Cell[R, C, V] {
	if t == nil || t.rows == nil {
		return nil
	}
	cells := []Cell[R, C, V]{}
	for row, columns := range t.rows.all() {
		if value, found := columns.get(column); found {
			cells = append(cells, Cell[R, C, V]{Row: row, Column: column, Value: value})
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
	for cell := range t.Cells() {
		result.Put(cell.Column, cell.Row, cell.Value)
	}
	return result
}

// Clone returns an independent sparse table with the same key functions.
func (t *HashTable[R, C, V]) Clone() *HashTable[R, C, V] {
	if t == nil {
		return nil
	}
	if t.rows == nil {
		return &HashTable[R, C, V]{}
	}
	result := NewHashTableFunc[R, C, V](t.rowEqual, t.rowHash, t.colEqual, t.colHash)
	for cell := range t.Cells() {
		result.Put(cell.Row, cell.Column, cell.Value)
	}
	return result
}
