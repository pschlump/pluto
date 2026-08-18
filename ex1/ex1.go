// Package ex1 is a minimal example of a generic binary search tree,
// demonstrating Go type parameters.
//
// The tree is not balanced, so operations degrade to O(n) in the worst
// case.  For a production binary search tree see the binary_tree package;
// for a self-balancing tree see the avl_tree package.
//
// BinaryTree is not safe for concurrent use.
package ex1

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"cmp"
	"iter"
)

// BinaryTree is a generic binary search tree built from linked nodes.
// The zero value is an empty tree ready for use.
type BinaryTree[T cmp.Ordered] struct {
	data        *T
	left, right *BinaryTree[T]
}

// IsEmpty reports whether the tree is empty.
func (tt BinaryTree[T]) IsEmpty() bool {
	return tt.data == nil && tt.left == nil && tt.right == nil
}

// Insert adds item to the tree.  Inserting an item that compares equal to
// an existing node replaces that node's value.
func (tt *BinaryTree[T]) Insert(item T) {
	if tt.IsEmpty() {
		tt.data = &item
		return
	}

	if item == *tt.data {
		tt.data = &item
	} else if item <= *tt.data && tt.left == nil {
		tt.left = &BinaryTree[T]{data: &item}
	} else if item > *tt.data && tt.right == nil {
		tt.right = &BinaryTree[T]{data: &item}
	} else if item <= *tt.data {
		tt.left.Insert(item)
	} else {
		tt.right.Insert(item)
	}
}

// All returns an iterator over the items of the tree in ascending
// (in-order) order, for use with range-over-func:
//
//	for v := range t.All() {
//		fmt.Println(v)
//	}
func (tt *BinaryTree[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt == nil {
			return
		}
		if tt.left != nil {
			for v := range tt.left.All() {
				if !yield(v) {
					return
				}
			}
		}
		if tt.data != nil {
			if !yield(*tt.data) {
				return
			}
		}
		if tt.right != nil {
			for v := range tt.right.All() {
				if !yield(v) {
					return
				}
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */
