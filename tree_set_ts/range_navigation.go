/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set_ts

import (
	"iter"
	"slices"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the largest element strictly less than key.
func (t *TreeSet[T]) Lower(key T) (T, bool) {
	if t == nil {
		return *new(T), false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Lower(key)
}

// Higher returns the smallest element strictly greater than key.
func (t *TreeSet[T]) Higher(key T) (T, bool) {
	if t == nil {
		return *new(T), false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.Higher(key)
}

// RangeOf returns a snapshot iterator over an open/closed/unbounded range.
func (t *TreeSet[T]) RangeOf(value ranges.Range[T]) iter.Seq[T] {
	if t == nil {
		return func(func(T) bool) {}
	}
	t.lock.RLock()
	values := []T{}
	for item := range t.inner.RangeOf(value) {
		values = append(values, item)
	}
	t.lock.RUnlock()
	return slices.Values(values)
}
