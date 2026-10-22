/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st_ts

import (
	"fmt"
	"iter"
	"strings"
)

// Keys returns a fresh slice of the keys in ascending order, copied
// under the read lock.  A nil table and an empty table both yield nil.
// Complexity is O(n).
func (st *BinarySearchST[K, V]) Keys() []K {
	if st == nil {
		return nil
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Keys()
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
// spaces inside the tree_set brackets.  The entries are snapshotted
// under the read lock and the format function runs WITHOUT the lock,
// so it may safely call back into the table.  A nil format function
// renders as "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n) plus the cost of the format function.
func (st *BinarySearchST[K, V]) StringFunc(format func(k K, v V) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, e := range st.entries() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(e.k, e.v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Clone returns a deep copy of the table: a fresh table over the same
// comparison function with the same key→value bindings, copied under
// one hold of the read lock — a consistent snapshot.  No backing array
// is shared, so Put, Delete or DeleteRange on either table afterwards
// never affects the other.  A nil table clones to nil; a zero-value
// table clones to a fresh zero value.
//
// As in the plain package there is no Equal: keys compare through the
// stored comparison function but values have none.
// Complexity is O(n).
func (st *BinarySearchST[K, V]) Clone() *BinarySearchST[K, V] {
	if st == nil {
		return nil
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	if st.cmp == nil {
		return &BinarySearchST[K, V]{} // a zero-value table clones to a fresh zero value
	}
	return &BinarySearchST[K, V]{inner: *st.inner.Clone(), cmp: st.cmp}
}

// PutIter stores every (key, value) pair of seq, in iteration order —
// for equal keys the last pair wins, exactly as the same Puts would.
// The iterator is drained (and any table snapshot it may come from
// taken) BEFORE the write lock is acquired — the lock is never held
// across caller iteration, so feeding a table its own All() is safe —
// and the whole batch is then stored under one hold of the write lock,
// atomic to concurrent readers.
// It is a storing operation, so it follows the Put contract and panics
// on a nil or zero-value table even when seq is empty.
// Complexity is O(k·(log₂ n + n)), k = pairs in seq.
func (st *BinarySearchST[K, V]) PutIter(seq iter.Seq2[K, V]) {
	st.writeContractCheck("PutIter")
	var ks []K
	var vs []V
	for k, v := range seq { // drained before any lock: caller iteration is never under the lock
		ks = append(ks, k)
		vs = append(vs, v)
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	for i := range ks {
		st.inner.Put(ks[i], vs[i])
	}
}

// DeleteFunc removes every entry whose key and value satisfy fx and
// reports whether anything was removed.  The entries are snapshotted
// under the read lock, fx runs on the snapshot WITHOUT any lock held —
// it may safely call back into the table — and the survivors are then
// re-installed under one hold of the write lock, so the whole bulk
// delete is atomic to concurrent readers.  Entries inserted after the
// snapshot are never judged and always survive.  The re-install keeps
// the ascending order (the survivors are re-put in ascending key
// order, each landing at the tail of the growing arrays).
// Complexity is O(n·log₂ n) for the rebuild.
func (st *BinarySearchST[K, V]) DeleteFunc(fx func(k K, v V) bool) (deleted bool) {
	if st == nil || fx == nil {
		return false
	}
	snapshot := st.entries()
	if len(snapshot) == 0 {
		return false
	}
	kept := make([]kv[K, V], 0, len(snapshot))
	for _, e := range snapshot { // fx runs with no lock held
		if fx(e.k, e.v) {
			deleted = true
			continue
		}
		kept = append(kept, e)
	}
	if !deleted {
		return false
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	st.inner.Truncate()
	for _, e := range kept {
		st.inner.Put(e.k, e.v)
	}
	return true
}
