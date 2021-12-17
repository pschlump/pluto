package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"github.com/pschlump/pluto/comparable"
)

// BinaryTree is a generic type buildt on top of a slice
type BinaryTree[T comparable.Comparable] struct {
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
	
func (tt *BinaryTree[T]) Remove(find T) ( found bool ) {
	// This is a little bit tricky.  To delare a local pointer to a function
	// that can recursivly call itslef you have to first declare the pointer
	// then initialize the pointer.   If you try to do this in one step 
	// it will error.  The variable is not declared until the end of the
	// funtion that initialized it - so it can't be used inside itself.
	var internalRemove func ( parent **BinaryTree[T], find T ) bool 
	internalRemove = func ( parent **BinaryTree[T], find T ) bool {
		this := **parent
		if cc := find.Compare ( *(this.data) ) ; cc == 0 {
			*parent = this.left
		} else if cc < 0 && this.left != nil {
			return internalRemove ( &this.left, find ) 
		} else if cc > 0 && this.right != nil {
			return internalRemove ( &this.right, find )
		} 
		return false
	}

	if c := find.Compare(*tt.data); c == 0 {
		if (*tt).left != nil {
			if (*tt).left.left == nil && (*tt).left.right != nil {
				(*tt).data = ((*tt).left.data)
				(*tt).left = (*tt).left.right // I think I need to go find the "right most" child at the leaf level and promote that.
			} else if (*tt).left.left != nil && (*tt).left.right == nil {
				(*tt).data = ((*tt).left.data)
				// not certin this is correct
				(*tt).left = (*tt).left.left
			} else {
				// have 2 children!
				panic("not implemented yet")
			}
		} else if (*tt).right != nil {
			(*tt).data = ((*tt).right.data)
		} else {
			(*tt).data = nil
		}
		return true
	} else if c < 0 && tt.left != nil {
		return internalRemove ( &tt.left, find )
	} else if c > 0 && tt.left != nil {
		return internalRemove ( &tt.right, find )
	} else {
		return false
	}

	return
}

