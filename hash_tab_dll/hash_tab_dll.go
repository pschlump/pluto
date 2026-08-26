/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_tab_dll implements a generic hash table with a fixed number
// of buckets in which every bucket is a doubly linked list (charon/dll).
// Collisions chain in the bucket's list, exactly like the singly chained
// hash_tab — what the dll buckets add is O(1) deletion of a previously
// located element: Search returns a live element handle and DeleteFound
// splices that element out through its prev/next pointers without
// re-walking the chain.  The bucket count is fixed at construction; when
// the number of elements is not known up front use the auto-growing
// hash_grow package instead.
//
// It is a rework of github.com/pschlump/pluto/hash_tab_dll in which the
// comparable.Equality interface constraint (and the Hashable/Stringer
// hashing on top of it) has been replaced with plain Go type parameters,
// so element data is never boxed into an interface and never unboxed with
// a type assertion.  The buckets are charon/dll lists because a bucket
// that is a doubly linked list with O(1) splice-out is exactly what dll
// is (pluto composed its dll the same way) — the fourth intra-charon
// composition after priority_queue/heap, heap_sort/heap and
// hash_tab_bt/binary_tree.  Tables of types that can be compared with ==
// (the builtin comparable constraint) are created with NewHashTab, which
// hashes with the stdlib hash/maphash — every table gets its own random
// seed, equal values always hash equal, and no method has to be
// implemented.  Tables of any other type — or with field-based equality —
// are created with NewHashTabFunc, which takes a caller supplied equality
// function and hash function; the element type does not have to implement
// any interface.  The two functions must agree: whenever eq(a, b) is
// true, hash(a) and hash(b) must be equal.
//
// Elements are stored by value (T, not *T) inside the dll nodes.  The
// element handles Search returns are the one pointer-bearing find in the
// charon hash-table family, inherited from dll's own Search (see that
// package) because the handle is what makes DeleteFound O(1);
// DllElement.GetData returns the element by value, and a handle stays
// valid across an Insert replacement (the replacement is SetData on the
// same node).
//
// Operations:
//
//	Insert — add a new element to the table, replacing any existing equal element.	O(1) average, O(n) worst
//	Delete — delete the element equal to `find`, if present.					O(1) average, O(n) worst
//	DeleteFound — delete a previously located element.								O(1)
//	Search — return a handle to the stored element equal to `find`.				O(1) average, O(n) worst
//	IsEmpty — Returns true if the table is empty.									O(1)
//	Len / Length — Returns number of elements in the table.  0 length is empty.	O(1)
//	Truncate — Delete all the elements in the table.								O(k)
//	Walk — Call a callback for each element in bucket order.						O(n)
//	Dump — Write a per-bucket listing of the table for debugging.					O(n)
//	All / Values — Range-over-func iterators in bucket order.						O(n)
//
// A nil *HashTab and the zero value both behave as an empty table for
// every read: searches report not-found, Delete and DeleteFound return
// false, and the iterators visit nothing.
//
// The package panics in exactly four situations, all programmer errors
// that cannot be handled where they occur — each message names the fix:
//
//	NewHashTabFunc with a nil equality or hash function — caught at construction.
//	NewHashTab/NewHashTabFunc with n < 5 — a smaller table has no headroom.
//	Insert on a nil table — a nil table cannot store an element.
//	Insert on a zero-value table — no equality/hash functions; the message names the constructors.
//
// This version of the table is not suitable for concurrent usage; use the
// thread-safe hash_tab_dll_ts twin (or the auto-growing hash_grow_ts when
// the size is not known up front) when the table is shared between
// goroutines.
package hash_tab_dll

import (
	"fmt"
	"hash/maphash"
	"io"

	"github.com/pschlump/charon/dll"
)

// HashTab is a generic hash table with a fixed number of buckets, each of
// which is a doubly linked list.  Use NewHashTab for element types that
// support ==, or NewHashTabFunc for a caller supplied equality and hash
// function.  The zero value is an empty read-only table.
type HashTab[T any] struct {
	buckets []*dll.Dll[T] // one list per bucket
	size    int           // number of buckets; fixed at construction
	length  int           // number of elements in the table

	// eq reports whether two elements are considered the same, and hash
	// returns a hash for an element.  Both are set by the constructors and
	// are the only things that know how to compare and hash T — T itself
	// never has to implement an interface.  They must agree: equal elements
	// must have equal hashes.  Every bucket list is constructed with the
	// same eq, so the table and its buckets always agree on equality.
	eq   func(a, b T) bool
	hash func(a T) uint64
}

// -------------------------------------------------------------------------------------------------------

// NewHashTab creates a hash table with n buckets (n must be at least 5).
// Elements are compared with the == operator and hashed with the stdlib
// hash/maphash using a per-table random seed — no method has to be
// implemented on T, and no element is ever boxed into an interface.
// Complexity is O(n) for the bucket allocation.
func NewHashTab[T comparable](n int) *HashTab[T] {
	var seed = maphash.MakeSeed()
	return newHashTab(
		n,
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
		"NewHashTab",
	)
}

// NewHashTabFunc creates a hash table with n buckets (n must be at least
// 5), a caller supplied equality function and a caller supplied hash
// function.  The two functions must agree: whenever eq(a, b) is true,
// hash(a) and hash(b) must be equal, otherwise Search and Delete can miss
// elements.
// Complexity is O(n) for the bucket allocation.
func NewHashTabFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, n int) *HashTab[T] {
	return newHashTab(n, eq, hash, "NewHashTabFunc")
}

