/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package avl_tree_ts implements a generic AVL self-balancing binary
// search tree that is safe for concurrent use.  It is the thread-safe
// twin of github.com/pschlump/pluto/avl_tree — the same API, guarded by
// a sync.RWMutex.
//
// An AVL tree keeps the heights of the two child subtrees of every node
// within one of each other, so Insert, Delete and Search are O(log₂ n)
// even in the worst case — regardless of insertion order.  Every node
// caches its subtree height, so Depth is O(1).
//
// Concurrency model:
//
//	Reads (Search, FindMin, FindMax, Floor, Ceil, Index, Depth, Len,
//	Length, IsEmpty) take the read lock and release it before returning,
//	so they run in parallel with each other.
//	Writes (Insert, Upsert, Delete, Remove, DeleteAtHead, DeleteAtTail,
//	Reverse, Truncate) take the write lock.
//	Front, All and Backward operate on a snapshot taken when they are
//	called (one O(n) copy, under the read lock), so they are safe to use
//	concurrently with any tree operation — including mutating the tree
//	from inside the loop — and never observe later modifications.
//	The Walk* functions and Dump hold the read lock for the whole
//	traversal; their callbacks must not call methods on the same tree, or
//	the call can deadlock.  To visit elements while mutating, iterate a
//	snapshot with All instead.
//	The set operations (Copy, Union, Minus, Intersect,
//	SymmetricDifference) snapshot their operands under each operand's own
//	read lock before taking the destination's write lock, so no two locks
//	are ever held at once and an operand may safely alias the destination
//	or the other operand.  Under the write lock the sorted snapshots are
//	merged in one linear pass and the result is built directly as a
//	perfectly balanced tree, so each set operation is O(n+m).
//	The capability batches (InsertSlice, InsertIter, DeleteSlice) hold
//	the write lock once for the whole batch — a batch is atomic.  The
//	compute-after-snapshot methods (Slice, String, StringFunc, Equal,
//	DeleteFunc) snapshot under the read lock and run the caller format or
//	predicate without any lock held, so their callbacks may safely call
//	back into the tree.
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
//	Upsert — Insert in one descent that also returns the replaced element.
//	Delete — delete a specified element from the tree (elements can be found via Search).
//	Remove — Delete in one descent that also returns the stored element.
//	Search — return the given element from the tree.
//	Index — return the Nth element of the tree in in-order order.
//	IsEmpty — report whether the tree is empty.
//	Len / Length — number of elements in the tree; 0 is an empty tree.
//	Reverse — swap the left and right children of every node in the tree.
//	Truncate — delete all the nodes in the tree.
//	FindMin / FindMax — return the smallest / largest element in the tree.
//	Floor / Ceil — return the largest element ≤ key / smallest ≥ key.
//	DeleteAtHead — delete the smallest element of the tree.
//	DeleteAtTail — delete the largest element of the tree.
//	Depth — number of levels in the deepest part of the tree.
//	WalkInOrder / WalkPreOrder / WalkPostOrder — callback-based traversals.
//	Copy / Union / Minus / Intersect / SymmetricDifference — whole-tree set operations.
//	Front — old-style in-order iterator (operates on a snapshot).
//	All / Backward — range-over-func iterators (operate on a snapshot).
//	Contains — membership predicate (the found flag of Search).
//	ContainsSlice / ContainsAny — membership batches: all / at least one present.
//	InsertSlice / InsertIter — batch insert from a slice / range-over-func iterator.
//	DeleteSlice / DeleteFunc — batch delete by elements / by predicate.
//	Slice — fresh slice of every element, in-order (ascending).
//	String / StringFunc — render the elements, in-order, with a caller format option.
//	Equal — same element set as another tree (membership, never shape).
//	Clone — deep copy of the tree.
//
// A nil *AvlTree and the zero value both behave as an empty tree for
// every operation except Insert and Upsert: Search finds nothing, Delete,
// DeleteAtHead and DeleteAtTail return false, Remove reports not-found,
// FindMin, FindMax, Floor,
// Ceil and Index report not-found, Len and Depth are 0, and the walks and
// iterators visit nothing.
//
// The batch inserters InsertSlice and InsertIter raise the same insert
// contract as Insert — a nil tree or a zero-value tree panics when, and
// only when, the batch would actually store an element; an empty batch is
// tolerated everywhere.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewAvlTreeFunc(nil)          — nil comparison function, caught at construction.
//	Insert or Upsert on a nil tree         — a nil tree cannot store an element.
//	Insert or Upsert on a zero-value tree  — no comparison function; the message names the constructors.
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

