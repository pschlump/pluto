/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_tab_bt implements a generic hash table with a fixed number
// of buckets in which every bucket is a binary search tree
// (pluto/binary_tree).  Collisions are resolved by the tree instead of a
// chain — the hash_grow/hash_tab sibling of this package — so Search,
// Insert and Delete stay logarithmic in the number of elements per bucket
// even when many keys collide: O(log(n/k)) on average with k buckets,
// where the chained hash_tab degrades to O(n/k) on the same load.
//
// Element data is never boxed into an interface and never unboxed with
// a type assertion.  Because a bucket is a search tree the element type
// needs an ordering, not just an equality: tables of naturally ordered
// types are created with NewHashTab, which orders with the type's builtin
// < and > operators (via binary_tree.Compare) and hashes with the stdlib
// hash/maphash — every table gets its own random seed, equal values always
// hash equal, and no method has to be implemented.  Tables of any other
// type — or with field-based ordering — are created with NewHashTabFunc,
// which takes a caller supplied comparison function and a caller supplied
// hash function.  The two functions must agree: whenever cmp(a, b) == 0,
// hash(a) and hash(b) must be equal, otherwise Search and Delete look for
// the element in a different bucket than it is stored in.
//
// Elements are stored and returned by value (T, not *T).
//
// Operations:
//
//	Insert — add a new element to the table, replacing an equal element.	O(log(n/k)) average, O(n) worst
//	Delete — delete the element equal to `find`, if present.					O(log(n/k)) average, O(n) worst
//	Search — return the stored element equal to `find`.						O(log(n/k)) average, O(n) worst
//	IsEmpty — Returns true if the table is empty.								O(1)
//	Len / Length — Returns number of elements in the table.  0 length is empty.	O(1)
//	Truncate — Delete all the elements in the table.							O(k)
//	Walk — Call a callback for each element in bucket order.					O(n)
//	Dump — Write a per-bucket listing of the table for debugging.				O(n)
//	All / Values — Range-over-func iterators in bucket order.					O(n)
//
// Within a bucket the elements are visited by the tree's in-order walk,
// that is in ascending order per the comparison function — unlike the
// chained hash_tab, whose buckets run newest-first.  Bucket order itself
// depends on the hash function and, for NewHashTab, on the per-table
// random seed, so the combined iteration order is nondeterministic and
// must not appear in fixed assertions.
//
// A nil *HashTab and the zero value both behave as an empty table for
// every read: searches report not-found, Delete returns false, and the
// iterators visit nothing.
//
// The package panics in exactly four situations, all programmer errors
// that cannot be handled where they occur — each message names the fix:
//
//	NewHashTabFunc with a nil comparison or hash function — caught at construction.
//	NewHashTab/NewHashTabFunc with n < 5 — a smaller table has no headroom.
//	Insert on a nil table — a nil table cannot store an element.
//	Insert on a zero-value table — no comparison/hash functions; the message names the constructors.
//
// This version of the table is not suitable for concurrent usage; a mutex
// guarded thread-safe twin, hash_tab_bt_ts, has the exact same interface.
package hash_tab_bt

import (
	"cmp"
	"fmt"
	"hash/maphash"
	"io"

	"github.com/pschlump/pluto/binary_tree"
)

// HashTab is a generic hash table with a fixed number of buckets, each of
// which is a binary search tree.  Use NewHashTab for element types with a
// builtin ordering, or NewHashTabFunc for a caller supplied comparison
// and hash function.  The zero value is an empty read-only table.
type HashTab[T any] struct {
	buckets []*binary_tree.BinaryTree[T] // one tree per bucket
	size    int                          // number of buckets; fixed at construction
	length  int                          // number of elements in the table

	// cmp orders the elements within a bucket's tree and defines equality
	// (cmp(a, b) == 0); hash picks the bucket an element belongs to.  Both
	// are set by the constructors and are the only things that know how to
	// compare and hash T — T itself never has to implement an interface.
	// They must agree: elements that compare equal must have equal hashes.
	cmp  func(a, b T) int
	hash func(a T) uint64
}

// -------------------------------------------------------------------------------------------------------

// NewHashTab creates a hash table with n buckets (n must be at least 5).
// Elements are ordered with the type's builtin < and > operators and
// hashed with the stdlib hash/maphash using a per-table random seed — no
// method has to be implemented on T, and no element is ever boxed into an
// interface.
// Complexity is O(n) for the bucket allocation.
func NewHashTab[T interface {
	cmp.Ordered
	comparable
}](n int) *HashTab[T] {
	var seed = maphash.MakeSeed()
	return newHashTab(
		n,
		binary_tree.Compare[T],
		func(a T) uint64 { return maphash.Comparable(seed, a) },
		"NewHashTab",
	)
}

// NewHashTabFunc creates a hash table with n buckets (n must be at least
// 5), a caller supplied comparison function and a caller supplied hash
// function.  The comparison function orders the tree inside each bucket
// (equal elements return 0); the hash function picks the bucket.  The two
// functions must agree: whenever cmp(a, b) == 0, hash(a) and hash(b) must
// be equal, otherwise Search and Delete can miss elements.
// Complexity is O(n) for the bucket allocation.
func NewHashTabFunc[T any](cmp func(a, b T) int, hash func(a T) uint64, n int) *HashTab[T] {
	return newHashTab(n, cmp, hash, "NewHashTabFunc")
}

