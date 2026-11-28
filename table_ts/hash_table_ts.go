/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package table_ts provides mutex-guarded twins of the table types. All
// iterators and row/column results are snapshots. Iterator callbacks and JSON
// encoding/decoding do not run while an internal lock is held.
package table_ts

import (
	"iter"
	"sync"

	"github.com/pschlump/pluto/table"
)

type HashTable[R any, C any, V any] struct {
	lock  sync.RWMutex
	plain *table.HashTable[R, C, V]
}

func NewHashTable[R comparable, C comparable, V any]() *HashTable[R, C, V] {
	return &HashTable[R, C, V]{plain: table.NewHashTable[R, C, V]()}
}

func NewHashTableFunc[R any, C any, V any](rowEqual func(R, R) bool, rowHash func(R) uint64, colEqual func(C, C) bool, colHash func(C) uint64) *HashTable[R, C, V] {
	return &HashTable[R, C, V]{plain: table.NewHashTableFunc[R, C, V](rowEqual, rowHash, colEqual, colHash)}
}

func wrapHash[R any, C any, V any](plain *table.HashTable[R, C, V]) *HashTable[R, C, V] {
	if plain == nil {
		return nil
	}
	return &HashTable[R, C, V]{plain: plain}
}

func (t *HashTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.plain == nil {
		panic("table_ts: " + caller + " on a nil or zero-value HashTable")
	}
}

func (t *HashTable[R, C, V]) Put(row R, column C, value V) (V, bool) {
	t.writeContract("Put")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Put(row, column, value)
}

func (t *HashTable[R, C, V]) Get(row R, column C) (V, bool) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Get(row, column)
}

func (t *HashTable[R, C, V]) Delete(row R, column C) (V, bool) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Delete(row, column)
}

func (t *HashTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

func (t *HashTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil || t.plain == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.ContainsRow(row)
}

func (t *HashTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.plain == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.ContainsColumn(column)
}

func (t *HashTable[R, C, V]) Len() int {
	if t == nil || t.plain == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Len()
}

func (t *HashTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

func (t *HashTable[R, C, V]) Clear() {
	if t == nil || t.plain == nil {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Clear()
}

func (t *HashTable[R, C, V]) Replace(cells []table.Cell[R, C, V]) {
	t.writeContract("Replace")
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Replace(cells)
}

func snapshotCells[R any, C any, V any](sequence iter.Seq[table.Cell[R, C, V]]) []table.Cell[R, C, V] {
	return snapshotCellsSized(sequence, 0)
}

// snapshotCellsSized collects sequence into a slice presized for count cells
// (the table's Len under the held lock), so a Cells snapshot is one
// allocation instead of an append-growth chain.
func snapshotCellsSized[R any, C any, V any](sequence iter.Seq[table.Cell[R, C, V]], count int) []table.Cell[R, C, V] {
	cells := make([]table.Cell[R, C, V], 0, count)
	for cell := range sequence {
		cells = append(cells, cell)
	}
	return cells
}

func cellsIterator[R any, C any, V any](cells []table.Cell[R, C, V]) iter.Seq[table.Cell[R, C, V]] {
	return func(yield func(table.Cell[R, C, V]) bool) {
		for _, cell := range cells {
			if !yield(cell) {
				return
			}
		}
	}
}

func (t *HashTable[R, C, V]) Cells() iter.Seq[table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return cellsIterator([]table.Cell[R, C, V](nil))
	}
	t.lock.RLock()
	cells := snapshotCellsSized(t.plain.Cells(), t.plain.Len())
	t.lock.RUnlock()
	return cellsIterator(cells)
}

func (t *HashTable[R, C, V]) Row(row R) []table.Cell[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Row(row)
}

func (t *HashTable[R, C, V]) Column(column C) []table.Cell[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Column(column)
}

func (t *HashTable[R, C, V]) Rows() iter.Seq2[R, []table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return func(func(R, []table.Cell[R, C, V]) bool) {}
	}
	t.lock.RLock()
	rows := t.plain.Rows()
	t.lock.RUnlock()
	return rows
}

func (t *HashTable[R, C, V]) Columns() iter.Seq2[C, []table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return func(func(C, []table.Cell[R, C, V]) bool) {}
	}
	t.lock.RLock()
	columns := t.plain.Columns()
	t.lock.RUnlock()
	return columns
}

func (t *HashTable[R, C, V]) RowKeys() iter.Seq[R] {
	rows := t.Rows()
	return func(yield func(R) bool) {
		for row := range rows {
			if !yield(row) {
				return
			}
		}
	}
}

func (t *HashTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	columns := t.Columns()
	return func(yield func(C) bool) {
		for column := range columns {
			if !yield(column) {
				return
			}
		}
	}
}

func (t *HashTable[R, C, V]) Transpose() *HashTable[C, R, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return wrapHash(t.plain.Transpose())
}

func (t *HashTable[R, C, V]) Clone() *HashTable[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return wrapHash(t.plain.Clone())
}

func (t *HashTable[R, C, V]) Freeze() *table.ImmutableTable[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Freeze()
}
