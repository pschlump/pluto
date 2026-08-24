// Package rb_tree_ts implements a generic red-black self-balancing binary
// search tree that is safe for concurrent use.  All operations are guarded
// by an internal sync.RWMutex.
//
// A red-black tree keeps every root-to-leaf path within a factor of two of
// the shortest one, so Insert, Delete and Search are all O(log2(n)) in the
// worst case.  Basic operations:
//
//	Insert - add a new element to the tree (duplicates replace the old data).	O(log2(n))
//	Delete - remove an element from the tree.					O(log2(n))
//	Search - return the stored element equal to the probe, or nil.			O(log2(n))
//	FindMin - return the smallest element in the tree.				O(log2(n))
//	FindMax - return the largest element in the tree.				O(log2(n))
//	DeleteAtHead - remove the smallest element (Delete(FindMin())).			O(log2(n))
//	DeleteAtTail - remove the largest element (Delete(FindMax())).			O(log2(n))
//	IsEmpty - report whether the tree is empty.					O(1)
//	Length - return the number of elements in the tree.				O(1)
//	Depth - return the height of the tree (longest root-to-leaf path).		O(n)
//	Truncate - remove all elements from the tree.					O(1)
//	All/Backward - Go 1.23 range-over-func iterators over a snapshot of the tree.	O(n)
//
// This is the thread-safe version of github.com/pschlump/pluto/rb_tree; both
// packages expose the identical API.  The iterators (All/Backward) operate
// on a consistent snapshot taken under the read lock, so the loop body never
// runs while the lock is held.
//
// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package rb_tree_ts

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pschlump/pluto/comparable"
)

// RbTreeNode is a single node of the tree.  It holds the item data, the
// left/right/parent links and the node color.  A nil child is treated as a
// black leaf.
type RbTreeNode[T comparable.Comparable] struct {
	data        *T
	red         bool // Red nodes may not have red children; nil children are black.
	left, right *RbTreeNode[T]
	parent      *RbTreeNode[T]
}

