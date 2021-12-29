// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright (C) 2021 Philip Schlump. All rights reserved.

package heap

// xyzzy - TODO - how to append an array of T
// xyzzy - TODO - how to append sorted array of T

import (
	"fmt"
	"strings"

	"github.com/pschlump/godebug"
	"github.com/pschlump/MiscLib"
	"github.com/pschlump/pluto/comparable"
)

//
// Complexity note.  The order uses 'n' where n = hp.Length().
//

// The heap data is stored in a slice of type *T
type Heap[T comparable.Comparable] struct {
	data []*T
}

// Create a new heap and return it.
// Complexity is O(1).
func NewHeap[T comparable.Comparable] () *Heap[T] {
	// We don't have to "heapify" at this point becasue we start all heaps with an empty set of data.
	return &Heap[T]{}
}


// Push appends the element x onto the end of the heap and re-orders the heap to be a heap.
// Complexity is O(log n).
func (hp *Heap[T]) Push( x *T ) {
	hp.data = append ( hp.data, x )	// hp.Push()
	hp.up( len(hp.data)-1 ) // Reorder to fix heap
}

// Pop removes and returns the minimum element (using comparable.Compare).
// Pop is the same as hp.Remove(0).
// Complexity is O(log n).
func (hp *Heap[T]) Pop () ( rv *T ) {
	if len(hp.data) == 0 {
		return nil
	}
	n := len(hp.data) - 1
	hp.data[0], hp.data[n] = hp.data[n], hp.data[0] // (*hp).Swap(0, n)
	hp.down(0, n) 									//
	rv = hp.data[n]									// Pop from sort
	// if n == 0 || n == 1 {
	if n == 0 {
		hp.data = []*T{}
	} else {
		// hp.data = hp.data[:n-1]						// remove element
		hp.data = hp.data[:n]						// remove element
	}
	return
}

func (hp *Heap[T]) Peek () ( rv *T ) {
	return hp.data[0]
}

func (hp *Heap[T]) Truncate() {
	hp.data = []*T{}
}

// Delete removes and returns the element at the specified index `ii` from the heap.
// Complexity is O(log n).
func (hp *Heap[T]) Delete(ii int) (rv *T) {
	if ii < 0 || ii >= len(hp.data) {
		panic ( "heap index out of range" )
	}
	n := len(hp.data) - 1 
	if n != ii {
		hp.data[ii], hp.data[n] = hp.data[n], hp.data[ii] // (*hp).Swap(ii, n)
		if !hp.down(0, n) {								
			hp.up(ii)
		}
	}
	rv = hp.data[0]									// Pop() from sort
	hp.data = hp.data[1:]							// remove element
	return
}


// Fix re-establishes the heap ordering after a change to the value of the element at locaiton `ii`.
// Changing the value of the element (indrement/decrement/update) at `ii` followed by a call to Fix()
// is the same as hp.Delete(ii) and hp.Push(NewValue).  It is less expesive to call use the Fix
// operation.
// Complexity is O(log n).
func (hp *Heap[T]) Fix(ii int, newValue *T) {
	if ii < 0 || ii >= len(hp.data) {
		panic ( "heap index out of range" )
	}
	hp.data[ii] = newValue
	if !hp.down(ii, len(hp.data)) {								
		hp.up(ii)
	}
}

// GetValue will return the value at index `ii` in the heap.
// Complexity is O(1).
func (hp *Heap[T]) GetValue(ii int) (value *T) {
	if ii < 0 || ii >= len(hp.data) {
		panic ( "heap index out of range" )
	}
	return hp.data[ii]
}

func (hp *Heap[T]) SetValue(ii int, newValue *T) {
	hp.Fix(ii,newValue)
}

// Len will return the number of items in the heap.
// Complexity is O(1).
func (hp *Heap[T]) Len() int {
	return len(hp.data)
}
func (hp *Heap[T]) Length() int {
	return len(hp.data)
}

// Complexity is O(n log n).
func (hp *Heap[T])Search(cmpVal *T) (rv *T, pos int, err error) {
	var findIt func ( root int )
	findIt = func ( i int ) {
		if i < len(hp.data) {
			c := (*(hp.data[i])).Compare(*cmpVal)
			if c == 0 {
				rv, pos = hp.data[i], i
			} else if c < 0 {
				l := 2 * i + 1 // left = 2*i + 1
				if l < len(hp.data) {
					findIt ( l )
				}
			} else {
				r := 2 * i + 2 // right = 2*i + 2
				if r < len(hp.data) {
					findIt ( r )
				}
			}
		}
	}
	findIt(0)
	return
}

func (hp *Heap[T]) up(j int) {
	if db10 {
		fmt.Printf ( "%sup: (before) at:%s\n", MiscLib.ColorCyan, godebug.LF())
		hp.printAsTree() 
	}
	for {
		i := (j - 1) / 2 // pick the parent
		c := (*(hp.data[j])).Compare(*(hp.data[i])) 
		if i == j || c > 0 {	
			break
		}
		hp.data[i], hp.data[j] = hp.data[j], hp.data[i] 
		j = i
	}
	if db10 {
		fmt.Printf ( "up: (after) at:%s\n", godebug.LF())
		hp.printAsTree() 
		fmt.Printf ( "%s\n", MiscLib.ColorReset )
	}
}

func ( hp *Heap[T]) down(i0, n int) ( rv bool ) {
	if db10 {
		fmt.Printf ( "%sdown: (before) at:%s\n", MiscLib.ColorYellow, godebug.LF())
		hp.printAsTree() 
	}
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { 
			break
		}
		j := j1 // choose the left child
		j2 := j1 + 1
		c0 := (*(hp.data[j2])).Compare(*(hp.data[j1]))
		if j2 < n && c0 < 0 { 
			j = j2   // choose the right child
		}
		if c := (*(hp.data[j])).Compare(*(hp.data[i])); c >= 0 { 
			break
		}
		hp.data[i], hp.data[j] = hp.data[j], hp.data[i] 
		i = j
	}
	rv = i > i0
	if db10 {
		fmt.Printf ( "down: (after) at:%s, will return %v\n", godebug.LF(), rv)
		hp.printAsTree() 
		fmt.Printf ( "%s\n", MiscLib.ColorReset )
	}
	return
}


// dump will print out the heap in JSON format.
func (hp *Heap[T]) printAsJSON() {
	fmt.Printf ( "Heap : %s\n", godebug.SVarI(hp.data) )
}

func (hp *Heap[T]) printAsTree() {
	fmt.Printf ( "Heap As Tree: Left, Mid, Right Order: (%s), called from:%s\n", godebug.LF() , godebug.LF(-1))
	
	var printIt func ( root, depth int )
	printIt = func ( i, depth int ) {
		n := hp.Length()
		l := 2*i + 1
		r := 2*i + 2
		if l < n {
			printIt ( l, depth+1 )
		}
		if i < n {
			fmt.Printf ( "%2d[%3d]: %s%+v\n", depth, i, strings.Repeat(" ",4*depth), *(hp.data[i]) )
		}
		if r < n {
			printIt ( r, depth+1 )
		}
	}

	printIt ( 0, 0 )
}


const db10 = false

