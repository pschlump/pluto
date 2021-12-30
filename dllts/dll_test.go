package dllts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"testing"

	"github.com/pschlump/godebug"
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
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return false
}

func TestDll(t *testing.T) {

	var Dll1 Dll[TestDemo]

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	if !Dll1.IsEmpty() {
		t.Errorf("Expected empty stack after decleration, failed to get one.")
	}

	Dll1.AppendAtTail(&TestDemo{S: "hi"})

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	if Dll1.IsEmpty() {
		t.Errorf("Expected non-empty stack after 1st push, failed to get one.")
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	_, err := Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd empty stack error after 1 pop")
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	_, err = Dll1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}

	Dll1.AppendAtTail(&TestDemo{S: "hi2"})
	Dll1.AppendAtTail(&TestDemo{S: "hi3"})

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	got := Dll1.Length()
	expect := 2
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	ss, err := Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty stack")
	}
	if ss.S != "hi2" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}

	ss, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty stack")
	}
	if ss.S != "hi3" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}

	// func (ns *Dll[T]) InsertBeforeHead(t *T) {
	// func (ns *Dll[T]) AppendAtTail(t *T) {
	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})

	got = Dll1.Length()
	expect = 3
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	a, err := Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "01" {
		t.Errorf("Unexpectd data")
	}

	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "02" {
		t.Errorf("Unexpectd data")
	}

	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "03" {
		t.Errorf("Unexpectd data")
	}

	a, err = Dll1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	// 	Test - DeleteAtHead
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})
	err = Dll1.DeleteAtHead()
	if err != nil {
		t.Errorf("Unexpectd error after pop on empty stack")
	}
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error after pop on empty stack")
	}
	if a.S != "02" {
		t.Errorf("Unexpectd data")
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	// Test - ReverseList - Reverse all the nodes in list. 												O(n)
	Dll1.Truncate()
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})
	Dll1.ReverseList()
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error after pop on empty stack")
	}
	if a.S != "03" {
		t.Errorf("Unexpectd data, got %s expected %s", a.S, "03")
	}

	// Test - DeleteAtTail — Deletes the last element of the linked list. 								O(1)
	Dll1.Truncate()
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})
	Dll1.DeleteAtTail()
	Dll1.DeleteAtTail()
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error after pop on empty stack")
	}
	if a.S != "01" {
		t.Errorf("Unexpectd data, got %s expected %s", a.S, "01")
	}
	if Dll1.Length() != 0 {
		t.Errorf("Unexpectd length")
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	// Walk - Iterate from head to tail of list. 													O(n)
	// func (ns *Dll[T]) Walk( fx ApplyFunction[T], userData interface{} ) (rv *DllElement[T], pos int) {
	Dll1.Truncate()
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})

	var found []string
	var fx ApplyFunction[TestDemo]
	fx = func(pos int, data TestDemo, userData interface{}) bool {
		if db1 {
			fmt.Printf("[%d] = %s\n", pos, data.S)
		}
		found = append(found, data.S)
		if userData.(string) == data.S {
			return true
		}
		return false
	}
	rv, pos := Dll1.Walk(fx, "02")
	_, _ = rv, pos

	if len(found) != 2 {
		t.Errorf("Unexpectd length")
	} else {
		if found[0] != "01" {
			t.Errorf("Unexpectd value")
		}
		if found[1] != "02" {
			t.Errorf("Unexpectd value")
		}
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	// ReverseWalk - Iterate from tail to head of list. 											O(n)
	found = []string{}
	rv, pos = Dll1.ReverseWalk(fx, "02")
	_, _ = rv, pos

	if len(found) != 2 {
		t.Errorf("Unexpectd length")
	} else {
		if found[0] != "03" {
			t.Errorf("Unexpectd value")
		}
		if found[1] != "02" {
			t.Errorf("Unexpectd value")
		}
	}

	/*

		+	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
		+	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)

		+	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)

	*/

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	Dll1.Truncate()
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})

	// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
	// func (ns *Dll[T]) Search( t *T ) (rv *DllElement[T], pos int) {
	rv, pos = Dll1.Search(&TestDemo{S: "02"})
	if db4 {
		fmt.Printf("%+v, at locaiton %d\n", rv, pos)
	}

	// Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)
	// func (ns *Dll[T]) Delete( it *DllElement[T] ) ( err error ) {
	err = Dll1.Delete(rv)

	if Dll1.Length() != 2 {
		t.Errorf("Unexpectd length, after search/delete, expected %d got %d", 2, Dll1.Length())
	}

	// Print the nodes in a list.
	fx = func(pos int, data TestDemo, userData interface{}) bool {
		if db3 {
			fmt.Printf("[%d] = %s\n", pos, data.S)
		}
		return false
	}
	Dll1.Walk(fx, "02")

	// ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	Dll1.Truncate()
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})

	// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
	// func (ns *Dll[T]) Search( t *T ) (rv *DllElement[T], pos int) {
	rv, pos = Dll1.ReverseSearch(&TestDemo{S: "02"})
	if db4 {
		fmt.Printf("%+v, at locaiton %d\n", rv, pos)
	}

	// Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)
	// func (ns *Dll[T]) Delete( it *DllElement[T] ) ( err error ) {
	err = Dll1.Delete(rv)

	if Dll1.Length() != 2 {
		t.Errorf("Unexpectd length, after search/delete, expected %d got %d", 2, Dll1.Length())
	}

	Dll1.Walk(fx, "02")

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	// TODO
	// func (ns *Dll[T]) Index(sub int) (rv *DllElement[T], err error) {
	// Index - return the Nth item																	O(n)

	Dll1.Truncate()
	Dll1.InsertBeforeHead(&TestDemo{S: "02"})
	Dll1.AppendAtTail(&TestDemo{S: "03"})
	Dll1.InsertBeforeHead(&TestDemo{S: "01"})

	rv, err = Dll1.Index(0)
	if err != nil {
		t.Errorf("Unexpectd error")
	} else {
		if (*rv).data.S != "01" {
			t.Errorf("Unexpectd value, expected ->%s<- got ->%s<-", "01", (*rv).data.S)
		}
	}

	rv, err = Dll1.Index(1)
	if err != nil {
		t.Errorf("Unexpectd error")
	} else {
		if (*rv).data.S != "02" {
			t.Errorf("Unexpectd value, expected ->%s<- got ->%s<-", "02", (*rv).data.S)
		}
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	rv, err = Dll1.Index(2)
	if err != nil {
		t.Errorf("Unexpectd error")
	} else {
		if (*rv).data.S != "03" {
			t.Errorf("Unexpectd value, expected ->%s<- got ->%s<-", "03", (*rv).data.S)
		}
	}

	rv, err = Dll1.Index(3)
	if err == nil {
		t.Errorf("Unexpectd lack of error")
	}

}