// maxAvlHeight bounds the height of every AVL tree that can exist.  A
// minimal AVL tree of height h holds F(h+2)-1 nodes (F the Fibonacci
// numbers) and F(94) exceeds 2^64, so no tree that fits in a 64-bit
// address space is taller than 91.  The descents (insert, delete) record
// their root-to-leaf path and the traversals keep their pending nodes in
// fixed arrays of this size, so neither allocates nor recurses through a
// closure.
const maxAvlHeight = 92

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one in place and
// false is returned; true is returned when a new node was added.  A
// replace allocates nothing; only a genuine insert allocates its node.
// Insert panics on a nil tree or on a zero-value tree (no comparison
// function); Upsert shares that contract.  Every other non-constructor
// operation treats both as an empty tree.
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

// Upsert inserts item, or replaces the stored element equal to it, in a
// single descent under one hold of the write lock.  When an equal element
// was present it is returned as old with replaced=true (the tree now
// holds item in its place, the shape unchanged and nothing allocated);
// otherwise item is added as a new node and old is the zero value with
// replaced=false.  Upsert is Insert that also hands back the previous
// element — an atomic replacement of the racy Search-then-Insert
// pattern.  Like Insert it panics on a nil tree or on a zero-value tree
// (no comparison function).
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Upsert(item T) (old T, replaced bool) {
	if tt == nil {
		panic("avl_tree_ts: Upsert called on a nil tree (a nil tree cannot store an element; create the tree with NewAvlTree or NewAvlTreeFunc)")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.cmp == nil {
		panic("avl_tree_ts: Upsert called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)")
	}
	return tt.nlUpsert(item)
}

// nlInsert is Insert without locking; the caller must hold the write lock
// and tt.cmp must be non-nil.
func (tt *AvlTree[T]) nlInsert(item T) bool {
	_, replaced := tt.nlUpsert(item)
	return !replaced
}

// nlUpsert is Upsert without locking; the caller must hold the write lock
// and tt.cmp must be non-nil.  It is an iterative descent that records
// the links it passes in a fixed-size path.  An equal element is replaced
// in place (no allocation, no rebalancing — the shape does not change);
// otherwise the node is allocated only now, at the nil link where it
// belongs, and the path is retraced bottom-up.
func (tt *AvlTree[T]) nlUpsert(item T) (old T, replaced bool) {
	var path [maxAvlHeight]**AvlTreeElement[T]
	n := 0
	link := &tt.root
	for cur := *link; cur != nil; cur = *link {
		c := tt.cmp(item, cur.data)
		if c == 0 {
			old, cur.data = cur.data, item
			return old, true
		}
		path[n] = link
		n++
		if c < 0 {
			link = &cur.left
		} else {
			link = &cur.right
		}
	}
	*link = &AvlTreeElement[T]{data: item, height: 1}
	tt.length++
	tt.retrace(path[:n])
	return old, false
}

// retrace walks a recorded descent path bottom-up, recomputing heights and
// rotating where the AVL invariant is broken.  It stops at the first
// subtree whose height came out unchanged: the heights (and therefore the
// balance factors) of everything above it are unchanged too, so the
// remaining rebalanceNode calls would all be no-ops.  The caller must
// hold the write lock.
func (tt *AvlTree[T]) retrace(path []**AvlTreeElement[T]) {
	for i := len(path) - 1; i >= 0; i-- {
		before := (*path[i]).height
		tt.rebalanceNode(path[i])
		if (*path[i]).height == before {
			return
		}
	}
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

// Remove deletes the element equal to find and returns it: removed is the
// stored element (with all of its fields, not just the ones the
// comparison function reads) and found reports whether one was present.
// It is Delete that also hands back the element, in one descent under one
// hold of the write lock — an atomic replacement of the racy
// Search-then-Delete pattern.  A nil tree and the zero value behave as an
// empty tree (found=false), exactly like Delete.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Remove(find T) (removed T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.nlRemove(find)
}

// nlDelete is Delete without locking; the caller must hold the write lock.
func (tt *AvlTree[T]) nlDelete(find T) (found bool) {
	_, found = tt.nlRemove(find)
	return found
}

// nlRemove is Remove without locking; the caller must hold the write
// lock.  An iterative descent records the links it passes in a fixed-size
// path.  A node with two children is replaced by its in-order successor
// (the leftmost node of its right subtree), whose own descent extends the
// same path; then the path is retraced bottom-up.
func (tt *AvlTree[T]) nlRemove(find T) (removed T, found bool) {
	var path [maxAvlHeight]**AvlTreeElement[T]
	n := 0
	link := &tt.root
	for {
		cur := *link
		if cur == nil {
			return // not found (or empty tree)
		}
		c := tt.compare(find, cur.data)
		if c == 0 {
			break
		}
		path[n] = link
		n++
		if c < 0 {
			link = &cur.left
		} else {
			link = &cur.right
		}
	}

	target := *link
	removed = target.data
	tt.length--
	switch {
	case target.left == nil:
		*link = target.right // leaf or single right child; may set nil
	case target.right == nil:
		*link = target.left // single left child
	default:
		// Two children: unlink the in-order successor m and put it in
		// target's place, keeping target's height for the retrace.
		slot := n
		path[n] = link
		n++
		ml := &target.right
		for (*ml).left != nil {
			path[n] = ml
			n++
			ml = &(*ml).left
		}
		m := *ml
		*ml = m.right // the successor may have a right subtree
		m.left, m.right, m.height = target.left, target.right, target.height
		*link = m
		if n > slot+1 {
			// The path recorded &target.right, a field of the unlinked
			// node; the live link to that subtree is now m.right.
			path[slot+1] = &m.right
		}
	}
	// Zero the unlinked node: it may live in a set operation's slab (see
	// setop.go), which stays allocated while any of its nodes is in use,
	// and must not keep the element (or its subtrees) alive.
	*target = AvlTreeElement[T]{}
	tt.retrace(path[:n])
	return removed, true
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

// Floor returns the largest element less than or equal to key, or false
// when key is smaller than every element (there is no floor).  It is the
// point-query companion of FindMax: FindMax is Floor(+∞).  The read lock
// is held for the descent.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Floor(key T) (rv T, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlFloor(key)
}

// nlFloor is Floor without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlFloor(key T) (rv T, found bool) {
	cur := tt.root
	for cur != nil {
		switch c := tt.compare(key, cur.data); {
		case c == 0:
			return cur.data, true
		case c > 0:
			rv, found = cur.data, true // cur is a candidate floor; look right
			cur = cur.right
		default:
			cur = cur.left
		}
	}
	return
}

