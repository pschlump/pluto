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

	// 	Test - DeleteAtHead 
	Dll1.InsertBeforeHead ( &TestDemo{S:"02"} )
	Dll1.AppendAtTail ( &TestDemo{S:"03"} )
	Dll1.InsertBeforeHead ( &TestDemo{S:"01"} )
	err = Dll1.DeleteAtHead()
	if err != nil {
		t.Errorf ( "Unexpectd error after pop on empty stack" )
	}
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error after pop on empty stack" )
	}
	if a.S != "02" {
		t.Errorf ( "Unexpectd data" )
	}

	// Test - ReverseList - Reverse all the nodes in list. 												O(n)
	Dll1.Truncate()  
	Dll1.InsertBeforeHead ( &TestDemo{S:"02"} )
	Dll1.AppendAtTail ( &TestDemo{S:"03"} )
	Dll1.InsertBeforeHead ( &TestDemo{S:"01"} )
	Dll1.ReverseList()
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error after pop on empty stack" )
	}
	if a.S != "03" {
		t.Errorf ( "Unexpectd data, got %s expected %s", a.S, "03" )
	}

	// Test - DeleteAtTail — Deletes the last element of the linked list. 								O(1)
	Dll1.Truncate()  
	Dll1.InsertBeforeHead ( &TestDemo{S:"02"} )
	Dll1.AppendAtTail ( &TestDemo{S:"03"} )
	Dll1.InsertBeforeHead ( &TestDemo{S:"01"} )
	Dll1.DeleteAtTail()
	Dll1.DeleteAtTail()
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error after pop on empty stack" )
	}
	if a.S != "01" {
		t.Errorf ( "Unexpectd data, got %s expected %s", a.S, "01" )
	}
	if Dll1.Length() != 0 {
		t.Errorf ( "Unexpectd length" )
	}



	// Walk - Iterate from head to tail of list. 													O(n)
	// func (ns *Dll[T]) Walk( fx ApplyFunction[T], userData interface{} ) (rv *DllNode[T], pos int) {
	Dll1.Truncate()  
	Dll1.InsertBeforeHead ( &TestDemo{S:"02"} )
	Dll1.AppendAtTail ( &TestDemo{S:"03"} )
	Dll1.InsertBeforeHead ( &TestDemo{S:"01"} )

	var found []string
	var fx ApplyFunction[TestDemo]
	fx = func ( pos int, data TestDemo, userData interface{} ) bool {
		if db1 {
			fmt.Printf ( "[%d] = %s\n", pos, data.S )
		}
		found = append(found, data.S)
		if userData.(string) == data.S {
			return true
		}
		return false
	}
	rv, pos := Dll1.Walk( fx, "02" )
	_, _ = rv, pos 

	if len(found) != 2 {
		t.Errorf ( "Unexpectd length" )
	} else {
		if found[0] != "01" {
			t.Errorf ( "Unexpectd value" )
		}
		if found[1] != "02" {
			t.Errorf ( "Unexpectd value" )
		}
	}

	// ReverseWalk - Iterate from tail to head of list. 											O(n)
	found = []string{}
	rv, pos = Dll1.ReverseWalk( fx, "02" )
	_, _ = rv, pos 

	if len(found) != 2 {
		t.Errorf ( "Unexpectd length" )
	} else {
		if found[0] != "03" {
			t.Errorf ( "Unexpectd value" )
		}
		if found[1] != "02" {
			t.Errorf ( "Unexpectd value" )
		}
	}

	/*

	+	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
	+	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)

	+	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)

	*/

}

var db1 = false

