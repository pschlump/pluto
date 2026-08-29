/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package splay_tree implements a generic splay tree — a self-adjusting
// binary search tree (Sleator and Tarjan) that uses top-down splaying as
// presented in Sedgwick's "Algorithms in C".  Every access — Insert,
// Delete, and even reads like Search and FindMin — splays the accessed
// node (or, on a miss, the last node visited) up to the root of the tree,
// so recently accessed elements are cheap to reach again.
//
// Elements are stored and returned by value and ordering is a direct
// function call, so element data is never boxed into an interface and
// never unboxed with a type assertion.
//
// Trees of naturally ordered key types (all integers, floats and strings)
// are created with NewSplayTree, which orders elements with the built-in
// < and > operators of the key type.  Trees of any other type — including
// structs ordered by a single field — are created with NewSplayTreeFunc,
// which takes a caller supplied comparison function; the element type does
// not have to implement any interface.
//
// Basic operations on a Splay Tree:
//
//	Insert — create a new element in the tree; a duplicate replaces the existing element.
//	Delete — delete a specified element from the tree (elements can be found via Search).
//	Search — return the given element from the tree; the found node is splayed to the root.
//	IsEmpty — report whether the tree is empty.
//	Len / Length — number of elements in the tree; 0 is an empty tree.
//	Truncate — delete all the nodes in the tree.
//	FindMin / FindMax — return the smallest / largest element in the tree.
//	DeleteAtHead — delete the smallest element of the tree.
//	DeleteAtTail — delete the largest element of the tree.
//	Depth — number of levels in the deepest part of the tree.
//	All / Backward — range-over-func iterators (in-order and reverse in-order).
//
// Insert, Delete, Search, FindMin, FindMax, DeleteAtHead and DeleteAtTail
// are O(log₂ n) amortized: any single operation can be O(n) in the worst
// case, but any sequence of m operations on a tree of n elements costs
// O(m log₂ n) overall.
//
// Every access mutates the tree: Search, FindMin and FindMax are NOT pure
// reads — each one restructures the tree by splaying a node to the root
// (on a miss, the last node visited is splayed).  The elements and their
// order never change, only the shape.  This is why a read on a nil or
// zero-value tree must be answered without consulting a comparison
// function at all.
//
// A SplayTree is created with NewSplayTree or NewSplayTreeFunc.  A nil
// *SplayTree and the zero value both behave as an empty tree for every
// operation except Insert: Search finds nothing, Delete, DeleteAtHead and
// DeleteAtTail return false, FindMin and FindMax report not-found, Len
// and Depth are 0, and the iterators visit nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewSplayTreeFunc(nil)          — nil comparison function, caught at construction.
//	Insert on a nil tree           — a nil tree cannot store an element.
//	Insert on a zero-value tree    — no comparison function; the message names the constructors.
//
// SplayTree is not safe for concurrent use — not even for concurrent
// reads, because every read mutates the tree.
package splay_tree

import (
	"cmp"
	"fmt"
	"io"
	"strings"
)

// SplayTreeElement is a single node of a SplayTree.
type SplayTreeElement[T any] struct {
	data        T
	left, right *SplayTreeElement[T]
}

