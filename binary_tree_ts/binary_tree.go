/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// Package binary_tree_ts implements a generic, unbalanced binary search tree
// that is safe for concurrent use.  Every operation is guarded by a
// sync.RWMutex.  It exposes the same API as
// github.com/pschlump/pluto/binary_tree.
//
// Basic operations on a Binary Tree:
//
//	Insert — create a new element in the tree; a duplicate replaces the existing element.
//	Delete — delete a specified element from the tree (elements can be found via Search).
//	Search — return the given element from the tree.
//	Index — return the Nth element of the tree in in-order order.
//	IsEmpty — report whether the tree is empty.
//	Len / Length — number of elements in the tree; 0 is an empty tree.
//	Reverse — swap the left and right children of every node in the tree.
//	Truncate — delete all the nodes in the tree.
//	FindMin / FindMax — return the smallest / largest element in the tree.
//	DeleteAtHead — delete the smallest element (Delete(FindMin())).
//	DeleteAtTail — delete the largest element (Delete(FindMax())).
//	Depth — number of levels in the deepest part of the tree.
//	WalkInOrder / WalkPreOrder / WalkPostOrder — callback-based traversals.
//	Front — old-style in-order iterator (operates on a snapshot).
//	All / Backward — Go 1.23 range-over-func iterators (operate on a snapshot).
//
// Insert, Delete and Search are O(log₂ n) on average for randomly ordered
// input and O(n) in the worst case (the tree is NOT self-balancing).
package binary_tree_ts

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pschlump/pluto/comparable"
)

// BinaryTreeElement is a single node of a BinaryTree.
type BinaryTreeElement[T comparable.Comparable] struct {
	data        *T
	left, right *BinaryTreeElement[T]
}

