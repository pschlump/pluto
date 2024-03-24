package binary_tree_ts

import (
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/stack"
)

// Implement a state machine based on a YCombinator that allows
// inorder iteration over a binary tree.

// An iteration type that allows a for loop to walk the tree inorder.
//
// This is more moemory effecient than the Walk* functions becasue it
// manages the stack interally.   It is a tiny bit faster than the
// Walk* functions.
//
// The main benefit is that it can be used to make cleaner code.
type BinaryTreeIter[T comparable.Comparable] struct {
	cur  *BinaryTreeElement[T] // Pointer to the current element.
	tree *BinaryTree[T]        // The root of the tree

	// will need a "Stack" of nodes to be able to iterate
	stk stack.Stack[*BinaryTreeElement[T]]
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the inorder traversal beginning of the tree for iteration over tree.
func (tt *BinaryTree[T]) Front() (rv *BinaryTreeIter[T]) {
	// Find the "head" lowest node in tree, point cur at this.
	// Setup the "Stack" so can walk tree.

	rv = &BinaryTreeIter[T]{
		tree: tt,
	}

	// findLeftMost is Local function that searches for the left most
	// node (first inorder node) in the tree.  It has a side-effect
	// of setting up the "stk" stack.
	findLeftMost := func(parent *BinaryTreeElement[T]) (ptr *BinaryTreeElement[T]) {
		ptr = nil
		// fmt.Printf ( "%sFindLeftMost/At Top: at:%s%s\n", MiscLib.ColorCyan, dbgo.LF(), MiscLib.ColorReset)
		if parent == nil {
			// fmt.Printf ( "%sFindLeftMost/no tree: at:%s%s\n", MiscLib.ColorCyan, dbgo.LF(), MiscLib.ColorReset)
			return
		}
		for (*parent).left != nil {
			// fmt.Printf ( "%sAdvance 1 step. at:%s%s\n", MiscLib.ColorCyan, dbgo.LF(), MiscLib.ColorReset)
			rv.stk.Push(parent)
			parent = (*parent).left
		}
		// fmt.Printf ( "%sat bottom at:%s%s\n", MiscLib.ColorCyan, dbgo.LF(), MiscLib.ColorReset)
		ptr = parent
		return
	}

	rv.cur = findLeftMost(tt.root)
	return
}

/*
// Front will start at the last inorder traversal node beginning of the tree for iteration over tree.
func (tt *BinaryTree[T]) Rear() *BinaryTreeIter[T] {
	return &BinaryTreeIter[T] {
		cur: tt.tail,
		tree: tt,
	}
}

// Current will take the node returned from Search or RevrseSearch
// 		func (tt *BinaryTree[T]) Search( t *T ) (rv *BinaryTreeElement[T], pos int) {
// and allow you to start an iteration process from that point.
func (tt *BinaryTree[T]) Current(el *BinaryTreeElement[T]) *BinaryTreeIter[T] {
	return &BinaryTreeIter[T] {
		cur: el,
		tree: tt,
	}
}

// Value returtt the current data for this element in the list.
func (iter *BinaryTreeIter[T]) Value() *T {
	if iter.cur != nil {
		return iter.cur.data
	}
	return nil
}

// Next advances to the next element in the list.
func (iter *BinaryTreeIter[T]) Next() {
	if iter.cur.next != nil {
		iter.cur = iter.cur.next
	}
}

// Prev moves back to the previous element in the list.
func (iter *BinaryTreeIter[T]) Prev() {
	if iter.cur.prev != nil {
		iter.cur = iter.cur.prev
	}
}

// Done returtt true if the end of the list has been reached.
func (iter *BinaryTreeIter[T]) Done() bool {
	return iter.cur != nil
}
*/
