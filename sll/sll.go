package sll

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.

Basic operations on a Singly Linked List (SLL)

*	IsEmpty() — Returns true if the sll is empty
	AppendSLL(t T) -
* 	Length() int -

*/

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/pschlump/pluto/comparable"
)

// A node in the singly linked list
type SllElement[T comparable.Equality] struct {
	next *SllElement[T]
	data *T
}

// Sll is a generic type buildt on top of a slice
type Sll[T comparable.Equality] struct {
	head, tail *SllElement[T]
	length     int
}

// An iteration type that allows a for loop to walk the list.
type SllIter[T comparable.Equality] struct {
	cur *SllElement[T]
	sll *Sll[T]
	pos int
}

// -------------------------------------------------------------------------------------------------------

// Create a new SLL and return it.
// Complexity is O(1).
func NewSll[T comparable.Equality]() *Sll[T] {
	return &Sll[T]{
		head:   nil,
		tail:   nil,
		length: 0,
	}
}

// Complexity is O(1).
func (ee *SllElement[T]) GetData() *T {
	return ee.data
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the beginning of a list for iteration over list.
func (ns *Sll[T]) Front() *SllIter[T] {
	return &SllIter[T]{
		cur: ns.head,
		sll: ns,
	}
}

// Current will take the node returned from Search or RevrseSearch
//
//	func (ns *Sll[T]) Search( t *T ) (rv *SllElement[T], pos int) {
//
// and allow you to start an iteration process from that point.
func (ns *Sll[T]) Current(el *SllElement[T], pos int) *SllIter[T] {
	return &SllIter[T]{
		cur: el,
		sll: ns,
		pos: pos,
	}
}

// Value returns the current data for this element in the list.
func (iter *SllIter[T]) Value() *T {
	if iter.cur != nil {
		return iter.cur.data
	}
	return nil
}

// Next advances to the next element in the list.
func (iter *SllIter[T]) Next() {
	if iter.cur == nil {
		return
	}
	iter.cur = iter.cur.next
	iter.pos++
}

// Done returns true if the end of the list has been reached.
func (iter *SllIter[T]) Done() bool {
	return iter.cur == nil
}

// Pos returns the current "index" of the elemnt being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *SllIter[T]) Pos() int {
	return iter.pos
}

// -------------------------------------------------------------------------------------------------------
// IsEmpty will return true if the stack is empty
func (ns *Sll[T]) IsEmpty() bool {
	// return (*ns).head == nil
	return (*ns).length == 0
}

// InsertBeforeHead will append a new node to the end of the list.
func (ns *Sll[T]) InsertBeforeHead(t *T) {
	x := SllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.head = &x
		ns.tail = &x
		ns.length = 1
	} else {
		x.next = ns.head
		ns.head = &x
		ns.length++
	}
}

// InsertAfterTail will append a new node to the end of the list.
func (ns *Sll[T]) InsertAfterTail(t *T) {
	x := SllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.head = &x
		ns.tail = &x
		ns.length = 1
	} else {
		ns.tail.next = &x
		ns.tail = &x
		ns.length++
	}
}

// Push will append a new node to the end of the list.
func (ns *Sll[T]) Push(t *T) {
	ns.InsertBeforeHead(t)
}

// Length returns the number of elements in the list.
func (ns *Sll[T]) Length() int {
	return (*ns).length
}

// An error to indicate that the stack is empty
var ErrEmptySll = errors.New("Empty Sll")
var ErrNotFound = errors.New("Not Found")

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Sll[T]) Pop() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = (*ns).head.data
	(*ns).head = (*ns).head.next
	(*ns).length--
	return
}

func (ns *Sll[T]) Delete(t *T) (err error) {
	it, pos := ns.Search(t)
	if pos < 0 {
		return fmt.Errorf("Not Found in SLL")
	}
	return ns.DeleteFound(it)
}

func (ns *Sll[T]) DeleteFound(t *SllElement[T]) (err error) {
	if ns.IsEmpty() {
		return ErrEmptySll
	}
	if (*((*ns.head).data)).IsEqual(*t.data) { // IsEqual(b Equality) bool
		*ns.head = *ns.head.next
		return
	}
	for pp := &((*ns.head).next); *pp != nil; pp = &((*pp).next) {
		if (*((*pp).data)).IsEqual(*t.data) { // IsEqual(b Equality) bool
			*pp = (*pp).next
			return
		}
	}
	return ErrNotFound
}

// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
func (ns *Sll[T]) Search(t *T) (rv *SllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	}

	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		if (*p.data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// Peek returns the top element of the stack or an error indicating that the stack is empty.		O(1)
func (ns *Sll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = (*ns).head.data
	return
}

// Truncate removes all data from the list. 		O(1)
func (ns *Sll[T]) Truncate() {
	(*ns).head = nil
	(*ns).tail = nil
	(*ns).length = 0
	return
}

// Dump prints out the list. 						O(n)
func (tt *Sll[T]) Dump(fp io.Writer) {
	i := 0
	for p := tt.head; p != nil; p = p.next {
		fmt.Fprintf(fp, "%d: %+v\n", i, *(p.data))
		i++
	}
}

// Reverse - effeciently reverse direciotn on a list.  O(n) with storage O(1)
func (ns *Sll[T]) Reverse() {

	var prev, next *SllElement[T]
	prev = nil
	for cp := ns.head; cp != nil; cp = next {
		next = cp.next // save next pointer at beginning
		cp.next = prev
		prev = cp
	}

	ns.head, ns.tail = ns.tail, ns.head

}

func (ns *Sll[T]) IterateOver() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		// for i, v := range items { // 					the loop control.....
		for i, p := 0, (*ns).head; p != nil; i, p = i+1, p.next {
			if !yield(i, *p.data) {
				return
			}
		}
	}
}

func (ns *Sll[T]) IteratePtr() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		// for i, v := range items { // 					the loop control.....
		for i, p := 0, (*ns).head; p != nil; i, p = i+1, p.next {
			if !yield(i, p.data) {
				return
			}
		}
	}
}
