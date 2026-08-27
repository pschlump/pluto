/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package union_find_ts implements the classic disjoint-set
// (union-find) data structure of Sedgwick & Wayne, Algorithms, 4th ed.
// §1.5, safe for concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/union_find — the same API, guarded by a
// sync.RWMutex.
//
// The elements ARE the indices 0..n-1 — there is no element type T.
// The implementation is union-by-rank with path halving in Find, giving
// amortized inverse-Ackermann (effectively constant) time per
// operation.
//
// Basic operations:
//
//	Find — Returns the root (representative) of p's set.			O(α(n)) amortized
//	Union — Merges the sets containing p and q.					O(α(n)) amortized
//	Connected — Reports whether p and q are in the same set.		O(α(n)) amortized
//	Count — Returns the number of disjoint sets.					O(1)
//	Len — Returns n, the number of elements.						O(1)
//
// Out-of-range indices REPORT, they do not panic: Find returns
// ok=false, Union and Connected return false.
//
// The package has exactly two panics, identical to union_find:
//
//	NewUnionFind(n) with n < 1.
//	Union on a nil *UnionFind.
//
// A nil *UnionFind and the zero value behave as an empty union-find
// for every other operation.
//
// Concurrency model — READ THIS:
//
//	Find and Connected take the WRITE lock, not the read lock.
//	Find performs path halving, which mutates the internal parent
//	links even though it changes no logical set — so under the race
//	detector a Find is a write.  Union also takes the write lock.
//	Only Count and Len are true readers and take the read lock.
//	Queries on a shared union-find therefore serialize; they never
//	run in parallel with each other.
//
// Run the tests with -race.
package union_find_ts

import (
	"sync"
)

// UnionFind is a disjoint-set forest over the elements 0..n-1, safe for
// concurrent use.  See the package documentation for the locking model.
type UnionFind struct {
	parent []int
	rank   []uint8
	count  int
	lock   sync.RWMutex
}

// NewUnionFind returns a union-find over the elements 0..n-1, with
// every element in its own singleton set (Count() == n).
// It panics if n < 1.
// Complexity is O(n).
func NewUnionFind(n int) *UnionFind {
	if n < 1 {
		panic("union_find_ts: NewUnionFind called with n < 1")
	}
	uf := &UnionFind{
		parent: make([]int, n),
		rank:   make([]uint8, n),
		count:  n,
	}
	for i := range uf.parent {
		uf.parent[i] = i
	}
	return uf
}

// inRange reports whether p is a valid element index of uf.  The caller
// must hold the lock (or uf must be unshared).
func (uf *UnionFind) inRange(p int) bool {
	return p >= 0 && p < len(uf.parent)
}

// Find returns the root (representative element) of the set containing
// p.  ok is false — and root is 0 — if p is out of range or uf is nil
// or a zero value.
//
// Find takes the WRITE lock: path halving mutates the parent links (the
// logical sets are unchanged — only the tree shape shortens).
// Complexity is O(α(n)) amortized, where α is the inverse Ackermann
// function.
func (uf *UnionFind) Find(p int) (root int, ok bool) {
	if uf == nil {
		return 0, false
	}
	uf.lock.Lock()
	defer uf.lock.Unlock()
	if !uf.inRange(p) {
		return 0, false
	}
	return uf.nlFind(p), true
}

// nlFind is Find without locking or the range check; the caller must
// hold the lock and must have verified that p is in range.
func (uf *UnionFind) nlFind(p int) int {
	for uf.parent[p] != p {
		uf.parent[p] = uf.parent[uf.parent[p]] // path halving
		p = uf.parent[p]
	}
	return p
}

// Union merges the sets containing p and q and returns true.  It
// returns false — changing nothing — if p or q is out of range or if p
// and q are already in the same set.
//
// It panics on a nil *UnionFind: a nil structure cannot record a merge
// (the package's only method panic).
// Complexity is O(α(n)) amortized.
func (uf *UnionFind) Union(p, q int) bool {
	if uf == nil {
		panic("union_find_ts: Union called on a nil UnionFind")
	}
	uf.lock.Lock()
	defer uf.lock.Unlock()
	if !uf.inRange(p) || !uf.inRange(q) {
		return false
	}
	pRoot := uf.nlFind(p)
	qRoot := uf.nlFind(q)
	if pRoot == qRoot {
		return false
	}
	if uf.rank[pRoot] < uf.rank[qRoot] {
		pRoot, qRoot = qRoot, pRoot
	}
	uf.parent[qRoot] = pRoot
	if uf.rank[pRoot] == uf.rank[qRoot] {
		uf.rank[pRoot]++
	}
	uf.count--
	return true
}

// Connected reports whether p and q are in the same set.  It returns
// false if either index is out of range or uf is nil or a zero value.
//
// Connected takes the WRITE lock: it runs two path-halving Finds, and
// path halving mutates the parent links.
// Complexity is O(α(n)) amortized.
func (uf *UnionFind) Connected(p, q int) bool {
	if uf == nil {
		return false
	}
	uf.lock.Lock()
	defer uf.lock.Unlock()
	if !uf.inRange(p) || !uf.inRange(q) {
		return false
	}
	return uf.nlFind(p) == uf.nlFind(q)
}

// Count returns the number of disjoint sets.  It is n for a freshly
// constructed union-find and decreases by one for each successful
// Union.  A nil or zero-value union-find reports 0.
// Complexity is O(1).
func (uf *UnionFind) Count() int {
	if uf == nil {
		return 0
	}
	uf.lock.RLock()
	defer uf.lock.RUnlock()
	return uf.count
}

// Len returns n, the number of elements (0..n-1).  A nil or zero-value
// union-find reports 0.
// Complexity is O(1).
func (uf *UnionFind) Len() int {
	if uf == nil {
		return 0
	}
	uf.lock.RLock()
	defer uf.lock.RUnlock()
	return len(uf.parent)
}
