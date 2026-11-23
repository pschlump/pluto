/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import "github.com/pschlump/pluto/table"

// Table is the existing independent read-only table snapshot.
type Table[R any, C any, V any] = table.ImmutableTable[R, C, V]

func CopyOfTable[R any, C any, V any](value *Table[R, C, V]) *Table[R, C, V] { return value }

func FreezeHashTable[R any, C any, V any](source *table.HashTable[R, C, V]) *Table[R, C, V] {
	return source.Freeze()
}

func FreezeTreeTable[R any, C any, V any](source *table.TreeTable[R, C, V]) *Table[R, C, V] {
	return source.Freeze()
}

func FreezeArrayTable[R any, C any, V any](source *table.ArrayTable[R, C, V]) *Table[R, C, V] {
	return source.Freeze()
}
