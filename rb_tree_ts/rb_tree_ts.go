/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

/*
Package rb_tree_ts implements a generic red-black self-balancing binary
search tree that is safe for concurrent use.  It is the thread-safe twin
of github.com/pschlump/charon/rb_tree — the same API, guarded by a
sync.RWMutex.

A red-black tree keeps every root-to-leaf path within a factor of two of
the shortest one, so Insert, Delete and Search are O(log₂ n) in the worst
case.  Basic operations:

  - Insert   — add an item; an item that compares equal to an existing one replaces it.	O(log₂ n)
  - Search   — find an item; reports not-found if not present.						O(log₂ n)
  - Delete   — remove an item; returns false if not present.							O(log₂ n)
  - IsEmpty  — true if the tree has no nodes.											O(1)
  - Len / Length — number of nodes in the tree.											O(1)
  - FindMin / FindMax — smallest / largest item.										O(log₂ n)
  - DeleteAtHead / DeleteAtTail — remove smallest / largest item.						O(log₂ n)
  - Depth    — number of nodes on the longest root-to-leaf path.						O(log₂ n), maintained by balancing
  - Truncate — remove all nodes.														O(1)
  - Dump     — write an indented picture of the tree for debugging.					O(n)
  - All / Backward — range-over-func iterators over a snapshot of the tree.			O(n)

Concurrency model:

	Reads (Search, FindMin, FindMax, Depth, Len, Length, IsEmpty) take the
	read lock and release it before returning, so they run in parallel with
	each other.
	Writes (Insert, Delete, DeleteAtHead, DeleteAtTail, Truncate) take the
	write lock.
	All and Backward operate on a snapshot taken when they are called (one
	O(n) copy, under the read lock), so they are safe to use concurrently
	with any tree operation — including mutating the tree from inside the
	loop — and never observe later modifications.
	Dump holds the read lock for the whole dump, so the writer must not call
	methods on the same tree.

Elements are stored and returned by value and ordering is a direct
function call, so element data is never boxed into an interface and
never unboxed with a type assertion.  Trees of naturally ordered key
types (all integers, floats and strings) are created with NewRbTree,
which orders elements with the built-in < and > operators of the key
type; trees of any other type — including structs ordered by a single
field — are created with NewRbTreeFunc, which takes a caller supplied
comparison function; the element type does not have to implement any
interface.

A nil *RbTree and the zero value both behave as an empty tree for every
operation except Insert: Search finds nothing, Delete, DeleteAtHead and
DeleteAtTail return false, FindMin and FindMax report not-found, Len and
Depth are 0, and the iterators visit nothing.

The package panics in exactly three situations, all programmer errors
that cannot be handled where they occur:

	NewRbTreeFunc(nil)            — nil comparison function, caught at construction.
	Insert on a nil tree          — a nil tree cannot store an element.
	Insert on a zero-value tree   — no comparison function; the message names the constructors.
*/
package rb_tree_ts

import (
	"cmp"
	"fmt"
	"io"
	"strings"
	"sync"
)

// RbTreeNode is a single node of the tree.  It holds the item data, the
// left/right/parent links and the node color.  A nil child is treated as a
// black leaf.
type RbTreeNode[T any] struct {
	data        T
	red         bool // Red nodes may not have red children; nil children are black.
	left, right *RbTreeNode[T]
	parent      *RbTreeNode[T]
}

