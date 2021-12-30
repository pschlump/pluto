package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"constraints"
)

// BinaryTree is a generic type buildt on top of a slice
type BinaryTree[T constraints.Ordered] struct {
	data        *T
	left, right *BinaryTree[T]
}

// IsEmpty will return true if the binary-tree is empty
func (tt BinaryTree[T]) IsEmpty() bool {
	return tt.data == nil && tt.left == nil && tt.right == nil
}

func (tt *BinaryTree[T]) Insert(item T) {
	if (*tt).IsEmpty() {
		tt.data = &item
		return
	}

	if item == *(tt.data) {
		tt.data = &item
	} else if item <= *(tt.data) && tt.left == nil {
		tt.left = &(BinaryTree[T]{data: &item})
	} else if item > *(tt.data) && tt.right == nil {
		tt.right = &(BinaryTree[T]{data: &item})
	} else if item <= *(tt.data) {
		tt.left.Insert(item)
	} else {
		tt.right.Insert(item)
	}
}

// TODO remove
// TODO search for item
