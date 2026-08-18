/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// Package hash_tab_bt_ts implements a generic hash table with a fixed
// number of buckets.  Collisions are resolved by storing the items of each
// bucket in a binary search tree
// (github.com/pschlump/pluto/binary_tree_ts), so Search, Insert and Delete
// stay O(log(n/k)) even in the worst case, where k is the number of
// buckets.
//
// The key for an item is derived from the item itself: if T (or *T)
// implements Hashable that is used, otherwise fmt.Stringer is used.  Items
// that implement neither cause a panic.
//
// This is the thread-safe variant: every exported method is guarded by a
// sync.RWMutex.  WriteLock/WriteUnlock/ReadLock/ReadUnlock allow callers to
// hold the lock across multiple operations, using the Nl-prefixed
// (no-lock) methods while the lock is held.  Use the hash_tab_bt package
// for a non-thread-safe version with an identical API.
package hash_tab_bt_ts

import (
	"fmt"
	"hash/fnv"
	"io"
	"sync"

	binary_tree "github.com/pschlump/pluto/binary_tree_ts"
	"github.com/pschlump/pluto/comparable"
)

// Hashable may be implemented by T (or *T) to supply the hash key used to
// place an item in a bucket.
type Hashable interface {
	HashKey(x any) int
}

// HashTab is a generic hash table with a binary search tree per bucket.
// It is safe for concurrent use.
type HashTab[T comparable.Comparable] struct {
	buckets []*binary_tree.BinaryTree[T] // the table
	length  int                          // # of elements in table
	size    int                          // Modulo size for table
	lock    sync.RWMutex
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
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlIsEmpty()
}

// nlIsEmpty is the no-lock version of IsEmpty; the caller must hold the lock.
func (tt *HashTab[T]) nlIsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all of the data from the table, resetting it to empty.
// Complexity is O(k) where k is the number of buckets.
func (tt *HashTab[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	for i := 0; i < tt.size; i++ {
		tt.buckets[i].Truncate()
	}
	tt.length = 0
}

// Insert will add a new item to the table.  If it is a duplicate of an
// existing item the new item will replace the existing one.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) Insert(item *T) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	h := tt.bucketOf(item)
	if tt.buckets[h].Insert(item) {
		tt.length++
	}
}

// bucketOf returns the index of the bucket that item hashes to.
// The caller must hold at least a read lock.
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
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Length returns the number of elements in the table.
// Complexity is O(1).
func (tt *HashTab[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Search will look for `find` and return the found item if it is in the
// table.  If it is not found then `nil` will be returned.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) Search(find *T) (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlSearch(find)
}

// NlSearch is the no-lock version of Search.  The caller must hold at
// least a read lock (see ReadLock/WriteLock).  It is useful for batching
// several operations under a single lock hold.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) NlSearch(find *T) (rv *T) {
	if tt.nlIsEmpty() {
		return nil
	}
	return tt.buckets[tt.bucketOf(find)].Search(find)
}

// ItemExists returns true if `find` is in the table.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) ItemExists(find *T) (rv bool) {
	return tt.Search(find) != nil
}

// WriteLock acquires the table write lock.  Use the Nl-prefixed methods
// (NlSearch, NlDelete) while the lock is held, then call WriteUnlock.
func (tt *HashTab[T]) WriteLock() {
	tt.lock.Lock()
}

// WriteUnlock releases the table write lock acquired by WriteLock.
func (tt *HashTab[T]) WriteUnlock() {
	tt.lock.Unlock()
}

// ReadLock acquires the table read lock.  Use the Nl-prefixed methods
// while the lock is held, then call ReadUnlock.
func (tt *HashTab[T]) ReadLock() {
	tt.lock.RLock()
}

// ReadUnlock releases the table read lock acquired by ReadLock.
func (tt *HashTab[T]) ReadUnlock() {
	tt.lock.RUnlock()
}

// Dump will print out the hash table to the writer `fo`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
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
// The table is read-locked for the duration of the walk; `fx` must not
// call back into the table or it will deadlock.
// Complexity is O(n).
func (tt *HashTab[T]) Walk(fx binary_tree.ApplyFunction[T], userData any) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for _, v := range tt.buckets {
		if v.Length() > 0 {
			v.WalkInOrder(fx, userData)
		}
	}
}

// Delete removes an element from the table.  The element can have been
// located with Search or as a result of a match using the Walk function.
// It returns true if the element was found and removed.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDelete(find)
}

// NlDelete is the no-lock version of Delete.  The caller must hold the
// write lock (see WriteLock).  It is useful for batching several
// operations under a single lock hold.
// Complexity is O(log(n/k)) where k is the number of buckets.
func (tt *HashTab[T]) NlDelete(find *T) (found bool) {
	if find == nil || tt.nlIsEmpty() {
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
