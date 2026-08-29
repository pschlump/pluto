// Package binomial_queue provides a generic binomial queue (binomial
// heap) — the mergeable priority queue from Sedgwick's Algorithms in C,
// chapter 9.  DeleteMin always removes and returns the minimum element.
//
// A binomial queue is a forest of binomial trees, at most one tree of
// each degree, kept in strictly increasing order of degree.  Every tree
// is min-heap-ordered (each child sorts at or after its parent per the
// comparison function).  The representation is a slice of tree roots;
// each node holds its children in a slice in increasing order of degree,
// so a degree-k tree has 2^k nodes and children of degrees 0..k-1.
// Insert is a 1-node merge (amortized O(1)) and Merge is binary addition
// over the two forests with tree linking as the carry — the O(log n)
// merge that a binary heap cannot do.
//
// The element type implements no interface: ordering is supplied as a
// plain comparison function over type parameters, elements are stored
// and returned by value, and empty/minimum lookups report not-found
// instead of panicking.
//
// A BinomialQueue is not safe for concurrent use.
//
// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package binomial_queue

import (
	"cmp"
	"fmt"
	"io"
	"strings"
)

// Complexity note.  The order uses 'n' where n = q.Length().

// bqNode is one node of a binomial tree.  children is in increasing
// order of degree: a node of degree k (len(children) == k) roots a true
// binomial tree B_k with 2^k nodes, and children[i] has degree i.
type bqNode[T any] struct {
	value    T
	children []*bqNode[T]
}

