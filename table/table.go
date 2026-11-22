/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package table implements sparse hash, ordered sparse, and fixed-universe
// dense tables. A table associates a value with a row/column pair. Rows and
// columns returned by this package are snapshots, not live write-through views.
package table

import (
	"errors"
	"hash/maphash"
	"iter"
)

// Cell is one row/column/value association.
type Cell[R any, C any, V any] struct {
	Row    R `json:"row"`
	Column C `json:"column"`
	Value  V `json:"value"`
}

// ErrOutsideUniverse reports an ArrayTable write outside its fixed key sets.
var ErrOutsideUniverse = errors.New("table: row or column outside fixed universe")

func naturalFunctions[T comparable]() (func(T, T) bool, func(T) uint64) {
	seed := maphash.MakeSeed()
	return func(a, b T) bool { return a == b }, func(value T) uint64 {
		return maphash.Comparable(seed, value)
	}
}

func copyCells[R any, C any, V any](cells []Cell[R, C, V]) []Cell[R, C, V] {
	if cells == nil {
		return nil
	}
	return append([]Cell[R, C, V](nil), cells...)
}

func cellSequence[R any, C any, V any](cells []Cell[R, C, V]) iter.Seq[Cell[R, C, V]] {
	return func(yield func(Cell[R, C, V]) bool) {
		for _, cell := range cells {
			if !yield(cell) {
				return
			}
		}
	}
}
