/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package union_find

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON implements json.Marshaler so a UnionFind can be used
// directly with the encoding/json package.  The partition is encoded as
// a JSON array of arrays — one inner array per disjoint set, holding
// the set's members in ascending order — with the sets ordered by
// their smallest member, so the output is deterministic: the classic
// 10-site trace's partition {0,1,2,5,6,7} {3,4,8,9} encodes as
// [[0,1,2,5,6,7],[3,4,8,9]].
//
// The elements are the indices 0..n-1 themselves (plain ints), so
// there is no element-level marshaling to honor — only the partition
// is pluto's.
//
// A zero-value union-find encodes as [].  A direct call on a nil
// union-find also encodes as [] (the "nil behaves as an empty
// union-find" read contract); note that json.Marshal on a nil
// *UnionFind never reaches this method — the json package writes null
// for nil pointers itself.
// Complexity is O(n·α(n)) — one Find per element.
func (uf *UnionFind) MarshalJSON() ([]byte, error) {
	if uf == nil {
		return []byte("[]"), nil
	}
	sets := make([][]int, 0, uf.count)
	slot := make(map[int]int, uf.count) // root → index of its set in sets
	for p := range uf.parent {
		root := uf.nlFind(p)
		if i, ok := slot[root]; ok {
			sets[i] = append(sets[i], p)
		} else {
			slot[root] = len(sets)
			sets = append(sets, []int{p})
		}
	}
	return json.Marshal(sets)
}

// UnmarshalJSON implements json.Unmarshaler so a UnionFind can be used
// directly with the encoding/json package.  data must be a JSON array
// of arrays (or null) — one inner array per disjoint set, as produced
// by MarshalJSON; the decoded partition replaces the current partition
// of uf, which keeps its size n.  Reconstruction goes through Union,
// so the rebuilt forest is structurally sound regardless of member
// order within a set.
//
// Every element 0..n-1 must appear exactly once across the sets; an
// out-of-range, duplicated, or missing element — or any decode error
// (malformed JSON, a non-array document, wrong element types) — is
// returned as an error and leaves the union-find untouched.
//
// Unmarshaling stores sets, so it follows the Union contract: data
// that would store a set into a nil union-find or a zero-value
// union-find (no elements) panics with the standard message.  An empty
// array or null resets uf to n singleton sets and is tolerated
// everywhere — it stores nothing.
// Complexity is O(n·α(n)).
func (uf *UnionFind) UnmarshalJSON(data []byte) error {
	var sets [][]int
	if err := json.Unmarshal(data, &sets); err != nil {
		return err
	}

	// The write contract only fires when a set would actually be
	// stored.
	if len(sets) > 0 {
		if uf == nil {
			panic("union_find: UnmarshalJSON called on a nil UnionFind")
		}
		if len(uf.parent) == 0 {
			panic("union_find: UnmarshalJSON called on a zero-value UnionFind (create it with NewUnionFind)")
		}
	}
	if uf == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	// Validate the whole partition before touching uf: the elements
	// must be exactly 0..n-1, each exactly once.  An empty array (or
	// null, which decodes to one) is the reset case and is exempt.
	n := len(uf.parent)
	seen := make([]bool, n)
	total := 0
	for _, set := range sets {
		for _, p := range set {
			if p < 0 || p >= n {
				return fmt.Errorf("union_find: UnmarshalJSON: element %d is out of range 0..%d", p, n-1)
			}
			if seen[p] {
				return fmt.Errorf("union_find: UnmarshalJSON: element %d appears in more than one set", p)
			}
			seen[p] = true
			total++
		}
	}
	if len(sets) > 0 && total != n {
		return fmt.Errorf("union_find: UnmarshalJSON: %d elements supplied for a union-find of size %d; every element 0..%d must appear exactly once", total, n, n-1)
	}

	// Reset to singletons, then rebuild the partition with Union.
	for i := range uf.parent {
		uf.parent[i] = i
		uf.rank[i] = 0
	}
	uf.count = n
	for _, set := range sets {
		for _, p := range set[1:] {
			uf.Union(set[0], p)
		}
	}
	return nil
}