// RbTree is a generic red-black self-balancing binary search tree of items
// that implement comparable.Comparable.  It is safe for concurrent use.  The
// zero value is ready to use.
type RbTree[T comparable.Comparable] struct {
	root   *RbTreeNode[T]
	length int // Number of nodes in the tree
	lock   sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------
// Lock-free internals; the caller must hold the appropriate lock.
// -------------------------------------------------------------------------------------------------------

// isRed reports whether n is a red node; nil nodes are black.
func isRed[T comparable.Comparable](n *RbTreeNode[T]) bool {
	return n != nil && n.red
}

// rotateLeft pivots the subtree rooted at x to the left, making x's right
// child the new subtree root.
func (tt *RbTree[T]) rotateLeft(x *RbTreeNode[T]) {
	y := x.right
	x.right = y.left
	if y.left != nil {
		y.left.parent = x
	}
	y.parent = x.parent
	if x.parent == nil {
		tt.root = y
	} else if x == x.parent.left {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.left = x
	x.parent = y
}

// rotateRight pivots the subtree rooted at x to the right, making x's left
// child the new subtree root.
func (tt *RbTree[T]) rotateRight(x *RbTreeNode[T]) {
	y := x.left
	x.left = y.right
	if y.right != nil {
		y.right.parent = x
	}
	y.parent = x.parent
	if x.parent == nil {
		tt.root = y
	} else if x == x.parent.right {
		x.parent.right = y
	} else {
		x.parent.left = y
	}
	y.right = x
	x.parent = y
}

// findNode returns the node holding the item that Compare-equal to `item`,
// or nil if it is not present.
func (tt *RbTree[T]) findNode(item T) *RbTreeNode[T] {
	cur := tt.root
	for cur != nil {
		c := item.Compare(*cur.data)
		if c == 0 {
			return cur
		}
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	return nil
}

// minNode returns the node holding the smallest item in the subtree rooted
// at n.
func minNode[T comparable.Comparable](n *RbTreeNode[T]) *RbTreeNode[T] {
	for n != nil && n.left != nil {
		n = n.left
	}
	return n
}

// maxNode returns the node holding the largest item in the subtree rooted
// at n.
func maxNode[T comparable.Comparable](n *RbTreeNode[T]) *RbTreeNode[T] {
	for n != nil && n.right != nil {
		n = n.right
	}
	return n
}

// insertFixup restores the red-black properties after inserting the red node
// z, recoloring and rotating until no red node has a red parent.
func (tt *RbTree[T]) insertFixup(z *RbTreeNode[T]) {
	for isRed(z.parent) {
		if z.parent == z.parent.parent.left {
			y := z.parent.parent.right // The uncle.
			if isRed(y) {
				z.parent.red = false
				y.red = false
				z.parent.parent.red = true
				z = z.parent.parent
			} else {
				if z == z.parent.right {
					z = z.parent
					tt.rotateLeft(z)
				}
				z.parent.red = false
				z.parent.parent.red = true
				tt.rotateRight(z.parent.parent)
			}
		} else {
			y := z.parent.parent.left // The uncle.
			if isRed(y) {
				z.parent.red = false
				y.red = false
				z.parent.parent.red = true
				z = z.parent.parent
			} else {
				if z == z.parent.left {
					z = z.parent
					tt.rotateRight(z)
				}
				z.parent.red = false
				z.parent.parent.red = true
				tt.rotateLeft(z.parent.parent)
			}
		}
	}
	tt.root.red = false
}

// transplant replaces the subtree rooted at u with the subtree rooted at v.
func (tt *RbTree[T]) transplant(u, v *RbTreeNode[T]) {
	if u.parent == nil {
		tt.root = v
	} else if u == u.parent.left {
		u.parent.left = v
	} else {
		u.parent.right = v
	}
	if v != nil {
		v.parent = u.parent
	}
}

// deleteFixup restores the red-black properties after splicing out a black
// node.  x is the node that moved into the spliced node's position (possibly
// nil); xParent is x's parent, which must be supplied separately because x
// may be nil.
func (tt *RbTree[T]) deleteFixup(x, xParent *RbTreeNode[T]) {
	for x != tt.root && !isRed(x) {
		if x == xParent.left {
			w := xParent.right // The sibling; never nil here.
			if isRed(w) {
				w.red = false
				xParent.red = true
				tt.rotateLeft(xParent)
				w = xParent.right
			}
			if !isRed(w.left) && !isRed(w.right) {
				w.red = true
				x = xParent
				xParent = x.parent
			} else {
				if !isRed(w.right) {
					w.left.red = false
					w.red = true
					tt.rotateRight(w)
					w = xParent.right
				}
				w.red = xParent.red
				xParent.red = false
				w.right.red = false
				tt.rotateLeft(xParent)
				x = tt.root
				xParent = nil
			}
		} else {
			w := xParent.left // The sibling; never nil here.
			if isRed(w) {
				w.red = false
				xParent.red = true
				tt.rotateRight(xParent)
				w = xParent.left
			}
			if !isRed(w.right) && !isRed(w.left) {
				w.red = true
				x = xParent
				xParent = x.parent
			} else {
				if !isRed(w.left) {
					w.right.red = false
					w.red = true
					tt.rotateLeft(w)
					w = xParent.left
				}
				w.red = xParent.red
				xParent.red = false
				w.left.red = false
				tt.rotateRight(xParent)
				x = tt.root
				xParent = nil
			}
		}
	}
	if x != nil {
		x.red = false
	}
}

// deleteLocked is the lock-free body of Delete; the caller must hold the
// write lock.
func (tt *RbTree[T]) deleteLocked(item T) (found bool) {
	z := tt.findNode(item)
	if z == nil {
		return false
	}
	tt.length--

	y := z                        // The node actually spliced out of the tree.
	yWasRed := y.red              // Its color before the splice.
	var x, xParent *RbTreeNode[T] // x moves into y's old position.

	if z.left == nil {
		x = z.right
		xParent = z.parent
		tt.transplant(z, z.right)
	} else if z.right == nil {
		x = z.left
		xParent = z.parent
		tt.transplant(z, z.left)
	} else {
		y = minNode(z.right)
		yWasRed = y.red
		x = y.right
		if y.parent == z {
			xParent = y
			if x != nil {
				x.parent = y
			}
		} else {
			xParent = y.parent
			tt.transplant(y, y.right)
			y.right = z.right
			y.right.parent = y
		}
		tt.transplant(z, y)
		y.left = z.left
		y.left.parent = y
		y.red = z.red
	}
	if !yWasRed {
		tt.deleteFixup(x, xParent)
	}
	return true
}

// -------------------------------------------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the tree is empty.
// Complexity is O(1).
func (tt *RbTree[T]) IsEmpty() bool {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.root == nil
}

// Length will return the number of elements in the tree.
// Complexity is O(1).
func (tt *RbTree[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (tt *RbTree[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.root = nil
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) Insert(item T) {
	if tt == nil {
		panic("tree should not be nil")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()

	// Standard BST insert; the new node starts out red.
	var parent *RbTreeNode[T]
	cur := tt.root
	for cur != nil {
		c := item.Compare(*cur.data)
		if c == 0 {
			cur.data = &item // Duplicate: replace the stored item.
			return
		}
		parent = cur
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	z := &RbTreeNode[T]{data: &item, red: true, parent: parent}
	if parent == nil {
		tt.root = z
	} else if item.Compare(*parent.data) < 0 {
		parent.left = z
	} else {
		parent.right = z
	}
	tt.length++
	tt.insertFixup(z)
}

// Search will return a copy of the item in the tree that Compare-equal to
// `item`, or nil if it is not present.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) Search(item T) (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if n := tt.findNode(item); n != nil {
		cp := *n.data
		return &cp
	}
	return nil
}

// Delete will remove the item in the tree that Compare-equal to `item`.  It
// returns false if the item is not present.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) Delete(item T) (found bool) {
	if tt == nil {
		panic("tree should not be nil")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.deleteLocked(item)
}

// FindMin returns a copy of the smallest item in the tree, or nil if it is
// empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) FindMin() (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if n := minNode(tt.root); n != nil {
		cp := *n.data
		return &cp
	}
	return nil
}

// FindMax returns a copy of the largest item in the tree, or nil if it is
// empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) FindMax() (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if n := maxNode(tt.root); n != nil {
		cp := *n.data
		return &cp
	}
	return nil
}

// DeleteAtHead removes the smallest item in the tree.  It returns false if
// the tree is empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("tree should not be nil")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	if tt.root == nil {
		return false
	}
	return tt.deleteLocked(*minNode(tt.root).data)
}

