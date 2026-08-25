/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// Package hash_tab implements a generic hash table with a fixed number of
// buckets.  Collisions are resolved by storing the items of each bucket in
// a binary search tree (github.com/pschlump/pluto/binary_tree), so Search,
// Insert and Delete stay O(log(n/k)) even in the worst case, where k is the
// number of buckets.
//
// The key for an item is derived from the item itself: if T (or *T)
// implements Hashable that is used, otherwise fmt.Stringer is used.  Items
// that implement neither cause a panic.
//
// This variant is NOT safe for concurrent use.  Use the hash_tab_bt_ts
// package for a thread-safe version with an identical API.
package hash_tab

import (
	"fmt"
	"hash/fnv"
	"io"

	"github.com/pschlump/pluto/binary_tree"
	"github.com/pschlump/pluto/comparable"
)

// Hashable may be implemented by T (or *T) to supply the hash key used to
// place an item in a bucket.
type Hashable interface {
	HashKey(x any) int
}

// HashTab is a generic hash table with a binary search tree per bucket.
type HashTab[T comparable.Comparable] struct {
	buckets []*binary_tree.BinaryTree[T] // the table
	length  int                          // # of elements in table
	size    int                          // Modulo size for table
}

// NewHashTab returns a new, empty hash table with `n` buckets.
// It panics if n < 5.
// Complexity is O(n).
func NewHashTab[T comparable.Comparable](n int) *HashTab[T] {
	if n < 5 {
		panic("n too small")
	}
	r := HashTab[T]{
		length:  0,
		size:    n,
		buckets: make([]*binary_tree.BinaryTree[T], n),
	}
	for i := range r.buckets {
		r.buckets[i] = binary_tree.NewBinaryTree[T]()
	}
	return &r
}

// IsEmpty returns true if the table is empty.
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all of the data from the table, resetting it to empty.
// Complexity is O(k) where k is the number of buckets.
func (tt *HashTab[T]) Truncate() {
	for i := 0; i < tt.size; i++ {
		tt.buckets[i].Truncate()
	}
	tt.length = 0
}

// Insert will add a new item to the table.  If it is a duplicate of an
// existing item the new item will replace the existing one.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) Insert(item *T) {
	h := tt.bucketOf(item)
	if tt.buckets[h].Insert(item) {
		tt.length++
	}
}

// bucketOf returns the index of the bucket that item hashes to.
func (tt *HashTab[T]) bucketOf(item *T) int {
	h := hash(item) % tt.size
	if h < 0 {
		h = -h
	}
	return h
}

// Len returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	return tt.length
}

// Length returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Length() int {
	return tt.length
}

// Search will look for `find` and return the found item if it is in the
// table.  If it is not found then `nil` will be returned.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) Search(find *T) (rv *T) {
	return tt.NlSearch(find)
}

// NlSearch is the no-lock version of Search.  It is identical to Search in
// this non-thread-safe variant and exists so that code can be written
// against the same API as the thread-safe hash_tab_bt_ts package.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) NlSearch(find *T) (rv *T) {
	if tt.IsEmpty() {
		return nil
	}
	return tt.buckets[tt.bucketOf(find)].Search(find)
}

// ItemExists returns true if `find` is in the table.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) ItemExists(find *T) (rv bool) {
	return tt.Search(find) != nil
}

// WriteLock is a no-op in this non-thread-safe variant.  It exists so that
// code written against the thread-safe hash_tab_bt_ts package compiles
// unchanged against this package.
func (tt *HashTab[T]) WriteLock() {}

// WriteUnlock is a no-op in this non-thread-safe variant.  See WriteLock.
func (tt *HashTab[T]) WriteUnlock() {}

// ReadLock is a no-op in this non-thread-safe variant.  See WriteLock.
func (tt *HashTab[T]) ReadLock() {}

// ReadUnlock is a no-op in this non-thread-safe variant.  See WriteLock.
func (tt *HashTab[T]) ReadUnlock() {}

// Dump will print out the hash table to the writer `fo`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	_, _ = fmt.Fprintf(fo, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		if v.Length() > 0 {
			_, _ = fmt.Fprintf(fo, "bucket [%04d] = \n", i)
			v.Dump(fo)
		}
	}
}

// Walk calls `fx` for every element of the table, walking each non-empty
// bucket in-order.  Iteration order is bucket order, not sorted order.
// If `fx` returns false the walk stops immediately (across all buckets).
// Complexity is O(n).
func (tt *HashTab[T]) Walk(fx binary_tree.ApplyFunction[T], userData any) {
	done := false
	stopFx := func(pos, depth int, data *T, ud interface{}) bool {
		if !fx(pos, depth, data, ud) {
			done = true
			return false
		}
		return true
	}
	for _, v := range tt.buckets {
		if done {
			return
		}
		if v.Length() > 0 {
			v.WalkInOrder(stopFx, userData)
		}
	}
}

// Delete removes an element from the table.  The element can have been
// located with Search or as a result of a match using the Walk function.
// It returns true if the element was found and removed.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	return tt.NlDelete(find)
}

// NlDelete is the no-lock version of Delete.  It is identical to Delete in
// this non-thread-safe variant and exists so that code can be written
// against the same API as the thread-safe hash_tab_bt_ts package.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) NlDelete(find *T) (found bool) {
	if find == nil || tt.IsEmpty() {
		return false
	}
	found = tt.buckets[tt.bucketOf(find)].Delete(find)
	if found {
		tt.length--
	}
	return
}

// hash returns the hash key for x.  It uses the Hashable interface if x
// implements it, then string, then fmt.Stringer.  It panics for any other
// type.
func hash(x any) (rv int) {
	hashstr := func(s string) int {
		h := fnv.New32a()
		_, _ = h.Write([]byte(s))
		return int(h.Sum32())
	}
	if v, ok := x.(Hashable); ok {
		return v.HashKey(x)
	}
	if v, ok := x.(string); ok {
		return hashstr(v)
	}
	if v, ok := x.(fmt.Stringer); ok {
		return hashstr(v.String())
	}
	panic(fmt.Sprintf("Invalid type, %T needs to be Stringer or Hashable interface\n", x))
}

/* vim: set noai ts=4 sw=4: */
