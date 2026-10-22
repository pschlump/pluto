/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package binary_search_st implements BinarySearchST, an ordered symbol
// table (key → value) on two parallel arrays kept sorted by key —
// Sedgewick & Wayne, Algorithms 4th ed., §3.1, BinarySearchST.java.
// Every lookup is a binary search over the key array, the classic
// array-ST trade: O(log₂ n) reads against O(n) writes (Put and Delete
// shift the tail), with no per-entry allocation and no pointer chasing,
// so the constants are as small as an ordered table can get.
//
// Tables whose keys are naturally ordered types (all integers, floats
// and strings) are created with NewBinarySearchST, which orders keys
// with the built-in < and > operators of the key type.  Any other key
// type — including a struct ordered by one of its fields — is created
// with NewBinarySearchSTFunc, which takes a caller supplied comparison
// function; the key type never has to implement an interface, and keys
// and values are stored and returned by value, never boxed.
//
// Basic operations on a BinarySearchST:
//
//	Put — add a key and value; an equal key replaces its value.	O(log₂ n + n)
//	Get — the value bound to a key.								O(log₂ n)
//	Delete — remove a key and its value.							O(log₂ n + n)
//	Contains — the found flag of Get, spelled as a predicate.	O(log₂ n)
//	FindMin / FindMax — smallest / largest key and its value.	O(1)
//	DeleteAtHead / DeleteAtTail — remove the smallest / largest.	O(n)
//	Len / Length / IsEmpty / Truncate — size and reset.			O(1)
//
//	Ordered riders (the skip_list capability set):
//
//	Rank — 0-based position of a key (exact match only).			O(log₂ n)
//	AtIndex — the entry at a 0-based position.					O(1)
//	Floor / Ceil — largest key ≤ key / smallest ≥ key.			O(log₂ n)
//	CountRange — number of keys with lo ≤ key ≤ hi.				O(log₂ n)
//	Range — iterate the entries with lo ≤ key ≤ hi, ascending.	O(log₂ n + m)
//	DeleteRange — remove all keys with lo ≤ key ≤ hi.			O(log₂ n + m)
//	All / Backward — ascending / descending iteration.			O(n)
//
//	Conveniences:
//
//	Keys — fresh []K of the keys, ascending.						O(n)
//	String / StringFunc — render the pairs: "[k:v k:v]".			O(n)
//	Clone — deep copy, no backing array shared.					O(n)
//	PutIter — store every pair of an iter.Seq2.					O(k·(log₂ n + n))
//	DeleteFunc — remove the entries matching a predicate.		O(n)
//	MarshalJSON / UnmarshalJSON — [{"k":key,"v":val},…] ascending.O(n)
//
// A nil *BinarySearchST and the zero value behave as an empty table
// for every operation that has a sane answer: Get reports not-found,
// Delete reports false, iteration yields nothing.  The only panics in
// the package are NewBinarySearchSTFunc(nil) and Put — directly or
// through PutIter or UnmarshalJSON — on a nil table or on a zero-value
// table (no comparison function); each message names the operation and
// the fix (the binary_tree insert contract).
//
// A BinarySearchST is not safe for concurrent use.  It has no _ts twin
// yet (the multiway_pq precedent) — run it single-threaded.
package binary_search_st

import (
	"cmp"
	"fmt"
	"slices"
)

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewBinarySearchST
// and is handy for building custom comparison functions.
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

// BinarySearchST is an ordered symbol table from keys of type K to
// values of type V, on two parallel arrays kept sorted by key.  Use
// NewBinarySearchST for naturally ordered key types (numbers, strings)
// or NewBinarySearchSTFunc for a caller supplied comparison function.
// The zero value is an empty table (reads as empty; Put panics).
type BinarySearchST[K, V any] struct {
	keys []K
	vals []V

	// cmp orders two keys: negative if a sorts before b, 0 if the two
	// are equal, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order K — K
	// itself never has to implement an interface.
	cmp func(a, b K) int
}

// NewBinarySearchST creates a new, empty symbol table for any naturally
// ordered key type (all integers, floats and strings — cmp.Ordered).
// Ordering uses the built-in < and > operators of K; no interface and
// no boxing is involved.
// Complexity is O(1).
func NewBinarySearchST[K cmp.Ordered, V any]() *BinarySearchST[K, V] {
	return &BinarySearchST[K, V]{cmp: Compare[K]}
}

// NewBinarySearchSTFunc creates a new, empty symbol table that orders
// keys with the caller supplied comparison function fx — for example a
// struct ordered by one of its fields.  fx must return a negative value
// if a sorts before b, 0 if the two are equal and a positive value if
// a sorts after b, and must order keys consistently.  It panics if fx
// is nil.
// Complexity is O(1).
func NewBinarySearchSTFunc[K, V any](fx func(a, b K) int) *BinarySearchST[K, V] {
	if fx == nil {
		panic("binary_search_st: NewBinarySearchSTFunc called with a nil comparison function")
	}
	return &BinarySearchST[K, V]{cmp: fx}
}