// DeleteAtTail removes the largest item in the tree.  It returns false if
// the tree is empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		panic("tree should not be nil")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	if tt.root == nil {
		return false
	}
	return tt.deleteLocked(*maxNode(tt.root).data)
}

// Depth returns the number of nodes on the longest path from the root to a
// leaf.  An empty tree has depth 0, a tree with a single node has depth 1.
// Balancing keeps this at O(log₂ n).
func (tt *RbTree[T]) Depth() (d int) {
	if tt == nil {
		panic("tree should not be nil")
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var depthOf func(cur *RbTreeNode[T]) int
	depthOf = func(cur *RbTreeNode[T]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depthOf(cur.left), depthOf(cur.right))
	}
	return depthOf(tt.root)
}

// toSlice returns a snapshot of the tree data in ascending (in-order)
// sequence.
// Complexity is O(n).
func (tt *RbTree[T]) toSlice() (items []T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var inorder func(cur *RbTreeNode[T])
	inorder = func(cur *RbTreeNode[T]) {
		if cur == nil {
			return
		}
		inorder(cur.left)
		items = append(items, *cur.data)
		inorder(cur.right)
	}
	inorder(tt.root)
	return
}

// Dump writes an indented picture of the tree to `w` for debugging.  Each
// line shows a node as "data(R)" for red and "data(B)" for black; the left
// subtree is printed above the right subtree.
// Complexity is O(n).
func (tt *RbTree[T]) Dump(w io.Writer) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.root == nil {
		fmt.Fprintf(w, "RbTree (empty)\n")
		return
	}
	var depthOf func(cur *RbTreeNode[T]) int
	depthOf = func(cur *RbTreeNode[T]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depthOf(cur.left), depthOf(cur.right))
	}
	fmt.Fprintf(w, "RbTree length=%d depth=%d\n", tt.length, depthOf(tt.root))
	var dump func(cur *RbTreeNode[T], depth int)
	dump = func(cur *RbTreeNode[T], depth int) {
		if cur == nil {
			return
		}
		dump(cur.left, depth+1)
		c := "B"
		if cur.red {
			c = "R"
		}
		fmt.Fprintf(w, "%s%v(%s)\n", strings.Repeat("  ", depth), *cur.data, c)
		dump(cur.right, depth+1)
	}
	dump(tt.root, 0)
}