// newHashTab is the shared constructor body; `caller` names the public
// constructor in panic messages.
func newHashTab[T any](n int, cmp func(a, b T) int, hash func(a T) uint64, caller string) *HashTab[T] {
	if cmp == nil {
		panic(fmt.Sprintf("hash_tab_bt: %s called with a nil comparison function", caller))
	}
	if hash == nil {
		panic(fmt.Sprintf("hash_tab_bt: %s called with a nil hash function", caller))
	}
	if n < 5 {
		panic(fmt.Sprintf("hash_tab_bt: %s called with n = %d, the initial size must be at least 5", caller, n))
	}
	r := &HashTab[T]{
		buckets: make([]*binary_tree.BinaryTree[T], n),
		size:    n,
		length:  0,
		cmp:     cmp,
		hash:    hash,
	}
	for i := range r.buckets {
		r.buckets[i] = binary_tree.NewBinaryTreeFunc(cmp)
	}
	return r
}

// IsEmpty will return true if the hash table is empty.
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	return tt == nil || tt.length == 0
}

// Truncate removes all data from the table by emptying every bucket tree.
// The bucket count and the comparison/hash functions are kept, so the
// table remains usable and can simply be refilled.
// Complexity is O(k), k = number of buckets.
func (tt *HashTab[T]) Truncate() {
	if tt == nil {
		return
	}
	for _, b := range tt.buckets {
		b.Truncate()
	}
	tt.length = 0
}

// hashOf returns the raw hash of `a`.  Unlike hash_grow there is no
// reserved zero value to remap: an empty bucket is an empty tree, not a
// hash marker, so a hash of 0 is just another hash.
func (tt *HashTab[T]) hashOf(a T) uint64 {
	return tt.hash(a)
}

// bucketOf returns the bucket index for the raw hash `h`.
func (tt *HashTab[T]) bucketOf(h uint64) int {
	return int(h % uint64(tt.size))
}

// Insert will add a new item to the table.  If an equal item (cmp == 0) is
// already in the bucket tree it is replaced by the new one and false is
// returned; true is returned when a new element was added.
// Complexity is O(log(n/k)) average, O(n) worst case.
func (tt *HashTab[T]) Insert(item T) bool {
	if tt == nil {
		panic("hash_tab_bt: Insert called on a nil table")
	}
	if tt.cmp == nil || tt.hash == nil {
		panic("hash_tab_bt: Insert called on a table with no comparison/hash functions (create the table with NewHashTab or NewHashTabFunc)")
	}
	if tt.buckets[tt.bucketOf(tt.hashOf(item))].Insert(item) {
		tt.length++
		return true
	}
	return false
}

// Search will look for `find` in the bucket it hashes to and return the
// stored element equal to it.  If it is not found the zero value of T and
// false are returned.  `find` only needs the fields that the comparison
// and hash functions read.
// Complexity is O(log(n/k)) average, O(n) worst case.
func (tt *HashTab[T]) Search(find T) (rv T, found bool) {
	if tt == nil || tt.cmp == nil || tt.hash == nil || tt.length == 0 {
		return // nil table, zero value or empty table: not found
	}
	return tt.buckets[tt.bucketOf(tt.hashOf(find))].Search(find)
}

// Delete an element from the table.  The element equal to `find` is
// located in its bucket tree with the same descent Search uses, then
// removed by the tree's delete (the two-children case promotes the
// in-order successor).  Returns true if the element was found and removed.
// Complexity is O(log(n/k)) average, O(n) worst case.
func (tt *HashTab[T]) Delete(find T) (found bool) {
	if tt == nil || tt.cmp == nil || tt.hash == nil || tt.length == 0 {
		return false
	}
	found = tt.buckets[tt.bucketOf(tt.hashOf(find))].Delete(find)
	if found {
		tt.length--
	}
	return
}

// Len returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Length returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Length() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// ApplyFunction is the callback type for Walk.  It is called with the
// bucket position and the element stored there.  Returning false stops the
// walk (the same convention as hash_tab and the tree packages; note
// dll/sll are the opposite).
type ApplyFunction[T any] func(pos int, data T) bool

// Walk calls `fx` for each element in the table, in bucket order and —
// within a bucket — in the tree's in-order (ascending per the comparison
// function), until all elements have been visited or `fx` returns false.
// It returns true if the walk ran to completion.
// Complexity is O(n).
func (tt *HashTab[T]) Walk(fx ApplyFunction[T]) (b bool) {
	b = true
	if tt == nil || tt.length == 0 {
		return
	}
	for ii := range tt.buckets {
		for data := range tt.buckets[ii].All() {
			if !fx(ii, data) {
				return false
			}
		}
	}
	return
}

// Dump will print out the hash table, including empty buckets, to `fo` —
// the element count and modulo size on the first line, then one line per
// element (a bucket with a tree of several elements prints one line per
// element, in ascending order).  Unlike hash_tab's Dump no hash values
// are shown: the tree nodes do not keep them.  The bucket assignment
// depends on the per-table hash seed, so the output varies from process to
// process; use it for debugging, not for golden files.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	if tt == nil {
		_, _ = fmt.Fprintf(fo, "Elements: 0, mod size:0\n")
		return
	}
	_, _ = fmt.Fprintf(fo, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i := range tt.buckets {
		if tt.buckets[i].IsEmpty() {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] empty\n", i)
			continue
		}
		for data := range tt.buckets[i].All() {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] = %v\n", i, data)
		}
	}
}
