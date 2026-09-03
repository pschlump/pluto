/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package union_find_ts

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON implements json.Marshaler so a UnionFind can be used
// directly with the encoding/json package.  The union-find is encoded
// as a JSON array of arrays — one inner array per disjoint set, listing
// the set's members.  Sets are ordered by their smallest member and the
// members within each set are ascending, so the encoding of a given
// partition is deterministic.  A fresh union-find of size n therefore
// encodes as [[0],[1],...,[n-1]].
//
// The snapshot is taken under the WRITE lock, not the read lock: the
// Finds that group the elements perform path halving, which mutates the
// parent links (the package's Find/Connected convention).  The encoding
// itself runs without the lock, so it never delays other operations.
//
// An empty union-find encodes as [].  A direct call on a nil union-find
// also encodes as [] (the "nil behaves as an empty union-find" read
// contract); note that json.Marshal on a nil *UnionFind never reaches
// this method — the json package writes null for nil pointers itself.
// Complexity is O(n·α(n)) plus the cost of encoding the elements.
func (uf *UnionFind) MarshalJSON() ([]byte, error) {
	sets := uf.snapshot() // takes and releases the write lock itself
	if sets == nil {
		return []byte("[]"), nil // a nil or empty union-find marshals as an empty array
	}
	return json.Marshal(sets)
}

// snapshot groups the elements by set under one hold of the write lock
// (path halving in nlFind mutates the forest, so the read lock is not
// enough).  Sets are ordered by first appearance while scanning the
// elements 0..n-1 — equivalently, by smallest member.
func (uf *UnionFind) snapshot() [][]int {
	if uf == nil {
		return nil
	}
	uf.lock.Lock()
	defer uf.lock.Unlock()
	n := len(uf.parent)
	if n == 0 {
		return nil
	}
	pos := make(map[int]int, n) // root -> index of its set in sets
	var sets [][]int
	for p := 0; p < n; p++ {
		root := uf.nlFind(p)
		i, ok := pos[root]
		if !ok {
			i = len(sets)
			pos[root] = i
			sets = append(sets, []int{})
		}
		sets[i] = append(sets[i], p)
	}
	return sets
}

// UnmarshalJSON implements json.Unmarshaler so a UnionFind can be used
// directly with the encoding/json package.  data must be a JSON array
// of arrays (or null) describing a partition of the elements 0..n-1 —
// every element exactly once, in some set.  The decoded sets replace
// the current partition under one hold of the write lock: the forest is
// reset to singletons and each set's members are re-merged with the
// no-lock union.  n is fixed at construction, so the receiver must have
// the same size as the decoded partition.
//
// data is decoded before the lock is taken, and a decode error
// (malformed JSON, a non-array document, wrong element types) — or a
// validation error (an out-of-range, duplicated, or missing element) —
// is returned with the union-find untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil union-find or a zero-value
// union-find (n == 0) panics with the standard insert-family message.
// An empty array or null resets the partition to singletons and is
// tolerated everywhere — it stores nothing.
// Complexity is O(n·α(n)) plus the cost of decoding the elements.
func (uf *UnionFind) UnmarshalJSON(data []byte) error {
	var sets [][]int
	if err := json.Unmarshal(data, &sets); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Concat precedent).
	total := 0
	for _, s := range sets {
		total += len(s)
	}
	if total > 0 && uf == nil {
		panic("union_find_ts: UnmarshalJSON called on a nil UnionFind")
	}
	if uf == nil {
		return nil // null or []: nothing to store, nothing to reset
	}

	uf.lock.Lock()
	defer uf.lock.Unlock()

	n := len(uf.parent)
	if total > 0 && n == 0 {
		panic("union_find_ts: UnmarshalJSON called on a zero-value UnionFind with no elements (create the union-find with NewUnionFind)")
	}

	// Validate the partition before touching the forest: every element
	// of 0..n-1 must appear exactly once.  An empty document (null or
	// []) stores no set; it just resets the partition below.
	if len(sets) > 0 {
		seen := make([]bool, n)
		for _, s := range sets {
			for _, p := range s {
				if p < 0 || p >= n {
					return fmt.Errorf("union_find_ts: UnmarshalJSON: element %d is out of range 0..%d", p, n-1)
				}
				if seen[p] {
					return fmt.Errorf("union_find_ts: UnmarshalJSON: element %d appears in more than one set", p)
				}
				seen[p] = true
			}
		}
		for p := 0; p < n; p++ {
			if !seen[p] {
				return fmt.Errorf("union_find_ts: UnmarshalJSON: the decoded sets do not cover element %d (they must partition 0..%d)", p, n-1)
			}
		}
	}

	// Reset to singletons, then re-merge each set's members.
	for i := range uf.parent {
		uf.parent[i] = i
		uf.rank[i] = 0
	}
	uf.count = n
	for _, s := range sets {
		for i := 1; i < len(s); i++ {
			uf.nlUnion(s[0], s[i])
		}
	}
	return nil
}

// nlUnion is Union without locking or the nil/range checks; the caller
// must hold the lock and must have verified that p and q are in range.
func (uf *UnionFind) nlUnion(p, q int) {
	pRoot := uf.nlFind(p)
	qRoot := uf.nlFind(q)
	if pRoot == qRoot {
		return
	}
	if uf.rank[pRoot] < uf.rank[qRoot] {
		pRoot, qRoot = qRoot, pRoot
	}
	uf.parent[qRoot] = pRoot
	if uf.rank[pRoot] == uf.rank[qRoot] {
		uf.rank[pRoot]++
	}
	uf.count--
}
