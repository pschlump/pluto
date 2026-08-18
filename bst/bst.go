package bst

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

/*
Package bst implements an ordered collection built on an unbalanced binary
search tree.  The operations provided are classic binary-search-tree
operations:

  - Insert   — add an item; an item that Compare-equal to an existing one replaces it.	O(log₂ n) average
  - Search   — find an item; returns nil if not present.								O(log₂ n) average
  - Delete   — remove an item; returns false if not present.							O(log₂ n) average
  - IsEmpty  — true if the tree has no nodes.											O(1)
  - Length   — number of nodes in the tree.												O(1)
  - FindMin / FindMax — smallest / largest item.										O(log₂ n) average
  - DeleteAtHead / DeleteAtTail — remove smallest / largest item.						O(log₂ n) average
  - Depth    — number of nodes on the longest root-to-leaf path.						O(n)
  - Index    — item at in-order position pos.											O(n)
  - Reverse  — mirror the tree (swap left/right at every node).							O(n)
  - Truncate — remove all nodes.														O(1)
  - WalkInOrder / WalkPreOrder / WalkPostOrder — callback traversal.					O(n)
  - All / Backward — Go 1.23+ range-over-func iterators (see iter.go).					O(n)

The tree is not balanced, so the average-case O(log₂ n) operations degrade to
O(n) when items are inserted in sorted order.
*/

import (
	"fmt"
	"io"
	"strings"

	"github.com/pschlump/pluto/comparable"
)

// BinarySearchTreeNode is a single node of the tree.  It holds the item
// data and the left/right child links.
type BinarySearchTreeNode[T comparable.Comparable] struct {
	data        *T
	left, right *BinarySearchTreeNode[T]
}

// BinarySearchTree is a generic unbalanced binary search tree of items
// that implement comparable.Comparable.
type BinarySearchTree[T comparable.Comparable] struct {
	root   *BinarySearchTreeNode[T]
	length int // Number of nodes in the tree
}

// IsEmpty will return true if the tree is empty.
func (tt BinarySearchTree[T]) IsEmpty() bool {
	return tt.root == nil
}

// Truncate removes all data from the tree.
func (tt *BinarySearchTree[T]) Truncate() {
	tt.root = nil
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an existing
// item the new item will replace the existing one.
func (tt *BinarySearchTree[T]) Insert(item T) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		tt.root = &BinarySearchTreeNode[T]{data: &item}
		tt.length = 1
		return
	}

	// Simple is recursive, can be replaced with an iterative tree traversal.
	var insert func(root **BinarySearchTreeNode[T])
	insert = func(root **BinarySearchTreeNode[T]) {
		if *root == nil {
			*root = &BinarySearchTreeNode[T]{data: &item}
			tt.length++
		} else if c := item.Compare(*(*root).data); c == 0 {
			// Duplicate: replace the stored item, keep the children.
			(*root).data = &item
		} else if c < 0 {
			insert(&(*root).left)
		} else {
			insert(&(*root).right)
		}
	}

	insert(&tt.root)
}

// Length returns the number of elements in the list.
func (tt *BinarySearchTree[T]) Length() int {
	return tt.length
}