// putContractCheck applies the Put panic contract for the operations
// that store entries.  `caller` names the public operation in the
// message.
func (st *BinarySearchST[K, V]) putContractCheck(caller string) {
	if st == nil {
		panic(fmt.Sprintf("binary_search_st: %s called on a nil symbol table (create the table with NewBinarySearchST or NewBinarySearchSTFunc)", caller))
	}
	if st.cmp == nil {
		panic(fmt.Sprintf("binary_search_st: %s called on a zero-value symbol table with no comparison function (create the table with NewBinarySearchST or NewBinarySearchSTFunc)", caller))
	}
}

// search returns the index of the first key that compares >= key — the
// insertion position: every key before it sorts strictly less, and a
// key equal to `key`, when present, sits exactly at the returned index.
// The caller is responsible for the empty/nil guards that make a zero
// comparison function unreachable.
func (st *BinarySearchST[K, V]) search(key K) int {
	lo, hi := 0, len(st.keys)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if st.cmp(st.keys[mid], key) < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// IsEmpty reports whether the table holds no entries.  A nil table and
// a zero-value table are empty.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) IsEmpty() bool {
	return st == nil || len(st.keys) == 0
}

// Len returns the number of entries in the table.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) Len() int {
	if st == nil {
		return 0
	}
	return len(st.keys)
}

// Length is an alias for Len; it returns the number of entries in the
// table.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) Length() int {
	if st == nil {
		return 0
	}
	return len(st.keys)
}

// Truncate removes all entries.  The comparison function is kept, so
// the table remains usable and can simply be refilled.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) Truncate() {
	if st == nil {
		return
	}
	st.keys = nil
	st.vals = nil
}

// Put adds the key/value pair.  If the table already holds an equal
// key, its value is replaced and the table does not grow; two Puts
// with equal keys leave exactly one entry.
// Put panics on a nil table or on a zero-value table (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty table.
// Complexity is O(log₂ n) to find the position plus O(n) for the shift.
func (st *BinarySearchST[K, V]) Put(key K, val V) {
	st.putContractCheck("Put")
	i := st.search(key)
	if i < len(st.keys) && st.cmp(st.keys[i], key) == 0 {
		st.vals[i] = val
		return
	}
	st.keys = slices.Insert(st.keys, i, key)
	st.vals = slices.Insert(st.vals, i, val)
}

// Get returns the value bound to key, or found=false when the table
// holds no equal key.  `key` only needs the fields that the table's
// comparison function reads.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Get(key K) (val V, found bool) {
	if st == nil || len(st.keys) == 0 {
		return
	}
	i := st.search(key)
	if i < len(st.keys) && st.cmp(st.keys[i], key) == 0 {
		return st.vals[i], true
	}
	return
}

// Delete removes key and its value, reporting whether the key was
// present.
// Complexity is O(log₂ n) to find the position plus O(n) for the shift.
func (st *BinarySearchST[K, V]) Delete(key K) (found bool) {
	if st == nil || len(st.keys) == 0 {
		return false
	}
	i := st.search(key)
	if i >= len(st.keys) || st.cmp(st.keys[i], key) != 0 {
		return false
	}
	st.keys = slices.Delete(st.keys, i, i+1)
	st.vals = slices.Delete(st.vals, i, i+1)
	return true
}

// Contains reports whether the table holds a key equal to `key` — the
// found flag of Get spelled as a predicate.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Contains(key K) bool {
	_, found := st.Get(key)
	return found
}

// FindMin returns the smallest key and its value.  found is false when
// the table is empty.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) FindMin() (key K, val V, found bool) {
	if st == nil || len(st.keys) == 0 {
		return
	}
	return st.keys[0], st.vals[0], true
}

// FindMax returns the largest key and its value.  found is false when
// the table is empty.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) FindMax() (key K, val V, found bool) {
	if st == nil || len(st.keys) == 0 {
		return
	}
	return st.keys[len(st.keys)-1], st.vals[len(st.vals)-1], true
}

// DeleteAtHead removes the smallest key and its value, reporting
// whether the table was non-empty.
// Complexity is O(n) for the shift.
func (st *BinarySearchST[K, V]) DeleteAtHead() (found bool) {
	if st == nil || len(st.keys) == 0 {
		return false
	}
	st.keys = slices.Delete(st.keys, 0, 1)
	st.vals = slices.Delete(st.vals, 0, 1)
	return true
}

// DeleteAtTail removes the largest key and its value, reporting
// whether the table was non-empty.
// Complexity is O(1).
func (st *BinarySearchST[K, V]) DeleteAtTail() (found bool) {
	if st == nil || len(st.keys) == 0 {
		return false
	}
	st.keys = slices.Delete(st.keys, len(st.keys)-1, len(st.keys))
	st.vals = slices.Delete(st.vals, len(st.vals)-1, len(st.vals))
	return true
}
