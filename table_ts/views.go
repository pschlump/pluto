/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_ts

import (
	"sync"

	"github.com/pschlump/pluto/internal/tableview"
	"github.com/pschlump/pluto/table"
)

type RowView[R any, C any, V any] = table.RowView[R, C, V]
type ColumnView[R any, C any, V any] = table.ColumnView[R, C, V]
type CellView[R any, C any, V any] = table.CellView[R, C, V]

func twinCells[R any, C any, V any](cells []table.Cell[R, C, V]) []tableview.Cell[R, C, V] {
	result := make([]tableview.Cell[R, C, V], len(cells))
	for index, cell := range cells {
		result[index] = tableview.Cell[R, C, V]{Row: cell.Row, Column: cell.Column, Value: cell.Value}
	}
	return result
}

func twinViewOperations[R any, C any, V any](
	lock *sync.RWMutex,
	get func(R, C) (V, bool),
	put func(R, C, V) (V, bool, error),
	remove func(R, C) (V, bool, error),
	row func(R) []table.Cell[R, C, V],
	column func(C) []table.Cell[R, C, V],
) tableview.Operations[R, C, V] {
	return tableview.Operations[R, C, V]{
		Get: func(r R, c C) (V, bool) {
			lock.RLock()
			defer lock.RUnlock()
			return get(r, c)
		},
		Put: func(r R, c C, value V) (V, bool, error) {
			lock.Lock()
			defer lock.Unlock()
			return put(r, c, value)
		},
		Delete: func(r R, c C) (V, bool, error) {
			lock.Lock()
			defer lock.Unlock()
			return remove(r, c)
		},
		Row: func(r R) []tableview.Cell[R, C, V] {
			lock.RLock()
			defer lock.RUnlock()
			return twinCells(row(r))
		},
		Column: func(c C) []tableview.Cell[R, C, V] {
			lock.RLock()
			defer lock.RUnlock()
			return twinCells(column(c))
		},
	}
}

func hashTwinViewOperations[R any, C any, V any](t *HashTable[R, C, V]) tableview.Operations[R, C, V] {
	return twinViewOperations(&t.lock, t.plain.Get,
		func(row R, column C, value V) (V, bool, error) {
			old, found := t.plain.Put(row, column, value)
			return old, found, nil
		},
		func(row R, column C) (V, bool, error) {
			old, found := t.plain.Delete(row, column)
			return old, found, nil
		}, t.plain.Row, t.plain.Column)
}

func treeTwinViewOperations[R any, C any, V any](t *TreeTable[R, C, V]) tableview.Operations[R, C, V] {
	return twinViewOperations(&t.lock, t.plain.Get,
		func(row R, column C, value V) (V, bool, error) {
			old, found := t.plain.Put(row, column, value)
			return old, found, nil
		},
		func(row R, column C) (V, bool, error) {
			old, found := t.plain.Delete(row, column)
			return old, found, nil
		}, t.plain.Row, t.plain.Column)
}

func arrayTwinViewOperations[R any, C any, V any](t *ArrayTable[R, C, V]) tableview.Operations[R, C, V] {
	return twinViewOperations(&t.lock, t.plain.Get, t.plain.Put, t.plain.Delete, t.plain.Row, t.plain.Column)
}

func (t *HashTable[R, C, V]) RowView(row R) *RowView[R, C, V] {
	return tableview.NewRow(hashTwinViewOperations(t), row)
}
func (t *HashTable[R, C, V]) ColumnView(column C) *ColumnView[R, C, V] {
	return tableview.NewColumn(hashTwinViewOperations(t), column)
}
func (t *HashTable[R, C, V]) CellView(row R, column C) *CellView[R, C, V] {
	return tableview.NewCell(hashTwinViewOperations(t), row, column)
}
func (t *TreeTable[R, C, V]) RowView(row R) *RowView[R, C, V] {
	return tableview.NewRow(treeTwinViewOperations(t), row)
}
func (t *TreeTable[R, C, V]) ColumnView(column C) *ColumnView[R, C, V] {
	return tableview.NewColumn(treeTwinViewOperations(t), column)
}
func (t *TreeTable[R, C, V]) CellView(row R, column C) *CellView[R, C, V] {
	return tableview.NewCell(treeTwinViewOperations(t), row, column)
}
func (t *ArrayTable[R, C, V]) RowView(row R) *RowView[R, C, V] {
	return tableview.NewRow(arrayTwinViewOperations(t), row)
}
func (t *ArrayTable[R, C, V]) ColumnView(column C) *ColumnView[R, C, V] {
	return tableview.NewColumn(arrayTwinViewOperations(t), column)
}
func (t *ArrayTable[R, C, V]) CellView(row R, column C) *CellView[R, C, V] {
	return tableview.NewCell(arrayTwinViewOperations(t), row, column)
}
