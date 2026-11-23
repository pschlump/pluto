/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import "github.com/pschlump/pluto/internal/tableview"

// RowView, ColumnView, and CellView are live write-through views. Their Cells
// and All methods take a fresh snapshot for each iteration.
type RowView[R any, C any, V any] = tableview.RowView[R, C, V]
type ColumnView[R any, C any, V any] = tableview.ColumnView[R, C, V]
type CellView[R any, C any, V any] = tableview.CellView[R, C, V]

func convertViewCells[R any, C any, V any](cells []Cell[R, C, V]) []tableview.Cell[R, C, V] {
	result := make([]tableview.Cell[R, C, V], len(cells))
	for index, cell := range cells {
		result[index] = tableview.Cell[R, C, V]{Row: cell.Row, Column: cell.Column, Value: cell.Value}
	}
	return result
}

func hashViewOperations[R any, C any, V any](t *HashTable[R, C, V]) tableview.Operations[R, C, V] {
	return tableview.Operations[R, C, V]{
		Get: t.Get,
		Put: func(row R, column C, value V) (V, bool, error) {
			old, found := t.Put(row, column, value)
			return old, found, nil
		},
		Delete: func(row R, column C) (V, bool, error) {
			old, found := t.Delete(row, column)
			return old, found, nil
		},
		Row:    func(row R) []tableview.Cell[R, C, V] { return convertViewCells(t.Row(row)) },
		Column: func(column C) []tableview.Cell[R, C, V] { return convertViewCells(t.Column(column)) },
	}
}

func treeViewOperations[R any, C any, V any](t *TreeTable[R, C, V]) tableview.Operations[R, C, V] {
	return tableview.Operations[R, C, V]{
		Get: t.Get,
		Put: func(row R, column C, value V) (V, bool, error) {
			old, found := t.Put(row, column, value)
			return old, found, nil
		},
		Delete: func(row R, column C) (V, bool, error) {
			old, found := t.Delete(row, column)
			return old, found, nil
		},
		Row:    func(row R) []tableview.Cell[R, C, V] { return convertViewCells(t.Row(row)) },
		Column: func(column C) []tableview.Cell[R, C, V] { return convertViewCells(t.Column(column)) },
	}
}

func arrayViewOperations[R any, C any, V any](t *ArrayTable[R, C, V]) tableview.Operations[R, C, V] {
	return tableview.Operations[R, C, V]{
		Get:    t.Get,
		Put:    t.Put,
		Delete: t.Delete,
		Row:    func(row R) []tableview.Cell[R, C, V] { return convertViewCells(t.Row(row)) },
		Column: func(column C) []tableview.Cell[R, C, V] { return convertViewCells(t.Column(column)) },
	}
}

func (t *HashTable[R, C, V]) RowView(row R) *RowView[R, C, V] {
	return tableview.NewRow(hashViewOperations(t), row)
}
func (t *HashTable[R, C, V]) ColumnView(column C) *ColumnView[R, C, V] {
	return tableview.NewColumn(hashViewOperations(t), column)
}
func (t *HashTable[R, C, V]) CellView(row R, column C) *CellView[R, C, V] {
	return tableview.NewCell(hashViewOperations(t), row, column)
}
func (t *TreeTable[R, C, V]) RowView(row R) *RowView[R, C, V] {
	return tableview.NewRow(treeViewOperations(t), row)
}
func (t *TreeTable[R, C, V]) ColumnView(column C) *ColumnView[R, C, V] {
	return tableview.NewColumn(treeViewOperations(t), column)
}
func (t *TreeTable[R, C, V]) CellView(row R, column C) *CellView[R, C, V] {
	return tableview.NewCell(treeViewOperations(t), row, column)
}
func (t *ArrayTable[R, C, V]) RowView(row R) *RowView[R, C, V] {
	return tableview.NewRow(arrayViewOperations(t), row)
}
func (t *ArrayTable[R, C, V]) ColumnView(column C) *ColumnView[R, C, V] {
	return tableview.NewColumn(arrayViewOperations(t), column)
}
func (t *ArrayTable[R, C, V]) CellView(row R, column C) *CellView[R, C, V] {
	return tableview.NewCell(arrayViewOperations(t), row, column)
}
