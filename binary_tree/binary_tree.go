/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package binary_tree implements a generic, unbalanced binary search tree.
//
// It is a rework of github.com/pschlump/pluto/binary_tree in which the
// comparable.Comparable interface constraint has been replaced with plain
// Go type parameters.  Elements are stored and returned by value and
// ordering is a direct function call, so element data is never boxed into
// an interface and never unboxed with a type assertion.
//
// Trees of naturally ordered key types (all integers, floats and strings)
// are created with NewBinaryTree, which orders elements with the built-in
// < and > operators of the key type.  Trees of any other type — including
// structs ordered by a single field — are created with NewBinaryTreeFunc,
// which takes a caller supplied comparison function; the element type does
// not have to implement any interface.
//
// Basic operations on a Binary Tree:
//
//	Insert — create a new element in the tree; a duplicate replaces the existing element.
//	Delete — delete a specified element from the tree (elements can be found via Search).
//	DeleteMatch — delete using a caller supplied comparison function.
//	Search — return the given element from the tree.
//	Index — return the Nth element of the tree in in-order order.
//	IsEmpty — report whether the tree is empty.
//	Len / Length — number of elements in the tree; 0 is an empty tree.
//	Reverse — swap the left and right children of every node in the tree.
//	Truncate — delete all the nodes in the tree.
//	FindMin / FindMax — return the smallest / largest element in the tree.
//	DeleteAtHead — delete the smallest element of the tree.
//	DeleteAtTail — delete the largest element of the tree.
//	Depth — number of levels in the deepest part of the tree.
//	WalkInOrder / WalkPreOrder / WalkPostOrder — callback-based traversals.
//	WalkFunc — apply a function to every element in pre-order.
//	Front — old-style in-order iterator.
//	All / Backward — range-over-func iterators (in-order and reverse in-order).
//
// Insert, Delete and Search are O(log₂ n) on average for randomly ordered
// input and O(n) in the worst case (the tree is NOT self-balancing).
//
// A BinaryTree is created with NewBinaryTree or NewBinaryTreeFunc.  A nil
// *BinaryTree and the zero value both behave as an empty tree for every
// operation except Insert: Search finds nothing, Delete, DeleteMatch,
// DeleteAtHead and DeleteAtTail return false, FindMin, FindMax and Index
// report not-found, Len and Depth are 0, and the walks and iterators
// visit nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewBinaryTreeFunc(nil)         — nil comparison function, caught at construction.
//	Insert on a nil tree           — a nil tree cannot store an element.
//	Insert on a zero-value tree    — no comparison function; the message names the constructors.
//
// BinaryTree is not safe for concurrent use.
package binary_tree

import (
	"cmp"
	"fmt"
	"io"
	"strings"
)

// BinaryTreeElement is a single node of a BinaryTree.
type BinaryTreeElement[T any] struct {
	data        T
	left, right *BinaryTreeElement[T]
}

// BinaryTree is a generic binary tree.  Use NewBinaryTree for naturally
// ordered key types (numbers, strings) or NewBinaryTreeFunc for a caller
// supplied comparison function.  The zero value is an empty tree.
type BinaryTree[T any] struct {
	root   *BinaryTreeElement[T]
	length int

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// -------------------------------------------------------------------------------------------------------

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewBinaryTree and
// is handy for building custom comparison functions.
func Compare[T cmp.Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	default:
		return 0
	}
}

// NewBinaryTree creates a new BinaryTree for any naturally ordered key
// type (all integers, floats and strings — cmp.Ordered).  Ordering uses
// the built-in < and > operators of T; no interface and no boxing is
// involved.
// Complexity is O(1).
func NewBinaryTree[T cmp.Ordered]() *BinaryTree[T] {
	return &BinaryTree[T]{cmp: Compare[T]}
}

// NewBinaryTreeFunc creates a new BinaryTree that orders elements with the
// caller supplied comparison function fx.  fx must return a negative value
// if a sorts before b, 0 if the two are duplicates and a positive value if
// a sorts after b, and must order elements consistently.  This lets any
// type — for example a struct ordered by one of its fields — be stored
// without implementing any interface.
// Complexity is O(1).
func NewBinaryTreeFunc[T any](fx func(a, b T) int) *BinaryTree[T] {
	if fx == nil {
		panic("binary_tree: NewBinaryTreeFunc called with a nil comparison function")
	}
	return &BinaryTree[T]{cmp: fx}
}

// compare orders a and b, guarding against a zero-value tree that was not
// created by one of the constructors.
func (tt *BinaryTree[T]) compare(a, b T) int {
	if tt.cmp == nil {
		panic("binary_tree: no comparison function (create the tree with NewBinaryTree or NewBinaryTreeFunc)")
	}
	return tt.cmp(a, b)
}

// -------------------------------------------------------------------------------------------------------

// GetData returns the data stored in this element.
// Complexity is O(1).
func (ee *BinaryTreeElement[T]) GetData() T {
	return ee.data
}

