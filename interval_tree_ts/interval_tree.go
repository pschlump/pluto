/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

/*
Package interval_tree_ts is the thread-safe twin of
github.com/pschlump/pluto/interval_tree — the interval search tree
(CLRS §14.3; algs4 exercise 3.3.43, IntervalST.java) behind a
sync.RWMutex.  The public API is identical; switch packages and every
operation becomes safe to call from multiple goroutines.

A red-black binary search tree keyed by closed intervals [Lo, Hi] of
any ordered type with a satellite value V, each node augmented with
maxHi (the largest Hi in its subtree) so that overlap and stabbing
queries prune whole subtrees.  See the plain package's documentation
for the data-structure contracts; the twin adds only the locking:

  - Every operation takes the lock for its whole duration; no locked
    public method calls another (the unexported internals run
    lock-free while the lock is held).

  - Nil receivers are detected BEFORE the lock is acquired (a
    nil-receiver RLock would itself panic) and behave as an empty tree
    for every read; the write panics carry interval_tree_ts: prefixes.

  - All/Backward/Overlaps/Stab return EAGER SNAPSHOTS collected under
    the read lock at call time — safe to mutate the tree from inside
    the loop, at the cost of O(n) (or O(k)) space.  The plain package's
    iterators walk the live tree instead.

  - The set-convenience capabilities (capabilities.go) are the plain
    package's map-shaped reduced menu (no batch inserters; Equal
    compares interval keys only) behind the same model: the batch
    mutators DeleteSlice and the delete pass of DeleteFunc hold the
    write lock once for the whole batch (a batch is atomic), while
    Slice/String/StringFunc/Equal and DeleteFunc's match pass snapshot
    under the read lock and compute without the lock held, so a caller
    format or predicate function may safely call back into the tree.
    Equal touches each tree under its own lock only — never two locks
    at once — so comparing trees never deadlocks.

  - Lock/Unlock are the real exposed write lock (nil no-ops), and the
    Nl* methods (NlSearch, NlInsert, NlDelete, NlAnyOverlap, ...) are
    the lock-free bodies of their locked counterparts for compound
    operations — the canonical one is check-overlap-then-book:

    tree.Lock()
    if _, _, clash := tree.NlAnyOverlap(slot); !clash {
    tree.NlInsert(slot, meeting)
    }
    tree.Unlock()

    Without the compound, another goroutine's booking can land between
    the AnyOverlap check and the Insert.  Calling a locked public
    method while holding Lock deadlocks; iterators get no Nl* forms
    because a lock cannot be held across their consumption.

Panics are the plain package's four, each with the interval_tree_ts:
prefix: NewIntervalTreeFunc(nil), Insert on a nil tree, Insert on a
zero-value tree (no comparison function), Insert with Hi < Lo.
*/
package interval_tree_ts

import (
	"cmp"
	"fmt"
	"io"
	"iter"
	"slices"
	"strings"
	"sync"
)

// Interval is a closed interval [Lo, Hi] — the key type of the tree.
// Insert requires Lo <= Hi under the tree's comparison function
// (a degenerate point interval Lo == Hi is legal); the queries all
// treat the endpoints as inclusive.  It is structurally identical to
// interval_tree.Interval; converting between the twins is a plain
// assignment of the two fields.
type Interval[K any] struct {
	Lo K // the low endpoint; the primary sort key
	Hi K // the high endpoint; breaks ties between equal-Lo intervals
}

// itNode is a single node of the tree: the interval key, the satellite
// value, the red-black links and color, and the interval augmentation
// maxHi — the largest Hi of any interval in the subtree rooted at the
// node.
type itNode[K any, V any] struct {
	iv                  Interval[K]
	val                 V
	maxHi               K
	red                 bool
	left, right, parent *itNode[K, V]
}

// IntervalTree is a red-black interval search tree mapping Interval[K]
// to V, guarded by one RWMutex.  Use NewIntervalTree for naturally
// ordered key types or NewIntervalTreeFunc for a caller supplied
// endpoint comparison function.  The zero value is an empty tree.
type IntervalTree[K any, V any] struct {
	lock   sync.RWMutex
	root   *itNode[K, V]
	length int // Number of nodes in the tree

	// cmp orders two endpoints: negative if a sorts before b, 0 if the
	// two are equal, positive if a sorts after b.  Set by the
	// constructors; the only thing that knows how to order K.
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
		panic("interval_tree_ts: NewIntervalTreeFunc called with a nil comparison function")
	}
	return &IntervalTree[K, V]{cmp: fx}
}

// Lock takes the tree's write lock.  Hold it to run an atomic compound
// operation with the Nl* methods (search-then-insert,
// overlap-check-then-book); call Unlock when done.  Calling any locked
// public method while the lock is held deadlocks.  A nil no-op.
func (tt *IntervalTree[K, V]) Lock() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
}

