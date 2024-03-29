package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Basic operations on a Hash Table.

* 	Insert — Insets a new value into the table                                                  O(1)
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
	"io"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dll"
	"github.com/pschlump/pluto/g_lib"
)

// HashTab is a generic binary tree
type HashTab[T comparable.Equality] struct {
	buckets [](*dll.Dll[T]) // the table
	length  int             // # of elements in table
	size    int             // Modulo size for table
}

type Hashable interface {
	HashKey(x interface{}) int
}

// Complexity is O(1).
func NewHashTab[T comparable.Equality](n int) *HashTab[T] {
	if n < 5 {
		panic("n too small")
	}
	r := HashTab[T]{
		length: 0,
		size:   n,
	}
	r.buckets = make([](*dll.Dll[T]), n, n)
	for i := 0; i < n; i++ {
		r.buckets[i] = dll.NewDll[T]()
	}
	return &r
}

// IsEmpty will return true if the binary-tree is empty
// Complexity is O(1).
func (tt HashTab[T]) IsEmpty() bool {
	return tt.length == 0
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (tt *HashTab[T]) Truncate() {
	for i := 0; i < tt.size; i++ {
		tt.buckets[i].Truncate()
	}
	(*tt).length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will (*old: replace the existing one.*) be inserted before
// the old one - hiding it (it will act like a stack).
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Insert(item *T) {
	h := g_lib.Abs(tt.hash(item) % tt.size)
	is_new := tt.buckets[h].InsertBeforeHead(item)
	if is_new {
		(*tt).length++
	}
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (tt *HashTab[T]) Len() int {
	return (*tt).length
}
func (tt *HashTab[T]) Length() int {
	return (*tt).length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Search(find *T) (rv *dll.DllElement[T]) {
	if (*tt).IsEmpty() {
		return nil
	}
	h := g_lib.Abs(tt.hash(find) % tt.size)
	var pos int
	rv, pos = tt.buckets[h].Search(find) // func (ns *Dll[T]) Search(t *T) (rv *DllElement[T], pos int) {
	if pos < 0 {
		return nil
	}
	return
}

// if ok := ht.ItemExists(&DefinedItem{Name: in}); !ok {
func (tt *HashTab[T]) ItemExists(find *T) (rv bool) {
	if x := tt.Search(find); x != nil {
		rv = true
	}
	return
}

// Dump will print out the hash table to the file `fo`.
// Complexity is O(n).
func (tt *HashTab[T]) Dump(fp io.Writer) {
	fmt.Fprintf(fp, "Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		if v.Length() > 0 {
			fmt.Fprintf(fp, "bucket [%04d] = \n", i)
			v.Dump(fp)
		}
	}
}

// Delete an element from the hash_tab. The element needs to have been
// located with "Search" or as a result of a match using the Walk function.
// Complexity is O(1)
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	if (*tt).IsEmpty() {
		return false
	}
	// h := tt.hash(find.GetData()) % tt.size
	h := g_lib.Abs(tt.hash(find) % tt.size)
	err := tt.buckets[h].Delete(find)
	found = err == nil
	if found {
		(*tt).length--
	}
	return
}

// xyzzy -
func (tt *HashTab[T]) DeleteFound(find *dll.DllElement[T]) (found bool) {
	if (*tt).IsEmpty() {
		return false
	}
	h := g_lib.Abs(tt.hash(find.GetData()) % tt.size)
	err := tt.buckets[h].DeleteFound(find)
	found = err == nil
	if found {
		(*tt).length--
	}
	return
}

// From DLL: type ApplyFunction[T comparable.Equality] func(pos int, data T, userData interface{}) bool

// Walk - Iterate from head to tail of list. 												O(n)
func (ns *HashTab[T]) Walk(fx dll.ApplyFunction[T], userData interface{}) (rv *dll.DllElement[T], pos int) {
	// ns.mu.RLock()
	// defer ns.mu.RUnlock()
	// if ns.IsEmpty() {
	//if ns.length == 0 {
	//	return nil, -1 // not found
	//}

	if (*ns).IsEmpty() {
		return nil, -1
	}

	for _, v := range ns.buckets {
		if v.Length() > 0 {
			// fmt.Fprintf(fp, "bucket [%04d] = \n", i)
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
