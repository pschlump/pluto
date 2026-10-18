/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

/*
Package interval_tree implements the interval search tree — a red-black
binary search tree keyed by closed intervals and augmented so that
intersection queries are logarithmic (CLRS §14.3; Sedgwick & Wayne
algs4 exercise 3.3.43, IntervalST.java).

Each node stores a key Interval[K] (a closed [Lo, Hi] range of any
ordered type) with a satellite value V, ordered by Lo with Hi breaking
ties, and carries one piece of augmentation: maxHi, the largest Hi in
its subtree.  An overlap query then prunes whole subtrees — a subtree
whose maxHi is below the query's Lo cannot contain an intersecting
interval — giving:

  - Insert   — add interval→value; an equal interval replaces its value.		O(log₂ n)
  - Search   — exact-match lookup by interval.						O(log₂ n)
  - Delete   — remove an interval; returns false if not present.			O(log₂ n)
  - AnyOverlap — any one stored interval that intersects the query, or false.	O(log₂ n)
  - Overlaps — every stored interval that intersects the query, ascending by Lo.	O(log₂ n + k)
  - Stab     — every stored interval containing a point (a stabbing query).		O(log₂ n + k)
  - Min / Max — interval with the smallest / largest (Lo, Hi).			O(log₂ n)
  - IsEmpty / Len / Length — size queries.						O(1)
  - Truncate — remove everything.							O(1)
  - Depth    — longest root-to-leaf path, kept at O(log₂ n) by balancing.		O(n)
  - Dump     — write an indented picture of the tree for debugging.			O(n)
  - All / Backward — range-over-func iterators in (Lo, Hi) order (see iter.go).	O(n), O(1) extra space
  - Contains / ContainsSlice / ContainsAny — exact-match membership predicate and batches.	O(log₂ n) / O(k·log₂ n)
  - DeleteSlice — batch delete by exact-match intervals.				O(k·log₂ n)
  - DeleteFunc — predicate bulk delete over (interval, value) pairs.			O(n log₂ n)
  - Slice / String / StringFunc — materialize / render the intervals, ascending.	O(n)
  - Equal / Clone — same-interval-set equality (keys only) / deep copy.		O(n log₂ n) / O(n)

Intervals are CLOSED: [1,2] and [2,3] intersect, at the point 2.  Insert
requires Lo <= Hi (a degenerate point interval Lo == Hi is legal); the
overlap and stabbing queries follow the same closed convention.

Keys and values are stored and returned by value and ordering is a
direct function call, so element data is never boxed into an interface
and never unboxed with a type assertion.  Trees of naturally ordered key
types (all integers, floats and strings) are created with NewIntervalTree,
which orders endpoints with the built-in < and > operators of the key
type; trees of any other key type are created with NewIntervalTreeFunc,
which takes a caller supplied comparison function on the endpoints.

A nil *IntervalTree and the zero value both behave as an empty tree for
every operation except Insert: Search and AnyOverlap report not-found,
Delete returns false, the iterators visit nothing and Len is 0.

The set-convenience capabilities (capabilities.go) are total functions —
none of them panics, on any receiver.  They are the map-shaped, reduced
form the tries use: there are no batch inserters (a map-shaped batch
insert would need key/value pairs — callers loop), and Equal compares
interval keys only, never the values (comparing V would need boxing).

The package panics in exactly four situations, all programmer errors
that cannot be handled where they occur:

	NewIntervalTreeFunc(nil)       — nil comparison function, caught at construction.
	Insert on a nil tree           — a nil tree cannot store an interval.
	Insert on a zero-value tree    — no comparison function; the message names the constructors.
	Insert with Hi < Lo            — an inverted interval has no closed-set meaning.

IntervalTree is not safe for concurrent use; see the interval_tree_ts
package for the guarded twin.
*/
package interval_tree

import (
	"cmp"
	"fmt"
	"io"
	"strings"
)

// Interval is a closed interval [Lo, Hi] — the key type of the tree.
// Insert requires Lo <= Hi under the tree's comparison function
// (a degenerate point interval Lo == Hi is legal); the queries all
// treat the endpoints as inclusive.
type Interval[K any] struct {
	Lo K // the low endpoint; the primary sort key
	Hi K // the high endpoint; breaks ties between equal-Lo intervals
}

