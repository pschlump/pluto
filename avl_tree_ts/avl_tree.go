/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package avl_tree_ts implements a generic AVL self-balancing binary
// search tree that is safe for concurrent use.  It is the thread-safe
// twin of github.com/pschlump/charon/avl_tree — the same API, guarded by
// a sync.RWMutex.
//
// An AVL tree keeps the heights of the two child subtrees of every node
// within one of each other, so Insert, Delete and Search are O(log₂ n)
// even in the worst case — regardless of insertion order.  Every node
// caches its subtree height, so Depth is O(1).
//
// Concurrency model:
//
//	Reads (Search, FindMin, FindMax, Index, Depth, Len, Length, IsEmpty)
//	take the read lock and release it before returning, so they run in
//	parallel with each other.
//	Writes (Insert, Delete, DeleteAtHead, DeleteAtTail, Reverse, Truncate)
//	take the write lock.
//	Front, All and Backward operate on a snapshot taken when they are
//	called (one O(n) copy, under the read lock), so they are safe to use
//	concurrently with any tree operation — including mutating the tree
//	from inside the loop — and never observe later modifications.
//	The Walk* functions and Dump hold the read lock for the whole
//	traversal; their callbacks must not call methods on the same tree, or
//	the call can deadlock.  To visit elements while mutating, iterate a
//	snapshot with All instead.
//	The set operations (Copy, Union, Minus, Intersect) snapshot their
//	operands under each operand's own read lock before taking the
//	destination's write lock, so no two locks are ever held at once and an
//	operand may safely alias the destination or the other operand.
//
// Elements are stored and returned by value and ordering is a direct
// function call, so element data is never boxed into an interface and
// never unboxed with a type assertion.  Trees of naturally ordered key
// types (all integers, floats and strings) are created with NewAvlTree,
// which orders elements with the built-in < and > operators of the key
// type; trees of any other type — including structs ordered by a single
// field — are created with NewAvlTreeFunc, which takes a caller supplied
// comparison function; the element type does not have to implement any
// interface.
//
// Basic operations on an Avl Tree:
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
//	DeleteAtHead — delete the smallest element of the tree.
//	DeleteAtTail — delete the largest element of the tree.
//	Depth — number of levels in the deepest part of the tree.
//	WalkInOrder / WalkPreOrder / WalkPostOrder — callback-based traversals.
//	Copy / Union / Minus / Intersect — whole-tree set operations.
//	Front — old-style in-order iterator (operates on a snapshot).
//	All / Backward — range-over-func iterators (operate on a snapshot).
//
// A nil *AvlTree and the zero value both behave as an empty tree for
// every operation except Insert: Search finds nothing, Delete,
// DeleteAtHead and DeleteAtTail return false, FindMin, FindMax and Index
// report not-found, Len and Depth are 0, and the walks and iterators
// visit nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewAvlTreeFunc(nil)          — nil comparison function, caught at construction.
//	Insert on a nil tree         — a nil tree cannot store an element.
//	Insert on a zero-value tree  — no comparison function; the message names the constructors.
package avl_tree_ts

import (
	"cmp"
	"fmt"
	"io"
	"strings"
	"sync"
)

// AvlTreeElement is a node of an AvlTree.
type AvlTreeElement[T any] struct {
	data        T
	height      int
	left, right *AvlTreeElement[T]
}

