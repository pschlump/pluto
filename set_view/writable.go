/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package set_view

import (
	"errors"
	"iter"
)

// ErrOutsideView reports an attempted write of a value excluded by a filter.
var ErrOutsideView = errors.New("set_view: value is outside the filtered view")

// MutableSet is the common write surface of Pluto's plain and thread-safe sets.
type MutableSet[T any] interface {
	Reader[T]
	Insert(T) bool
	Delete(T) bool
	Truncate()
}

// MutableView is a live write-through set view. Wrap exposes the complete set;
// Filter exposes only accepted values and rejects writes outside that domain.
type MutableView[T any] struct {
	set        MutableSet[T]
	accept     func(T) bool
	unfiltered bool
}

// Wrap returns an unfiltered live write-through view.
func Wrap[T any](set MutableSet[T]) *MutableView[T] {
	return &MutableView[T]{set: set, accept: func(T) bool { return true }, unfiltered: true}
}

// Filter returns a live filtered view. Insert of a rejected value returns
// ErrOutsideView. Delete and Truncate affect only values visible through the view.
func Filter[T any](set MutableSet[T], accept func(T) bool) *MutableView[T] {
	if accept == nil {
		panic("set_view: Filter requires a non-nil predicate")
	}
	return &MutableView[T]{set: set, accept: accept}
}

func (v *MutableView[T]) Contains(value T) bool {
	return v != nil && v.set != nil && v.accept(value) && v.set.Contains(value)
}

// Insert writes through to the underlying set.
func (v *MutableView[T]) Insert(value T) (bool, error) {
	if v == nil || v.set == nil {
		return false, nil
	}
	if !v.accept(value) {
		return false, ErrOutsideView
	}
	return v.set.Insert(value), nil
}

// Delete removes value only when it belongs to the view.
func (v *MutableView[T]) Delete(value T) bool {
	return v != nil && v.set != nil && v.accept(value) && v.set.Delete(value)
}

// Truncate removes all visible values. An unfiltered view uses the underlying
// set's O(1)-or-O(n) Truncate operation; a filtered view deletes matching values.
func (v *MutableView[T]) Truncate() {
	if v == nil || v.set == nil {
		return
	}
	if v.unfiltered {
		v.set.Truncate()
		return
	}
	values := []T{}
	for value := range v.All() {
		values = append(values, value)
	}
	for _, value := range values {
		v.set.Delete(value)
	}
}

func (v *MutableView[T]) Clear() { v.Truncate() }

// All lazily yields visible values and propagates early termination.
func (v *MutableView[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if v == nil || v.set == nil {
			return
		}
		for value := range v.set.All() {
			if v.accept(value) && !yield(value) {
				return
			}
		}
	}
}

func (v *MutableView[T]) Len() int {
	count := 0
	for range v.All() {
		count++
	}
	return count
}

func (v *MutableView[T]) IsEmpty() bool { return v.Len() == 0 }
