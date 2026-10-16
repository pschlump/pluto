// Package fib_heap provides a generic indexed Fibonacci priority queue
// — the decrease-key priority queue from Fredman & Tarjan (1986), as in
// Sedgwick's Algorithms §9 "Beyond" (FibonacciMinPQ / IndexFibonacciMinPQ)
// and CLRS chapter 19.  It mirrors the API of pluto's index_pq (Sedgwick's
// IndexMinPQ): the client owns stable indices 0..n-1, Peek/Pop return the
// minimum (index, value), and Change replaces the value of any present
// index.
//
// A Fibonacci heap is a forest of heap-ordered trees linked into a
// circular doubly-linked root list, with a pointer to the minimum root.
// Insert lazily splices a new singleton into the root list in O(1);
// Pop promotes the minimum's children and then consolidates the roots
// by degree (repeatedly linking same-degree trees) in amortized O(log n);
// a decrease of a value cuts the node from its parent and "cascades"
// upward through marked parents in amortized O(1).  The cascading cuts
// keep every node's degree bounded by log_φ n (φ = the golden ratio),
// which is what makes consolidation cheap.  The mark bit means "this
// node has lost a child since it last became a child of its current
// parent" and is only meaningful on non-roots.
//
// The value type implements no interface: ordering is either the value
// type's built-in < / > operators (NewIndexFibonacciPQ) or a caller
// supplied function (NewIndexFibonacciPQFunc) — no boxing, no dynamic
// dispatch, no type assertions.  Values are stored and returned by
// value (T, not *T).
//
// An IndexFibonacciPQ is not safe for concurrent use.
//
// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package fib_heap

import (
	"cmp"
	"iter"
)

// Complexity note.  The orders below use 'n' where n = q.Len().  The
// O(1) bounds on Insert and DecreaseKey are amortized, not worst-case:
// a lazy Insert may leave up to n same-degree roots to be linked by
// some later Pop, and a lazy decrease may leave several cut subtrees
// in the root list.  Worst case Insert is O(1); worst case Pop and
// DecreaseKey are O(n) and O(log n) respectively; the amortized bound
// is the guarantee that matters.

// Compare returns -1, 0 or +1 as a orders before, equal to, or after b.
// It is the comparison function NewIndexFibonacciPQ stores for types
// with a built-in ordering, exported so callers can reuse it (including
// negated, for a max-first queue) in NewIndexFibonacciPQFunc.
func Compare[T cmp.Ordered](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// fibNode is one node of the forest.  left/right link a node into its
// circular sibling list — the root list for a root, the parent's child
// list otherwise; parent is nil exactly for roots; child points at one
// of the children (which one is arbitrary) or is nil; degree is the
// number of children; mark is the Fibonacci mark bit (see the package
// doc), reset whenever the node becomes a root.
type fibNode[T any] struct {
	value  T
	index  int
	parent *fibNode[T]
	child  *fibNode[T]
	left   *fibNode[T]
	right  *fibNode[T]
	degree int
	mark   bool
}

// IndexFibonacciPQ is an indexed Fibonacci min-priority queue over the
// index space 0..n-1 (n fixed at construction).  The zero value is not
// usable for writes; create the queue with NewIndexFibonacciPQ or
// NewIndexFibonacciPQFunc.
type IndexFibonacciPQ[T any] struct {
	n      int           // size of the index space, nodes has n slots
	length int           // number of indices present
	min    *fibNode[T]   // minimum root, nil when empty
	nodes  []*fibNode[T] // nodes[k] is the node for index k, nil when absent
	cmp    func(a, b T) int

	// consolidate scratch, reused across Pops so an extract-min at a
	// steady state allocates nothing (grown on demand, never shrunk;
	// the skip_list findPathRank caller-allocated-array precedent)
	scratchRoots []*fibNode[T]
	scratchTable []*fibNode[T]
}

// NewIndexFibonacciPQ returns an empty queue over the index space
// 0..n-1 that orders values by their built-in < / > operators.
//
// Complexity is O(n).
func NewIndexFibonacciPQ[T cmp.Ordered](n int) *IndexFibonacciPQ[T] {
	return NewIndexFibonacciPQFunc(n, Compare[T])
}

// NewIndexFibonacciPQFunc returns an empty queue over the index space
// 0..n-1 that orders values with fx (negative/zero/positive).
//
// Complexity is O(n).
func NewIndexFibonacciPQFunc[T any](n int, fx func(a, b T) int) *IndexFibonacciPQ[T] {
	if fx == nil {
		panic("fib_heap: NewIndexFibonacciPQFunc called with a nil comparison function")
	}
	if n < 1 {
		panic("fib_heap: NewIndexFibonacciPQ/NewIndexFibonacciPQFunc called with n < 1 (no usable index space)")
	}
	return &IndexFibonacciPQ[T]{
		n:     n,
		nodes: make([]*fibNode[T], n),
		cmp:   fx,
	}
}

// compare is cmp with the zero-value guard: a queue that never went
// through a constructor has no comparison function and no ordering of
// values can be defined, so the ordering operations that need one panic.
func (q *IndexFibonacciPQ[T]) compare(a, b T) int {
	if q.cmp == nil {
		panic("fib_heap: no comparison function (create the queue with NewIndexFibonacciPQ or NewIndexFibonacciPQFunc)")
	}
	return q.cmp(a, b)
}

// ---------------------------------------------------------------------------
// structural internals — everything below assumes the lock-free caller
// has established that q.min != nil where it matters
// ---------------------------------------------------------------------------

// insertNode splices the (freshly isolated) node x into the root list
// and makes it min if it orders before the current min.  It does not
// touch length or nodes.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) insertNode(x *fibNode[T]) {
	x.parent = nil
	x.mark = false
	if q.min == nil {
		x.left = x
		x.right = x
		q.min = x
		return
	}
	x.left = q.min.left
	x.right = q.min
	q.min.left.right = x
	q.min.left = x
	if q.compare(x.value, q.min.value) < 0 {
		q.min = x
	}
}

