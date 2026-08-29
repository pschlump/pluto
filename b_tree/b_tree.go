/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package b_tree implements a generic B-tree, the balanced
// external-searching tree of Sedgwick (Algorithms, B-Trees; Bayer &
// McCreight, 1972).
//
// A B-tree of order M stores up to M-1 keys per node and up to M children
// per internal node; every node other than the root holds at least
// ceil(M/2)-1 keys, and all leaves are at the same depth.  Insert splits
// overflowing nodes and pushes the median key up; Delete merges or
// redistributes nodes to restore the minimum occupancy.  Insert, Delete
// and Search are therefore O(log n) in the worst case, regardless of the
// order of operations — unlike the unbalanced binary_tree package.
//
// Elements are stored and returned by value and ordering is a direct
// function call, so element data is never boxed into an interface and
// never unboxed with a type assertion.  The keys are the elements
// themselves: the tree is an ordered set, not a key/value map.
//
// Trees of naturally ordered key types (all integers, floats and strings)
// are created with NewBTree, which orders elements with the built-in <
// and > operators of the key type.  Trees of any other type — including
// structs ordered by a single field — are created with NewBTreeFunc,
// which takes a caller supplied comparison function; the element type
// does not have to implement any interface.
//
// Basic operations on a BTree:
//
//	Insert — add an element; a duplicate replaces the existing element.
//	Delete — remove the element equal to the probe.
//	Search — return the stored element equal to the probe.
//	FindMin / FindMax — return the smallest / largest element.
//	DeleteAtHead / DeleteAtTail — remove the smallest / largest element.
//	IsEmpty / Len / Length / Depth — size and height queries.
//	Truncate — delete all the elements, keeping the comparison function.
//	Dump — write the tree structure for debugging.
//	All / Backward — range-over-func iterators (ascending / descending).
//
// A nil *BTree and the zero value both behave as an empty tree for every
// operation except Insert: Search finds nothing, Delete, DeleteAtHead and
// DeleteAtTail return false, FindMin and FindMax report not-found, Len
// and Depth are 0, and the iterators yield nothing.
//
// The package panics in exactly four situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewBTree / NewBTreeFunc with order < 3  — a node needs room for at least 2 keys and 3 children.
//	NewBTreeFunc with a nil comparison      — nil comparison function, caught at construction.
//	Insert on a nil tree                    — a nil tree cannot store an element.
//	Insert on a zero-value tree             — no comparison function; the message names the constructors.
//
// BTree is not safe for concurrent use.
package b_tree

import (
	"cmp"
	"fmt"
	"io"
	"strings"
)

// BTreeNode is a node of a BTree.  A leaf has a nil children slice; an
// internal node with k keys has exactly k+1 children.
type BTreeNode[T any] struct {
	keys     []T
	children []*BTreeNode[T]
}

