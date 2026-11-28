/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_ts

import (
	"iter"
	"sync"

	"github.com/pschlump/pluto/table"
)

type ArrayTable[R any, C any, V any] struct {
	lock  sync.RWMutex
	plain *table.ArrayTable[R, C, V]
}

func NewArrayTable[R comparable, C comparable, V any](rows []R, columns []C) *ArrayTable[R, C, V] {
	return &ArrayTable[R, C, V]{plain: table.NewArrayTable[R, C, V](rows, columns)}
}

func NewArrayTableFunc[R any, C any, V any](rows []R, columns []C, rowEqual func(R, R) bool, rowHash func(R) uint64, colEqual func(C, C) bool, colHash func(C) uint64) *ArrayTable[R, C, V] {
	return &ArrayTable[R, C, V]{plain: table.NewArrayTableFunc[R, C, V](rows, columns, rowEqual, rowHash, colEqual, colHash)}
}

func wrapArray[R any, C any, V any](plain *table.ArrayTable[R, C, V]) *ArrayTable[R, C, V] {
	if plain == nil {
		return nil
	}
	return &ArrayTable[R, C, V]{plain: plain}
}

func (t *ArrayTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.plain == nil {
		panic("table_ts: " + caller + " on a nil or zero-value ArrayTable")
	}
}

func (t *ArrayTable[R, C, V]) Put(row R, column C, value V) (V, bool, error) {
	t.writeContract("Put")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Put(row, column, value)
}

func (t *ArrayTable[R, C, V]) Get(row R, column C) (V, bool) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Get(row, column)
}

func (t *ArrayTable[R, C, V]) GetAt(rowIndex, columnIndex int) (V, bool) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.GetAt(rowIndex, columnIndex)
}

func (t *ArrayTable[R, C, V]) PutAt(rowIndex, columnIndex int, value V) (V, bool) {
	t.writeContract("PutAt")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.PutAt(rowIndex, columnIndex, value)
}

func (t *ArrayTable[R, C, V]) Delete(row R, column C) (V, bool, error) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false, nil
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Delete(row, column)
}

func (t *ArrayTable[R, C, V]) DeleteAt(rowIndex, columnIndex int) (V, bool) {
	t.writeContract("DeleteAt")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.DeleteAt(rowIndex, columnIndex)
}

func (t *ArrayTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

func (t *ArrayTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil || t.plain == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.ContainsRow(row)
}

func (t *ArrayTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.plain == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.ContainsColumn(column)
}

func (t *ArrayTable[R, C, V]) Len() int {
	if t == nil || t.plain == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Len()
}

func (t *ArrayTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

func (t *ArrayTable[R, C, V]) Clear() {
	if t == nil || t.plain == nil {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Clear()
}

func (t *ArrayTable[R, C, V]) Replace(cells []table.Cell[R, C, V]) error {
	t.writeContract("Replace")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Replace(cells)
}

func (t *ArrayTable[R, C, V]) Cells() iter.Seq[table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return cellsIterator([]table.Cell[R, C, V](nil))
	}
	t.lock.RLock()
	cells := snapshotCellsSized(t.plain.Cells(), t.plain.Len())
	t.lock.RUnlock()
	return cellsIterator(cells)
}

func (t *ArrayTable[R, C, V]) Row(row R) []table.Cell[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Row(row)
}

func (t *ArrayTable[R, C, V]) Column(column C) []table.Cell[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Column(column)
}

func (t *ArrayTable[R, C, V]) Rows() iter.Seq2[R, []table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return func(func(R, []table.Cell[R, C, V]) bool) {}
	}
	t.lock.RLock()
	rows := t.plain.Rows()
	t.lock.RUnlock()
	return rows
}

func (t *ArrayTable[R, C, V]) Columns() iter.Seq2[C, []table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return func(func(C, []table.Cell[R, C, V]) bool) {}
	}
	t.lock.RLock()
	columns := t.plain.Columns()
	t.lock.RUnlock()
	return columns
}

func (t *ArrayTable[R, C, V]) RowKeys() iter.Seq[R] {
	rows := t.Rows()
	return func(yield func(R) bool) {
		for row := range rows {
			if !yield(row) {
				return
			}
		}
	}
}

func (t *ArrayTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	columns := t.Columns()
	return func(yield func(C) bool) {
		for column := range columns {
			if !yield(column) {
				return
			}
		}
	}
}

func (t *ArrayTable[R, C, V]) Transpose() *ArrayTable[C, R, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return wrapArray(t.plain.Transpose())
}

func (t *ArrayTable[R, C, V]) Clone() *ArrayTable[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return wrapArray(t.plain.Clone())
}

func (t *ArrayTable[R, C, V]) Freeze() *table.ImmutableTable[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Freeze()
}
