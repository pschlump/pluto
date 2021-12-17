package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

type Comparable interface {
	// Compare will return -1 (or a value less than 0) if a.Compare(b) has a < b,
	// 0 if the two are considered to be equal, and
	// +1 (or a value larger than 0) if a.Compare(b) has a > b.  
	// For int this can be implemented as "a - b"
	Compare(b Comparable) int // Compare(b interface{}) int
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

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
func (tt *BinaryTree[T]) Search(find T) ( item *T ) {
	if (*tt).IsEmpty() {
		return nil
	}

	if c := find.Compare(*tt.data); c == 0 {
		return tt.data 
	} else if c < 0 && tt.left != nil {
		return tt.left.Search ( find )
	} else if c > 0 && tt.right != nil {
		return tt.right.Search ( find )
	} 
	return nil
}
	
// TODO remove