// RbTree is a generic red-black self-balancing binary search tree that is
// safe for concurrent use.  Use NewRbTree for naturally ordered key types
// (numbers, strings) or NewRbTreeFunc for a caller supplied comparison
// function.  The zero value is an empty tree.
type RbTree[T any] struct {
	root   *RbTreeNode[T]
	length int // Number of nodes in the tree
	lock   sync.RWMutex

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// -------------------------------------------------------------------------------------------------------
// Lock-free internals; the caller must hold the appropriate lock.
// -------------------------------------------------------------------------------------------------------

// isRed reports whether n is a red node; nil nodes are black.
func isRed[T any](n *RbTreeNode[T]) bool {
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

// findNode returns the node holding the item that compares equal to
// `item`, or nil if it is not present.  The caller must hold a lock.
func (tt *RbTree[T]) findNode(item T) *RbTreeNode[T] {
	cur := tt.root
	for cur != nil {
		c := tt.cmp(item, cur.data)
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

// insertFixup restores the red-black properties after inserting the red
// node z, recoloring and rotating until no red node has a red parent.  The
// caller must hold the write lock.
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

// transplant replaces the subtree rooted at u with the subtree rooted at
// v.  The caller must hold the write lock.
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

// deleteNode splices the node z out of the tree and restores the
// red-black properties.  The caller must hold the write lock.
func (tt *RbTree[T]) deleteNode(z *RbTreeNode[T]) {
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
}

// deleteFixup restores the red-black properties after splicing out a
// black node.  x is the node that moved into the spliced node's position
// (possibly nil); xParent is x's parent, which must be supplied separately
// because x may be nil.  The caller must hold the write lock.
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

// minNode returns the node holding the smallest item in the subtree rooted
// at n.
func minNode[T any](n *RbTreeNode[T]) *RbTreeNode[T] {
	for n != nil && n.left != nil {
		n = n.left
	}
	return n
}

// maxNode returns the node holding the largest item in the subtree rooted
// at n.
func maxNode[T any](n *RbTreeNode[T]) *RbTreeNode[T] {
	for n != nil && n.right != nil {
		n = n.right
	}
	return n
}

// -------------------------------------------------------------------------------------------------------
// Constructors
// -------------------------------------------------------------------------------------------------------

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewRbTree and
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

// NewRbTree creates a new RbTree for any naturally ordered key type (all
// integers, floats and strings — cmp.Ordered).  Ordering uses the built-in
// < and > operators of T; no interface and no boxing is involved.
// Complexity is O(1).
func NewRbTree[T cmp.Ordered]() *RbTree[T] {
	return &RbTree[T]{cmp: Compare[T]}
}

// NewRbTreeFunc creates a new RbTree that orders elements with the caller
// supplied comparison function fx.  fx must return a negative value if a
// sorts before b, 0 if the two are duplicates and a positive value if a
// sorts after b, and must order elements consistently.  This lets any
// type — for example a struct ordered by one of its fields — be stored
// without implementing any interface.
// Complexity is O(1).
func NewRbTreeFunc[T any](fx func(a, b T) int) *RbTree[T] {
	if fx == nil {
		panic("rb_tree_ts: NewRbTreeFunc called with a nil comparison function")
	}
	return &RbTree[T]{cmp: fx}
}

// -------------------------------------------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the tree is empty.
// Complexity is O(1).
func (tt *RbTree[T]) IsEmpty() bool {
	if tt == nil {
		return true
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.root == nil
}

// Len returns the number of elements in the tree.
// Complexity is O(1).
func (tt *RbTree[T]) Len() int {
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
func (tt *RbTree[T]) Length() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Truncate removes all data from the tree.  The comparison function is
// kept, so the tree remains usable and can simply be refilled.
// Complexity is O(1).
func (tt *RbTree[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.root = nil
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one in place and
// false is returned; true is returned when a new node was added.
// Insert panics on a nil tree or on a zero-value tree (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty tree.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) Insert(item T) (added bool) {
	if tt == nil {
		panic("rb_tree_ts: Insert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("rb_tree_ts: Insert called on a tree with no comparison function (create the tree with NewRbTree or NewRbTreeFunc)")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	// Standard BST insert; the new node starts out red.
	var parent *RbTreeNode[T]
	cur := tt.root
	for cur != nil {
		c := tt.cmp(item, cur.data)
		if c == 0 {
			cur.data = item // Duplicate: replace the stored value in place.
			return false
		}
		parent = cur
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	z := &RbTreeNode[T]{data: item, red: true, parent: parent}
	if parent == nil {
		tt.root = z
	} else if tt.cmp(item, parent.data) < 0 {
		parent.left = z
	} else {
		parent.right = z
	}
	tt.length++
	tt.insertFixup(z)
	return true
}

// Search will return the item in the tree that compares equal to `item`,
// or false if it is not present.  `item` only needs the fields that the
// tree's comparison function reads: a probe with just the key fields set
// finds the element with the full data.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) Search(item T) (rv T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if n := tt.findNode(item); n != nil {
		return n.data, true
	}
	return
}

// Delete will remove the item in the tree that compares equal to `item`.
// It returns false if the item is not present.  As with Search, `item`
// only needs the fields that the comparison function reads.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) Delete(item T) (found bool) {
	if tt == nil {
		return false
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	z := tt.findNode(item)
	if z == nil {
		return false
	}
	tt.deleteNode(z)
	return true
}

// FindMin returns the smallest item in the tree, or false if it is empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) FindMin() (rv T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if n := minNode(tt.root); n != nil {
		return n.data, true
	}
	return
}

// FindMax returns the largest item in the tree, or false if it is empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) FindMax() (rv T, found bool) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if n := maxNode(tt.root); n != nil {
		return n.data, true
	}
	return
}

// DeleteAtHead removes the smallest item in the tree.  It returns false if
// the tree is empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		return false
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.root == nil {
		return false
	}
	tt.deleteNode(minNode(tt.root))
	return true
}

// DeleteAtTail removes the largest item in the tree.  It returns false if
// the tree is empty.
// Complexity is O(log₂ n).
func (tt *RbTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		return false
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if tt.root == nil {
		return false
	}
	tt.deleteNode(maxNode(tt.root))
	return true
}

// Depth returns the number of nodes on the longest path from the root to a
// leaf.  An empty tree has depth 0, a tree with a single node has depth 1.
// Balancing keeps this at O(log₂ n).  The read lock is held for the whole
// traversal.
func (tt *RbTree[T]) Depth() (d int) {
	if tt == nil {
		return 0
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return nodeDepth(tt.root)
}

// nodeDepth returns the number of nodes on the longest path from the
// subtree rooted at cur down to a leaf.  The caller must hold a lock.
func nodeDepth[T any](cur *RbTreeNode[T]) int {
	if cur == nil {
		return 0
	}
	return 1 + max(nodeDepth(cur.left), nodeDepth(cur.right))
}

// toSlice returns a snapshot of the items of the tree in in-order (sorted)
// sequence, taken under the read lock.  A nil tree yields nil.  The caller
// must NOT hold the lock.
// Complexity is O(n).
func (tt *RbTree[T]) toSlice() []T {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()

	// Successor walk over the parent pointers: O(1) extra space.
	items := make([]T, 0, tt.length)
	for cur := minNode(tt.root); cur != nil; cur = successor(cur) {
		items = append(items, cur.data)
	}
	return items
}

// Dump writes an indented picture of the tree to `w` for debugging.  Each
// line shows a node as "data(R)" for red and "data(B)" for black; the left
// subtree is printed above the right subtree.  It is a debugging aid; use
// All or Backward to process the data.  The read lock is held for the
// whole dump, so the writer must not call methods on the same tree.
// Complexity is O(n).
func (tt *RbTree[T]) Dump(w io.Writer) {
	if tt == nil {
		_, _ = fmt.Fprintf(w, "RbTree (empty)\n")
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if tt.root == nil {
		_, _ = fmt.Fprintf(w, "RbTree (empty)\n")
		return
	}
	_, _ = fmt.Fprintf(w, "RbTree length=%d depth=%d\n", tt.length, nodeDepth(tt.root))
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
		_, _ = fmt.Fprintf(w, "%s%v(%s)\n", strings.Repeat("  ", depth), cur.data, c)
		dump(cur.right, depth+1)
	}
	dump(tt.root, 0)
}
