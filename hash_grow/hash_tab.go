package hash_grow

// Non locking hash table for use with a DLL for timeout.

/*
Copyright (C) Philip Schlump, 2023.

BSD 3 Clause Licensed. See ../LICENSE
*/

/*

Basic operations on a Hash Table.

* 	Insert - create a new element in tree.														O(log|2(n))
*  	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(log|2(n))
* 	IsEmpty — Returns true if the linked list is empty											O(1)
* 	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
* 	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n/k) where k is # of buckets.
* 	Truncate - Delete all the nodes in list. 													O(1)
*	Walk - Walk the table																		O(n)
* 	Print - Using Walk to print out the contents of the table.									O(n)

Possibly change to an extensible size with layers, so max dups in a layer or saturation causes
geneation of a new layer - and not a re-hash of all existing keys.

*/

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"

	"github.com/pschlump/MiscLib"
	"github.com/pschlump/dbgo"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/g_lib"
)

// HashTab is a generic hash table that grows the underlying ttable when the number of
// entries exceeds a threshold.    The table is doulbed in size.
type HashTab[T comparable.Comparable] struct {
	buckets             []*T    // the table
	originalHash        []int   // the original hash values (used during delete, search)
	size                int     // Modulo size for table	Current Size!
	length              int     // # of elements in table
	saturationThreshold float64 // Proportion before grow of table. (default 0.5)
	//lock                sync.RWMutex
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
		originalHash:        make([]int, n, n),
	}
}

// IsEmpty will return true if the hash table is empty
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	//tt.lock.RLock()
	//defer tt.lock.RUnlock()
	return tt.length == 0
}

func (tt *HashTab[T]) nlIsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (tt *HashTab[T]) Truncate() {
	//tt.lock.Lock()
	//defer tt.lock.Unlock()
	for i := 0; i < tt.size; i++ {
		tt.buckets[i] = nil
	}
	tt.length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Insert(item *T) {
	//tt.lock.Lock()
	//defer tt.lock.Unlock()
	rh := hash(item)

	// Increment a position in table modulo the size of the table.
	var incSize = func(xx int) (rv int) {
		rv = xx + 1
		if rv >= tt.size {
			rv = 0
		}
		return
	}

	if db4 {
		dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
	}
	var insertNewItem = func(rh int, itemx *T, buckets []*T, originalHash []int) {
		hh := rh % tt.size
		if db4 {
			dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF), rh=%d tt.size=%d hh=%d, len(buckets)=%d\n", rh, hh, tt.size, len(buckets))
		}
		if buckets[hh] == nil {
			if db4 {
				dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF), hh=%d, len(buckets)=%d\n", hh, len(buckets))
			}
			buckets[hh] = itemx
			originalHash[hh] = rh
			tt.length++
		} else if (*itemx).Compare(*buckets[hh]) == 0 {
			if db4 {
				dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
			}
			buckets[hh] = itemx // Replace, This means that you don't have a new key.
			originalHash[hh] = rh
		} else {
			if db4 {
				dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF) -- walk down table looking for empty slot (modulo size of table)\n")
			}
			// collision, something already at tt.buckets[hh] (original)
			for np := incSize(hh); np < tt.size; np = incSize(np) {
				// dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
				if buckets[np] == nil { // Found an empty, so put it in and leave loop
					// dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
					buckets[np] = itemx
					originalHash[np] = rh
					tt.length++
					break
				} else if (*itemx).Compare(*buckets[np]) == 0 {
					buckets[np] = itemx
					originalHash[np] = rh
					break
				}
			}
		}
	}

	insertNewItem(rh, item, tt.buckets, tt.originalHash)

	// dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
	if (((float64)(tt.length)) / ((float64)(tt.size))) > tt.saturationThreshold {
		// dbgo.Fprintf(os.Stderr, "%(yellow)Passed Threshold for size, will double.......................................................\n")
		originalSize := tt.size
		n := tt.size * 2 // Double the size
		// dbgo.Fprintf(os.Stderr, "%(yellow)    new size(n) = %d\n", n)
		oldBuckets, oldOriginal := tt.buckets, tt.originalHash
		tt.size = n
		tt.length = 0
		tt.buckets = make([]*T, n, n)
		tt.originalHash = make([]int, n, n)
		for i := 0; i < originalSize; i++ {
			if oldBuckets[i] != nil {
				item, rh := oldBuckets[i], oldOriginal[i]
				insertNewItem(rh, item, tt.buckets, tt.originalHash)
			}
		}
		// dbgo.Fprintf(os.Stderr, "%(cyan)AT:%(LF)\n")
	}
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	//tt.lock.RLock()
	//defer tt.lock.RUnlock()
	return tt.length
}
func (tt *HashTab[T]) Length() int {
	//tt.lock.RLock()
	//defer tt.lock.RUnlock()
	return tt.length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Search(find *T) (rv *T) {
	//tt.lock.RLock()
	//defer tt.lock.RUnlock()
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
	for {
		// if tt.buckets[h] == nil { 			// Delete of duplicates overlaping with duplicates fix.
		if tt.originalHash[h] == 0 {
			return // not found
			// } else if (*find).Compare(*tt.buckets[h]) == 0 { 			// Delete of duplicates overlaping with duplicates fix.
		} else if tt.buckets[h] != nil && (*find).Compare(*tt.buckets[h]) == 0 {
			rv = tt.buckets[h] // found
			return
		}
		h++
		if h >= tt.size {
			h = 0 // wrap back to top
		}
	}
	// return	-- detected as unrachable as of Go 1.23, before this missing return
}

// ht.WriteLock()
// ht.WriteUnlock()
// ht.ReadLock()
// ht.ReadUnlock()
/*
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
*/

// Dump will print out the hash table to the file `fo`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	//tt.lock.RLock()
	//defer tt.lock.RUnlock()
	fmt.Printf("Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		fmt.Fprintf(fo, "bucket [%04d] h=%d h%%size=%d = %v\n", i, tt.originalHash[i], tt.originalHash[i]%tt.size, v) // v.Dump(fo) // Xyzzy TODO - fix
	}
}

