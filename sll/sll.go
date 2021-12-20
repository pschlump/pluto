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
type SllNode[T any] struct {
	next *SllNode[T]
	data T
}
// Sll is a generic type buildt on top of a slice
type Sll[T any] struct {
	head, tail *SllNode[T]
	length int
}

// IsEmpty will return true if the stack is empty
func (ns *Sll[T]) IsEmpty() bool {
	return (*ns).head == nil
	return (*ns).length == 0
}

// Push will append a new node to the end of the list.
func (ns *Sll[T]) AppendSLL(t SllNode[T]) {
	if (*ns).head == nil {
		(*ns).head = &t
		(*ns).tail = &t
		(*ns).length = 1
	} else {
		(*ns).tail.next = &t
		(*ns).tail = &t
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
func (ns *Sll[T]) Pop() ( rv *SllNode[T], err error ) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = (*ns).head
	rv.next = nil
	(*ns).head = (*ns).head.next
	(*ns).length--
	return 
}


// Peek returns the top element of the stack or an error indicating that the stack is empty.
func (ns *Sll[T]) Peek() (rv *SllNode[T], err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	} 
	rv = (*ns).head
	return 
}

