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

// columnCapacityHint sizes a column snapshot before its row scan: a column has
// at most one cell per row and at most cellCount cells. Up to
// maxColumnPrealloc cells are reserved up front (one allocation for typical
// tables); beyond that, append growth takes over so a sparse column in a table
// with very many rows does not reserve an O(rows) buffer it will not fill.
func columnCapacityHint(rows, cellCount int) int {
	return min(rows, cellCount, maxColumnPrealloc)
}

const maxColumnPrealloc = 1024

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
