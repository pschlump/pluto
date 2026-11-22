/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package multiset implements hash and sorted multisets. A multiset stores
// distinct values together with positive occurrence counts. Len counts every
// occurrence; DistinctLen counts stored values. All counts and sizes are int.
// Nil and zero-value multisets are empty for reads. Constructors are required
// before storing values.
package multiset

import "fmt"

// Entry associates a distinct value with its positive occurrence count.
type Entry[T any] struct {
	Value T   `json:"value"`
	Count int `json:"count"`
}

const maxInt = int(^uint(0) >> 1)

func checkedCount(value string, count int) {
	if count < 0 {
		panic(fmt.Sprintf("multiset: %s called with a negative count", value))
	}
}

func checkedAddition(left, right int) int {
	if right > 0 && left > maxInt-right {
		panic("multiset: count or size overflow")
	}
	return left + right
}