// Ceil returns the smallest element greater than or equal to key, or
// false when key is larger than every element (there is no ceiling).  It
// is the point-query companion of FindMin: FindMin is Ceil(−∞).  The read
// lock is held for the descent.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Ceil(key T) (rv T, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlCeil(key)
}

// nlCeil is Ceil without locking; the caller must hold the lock.
func (tt *AvlTree[T]) nlCeil(key T) (rv T, found bool) {
	cur := tt.root
	for cur != nil {
		switch c := tt.compare(key, cur.data); {
		case c == 0:
			return cur.data, true
		case c < 0:
			rv, found = cur.data, true // cur is a candidate ceiling; look left
			cur = cur.left
		default:
			cur = cur.right
		}
	}
	return
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

	// Iterative pre-order: swap at each node, keep one pending subtree
	// per level on a fixed-size stack (mirroring preserves heights).
	var stk [maxAvlHeight]*AvlTreeElement[T]
	sp := 0
	cur := tt.root
	for {
		for cur != nil {
			cur.left, cur.right = cur.right, cur.left
			if cur.right != nil {
				stk[sp] = cur.right
				sp++
			}
			cur = cur.left
		}
		if sp == 0 {
			return
		}
		sp--
		cur = stk[sp]
	}
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

	i := 0
	tt.walkInOrder(func(_, _ int, v T) bool {
		if i == pos {
			item, found = v, true
			return false
		}
		i++
		return true
	})
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

	tt.walkInOrder(fx)
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

	tt.walkPreOrder(fx)
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

	tt.walkPostOrder(fx)
}

