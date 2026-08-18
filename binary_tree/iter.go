package binary_tree

import (
	"iter"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/stack"
)

// BinaryTreeIter is an old-style (Front/Next/Done/Value) in-order iterator
// over a BinaryTree.  It is more memory efficient than the Walk* functions
// because it manages an explicit stack of only the pending nodes instead of
// using recursion, and unlike Walk* it can be paused and resumed.
//
// The main benefit is that it can be used to make cleaner code:
//
//	for it := tree.Front(); !it.Done(); it.Next() {
//		v := it.Value()
//		...
//	}
//
// For most new code the All and Backward range-over-func iterators are a
// simpler choice.
type BinaryTreeIter[T comparable.Comparable] struct {
	cur *BinaryTreeElement[T] // Pointer to the current element.

	// Stack of nodes pending a visit; the top of the stack is next.
	stk stack.Stack[*BinaryTreeElement[T]]
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the in-order traversal beginning of the tree for
// iteration over the tree.
func (tt *BinaryTree[T]) Front() (rv *BinaryTreeIter[T]) {
	rv = &BinaryTreeIter[T]{}
	// Push the spine of left children so the smallest element is on top.
	for n := tt.root; n != nil; n = n.left {
		rv.stk.Push(n)
	}
	rv.Next()
	return
}

// Value returns the current data for this element in the tree, or nil if the
// iteration is done.
func (iter *BinaryTreeIter[T]) Value() *T {
	if iter.cur != nil {
		return iter.cur.data
	}
	return nil
}

// Next advances to the next element of the tree in in-order order.
func (iter *BinaryTreeIter[T]) Next() {
	n, err := iter.stk.Pop()
	if err != nil {
		iter.cur = nil
		return
	}
	iter.cur = n
	// Push the spine of left children of the right subtree, if any.
	for r := n.right; r != nil; r = r.left {
		iter.stk.Push(r)
	}
}

// Done returns true if the end of the tree has been reached.
func (iter *BinaryTreeIter[T]) Done() bool {
	return iter.cur == nil
}

// -------------------------------------------------------------------------------------------------------

// All returns an iterator that yields every element of the tree in in-order
// (ascending) order.  It is a Go 1.23 range-over-func iterator:
//
//	for v := range tree.All() {
//		...
//	}
func (tt *BinaryTree[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		var stk []*BinaryTreeElement[T]
		n := tt.root
		for n != nil || len(stk) > 0 {
			for n != nil {
				stk = append(stk, n)
				n = n.left
			}
			n = stk[len(stk)-1]
			stk = stk[:len(stk)-1]
			if !yield(n.data) {
				return
			}
			n = n.right
		}
	}
}

// Backward returns an iterator that yields every element of the tree in
// reverse in-order (descending) order.  It is a Go 1.23 range-over-func
// iterator:
//
//	for v := range tree.Backward() {
//		...
//	}
func (tt *BinaryTree[T]) Backward() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		var stk []*BinaryTreeElement[T]
		n := tt.root
		for n != nil || len(stk) > 0 {
			for n != nil {
				stk = append(stk, n)
				n = n.right
			}
			n = stk[len(stk)-1]
			stk = stk[:len(stk)-1]
			if !yield(n.data) {
				return
			}
			n = n.left
		}
	}
}
