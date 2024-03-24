package sll_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"testing"

	"github.com/pschlump/dbgo"
)

type TestDemo struct {
	S string
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

	// func (ns *Sll[T]) InsertHeadSLL(t *T) {
	// func (ns *Sll[T]) InsertBeforeHead(t *T) {

	Sll1.InsertHeadSLL(&TestDemo{S: "02"})
	Sll1.InsertBeforeHead(&TestDemo{S: "03"})
	Sll1.InsertHeadSLL(&TestDemo{S: "01"})

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
	Sll2.InsertHeadSLL(&TestDemo{S: "02"})
	Sll2.InsertBeforeHead(&TestDemo{S: "03"})
	Sll2.InsertHeadSLL(&TestDemo{S: "01"})

	expected := []string{"01", "03", "02"}
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

var db6 = false
var db7 = false