// walkFrame is one pending node of an iterative Walk*, with its depth.
type walkFrame[T any] struct {
	n     *AvlTreeElement[T]
	depth int
}

// walkInOrder is the iterative in-order walk shared by WalkInOrder and
// Index: a fixed-size stack of (node, depth) frames.  The caller must
// hold the lock.
func (tt *AvlTree[T]) walkInOrder(fx ApplyFunction[T]) {
	var stk [maxAvlHeight]walkFrame[T]
	sp, p := 0, 0
	cur, d := tt.root, 0
	for {
		for ; cur != nil; cur, d = cur.left, d+1 {
			stk[sp] = walkFrame[T]{cur, d}
			sp++
		}
		if sp == 0 {
			return
		}
		sp--
		f := stk[sp]
		if !fx(p, f.depth, f.n.data) {
			return
		}
		p++
		cur, d = f.n.right, f.depth+1
	}
}

// walkPreOrder is the iterative pre-order walk: visit, remember the right
// child, go left.  At most one pending right child per level.  The caller
// must hold the lock.
func (tt *AvlTree[T]) walkPreOrder(fx ApplyFunction[T]) {
	var stk [maxAvlHeight]walkFrame[T]
	sp, p := 0, 0
	cur, d := tt.root, 0
	for {
		for ; cur != nil; cur, d = cur.left, d+1 {
			if !fx(p, d, cur.data) {
				return
			}
			p++
			if cur.right != nil {
				stk[sp] = walkFrame[T]{cur.right, d + 1}
				sp++
			}
		}
		if sp == 0 {
			return
		}
		sp--
		cur, d = stk[sp].n, stk[sp].depth
	}
}

// walkPostOrder is the iterative post-order walk: the stack holds the
// current root-to-node path; a node is visited once its right subtree is
// done (last == its right child, or it has none).  The caller must hold
// the lock.
func (tt *AvlTree[T]) walkPostOrder(fx ApplyFunction[T]) {
	var stk [maxAvlHeight]*AvlTreeElement[T]
	sp, p := 0, 0
	cur := tt.root
	var last *AvlTreeElement[T]
	for {
		for ; cur != nil; cur = cur.left {
			stk[sp] = cur
			sp++
		}
		if sp == 0 {
			return
		}
		top := stk[sp-1]
		if top.right != nil && top.right != last {
			cur = top.right
			continue
		}
		sp--
		if !fx(p, sp, top.data) { // depth == number of ancestors on the stack
			return
		}
		p++
		last = top
	}
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

	return tt.nlAppendInOrder(make([]T, 0, tt.length)), tt.cmp
}

