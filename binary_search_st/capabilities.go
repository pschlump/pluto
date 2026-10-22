/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st

import (
	"fmt"
	"iter"
	"strings"
)

// Keys returns a fresh slice of the keys in ascending order.  A nil
// table and an empty table both yield nil.
// Complexity is O(n).
func (st *BinarySearchST[K, V]) Keys() []K {
	if st == nil || len(st.keys) == 0 {
		return nil
	}
	out := make([]K, len(st.keys))
	copy(out, st.keys)
	return out
}

// String renders the table as its (key: value) pairs in ascending key
// order, "[k1:v1 k2:v2]" — the tree_set bracket style.  An empty or
// nil table renders as "[]".
// Complexity is O(n).
func (st *BinarySearchST[K, V]) String() string {
	return st.StringFunc(func(k K, v V) string {
		return fmt.Sprintf("%v:%v", k, v)
	})
}

// StringFunc renders the table through a caller supplied format
// function, one string per entry in ascending key order, joined with
// spaces inside the tree_set brackets.  An empty or nil table renders
// as "[]".
// Complexity is O(n) plus the cost of the format function.
func (st *BinarySearchST[K, V]) StringFunc(format func(k K, v V) string) string {
	var sb strings.Builder
	sb.WriteByte('[')
	first := true
	for k, v := range st.All() {
		if !first {
			sb.WriteByte(' ')
		}
		first = false
		fmt.Fprint(&sb, format(k, v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Clone returns a deep copy of the table: a fresh table over the same
// comparison function with the same key→value bindings.  No backing
// array is shared, so Put, Delete or DeleteRange on either table
// afterwards never affects the other.  A nil table clones to nil; a
// zero-value table clones to a fresh zero value.
//
// There is no Equal: keys compare through the stored comparison
// function but values have none — the table stores a single cmp, not a
// key-cmp plus value-eq pair (the lru precedent, which also has no
// Equal).  Compare tables via Keys and Get when needed.
// Complexity is O(n).
func (st *BinarySearchST[K, V]) Clone() *BinarySearchST[K, V] {
	if st == nil {
		return nil
	}
	if st.cmp == nil {
		return &BinarySearchST[K, V]{}
	}
	out := &BinarySearchST[K, V]{cmp: st.cmp}
	if len(st.keys) > 0 {
		out.keys = make([]K, len(st.keys))
		copy(out.keys, st.keys)
		out.vals = make([]V, len(st.vals))
		copy(out.vals, st.vals)
	}
	return out
}

// PutIter stores every (key, value) pair of seq, in iteration order —
// for equal keys the last pair wins, exactly as the same Puts would.
// It is a storing operation, so it follows the Put contract and panics
// on a nil or zero-value table even when seq is empty.
// Complexity is O(k·(log₂ n + n)), k = pairs in seq.
func (st *BinarySearchST[K, V]) PutIter(seq iter.Seq2[K, V]) {
	st.putContractCheck("PutIter")
	for k, v := range seq {
		st.Put(k, v)
	}
}

// DeleteFunc removes every entry whose key and value satisfy fx and
// reports whether anything was removed.  The surviving entries keep
// their ascending order; fx runs once per entry.
// Complexity is O(n).
func (st *BinarySearchST[K, V]) DeleteFunc(fx func(k K, v V) bool) (deleted bool) {
	if st == nil || len(st.keys) == 0 {
		return false
	}
	keepK, keepV := st.keys[:0], st.vals[:0]
	for i := range st.keys {
		k, v := st.keys[i], st.vals[i]
		if fx(k, v) {
			deleted = true
			continue
		}
		keepK = append(keepK, k)
		keepV = append(keepV, v)
	}
	if deleted {
		clear(st.keys[len(keepK):]) // zero the vacated slots, the queue precedent
		clear(st.vals[len(keepV):])
		st.keys, st.vals = keepK, keepV
	}
	return deleted
}