// addNode adds an isolated node x to the queue: root-list splice plus
// the length and index bookkeeping shared by Insert and the
// increase-direction Change.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) addNode(x *fibNode[T]) {
	q.insertNode(x)
	q.nodes[x.index] = x
	q.length++
}

// cut removes x from its parent's child list and splices it into the
// root list (parent nil, mark false).  It does not update min — the
// decrease path does that itself and the extract path consolidates.
// p must be x's non-nil parent.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) cut(x *fibNode[T]) {
	p := x.parent
	if p.degree == 1 {
		p.child = nil
	} else {
		if p.child == x {
			p.child = x.right
		}
		x.left.right = x.right
		x.right.left = x.left
	}
	p.degree--
	q.insertNode(x) // the root-list splice, including parent/mark resets
}

// cascadingCut walks up from y: an unmarked parent gets marked (it has
// now lost a child); a marked parent is cut to the root list itself and
// the walk continues from ITS parent.  This is what keeps node degrees
// logarithmic, which is what makes Pop amortized O(log n).
//
// Complexity is O(1) amortized, O(log n) worst case.
func (q *IndexFibonacciPQ[T]) cascadingCut(y *fibNode[T]) {
	for {
		z := y.parent
		if z == nil {
			return
		}
		if !y.mark {
			y.mark = true
			return
		}
		q.cut(y)
		y = z
	}
}

// link makes y a child of r (y was a root, r stays a root).
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) link(r, y *fibNode[T]) {
	y.parent = r
	y.mark = false
	if r.child == nil {
		y.left = y
		y.right = y
		r.child = y
	} else {
		y.left = r.child.left
		y.right = r.child
		r.child.left.right = y
		r.child.left = y
	}
	r.degree++
}

// consolidate rebuilds the root list so that no two roots share a
// degree, by repeatedly linking same-degree roots into heap-ordered
// trees (the smaller value becomes the parent), and recomputes min.
// Called by extract after a removal left roots behind.
//
// Complexity is O(R) where R is the number of roots, amortized O(log n).
func (q *IndexFibonacciPQ[T]) consolidate() {
	// Snapshot the current roots into the reusable scratch: the linking
	// below rewrites the root list, which is rebuilt from scratch at the
	// end.
	q.scratchRoots = q.scratchRoots[:0]
	for r := q.min; ; {
		q.scratchRoots = append(q.scratchRoots, r)
		r = r.right
		if r == q.min {
			break
		}
	}
	if cap(q.scratchTable) < 64 {
		q.scratchTable = make([]*fibNode[T], 64)
	} else {
		q.scratchTable = q.scratchTable[:64]
		clear(q.scratchTable)
	}
	table := q.scratchTable
	for _, r := range q.scratchRoots {
		for d := r.degree; ; d++ {
			for d >= len(table) {
				table = append(table, nil)
			}
			other := table[d]
			if other == nil {
				table[d] = r
				break
			}
			table[d] = nil
			if q.compare(other.value, r.value) < 0 {
				r, other = other, r
			}
			q.link(r, other) // other becomes a child of r
		}
	}
	q.min = nil
	for _, r := range table {
		if r != nil {
			q.insertNode(r)
		}
	}
	// The table entries are links into the structure; drop them so the
	// scratch does not pin removed subtrees between Pops.
	clear(table)
}