// SplayTree is a generic splay tree.  Use NewSplayTree for naturally
// ordered key types (numbers, strings) or NewSplayTreeFunc for a caller
// supplied comparison function.  The zero value is an empty tree.
type SplayTree[T any] struct {
	root   *SplayTreeElement[T]
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
// strings).  It is the comparison function installed by NewSplayTree and
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

// NewSplayTree creates a new SplayTree for any naturally ordered key
// type (all integers, floats and strings — cmp.Ordered).  Ordering uses
// the built-in < and > operators of T; no interface and no boxing is
// involved.
// Complexity is O(1).
func NewSplayTree[T cmp.Ordered]() *SplayTree[T] {
	return &SplayTree[T]{cmp: Compare[T]}
}

// NewSplayTreeFunc creates a new SplayTree that orders elements with the
// caller supplied comparison function fx.  fx must return a negative value
// if a sorts before b, 0 if the two are duplicates and a positive value if
// a sorts after b, and must order elements consistently.  This lets any
// type — for example a struct ordered by one of its fields — be stored
// without implementing any interface.
// Complexity is O(1).
func NewSplayTreeFunc[T any](fx func(a, b T) int) *SplayTree[T] {
	if fx == nil {
		panic("splay_tree: NewSplayTreeFunc called with a nil comparison function")
	}
	return &SplayTree[T]{cmp: fx}
}

// compare orders a and b, guarding against a zero-value tree that was not
// created by one of the constructors.
func (tt *SplayTree[T]) compare(a, b T) int {
	if tt.cmp == nil {
		panic("splay_tree: no comparison function (create the tree with NewSplayTree or NewSplayTreeFunc)")
	}
	return tt.cmp(a, b)
}

// -------------------------------------------------------------------------------------------------------

// GetData returns the data stored in this element.
// Complexity is O(1).
func (ee *SplayTreeElement[T]) GetData() T {
	return ee.data
}

// SetData replaces the data stored in this element.  Calling it on a node
// that is inside a tree can break the tree's ordering invariant; it is
// intended for standalone elements.
// Complexity is O(1).
func (ee *SplayTreeElement[T]) SetData(x T) {
	ee.data = x
}

// -------------------------------------------------------------------------------------------------------

// splay brings the node matching find (or the last node visited on a miss)
// to the root of the tree rooted at root, returning the new root and the
// comparison of find against the new root's data (0 means found).
//
// This is the Sleator-Tarjan top-down splay as presented in Sedgwick's
// "Algorithms in C": as the search descends, the tree is split into three
// parts — a left tree holding the keys known to be smaller than find, a
// right tree holding the keys known to be larger, and the middle subtree
// still being searched — with zig-zig pairs rotated before each link.
// The three parts are reassembled around the final node.
// root must not be nil and the tree must have a comparison function.
func (tt *SplayTree[T]) splay(root *SplayTreeElement[T], find T) (*SplayTreeElement[T], int) {
	var header SplayTreeElement[T] // dummy node; its children head the left and right trees
	l, r := &header, &header       // bottoms of the left and right trees
	for {
		c := tt.compare(find, root.data)
		if c < 0 {
			if root.left == nil {
				break
			}
			if tt.compare(find, root.left.data) < 0 {
				// zig-zig: rotate right, then continue linking below.
				x := root.left
				root.left = x.right
				x.right = root
				root = x
				if root.left == nil {
					break
				}
			}
			// Link right: root and its right subtree join the right tree.
			r.left = root
			r = root
			root = root.left
		} else if c > 0 {
			if root.right == nil {
				break
			}
			if tt.compare(find, root.right.data) > 0 {
				// zag-zag: rotate left, then continue linking below.
				x := root.right
				root.right = x.left
				x.left = root
				root = x
				if root.right == nil {
					break
				}
			}
			// Link left: root and its left subtree join the left tree.
			l.right = root
			l = root
			root = root.right
		} else {
			break
		}
	}
	c := tt.compare(find, root.data)
	// Reassemble: root's subtrees close the left and right trees, and the
	// two trees become root's new subtrees.
	l.right = root.left
	r.left = root.right
	root.left = header.right
	root.right = header.left
	return root, c
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty returns true if the tree is empty.
// Complexity is O(1).
func (tt *SplayTree[T]) IsEmpty() bool {
	return tt == nil || tt.root == nil
}

// Truncate removes all data from the tree.  The comparison function is
// kept, so the tree remains usable and can simply be refilled.
// Complexity is O(1).
func (tt *SplayTree[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.root = nil
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an
// existing item the new item will replace the existing one and false is
// returned; true is returned when a new node was added.  The inserted (or
// replaced) node is splayed to the root of the tree.
// Insert panics on a nil tree or on a zero-value tree (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty tree.
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) Insert(item T) (vv bool) {
	if tt == nil {
		panic("splay_tree: Insert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("splay_tree: Insert called on a tree with no comparison function (create the tree with NewSplayTree or NewSplayTreeFunc)")
	}
	node := &SplayTreeElement[T]{data: item}
	if tt.IsEmpty() {
		tt.root = node
		tt.length = 1
		return true
	}

	// Splay for the item: afterwards the root holds either the duplicate
	// (replace it in place) or the node nearest to where item belongs
	// (split the tree around it and link item in as the new root).
	root, c := tt.splay(tt.root, item)
	if c == 0 {
		root.data = item
		tt.root = root
		return false
	}
	if c < 0 {
		// root > item: item takes root's left subtree, root goes right.
		node.left = root.left
		node.right = root
		root.left = nil
	} else {
		// root < item: item takes root's right subtree, root goes left.
		node.right = root.right
		node.left = root
		root.right = nil
	}
	tt.root = node
	tt.length++
	return true
}

// Len returns the number of elements in the tree.
// Complexity is O(1).
func (tt *SplayTree[T]) Len() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Length is an alias for Len; it returns the number of elements in the tree.
// Complexity is O(1).
func (tt *SplayTree[T]) Length() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Search will walk the tree looking for `find` and return the found item
// from the tree.  If it is not found then false is returned.  `find` only
// needs the fields that the tree's comparison function reads: a probe with
// just the key fields set finds the element with the full data.
// Search mutates the tree: the found node — or, on a miss, the last node
// visited — is splayed to the root, so a repeated Search for the same key
// is O(1).
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) Search(find T) (item T, found bool) {
	if tt == nil || tt.root == nil {
		return
	}

	root, c := tt.splay(tt.root, find)
	tt.root = root
	if c != 0 {
		return
	}
	return root.data, true
}

// Dump writes one line per element to `fo`: an in-order traversal indented
// by depth, including the left/right child pointers.  It is a debugging
// aid; use All or Backward to process the data.  Because every access
// splays the tree, the shape Dump prints is deterministic only for a fixed
// history of operations.
// Complexity is O(n).
func (tt *SplayTree[T]) Dump(fo io.Writer) {
	if tt == nil {
		return
	}
	k := tt.Depth() * 4
	var inorderTraversal func(cur *SplayTreeElement[T], n int) bool
	inorderTraversal = func(cur *SplayTreeElement[T], n int) bool {
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
// the fields that the comparison function reads.  The victim is splayed to
// the root and its two subtrees are joined (the maximum of the left
// subtree is splayed to its root — it has no right child — and the right
// subtree is linked under it).  On a miss the tree is still splayed but
// left otherwise unchanged.
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) Delete(find T) (found bool) {
	if tt == nil || tt.root == nil {
		return false
	}

	root, c := tt.splay(tt.root, find)
	if c != 0 {
		tt.root = root
		return false
	}
	tt.length--
	if root.left == nil {
		tt.root = root.right
		return true
	}
	// Join the subtrees: splaying the left subtree for find brings its
	// maximum (which has no right child) to its root.
	left, _ := tt.splay(root.left, find)
	left.right = root.right
	tt.root = left
	return true
}

// FindMin returns the smallest element in the tree, or false if the tree
// is empty.  The minimum node is splayed to the root.
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) FindMin() (item T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}

	cur := tt.root
	for cur.left != nil {
		cur = cur.left
	}
	root, _ := tt.splay(tt.root, cur.data)
	tt.root = root
	return root.data, true
}

// FindMax returns the largest element in the tree, or false if the tree is
// empty.  The maximum node is splayed to the root.
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) FindMax() (item T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}

	cur := tt.root
	for cur.right != nil {
		cur = cur.right
	}
	root, _ := tt.splay(tt.root, cur.data)
	tt.root = root
	return root.data, true
}

