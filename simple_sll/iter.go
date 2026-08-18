package sll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

Modern Go 1.23+ range-over-func iterators for the singly linked list.
The old-style Front/Next/Done/Value iterator in sll.go is kept for
compatibility.
*/

import (
	"iter"
)

// IterateOver returns an iterator over the values in the list, from head to
// tail, as (index, value) pairs.  It can be used directly in a for/range loop:
//
//	for i, v := range list.IterateOver() { ... }
//
// Complexity is O(n) for a full iteration.
func (ns *Sll[T]) IterateOver() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, p := 0, ns.head; p != nil; i, p = i+1, p.next {
			if !yield(i, *p.data) {
				return
			}
		}
	}
}

// IteratePtr returns an iterator over the values in the list, from head to
// tail, as (index, *value) pairs.  It can be used directly in a for/range
// loop:
//
//	for i, v := range list.IteratePtr() { ... }
//
// Complexity is O(n) for a full iteration.
func (ns *Sll[T]) IteratePtr() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		for i, p := 0, ns.head; p != nil; i, p = i+1, p.next {
			if !yield(i, p.data) {
				return
			}
		}
	}
}