// itNode is a single node of the tree.  It holds the interval key, the
// satellite value, the red-black links and color, and the interval
// augmentation: maxHi is the largest Hi of any interval in the subtree
// rooted at the node — the value that lets overlap queries prune
// subtrees.  A nil child is treated as a black leaf that contributes no
// maximum.
type itNode[K any, V any] struct {
	iv                  Interval[K]
	val                 V
	maxHi               K
	red                 bool // Red nodes may not have red children; nil children are black.
	left, right, parent *itNode[K, V]
}

// IntervalTree is a red-black interval search tree mapping Interval[K]
// to V.  Use NewIntervalTree for naturally ordered key types (numbers,
// strings) or NewIntervalTreeFunc for a caller supplied endpoint
// comparison function.  The zero value is an empty tree.
type IntervalTree[K any, V any] struct {
	root   *itNode[K, V]
	length int // Number of nodes in the tree

	// cmp orders two endpoints: negative if a sorts before b, 0 if the
	// two are equal, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order K — K
	// itself never has to implement an interface.
	cmp func(a, b K) int
}

// isRed reports whether n is a red node; nil nodes are black.
func isRed[K any, V any](n *itNode[K, V]) bool {
	return n != nil && n.red
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the endpoint comparison function installed by
// NewIntervalTree and is handy for building custom comparison functions.
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

// NewIntervalTree creates a new IntervalTree over any naturally ordered
// key type (all integers, floats and strings — cmp.Ordered).  Endpoint
// ordering uses the built-in < and > operators of K; no interface and no
// boxing is involved.
// Complexity is O(1).
func NewIntervalTree[K cmp.Ordered, V any]() *IntervalTree[K, V] {
	return &IntervalTree[K, V]{cmp: Compare[K]}
}

// NewIntervalTreeFunc creates a new IntervalTree that orders endpoints
// with the caller supplied comparison function fx.  fx must return a
// negative value if a sorts before b, 0 if the two are equal and a
// positive value if a sorts after b, and must order endpoints
// consistently.  This lets intervals over any key type — a struct, a
// time represented as a string — be stored without implementing any
// interface.
// Complexity is O(1).
func NewIntervalTreeFunc[K any, V any](fx func(a, b K) int) *IntervalTree[K, V] {
	if fx == nil {
		panic("interval_tree: NewIntervalTreeFunc called with a nil comparison function")
	}
	return &IntervalTree[K, V]{cmp: fx}
}

// keyCmp orders two endpoints, guarding against a zero-value tree that
// was not created by one of the constructors.
func (tt *IntervalTree[K, V]) keyCmp(a, b K) int {
	if tt.cmp == nil {
		panic("interval_tree: no comparison function (create the tree with NewIntervalTree or NewIntervalTreeFunc)")
	}
	return tt.cmp(a, b)
}

// ivCmp orders two interval keys: by Lo first, then by Hi.  Two
// intervals with equal Lo and equal Hi compare 0 — they are the same
// key, and inserting one replaces the other's value.
func (tt *IntervalTree[K, V]) ivCmp(a, b Interval[K]) int {
	if c := tt.keyCmp(a.Lo, b.Lo); c != 0 {
		return c
	}
	return tt.keyCmp(a.Hi, b.Hi)
}

// fixMax recomputes n's maxHi augmentation from its interval and its
// two children's maxima.  It must be called (bottom up) on every node
// whose subtree changes.
func (tt *IntervalTree[K, V]) fixMax(n *itNode[K, V]) {
	m := n.iv.Hi
	if l := n.left; l != nil && tt.cmp(l.maxHi, m) > 0 {
		m = l.maxHi
	}
	if r := n.right; r != nil && tt.cmp(r.maxHi, m) > 0 {
		m = r.maxHi
	}
	n.maxHi = m
}

// IsEmpty will return true if the tree is empty.
// Complexity is O(1).
func (tt *IntervalTree[K, V]) IsEmpty() bool {
	return tt == nil || tt.root == nil
}

// Len returns the number of intervals in the tree.
// Complexity is O(1).
func (tt *IntervalTree[K, V]) Len() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Length is an alias for Len; it returns the number of intervals in the
// tree.
// Complexity is O(1).
func (tt *IntervalTree[K, V]) Length() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Truncate removes all data from the tree.  The comparison function is
// kept, so the tree remains usable and can simply be refilled.
// Complexity is O(1).
func (tt *IntervalTree[K, V]) Truncate() {
	if tt == nil {
		return
	}
	tt.root = nil
	tt.length = 0
}

// -------------------------------------------------------------------------------------------------------
// Rotations
// -------------------------------------------------------------------------------------------------------

// rotateLeft pivots the subtree rooted at x to the left, making x's
// right child the new subtree root.  Both affected nodes' maxHi
// augmentations are recomputed — x first, since it becomes a child of
// the new subtree root.
func (tt *IntervalTree[K, V]) rotateLeft(x *itNode[K, V]) {
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
	tt.fixMax(x)
	tt.fixMax(y)
}

// rotateRight pivots the subtree rooted at x to the right, making x's
// left child the new subtree root.  Both affected nodes' maxHi
// augmentations are recomputed — x first, since it becomes a child of
// the new subtree root.
func (tt *IntervalTree[K, V]) rotateRight(x *itNode[K, V]) {
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
	tt.fixMax(x)
	tt.fixMax(y)
}

// -------------------------------------------------------------------------------------------------------
// Insert
// -------------------------------------------------------------------------------------------------------

// Insert adds the interval iv with value val to the tree.  If an equal
// interval (same Lo and same Hi) is already present its value is
// replaced in place and false is returned; true is returned when a new
// node was added.  Insert requires Hi >= Lo; an inverted interval
// panics.  Insert also panics on a nil tree or on a zero-value tree (no
// comparison function); these are the only Insert panics — every other
// operation treats both as an empty tree.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Insert(iv Interval[K], val V) (added bool) {
	if tt == nil {
		panic("interval_tree: Insert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("interval_tree: Insert called on a tree with no comparison function (create the tree with NewIntervalTree or NewIntervalTreeFunc)")
	}
	if tt.cmp(iv.Hi, iv.Lo) < 0 {
		panic("interval_tree: Insert called with an inverted interval (Hi < Lo)")
	}

	// Standard BST insert; the new node starts out red.
	var parent *itNode[K, V]
	cur := tt.root
	for cur != nil {
		c := tt.ivCmp(iv, cur.iv)
		if c == 0 {
			cur.val = val // Duplicate key: replace the stored value in place.
			return false
		}
		parent = cur
		if c < 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	z := &itNode[K, V]{iv: iv, val: val, maxHi: iv.Hi, red: true, parent: parent}
	if parent == nil {
		tt.root = z
	} else if tt.ivCmp(iv, parent.iv) < 0 {
		parent.left = z
	} else {
		parent.right = z
	}
	tt.length++

	// Grow the maxHi augmentation up the ancestor path.  A new interval
	// can only raise maxima, so the walk stops at the first ancestor
	// whose maxHi already covers the new Hi.
	for n := z.parent; n != nil; n = n.parent {
		if tt.cmp(n.maxHi, z.iv.Hi) >= 0 {
			break
		}
		n.maxHi = z.iv.Hi
	}

	tt.insertFixup(z)
	return true
}

// insertFixup restores the red-black properties after inserting the red
// node z, recoloring and rotating until no red node has a red parent.
// The rotations maintain the maxHi augmentation themselves.
func (tt *IntervalTree[K, V]) insertFixup(z *itNode[K, V]) {
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

// -------------------------------------------------------------------------------------------------------
// Search
// -------------------------------------------------------------------------------------------------------

// findNode returns the node holding an interval equal to iv (same Lo
// and same Hi), or nil if it is not present.
func (tt *IntervalTree[K, V]) findNode(iv Interval[K]) *itNode[K, V] {
	cur := tt.root
	for cur != nil {
		c := tt.ivCmp(iv, cur.iv)
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

// Search returns the value stored for the interval iv (same Lo and same
// Hi), or false if that exact interval is not present.  Overlapping but
// unequal intervals are not found — use AnyOverlap for that.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Search(iv Interval[K]) (rv V, found bool) {
	if tt == nil {
		return
	}
	if n := tt.findNode(iv); n != nil {
		return n.val, true
	}
	return
}

// -------------------------------------------------------------------------------------------------------
// Delete
// -------------------------------------------------------------------------------------------------------

// transplant replaces the subtree rooted at u with the subtree rooted at v.
func (tt *IntervalTree[K, V]) transplant(u, v *itNode[K, V]) {
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

// Delete removes the interval iv (same Lo and same Hi) from the tree.
// It returns false if that interval is not present.  Deleting is a pure
// lookup — an inverted probe interval simply matches nothing.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Delete(iv Interval[K]) (found bool) {
	if tt == nil {
		return false
	}
	z := tt.findNode(iv)
	if z == nil {
		return false
	}
	tt.deleteNode(z)
	return true
}

// deleteNode splices the node z out of the tree and restores the
// red-black properties and the maxHi augmentation.
func (tt *IntervalTree[K, V]) deleteNode(z *itNode[K, V]) {
	tt.length--

	y := z                       // The node actually spliced out of the tree.
	yWasRed := y.red             // Its color before the splice.
	var x, xParent *itNode[K, V] // x moves into y's old position.

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

	// Restore the maxHi augmentation along the ancestor path of the
	// splice.  A deletion can only lower maxima, so every node on the
	// path must be recomputed from its children — bottom up, which is
	// what this walk is (x's own subtree moved wholesale and is already
	// correct; every node above it lost a descendant).  This must run
	// before deleteFixup, whose rotations read their children's maxima.
	for n := xParent; n != nil; n = n.parent {
		tt.fixMax(n)
	}

	if !yWasRed {
		tt.deleteFixup(x, xParent)
	}
}

// deleteFixup restores the red-black properties after splicing out a
// black node.  x is the node that moved into the spliced node's
// position (possibly nil); xParent is x's parent, which must be
// supplied separately because x may be nil.
func (tt *IntervalTree[K, V]) deleteFixup(x, xParent *itNode[K, V]) {
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

// -------------------------------------------------------------------------------------------------------
// Extremes
// -------------------------------------------------------------------------------------------------------

// minNode returns the node holding the smallest interval in the subtree
// rooted at n.
func minNode[K any, V any](n *itNode[K, V]) *itNode[K, V] {
	for n != nil && n.left != nil {
		n = n.left
	}
	return n
}

// maxNode returns the node holding the largest interval in the subtree
// rooted at n.
func maxNode[K any, V any](n *itNode[K, V]) *itNode[K, V] {
	for n != nil && n.right != nil {
		n = n.right
	}
	return n
}

// Min returns the interval with the smallest (Lo, Hi) in the tree, or
// false if the tree is empty.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Min() (iv Interval[K], rv V, found bool) {
	if tt == nil {
		return
	}
	if n := minNode(tt.root); n != nil {
		return n.iv, n.val, true
	}
	return
}

// Max returns the interval with the largest (Lo, Hi) in the tree, or
// false if the tree is empty.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Max() (iv Interval[K], rv V, found bool) {
	if tt == nil {
		return
	}
	if n := maxNode(tt.root); n != nil {
		return n.iv, n.val, true
	}
	return
}

// -------------------------------------------------------------------------------------------------------
// Debugging
// -------------------------------------------------------------------------------------------------------

// Depth returns the number of nodes on the longest path from the root
// to a leaf.  An empty tree has depth 0, a tree with a single node has
// depth 1.  Balancing keeps this at O(log₂ n).
func (tt *IntervalTree[K, V]) Depth() (d int) {
	if tt == nil {
		return 0
	}
	var depthOf func(cur *itNode[K, V]) int
	depthOf = func(cur *itNode[K, V]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depthOf(cur.left), depthOf(cur.right))
	}
	return depthOf(tt.root)
}

// Dump writes an indented picture of the tree to w for debugging.  Each
// line shows a node as "lo-hi max=hi(R)" for red and "lo-hi max=hi(B)"
// for black, followed by the stored value; the left subtree is printed
// above the right subtree.  It is a debugging aid; use All or the
// query iterators to process the data.
// Complexity is O(n).
func (tt *IntervalTree[K, V]) Dump(w io.Writer) {
	if tt == nil || tt.IsEmpty() {
		_, _ = fmt.Fprintf(w, "IntervalTree (empty)\n")
		return
	}
	_, _ = fmt.Fprintf(w, "IntervalTree length=%d depth=%d\n", tt.length, tt.Depth())
	var dump func(cur *itNode[K, V], depth int)
	dump = func(cur *itNode[K, V], depth int) {
		if cur == nil {
			return
		}
		dump(cur.left, depth+1)
		c := "B"
		if cur.red {
			c = "R"
		}
		_, _ = fmt.Fprintf(w, "%s%v-%v max=%v %v(%s)\n", strings.Repeat("  ", depth), cur.iv.Lo, cur.iv.Hi, cur.maxHi, cur.val, c)
		dump(cur.right, depth+1)
	}
	dump(tt.root, 0)
}
