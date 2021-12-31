package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Basic operations on a Binary Tree.

* 	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(log|2(n))
* 	Index - return the Nth item	in the list - in a format usable with Delete.					O(n)
* 	IsEmpty — Returns true if the linked list is empty											O(1)
* 	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
* 	Reverse - Reverse all the nodes in list. 													O(n)
* 	Search — Returns the given element from a linked list.  Search is from head to tail.		O(log|2(n))
* 	Truncate - Delete all the nodes in list. 													O(1)
*	FindMin
*	FindMax
*	Depth -> int to get deepest part of tree

* 	DeleteAtHead — Deletes the first element of the linked list.  								O(log|2(n))
		Delete ( FindMin ( ) )
* 	DeleteAtTail — Deletes the last element of the linked list. 								O(log|2(n))
		Delete ( FindMax ( ) )

*	WalkInOrder
+	WalkPreOrder
+	WalkPostOrder

*/

import (
	"fmt"
	"hash/fnv"
	"os"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/sll"
)

// xyzzy var _ comparable.Equality = (*TestDemo)(nil)

// HashTab is a generic binary tree
type HashTab[T comparable.Equality] struct {
	buckets [](*sll.Sll[T]) // the table
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
	r.buckets = make([](*sll.Sll[T]), n, n)
	for i := 0; i < n; i++ {
		r.buckets[i] = sll.NewSll[T]()
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
// item the new item will replace the existing one.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Insert(item *T) {
	h := tt.hash(item) % tt.size
	tt.buckets[h].InsertBeforeHead(item)
	(*tt).length++
}

// Length returns the number of elements in the list.
func (tt *HashTab[T]) Len() int {
	return (*tt).length
}
func (tt *HashTab[T]) Length() int {
	return (*tt).length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
// Complexity is O(log n)/k.
func (tt *HashTab[T]) Search(find *T) (item *T) {
	if (*tt).IsEmpty() {
		return nil
	}
	h := tt.hash(item) % tt.size
	// func (ns *Dll[T]) Search( t *T ) (rv *DllElement[T], pos int) {
	x, pos := tt.buckets[h].Search(find)
	if pos < 0 {
		return nil
	}
	return x.GetData()
}

// Dump will print out the hash table to the file `fo`.
func (tt *HashTab[T]) Dump(fo *os.File) {
	fmt.Printf("Elements: %d, mod size:%d\n", tt.length, tt.size)
	for i, v := range tt.buckets {
		if v.Length() > 0 {
			fmt.Printf("bucket [%04d] = \n", i)
			v.Dump(fo)
		}
	}
}

// Complexity is O(log n)/k.
func (tt *HashTab[T]) Delete(find *T) (found bool) {
	if (*tt).IsEmpty() {
		return false
	}
	h := tt.hash(find) % tt.size
	it, pos := tt.buckets[h].Search(find)
	if pos >= 0 {
		(*tt).length++
		err := tt.buckets[h].Delete(it)
		found = err != nil
	}
	return
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