// BTree is a generic B-tree of the given order (the maximum number of
// children per node).  Use NewBTree for naturally ordered key types
// (numbers, strings) or NewBTreeFunc for a caller supplied comparison
// function.  The zero value is an empty tree.
type BTree[T any] struct {
	root   *BTreeNode[T]
	order  int // maximum number of children per node (Sedgwick's M)
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
// strings).  It is the comparison function installed by NewBTree and is
// handy for building custom comparison functions.
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

// NewBTree creates a new BTree of the given order for any naturally
// ordered key type (all integers, floats and strings — cmp.Ordered).
// Ordering uses the built-in < and > operators of T; no interface and no
// boxing is involved.  order is the maximum number of children per node
// and must be at least 3.
// Complexity is O(1).
func NewBTree[T cmp.Ordered](order int) *BTree[T] {
	if order < 3 {
		panic("b_tree: NewBTree called with order < 3 (order is the maximum number of children per node and must be at least 3)")
	}
	return &BTree[T]{order: order, cmp: Compare[T]}
}

// NewBTreeFunc creates a new BTree of the given order that orders
// elements with the caller supplied comparison function fx.  fx must
// return a negative value if a sorts before b, 0 if the two are
// duplicates and a positive value if a sorts after b, and must order
// elements consistently.  This lets any type — for example a struct
// ordered by one of its fields — be stored without implementing any
// interface.  order is the maximum number of children per node and must
// be at least 3.
// Complexity is O(1).
func NewBTreeFunc[T any](order int, fx func(a, b T) int) *BTree[T] {
	if order < 3 {
		panic("b_tree: NewBTreeFunc called with order < 3 (order is the maximum number of children per node and must be at least 3)")
	}
	if fx == nil {
		panic("b_tree: NewBTreeFunc called with a nil comparison function")
	}
	return &BTree[T]{order: order, cmp: fx}
}

// compare orders a and b, guarding against a zero-value tree that was not
// created by one of the constructors.
func (tt *BTree[T]) compare(a, b T) int {
	if tt.cmp == nil {
		panic("b_tree: no comparison function (create the tree with NewBTree or NewBTreeFunc)")
	}
	return tt.cmp(a, b)
}

// minKeys returns the minimum number of keys a non-root node may hold,
// ceil(order/2)-1.
func (tt *BTree[T]) minKeys() int {
	return (tt.order+1)/2 - 1
}

// findKey returns the index of the first key of x that is not less than
// item — len(x.keys) if every key is less.
// Complexity is O(log₂ order).
func (tt *BTree[T]) findKey(x *BTreeNode[T], item T) int {
	lo, hi := 0, len(x.keys)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if tt.cmp(x.keys[mid], item) < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty returns true if the tree is empty.
// Complexity is O(1).
func (tt *BTree[T]) IsEmpty() bool {
	return tt == nil || tt.root == nil
}

// Truncate removes all data from the tree.  The comparison function and
// the order are kept, so the tree remains usable and can simply be
// refilled.
// Complexity is O(1).
func (tt *BTree[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.root = nil
	tt.length = 0
}

// Len returns the number of elements in the tree.
// Complexity is O(1).
func (tt *BTree[T]) Len() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Length is an alias for Len; it returns the number of elements in the
// tree.
// Complexity is O(1).
func (tt *BTree[T]) Length() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Depth returns the height of the tree: the number of nodes on a
// root-to-leaf path (all leaves are at the same depth in a B-tree).  An
// empty tree has depth 0, a single node depth 1.
// Complexity is O(log n).
func (tt *BTree[T]) Depth() int {
	if tt == nil {
		return 0
	}
	d := 0
	for n := tt.root; n != nil; {
		d++
		if n.children == nil {
			break
		}
		n = n.children[0]
	}
	return d
}

// Search walks the tree looking for `find` and returns the found item
// from the tree.  If it is not found then false is returned.  `find` only
// needs the fields that the tree's comparison function reads: a probe
// with just the key fields set finds the element with the full data.
// Complexity is O(log n).
func (tt *BTree[T]) Search(find T) (item T, found bool) {
	if tt == nil || tt.root == nil {
		return
	}
	cur := tt.root
	for {
		i := tt.findKey(cur, find)
		if i < len(cur.keys) && tt.compare(find, cur.keys[i]) == 0 {
			return cur.keys[i], true
		}
		if cur.children == nil {
			return
		}
		cur = cur.children[i]
	}
}

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one in place and
// false is returned; true is returned when a new element was added.
// Insert panics on a nil tree or on a zero-value tree (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty tree.
// Complexity is O(log n).
func (tt *BTree[T]) Insert(item T) (added bool) {
	if tt == nil {
		panic("b_tree: Insert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("b_tree: Insert called on a tree with no comparison function (create the tree with NewBTree or NewBTreeFunc)")
	}
	if tt.root == nil {
		tt.root = &BTreeNode[T]{keys: []T{item}}
		tt.length = 1
		return true
	}
	median, right, added, didSplit := tt.insertRec(tt.root, item)
	if didSplit {
		tt.root = &BTreeNode[T]{
			keys:     []T{median},
			children: []*BTreeNode[T]{tt.root, right},
		}
	}
	return added
}

// insertRec inserts item into the subtree rooted at x.  If x overflows
// (it ends up with order keys) it is split around its median: x keeps the
// keys below the median and the median and a new node holding the keys
// above it are returned with didSplit true.  added reports whether the
// item was a new element (false on a duplicate replace).
func (tt *BTree[T]) insertRec(x *BTreeNode[T], item T) (median T, right *BTreeNode[T], added, didSplit bool) {
	i := tt.findKey(x, item)
	if i < len(x.keys) && tt.cmp(x.keys[i], item) == 0 {
		// Duplicate: replace the stored value in place; the tree shape
		// does not change.
		x.keys[i] = item
		return median, nil, false, false
	}
	if x.children == nil {
		// Leaf: insert at position i.
		x.keys = append(x.keys, item)
		copy(x.keys[i+1:], x.keys[i:])
		x.keys[i] = item
		tt.length++
		added = true
	} else {
		m, r, a, split := tt.insertRec(x.children[i], item)
		added = a
		if split {
			// Pull the median of the split child up into x at position i.
			x.keys = append(x.keys, m)
			copy(x.keys[i+1:], x.keys[i:])
			x.keys[i] = m
			x.children = append(x.children, nil)
			copy(x.children[i+2:], x.children[i+1:])
			x.children[i+1] = r
		}
	}
	if len(x.keys) < tt.order {
		return median, nil, added, false
	}
	// x overflows with order keys: split around the middle key.
	mid := tt.order / 2
	median = x.keys[mid]
	right = &BTreeNode[T]{}
	right.keys = append(right.keys, x.keys[mid+1:]...)
	if x.children != nil {
		right.children = append(right.children, x.children[mid+1:]...)
		x.children = x.children[:mid+1]
	}
	x.keys = x.keys[:mid]
	return median, right, added, true
}

// Delete removes the element matching `find` from the tree.  True is
// returned if an element was removed, false otherwise.  As with Search,
// `find` only needs the fields that the comparison function reads.
// Complexity is O(log n).
func (tt *BTree[T]) Delete(find T) (found bool) {
	if tt == nil || tt.root == nil {
		return false
	}
	if !tt.deleteRec(tt.root, find) {
		return false
	}
	tt.length--
	tt.shrinkRoot()
	return true
}

// deleteRec removes the element equal to find from the subtree rooted at
// x.  On the way back up the recursion repairs any node that fell below
// the minimum occupancy by borrowing from a sibling or merging.
func (tt *BTree[T]) deleteRec(x *BTreeNode[T], find T) (found bool) {
	i := tt.findKey(x, find)
	if i < len(x.keys) && tt.cmp(x.keys[i], find) == 0 {
		if x.children == nil {
			copy(x.keys[i:], x.keys[i+1:])
			x.keys = x.keys[:len(x.keys)-1]
			return true
		}
		// Internal node: replace the key with its in-order predecessor
		// (the maximum key of the subtree to its left) and delete that.
		pred := tt.deleteMaxRec(x.children[i])
		x.keys[i] = pred
		tt.fixUnderflow(x, i)
		return true
	}
	if x.children == nil {
		return false
	}
	if !tt.deleteRec(x.children[i], find) {
		return false
	}
	tt.fixUnderflow(x, i)
	return true
}

// deleteMinRec removes and returns the smallest key of the subtree rooted
// at x, which must be non-empty.  Nodes that fall below the minimum
// occupancy are repaired on the way back up; x itself may end up with
// minKeys()-1 keys, which the caller (or shrinkRoot, at the top) fixes.
func (tt *BTree[T]) deleteMinRec(x *BTreeNode[T]) (min T) {
	if x.children == nil {
		min = x.keys[0]
		copy(x.keys[0:], x.keys[1:])
		x.keys = x.keys[:len(x.keys)-1]
		return min
	}
	min = tt.deleteMinRec(x.children[0])
	tt.fixUnderflow(x, 0)
	return min
}

// deleteMaxRec removes and returns the largest key of the subtree rooted
// at x, which must be non-empty.  As with deleteMinRec the last node may
// underflow for the caller to fix.
func (tt *BTree[T]) deleteMaxRec(x *BTreeNode[T]) (max T) {
	if x.children == nil {
		max = x.keys[len(x.keys)-1]
		x.keys = x.keys[:len(x.keys)-1]
		return max
	}
	max = tt.deleteMaxRec(x.children[len(x.children)-1])
	tt.fixUnderflow(x, len(x.children)-1)
	return max
}

// fixUnderflow repairs x.children[i] if it fell below the minimum key
// count: borrow a key from a sibling that has keys to spare, otherwise
// merge the child with a sibling through the separating key of x.
func (tt *BTree[T]) fixUnderflow(x *BTreeNode[T], i int) {
	child := x.children[i]
	if len(child.keys) >= tt.minKeys() {
		return
	}
	if i > 0 && len(x.children[i-1].keys) > tt.minKeys() {
		// Borrow from the left sibling: the separator moves down to the
		// front of child and the sibling's last key becomes the separator.
		left := x.children[i-1]
		var zero T
		child.keys = append(child.keys, zero) // grow by one (value overwritten below)
		copy(child.keys[1:], child.keys)
		child.keys[0] = x.keys[i-1]
		x.keys[i-1] = left.keys[len(left.keys)-1]
		left.keys = left.keys[:len(left.keys)-1]
		if left.children != nil {
			child.children = append(child.children, nil)
			copy(child.children[1:], child.children)
			child.children[0] = left.children[len(left.children)-1]
			left.children = left.children[:len(left.children)-1]
		}
		return
	}
	if i < len(x.children)-1 && len(x.children[i+1].keys) > tt.minKeys() {
		// Borrow from the right sibling: the separator moves down to the
		// end of child and the sibling's first key becomes the separator.
		right := x.children[i+1]
		child.keys = append(child.keys, x.keys[i])
		x.keys[i] = right.keys[0]
		copy(right.keys[0:], right.keys[1:])
		right.keys = right.keys[:len(right.keys)-1]
		if right.children != nil {
			child.children = append(child.children, right.children[0])
			copy(right.children[0:], right.children[1:])
			right.children[len(right.children)-1] = nil
			right.children = right.children[:len(right.children)-1]
		}
		return
	}
	// No sibling can spare a key: merge the child with a sibling through
	// the separating key of x.
	if i > 0 {
		tt.mergeChildren(x, i-1)
	} else {
		tt.mergeChildren(x, i)
	}
}

// mergeChildren merges x.children[i+1] into x.children[i], with the
// separating key x.keys[i] between them, and removes both from x.
func (tt *BTree[T]) mergeChildren(x *BTreeNode[T], i int) {
	a, b := x.children[i], x.children[i+1]
	a.keys = append(a.keys, x.keys[i])
	a.keys = append(a.keys, b.keys...)
	if b.children != nil {
		a.children = append(a.children, b.children...)
	}
	copy(x.keys[i:], x.keys[i+1:])
	x.keys = x.keys[:len(x.keys)-1]
	copy(x.children[i+1:], x.children[i+2:])
	x.children[len(x.children)-1] = nil
	x.children = x.children[:len(x.children)-1]
}

// shrinkRoot collapses an emptied root: a leaf root makes the tree empty,
// an internal root is replaced by its only remaining child.
func (tt *BTree[T]) shrinkRoot() {
	if tt.root == nil || len(tt.root.keys) > 0 {
		return
	}
	if tt.root.children == nil {
		tt.root = nil
	} else {
		tt.root = tt.root.children[0]
	}
}

// FindMin returns the smallest value in the tree, or false if the tree is
// empty.
// Complexity is O(log n).
func (tt *BTree[T]) FindMin() (item T, found bool) {
	if tt == nil || tt.root == nil {
		return
	}
	cur := tt.root
	for cur.children != nil {
		cur = cur.children[0]
	}
	return cur.keys[0], true
}

// FindMax returns the largest value in the tree, or false if the tree is
// empty.
// Complexity is O(log n).
func (tt *BTree[T]) FindMax() (item T, found bool) {
	if tt == nil || tt.root == nil {
		return
	}
	cur := tt.root
	for cur.children != nil {
		cur = cur.children[len(cur.children)-1]
	}
	return cur.keys[len(cur.keys)-1], true
}

// DeleteAtHead removes the smallest element of the tree with a single
// descent down the left spine, returning false if the tree is empty.
// Complexity is O(log n).
func (tt *BTree[T]) DeleteAtHead() (found bool) {
	if tt == nil || tt.root == nil {
		return false
	}
	tt.deleteMinRec(tt.root)
	tt.length--
	tt.shrinkRoot()
	return true
}

// DeleteAtTail removes the largest element of the tree with a single
// descent down the right spine, returning false if the tree is empty.
// Complexity is O(log n).
func (tt *BTree[T]) DeleteAtTail() (found bool) {
	if tt == nil || tt.root == nil {
		return false
	}
	tt.deleteMaxRec(tt.root)
	tt.length--
	tt.shrinkRoot()
	return true
}

// Dump writes the structure of the tree to `fo`: one line per node with
// the node's keys, indented by depth.  It is a debugging aid; use All or
// Backward to process the data.
// Complexity is O(n).
func (tt *BTree[T]) Dump(fo io.Writer) {
	if tt == nil {
		return
	}
	var dump func(n *BTreeNode[T], depth int) bool
	dump = func(n *BTreeNode[T], depth int) bool {
		if _, err := fmt.Fprintf(fo, "%s%v\n", strings.Repeat("  ", depth), n.keys); err != nil {
			return false // stop the dump on write error
		}
		for _, c := range n.children {
			if !dump(c, depth+1) {
				return false
			}
		}
		return true
	}
	if tt.root != nil {
		dump(tt.root, 0)
	}
}