// SetData replaces the data stored in this element.  Calling it on a node
// that is inside a tree can break the tree's ordering invariant; it is
// intended for standalone elements.
// Complexity is O(1).
func (ee *BinaryTreeElement[T]) SetData(x T) {
	ee.data = x
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty returns true if the tree is empty.
// Complexity is O(1).
func (tt *BinaryTree[T]) IsEmpty() bool {
	return tt == nil || tt.root == nil
}

// Truncate removes all data from the tree.  The comparison function is
// kept, so the tree remains usable and can simply be refilled.
// Complexity is O(1).
func (tt *BinaryTree[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.root = nil
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one and false is
// returned; true is returned when a new node was added.
// Insert panics on a nil tree or on a zero-value tree (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty tree.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) Insert(item T) (vv bool) {
	if tt == nil {
		panic("binary_tree: Insert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("binary_tree: Insert called on a tree with no comparison function (create the tree with NewBinaryTree or NewBinaryTreeFunc)")
	}
	node := &BinaryTreeElement[T]{data: item}
	if tt.IsEmpty() {
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
		} else if c := tt.cmp(item, (*root).data); c == 0 {
			// Duplicate: replace the stored value in place, keeping the node
			// (and its children) so the tree shape does not change.
			(*root).data = item
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
	if tt == nil {
		return 0
	}
	return tt.length
}

// Length is an alias for Len; it returns the number of elements in the tree.
// Complexity is O(1).
func (tt *BinaryTree[T]) Length() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Search will walk the tree looking for `find` and return the found item
// from the tree.  If it is not found then false is returned.  `find` only
// needs the fields that the tree's comparison function reads: a probe with
// just the key fields set finds the element with the full data.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) Search(find T) (item T, found bool) {
	if tt == nil {
		return
	}

	cur := tt.root
	for cur != nil {
		c := tt.compare(find, cur.data)
		if c == 0 {
			return cur.data, true
		}
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	return
}

// Dump writes one line per element to `fo`: an in-order traversal indented
// by depth, including the left/right child pointers.  It is a debugging
// aid; use All, Backward or the Walk* functions to process the data.
// Complexity is O(n).
func (tt *BinaryTree[T]) Dump(fo io.Writer) {
	if tt == nil {
		return
	}
	k := tt.Depth() * 4
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
			strings.Repeat(" ", 4*n), cur.data, strings.Repeat(" ", k-(4*n)),
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
// if an element was found and removed.  As with Search, `find` only needs
// the fields that the comparison function reads.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) Delete(find T) (found bool) {
	if tt == nil {
		return false
	}
	return tt.deleteBy(func(data T) int {
		return tt.compare(find, data)
	})
}

// DeleteMatch is like Delete but uses the caller supplied comparison
// function `fx` (with the same contract as the tree's comparison function)
// to find the element to remove.
func (tt *BinaryTree[T]) DeleteMatch(find T, fx func(a, b T) int) (found bool) {
	if tt == nil {
		return false
	}
	return tt.deleteBy(func(data T) int {
		return fx(find, data)
	})
}

// deleteBy removes the node for which cmp(data) == 0, using cmp to steer the
// descent through the tree.
func (tt *BinaryTree[T]) deleteBy(cmp func(data T) int) (found bool) {
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

// FindMin returns the smallest element in the tree, or false if the tree is empty.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) FindMin() (item T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}

	cur := tt.root
	for cur.left != nil {
		cur = cur.left
	}
	return cur.data, true
}

// FindMax returns the largest element in the tree, or false if the tree is empty.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) FindMax() (item T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}

	cur := tt.root
	for cur.right != nil {
		cur = cur.right
	}
	return cur.data, true
}

// DeleteAtHead removes the smallest element of the tree, returning true if
// an element was removed.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) DeleteAtHead() (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	tt.length--
	// Splice out the left-most node; by construction it has no left child.
	cur := &tt.root
	for (*cur).left != nil {
		cur = &(*cur).left
	}
	*cur = (*cur).right
	return true
}

// DeleteAtTail removes the largest element of the tree, returning true if
// an element was removed.
// Complexity is O(log₂ n) on average, O(n) in the worst case.
func (tt *BinaryTree[T]) DeleteAtTail() (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	tt.length--
	// Splice out the right-most node; by construction it has no right child.
	cur := &tt.root
	for (*cur).right != nil {
		cur = &(*cur).right
	}
	*cur = (*cur).left
	return true
}

// Reverse swaps the left and right children of every node in the tree.
// The result is no longer ordered by the tree's comparison function until
// it is reversed back.
// Complexity is O(n).
func (tt *BinaryTree[T]) Reverse() {
	if tt == nil || tt.IsEmpty() {
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

// Index returns the `pos`-th element of the tree in in-order order, or
// false if `pos` is out of range.
// Complexity is O(n).
func (tt *BinaryTree[T]) Index(pos int) (item T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}

	if pos < 0 || pos >= tt.length {
		return
	}

	var n = 0
	var inorderTraversal func(cur *BinaryTreeElement[T])
	inorderTraversal = func(cur *BinaryTreeElement[T]) {
		if cur == nil {
			return
		}
		if !found {
			if cur.left != nil {
				inorderTraversal(cur.left)
			}
		}
		if n == pos {
			item = cur.data
			found = true
		}
		n++
		if !found {
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
		return 0
	}

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
// the ordinal position of the element in the walk order and `depth` is the
// depth of the node in the tree (root is 0).  Returning false stops the
// walk.  Caller state that the pluto version of this package passed as an
// interface{} userData parameter is captured in a closure instead, so it
// keeps its static type and is never boxed.
type ApplyFunction[T any] func(pos, depth int, data T) bool

// WalkInOrder visits every element in in-order (ascending) order.
// Returning false from fx stops the walk.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkInOrder(fx ApplyFunction[T]) {
	if tt == nil {
		return
	}

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
		b = b && fx(p, n, cur.data)
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
// Returning false from fx stops the walk.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkPreOrder(fx ApplyFunction[T]) {
	if tt == nil {
		return
	}

	p := 0
	b := true
	var preOrderTraversal func(cur *BinaryTreeElement[T], n int)
	preOrderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, cur.data)
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
// Returning false from fx stops the walk.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkPostOrder(fx ApplyFunction[T]) {
	if tt == nil {
		return
	}

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
		b = b && fx(p, n, cur.data)
		p++
		// ----------------------------------------------------------------------
	}
	postOrderTraversal(tt.root, 0)
}