func TestIter(t *testing.T) {
	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	var Dll2 Dll[TestDemo]
	Dll2.InsertBeforeHead(&TestDemo{S: "02"})
	Dll2.AppendAtTail(&TestDemo{S: "03"})
	Dll2.InsertBeforeHead(&TestDemo{S: "01"})

	expected := []string{"01", "02", "03"}
	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	for ii := Dll2.Front(); !ii.Done(); ii.Next() {
		if db6 {
			fmt.Printf("at:%s pos %d value %+v\n", godebug.LF(), ii.Pos(), ii.Value())
		}
		j := ii.Pos()
		if j < 0 || j >= len(expected) {
			t.Errorf("Unexpectd location in list: %d\n", j)
		} else {
			if expected[j] != ii.Value().S {
				t.Errorf("Unexpectd Value got ->%s<- expectd ->%s<- at pos %d\n", ii.Value().S, expected[j], j)
			}
		}
	}

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	for ii := Dll2.Rear(); !ii.Done(); ii.Prev() {
		if db6 {
			fmt.Printf("at:%s pos %d value %+v\n", godebug.LF(), ii.Pos(), ii.Value())
		}
		j := ii.Pos()
		if j < 0 || j >= len(expected) {
			t.Errorf("Unexpectd location in list: %d\n", j)
		} else {
			if expected[j] != ii.Value().S {
				t.Errorf("Unexpectd Value got ->%s<- expectd ->%s<- at pos %d\n", ii.Value().S, expected[j], j)
			}
		}
	}

}

var db1 = false
var db3 = false
var db4 = false
var db6 = false
var db7 = false