// Unlock releases the tree's write lock.  A nil no-op.
func (tt *IntervalTree[K, V]) Unlock() {
	if tt == nil {
		return
	}
	tt.lock.Unlock()
}

// keyCmp orders two endpoints, guarding against a zero-value tree that
// was not created by one of the constructors.  Only called from write
// paths that have already validated cmp; readers check it defensively.
func (tt *IntervalTree[K, V]) keyCmp(a, b K) int {
	if tt.cmp == nil {
		panic("interval_tree_ts: no comparison function (create the tree with NewIntervalTree or NewIntervalTreeFunc)")
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
	if tt == nil {
		return true
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.root == nil
}

// Len returns the number of intervals in the tree.
// Complexity is O(1).
func (tt *IntervalTree[K, V]) Len() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Length is an alias for Len; it returns the number of intervals in the
// tree.
// Complexity is O(1).
func (tt *IntervalTree[K, V]) Length() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// NlIsEmpty is the no-lock form of IsEmpty, for compound operations run
// under Lock.  The caller must hold the write lock (or guarantee
// exclusive access); race-free because nothing mutates concurrently.
func (tt *IntervalTree[K, V]) NlIsEmpty() bool {
	return tt == nil || tt.root == nil
}

// NlLen is the no-lock form of Len, for compound operations run under
// Lock.
func (tt *IntervalTree[K, V]) NlLen() int {
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.root = nil
	tt.length = 0
}

// NlTruncate is the no-lock form of Truncate, for compound operations
// run under Lock.
func (tt *IntervalTree[K, V]) NlTruncate() {
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
// right child the new subtree root, maintaining the maxHi augmentation.
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
// left child the new subtree root, maintaining the maxHi augmentation.
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
// panics, as do Insert on a nil tree and on a zero-value tree (no
// comparison function).
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Insert(iv Interval[K], val V) (added bool) {
	if tt == nil {
		panic("interval_tree_ts: Insert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("interval_tree_ts: Insert called on a tree with no comparison function (create the tree with NewIntervalTree or NewIntervalTreeFunc)")
	}
	if tt.cmp(iv.Hi, iv.Lo) < 0 {
		panic("interval_tree_ts: Insert called with an inverted interval (Hi < Lo)")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.nucleusInsert(iv, val)
}

// NlInsert is the no-lock form of Insert, for compound operations run
// under Lock (its nil/zero-value/inverted checks panic with the
// interval_tree_ts: prefix).  The canonical compound is
// overlap-check-then-book:
//
//	tree.Lock()
//	if _, _, clash := tree.NlAnyOverlap(slot); !clash {
//		tree.NlInsert(slot, meeting)
//	}
//	tree.Unlock()
func (tt *IntervalTree[K, V]) NlInsert(iv Interval[K], val V) (added bool) {
	if tt == nil {
		panic("interval_tree_ts: NlInsert called on a nil tree")
	}
	if tt.cmp == nil {
		panic("interval_tree_ts: NlInsert called on a tree with no comparison function (create the tree with NewIntervalTree or NewIntervalTreeFunc)")
	}
	if tt.cmp(iv.Hi, iv.Lo) < 0 {
		panic("interval_tree_ts: NlInsert called with an inverted interval (Hi < Lo)")
	}
	return tt.nucleusInsert(iv, val)
}

// nucleusInsert is the lock-free insert body; the caller holds the
// write lock (or Lock) and cmp is non-nil.
func (tt *IntervalTree[K, V]) nucleusInsert(iv Interval[K], val V) (added bool) {
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

	// Grow the maxHi augmentation up the ancestor path; a new interval
	// can only raise maxima, so stop at the first ancestor already
	// covering the new Hi.
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
// node z.  The rotations maintain the maxHi augmentation themselves.
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

// findNode returns the node holding an interval equal to iv, or nil.
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
	if tt == nil || tt.cmp == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if n := tt.findNode(iv); n != nil {
		return n.val, true
	}
	return
}

// NlSearch is the no-lock form of Search, for compound operations run
// under Lock (search-then-delete, search-then-insert).
func (tt *IntervalTree[K, V]) NlSearch(iv Interval[K]) (rv V, found bool) {
	if tt == nil || tt.cmp == nil {
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
// It returns false if that interval is not present.  The write lock is
// held across the search and the splice, so the delete is atomic.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) Delete(iv Interval[K]) (found bool) {
	if tt == nil || tt.cmp == nil {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	z := tt.findNode(iv)
	if z == nil {
		return false
	}
	tt.deleteNode(z)
	return true
}

// NlDelete is the no-lock form of Delete, for compound operations run
// under Lock.
func (tt *IntervalTree[K, V]) NlDelete(iv Interval[K]) (found bool) {
	if tt == nil || tt.cmp == nil {
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
	// splice — bottom up, every node on the path recomputed from its
	// (already correct) children.  Must run before deleteFixup, whose
	// rotations read their children's maxima.
	for n := xParent; n != nil; n = n.parent {
		tt.fixMax(n)
	}

	if !yWasRed {
		tt.deleteFixup(x, xParent)
	}
}

// deleteFixup restores the red-black properties after splicing out a
// black node.  x may be nil, so its parent is supplied separately.
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
	tt.lock.RLock()
	defer tt.lock.RUnlock()
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
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if n := maxNode(tt.root); n != nil {
		return n.iv, n.val, true
	}
	return
}

// -------------------------------------------------------------------------------------------------------
// Snapshots and debugging
// -------------------------------------------------------------------------------------------------------

// ivPair is one (interval, value) pair of a snapshot.
type ivPair[K any, V any] struct {
	iv  Interval[K]
	val V
}

// toPairs snapshots the whole tree in ascending (Lo, Hi) order; the
// caller holds the read lock.
func (tt *IntervalTree[K, V]) toPairs() []ivPair[K, V] {
	out := make([]ivPair[K, V], 0, tt.length)
	var walk func(n *itNode[K, V])
	walk = func(n *itNode[K, V]) {
		if n == nil {
			return
		}
		walk(n.left)
		out = append(out, ivPair[K, V]{n.iv, n.val})
		walk(n.right)
	}
	walk(tt.root)
	return out
}

// Depth returns the number of nodes on the longest path from the root
// to a leaf.  An empty tree has depth 0, a tree with a single node has
// depth 1.  Balancing keeps this at O(log₂ n).
func (tt *IntervalTree[K, V]) Depth() (d int) {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var depthOf func(cur *itNode[K, V]) int
	depthOf = func(cur *itNode[K, V]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(depthOf(cur.left), depthOf(cur.right))
	}
	return depthOf(tt.root)
}

// Dump writes an indented picture of the tree to w for debugging,
// holding the read lock for the whole traversal (so nothing else
// touches the tree meanwhile).  Each line shows a node as
// "lo-hi max=hi(R)" for red and "lo-hi max=hi(B)" for black, followed
// by the stored value; the left subtree prints above the right.
// Complexity is O(n).
func (tt *IntervalTree[K, V]) Dump(w io.Writer) {
	if tt == nil {
		_, _ = fmt.Fprintf(w, "IntervalTree (empty)\n")
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.root == nil {
		_, _ = fmt.Fprintf(w, "IntervalTree (empty)\n")
		return
	}

	// nodeDepth is the lock-free depth helper (the rb_tree_ts Dump
	// precedent — Dump must not call the locked Depth while holding
	// the read lock: a queued writer between the two RLocks deadlocks).
	var nodeDepth func(cur *itNode[K, V]) int
	nodeDepth = func(cur *itNode[K, V]) int {
		if cur == nil {
			return 0
		}
		return 1 + max(nodeDepth(cur.left), nodeDepth(cur.right))
	}

	_, _ = fmt.Fprintf(w, "IntervalTree length=%d depth=%d\n", tt.length, nodeDepth(tt.root))
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

// emptySeq is returned by the iterators of a nil tree.
func emptySeq[K any, V any](func(Interval[K], V) bool) {}

// pairSeq wraps a snapshot as a range-over-func sequence.
func pairSeq[K any, V any](snap []ivPair[K, V]) iter.Seq2[Interval[K], V] {
	return func(yield func(Interval[K], V) bool) {
		for _, p := range snap {
			if !yield(p.iv, p.val) {
				return
			}
		}
	}
}

// All returns an iterator over the tree's intervals and values in
// ascending (Lo, Hi) order.  In a one-variable range the interval takes
// the first slot.  The iteration is over an EAGER SNAPSHOT taken under
// the read lock at call time — unlike the plain package's live walk it
// is safe to mutate the tree from inside the loop, at O(n) snapshot
// cost.  A nil tree yields nothing.
// Complexity is O(n) for the snapshot, then O(1) per element.
func (tt *IntervalTree[K, V]) All() iter.Seq2[Interval[K], V] {
	if tt == nil {
		return emptySeq[K, V]
	}
	tt.lock.RLock()
	snap := tt.toPairs()
	tt.lock.RUnlock()
	return pairSeq(snap)
}

// Backward returns an iterator over the tree's intervals and values in
// descending (Lo, Hi) order — All reversed, with the same snapshot
// semantics.
// Complexity is O(n) for the snapshot, then O(1) per element.
func (tt *IntervalTree[K, V]) Backward() iter.Seq2[Interval[K], V] {
	if tt == nil {
		return emptySeq[K, V]
	}
	tt.lock.RLock()
	snap := tt.toPairs()
	tt.lock.RUnlock()
	slices.Reverse(snap)
	return pairSeq(snap)
}