// extract removes the node x from the queue (whatever its position) and
// returns it as an isolated singleton (child pointers cleared, degree
// 0).  x's children are promoted to the root list, the root list is
// consolidated, min is recomputed, and the index/length bookkeeping is
// done — after extract returns, the queue is in a valid state.
//
// This one routine is the removal machinery for Pop, Delete and the
// increase-direction Change (which re-inserts the returned node with
// its new value).
//
// Complexity is O(log n) amortized.
func (q *IndexFibonacciPQ[T]) extract(x *fibNode[T]) *fibNode[T] {
	if p := x.parent; p != nil {
		q.cut(x) // x becomes a root
		q.cascadingCut(p)
	}
	// Promote x's children to the root list.  next is captured before
	// the splice overwrites cur.right.
	if c := x.child; c != nil {
		for cur := c; ; {
			next := cur.right
			q.insertNode(cur)
			cur = next
			if cur == c {
				break
			}
		}
	}
	// Unlink x itself from the root list.
	if x.right == x {
		q.min = nil
	} else {
		x.left.right = x.right
		x.right.left = x.left
		q.min = x.right // provisional; consolidate recomputes it
	}
	x.parent = nil
	x.child = nil
	x.left = x
	x.right = x
	x.degree = 0
	x.mark = false
	q.nodes[x.index] = nil
	q.length--
	if q.min != nil {
		q.consolidate()
	}
	return x
}

// ---------------------------------------------------------------------------
// public API
// ---------------------------------------------------------------------------

// Insert adds index k with value v, or replaces the value if k is
// already present (in which direction soever the replacement goes).
// It reports false only for a k outside 0..n-1.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) Insert(k int, v T) bool {
	if q == nil {
		panic("fib_heap: Insert called on a nil queue")
	}
	if q.cmp == nil {
		panic("fib_heap: Insert called on a queue with no comparison function (create the queue with NewIndexFibonacciPQ or NewIndexFibonacciPQFunc)")
	}
	if k < 0 || k >= q.n {
		return false
	}
	if x := q.nodes[k]; x != nil {
		q.changeNode(x, v)
		return true
	}
	x := &fibNode[T]{value: v, index: k}
	x.left = x
	x.right = x
	q.addNode(x)
	return true
}

// changeNode is the shared body of Change and DecreaseKey: give the
// node x the new value v and repair the structure.  A decrease (or an
// equal value) cuts x toward the root when heap order is violated; an
// increase removes and re-inserts x (the extract machinery), because
// the whole subtree under x may now be out of order.
func (q *IndexFibonacciPQ[T]) changeNode(x *fibNode[T], v T) {
	old := x.value
	x.value = v
	if q.compare(v, old) <= 0 {
		// Decrease or no change: repair only the x-to-parent edge.
		if p := x.parent; p != nil && q.compare(v, p.value) < 0 {
			q.cut(x)
			q.cascadingCut(p)
		}
		if q.compare(v, q.min.value) < 0 {
			q.min = x
		}
		return
	}
	// Increase: remove and re-insert with the new value (extract has
	// already dropped the node's old bookkeeping).
	x = q.extract(x)
	q.addNode(x)
}

