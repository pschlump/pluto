/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package iter_util provides lazy algorithms and explicit materialization for
// Go's push and pull iterator types.
package iter_util

import "iter"

// Pair is the materialized form of one key/value or pair iteration item.
type Pair[A, B any] struct {
	First  A
	Second B
}

// Concat lazily yields every sequence in argument order.
func Concat[T any](sequences ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, sequence := range sequences {
			for value := range sequence {
				if !yield(value) {
					return
				}
			}
		}
	}
}

// Filter lazily yields values for which keep returns true.
func Filter[T any](sequence iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	if keep == nil {
		panic("iter_util: nil filter")
	}
	return func(yield func(T) bool) {
		for value := range sequence {
			if keep(value) && !yield(value) {
				return
			}
		}
	}
}

// Map lazily transforms every input value.
func Map[A, B any](sequence iter.Seq[A], transform func(A) B) iter.Seq[B] {
	if transform == nil {
		panic("iter_util: nil transform")
	}
	return func(yield func(B) bool) {
		for value := range sequence {
			if !yield(transform(value)) {
				return
			}
		}
	}
}

// Take lazily yields at most count values. A negative count is a programmer
// error. Taking zero values does not start the input sequence.
func Take[T any](sequence iter.Seq[T], count int) iter.Seq[T] {
	if count < 0 {
		panic("iter_util: negative take count")
	}
	return func(yield func(T) bool) {
		if count == 0 {
			return
		}
		remaining := count
		for value := range sequence {
			if !yield(value) {
				return
			}
			remaining--
			if remaining == 0 {
				return
			}
		}
	}
}

// Skip lazily discards at most count values and yields the remainder. A
// negative count is a programmer error.
func Skip[T any](sequence iter.Seq[T], count int) iter.Seq[T] {
	if count < 0 {
		panic("iter_util: negative skip count")
	}
	return func(yield func(T) bool) {
		remaining := count
		for value := range sequence {
			if remaining > 0 {
				remaining--
				continue
			}
			if !yield(value) {
				return
			}
		}
	}
}

// Partition lazily yields independent slices of at most size values. The last
// slice may be shorter. Size must be positive.
func Partition[T any](sequence iter.Seq[T], size int) iter.Seq[[]T] {
	if size <= 0 {
		panic("iter_util: non-positive partition size")
	}
	return func(yield func([]T) bool) {
		part := make([]T, 0, size)
		for value := range sequence {
			part = append(part, value)
			if len(part) == size {
				if !yield(part) {
					return
				}
				part = make([]T, 0, size)
			}
		}
		if len(part) != 0 {
			yield(part)
		}
	}
}

// Frequency consumes sequence and counts equal values.
func Frequency[T comparable](sequence iter.Seq[T]) map[T]int {
	result := make(map[T]int)
	for value := range sequence {
		result[value]++
	}
	return result
}

// ElementsEqual reports whether both sequences contain equal values in the
// same order. It stops as soon as a difference is known.
func ElementsEqual[T comparable](left, right iter.Seq[T]) bool {
	return ElementsEqualFunc(left, right, func(a, b T) bool { return a == b })
}

// ElementsEqualFunc is ElementsEqual with caller-supplied equality.
func ElementsEqualFunc[T any](left, right iter.Seq[T], equal func(T, T) bool) bool {
	if equal == nil {
		panic("iter_util: nil equality")
	}
	leftNext, leftStop := iter.Pull(left)
	defer leftStop()
	rightNext, rightStop := iter.Pull(right)
	defer rightStop()
	for {
		leftValue, leftOK := leftNext()
		rightValue, rightOK := rightNext()
		if leftOK != rightOK {
			return false
		}
		if !leftOK {
			return true
		}
		if !equal(leftValue, rightValue) {
			return false
		}
	}
}

// First returns the first value, if any, and stops the input immediately.
func First[T any](sequence iter.Seq[T]) (T, bool) {
	for value := range sequence {
		return value, true
	}
	var zero T
	return zero, false
}

// Last consumes sequence and returns its final value, if any.
func Last[T any](sequence iter.Seq[T]) (T, bool) {
	var result T
	found := false
	for value := range sequence {
		result, found = value, true
	}
	return result, found
}