// newHashTab is the shared constructor body; `caller` names the public
// constructor in panic messages.
func newHashTab[T any](n int, eq func(a, b T) bool, hash func(a T) uint64, caller string) *HashTab[T] {
	if eq == nil {
		panic(fmt.Sprintf("hash_tab_dll: %s called with a nil equality function", caller))
	}
	if hash == nil {
		panic(fmt.Sprintf("hash_tab_dll: %s called with a nil hash function", caller))
	}
	if n < 5 {
		panic(fmt.Sprintf("hash_tab_dll: %s called with n = %d, the initial size must be at least 5", caller, n))
	}
	r := &HashTab[T]{
		buckets: make([]*dll.Dll[T], n),
		size:    n,
		length:  0,
		eq:      eq,
		hash:    hash,
	}
	for i := range r.buckets {
		r.buckets[i] = dll.NewDllFunc(eq)
	}
	return r
}

// IsEmpty will return true if the hash table is empty.
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	return tt == nil || tt.length == 0
}

// Truncate removes all data from the table by emptying every bucket list.
// The bucket count and the equality/hash functions are kept (each bucket
// list keeps its copy of the equality function), so the table remains
// usable and can simply be refilled.
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
// reserved zero value to remap: an empty bucket is an empty list, not a
// hash marker, so a hash of 0 is just another hash.
func (tt *HashTab[T]) hashOf(a T) uint64 {
	return tt.hash(a)
}

// bucketOf returns the bucket index for the raw hash `h`.
func (tt *HashTab[T]) bucketOf(h uint64) int {
	return int(h % uint64(tt.size))
}

// Insert will add a new item to the table.  If an equal item is already
// in the bucket list its node is updated in place with the new value
// (handles previously returned by Search stay valid and observe the new
// value) and false is returned; true is returned when a new element was
// added at the head of its bucket list.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Insert(item T) bool {
	if tt == nil {
		panic("hash_tab_dll: Insert called on a nil table")
	}
	if tt.eq == nil || tt.hash == nil {
		panic("hash_tab_dll: Insert called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
	}
	bucket := tt.buckets[tt.bucketOf(tt.hashOf(item))]
	if el, pos := bucket.Search(item); pos >= 0 { // equal element present: replace it
		el.SetData(item)
		return false
	}
	bucket.InsertBeforeHead(item) // push at the list head, ahead of older collisions
	tt.length++
	return true
}

// Search will look for `find` in the bucket it hashes to and return a
// handle to the stored element equal to it.  If it is not found nil and
// false are returned.  The handle is a live node of the bucket list: pass
// it to DeleteFound for an O(1) removal, or call GetData on it for the
// stored value — the element data itself is kept by value.  `find` only
// needs the fields that the equality and hash functions read.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Search(find T) (el *dll.DllElement[T], found bool) {
	if tt == nil || tt.eq == nil || tt.length == 0 {
		return // nil table, zero value or empty table: not found
	}
	el, pos := tt.buckets[tt.bucketOf(tt.hashOf(find))].Search(find)
	return el, pos >= 0
}

// Delete an element from the table.  The element equal to `find` is
// located with the same chain walk Search uses, then unlinked through its
// prev/next pointers.  Returns true if the element was found and removed.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Delete(find T) (found bool) {
	if tt == nil || tt.eq == nil || tt.length == 0 {
		return false
	}
	found = tt.buckets[tt.bucketOf(tt.hashOf(find))].Delete(find) == nil
	if found {
		tt.length--
	}
	return
}

// DeleteFound removes an element that was previously located with Search,
// returning true if the element was removed.  The bucket is found by
// hashing the element's own data, so no chain is walked — this is the
// operation the dll buckets exist for.  The element must be a live
// element of this table (the same contract as dll.DeleteFound): deleting
// an element twice, or passing an element of another table, is undefined.
// Complexity is O(1).
func (tt *HashTab[T]) DeleteFound(el *dll.DllElement[T]) (found bool) {
	if tt == nil || tt.length == 0 || el == nil {
		return false
	}
	found = tt.buckets[tt.bucketOf(tt.hashOf(el.GetData()))].DeleteFound(el) == nil
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
// walk (the same convention as hash_tab and the tree packages; note dll's
// own Walk is the opposite).
type ApplyFunction[T any] func(pos int, data T) bool

// Walk calls `fx` for each element in the table, in bucket order and —
// within a bucket — from the most recently inserted element to the oldest
// (the bucket lists run head-newest to tail-oldest), until all elements
// have been visited or `fx` returns false.  It returns true if the walk
// ran to completion.
// Complexity is O(n).
func (tt *HashTab[T]) Walk(fx ApplyFunction[T]) (b bool) {
	b = true
	if tt == nil || tt.length == 0 {
		return
	}
	for ii := range tt.buckets {
		for _, data := range tt.buckets[ii].All() {
			if !fx(ii, data) {
				return false
			}
		}
	}
	return
}

// Dump will print out the hash table, including empty buckets, to `fo` —
// the element count and modulo size on the first line, then one line per
// element (a bucket with a chain of several elements prints one line per
// element).  Unlike hash_tab's Dump no hash values are shown: the dll
// nodes do not keep them.  The bucket assignment depends on the per-table
// hash seed, so the output varies from process to process; use it for
// debugging, not for golden files.
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
		for _, data := range tt.buckets[i].All() {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] = %v\n", i, data)
		}
	}
}
