/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package set_view provides live, lazy, read-only set algebra over ordered or
// hash sets. Iteration deduplicates through operand Contains calls and uses no
// auxiliary set. Views reflect later operand mutations.
package set_view

import "iter"

// Reader is the common read surface of Pluto's plain and thread-safe sets.
type Reader[T any] interface {
	Len() int
	Contains(T) bool
	All() iter.Seq[T]
}

// SetView is a lazy algebraic view. Do not concurrently mutate plain operands.
type SetView[T any] struct {
	kind        string
	left, right Reader[T]
}

func Union[T any](left, right Reader[T]) *SetView[T] {
	return &SetView[T]{kind: "union", left: left, right: right}
}
func Intersection[T any](left, right Reader[T]) *SetView[T] {
	return &SetView[T]{kind: "intersection", left: left, right: right}
}
func Difference[T any](left, right Reader[T]) *SetView[T] {
	return &SetView[T]{kind: "difference", left: left, right: right}
}
func SymmetricDifference[T any](left, right Reader[T]) *SetView[T] {
	return &SetView[T]{kind: "symmetricDifference", left: left, right: right}
}

func (v *SetView[T]) Contains(value T) bool {
	if v == nil {
		return false
	}
	switch v.kind {
	case "union":
		return v.left.Contains(value) || v.right.Contains(value)
	case "intersection":
		return v.left.Contains(value) && v.right.Contains(value)
	case "difference":
		return v.left.Contains(value) && !v.right.Contains(value)
	case "symmetricDifference":
		return v.left.Contains(value) != v.right.Contains(value)
	}
	return false
}
func (v *SetView[T]) Len() int {
	if v == nil {
		return 0
	}
	count := 0
	for range v.All() {
		count++
	}
	return count
}
func (v *SetView[T]) IsEmpty() bool { return v.Len() == 0 }

// All lazily yields each result once and propagates early termination.
func (v *SetView[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if v == nil {
			return
		}
		if v.kind == "intersection" || v.kind == "difference" {
			for value := range v.left.All() {
				if v.Contains(value) && !yield(value) {
					return
				}
			}
			return
		}
		for value := range v.left.All() {
			if (v.kind == "union" || !v.right.Contains(value)) && !yield(value) {
				return
			}
		}
		for value := range v.right.All() {
			if !v.left.Contains(value) && !yield(value) {
				return
			}
		}
	}
}
