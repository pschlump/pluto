package dll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"testing"
	"fmt"

	// "github.com/pschlump/godebug"
	"github.com/pschlump/pluto/comparable"
)

type TestDemo struct {
	S string
}

func NewTestDemo() *TestDemo {
	return &TestDemo{}
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Equality = (*TestDemo)(nil)

// 
func (aa TestDemo) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(TestDemo); ok {
		if aa.S == bb.S {
			return true
		}
		return false
	} else if bb, ok := x.(*TestDemo); ok {
		if aa.S == bb.S {
			return true
		}
		return false
	} else {
		panic ( fmt.Sprintf("Passed invalid type %T to a Compare function.",x) )
	}
	return false
}

func TestDll(t *testing.T) {

	var Dll1 Dll[TestDemo]

	if !Dll1.IsEmpty() {
		t.Errorf ( "Expected empty stack after decleration, failed to get one." )
	}

	Dll1.AppendAtTail ( &TestDemo{S:"hi"} )

	if Dll1.IsEmpty() {
		t.Errorf ( "Expected non-empty stack after 1st push, failed to get one." )
	}

	_, err := Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd empty stack error after 1 pop" )
	}
	_, err = Dll1.Pop()
	if err == nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}

	Dll1.AppendAtTail ( &TestDemo{S:"hi2"} )
	Dll1.AppendAtTail ( &TestDemo{S:"hi3"} )

	got := Dll1.Length() 
	expect := 2
	if got != expect {
		t.Errorf ( "Expected length of %d got %d", expect, got )
	}

	ss, err := Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error on non-empty stack" )
	}
	if ss.S != "hi2" {
		t.Errorf ( "Expected %s got %s", "hi3", ss.S )
	}

	ss, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error on non-empty stack" )
	}
	if ss.S != "hi3" {
		t.Errorf ( "Expected %s got %s", "hi3", ss.S )
	}

	// func (ns *Dll[T]) InsertBeforeHead(t *T) {
	// func (ns *Dll[T]) AppendAtTail(t *T) {

	Dll1.InsertBeforeHead ( &TestDemo{S:"02"} )
	Dll1.AppendAtTail ( &TestDemo{S:"03"} )
	Dll1.InsertBeforeHead ( &TestDemo{S:"01"} )

	got = Dll1.Length() 
	expect = 3
	if got != expect {
		t.Errorf ( "Expected length of %d got %d", expect, got )
	}

	a, err := Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}
	if a.S != "01" {
		t.Errorf ( "Unexpectd data" )
	}

	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}
	if a.S != "02" {
		t.Errorf ( "Unexpectd data" )
	}

	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}
	if a.S != "03" {
		t.Errorf ( "Unexpectd data" )
	}

	a, err = Dll1.Pop()
	if err == nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}
}


