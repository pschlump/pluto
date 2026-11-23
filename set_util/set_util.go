/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package set_util provides lazy combinatorial iterators. Cartesian products
// have multiplicative output size and power sets have 2^n output size; each
// yielded slice is independent and early termination avoids further work.
package set_util

import "iter"

// Cartesian lazily yields tuples in lexicographic odometer order. No arguments
// produce one empty tuple; any empty input produces no tuples.
func Cartesian[T any](inputs ...[]T) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		if len(inputs) == 0 {
			yield([]T{})
			return
		}
		for _, input := range inputs {
			if len(input) == 0 {
				return
			}
		}
		indexes := make([]int, len(inputs))
		for {
			tuple := make([]T, len(inputs))
			for index, input := range inputs {
				tuple[index] = input[indexes[index]]
			}
			if !yield(tuple) {
				return
			}
			position := len(indexes) - 1
			for {
				indexes[position]++
				if indexes[position] < len(inputs[position]) {
					break
				}
				indexes[position] = 0
				position--
				if position < 0 {
					return
				}
			}
		}
	}
}

// PowerSet lazily yields every subset of values in binary-inclusion order.
// Inputs are treated as set elements and should be distinct.
func PowerSet[T any](values []T) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		included := make([]bool, len(values))
		for {
			subset := []T{}
			for index, value := range values {
				if included[index] {
					subset = append(subset, value)
				}
			}
			if !yield(subset) {
				return
			}
			position := 0
			for {
				if position == len(values) {
					return
				}
				included[position] = !included[position]
				if included[position] {
					break
				}
				position++
			}
		}
	}
}
