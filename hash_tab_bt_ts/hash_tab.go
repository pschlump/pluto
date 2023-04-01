package hash_tab_ts_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Basic operations on a Hash Table.

* 	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)
* 	IsEmpty — Returns true if the linked list is empty											O(1)
* 	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
* 	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n/k) where k is # of buckets.
* 	Truncate - Delete all the nodes in list. 													O(1)

*	Walk - Walk the table
	Print - Using Walk to print out the contents of the table.

*/

import (
	"fmt"
	"hash/fnv"
	"io"
	"sync"

	"github.com/pschlump/MiscLib"
	binary_tree "github.com/pschlump/pluto/binary_tree_ts"
	"github.com/pschlump/pluto/comparable"
)

// HashTab is a generic binary tree
type HashTab[T comparable.Comparable] struct {
	buckets [](*binary_tree.BinaryTree[T]) // the table
	length  int                            // # of elements in table
	size    int                            // Modulo size for table
	lock    sync.RWMutex
}

type Hashable interface {
	HashKey(x interface{}) int
}

// Complexity is O(1).
func NewHashTab[T comparable.Comparable](n int) *HashTab[T] {
	if n < 5 {
		panic("n too small")
	}
	r := HashTab[T]{
		length: 0,
		size:   n,
	}
	r.buckets = make([](*binary_tree.BinaryTree[T]), n, n)
	for i := 0; i < n; i++ {
		r.buckets[i] = binary_tree.NewBinaryTree[T]()
	}
	return &r
}

// IsEmpty will return true if the binary-tree is empty
// Complexity is O(1).
func (tt HashTab[T]) IsEmpty() bool {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length == 0
}

func (tt HashTab[T]) nlIsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (tt *HashTab[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	for i := 0; i < tt.size; i++ {
		tt.buckets[i].Truncate()
	}
	(*tt).length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Insert(item *T) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	h := hash(item) % tt.size
	isNew := tt.buckets[h].Insert(item)
	if isNew {
		(*tt).length++
	}
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return (*tt).length
}
func (tt *HashTab[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return (*tt).length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Search(find *T) (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlSearch(find)
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) NlSearch(find *T) (rv *T) {
	if (*tt).nlIsEmpty() {
		return nil
	}
	h := hash(find) % tt.size
	if db1 {
		fmt.Printf("%sh=%d - for ->%+v<-%s\n", MiscLib.ColorYellow, h, find, MiscLib.ColorReset)
	}
	rv = tt.buckets[h].Search(find) // func (ns *BinaryTree[T]) Search(t *T) (rv *BinaryTreeElement[T], pos int) {
	if rv == nil {
		return nil
	}
	return
}

// ht.WriteLock()
// ht.WriteUnlock()
// ht.ReadLock()
// ht.ReadUnlock()
func (tt *HashTab[T]) WriteLock() {
	tt.lock.Lock()
}
func (tt *HashTab[T]) WriteUnlock() {
	tt.lock.Unlock()
}
func (tt *HashTab[T]) ReadLock() {
	tt.lock.RLock()
}
func (tt *HashTab[T]) ReadUnlock() {
	tt.lock.RUnlock()
}

// Dump will print out the hash table to the file `fo`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	fmt.Fprintf(fo, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		if v.Length() > 0 {
			fmt.Fprintf(fo, "bucket [%04d] = \n", i)
			v.Dump(fo)
		}
	}
}

// type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool
// func (tt *BinaryTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {

func (tt *HashTab[T]) Walk(fx binary_tree.ApplyFunction[T], userData interface{}) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for _, v := range tt.buckets {
		if v.Length() > 0 {
			v.WalkInOrder(fx, userData)
		}
	}
}

// Delete an element from the hash_tab. The element needs to have been
// located with "Search" or as a result of a match using the Walk function.
// Complexity is O(1)
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDelete(find)
}

func (tt *HashTab[T]) NlDelete(find *T) (found bool) {
	if find == nil || (*tt).nlIsEmpty() {
		return false
	}
	h := hash(find) % tt.size
	found = tt.buckets[h].Delete(find)
	if found {
		(*tt).length--
	}
	return
}

func hash(x interface{}) (rv int) {
	hashstr := func(s string) int {
		h := fnv.New32a()
		h.Write([]byte(s))
		return int(h.Sum32())
	}
	if v, ok := x.(Hashable); ok {
		h := v.HashKey(x)
		return int(h)
	}
	if v, ok := x.(string); ok {
		h := hashstr(v)
		return h
	}
	if v, ok := x.(fmt.Stringer); ok {
		h := hashstr(v.String())
		return int(h)
	}
	panic(fmt.Sprintf("Invalid type, %T needs to be Stringer or Hashable interface\n", x))
}

const db1 = false

/* vim: set noai ts=4 sw=4: */