// Delete an element from the hash_tab. The element needs to have been
// located with "Search" or as a result of a match using the Walk function.
// Complexity is O(1)
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	//tt.lock.Lock()
	//defer tt.lock.Unlock()
	return tt.NlDelete(find)
}

func (tt *HashTab[T]) NlDelete(find *T) (found bool) {
	if find == nil || tt.nlIsEmpty() {
		return false
	}
	rh := hash(find)
	h := rh % tt.size

	// Increment a position in table modulo the size of the table.
	var incSize = func(xx int) (rv int) {
		rv = xx + 1
		if rv >= tt.size {
			rv = 0
		}
		return
	}

	if db1 {
		fmt.Printf("%sh=%d - for ->%+v<-%s $(LF)\n", MiscLib.ColorYellow, h, find, MiscLib.ColorReset)
	}
	for {
		// if tt.buckets[h] == nil {
		if tt.originalHash[h] == 0 {
			return false
			// } else if (*find).Compare(*tt.buckets[h]) == 0 {
		} else if tt.buckets[h] != nil && (*find).Compare(*tt.buckets[h]) == 0 {
			tt.buckets[h] = nil // found, delete the node we want to et rid of.
			tt.length--         // one less node
			found = true        // we found it
			// dbgo.Printf("%(LF)%(green) We Fond and Deleted It:  h=%d, tt.length=%d \n", h, tt.length)

			// now we need to cleanup the empty stpot at tt.buckets[h]
			// h -->> deleted slot, now nil.

			// Must move up - and re-hash stuff ! unilt NIL found. ( 2nd loop ! )
			// Find the "end" where our duplicates end.
			h2 := h // To Locaiton in buckets
			hf := h
			oh := h
			// dbgo.Printf("%(LF) h2=%d hf=%d oh=%d\n", h2, hf, oh)

			// xyzzy TODO -------------------------------- <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
			//               +--------------------- oh
			//               |                +---- he
			//               |                |
			//               v                v
			// Before:    A1 A2 b A3 b b c A4 __
			// Delete '2nd' A
			// Before:    A1 __ b A3 b c c A4 __
			// Move Up:   A1 A3 b A4 b b c __ __

			for {
				hf = incSize(hf)
				// dbgo.Printf("%(LF) h2=%d hf=%d\n", h2, hf)
				if tt.buckets[hf] == nil {
					break
				}
				// if (tt.originalHash[h] % tt.size) == (tt.originalHash[hf] % tt.size) {
				if oh == (tt.originalHash[hf] % tt.size) {
					tt.buckets[h2] = tt.buckets[hf]
					tt.originalHash[h2] = tt.originalHash[hf]
					tt.buckets[hf] = nil
					h2 = hf
				}
			}
			return
		}
		h = incSize(h)
	}
	// return	-- detected as unrachable as of Go 1.23, before this missing return
}

type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool

func (tt *HashTab[T]) Walk(fx ApplyFunction[T], userData interface{}) (b bool) {
	//tt.lock.RLock()
	//defer tt.lock.RUnlock()
	b = true
	if tt.nlIsEmpty() {
		return
	}
	for ii, vv := range tt.buckets {
		if vv != nil {
			b = b && fx(ii, 0, vv, userData)
			if !b {
				return
			}
		}
	}
	return
}

func hash(x interface{}) (rv int) {
	hashstr := func(s string) int {
		h := fnv.New32a()
		h.Write([]byte(s))
		return g_lib.Abs(int(h.Sum32()))
	}
	if v, ok := x.(Hashable); ok {
		h := v.HashKey(x)
		return g_lib.Abs(int(h))
	}
	if v, ok := x.(string); ok {
		h := hashstr(v)
		return g_lib.Abs(h)
	}
	if v, ok := x.(fmt.Stringer); ok {
		h := hashstr(v.String())
		return g_lib.Abs(int(h))
	}
	fmt.Fprintf(os.Stderr, "Invalid type, %T needs to be string, Stringer or Hashable interface\n", x)
	panic(fmt.Sprintf("Invalid type, %T needs to be string, Stringer or Hashable interface\n", x))
}

func (tt *HashTab[T]) Print(out io.Writer) {
	// type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool
	// var fx ApplyFunction[T]
	// fx = func(pos, depth int, data *T, y interface{}) bool {
	fx := func(pos, depth int, data *T, y interface{}) bool {
		fmt.Fprintf(out, "%v\n", *tt.buckets[pos])
		return true
	}
	// func (tt *HashTab[T]) Walk(fx binary_tree_ts.ApplyFunction[T], userData interface{}) (b bool) {
	tt.Walk(fx, nil)
}

const db1 = false
const db4 = false

/* vim: set noai ts=4 sw=4: */