// BinaryTree is a generic binary tree
type BinaryTree[T comparable.Comparable] struct {
	root   *BinaryTreeElement[T]
	length int
	lock   sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// NewBinaryTree creates a new BinaryTree and returns it.
// Complexity is O(1).
func NewBinaryTree[T comparable.Comparable]() *BinaryTree[T] {
	return &BinaryTree[T]{
		root:   nil,
		length: 0,
	}
}

// GetData returns the data stored in this element.
// Complexity is O(1).
func (ee *BinaryTreeElement[T]) GetData() *T {
	return ee.data
}

// SetData replaces the data stored in this element.
// Complexity is O(1).
func (ee *BinaryTreeElement[T]) SetData(x *T) {
	ee.data = x
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the binary-tree is empty.
// Complexity is O(1).
func (tt *BinaryTree[T]) IsEmpty() bool {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlIsEmpty()
}

// nlIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (tt *BinaryTree[T]) nlIsEmpty() bool {
	return tt.root == nil
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (tt *BinaryTree[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.root = nil
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an existing
// item the new item will replace the existing one and false is returned;
// true is returned when a new node was added.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) Insert(item *T) (vv bool) {
	if tt == nil {
		panic("binary_tree_ts: Insert called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	node := &BinaryTreeElement[T]{data: item}
	if tt.nlIsEmpty() {
		tt.root = node
		tt.length = 1
		return true
	}

	// Simple is recursive, can be replaced with an iterative tree traversal.
	var insert func(root **BinaryTreeElement[T]) bool
	insert = func(root **BinaryTreeElement[T]) bool {
		if *root == nil {
			*root = node
			tt.length++
			return true
		} else if c := (*item).Compare(*(*root).data); c == 0 {
			node.left = (*root).left
			node.right = (*root).right
			*root = node
			return false
		} else if c < 0 {
			return insert(&(*root).left)
		} else {
			return insert(&(*root).right)
		}
	}

	vv = insert(&tt.root)
	return
}

// Len returns the number of elements in the tree.
// Complexity is O(1).
func (tt *BinaryTree[T]) Len() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Length returns the number of elements in the tree.
// Complexity is O(1).
func (tt *BinaryTree[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Search will walk the tree looking for `find` and return the found item
// if it is in the tree. If it is not found then `nil` will be returned.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) Search(find *T) (item *T) {
	if tt == nil {
		panic("binary_tree_ts: Search called on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	cur := tt.root
	for cur != nil {
		c := (*find).Compare(*cur.data)
		if c == 0 {
			return cur.data
		}
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	return nil
}

// Dump will print out the tree to the writer `fo`.
// Complexity is O(n).
func (tt *BinaryTree[T]) Dump(fo io.Writer) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	k := tt.nlDepth() * 4
	var inorderTraversal func(cur *BinaryTreeElement[T], n int) bool
	inorderTraversal = func(cur *BinaryTreeElement[T], n int) bool {
		if cur == nil {
			return true
		}
		if cur.left != nil {
			if !inorderTraversal(cur.left, n+1) {
				return false
			}
		}
		_, err := fmt.Fprintf(fo, "%s%v%s (left=%p/%p, right=%p/%p) self=%p\n",
			strings.Repeat(" ", 4*n), *cur.data, strings.Repeat(" ", k-(4*n)),
			cur.left, &cur.left, cur.right, &cur.right, cur)
		if err != nil {
			return false
		}
		if cur.right != nil {
			if !inorderTraversal(cur.right, n+1) {
				return false
			}
		}
		return true
	}
	inorderTraversal(tt.root, 0)
}

// Delete removes the element matching `find` from the tree, returning true
// if an element was found and removed.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) Delete(find *T) (found bool) {
	if tt == nil {
		panic("binary_tree_ts: Delete called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.deleteBy(func(data *T) int {
		return (*find).Compare(*data)
	})
}

// DeleteMatch is like Delete but uses the caller supplied comparison
// function `fx` (with the same contract as Compare) instead of the
// Compare method of T.
func (tt *BinaryTree[T]) DeleteMatch(find *T, fx func(a, b *T) int) (found bool) {
	if tt == nil {
		panic("binary_tree_ts: DeleteMatch called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.deleteBy(func(data *T) int {
		return fx(find, data)
	})
}

// deleteBy removes the node for which cmp(data) == 0, using cmp to steer the
// descent through the tree.  The caller must hold the write lock.
func (tt *BinaryTree[T]) deleteBy(cmp func(data *T) int) (found bool) {
	cur := &tt.root
	for *cur != nil {
		c := cmp((*cur).data)
		if c == 0 {
			tt.length--
			switch {
			case (*cur).left == nil:
				// No left child (leaf, or right child only): splice in the right subtree.
				*cur = (*cur).right
			case (*cur).right == nil:
				// Left child only: splice in the left subtree.
				*cur = (*cur).left
			default:
				// Two children: promote the in-order successor (the left-most
				// node of the right subtree) into this node, then splice the
				// successor out of the right subtree.  The successor has no
				// left child, so its right subtree is spliced in its place.
				pSucc := &(*cur).right
				for (*pSucc).left != nil {
					pSucc = &(*pSucc).left
				}
				(*cur).data = (*pSucc).data
				*pSucc = (*pSucc).right
			}
			return true
		}
		if c < 0 {
			cur = &(*cur).left
		} else {
			cur = &(*cur).right
		}
	}
	return false
}

/*
        {00}
    {02}
        {03}
{05}
    {09}
*/

// FindMin returns the smallest element in the tree, or nil if the tree is empty.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) FindMin() (item *T) {
	if tt == nil {
		panic("binary_tree_ts: FindMin called on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMin()
}

// nlFindMin is FindMin without locking; the caller must hold the lock.
func (tt *BinaryTree[T]) nlFindMin() (item *T) {
	if tt.nlIsEmpty() {
		return nil
	}

	cur := tt.root
	for cur.left != nil {
		cur = cur.left
	}
	return cur.data
}

// FindMax returns the largest element in the tree, or nil if the tree is empty.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) FindMax() (item *T) {
	if tt == nil {
		panic("binary_tree_ts: FindMax called on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMax()
}

// nlFindMax is FindMax without locking; the caller must hold the lock.
func (tt *BinaryTree[T]) nlFindMax() (item *T) {
	if tt.nlIsEmpty() {
		return nil
	}

	cur := tt.root
	for cur.right != nil {
		cur = cur.right
	}
	return cur.data
}

// DeleteAtHead removes the smallest element of the tree, returning true if
// an element was removed.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("binary_tree_ts: DeleteAtHead called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}

	x := tt.nlFindMin()
	tt.deleteBy(func(data *T) int {
		return (*x).Compare(*data)
	})
	return true
}

// DeleteAtTail removes the largest element of the tree, returning true if
// an element was removed.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		panic("binary_tree_ts: DeleteAtTail called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}

	x := tt.nlFindMax()
	tt.deleteBy(func(data *T) int {
		return (*x).Compare(*data)
	})
	return true
}

// Reverse swaps the left and right children of every node in the tree.
// Complexity is O(n).
func (tt *BinaryTree[T]) Reverse() {
	if tt == nil {
		panic("binary_tree_ts: Reverse called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return
	}

	var postTraversal func(cur *BinaryTreeElement[T])
	postTraversal = func(cur *BinaryTreeElement[T]) {
		if cur == nil {
			return
		}
		if cur.left != nil {
			postTraversal(cur.left)
		}
		if cur.right != nil {
			postTraversal(cur.right)
		}
		cur.left, cur.right = cur.right, cur.left
	}
	postTraversal(tt.root)
}

// Index returns the `pos`-th element of the tree in in-order order,
// or nil if `pos` is out of range.
// Complexity is O(n).
func (tt *BinaryTree[T]) Index(pos int) (item *T) {
	if tt == nil {
		panic("binary_tree_ts: Index called on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if tt.nlIsEmpty() {
		return nil
	}

	if pos < 0 || pos >= tt.length {
		return nil
	}

	var n = 0
	var done = false
	var inorderTraversal func(cur *BinaryTreeElement[T])
	inorderTraversal = func(cur *BinaryTreeElement[T]) {
		if cur == nil {
			return
		}
		if !done {
			if cur.left != nil {
				inorderTraversal(cur.left)
			}
		}
		if n == pos {
			item = cur.data
			done = true
		}
		n++
		if !done {
			if cur.right != nil {
				inorderTraversal(cur.right)
			}
		}
	}
	inorderTraversal(tt.root)
	return
}

// Depth returns the number of levels in the deepest part of the tree.
// An empty tree has depth 0; a tree with only a root has depth 1.
// Complexity is O(n).
func (tt *BinaryTree[T]) Depth() int {
	if tt == nil {
		panic("binary_tree_ts: Depth called on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlDepth()
}

// nlDepth is Depth without locking; the caller must hold the lock.
func (tt *BinaryTree[T]) nlDepth() int {
	var depth func(cur *BinaryTreeElement[T]) int
	depth = func(cur *BinaryTreeElement[T]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depth(cur.left), depth(cur.right))
	}
	return depth(tt.root)
}

// ApplyFunction is the callback type used by the Walk* functions.  `pos` is
// the ordinal position of the element in the walk order, `depth` is the
// depth of the node in the tree (root is 0) and `userData` is the value
// passed to the walk.  Returning false stops the walk.
type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool

// WalkInOrder visits every element in in-order (ascending) order.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var inorderTraversal func(cur *BinaryTreeElement[T], n int)
	inorderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if b {
			if cur.left != nil {
				inorderTraversal(cur.left, n+1)
			}
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, cur.data, userData)
		p++
		// ----------------------------------------------------------------------
		if b {
			if cur.right != nil {
				inorderTraversal(cur.right, n+1)
			}
		}
	}
	inorderTraversal(tt.root, 0)
}

// WalkPreOrder visits every element in pre-order (node, left, right) order.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var preOrderTraversal func(cur *BinaryTreeElement[T], n int)
	preOrderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, cur.data, userData)
		p++
		// ----------------------------------------------------------------------
		if b {
			if cur.left != nil {
				preOrderTraversal(cur.left, n+1)
			}
		}
		if b {
			if cur.right != nil {
				preOrderTraversal(cur.right, n+1)
			}
		}
	}
	preOrderTraversal(tt.root, 0)
}

// WalkPostOrder visits every element in post-order (left, right, node) order.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var postOrderTraversal func(cur *BinaryTreeElement[T], n int)
	postOrderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if b {
			if cur.left != nil {
				postOrderTraversal(cur.left, n+1)
			}
		}
		if b {
			if cur.right != nil {
				postOrderTraversal(cur.right, n+1)
			}
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, cur.data, userData)
		p++
		// ----------------------------------------------------------------------
	}
	postOrderTraversal(tt.root, 0)
}

/* vim: set noai ts=4 sw=4: */
