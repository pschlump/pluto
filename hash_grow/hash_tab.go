package hash_grow_ts

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

	Walk - Walk the table
	Print - Using Walk to print out the contents of the table.

*/

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/pschlump/MiscLib"
	"github.com/pschlump/dbgo"
	"github.com/pschlump/pluto/comparable"
)

type FlagType struct {
	IsNotData bool
	Mixin     string // data to mixin if collision
}

// HashTab is a generic binary tree
type HashTab[T comparable.Comparable] struct {
	buckets             []*T // the table
	flags               []FlagType
	length              int     // # of elements in table
	saturationThreshold float64 // Proportion before grow of table. (default 0.5)
	size                int     // Modulo size for table	Current Size!
	lock                sync.RWMutex
}

type Hashable interface {
	HashKey(x interface{}) int
}

// Complexity is O(1).
func NewHashTab[T comparable.Comparable](n int, saturation float64) *HashTab[T] {
	if n < 5 {
		panic("n too small")
	}
	if saturation == 0 {
		saturation = 0.5
	}
	return &HashTab[T]{
		length:              0,
		size:                n,
		saturationThreshold: saturation,
		buckets:             make([]*T, n, n),
		flags:               make([]FlagType, n, n),
	}
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
		tt.buckets[i] = nil
		tt.flags[i].IsNotData = false
		tt.flags[i].Mixin = ""
	}
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Insert(item *T) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	h := hash(item) % tt.size
	new_key := false
	np := 0

	dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
	if tt.buckets[h] == nil {
		dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
		tt.buckets[h] = item
		tt.length++
		new_key = true
	} else if (*item).Compare(*tt.buckets[h]) == 0 {
		dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
		tt.buckets[h] = item // Replace, This means that you don't have a new key.
	} else {
		dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
		// collision, something already at tt.buckets[h] (original)
		oh := h
		mixin := genRandomMixin()
		dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF) original mixin ->%s<-\n", mixin)
		tt.flags[oh] = FlagType{
			IsNotData: true,
			Mixin:     mixin,
		}
		// must_split := false
		np = 0
		for np < 20 { // likely number of loops iterations is less than log 2 decreasing proability because table is less than 1/2 full.
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
			h = rehash(item, mixin) % tt.size
			if tt.buckets[h] == nil { // Found an empty, so put it in and save.
				dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
				tt.buckets[h] = item
				tt.flags[oh] = FlagType{
					IsNotData: true,
					Mixin:     mixin,
				}
				break
			} else if tt.flags[h].IsNotData == false { // Already a colision
				mixin = tt.flags[h].Mixin
				dbgo.Fprintf(os.Stderr, "%(yellow)AT:%(LF) -- new mixin ->%s<-\n", mixin)
				np++
			} else {
				// tt.bucket[h] != nil and tt.flags[h].IsNotData == true, this imples a non-colision.
				dbgo.Fprintf(os.Stderr, "%(red)AT:%(LF)\n")
				mixin := genRandomMixin()
				tt.flags[oh] = FlagType{
					IsNotData: true,
					Mixin:     mixin,
				}
				np++
			}
		}
		if np >= 20 {
			// stick in anywhere, will split
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
		} else {
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
			new_key = true
			tt.length++
		}
	}

	dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
	if new_key || np >= 20 {
		dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
		if (((float64)(tt.length))/((float64)(tt.size))) > tt.saturationThreshold || np >= 20 {
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
			dbgo.Fprintf(os.Stderr, "%(yellow)Passed Threshold for size, will double.......................................................\n")
			n := tt.size * 2 // xyzzy - improve this., should be prime lookup table and double size, then go up to nex larger value.
			dbgo.Fprintf(os.Stderr, "%(yellow)    new size(n) = %d\n", n)
			newBuckets := make([]*T, n, n)
			newFlags := make([]FlagType, n, n)
			// xyzzy - Rehash, double in size etc.
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
			for i := 0; i < tt.size; i++ {
				dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
				item := tt.buckets[h]
				h := hash(item) % n
				if newBuckets[h] == nil {
					dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
					newBuckets[h] = item
				} else {
					dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
					oh := h // old h, save for later
					for {   // likely number of loops iterations is (log 2)/2 decreasing proability.
						dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
						mixin := genRandomMixin()
						h = rehash(item, mixin) % n
						if newBuckets[h] == nil {
							newFlags[oh] = FlagType{
								IsNotData: true,
								Mixin:     mixin,
							}
							break
						} // xyzzy - error - run test.
					}
				}
			}
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
			tt.buckets = newBuckets
			tt.flags = newFlags
			tt.size = n
		}
	}
}

func genRandomMixin() string {
	return RandStringRunes(4)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}
func (tt *HashTab[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
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
	if tt.nlIsEmpty() {
		return nil
	}
	h := hash(find) % tt.size
	if db1 {
		fmt.Printf("%sh=%d - for ->%+v<-%s\n", MiscLib.ColorYellow, h, find, MiscLib.ColorReset)
	}
	// xyzzy TODO - fix
	// rv = tt.buckets[h].Search(find) // func (ns *BinaryTree[T]) Search(t *T) (rv *BinaryTreeElement[T], pos int) {
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
func (tt *HashTab[T]) Dump(fo io.Write) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	fmt.Printf("Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		fmt.Fprintf(fp, "bucket [%04d] = %v\n", i, v) // v.Dump(fo) // xyzzy TODO - fix
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
	if find == nil || tt.nlIsEmpty() {
		return false
	}
	h := hash(find) % tt.size
	_ = h
	// xyzzy TODO - fix
	// found = tt.buckets[h].Delete(find)
	if found {
		tt.length--
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

func rehash(x interface{}, ss string) (rv int) {
	hashstr := func(s string) int {
		h := fnv.New32a()
		h.Write([]byte(s))
		h.Write([]byte(ss))
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
