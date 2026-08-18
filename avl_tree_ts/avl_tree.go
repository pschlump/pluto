// Package avl_tree_ts implements a generic AVL self-balancing binary search
// tree that is safe for concurrent use.  All operations are guarded by an
// internal sync.RWMutex.
//
// An AVL tree keeps the heights of the two child subtrees of every node
// within one of each other, so Insert, Delete and Search are all O(log n).
//
// Basic operations:
//
//	Insert - add a new element to the tree (duplicates replace the old data).	O(log2(n))
//	Delete - remove an element from the tree.					O(log2(n))
//	Search - return the stored element equal to the probe, or nil.			O(log2(n))
//	FindMin - return the smallest element in the tree.				O(log2(n))
//	FindMax - return the largest element in the tree.				O(log2(n))
//	DeleteAtHead - remove the smallest element (Delete(FindMin())).			O(log2(n))
//	DeleteAtTail - remove the largest element (Delete(FindMax())).			O(log2(n))
//	Index - return the Nth element in in-order sequence.				O(n)
//	IsEmpty - report whether the tree is empty.					O(1)
//	Length - return the number of elements in the tree.				O(1)
//	Depth - return the height of the tree (longest root-to-leaf path).		O(1)
//	Truncate - remove all elements from the tree.					O(1)
//	Reverse - mirror the tree (swaps ordering; mainly useful for testing).		O(n)
//	WalkInOrder/WalkPreOrder/WalkPostOrder - apply a function to every node.	O(n)
//	Copy/Union/Minus/Intersect - whole-tree set operations.				O(n log2(n))
//	All/Backward - Go 1.23 range-over-func iterators over a snapshot of the tree.	O(n)
//
// This is the thread-safe version of github.com/pschlump/pluto/avl_tree; both
// packages expose the identical API.  Note that the Walk* callbacks are invoked
// while holding a read lock, so they must not call back into the same tree.
// The iterators (Front/All/Backward) operate on a consistent snapshot taken
// under the read lock.
//
// Copyright (C) Philip Schlump, 2012-2021.
// BSD 3 Clause Licensed.
package avl_tree_ts

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/g_lib"
)

// AvlTreeElement is a node of an AvlTree.
type AvlTreeElement[T comparable.Comparable] struct {
	data        *T
	height      int
	left, right *AvlTreeElement[T]
}

