package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

type Comparable interface {
	Compare(b interface{}) int
}

// BinaryTree is a generic type buildt on top of a slice
type BinaryTree[T Comparable] struct {
	data *T
	left, right *BinaryTree[T]
}

// IsEmpty will return true if the binary-tree is empty
func (tt BinaryTree[T]) IsEmpty() bool {
	return tt.data == nil && tt.left == nil && tt.right == nil
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
func (tt *BinaryTree[T]) Insert(item T) {
	if (*tt).IsEmpty() {
		tt.data = &item
		return
	}

	if c := item.Compare(*tt.data); c == 0 {
		tt.data = &item
	} else if c < 0  && tt.left == nil {
		tt.left = &(BinaryTree[T]{ data: &item })
	} else if c > 0  && tt.right == nil {
		tt.right = &(BinaryTree[T]{ data: &item })
	} else if c < 0 {
		tt.left.Insert ( item )
	} else {
		tt.right.Insert ( item )
	}
}

// TODO remove
// TODO search for item

