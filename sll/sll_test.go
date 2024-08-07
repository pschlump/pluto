package sll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"os"
	"testing"

	"github.com/pschlump/dbgo"
	"github.com/pschlump/pluto/comparable"
)

type TestDemo struct {
	S string
}

var _ comparable.Equality = (*TestDemo)(nil)

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
	// return false
}

func TestStack(t *testing.T) {

	var Sll1 Sll[TestDemo]

	if !Sll1.IsEmpty() {
		t.Errorf("Expected empty stack after decleration, failed to get one.")
	}

	Sll1.InsertBeforeHead(&TestDemo{S: "hi"})

	if Sll1.IsEmpty() {
		t.Errorf("Expected non-empty stack after 1st push, failed to get one.")
	}

	_, err := Sll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd empty stack error after 1 pop")
	}
	_, err = Sll1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}

	Sll1.InsertBeforeHead(&TestDemo{S: "hi2"})
	Sll1.InsertBeforeHead(&TestDemo{S: "hi3"})

	got := Sll1.Length()
	expect := 2
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	ss, err := Sll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty stack")
	}
	if ss.S != "hi3" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}

	ss, err = Sll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty stack")
	}
	if ss.S != "hi2" {
		t.Errorf("Expected %s got %s", "hi2", ss.S)
	}

	Sll1.Truncate()
	got = Sll1.Length()
	expect = 0
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	Sll1.InsertBeforeHead(&TestDemo{S: "02"})
	Sll1.InsertBeforeHead(&TestDemo{S: "03"})
	Sll1.InsertBeforeHead(&TestDemo{S: "01"})

	got = Sll1.Length()
	expect = 3
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	a, err := Sll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "01" {
		t.Errorf("Unexpectd data")
	}

	a, err = Sll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "03" {
		t.Errorf("Unexpectd data, got %v", a)
	}

	a, err = Sll1.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "02" {
		t.Errorf("Unexpectd data, got %v", a)
	}

	a, err = Sll1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
}

func TestIter(t *testing.T) {
	if db7 {
		fmt.Printf("AT: %s\n", dbgo.LF())
	}

	var Sll2 Sll[TestDemo]
	Sll2.InsertBeforeHead(&TestDemo{S: "03"})
	Sll2.InsertBeforeHead(&TestDemo{S: "02"})
	Sll2.InsertBeforeHead(&TestDemo{S: "01"})

	expected := []string{"01", "02", "03"}
	if db7 {
		fmt.Printf("AT: %s\n", dbgo.LF())
	}

	for ii := Sll2.Front(); !ii.Done(); ii.Next() {
		if db6 {
			fmt.Printf("at:%s pos %d value %+v\n", dbgo.LF(), ii.Pos(), ii.Value())
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

func TestReverse(t *testing.T) {
	// Build a list with 3 items, 03, 02, 01
	var Sll3 Sll[TestDemo]
	Sll3.InsertAfterTail(&TestDemo{S: "03"})
	Sll3.InsertAfterTail(&TestDemo{S: "02"})
	Sll3.InsertAfterTail(&TestDemo{S: "01"})

	if db8 {
		Sll3.Dump(os.Stdout)
	}

	Sll3.Reverse()

	if db8 {
		Sll3.Dump(os.Stdout)
	}

	got := Sll3.Length()
	expect := 3
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	a, err := Sll3.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "01" {
		t.Errorf("Unexpectd data")
	}

	a, err = Sll3.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "02" {
		t.Errorf("Unexpectd data, got %v", a)
	}

	a, err = Sll3.Pop()
	if err != nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}
	if a.S != "03" {
		t.Errorf("Unexpectd data, got %v", a)
	}

}

// func (ns *Sll[T]) IterateOver(items []T) iter.Seq2[int, T] {
func TestIterateOver(t *testing.T) {

	var Sll3 Sll[TestDemo]
	Sll3.InsertAfterTail(&TestDemo{S: "01"})
	Sll3.InsertAfterTail(&TestDemo{S: "02"})
	Sll3.InsertAfterTail(&TestDemo{S: "03"})

	if db9 {
		Sll3.Dump(os.Stdout)
	}

	j := 0
	for i, v := range Sll3.IterateOver() {
		if db9 {
			dbgo.Printf("%d %v\n", i, v)
		}
		if i != j {
			t.Errorf("Unexpectd position, expected %v got %v", j, i)
		}
		want := fmt.Sprintf("{%02d}", j+1)
		got := fmt.Sprintf("%v", v)
		if want != got {
			t.Errorf("Unexpectd value at position, postion %d want %s got %s", j, want, got)
		}
		j++
	}

}

var db6 = false
var db7 = false
var db8 = false
var db9 = false