// BinomialQueue is a generic mergeable min-first priority queue.  Use
// NewBinomialQueue for naturally ordered element types (numbers,
// strings) or NewBinomialQueueFunc for a caller supplied comparison
// function.  The zero value is an empty queue that tolerates every
// operation except Insert (and a Merge that would have to store
// elements), which need a comparison function.
type BinomialQueue[T any] struct {
	trees  []*bqNode[T] // the forest, in strictly increasing order of degree
	length int

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewBinomialQueue
// and is handy for building custom comparison functions — including
// reversed ones, which turn the min-first queue into a max-first queue.
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

// NewBinomialQueue creates a new empty binomial queue for any naturally
// ordered element type (all integers, floats and strings — cmp.Ordered).
// Ordering uses the built-in < and > operators of T; no interface and no
// boxing is involved.
// Complexity is O(1).
func NewBinomialQueue[T cmp.Ordered]() *BinomialQueue[T] {
	return &BinomialQueue[T]{cmp: Compare[T]}
}

// NewBinomialQueueFunc creates a new empty binomial queue that orders
// elements with the caller supplied comparison function fx.  fx must
// return a negative value if a sorts before b, 0 if the two are
// duplicates and a positive value if a sorts after b, and must order
// elements consistently.  A reversed comparison turns the queue into a
// max-first priority queue.
// Complexity is O(1).
func NewBinomialQueueFunc[T any](fx func(a, b T) int) *BinomialQueue[T] {
	if fx == nil {
		panic("binomial_queue: NewBinomialQueueFunc called with a nil comparison function")
	}
	return &BinomialQueue[T]{cmp: fx}
}

// compare orders a and b, guarding against a zero-value queue that was
// not created by one of the constructors.
func (q *BinomialQueue[T]) compare(a, b T) int {
	if q.cmp == nil {
		panic("binomial_queue: no comparison function (create the queue with NewBinomialQueue or NewBinomialQueueFunc)")
	}
	return q.cmp(a, b)
}

// Insert adds value to the queue.  Insert is a merge with a 1-node
// queue, so it is O(1) amortized — the same carry argument as
// incrementing a binary counter.
// Insert panics on a nil queue or on a zero-value queue (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty queue.
// Complexity is O(1) amortized, O(log n) in the worst case.
func (q *BinomialQueue[T]) Insert(value T) {
	if q == nil {
		panic("binomial_queue: Insert called on a nil queue")
	}
	if q.cmp == nil {
		panic("binomial_queue: Insert called on a queue with no comparison function (create the queue with NewBinomialQueue or NewBinomialQueueFunc)")
	}
	q.mergeForest([]*bqNode[T]{{value: value}})
	q.length++
}

// FindMin returns the minimum element of the queue without removing it.
// The minimum is the smallest of the tree roots, found by scanning the
// forest; the roots are not cached.  FindMin on an empty queue reports
// false.
// Complexity is O(log n) — the forest has at most log₂ n + 1 trees.
func (q *BinomialQueue[T]) FindMin() (rv T, found bool) {
	if q == nil || q.length == 0 {
		return
	}
	mi := q.minRoot()
	return q.trees[mi].value, true
}

// Peek returns the minimum element of the queue without removing it.
// It is an alias for FindMin.
// Complexity is O(log n).
func (q *BinomialQueue[T]) Peek() (rv T, found bool) {
	return q.FindMin()
}

// DeleteMin removes and returns the minimum element.  The root with the
// smallest value is detached; its children — already a valid forest of
// binomial trees in increasing order of degree — are merged back into
// the queue.  DeleteMin on an empty queue reports false.
// Complexity is O(log n).
func (q *BinomialQueue[T]) DeleteMin() (rv T, found bool) {
	if q == nil || q.length == 0 {
		return
	}
	mi := q.minRoot()
	root := q.trees[mi]
	rv = root.value
	rest := make([]*bqNode[T], 0, len(q.trees)-1)
	rest = append(rest, q.trees[:mi]...)
	rest = append(rest, q.trees[mi+1:]...)
	q.trees = rest
	q.mergeForest(root.children) // children are a forest, degrees 0..k-1 ascending
	root.children = nil          // drop the deleted node's links so the GC can reclaim the subtree
	q.length--
	if q.length == 0 {
		q.trees = nil // release the forest on a full drain
	}
	return rv, true
}

// minRoot returns the index in q.trees of the tree with the smallest
// root value.  The queue must be non-empty.
func (q *BinomialQueue[T]) minRoot() int {
	mi := 0
	for i := 1; i < len(q.trees); i++ {
		if q.compare(q.trees[i].value, q.trees[mi].value) < 0 {
			mi = i
		}
	}
	return mi
}

// Merge absorbs other into the receiver: every element of other is moved
// into q and other is left empty (but still usable).  The result is
// ordered by the receiver's comparison function; a zero-value receiver
// adopts other's comparison function.  Merging an empty or nil other is
// a no-op, and merging a queue into itself is a no-op.  Merge panics on
// a nil receiver when other is non-empty — there is nowhere to store the
// elements.
// Complexity is O(log n + log m) where n and m are the two queue sizes.
func (q *BinomialQueue[T]) Merge(other *BinomialQueue[T]) {
	if q == other || other == nil || other.length == 0 {
		return
	}
	if q == nil {
		panic("binomial_queue: Merge called on a nil queue with a non-empty other (create the receiver with NewBinomialQueue or NewBinomialQueueFunc)")
	}
	if q.cmp == nil {
		q.cmp = other.cmp // a zero-value receiver adopts other's comparison function
	}
	q.mergeForest(other.trees)
	q.length += other.length
	other.trees = nil
	other.length = 0
}

// link combines two same-degree trees into one tree of degree+1: the
// root whose value sorts first keeps the other tree as its newest
// (highest-degree) child.  The queue must have a comparison function.
func (q *BinomialQueue[T]) link(x, y *bqNode[T]) *bqNode[T] {
	if q.compare(x.value, y.value) <= 0 {
		x.children = append(x.children, y)
		return x
	}
	y.children = append(y.children, x)
	return y
}

// mergeForest merges the forest other (strictly increasing degrees, like
// q.trees) into q.trees — binary addition over the degree positions with
// link as the carry.  q.length is NOT updated; the caller accounts for
// the moved nodes.
func (q *BinomialQueue[T]) mergeForest(other []*bqNode[T]) {
	if len(other) == 0 {
		return
	}
	a, b := q.trees, other
	var out []*bqNode[T]
	var carry *bqNode[T]
	i, j := 0, 0
	for i < len(a) || j < len(b) || carry != nil {
		// The smallest degree present at the head of either forest or
		// in the carry; collect every tree of that degree (at most 3 —
		// one from each forest plus the carry).
		d := -1
		if i < len(a) && (d < 0 || len(a[i].children) < d) {
			d = len(a[i].children)
		}
		if j < len(b) && (d < 0 || len(b[j].children) < d) {
			d = len(b[j].children)
		}
		if carry != nil && (d < 0 || len(carry.children) < d) {
			d = len(carry.children)
		}
		var group []*bqNode[T]
		if i < len(a) && len(a[i].children) == d {
			group = append(group, a[i])
			i++
		}
		if j < len(b) && len(b[j].children) == d {
			group = append(group, b[j])
			j++
		}
		if carry != nil && len(carry.children) == d {
			group = append(group, carry)
			carry = nil
		}
		switch len(group) {
		case 1: // 0 or 1 tree at this degree: it passes through
			out = append(out, group[0])
		case 2: // 1+1: link into the carry
			carry = q.link(group[0], group[1])
		case 3: // 1+1+1: one stays, the other two link into the carry
			out = append(out, group[0])
			carry = q.link(group[1], group[2])
		}
	}
	q.trees = out
}

// Len will return the number of items in the queue.
// Complexity is O(1).
func (q *BinomialQueue[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// Length will return the number of items in the queue.  It is an alias
// for Len.
// Complexity is O(1).
func (q *BinomialQueue[T]) Length() int {
	if q == nil {
		return 0
	}
	return q.length
}

// IsEmpty returns true if the queue is empty.
// Complexity is O(1).
func (q *BinomialQueue[T]) IsEmpty() bool {
	return q == nil || q.length == 0
}

// Truncate removes all elements from the queue, releasing the underlying
// nodes so the GC can reclaim them.  The queue keeps its comparison
// function and is immediately reusable.
// Complexity is O(1).
func (q *BinomialQueue[T]) Truncate() {
	if q == nil {
		return
	}
	q.trees = nil
	q.length = 0
}

// Dump writes the contents of the queue (in internal forest order — not
// sorted order) to `fp`: a header line, then every node of every tree in
// pre-order, indented by depth.  An empty queue produces no output.  It
// is a debugging aid; repeatedly calling DeleteMin is the way to consume
// a queue in sorted order.
// Complexity is O(n).
func (q *BinomialQueue[T]) Dump(fp io.Writer) {
	if q == nil || q.length == 0 {
		return
	}
	_, _ = fmt.Fprintf(fp, "BinomialQueue length=%d trees=%d\n", q.length, len(q.trees))
	var walk func(n *bqNode[T], depth int)
	walk = func(n *bqNode[T], depth int) {
		_, _ = fmt.Fprintf(fp, "%s%+v\n", strings.Repeat("  ", depth), n.value)
		for _, c := range n.children {
			walk(c, depth+1)
		}
	}
	for _, tr := range q.trees {
		walk(tr, 0)
	}
}
