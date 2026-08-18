package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Package hash_tab implements a generic hash table with separate chaining.
Each bucket is a doubly linked list (github.com/pschlump/pluto/dll), so
deletion of a located element is O(1).

Basic operations on a Hash Table.

* 	Insert — Inserts a new value into the table.											O(1)
* 	Delete — Deletes a specified element from the table (element can be found via Search).	O(n/k), k = # of buckets
* 	DeleteFound — Deletes a previously located element.										O(1)
* 	IsEmpty — Returns true if the table is empty.											O(1)
* 	Length — Returns number of elements in the table.  0 length is an empty table.			O(1)
* 	Search — Returns the given element from the table.										O(n/k), k = # of buckets
* 	Truncate — Deletes all the elements in the table.										O(k), k = # of buckets
* 	Walk — Iterates over the table with a callback.											O(n)
* 	All — Range-over-func iterator over all elements (see iter.go).							O(n)

Keys are hashed with FNV-32a over the element's String() method, or with the
element's own HashKey method if it implements the Hashable interface.

*/

import (
	"fmt"
	"hash/fnv"
	"io"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dll"
	"github.com/pschlump/pluto/g_lib"
)

// HashTab is a generic hash table using separate chaining; each bucket is a
// doubly linked list of elements.
type HashTab[T comparable.Equality] struct {
	buckets []*dll.Dll[T] // the table
	length  int           // # of elements in table
	size    int           // Modulo size for table
}

// Hashable may be implemented by an element type to supply its own hash key,
// overriding the default FNV-32a hash of the element's String() value.
type Hashable interface {
	HashKey(x interface{}) int
}

// NewHashTab creates a hash table with `n` buckets.  n must be at least 5.
// Complexity is O(n).
func NewHashTab[T comparable.Equality](n int) *HashTab[T] {
	if n < 5 {
		panic("n too small")
	}
	r := HashTab[T]{
		length: 0,
		size:   n,
	}
	r.buckets = make([]*dll.Dll[T], n)
	for i := 0; i < n; i++ {
		r.buckets[i] = dll.NewDll[T]()
	}
	return &r
}

// IsEmpty will return true if the table is empty.
// Complexity is O(1).
func (tt *HashTab[T]) IsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all data from the table.
// Complexity is O(k), where k is the number of buckets.
func (tt *HashTab[T]) Truncate() {
	for i := 0; i < tt.size; i++ {
		tt.buckets[i].Truncate()
	}
	tt.length = 0
}

// Insert will add a new item to the table.  If it is a duplicate of an
// existing item the new item is inserted before the old one, hiding it
// (the bucket acts like a stack).
// Complexity is O(1).
func (tt *HashTab[T]) Insert(item *T) {
	h := g_lib.Abs(tt.hash(item) % tt.size)
	isNew := tt.buckets[h].InsertBeforeHead(item)
	if isNew {
		tt.length++
	}
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

// Search will look for `find` and return the found element
// if it is in the table. If it is not found then `nil` will be returned.
// Complexity is O(n/k), where k is the number of buckets.
func (tt *HashTab[T]) Search(find *T) (rv *dll.DllElement[T]) {
	if tt.IsEmpty() {
		return nil
	}
	h := g_lib.Abs(tt.hash(find) % tt.size)
	var pos int
	rv, pos = tt.buckets[h].Search(find)
	if pos < 0 {
		return nil
	}
	return
}

// ItemExists returns true if an element equal to `find` is in the table.
// Complexity is O(n/k), where k is the number of buckets.
func (tt *HashTab[T]) ItemExists(find *T) (rv bool) {
	if x := tt.Search(find); x != nil {
		rv = true
	}
	return
}

// Dump will print out the hash table to the writer `fp`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fp io.Writer) {
	_, _ = fmt.Fprintf(fp, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		if v.Length() > 0 {
			_, _ = fmt.Fprintf(fp, "bucket [%04d] = \n", i)
			v.Dump(fp)
		}
	}
}

// Delete removes the first element equal to `find` from the table,
// returning true if an element was found and removed.
// Complexity is O(n/k), where k is the number of buckets.
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	if tt.IsEmpty() {
		return false
	}
	h := g_lib.Abs(tt.hash(find) % tt.size)
	err := tt.buckets[h].Delete(find)
	found = err == nil
	if found {
		tt.length--
	}
	return
}

// DeleteFound removes an element that was previously located with Search or
// Walk from the table, returning true if the element was removed.
// Complexity is O(1).
func (tt *HashTab[T]) DeleteFound(find *dll.DllElement[T]) (found bool) {
	if tt.IsEmpty() {
		return false
	}
	h := g_lib.Abs(tt.hash(find.GetData()) % tt.size)
	err := tt.buckets[h].DeleteFound(find)
	found = err == nil
	if found {
		tt.length--
	}
	return
}

// Walk iterates over the table calling `fx` for each element.  If `fx`
// returns true the walk stops and the matching element and its position in
// its bucket are returned; otherwise nil, -1 is returned.
// Complexity is O(n).
func (ns *HashTab[T]) Walk(fx dll.ApplyFunction[T], userData interface{}) (rv *dll.DllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	}

	for _, v := range ns.buckets {
		if v.Length() > 0 {
			if p, i := v.Walk(fx, userData); i >= 0 {
				return p, i
			}
		}
	}

	return nil, -1 // not found
}

func (tt *HashTab[T]) hash(x interface{}) (rv int) {
	hashstr := func(s string) int {
		h := fnv.New32a()
		_, _ = h.Write([]byte(s)) // hash.Hash Write never fails
		return int(h.Sum32())
	}
	if v, ok := x.(Hashable); ok {
		h := v.HashKey(x)
		return h
	}
	if v, ok := x.(string); ok {
		h := hashstr(v)
		return h
	}
	if v, ok := x.(fmt.Stringer); ok {
		h := hashstr(v.String())
		return h
	}
	panic(fmt.Sprintf("Invalid type, %T needs to be Stringer or Hashable interface\n", x))
}
