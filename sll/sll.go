package sll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

Basic operations on a Singly Linked List (SLL)

	IsEmpty() — Returns true if the sll is empty
	AppendSLL(t T) -
 	Length() int - 

*/

import (
	"errors"
)

// A node in the singly linked list
type SllElement[T any] struct {
	next *SllElement[T]
	data *T
}
// Sll is a generic type buildt on top of a slice
type Sll[T any] struct {
	head, tail *SllElement[T]
	length int
}


// An iteration type that allows a for loop to walk the list.
type SllIter[T any] struct {
	cur 		*SllElement[T]
	sll 		*Sll[T]
	pos 		int
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the beginning of a list for iteration over list.
func (ns *Sll[T]) Front() *SllIter[T] {
	return &SllIter[T] {
		cur: ns.head,
		sll: ns,
	}
}

// Current will take the node returned from Search or RevrseSearch
// 		func (ns *Sll[T]) Search( t *T ) (rv *SllElement[T], pos int) {
// and allow you to start an iteration process from that point.
func (ns *Sll[T]) Current(el *SllElement[T], pos int) *SllIter[T] {
	return &SllIter[T] {
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

// Push will append a new node to the end of the list.
func (ns *Sll[T]) InsertHeadSLL(t *T) {
	x := SllElement[T] { data: t }	// Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
	} else {
		x.next = (*ns).head
		(*ns).head = &x
		(*ns).length++
	}
}

// Push will append a new node to the end of the list.
func (ns *Sll[T]) AppendTailSLL(t *T) {
	x := SllElement[T] { data: t }	// Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
	} else {
		(*ns).tail.next = &x
		(*ns).tail = &x
		(*ns).length++
	}
}

// Length returns the number of elements in the list.
func (ns *Sll[T]) Length() int {
	return (*ns).length
}

// An error to indicate that the stack is empty
var ErrEmptySll = errors.New("Empty Sll")

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Sll[T]) Pop() ( rv *T, err error ) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = (*ns).head.data
	(*ns).head = (*ns).head.next
	(*ns).length--
	return 
}


// Peek returns the top element of the stack or an error indicating that the stack is empty.
func (ns *Sll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	} 
	rv = (*ns).head.data
	return 
}



// Truncate removes all data from the list.
func (ns *Sll[T]) Truncate()  {
	(*ns).head = nil
   	(*ns).tail = nil
	(*ns).length = 0
	return 
}