// AvlTree is a generic AVL balanced binary tree that is safe for
// concurrent use.  Use NewAvlTree for naturally ordered key types
// (numbers, strings) or NewAvlTreeFunc for a caller supplied comparison
// function.  The zero value is an empty tree.
type AvlTree[T any] struct {
	root   *AvlTreeElement[T]
	length int
	lock   sync.RWMutex

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// -------------------------------------------------------------------------------------------------------

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewAvlTree and
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

// NewAvlTree creates a new AvlTree for any naturally ordered key type
// (all integers, floats and strings — cmp.Ordered).  Ordering uses the
// built-in < and > operators of T; no interface and no boxing is
// involved.
// Complexity is O(1).
func NewAvlTree[T cmp.Ordered]() *AvlTree[T] {
	return &AvlTree[T]{cmp: Compare[T]}
}

// NewAvlTreeFunc creates a new AvlTree that orders elements with the
// caller supplied comparison function fx.  fx must return a negative
// value if a sorts before b, 0 if the two are duplicates and a positive
// value if a sorts after b, and must order elements consistently.  This
// lets any type — for example a struct ordered by one of its fields — be
// stored without implementing any interface.
// Complexity is O(1).
func NewAvlTreeFunc[T any](fx func(a, b T) int) *AvlTree[T] {
	if fx == nil {
		panic("avl_tree_ts: NewAvlTreeFunc called with a nil comparison function")
	}
	return &AvlTree[T]{cmp: fx}
}

// compare orders a and b, guarding against a zero-value tree that was not
// created by one of the constructors.  The caller must hold the lock.
func (tt *AvlTree[T]) compare(a, b T) int {
	if tt.cmp == nil {
		panic("avl_tree_ts: no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)")
	}
	return tt.cmp(a, b)
}

// NewAvlTreeElement creates a new tree node holding `x`.
// Complexity is O(1).
func NewAvlTreeElement[T any](x T) *AvlTreeElement[T] {
	return &AvlTreeElement[T]{
		data:   x,
		height: 1,
	}
}

// Height returns the saved height of the node `e` (0 for a nil node).
// The height is re-calculated as the tree is modified.
// Complexity is O(1).
func (tt *AvlTree[T]) Height(e *AvlTreeElement[T]) int {
	if e == nil {
		return 0
	}
	return e.height
}

// calcAvlBalance returns the difference in height between the left and
// right subtrees of `e`.  When the absolute value exceeds 1 the subtree is
// rotated to restore balance.
// Complexity is O(1).
func (tt *AvlTree[T]) calcAvlBalance(e *AvlTreeElement[T]) int {
	if e == nil {
		return 0
	}
	return tt.Height(e.left) - tt.Height(e.right)
}

// GetData returns the user data from the AVL tree node.
// Complexity is O(1).
func (ee *AvlTreeElement[T]) GetData() T {
	return ee.data
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty returns true if the tree is empty.
// Complexity is O(1).
func (tt *AvlTree[T]) IsEmpty() bool {
	if tt == nil {
		return true
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlIsEmpty()
}

// nlIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlIsEmpty() bool {
	return tt.root == nil
}

// Truncate removes all data from the tree.  The comparison function is
// kept, so the tree remains usable and can simply be refilled.
// Complexity is O(1).
func (tt *AvlTree[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
}

// nlTruncate is Truncate without locking; the caller must hold the write
// lock.
func (tt *AvlTree[T]) nlTruncate() {
	tt.root = nil
	tt.length = 0
}

/*

Rotations used to rebalance the tree after Insert and Delete.

Let the newly inserted (or deleted) node be w.
1) Perform the standard BST insert/delete for w.
2) Starting from w, travel up and find the first unbalanced node.  Let z be
   the first unbalanced node, y the child of z on the path from w to z, and
   x the grandchild of z on the path from w to z.
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
// root (the old left child of z).  The caller must hold the lock.
func (tt *AvlTree[T]) rotateRight(z *AvlTreeElement[T]) *AvlTreeElement[T] {
	y := z.left
	z.left = y.right
	y.right = z
	z.height = max(tt.Height(z.left), tt.Height(z.right)) + 1
	y.height = max(tt.Height(y.left), tt.Height(y.right)) + 1
	return y
}

// rotateLeft performs a left rotation about z and returns the new subtree
// root (the old right child of z).  The caller must hold the lock.
func (tt *AvlTree[T]) rotateLeft(z *AvlTreeElement[T]) *AvlTreeElement[T] {
	y := z.right
	z.right = y.left
	y.left = z
	z.height = max(tt.Height(z.left), tt.Height(z.right)) + 1
	y.height = max(tt.Height(y.left), tt.Height(y.right)) + 1
	return y
}

// rebalanceNode recomputes the height of *root and, if the subtree rooted
// there is out of balance, performs the rotation (single or double)
// required to restore the AVL height invariant.  The caller must hold the
// lock.
func (tt *AvlTree[T]) rebalanceNode(root **AvlTreeElement[T]) {
	z := *root
	if z == nil {
		return
	}
	z.height = max(tt.Height(z.left), tt.Height(z.right)) + 1
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

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one in place and
// false is returned; true is returned when a new node was added.
// Insert panics on a nil tree or on a zero-value tree (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty tree.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Insert(item T) (vv bool) {
	if tt == nil {
		panic("avl_tree_ts: Insert called on a nil tree")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.cmp == nil {
		panic("avl_tree_ts: Insert called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)")
	}
	return tt.nlInsert(item)
}

// nlInsert is Insert without locking; the caller must hold the write lock
// and tt.cmp must be non-nil.
func (tt *AvlTree[T]) nlInsert(item T) (vv bool) {
	node := NewAvlTreeElement(item)
	if tt.nlIsEmpty() {
		tt.root = node
		tt.length = 1
		return true
	}

	// Recursive insert with AVL rebalancing on the way back up.
	var insert func(root **AvlTreeElement[T]) bool
	insert = func(root **AvlTreeElement[T]) bool {
		if *root == nil {
			*root = node
			tt.length++
			return true
		}
		if c := tt.cmp(item, (*root).data); c == 0 {
			// Duplicate: replace the stored value in place, keeping the
			// children and the height so the tree shape does not change.
			(*root).data = item
			return false
		} else if c < 0 {
			vv = insert(&(*root).left)
		} else {
			vv = insert(&(*root).right)
		}
		tt.rebalanceNode(root)
		return vv
	}

	return insert(&tt.root)
}

// Len returns the number of elements in the tree.
// Complexity is O(1).
func (tt *AvlTree[T]) Len() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Length is an alias for Len; it returns the number of elements in the
// tree.
// Complexity is O(1).
func (tt *AvlTree[T]) Length() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Search walks the tree looking for `find` and returns the found item from
// the tree.  If it is not found then false is returned.  `find` only needs
// the fields that the tree's comparison function reads: a probe with just
// the key fields set finds the element with the full data.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Search(find T) (item T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlSearch(find)
}

// nlSearch is Search without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlSearch(find T) (item T, found bool) {
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
// aid; use All, Backward or the Walk* functions to process the data.  The
// read lock is held for the whole dump, so `fo` must not call methods on
// the same tree.
// Complexity is O(n).
func (tt *AvlTree[T]) Dump(fo io.Writer) {
	if tt == nil {
		return
	}

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
			strings.Repeat(" ", 4*n), cur.data, strings.Repeat(" ", k-(4*n)),
			cur.left, &(cur.left), cur.right, &(cur.right), cur); err != nil {
			return // stop the dump on write error
		}
		if cur.right != nil {
			inorderTraversal(cur.right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

// Delete removes the node matching `find` from the tree.  True is returned
// if a node was removed, false otherwise.  As with Search, `find` only
// needs the fields that the comparison function reads.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Delete(find T) (found bool) {
	if tt == nil {
		return false
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}
	return tt.nlDelete(find)
}

// nlDelete is Delete without locking; the caller must hold the write lock
// and the tree must be non-empty.
func (tt *AvlTree[T]) nlDelete(find T) (found bool) {

	// Recursive delete with AVL rebalancing on the way back up.
	var delete func(root **AvlTreeElement[T])
	delete = func(root **AvlTreeElement[T]) {
		if *root == nil {
			return // not found
		}
		c := tt.compare(find, (*root).data)
		if c < 0 {
			delete(&(*root).left)
		} else if c > 0 {
			delete(&(*root).right)
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
					rv := removeMin(&(*r).left)
					tt.rebalanceNode(r)
					return rv
				}
				succ := removeMin(&(n.right))
				succ.left, succ.right, succ.height = n.left, n.right, n.height
				*root = succ
			}
		}
		tt.rebalanceNode(root)
	}

	delete(&tt.root)
	return
}

// FindMin returns the smallest value in the tree, or false if the tree is
// empty.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) FindMin() (item T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMin()
}

// nlFindMin is FindMin without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlFindMin() (item T, found bool) {
	cur := tt.root
	if cur == nil {
		return
	}
	for cur.left != nil {
		cur = cur.left
	}
	return cur.data, true
}

// FindMax returns the largest value in the tree, or false if the tree is
// empty.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) FindMax() (item T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMax()
}

// nlFindMax is FindMax without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlFindMax() (item T, found bool) {
	cur := tt.root
	if cur == nil {
		return
	}
	for cur.right != nil {
		cur = cur.right
	}
	return cur.data, true
}

// DeleteAtHead removes the smallest element of the tree, returning false
// if the tree is empty.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		return false
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}

	x, _ := tt.nlFindMin()
	tt.nlDelete(x)
	return true
}

// DeleteAtTail removes the largest element of the tree, returning false if
// the tree is empty.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		return false
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return false
	}

	x, _ := tt.nlFindMax()
	tt.nlDelete(x)
	return true
}

// Reverse swaps the left and right children of every node, mirroring the
// tree.  After a Reverse the tree is no longer ordered by its comparison
// function until it is reversed again.
// Complexity is O(n).
func (tt *AvlTree[T]) Reverse() {
	if tt == nil {
		return
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.nlIsEmpty() {
		return
	}

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

// Index returns the `pos`-th element of the tree in in-order order, or
// false if `pos` is out of range.
// Complexity is O(n).
func (tt *AvlTree[T]) Index(pos int) (item T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if tt.nlIsEmpty() {
		return
	}
	if pos < 0 || pos >= tt.length {
		return
	}

	var n = 0
	var inorderTraversal func(cur *AvlTreeElement[T])
	inorderTraversal = func(cur *AvlTreeElement[T]) {
		if cur == nil || found {
			return
		}
		inorderTraversal(cur.left)
		if n == pos {
			item = cur.data
			found = true
		}
		n++
		if !found {
			inorderTraversal(cur.right)
		}
	}
	inorderTraversal(tt.root)
	return
}

// Depth returns the height of the tree: the number of nodes on the longest
// root-to-leaf path.  An empty tree has depth 0, a single node depth 1.
// Complexity is O(1) — every node caches its subtree height.
func (tt *AvlTree[T]) Depth() int {
	if tt == nil {
		return 0
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlDepth()
}

// nlDepth is Depth without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlDepth() (d int) {
	return tt.Height(tt.root)
}

// ApplyFunction is the callback type used by the Walk* functions.  `pos` is
// the ordinal position of the element in the walk order and `depth` is the
// depth of the node in the tree (root is 0).  Returning false stops the
// walk.  Caller state is captured in a closure, so it keeps its static
// type and is never boxed.
type ApplyFunction[T any] func(pos, depth int, data T) bool

// WalkInOrder visits every element in in-order (ascending) order.
// Returning false from fx stops the walk.  The read lock is held for the
// whole walk: fx must not call methods on the same tree, or the call can
// deadlock.
// Complexity is O(n).
func (tt *AvlTree[T]) WalkInOrder(fx ApplyFunction[T]) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

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
		// ----------------------------------------------------------------------
		if b {
			b = fx(p, n, cur.data)
			p++
		}
		// ----------------------------------------------------------------------
		if b {
			inorderTraversal(cur.right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

// WalkPreOrder visits every element in pre-order (node, left, right)
// order.  Returning false from fx stops the walk.  The read lock is held
// for the whole walk: fx must not call methods on the same tree, or the
// call can deadlock.
// Complexity is O(n).
func (tt *AvlTree[T]) WalkPreOrder(fx ApplyFunction[T]) {
	if tt == nil {
		return
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
		// ----------------------------------------------------------------------
		if b {
			b = fx(p, n, cur.data)
			p++
		}
		// ----------------------------------------------------------------------
		if b {
			preOrderTraversal(cur.left, n+1)
		}
		if b {
			preOrderTraversal(cur.right, n+1)
		}
	}
	preOrderTraversal(tt.root, 0)
}

// WalkPostOrder visits every element in post-order (left, right, node)
// order.  Returning false from fx stops the walk.  The read lock is held
// for the whole walk: fx must not call methods on the same tree, or the
// call can deadlock.
// Complexity is O(n).
func (tt *AvlTree[T]) WalkPostOrder(fx ApplyFunction[T]) {
	if tt == nil {
		return
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
		// ----------------------------------------------------------------------
		if b {
			b = fx(p, n, cur.data)
			p++
		}
		// ----------------------------------------------------------------------
	}
	postOrderTraversal(tt.root, 0)
}

// snapshot returns the data of the tree in in-order (sorted) sequence
// together with the tree's comparison function, all read under the read
// lock.  A nil tree yields no data and no comparison function.  The caller
// must NOT hold any lock.
// Complexity is O(n).
func (tt *AvlTree[T]) snapshot() (items []T, cmp func(a, b T) int) {
	if tt == nil {
		return nil, nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()

	items = make([]T, 0, tt.length)
	var stk []*AvlTreeElement[T]
	n := tt.root
	for n != nil || len(stk) > 0 {
		for n != nil {
			stk = append(stk, n)
			n = n.left
		}
		n = stk[len(stk)-1]
		stk = stk[:len(stk)-1]
		items = append(items, n.data)
		n = n.right
	}
	return items, tt.cmp
}

// adoptCmp makes sure tt has a comparison function while holding the write
// lock, adopting one of the operand comparison functions if tt is a
// zero-value tree.  The functions must have been read under the operand's
// own read lock (see snapshot); reading another tree's cmp field directly
// here would race with a concurrent set operation adopting into it.
func (tt *AvlTree[T]) adoptCmp(yCmp, zCmp func(a, b T) int) {
	if tt.cmp == nil {
		tt.cmp = yCmp
	}
	if tt.cmp == nil {
		tt.cmp = zCmp
	}
}

// Copy replaces the contents of tt with a copy of yy.  The source is
// snapshotted under its own read lock before tt's write lock is taken, so
// yy may alias tt.  The result is ordered by tt's comparison function; a
// zero-value tt adopts yy's.
// Complexity is O(n log₂ n).
func (tt *AvlTree[T]) Copy(yy *AvlTree[T]) {
	if tt == nil {
		return
	}
	data, yCmp := yy.snapshot()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	tt.adoptCmp(yCmp, nil)
	if tt.cmp == nil {
		return // no ordering available: yy cannot have had any data
	}
	for _, d := range data {
		tt.nlInsert(d)
	}
}

// Union is a set union, tt = yy union zz.  If an item is in both yy and zz
// the one from zz is kept.  The sources are snapshotted under their own
// read locks before tt's write lock is taken, so yy and zz may alias tt or
// each other.  The result is ordered by tt's comparison function; a
// zero-value tt adopts yy's (then zz's).
// Complexity is O(n log₂ n).
func (tt *AvlTree[T]) Union(yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, aCmp := yy.snapshot()
	b, bCmp := zz.snapshot()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	tt.adoptCmp(aCmp, bCmp)
	if tt.cmp == nil {
		return // no source had a comparison function, so no source had data
	}
	for _, d := range a {
		tt.nlInsert(d)
	}
	for _, d := range b {
		tt.nlInsert(d)
	}
}

// Minus is a set minus, tt = yy - zz.  The sources are snapshotted under
// their own read locks before tt's write lock is taken, so yy and zz may
// alias tt or each other.  The result is ordered by tt's comparison
// function; a zero-value tt adopts yy's (then zz's).
// Complexity is O(n log₂ n).
func (tt *AvlTree[T]) Minus(yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, aCmp := yy.snapshot()
	b, bCmp := zz.snapshot()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	tt.adoptCmp(aCmp, bCmp)
	if tt.cmp == nil {
		return
	}
	var zzTree AvlTree[T]
	zzTree.cmp = tt.cmp
	for _, d := range b {
		zzTree.nlInsert(d)
	}
	for _, d := range a {
		if _, found := zzTree.nlSearch(d); !found {
			tt.nlInsert(d)
		}
	}
}

// Intersect is a set intersection, tt = yy intersect zz.  The sources are
// snapshotted under their own read locks before tt's write lock is taken,
// so yy and zz may alias tt or each other.  The result is ordered by tt's
// comparison function; a zero-value tt adopts yy's (then zz's).
// Complexity is O(n log₂ n).
func (tt *AvlTree[T]) Intersect(yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, aCmp := yy.snapshot()
	b, bCmp := zz.snapshot()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	tt.adoptCmp(aCmp, bCmp)
	if tt.cmp == nil {
		return
	}
	var zzTree AvlTree[T]
	zzTree.cmp = tt.cmp
	for _, d := range b {
		zzTree.nlInsert(d)
	}
	for _, d := range a {
		if _, found := zzTree.nlSearch(d); found {
			tt.nlInsert(d)
		}
	}
}
