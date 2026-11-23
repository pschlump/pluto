/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package tableview contains the shared implementation of live table views.
package tableview

import "iter"

// Cell identifies one table cell without depending on the public table package.
type Cell[R any, C any, V any] struct {
	Row    R
	Column C
	Value  V
}

// Operations normalizes sparse and fixed-universe table mutations.
type Operations[R any, C any, V any] struct {
	Get    func(R, C) (V, bool)
	Put    func(R, C, V) (V, bool, error)
	Delete func(R, C) (V, bool, error)
	Row    func(R) []Cell[R, C, V]
	Column func(C) []Cell[R, C, V]
}

// RowView is a live write-through row. Each call consults the parent table.
type RowView[R any, C any, V any] struct {
	ops Operations[R, C, V]
	row R
}

func NewRow[R any, C any, V any](ops Operations[R, C, V], row R) *RowView[R, C, V] {
	return &RowView[R, C, V]{ops: ops, row: row}
}

func (v *RowView[R, C, V]) Get(column C) (V, bool) { return v.ops.Get(v.row, column) }
func (v *RowView[R, C, V]) Put(column C, value V) (V, bool, error) {
	return v.ops.Put(v.row, column, value)
}
func (v *RowView[R, C, V]) Delete(column C) (V, bool, error) {
	return v.ops.Delete(v.row, column)
}
func (v *RowView[R, C, V]) Contains(column C) bool {
	_, found := v.Get(column)
	return found
}
func (v *RowView[R, C, V]) Cells() []Cell[R, C, V] { return v.ops.Row(v.row) }
func (v *RowView[R, C, V]) Len() int               { return len(v.Cells()) }
func (v *RowView[R, C, V]) IsEmpty() bool          { return v.Len() == 0 }
func (v *RowView[R, C, V]) All() iter.Seq2[C, V] {
	cells := v.Cells()
	return func(yield func(C, V) bool) {
		for _, cell := range cells {
			if !yield(cell.Column, cell.Value) {
				return
			}
		}
	}
}
func (v *RowView[R, C, V]) Clear() error {
	for _, cell := range v.Cells() {
		if _, _, err := v.Delete(cell.Column); err != nil {
			return err
		}
	}
	return nil
}

// ColumnView is a live write-through column. Each call consults the parent table.
type ColumnView[R any, C any, V any] struct {
	ops    Operations[R, C, V]
	column C
}

func NewColumn[R any, C any, V any](ops Operations[R, C, V], column C) *ColumnView[R, C, V] {
	return &ColumnView[R, C, V]{ops: ops, column: column}
}

func (v *ColumnView[R, C, V]) Get(row R) (V, bool) { return v.ops.Get(row, v.column) }
func (v *ColumnView[R, C, V]) Put(row R, value V) (V, bool, error) {
	return v.ops.Put(row, v.column, value)
}
func (v *ColumnView[R, C, V]) Delete(row R) (V, bool, error) {
	return v.ops.Delete(row, v.column)
}
func (v *ColumnView[R, C, V]) Contains(row R) bool {
	_, found := v.Get(row)
	return found
}
func (v *ColumnView[R, C, V]) Cells() []Cell[R, C, V] { return v.ops.Column(v.column) }
func (v *ColumnView[R, C, V]) Len() int               { return len(v.Cells()) }
func (v *ColumnView[R, C, V]) IsEmpty() bool          { return v.Len() == 0 }
func (v *ColumnView[R, C, V]) All() iter.Seq2[R, V] {
	cells := v.Cells()
	return func(yield func(R, V) bool) {
		for _, cell := range cells {
			if !yield(cell.Row, cell.Value) {
				return
			}
		}
	}
}
func (v *ColumnView[R, C, V]) Clear() error {
	for _, cell := range v.Cells() {
		if _, _, err := v.Delete(cell.Row); err != nil {
			return err
		}
	}
	return nil
}

// CellView is a live write-through single cell.
type CellView[R any, C any, V any] struct {
	ops    Operations[R, C, V]
	row    R
	column C
}

func NewCell[R any, C any, V any](ops Operations[R, C, V], row R, column C) *CellView[R, C, V] {
	return &CellView[R, C, V]{ops: ops, row: row, column: column}
}

func (v *CellView[R, C, V]) Get() (V, bool) { return v.ops.Get(v.row, v.column) }
func (v *CellView[R, C, V]) Put(value V) (V, bool, error) {
	return v.ops.Put(v.row, v.column, value)
}
func (v *CellView[R, C, V]) Delete() (V, bool, error) {
	return v.ops.Delete(v.row, v.column)
}
func (v *CellView[R, C, V]) Exists() bool {
	_, found := v.Get()
	return found
}