// Change replaces the value of a present index k with v, in either
// direction.  It reports false if k is absent or outside 0..n-1.
//
// Complexity is O(1) amortized when v does not order after the current
// value (the decrease-key case), O(log n) amortized otherwise.
func (q *IndexFibonacciPQ[T]) Change(k int, v T) bool {
	if q == nil {
		panic("fib_heap: Change called on a nil queue")
	}
	if q.cmp == nil {
		panic("fib_heap: Change called on a queue with no comparison function (create the queue with NewIndexFibonacciPQ or NewIndexFibonacciPQFunc)")
	}
	x := q.nodeAt(k)
	if x == nil {
		return false
	}
	q.changeNode(x, v)
	return true
}

// DecreaseKey replaces the value of a present index k with v only when
// v orders strictly before the current value (the Dijkstra/Prim shape);
// it reports false — and changes nothing — if k is absent, outside the
// index space, or not a decrease.
//
// Complexity is O(1) amortized.
func (q *IndexFibonacciPQ[T]) DecreaseKey(k int, v T) bool {
	if q == nil {
		return false
	}
	x := q.nodeAt(k)
	if x == nil || q.compare(v, x.value) >= 0 {
		return false
	}
	q.changeNode(x, v)
	return true
}

// Delete removes the index k.  It reports false if k is absent or
// outside 0..n-1.  Removing the minimum is exactly Pop; removing any
// other node cuts it to a root, promotes its children and consolidates.
//
// Complexity is O(log n) amortized.
func (q *IndexFibonacciPQ[T]) Delete(k int) bool {
	x := q.nodeAt(k)
	if x == nil {
		return false
	}
	q.extract(x)
	return true
}

// nodeAt returns the node for index k, or nil when k is out of range
// or absent.  A method on a nil queue answers nil: every caller of
// nodeAt is a tolerated read.
func (q *IndexFibonacciPQ[T]) nodeAt(k int) *fibNode[T] {
	if q == nil || k < 0 || k >= q.n {
		return nil
	}
	return q.nodes[k]
}

// Contains reports whether index k is in the queue.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) Contains(k int) bool {
	return q.nodeAt(k) != nil
}

// Value returns the value associated with index k, or (_, false) when
// k is absent or outside the index space.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) Value(k int) (T, bool) {
	x := q.nodeAt(k)
	if x == nil {
		var zero T
		return zero, false
	}
	return x.value, true
}

// Peek returns the minimum (index, value) without removing it.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) Peek() (int, T, bool) {
	if q == nil || q.min == nil {
		var zero T
		return 0, zero, false
	}
	return q.min.index, q.min.value, true
}

// Pop removes and returns the minimum (index, value).
//
// Complexity is O(log n) amortized.
func (q *IndexFibonacciPQ[T]) Pop() (int, T, bool) {
	if q == nil || q.min == nil {
		var zero T
		return 0, zero, false
	}
	x := q.extract(q.min)
	return x.index, x.value, true
}

// Len returns the number of indices in the queue.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// IsEmpty reports whether the queue holds no indices.  A nil queue and
// the zero value are empty.
//
// Complexity is O(1).
func (q *IndexFibonacciPQ[T]) IsEmpty() bool {
	return q.Len() == 0
}

// Truncate removes every index, keeping the ordering function and the
// index space (the queue remains usable).
//
// Complexity is O(n).
func (q *IndexFibonacciPQ[T]) Truncate() {
	if q == nil {
		return
	}
	q.min = nil
	clear(q.nodes)
	q.length = 0
}

// clone returns a private shallow copy of the queue that shares only
// the comparison function.  All drains the clone, so the original is
// untouched by the iteration.
//
// Complexity is O(n).
func (q *IndexFibonacciPQ[T]) clone() *IndexFibonacciPQ[T] {
	c := &IndexFibonacciPQ[T]{n: q.n, nodes: make([]*fibNode[T], q.n), cmp: q.cmp}
	for k, x := range q.nodes {
		if x != nil {
			y := &fibNode[T]{value: x.value, index: k}
			y.left = y
			y.right = y
			c.addNode(y)
		}
	}
	return c
}

// All iterates the (index, value) pairs in priority order (minimum
// first) by draining a private snapshot copy taken when it is called.
// The queue is unchanged afterwards, later modifications are not
// observed, and mutating the queue from inside the loop is safe.  Ties
// may yield in any order.
//
// Complexity is O(n log n).
func (q *IndexFibonacciPQ[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if q == nil {
			return
		}
		c := q.clone()
		for {
			k, v, ok := c.Pop()
			if !ok || !yield(k, v) {
				return
			}
		}
	}
}