// Search will walk the tree looking for `find` and return the found item
// if it is in the tree. If it is not found then `nil` will be returned.
func (tt *BinarySearchTree[T]) Search(find T) (item *T) {
	if tt == nil {
		panic("tree should not be nil")
	}

	cur := tt.root
	for cur != nil {
		c := find.Compare(*cur.data)
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

// Dump will print out the tree to the file `fo`.
func (tt *BinarySearchTree[T]) Dump(fo io.Writer) {
	var inorderTraversal func(cur *BinarySearchTreeNode[T], n int)
	inorderTraversal = func(cur *BinarySearchTreeNode[T], n int) {
		if cur == nil {
			return
		}
		if cur.left != nil {
			inorderTraversal(cur.left, n+1)
		}
		pad := max(20-4*n, 1)
		_, _ = fmt.Fprintf(fo, "%s%v%s (left=%p/%p, right=%p/%p) self=%p\n",
			strings.Repeat(" ", 4*n), *cur.data, strings.Repeat(" ", pad),
			cur.left, &cur.left, cur.right, &cur.right, cur)
		if cur.right != nil {
			inorderTraversal(cur.right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

// Delete removes the item Compare-equal to `find` from the tree.  It returns
// true if an item was found and removed, false otherwise.
func (tt *BinarySearchTree[T]) Delete(find T) (found bool) {
	if tt == nil {
		panic("tree should not be nil")
	}

	// cur is a pointer to the link (root or a child field) that refers to
	// the node currently being examined.
	cur := &tt.root
	for *cur != nil {
		c := find.Compare(*(*cur).data)
		if c == 0 {
			tt.length--
			switch {
			case (*cur).left == nil && (*cur).right == nil:
				*cur = nil // just delete the node, it has no children.
			case (*cur).left != nil && (*cur).right == nil:
				*cur = (*cur).left // Has only left child, promote it.
			case (*cur).left == nil && (*cur).right != nil:
				*cur = (*cur).right // Has only right child, promote it.
			default: // has both children.
				// Find the in-order successor: the left-most node of the
				// right sub-tree.  It has no left child by construction.
				successor := &(*cur).right
				for (*successor).left != nil {
					successor = &(*successor).left
				}
				(*cur).data = (*successor).data // promote successor's data.
				*successor = (*successor).right // unlink successor, keeping its right sub-tree.
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

// FindMin returns a pointer to the smallest item in the tree, or nil if the
// tree is empty.
func (tt *BinarySearchTree[T]) FindMin() (item *T) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return nil
	}

	cur := tt.root
	for cur.left != nil {
		cur = cur.left
	}
	return cur.data
}

// FindMax returns a pointer to the largest item in the tree, or nil if the
// tree is empty.
func (tt *BinarySearchTree[T]) FindMax() (item *T) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return nil
	}

	cur := tt.root
	for cur.right != nil {
		cur = cur.right
	}
	return cur.data
}

// DeleteAtHead removes the smallest item in the tree.  It returns false if
// the tree is empty.
func (tt *BinarySearchTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return false
	}

	x := tt.FindMin()
	return tt.Delete(*x)
}

// DeleteAtTail removes the largest item in the tree.  It returns false if
// the tree is empty.
func (tt *BinarySearchTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return false
	}

	x := tt.FindMax()
	return tt.Delete(*x)
}

// Reverse mirrors the tree in place by swapping the left and right children
// of every node.  After Reverse, an in-order traversal yields items in
// descending order.
func (tt *BinarySearchTree[T]) Reverse() {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return
	}

	var postTraversal func(cur *BinarySearchTreeNode[T])
	postTraversal = func(cur *BinarySearchTreeNode[T]) {
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

// Index returns the item at in-order position `pos` (0-based), or nil if pos
// is out of range or the tree is empty.
func (tt *BinarySearchTree[T]) Index(pos int) (item *T) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return nil
	}

	if pos < 0 || pos >= tt.length {
		return nil
	}

	var n = 0
	var done = false
	var inorderTraversal func(cur *BinarySearchTreeNode[T])
	inorderTraversal = func(cur *BinarySearchTreeNode[T]) {
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

// Depth returns the number of nodes on the longest path from the root to a
// leaf.  An empty tree has depth 0, a tree with a single node has depth 1.
func (tt *BinarySearchTree[T]) Depth() (d int) {
	if tt == nil {
		panic("tree should not be nil")
	}
	if tt.IsEmpty() {
		return 0
	}

	var depthOf func(cur *BinarySearchTreeNode[T]) int
	depthOf = func(cur *BinarySearchTreeNode[T]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depthOf(cur.left), depthOf(cur.right))
	}
	return depthOf(tt.root)
}

// ApplyFunction is the callback type used by the Walk* traversal functions.
// pos is the 0-based visit position, depth is the depth of the node in the
// tree (root is 0), data is the item stored in the node, and userData is the
// value passed to the Walk* call.  Return true to continue the walk, false
// to stop it early.
type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool

// WalkInOrder visits every node in ascending (in-order) sequence, calling fx
// for each one.  The walk stops early if fx returns false.
func (tt *BinarySearchTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {

	p := 0
	b := true
	var inorderTraversal func(cur *BinarySearchTreeNode[T], n int)
	inorderTraversal = func(cur *BinarySearchTreeNode[T], n int) {
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

// WalkPreOrder visits every node in pre-order (node, left, right), calling fx
// for each one.  The walk stops early if fx returns false.
func (tt *BinarySearchTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {

	p := 0
	b := true
	var preOrderTraversal func(cur *BinarySearchTreeNode[T], n int)
	preOrderTraversal = func(cur *BinarySearchTreeNode[T], n int) {
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

// WalkPostOrder visits every node in post-order (left, right, node), calling
// fx for each one.  The walk stops early if fx returns false.
func (tt *BinarySearchTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {

	p := 0
	b := true
	var postOrderTraversal func(cur *BinarySearchTreeNode[T], n int)
	postOrderTraversal = func(cur *BinarySearchTreeNode[T], n int) {
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
