package sll

/*
Copyright (C) Philip Schlump, 2012-2024.

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
		return aa.S == bb.S
	} else if bb, ok := x.(*TestDemo); ok {
		return aa.S == bb.S
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

	_, err = Sll1.Pop()
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
			_, _ = dbgo.Printf("%d %v\n", i, v)
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

func TestSearchDelete(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.InsertAfterTail(&TestDemo{S: "03"})

	// Search for existing and missing values.
	el, pos := list.Search(&TestDemo{S: "02"})
	if pos != 1 || el == nil || el.GetData().S != "02" {
		t.Errorf("Search: expected pos 1 for 02, got pos %d el %v", pos, el)
	}
	if _, pos := list.Search(&TestDemo{S: "99"}); pos != -1 {
		t.Errorf("Search: expected pos -1 for missing value, got %d", pos)
	}
	if _, pos := list.Search(nil); pos != -1 {
		t.Errorf("Search: expected pos -1 for nil, got %d", pos)
	}

	// DeleteFound on the middle element.
	if err := list.DeleteFound(el); err != nil {
		t.Errorf("DeleteFound: unexpected error %v", err)
	}
	if got := list.Length(); got != 2 {
		t.Errorf("Expected length 2 after delete, got %d", got)
	}

	// Delete the head by value.
	if err := list.Delete(&TestDemo{S: "01"}); err != nil {
		t.Errorf("Delete: unexpected error %v", err)
	}
	// Delete the tail by value; tail pointer must be maintained.
	if err := list.Delete(&TestDemo{S: "03"}); err != nil {
		t.Errorf("Delete: unexpected error %v", err)
	}
	if got := list.Length(); got != 0 {
		t.Errorf("Expected length 0 after deleting all, got %d", got)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting all elements")
	}
	// Deleting from an empty list reports ErrEmptySll.
	if _, err := list.Pop(); err != ErrEmptySll {
		t.Errorf("Expected ErrEmptySll, got %v", err)
	}
	// Deleting a missing value reports ErrNotFound.
	list.InsertAfterTail(&TestDemo{S: "07"})
	if err := list.Delete(&TestDemo{S: "99"}); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

// TestDeleteFoundSingleElement is a regression test: deleting the only
// element must not panic and must leave head/tail consistent.
func TestDeleteFoundSingleElement(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "01"})
	el, pos := list.Search(&TestDemo{S: "01"})
	if pos != 0 || el == nil {
		t.Fatalf("Search: expected to find 01 at pos 0")
	}
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound: unexpected error %v", err)
	}
	if list.Length() != 0 || !list.IsEmpty() {
		t.Errorf("Expected empty list, got length %d", list.Length())
	}
	// List must still be usable at both ends.
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.InsertBeforeHead(&TestDemo{S: "00"})
	got := []string{}
	for _, v := range list.IterateOver() {
		got = append(got, v.S)
	}
	want := []string{"00", "02"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("Expected %v, got %v", want, got)
	}
}

// TestPopThenInsertAfterTail is a regression test: popping the last element
// must clear the tail so that a subsequent InsertAfterTail does not resurrect
// stale nodes.
func TestPopThenInsertAfterTail(t *testing.T) {
	var list Sll[TestDemo]
	list.Push(&TestDemo{S: "01"})
	if _, err := list.Pop(); err != nil {
		t.Fatalf("Pop: unexpected error %v", err)
	}
	list.InsertAfterTail(&TestDemo{S: "02"})
	if got := list.Length(); got != 1 {
		t.Errorf("Expected length 1, got %d", got)
	}
	v, err := list.Pop()
	if err != nil || v.S != "02" {
		t.Errorf("Expected to pop 02, got %v err %v", v, err)
	}
	if _, err := list.Pop(); err != ErrEmptySll {
		t.Errorf("Expected ErrEmptySll, got %v", err)
	}
}

func TestPeek(t *testing.T) {
	var list Sll[TestDemo]
	if _, err := list.Peek(); err != ErrEmptySll {
		t.Errorf("Expected ErrEmptySll on empty list, got %v", err)
	}
	list.Push(&TestDemo{S: "01"})
	list.Push(&TestDemo{S: "02"})
	v, err := list.Peek()
	if err != nil || v.S != "02" {
		t.Errorf("Expected to peek 02, got %v err %v", v, err)
	}
	if got := list.Length(); got != 2 {
		t.Errorf("Peek must not remove; expected length 2, got %d", got)
	}
}

func TestIteratePtr(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.InsertAfterTail(&TestDemo{S: "03"})

	j := 0
	for i, v := range list.IteratePtr() {
		if i != j {
			t.Errorf("Unexpected position, expected %v got %v", j, i)
		}
		want := fmt.Sprintf("0%d", j+1)
		if v.S != want {
			t.Errorf("Unexpected value at %d, want %s got %s", j, want, v.S)
		}
		j++
	}
	if j != 3 {
		t.Errorf("Expected 3 iterations, got %d", j)
	}

	// Early exit must stop the iteration.
	n := 0
	for range list.IterateOver() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early exit after 1 element, got %d", n)
	}

	// Iterating an empty list yields nothing.
	var empty Sll[TestDemo]
	for range empty.IterateOver() {
		t.Errorf("Expected no elements from empty list")
	}
}

func BenchmarkInsertBeforeHead(b *testing.B) {
	var list Sll[TestDemo]
	v := TestDemo{S: "x"}
	for i := 0; i < b.N; i++ {
		list.InsertBeforeHead(&v)
	}
}

func BenchmarkInsertAfterTail(b *testing.B) {
	var list Sll[TestDemo]
	v := TestDemo{S: "x"}
	for i := 0; i < b.N; i++ {
		list.InsertAfterTail(&v)
	}
}

func BenchmarkPop(b *testing.B) {
	var list Sll[TestDemo]
	v := TestDemo{S: "x"}
	for i := 0; i < b.N; i++ {
		list.Push(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := list.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	var list Sll[TestDemo]
	for i := 0; i < 1000; i++ {
		list.InsertAfterTail(&TestDemo{S: fmt.Sprintf("%04d", i)})
	}
	needle := TestDemo{S: "0999"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = list.Search(&needle)
	}
}

func BenchmarkIterateOver(b *testing.B) {
	var list Sll[TestDemo]
	v := TestDemo{S: "x"}
	for i := 0; i < 1000; i++ {
		list.InsertAfterTail(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range list.IterateOver() {
		}
	}
}
