/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package union_find implements the classic disjoint-set (union-find)
// data structure of Sedgwick & Wayne, Algorithms, 4th ed. §1.5.
//
// The elements ARE the indices 0..n-1 — there is no element type T:
// a union-find tracks which of n abstract sites are connected, nothing
// more.  The implementation is union-by-rank with path halving in Find,
// giving amortized inverse-Ackermann (effectively constant) time per
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
// Out-of-range indices REPORT, they do not panic (the heap
// indexed-operation convention, not Sedgwick's exception): Find returns
// ok=false, Union and Connected return false.
//
// The package has exactly two panics:
//
//	NewUnionFind(n) with n < 1 — a union-find over no elements is
//	meaningless.
//	Union on a nil *UnionFind — the one write with no sane answer.
//
// A nil *UnionFind and the zero value behave as an empty union-find
// (no elements, no sets) for every other operation.
//
// Find performs path halving, so it MUTATES the internal parent links
// (the logical structure is unchanged — only the tree shape shortens).
// This matters for the thread-safe twin union_find_ts, where Find and
// Connected therefore take the write lock.
//
// The structure is not safe for concurrent use.
package union_find

// UnionFind is a disjoint-set forest over the elements 0..n-1.
//
// parent[i] is i's parent in the forest; a root is its own parent.
// rank is an upper bound on a root's tree height, compared only when
// merging two roots (it is never decremented).  count is the number of
// disjoint sets.
type UnionFind struct {
	parent []int
	rank   []uint8
	count  int
}

// NewUnionFind returns a union-find over the elements 0..n-1, with
// every element in its own singleton set (Count() == n).
// It panics if n < 1.
// Complexity is O(n).
func NewUnionFind(n int) *UnionFind {
	if n < 1 {
		panic("union_find: NewUnionFind called with n < 1")
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

// inRange reports whether p is a valid element index of uf.
func (uf *UnionFind) inRange(p int) bool {
	return uf != nil && p >= 0 && p < len(uf.parent)
}

// Find returns the root (representative element) of the set containing
// p.  ok is false — and root is 0 — if p is out of range or uf is nil
// or a zero value.
//
// Find performs path halving: every other link on the walk to the root
// is shortcut to point at its grandparent.  This mutates the internal
// tree shape but never the logical sets.
// Complexity is O(α(n)) amortized, where α is the inverse Ackermann
// function.
func (uf *UnionFind) Find(p int) (root int, ok bool) {
	if !uf.inRange(p) {
		return 0, false
	}
	return uf.nlFind(p), true
}

// nlFind is Find without the range check; the caller must have verified
// that p is in range.
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
//
// The merge is by rank: the shorter tree's root is attached under the
// taller one's, and the rank rises only when two equal ranks merge.
// Complexity is O(α(n)) amortized.
func (uf *UnionFind) Union(p, q int) bool {
	if uf == nil {
		panic("union_find: Union called on a nil UnionFind")
	}
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
// Complexity is O(α(n)) amortized.
func (uf *UnionFind) Connected(p, q int) bool {
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
	return uf.count
}

// Len returns n, the number of elements (0..n-1).  A nil or zero-value
// union-find reports 0.
// Complexity is O(1).
func (uf *UnionFind) Len() int {
	if uf == nil {
		return 0
	}
	return len(uf.parent)
}
