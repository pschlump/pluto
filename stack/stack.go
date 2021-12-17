package stack

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
)

type Stack[T any] []T

func (ns Stack[T]) IsEmpty() bool {
	return len(ns) == 0
}

func (ns *Stack[T]) Push(t T) {
	*ns = append(*ns, t)
}

var ErrEmptyStack = errors.New("Empty Stack")

func (ns *Stack[T]) Pop() error {
	if ns.IsEmpty() {
		return ErrEmptyStack
	}
	(*ns) = (*ns)[0:len((*ns))-1]
	return nil
}

func (ns Stack[T]) Length() int {
	return len(ns)
}

func (ns *Stack[T]) Peek() (*T, error) {
	if !ns.IsEmpty() {
		return &((*ns)[len(*ns)-1]), nil
	} 
	return nil, ErrEmptyStack
}