// DeleteAtHead removes the smallest element of the tree, returning true if
// an element was removed.  The minimum is splayed to the root (leaving it
// with no left child) and its right subtree takes over.
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) DeleteAtHead() (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	cur := tt.root
	for cur.left != nil {
		cur = cur.left
	}
	root, _ := tt.splay(tt.root, cur.data) // minimum at root; root.left is nil
	tt.root = root.right
	tt.length--
	return true
}

// DeleteAtTail removes the largest element of the tree, returning true if
// an element was removed.  The maximum is splayed to the root (leaving it
// with no right child) and its left subtree takes over.
// Complexity is O(log₂ n) amortized.
func (tt *SplayTree[T]) DeleteAtTail() (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	cur := tt.root
	for cur.right != nil {
		cur = cur.right
	}
	root, _ := tt.splay(tt.root, cur.data) // maximum at root; root.right is nil
	tt.root = root.left
	tt.length--
	return true
}

// Depth returns the number of levels in the deepest part of the tree.
// An empty tree has depth 0; a tree with only a root has depth 1.
// Complexity is O(n).
func (tt *SplayTree[T]) Depth() int {
	if tt == nil {
		return 0
	}

	var depth func(cur *SplayTreeElement[T]) int
	depth = func(cur *SplayTreeElement[T]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depth(cur.left), depth(cur.right))
	}
	return depth(tt.root)
}
