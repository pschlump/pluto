// Package hash_grow_ts implements a generic hash table using open addressing
// with linear probing.  The table automatically doubles in size when the
// load factor exceeds a configurable saturation threshold (default 0.5).
//
// This is the thread-safe variant: every operation is guarded by a
// sync.RWMutex.  Use github.com/pschlump/pluto/hash_grow for a non-locking
// version with an identical API.
package hash_grow_ts

/*
Copyright (C) Philip Schlump, 2023.

BSD 3 Clause Licensed. See ../LICENSE
*/

/*

Basic operations on the hash table.

* 	Insert - add a new element to the table, replacing any existing equal element.	O(1) average, O(n) worst
*  	Delete — Deletes a specified element from the table (element can be found via Search).	O(1) average, O(n) worst
* 	IsEmpty — Returns true if the table is empty											O(1)
* 	Length — Returns number of elements in the table.  0 length is an empty table.			O(1)
* 	Search — Returns the given element from the table, or nil if not found.					O(1) average, O(n) worst
* 	Truncate - Delete all the elements in the table. 										O(n)
*	Walk - Walk the table																	O(n)
*	Print - Using Walk to print out the contents of the table.								O(n)

*/

import (
	"fmt"
	"hash/fnv"
	"io"
	"sync"

	"github.com/pschlump/pluto/comparable"
)

// HashTab is a generic hash table that grows the underlying table when the number of
// entries exceeds a threshold.  The table is doubled in size when it grows.
type HashTab[T comparable.Comparable] struct {
	buckets             []*T  // the table
	originalHash        []int // the original hash values (used during delete, search)
	size                int   // modulo size for table - current size
	lock                sync.RWMutex
	length              int     // number of elements in table
	saturationThreshold float64 // load factor that triggers growth of the table (default 0.5)
}

// Hashable may be implemented by stored types to supply their own hash key.
// It takes precedence over the string/fmt.Stringer based hashing.
type Hashable interface {
	HashKey(x any) int
}

// NewHashTab creates a hash table with an initial size of `n` buckets (n must
// be at least 5) and the given saturation threshold.  A saturation of 0 selects
// the default of 0.5.
//
// Complexity is O(n) for the bucket allocation.
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
		buckets:             make([]*T, n),
		originalHash:        make([]int, n),
	}
}

// IsEmpty will return true if the hash table is empty.
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length == 0
}

// nlIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (tt *HashTab[T]) nlIsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all data from the table.  All bucket and hash slots are
// cleared so the garbage collector can reclaim the stored elements.
// Complexity is O(n).
func (tt *HashTab[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	for i := 0; i < tt.size; i++ {
		tt.buckets[i] = nil
		tt.originalHash[i] = 0
	}
	tt.length = 0
}

// nextIndex returns the next position in the table, wrapping at the end.
func (tt *HashTab[T]) nextIndex(xx int) (rv int) {
	rv = xx + 1
	if rv >= tt.size {
		rv = 0
	}
	return
}

// Insert will add a new item to the table.  If it is a duplicate of an existing
// item the new item will replace the existing one.
// Complexity is O(1) average, O(n) worst case; growing the table is amortized O(1).
func (tt *HashTab[T]) Insert(item *T) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	rh := hash(item)

	insertNewItem(rh, item, tt)

	// Grow (double) the table when the load factor passes the threshold.
	if (float64(tt.length) / float64(tt.size)) > tt.saturationThreshold {
		originalSize := tt.size
		n := tt.size * 2 // double the size
		oldBuckets, oldOriginal := tt.buckets, tt.originalHash
		tt.size = n
		tt.length = 0
		tt.buckets = make([]*T, n)
		tt.originalHash = make([]int, n)
		for i := 0; i < originalSize; i++ {
			if oldBuckets[i] != nil {
				item, rh := oldBuckets[i], oldOriginal[i]
				insertNewItem(rh, item, tt)
			}
		}
	}
}

// insertNewItem places `item` with raw hash `rh` into the table of `tt`,
// replacing an equal item if one is found along the probe chain.
func insertNewItem[T comparable.Comparable](rh int, item *T, tt *HashTab[T]) {
	hh := rh % tt.size
	if tt.buckets[hh] == nil {
		tt.buckets[hh] = item
		tt.originalHash[hh] = rh
		tt.length++
	} else if (*item).Compare(*tt.buckets[hh]) == 0 {
		tt.buckets[hh] = item // Replace: an equal key is already present.
		tt.originalHash[hh] = rh
	} else {
		// Collision: walk down the table looking for an equal item or an
		// empty slot (modulo the size of the table).
		for np := tt.nextIndex(hh); np < tt.size; np = tt.nextIndex(np) {
			if tt.buckets[np] == nil { // Found an empty slot, put it in and leave the loop.
				tt.buckets[np] = item
				tt.originalHash[np] = rh
				tt.length++
				break
			} else if (*item).Compare(*tt.buckets[np]) == 0 {
				tt.buckets[np] = item
				tt.originalHash[np] = rh
				break
			}
		}
	}
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

// Search will walk the table looking for `find` and return the found item
// if it is in the table.  If it is not found then `nil` will be returned.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Search(find *T) (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlSearch(find)
}

