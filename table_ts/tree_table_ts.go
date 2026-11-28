/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_ts

import (
	"cmp"
	"iter"
	"sync"

	"github.com/pschlump/pluto/table"
)

type TreeTable[R any, C any, V any] struct {
	lock  sync.RWMutex
	plain *table.TreeTable[R, C, V]
}

func NewTreeTable[R cmp.Ordered, C cmp.Ordered, V any]() *TreeTable[R, C, V] {
	return &TreeTable[R, C, V]{plain: table.NewTreeTable[R, C, V]()}
}

func NewTreeTableFunc[R any, C any, V any](rowCompare func(R, R) int, colCompare func(C, C) int) *TreeTable[R, C, V] {
	return &TreeTable[R, C, V]{plain: table.NewTreeTableFunc[R, C, V](rowCompare, colCompare)}
}

func wrapTree[R any, C any, V any](plain *table.TreeTable[R, C, V]) *TreeTable[R, C, V] {
	if plain == nil {
		return nil
	}
	return &TreeTable[R, C, V]{plain: plain}
}

func (t *TreeTable[R, C, V]) writeContract(caller string) {
	if t == nil || t.plain == nil {
		panic("table_ts: " + caller + " on a nil or zero-value TreeTable")
	}
}

func (t *TreeTable[R, C, V]) Put(row R, column C, value V) (V, bool) {
	t.writeContract("Put")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Put(row, column, value)
}

func (t *TreeTable[R, C, V]) Get(row R, column C) (V, bool) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Get(row, column)
}

func (t *TreeTable[R, C, V]) Delete(row R, column C) (V, bool) {
	if t == nil || t.plain == nil {
		var zero V
		return zero, false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Delete(row, column)
}

func (t *TreeTable[R, C, V]) Contains(row R, column C) bool {
	_, found := t.Get(row, column)
	return found
}

func (t *TreeTable[R, C, V]) ContainsRow(row R) bool {
	if t == nil || t.plain == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.ContainsRow(row)
}

func (t *TreeTable[R, C, V]) ContainsColumn(column C) bool {
	if t == nil || t.plain == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.ContainsColumn(column)
}

func (t *TreeTable[R, C, V]) Len() int {
	if t == nil || t.plain == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Len()
}

func (t *TreeTable[R, C, V]) IsEmpty() bool { return t.Len() == 0 }

func (t *TreeTable[R, C, V]) Clear() {
	if t == nil || t.plain == nil {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Clear()
}

func (t *TreeTable[R, C, V]) Replace(cells []table.Cell[R, C, V]) {
	t.writeContract("Replace")
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Replace(cells)
}

func (t *TreeTable[R, C, V]) Cells() iter.Seq[table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return cellsIterator([]table.Cell[R, C, V](nil))
	}
	t.lock.RLock()
	cells := snapshotCellsSized(t.plain.Cells(), t.plain.Len())
	t.lock.RUnlock()
	return cellsIterator(cells)
}

func (t *TreeTable[R, C, V]) Row(row R) []table.Cell[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Row(row)
}

func (t *TreeTable[R, C, V]) Column(column C) []table.Cell[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Column(column)
}

func (t *TreeTable[R, C, V]) Rows() iter.Seq2[R, []table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return func(func(R, []table.Cell[R, C, V]) bool) {}
	}
	t.lock.RLock()
	rows := t.plain.Rows()
	t.lock.RUnlock()
	return rows
}

func (t *TreeTable[R, C, V]) Columns() iter.Seq2[C, []table.Cell[R, C, V]] {
	if t == nil || t.plain == nil {
		return func(func(C, []table.Cell[R, C, V]) bool) {}
	}
	t.lock.RLock()
	columns := t.plain.Columns()
	t.lock.RUnlock()
	return columns
}

func (t *TreeTable[R, C, V]) RowKeys() iter.Seq[R] {
	rows := t.Rows()
	return func(yield func(R) bool) {
		for row := range rows {
			if !yield(row) {
				return
			}
		}
	}
}

func (t *TreeTable[R, C, V]) ColumnKeys() iter.Seq[C] {
	columns := t.Columns()
	return func(yield func(C) bool) {
		for column := range columns {
			if !yield(column) {
				return
			}
		}
	}
}

func (t *TreeTable[R, C, V]) Transpose() *TreeTable[C, R, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return wrapTree(t.plain.Transpose())
}

func (t *TreeTable[R, C, V]) Clone() *TreeTable[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return wrapTree(t.plain.Clone())
}

func (t *TreeTable[R, C, V]) Freeze() *table.ImmutableTable[R, C, V] {
	if t == nil || t.plain == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.plain.Freeze()
}