// Only returns the sole value. It returns false for an empty sequence or after
// observing a second value, without consuming the remainder.
func Only[T any](sequence iter.Seq[T]) (T, bool) {
	var result T
	count := 0
	for value := range sequence {
		if count == 1 {
			var zero T
			return zero, false
		}
		result = value
		count++
	}
	return result, count == 1
}

// At returns the zero-based value at index. A negative index returns false
// without starting the sequence.
func At[T any](sequence iter.Seq[T], index int) (T, bool) {
	if index < 0 {
		var zero T
		return zero, false
	}
	for value := range sequence {
		if index == 0 {
			return value, true
		}
		index--
	}
	var zero T
	return zero, false
}

// IsEmpty reports whether sequence yields no values. Non-empty inputs are
// stopped after their first value.
func IsEmpty[T any](sequence iter.Seq[T]) bool {
	_, found := First(sequence)
	return !found
}

// Len consumes sequence and returns its number of values.
func Len[T any](sequence iter.Seq[T]) int {
	count := 0
	for range sequence {
		count++
	}
	return count
}

// FromSlice returns an iterator over a snapshot of values.
func FromSlice[T any](values []T) iter.Seq[T] {
	snapshot := append([]T(nil), values...)
	return func(yield func(T) bool) {
		for _, value := range snapshot {
			if !yield(value) {
				return
			}
		}
	}
}

// ToSlice explicitly materializes all sequence values.
func ToSlice[T any](sequence iter.Seq[T]) []T {
	return slicesCollect(sequence)
}

func slicesCollect[T any](sequence iter.Seq[T]) []T {
	result := []T{}
	for value := range sequence {
		result = append(result, value)
	}
	return result
}

// Pairs adapts a two-value sequence to a sequence of Pair values.
func Pairs[A, B any](sequence iter.Seq2[A, B]) iter.Seq[Pair[A, B]] {
	return func(yield func(Pair[A, B]) bool) {
		for first, second := range sequence {
			if !yield(Pair[A, B]{First: first, Second: second}) {
				return
			}
		}
	}
}

// Seq2 adapts a Pair sequence to a two-value sequence.
func Seq2[A, B any](sequence iter.Seq[Pair[A, B]]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for pair := range sequence {
			if !yield(pair.First, pair.Second) {
				return
			}
		}
	}
}

// Keys lazily yields only the first value from each sequence item.
func Keys[A, B any](sequence iter.Seq2[A, B]) iter.Seq[A] {
	return func(yield func(A) bool) {
		for key := range sequence {
			if !yield(key) {
				return
			}
		}
	}
}

// Values lazily yields only the second value from each sequence item.
func Values[A, B any](sequence iter.Seq2[A, B]) iter.Seq[B] {
	return func(yield func(B) bool) {
		for _, value := range sequence {
			if !yield(value) {
				return
			}
		}
	}
}

// Peeking converts a push sequence into a pull cursor with one-value lookahead.
// Call Close when abandoning the cursor before exhaustion.
type Peeking[T any] struct {
	next   func() (T, bool)
	stop   func()
	value  T
	peeked bool
	closed bool
}

// NewPeeking starts a pull cursor. The first input value is not requested until
// Peek or Next is called.
func NewPeeking[T any](sequence iter.Seq[T]) *Peeking[T] {
	next, stop := iter.Pull(sequence)
	return &Peeking[T]{next: next, stop: stop}
}

// Peek returns the next value without consuming it. Repeated calls are
// idempotent until Next is called.
func (cursor *Peeking[T]) Peek() (T, bool) {
	if cursor.closed {
		var zero T
		return zero, false
	}
	if !cursor.peeked {
		value, found := cursor.next()
		if !found {
			cursor.Close()
			return value, false
		}
		cursor.value, cursor.peeked = value, true
	}
	return cursor.value, true
}

// Next consumes and returns the next value.
func (cursor *Peeking[T]) Next() (T, bool) {
	if !cursor.peeked {
		if cursor.closed {
			var zero T
			return zero, false
		}
		value, found := cursor.next()
		if !found {
			cursor.Close()
		}
		return value, found
	}
	value := cursor.value
	var zero T
	cursor.value = zero
	cursor.peeked = false
	return value, true
}

// Close releases the pull iterator. It is idempotent and discards lookahead.
func (cursor *Peeking[T]) Close() {
	if cursor.closed {
		return
	}
	cursor.closed = true
	var zero T
	cursor.value = zero
	cursor.peeked = false
	cursor.stop()
}