// AvlTree is a generic AVL balanced binary tree that is balanced using the
// AVL rotation system.  It is safe for concurrent use.
type AvlTree[T comparable.Comparable] struct {
	root   *AvlTreeElement[T]
	length int
	lock   sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// NewAvlTreeElement creates a new tree node holding `x`.
// Complexity is O(1).
func NewAvlTreeElement[T comparable.Comparable](x *T) *AvlTreeElement[T] {
	return &AvlTreeElement[T]{
		data:   x,
		height: 1,
	}
}

// Height returns the saved height of the node `e` (0 for a nil node).  The
// height is re-calculated as the tree is modified.
// Complexity is O(1).
func (tt *AvlTree[T]) Height(e *AvlTreeElement[T]) int {
	if e == nil {
		return 0
	}
	return e.height
}

// calcAvlBalance returns the difference in height between the left and right
// subtrees of `e`.  When the absolute value exceeds 1 the subtree is rotated
// to restore balance.
// Complexity is O(1).
func (tt *AvlTree[T]) calcAvlBalance(e *AvlTreeElement[T]) int {
	if e == nil {
		return 0
	}
	return tt.Height(e.left) - tt.Height(e.right)
}

// NewAvlTree creates a new empty AvlTree and returns it.
// Complexity is O(1).
func NewAvlTree[T comparable.Comparable]() *AvlTree[T] {
	return &AvlTree[T]{}
}

// GetData returns the user data from the AVL tree node.
// Complexity is O(1).
func (ee *AvlTreeElement[T]) GetData() *T {
	return ee.data
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty returns true if the tree is empty.
// Complexity is O(1).
func (tt *AvlTree[T]) IsEmpty() bool {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlIsEmpty()
}

// nlIsEmpty is the no-lock internal version of IsEmpty.
func (tt *AvlTree[T]) nlIsEmpty() bool {
	return tt.root == nil
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (tt *AvlTree[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
}

// nlTruncate is the no-lock internal version of Truncate.
func (tt *AvlTree[T]) nlTruncate() {
	tt.root = nil
	tt.length = 0
}

/*

Rotations used to rebalance the tree after Insert and Delete.

Let the newly inserted (or deleted) node be w.
1) Perform the standard BST insert/delete for w.
2) Starting from w, travel up and find the first unbalanced node.  Let z be
   the first unbalanced node, y the child of z on the path from w to z, and x
   the grandchild of z on the path from w to z.
3) Re-balance the tree by performing the appropriate rotation on the subtree
   rooted with z.  There are 4 possible cases:

	a) y is left child of z and x is left child of y (Left Left Case)
	b) y is left child of z and x is right child of y (Left Right Case)
	c) y is right child of z and x is right child of y (Right Right Case)
	d) y is right child of z and x is left child of y (Right Left Case)

a) Left Left Case

T1, T2, T3 and T4 are subtrees.

	     z                                      y
	    / \                                   /   \
	   y   T4      Right Rotate (z)          x      z
	  / \          - - - - - - - - ->      /  \    /  \
	 x   T3                               T1  T2  T3  T4
	/ \
	T1   T2

b) Left Right Case

	 z                               z                           x
	/ \                            /   \                        /  \
	y   T4  Left Rotate (y)        x    T4  Right Rotate(z)    y      z
	/ \      - - - - - - - - ->    /  \      - - - - - - - ->  / \    / \
	T1   x                          y    T3                    T1  T2 T3  T4
	    / \                        / \
	  T2   T3                    T1   T2

c) Right Right Case

	z                                y
	 /  \                            /   \
	T1   y     Left Rotate(z)       z      x
	    /  \   - - - - - - - ->    / \    / \
	   T2   x                     T1  T2 T3  T4
	       / \
	     T3  T4

d) Right Left Case

	   z                            z                            x
	  / \                          / \                          /  \
	T1   y   Right Rotate (y)    T1   x      Left Rotate(z)   z      y
	    / \  - - - - - - - - ->     /  \   - - - - - - - ->  / \    / \
	   x   T4                      T2   y                  T1  T2 T3  T4
	  / \                              /  \
	T2   T3                           T3   T4
*/

// rotateRight performs a right rotation about z and returns the new subtree
// root (the old left child of z).
func (tt *AvlTree[T]) rotateRight(z *AvlTreeElement[T]) *AvlTreeElement[T] {
	y := z.left
	z.left = y.right
	y.right = z
	z.height = g_lib.Max(tt.Height(z.left), tt.Height(z.right)) + 1
	y.height = g_lib.Max(tt.Height(y.left), tt.Height(y.right)) + 1
	return y
}

// rotateLeft performs a left rotation about z and returns the new subtree
// root (the old right child of z).
func (tt *AvlTree[T]) rotateLeft(z *AvlTreeElement[T]) *AvlTreeElement[T] {
	y := z.right
	z.right = y.left
	y.left = z
	z.height = g_lib.Max(tt.Height(z.left), tt.Height(z.right)) + 1
	y.height = g_lib.Max(tt.Height(y.left), tt.Height(y.right)) + 1
	return y
}

// rebalanceNode recomputes the height of *root and, if the subtree rooted
// there is out of balance, performs the rotation (single or double) required
// to restore the AVL height invariant.
func (tt *AvlTree[T]) rebalanceNode(root **AvlTreeElement[T]) {
	z := *root
	if z == nil {
		return
	}
	z.height = g_lib.Max(tt.Height(z.left), tt.Height(z.right)) + 1
	switch b := tt.calcAvlBalance(z); {
	case b > 1:
		// Left heavy: Left Right case if the left child is right heavy.
		if tt.calcAvlBalance(z.left) < 0 {
			z.left = tt.rotateLeft(z.left)
		}
		*root = tt.rotateRight(z)
	case b < -1:
		// Right heavy: Right Left case if the right child is left heavy.
		if tt.calcAvlBalance(z.right) > 0 {
			z.right = tt.rotateRight(z.right)
		}
		*root = tt.rotateLeft(z)
	}
}

// Insert will add a new item to the tree.  If it is a duplicate of an existing
// item the new item will replace the existing one.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) Insert(item *T) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	tt.nlInsert(item)
}

// nlInsert is the no-lock internal version of Insert.
func (tt *AvlTree[T]) nlInsert(item *T) {

	node := NewAvlTreeElement[T](item)
	if tt.nlIsEmpty() {
		tt.root = node
		tt.length = 1
		return
	}

	// Recursive insert with AVL rebalancing on the way back up.
	var insert func(root **AvlTreeElement[T])
	insert = func(root **AvlTreeElement[T]) {
		if *root == nil {
			*root = node
			tt.length++
			return
		}
		if c := (*item).Compare(*((*root).data)); c == 0 {
			// Replace duplicate node with new node, keeping children/height.
			node.left = (*root).left
			node.right = (*root).right
			node.height = (*root).height
			*root = node
			return
		} else if c < 0 {
			insert(&((*root).left))
		} else {
			insert(&((*root).right))
		}
		tt.rebalanceNode(root)
	}

	insert(&(tt.root))
}

// Length returns the number of elements in the tree.
// Complexity is O(1).
func (tt *AvlTree[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Search walks the tree looking for `find` and returns the found item if it
// is in the tree.  If it is not found then nil is returned.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) Search(find *T) (item *T) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlSearch(find)
}

// nlSearch is the no-lock internal version of Search.
func (tt *AvlTree[T]) nlSearch(find *T) (item *T) {
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

// Dump prints the tree to the writer `fo`.
// Complexity is O(n).
func (tt *AvlTree[T]) Dump(fo io.Writer) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()

	k := tt.nlDepth() * 4
	var inorderTraversal func(cur *AvlTreeElement[T], n int)
	inorderTraversal = func(cur *AvlTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if cur.left != nil {
			inorderTraversal(cur.left, n+1)
		}
		if _, err := fmt.Fprintf(fo, "%s%v%s (left=%p/%p, right=%p/%p) self=%p\n",
			strings.Repeat(" ", 4*n), *(cur.data), strings.Repeat(" ", k-(4*n)),
			cur.left, &(cur.left), cur.right, &(cur.right), cur); err != nil {
			return // stop the dump on write error
		}
		if cur.right != nil {
			inorderTraversal(cur.right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

// Delete removes the node matching `find` from the tree.  True is returned if
// a node was removed, false otherwise.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) Delete(find *T) (found bool) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.nlDelete(find)
}

// nlDelete is the no-lock internal version of Delete.
func (tt *AvlTree[T]) nlDelete(find *T) (found bool) {

	if tt.nlIsEmpty() {
		return false
	}

	// Recursive delete with AVL rebalancing on the way back up.
	var delete func(root **AvlTreeElement[T])
	delete = func(root **AvlTreeElement[T]) {
		if *root == nil {
			return // not found
		}
		c := (*find).Compare(*((*root).data))
		if c < 0 {
			delete(&((*root).left))
		} else if c > 0 {
			delete(&((*root).right))
		} else {
			found = true
			tt.length--
			n := *root
			switch {
			case n.left == nil:
				*root = n.right // leaf or single right child; may set nil
			case n.right == nil:
				*root = n.left // single left child
			default:
				// Two children: unlink the in-order successor (the leftmost
				// node of the right subtree) and put it in n's place.
				var removeMin func(r **AvlTreeElement[T]) *AvlTreeElement[T]
				removeMin = func(r **AvlTreeElement[T]) *AvlTreeElement[T] {
					if (*r).left == nil {
						m := *r
						*r = m.right // leftmost node may have a right subtree
						m.right = nil
						return m
					}
					rv := removeMin(&((*r).left))
					tt.rebalanceNode(r)
					return rv
				}
				succ := removeMin(&(n.right))
				succ.left, succ.right, succ.height = n.left, n.right, n.height
				*root = succ
				n.left, n.right, n.data = nil, nil, nil // release the old node
			}
		}
		tt.rebalanceNode(root)
	}

	delete(&(tt.root))
	return
}

// FindMin returns the smallest value in the tree, or nil if the tree is empty.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) FindMin() (item *T) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMin()
}

// nlFindMin is the no-lock internal version of FindMin.
func (tt *AvlTree[T]) nlFindMin() (item *T) {
	cur := tt.root
	if cur == nil {
		return nil
	}
	for cur.left != nil {
		cur = cur.left
	}
	return cur.data
}

// FindMax returns the largest value in the tree, or nil if the tree is empty.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) FindMax() (item *T) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMax()
}

// nlFindMax is the no-lock internal version of FindMax.
func (tt *AvlTree[T]) nlFindMax() (item *T) {
	cur := tt.root
	if cur == nil {
		return nil
	}
	for cur.right != nil {
		cur = cur.right
	}
	return cur.data
}

// DeleteAtHead removes the smallest element of the tree, returning false if
// the tree is empty.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}

	x := tt.nlFindMin()
	tt.nlDelete(x)
	return true
}

// DeleteAtTail removes the largest element of the tree, returning false if
// the tree is empty.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}

	x := tt.nlFindMax()
	tt.nlDelete(x)
	return true
}

// Reverse swaps the left and right children of every node, mirroring the
// tree.  This is a strange but useful operation since it renders the tree
// unusable for future inserts/searches unless it is reversed again.
// Complexity is O(n).
func (tt *AvlTree[T]) Reverse() {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	var postTraversal func(cur *AvlTreeElement[T])
	postTraversal = func(cur *AvlTreeElement[T]) {
		if cur == nil {
			return
		}
		postTraversal(cur.left)
		postTraversal(cur.right)
		cur.left, cur.right = cur.right, cur.left
	}
	postTraversal(tt.root)
}

// Index returns the Nth item of the tree in in-order sequence, or nil if pos
// is out of range.
// Complexity is O(n).
func (tt *AvlTree[T]) Index(pos int) (item *T) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if pos < 0 || pos >= tt.length {
		return nil
	}

	var n = 0
	var done = false
	var inorderTraversal func(cur *AvlTreeElement[T])
	inorderTraversal = func(cur *AvlTreeElement[T]) {
		if cur == nil || done {
			return
		}
		inorderTraversal(cur.left)
		if n == pos {
			item = cur.data
			done = true
		}
		n++
		if !done {
			inorderTraversal(cur.right)
		}
	}
	inorderTraversal(tt.root)
	return
}

// Depth returns the height of the tree: the number of nodes on the longest
// root-to-leaf path.  An empty tree has depth 0, a single node depth 1.
// Complexity is O(1).
func (tt *AvlTree[T]) Depth() (d int) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlDepth()
}

// nlDepth is the no-lock internal version of Depth.
func (tt *AvlTree[T]) nlDepth() (d int) {
	return tt.Height(tt.root)
}

// ApplyFunction is the callback type used by the Walk* functions.  It is
// called with the in-walk position, the depth of the node, the node data and
// the userData passed to the walk.  Returning false stops the walk.
type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool

// WalkInOrder walks the tree in-order applying `fx` to each node.  If `fx`
// returns false the walk stops.  `fx` is called while holding a read lock and
// must not call back into the same tree.
// Complexity is O(n).
func (tt *AvlTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	tt.nlWalkInOrder(fx, userData)
}

// nlWalkInOrder is the no-lock internal version of WalkInOrder.
func (tt *AvlTree[T]) nlWalkInOrder(fx ApplyFunction[T], userData interface{}) {
	p := 0
	b := true
	var inorderTraversal func(cur *AvlTreeElement[T], n int)
	inorderTraversal = func(cur *AvlTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if b {
			inorderTraversal(cur.left, n+1)
		}
		if b {
			b = fx(p, n, cur.data, userData)
			p++
		}
		if b {
			inorderTraversal(cur.right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

// WalkPreOrder walks the tree pre-order applying `fx` to each node.  If `fx`
// returns false the walk stops.  `fx` is called while holding a read lock and
// must not call back into the same tree.
// Complexity is O(n).
func (tt *AvlTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var preOrderTraversal func(cur *AvlTreeElement[T], n int)
	preOrderTraversal = func(cur *AvlTreeElement[T], n int) {
		if cur == nil {
			return
		}
		b = fx(p, n, cur.data, userData)
		p++
		if b {
			preOrderTraversal(cur.left, n+1)
		}
		if b {
			preOrderTraversal(cur.right, n+1)
		}
	}
	preOrderTraversal(tt.root, 0)
}

// WalkPostOrder walks the tree post-order applying `fx` to each node.  If `fx`
// returns false the walk stops.  `fx` is called while holding a read lock and
// must not call back into the same tree.
// Complexity is O(n).
func (tt *AvlTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {
	if tt == nil {
		panic("operation on a nil tree")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var postOrderTraversal func(cur *AvlTreeElement[T], n int)
	postOrderTraversal = func(cur *AvlTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if b {
			postOrderTraversal(cur.left, n+1)
		}
		if b {
			postOrderTraversal(cur.right, n+1)
		}
		if b {
			b = fx(p, n, cur.data, userData)
			p++
		}
	}
	postOrderTraversal(tt.root, 0)
}

// toSlice returns a snapshot of the data of the tree in in-order (sorted)
// sequence, taken under the read lock.
// Complexity is O(n).
func (tt *AvlTree[T]) toSlice() (rv []*T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	tt.nlWalkInOrder(func(_, _ int, data *T, _ interface{}) bool {
		rv = append(rv, data)
		return true
	}, nil)
	return
}

// Copy replaces the contents of tt with a copy of yy.  The data items are
// shared, not deep-copied.  The source is snapshotted under its read lock
// before tt is modified, so tt.Copy(tt) is a safe no-op.
// Complexity is O(n log2(n)).
func (tt *AvlTree[T]) Copy(yy *AvlTree[T]) {
	if tt == nil {
		panic("operation on a nil tree")
	}
	data := yy.toSlice()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	for _, d := range data {
		tt.nlInsert(d)
	}
}

// Union is a set union, tt = yy union zz.  If an item is in both yy and zz
// the one from zz is kept.  Sources are snapshotted under their read locks
// before tt is modified, so tt may alias yy or zz.
// Complexity is O(n log2(n)).
func (tt *AvlTree[T]) Union(yy, zz *AvlTree[T]) {
	if tt == nil {
		panic("operation on a nil tree")
	}
	a := yy.toSlice()
	b := zz.toSlice()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	for _, d := range a {
		tt.nlInsert(d)
	}
	for _, d := range b {
		tt.nlInsert(d)
	}
}

// Minus is a set minus, tt = yy - zz.  Sources are snapshotted under their
// read locks before tt is modified, so tt may alias yy or zz.
// Complexity is O(n log2(n)).
func (tt *AvlTree[T]) Minus(yy, zz *AvlTree[T]) {
	if tt == nil {
		panic("operation on a nil tree")
	}
	a := yy.toSlice()
	b := zz.toSlice()
	var zzTree AvlTree[T]
	for _, d := range b {
		zzTree.nlInsert(d)
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	for _, d := range a {
		if zzTree.nlSearch(d) == nil {
			tt.nlInsert(d)
		}
	}
}

// Intersect is a set intersection, tt = yy intersect zz.  Sources are
// snapshotted under their read locks before tt is modified, so tt may alias
// yy or zz.
// Complexity is O(n log2(n)).
func (tt *AvlTree[T]) Intersect(yy, zz *AvlTree[T]) {
	if tt == nil {
		panic("operation on a nil tree")
	}
	a := yy.toSlice()
	b := zz.toSlice()
	var zzTree AvlTree[T]
	for _, d := range b {
		zzTree.nlInsert(d)
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	for _, d := range a {
		if zzTree.nlSearch(d) != nil {
			tt.nlInsert(d)
		}
	}
}

/* vim: set noai ts=4 sw=4: */