// NlSearch is Search without locking; the caller must hold the lock.
func (tt *HashTab[T]) NlSearch(find *T) (rv *T) {
	if find == nil || tt.nlIsEmpty() {
		return nil
	}
	h := hash(find) % tt.size
	for {
		if tt.originalHash[h] == 0 {
			return // not found
		} else if tt.buckets[h] != nil && (*find).Compare(*tt.buckets[h]) == 0 {
			rv = tt.buckets[h] // found
			return
		}
		h = tt.nextIndex(h)
	}
}

// Delete an element from the table.  The element needs to have been
// located with "Search" or as a result of a match using the Walk function.
// Returns true if the element was found and removed.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDelete(find)
}

// NlDelete is Delete without locking; the caller must hold the lock.
func (tt *HashTab[T]) NlDelete(find *T) (found bool) {
	if find == nil || tt.nlIsEmpty() {
		return false
	}
	rh := hash(find)
	h := rh % tt.size

	for {
		if tt.originalHash[h] == 0 {
			return false
		} else if tt.buckets[h] != nil && (*find).Compare(*tt.buckets[h]) == 0 {
			tt.buckets[h] = nil // found, delete the element we want to get rid of.
			tt.originalHash[h] = 0
			tt.length-- // one less element
			found = true

			// Backward-shift deletion: fill the gap at h with any following
			// element whose probe sequence passes through the gap.  This keeps
			// probe chains contiguous so Search can stop at the first truly
			// empty (originalHash == 0) slot.
			//
			// Before:    A1 A2 b A3 b b c A4 __
			// Delete '2nd' A
			// Before:    A1 __ b A3 b b c A4 __
			// Move Up:   A1 A3 b A4 b b c __ __
			gap := h
			for hf := h; ; {
				hf = tt.nextIndex(hf)
				if tt.buckets[hf] == nil {
					break
				}
				home := tt.originalHash[hf] % tt.size
				if !inProbeRange(gap, hf, home) {
					tt.buckets[gap] = tt.buckets[hf]
					tt.originalHash[gap] = tt.originalHash[hf]
					tt.buckets[hf] = nil
					tt.originalHash[hf] = 0
					gap = hf
				}
			}
			return
		}
		h = tt.nextIndex(h)
	}
}

// inProbeRange reports whether `home` is in the cyclic half-open interval
// (gap, hf] of table positions.  An element at `hf` whose home position is in
// this range never probed through `gap` and therefore must not be moved into
// the gap.
func inProbeRange(gap, hf, home int) bool {
	if gap < hf {
		return home > gap && home <= hf
	}
	return home > gap || home <= hf
}

// ApplyFunction is the callback type for Walk.  It is called with the bucket
// position, a depth (always 0 for this flat table), the element and the user
// data passed to Walk.  Returning false stops the walk.
type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData any) bool

// Walk calls `fx` for each element in the table, in bucket order, until all
// elements have been visited or `fx` returns false.  It returns true if the
// walk ran to completion.
//
// The read lock is held for the duration of the walk, so `fx` must not call
// other methods of the table.
// Complexity is O(n).
func (tt *HashTab[T]) Walk(fx ApplyFunction[T], userData any) (b bool) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
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

// Dump will print out the hash table, including empty buckets, to `fo`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fo io.Writer) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	_, _ = fmt.Fprintf(fo, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		_, _ = fmt.Fprintf(fo, "bucket [%04d] h=%d h%%size=%d = %v\n", i, tt.originalHash[i], tt.originalHash[i]%tt.size, v)
	}
}

// Print writes the elements of the table to `out`, one per line, in bucket
// order.  Complexity is O(n).
func (tt *HashTab[T]) Print(out io.Writer) {
	fx := func(pos, depth int, data *T, y any) bool {
		_, _ = fmt.Fprintf(out, "%v\n", *data)
		return true
	}
	tt.Walk(fx, nil)
}

// hash returns a positive, non-zero hash key for `x`.  A zero or negative
// value can never be returned: zero is used internally to mark empty slots
// and a negative value would produce an invalid bucket index.
func hash(x any) (rv int) {
	hashString := func(s string) int {
		h := fnv.New32a()
		h.Write([]byte(s))
		return absInt(int(h.Sum32()))
	}
	if v, ok := x.(Hashable); ok {
		rv = absInt(v.HashKey(x))
	} else if v, ok := x.(string); ok {
		rv = hashString(v)
	} else if v, ok := x.(fmt.Stringer); ok {
		rv = hashString(v.String())
	} else {
		panic(fmt.Sprintf("Invalid type, %T needs to be string, Stringer or Hashable interface", x))
	}
	if rv == 0 {
		rv = 1
	}
	return
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

/* vim: set noai ts=4 sw=4: */