// nlAppendInOrder appends the elements in in-order sequence to dst and
// returns it; the walk uses a fixed-size stack, so the only allocation is
// dst's growth (none when dst has room for Len elements).  The caller
// must hold the lock.
func (tt *AvlTree[T]) nlAppendInOrder(dst []T) []T {
	var stk [maxAvlHeight]*AvlTreeElement[T]
	sp := 0
	n := tt.root
	for {
		for ; n != nil; n = n.left {
			stk[sp] = n
			sp++
		}
		if sp == 0 {
			return dst
		}
		sp--
		n = stk[sp]
		dst = append(dst, n.data)
		n = n.right
	}
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
// zero-value tt adopts yy's.  When the snapshot is ascending under that
// function (always, unless tt has a different comparator or yy was
// Reverse()d) the copy is built directly as a perfectly balanced tree
// (see setop.go).
// Complexity is O(n) (O(n log₂ n) when yy is not ascending under tt's
// comparison function).
func (tt *AvlTree[T]) Copy(yy *AvlTree[T]) {
	if tt == nil {
		return
	}
	// The snapshot goes straight into the result's slab, so it IS the new
	// tree's storage once linked.
	slab, yCmp := yy.nodeSnapshot()
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	tt.adoptCmp(yCmp, nil)
	if tt.cmp == nil {
		return // no ordering available: yy cannot have had any data
	}
	for i := 1; i < len(slab); i++ {
		if tt.cmp(slab[i-1].data, slab[i].data) >= 0 {
			// Not ascending under tt's ordering: insert one by one.
			for j := range slab {
				tt.nlInsert(slab[j].data)
			}
			return
		}
	}
	tt.length = len(slab)
	tt.root = linkBalanced(slab)
}

// nodeSnapshot returns, read under the read lock, a fresh slab of Len
// unlinked nodes whose data fields hold the elements in in-order sequence
// (nil for a nil or empty tree) together with the tree's comparison
// function — snapshot's node-slab form, used by Copy.  The caller must
// NOT hold any lock.
// Complexity is O(n).
func (tt *AvlTree[T]) nodeSnapshot() (slab []AvlTreeElement[T], cmp func(a, b T) int) {
	if tt == nil {
		return nil, nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.root == nil {
		return nil, tt.cmp
	}
	slab = make([]AvlTreeElement[T], tt.length)
	var stk [maxAvlHeight]*AvlTreeElement[T]
	sp, k := 0, 0
	n := tt.root
	for {
		for ; n != nil; n = n.left {
			stk[sp] = n
			sp++
		}
		if sp == 0 {
			return slab[:k], tt.cmp
		}
		sp--
		n = stk[sp]
		slab[k].data = n.data
		k++
		n = n.right
	}
}

// Union is a set union, tt = yy union zz.  If an item is in both yy and zz
// the one from zz is kept.  The sources are snapshotted under their own
// read locks before tt's write lock is taken, so yy and zz may alias tt or
// each other.  The result is ordered by tt's comparison function; a
// zero-value tt adopts yy's (then zz's).
//
// The two snapshots are merged in one linear pass and the result is built
// directly as a perfectly balanced tree (see setop.go).
// Complexity is O(n+m) (O((n+m) log₂(n+m)) when an operand is not
// ascending under tt's comparison function).
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
	tt.setOpBuild(a, b, opUnion)
}

// Minus is a set minus, tt = yy - zz.  The sources are snapshotted under
// their own read locks before tt's write lock is taken, so yy and zz may
// alias tt or each other.  The result is ordered by tt's comparison
// function; a zero-value tt adopts yy's (then zz's).
//
// The two snapshots are merged in one linear pass and the result is built
// directly as a perfectly balanced tree (see setop.go).
// Complexity is O(n+m) (O((n+m) log₂(n+m)) when an operand is not
// ascending under tt's comparison function).
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
	tt.setOpBuild(a, b, opMinus)
}

// Intersect is a set intersection, tt = yy intersect zz.  If an item is in
// both yy and zz the one from yy is kept.  The sources are snapshotted
// under their own read locks before tt's write lock is taken, so yy and zz
// may alias tt or each other.  The result is ordered by tt's comparison
// function; a zero-value tt adopts yy's (then zz's).
//
// The two snapshots are merged in one linear pass and the result is built
// directly as a perfectly balanced tree (see setop.go).
// Complexity is O(n+m) (O((n+m) log₂(n+m)) when an operand is not
// ascending under tt's comparison function).
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
	tt.setOpBuild(a, b, opIntersect)
}

// SymmetricDifference is a set symmetric difference, tt = (yy union zz)
// minus (yy intersect zz) — every element in exactly one of the two
// operands.  The sources are snapshotted under their own read locks
// before tt's write lock is taken, so yy and zz may alias tt or each
// other.  The result is ordered by tt's comparison function; a zero-value
// tt adopts yy's (then zz's).
//
// Both snapshots are in-order (sorted), so a merge walk emits the
// elements present in exactly one operand without any searching, and the
// result is built directly as a perfectly balanced tree (see setop.go).
// Complexity is O(n+m) (O((n+m) log₂(n+m)) when an operand is not
// ascending under tt's comparison function).
func (tt *AvlTree[T]) SymmetricDifference(yy, zz *AvlTree[T]) {
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
	tt.setOpBuild(a, b, opSymDiff)
}
