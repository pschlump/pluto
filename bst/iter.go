package bst

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

// Go 1.23+ range-over-func iterators for the tree.  Both iterators use an
// explicit stack instead of recursion, and both support early exit via break.

import (
	"iter"
)

// All returns an iterator over the items in the tree in ascending (in-order)
// sequence.  It can be used directly in a for/range loop:
//
//	for v := range tree.All() {
//		fmt.Println(v)
//	}
func (tt *BinarySearchTree[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		var stk []*BinarySearchTreeNode[T]
		cur := tt.root
		for cur != nil || len(stk) > 0 {
			for cur != nil {
				stk = append(stk, cur)
				cur = cur.left
			}
			cur = stk[len(stk)-1]
			stk[len(stk)-1] = nil // let the GC reclaim popped nodes.
			stk = stk[:len(stk)-1]
			if !yield(*cur.data) {
				return
			}
			cur = cur.right
		}
	}
}

// Backward returns an iterator over the items in the tree in descending
// (reverse in-order) sequence.  It can be used directly in a for/range loop:
//
//	for v := range tree.Backward() {
//		fmt.Println(v)
//	}
func (tt *BinarySearchTree[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		var stk []*BinarySearchTreeNode[T]
		cur := tt.root
		for cur != nil || len(stk) > 0 {
			for cur != nil {
				stk = append(stk, cur)
				cur = cur.right
			}
			cur = stk[len(stk)-1]
			stk[len(stk)-1] = nil // let the GC reclaim popped nodes.
			stk = stk[:len(stk)-1]
			if !yield(*cur.data) {
				return
			}
			cur = cur.left
		}
	}
}
