package dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestDllItem is the test element type.  Equality is supplied
// to the list as a plain function (eqTestDllItem below).
type TestDllItem struct {
	S string
}

// eqTestDllItem reports equality of TestDllItem by its S field.
func eqTestDllItem(a, b TestDllItem) bool {
	return a.S == b.S
}

// newTestDll builds a Dll of TestDllItem with equality by S.
func newTestDll() *Dll[TestDllItem] {
	return NewDllFunc(eqTestDllItem)
}

func TestDll(t *testing.T) {

	Dll1 := newTestDll()

	if !Dll1.IsEmpty() {
		t.Errorf("Expected empty list after declaration, failed to get one.")
	}

	if !Dll1.AppendAtTail(TestDllItem{S: "hi"}) {
		t.Errorf("Expected AppendAtTail to return true.")
	}

	if Dll1.IsEmpty() {
		t.Errorf("Expected non-empty list after 1st append, failed to get one.")
	}

	if _, err := Dll1.Pop(); err != nil {
		t.Errorf("Unexpected empty-list error after 1 pop: %v", err)
	}

	if _, err := Dll1.Pop(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll after pop on empty list, got %v", err)
	}

	Dll1.AppendAtTail(TestDllItem{S: "hi2"})
	Dll1.AppendAtTail(TestDllItem{S: "hi3"})

	if got := Dll1.Length(); got != 2 {
		t.Errorf("Expected length of 2 got %d", got)
	}

	ss, err := Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpected error on non-empty list")
	}
	if ss.S != "hi2" {
		t.Errorf("Expected hi2 got %s", ss.S)
	}

	ss, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpected error on non-empty list")
	}
	if ss.S != "hi3" {
		t.Errorf("Expected hi3 got %s", ss.S)
	}

	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})

	if got := Dll1.Length(); got != 3 {
		t.Errorf("Expected length of 3 got %d", got)
	}

	for _, want := range []string{"01", "02", "03"} {
		a, err := Dll1.Pop()
		if err != nil {
			t.Errorf("Unexpected error on non-empty list")
		}
		if a.S != want {
			t.Errorf("Expected %s got %s", want, a.S)
		}
	}

	if _, err := Dll1.Pop(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll on drained list, got %v", err)
	}

	// Test DeleteAtHead.
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})
	if err := Dll1.DeleteAtHead(); err != nil {
		t.Errorf("Unexpected error from DeleteAtHead: %v", err)
	}
	a, err := Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpected error on non-empty list")
	}
	if a.S != "02" {
		t.Errorf("Unexpected data, got %s expected 02", a.S)
	}

	// Test ReverseList.
	Dll1.Truncate()
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})
	Dll1.ReverseList()
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpected error on non-empty list")
	}
	if a.S != "03" {
		t.Errorf("Unexpected data, got %s expected 03", a.S)
	}

	// Test DeleteAtTail — Deletes the last element of the linked list.
	Dll1.Truncate()
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})
	_ = Dll1.DeleteAtTail()
	_ = Dll1.DeleteAtTail()
	a, err = Dll1.Pop()
	if err != nil {
		t.Errorf("Unexpected error on non-empty list")
	}
	if a.S != "01" {
		t.Errorf("Unexpected data, got %s expected 01", a.S)
	}
	if Dll1.Length() != 0 {
		t.Errorf("Unexpected length")
	}
}

// TestDllWalkStop verifies Walk and ReverseWalk.  Note the dll convention:
// returning true from the callback STOPS the walk (the opposite of the
// pluto tree packages).
func TestDllWalkStop(t *testing.T) {

	Dll1 := newTestDll()
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})

	stopAt := func(target string) ApplyFunction[TestDllItem] {
		return func(pos int, data TestDllItem) bool {
			return data.S == target // true stops the walk
		}
	}

	var hit []string
	el, pos := Dll1.Walk(func(pos int, data TestDllItem) bool {
		hit = append(hit, data.S)
		return data.S == "02"
	})
	_ = stopAt
	if len(hit) != 2 || hit[0] != "01" || hit[1] != "02" {
		t.Errorf("Walk visited %v, expected [01 02]", hit)
	}
	if el == nil || pos != 1 {
		t.Errorf("Walk returned (%v, %d), expected element at pos 1", el, pos)
	}

	hit = nil
	el, pos = Dll1.ReverseWalk(func(pos int, data TestDllItem) bool {
		hit = append(hit, data.S)
		return data.S == "02"
	})
	if len(hit) != 2 || hit[0] != "03" || hit[1] != "02" {
		t.Errorf("ReverseWalk visited %v, expected [03 02]", hit)
	}
	if el == nil || pos != 1 {
		t.Errorf("ReverseWalk returned (%v, %d), expected element at pos 1", el, pos)
	}

	// A walk whose callback never returns true reports nil, -1.
	el, pos = Dll1.Walk(func(pos int, data TestDllItem) bool { return false })
	if el != nil || pos != -1 {
		t.Errorf("Full walk returned (%v, %d), expected (nil, -1)", el, pos)
	}
	el, pos = Dll1.ReverseWalk(func(pos int, data TestDllItem) bool { return false })
	if el != nil || pos != -1 {
		t.Errorf("Full ReverseWalk returned (%v, %d), expected (nil, -1)", el, pos)
	}
}

func TestDllSearchAndDelete(t *testing.T) {

	Dll1 := newTestDll()
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})

	// Search — from head to tail.
	rv, pos := Dll1.Search(TestDllItem{S: "02"})
	if rv == nil || pos != 1 {
		t.Fatalf("Search returned (%v, %d), expected element at pos 1", rv, pos)
	}
	if err := Dll1.DeleteFound(rv); err != nil {
		t.Errorf("Unexpected error from DeleteFound: %v", err)
	}
	if Dll1.Length() != 2 {
		t.Errorf("Unexpected length after search/delete: %d", Dll1.Length())
	}

	// ReverseSearch — from tail to head.
	Dll1.Truncate()
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})
	rv, pos = Dll1.ReverseSearch(TestDllItem{S: "02"})
	if rv == nil || pos != 1 {
		t.Fatalf("ReverseSearch returned (%v, %d), expected element at pos 1", rv, pos)
	}
	if err := Dll1.DeleteFound(rv); err != nil {
		t.Errorf("Unexpected error from DeleteFound: %v", err)
	}
	if Dll1.Length() != 2 {
		t.Errorf("Unexpected length after reverse-search/delete: %d", Dll1.Length())
	}

	// Index — return the Nth item.
	Dll1.Truncate()
	Dll1.InsertBeforeHead(TestDllItem{S: "02"})
	Dll1.AppendAtTail(TestDllItem{S: "03"})
	Dll1.InsertBeforeHead(TestDllItem{S: "01"})

	for sub, want := range []string{"01", "02", "03"} {
		rv, err := Dll1.Index(sub)
		if err != nil {
			t.Errorf("Unexpected error from Index(%d): %v", sub, err)
		} else if rv.GetData().S != want {
			t.Errorf("Index(%d) = %s, expected %s", sub, rv.GetData().S, want)
		}
	}

	if _, err := Dll1.Index(3); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from Index(3), got %v", err)
	}
	if _, err := Dll1.Index(-1); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from Index(-1), got %v", err)
	}

	// Delete by value.
	if err := Dll1.Delete(TestDllItem{S: "02"}); err != nil {
		t.Errorf("Failed to delete 02: %v", err)
	}
	rv, err := Dll1.Index(0)
	if err != nil {
		t.Errorf("Unexpected error from Index(0): %v", err)
	} else if rv.GetData().S != "01" {
		t.Errorf("Index(0) = %s, expected 01", rv.GetData().S)
	}
	rv, err = Dll1.Index(1)
	if err != nil {
		t.Errorf("Unexpected error from Index(1): %v", err)
	} else if rv.GetData().S != "03" {
		t.Errorf("Index(1) = %s, expected 03", rv.GetData().S)
	}

	if err := Dll1.Delete(TestDllItem{S: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from Delete of absent item, got %v", err)
	}

	// DeleteSearch — the documented alias for Delete.
	if err := Dll1.DeleteSearch(TestDllItem{S: "03"}); err != nil {
		t.Errorf("Failed to DeleteSearch 03: %v", err)
	}
	if Dll1.Length() != 1 {
		t.Errorf("Unexpected length after DeleteSearch: %d", Dll1.Length())
	}
	if err := Dll1.DeleteSearch(TestDllItem{S: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from DeleteSearch of absent item, got %v", err)
	}
}

func TestIter(t *testing.T) {

	Dll2 := newTestDll()
	Dll2.InsertBeforeHead(TestDllItem{S: "02"})
	Dll2.AppendAtTail(TestDllItem{S: "03"})
	Dll2.InsertBeforeHead(TestDllItem{S: "01"})

	expected := []string{"01", "02", "03"}

	for ii := Dll2.Front(); !ii.Done(); ii.Next() {
		j := ii.Pos()
		if j < 0 || j >= len(expected) {
			t.Errorf("Unexpected location in list: %d", j)
			continue
		}
		v, found := ii.Value()
		if !found {
			t.Fatalf("Value not found while not Done.")
		}
		if expected[j] != v.S {
			t.Errorf("Unexpected value got %s expected %s at pos %d", v.S, expected[j], j)
		}
	}

	for ii := Dll2.Rear(); !ii.Done(); ii.Prev() {
		j := ii.Pos()
		if j < 0 || j >= len(expected) {
			t.Errorf("Unexpected location in list: %d", j)
			continue
		}
		v, found := ii.Value()
		if !found {
			t.Fatalf("Value not found while not Done.")
		}
		if expected[j] != v.S {
			t.Errorf("Unexpected value got %s expected %s at pos %d", v.S, expected[j], j)
		}
	}

	// Current starts an iteration from a found position.
	rv, pos := Dll2.Search(TestDllItem{S: "02"})
	if rv == nil {
		t.Fatalf("Expected to find 02.")
	}
	var got []string
	for it := Dll2.Current(rv, pos); !it.Done(); it.Next() {
		v, _ := it.Value()
		got = append(got, v.S)
	}
	if expect := []string{"02", "03"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Current iteration got %v, expected %v", got, expect)
	}
}

// TestPeekPopTail verifies the queue-style tail operations.
func TestPeekPopTail(t *testing.T) {
	list := newTestDll()

	if _, err := list.Peek(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Peek on empty list, got %v", err)
	}
	if _, err := list.PeekTail(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from PeekTail on empty list, got %v", err)
	}
	if _, err := list.PopTail(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from PopTail on empty list, got %v", err)
	}

	list.Enqueue(TestDllItem{S: "01"})
	list.Enqueue(TestDllItem{S: "02"})
	list.Enqueue(TestDllItem{S: "03"})

	if v, err := list.Peek(); err != nil || v.S != "01" {
		t.Errorf("Peek = (%v, %v), expected 01", v, err)
	}
	if v, err := list.PeekTail(); err != nil || v.S != "03" {
		t.Errorf("PeekTail = (%v, %v), expected 03", v, err)
	}

	if v, err := list.PopTail(); err != nil || v.S != "03" {
		t.Errorf("PopTail = (%v, %v), expected 03", v, err)
	}
	if list.Length() != 2 {
		t.Errorf("Expected length 2 after PopTail, got %d", list.Length())
	}
}

// TestRangeOverFunc verifies the All/Backward/IterateOver iterators.
func TestRangeOverFunc(t *testing.T) {
	list := newTestDll()
	for _, s := range []string{"01", "02", "03"} {
		list.AppendAtTail(TestDllItem{S: s})
	}

	var got []string
	for i, v := range list.All() {
		if i < 0 || i > 2 {
			t.Fatalf("All: unexpected index %d", i)
		}
		got = append(got, v.S)
	}
	if expect := []string{"01", "02", "03"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All got %v, expected %v", got, expect)
	}

	got = nil
	for i, v := range list.Backward() {
		if i != 3-1-len(got) {
			t.Fatalf("Backward: unexpected index %d at step %d", i, len(got))
		}
		got = append(got, v.S)
	}
	if expect := []string{"03", "02", "01"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Backward got %v, expected %v", got, expect)
	}

	// IterateOver is a legacy alias for All.
	got = nil
	for _, v := range list.IterateOver() {
		got = append(got, v.S)
	}
	if expect := []string{"01", "02", "03"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("IterateOver got %v, expected %v", got, expect)
	}

	// Early break stops iteration.
	n := 0
	for range list.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Early break stops backward iteration too.
	n = 0
	for range list.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected Backward early break to yield exactly 1 item, got %d", n)
	}

	// Empty list yields nothing.
	empty := newTestDll()
	for range empty.All() {
		t.Errorf("Expected no items from All on empty list")
	}
	for range empty.Backward() {
		t.Errorf("Expected no items from Backward on empty list")
	}
}

// TestTrimReuse verifies Trim/TrimTail behavior and that the list is
// reusable afterwards.
func TestTrimReuse(t *testing.T) {
	list := newTestDll()
	if err := list.Trim(3); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Trim on empty list, got %v", err)
	}
	if err := list.TrimTail(3); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from TrimTail on empty list, got %v", err)
	}

	for i := range 5 {
		list.AppendAtTail(TestDllItem{S: fmt.Sprintf("%02d", i)})
	}

	// No-op when n >= length.
	if err := list.Trim(5); err != nil || list.Length() != 5 {
		t.Errorf("Trim(5) = %v, length %d; expected no-op", err, list.Length())
	}
	if err := list.Trim(9); err != nil || list.Length() != 5 {
		t.Errorf("Trim(9) = %v, length %d; expected no-op", err, list.Length())
	}

	// Trim keeps the head.
	if err := list.Trim(3); err != nil {
		t.Fatalf("Trim(3): %v", err)
	}
	var got []string
	for _, v := range list.All() {
		got = append(got, v.S)
	}
	if expect := []string{"00", "01", "02"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After Trim(3) got %v, expected %v", got, expect)
	}

	// n <= 0 empties the list.
	if err := list.Trim(0); err != nil || !list.IsEmpty() {
		t.Errorf("Trim(0) = %v, empty=%v; expected emptied list", err, list.IsEmpty())
	}

	// TrimTail keeps the tail.
	for i := range 5 {
		list.AppendAtTail(TestDllItem{S: fmt.Sprintf("%02d", i)})
	}
	if err := list.TrimTail(2); err != nil {
		t.Fatalf("TrimTail(2): %v", err)
	}
	got = nil
	for _, v := range list.All() {
		got = append(got, v.S)
	}
	if expect := []string{"03", "04"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After TrimTail(2) got %v, expected %v", got, expect)
	}
	if err := list.TrimTail(-1); err != nil || !list.IsEmpty() {
		t.Errorf("TrimTail(-1) = %v, empty=%v; expected emptied list", err, list.IsEmpty())
	}

	// Reusable after the drains.
	list.AppendAtTail(TestDllItem{S: "99"})
	if list.Length() != 1 {
		t.Errorf("Expected reusable list of length 1, got %d", list.Length())
	}
}

// TestDeleteByValue deletes by value through the search-based Delete.
func TestDeleteByValue(t *testing.T) {
	list := newTestDll()
	for _, s := range []string{"a", "b", "c", "b", "d"} {
		list.AppendAtTail(TestDllItem{S: s})
	}

	// Delete removes only the first match.
	if err := list.Delete(TestDllItem{S: "b"}); err != nil {
		t.Fatalf("Delete(b): %v", err)
	}
	var got []string
	for _, v := range list.All() {
		got = append(got, v.S)
	}
	if expect := []string{"a", "c", "b", "d"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After Delete(b) got %v, expected %v", got, expect)
	}
}

// TestReverseSearchPos verifies positions reported by ReverseSearch.
func TestReverseSearchPos(t *testing.T) {
	list := newTestDll()
	for _, s := range []string{"a", "b", "c", "b", "d"} {
		list.AppendAtTail(TestDllItem{S: s})
	}

	// ReverseSearch finds the LAST match (searching from the tail).
	el, pos := list.ReverseSearch(TestDllItem{S: "b"})
	if el == nil || pos != 3 {
		t.Errorf("ReverseSearch(b) = (%v, %d), expected pos 3", el, pos)
	}
	// Search finds the FIRST match.
	el, pos = list.Search(TestDllItem{S: "b"})
	if el == nil || pos != 1 {
		t.Errorf("Search(b) = (%v, %d), expected pos 1", el, pos)
	}
	// Not found.
	if el, pos := list.ReverseSearch(TestDllItem{S: "z"}); el != nil || pos != -1 {
		t.Errorf("ReverseSearch(z) = (%v, %d), expected (nil, -1)", el, pos)
	}
}

// TestReverseAndConcat verifies Reverse plus Concat, including
// self-concatenation.
func TestReverseAndConcat(t *testing.T) {
	a := newTestDll()
	for _, s := range []string{"1", "2", "3"} {
		a.AppendAtTail(TestDllItem{S: s})
	}
	a.Reverse()
	var got []string
	for _, v := range a.All() {
		got = append(got, v.S)
	}
	if expect := []string{"3", "2", "1"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After Reverse got %v, expected %v", got, expect)
	}
	// Reverse is its own inverse.
	a.Reverse()
	got = nil
	for _, v := range a.All() {
		got = append(got, v.S)
	}
	if expect := []string{"1", "2", "3"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After double Reverse got %v, expected %v", got, expect)
	}

	b := newTestDll()
	for _, s := range []string{"4", "5"} {
		b.AppendAtTail(TestDllItem{S: s})
	}
	a.Concat(b)
	got = nil
	for _, v := range a.All() {
		got = append(got, v.S)
	}
	if expect := []string{"1", "2", "3", "4", "5"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After Concat got %v, expected %v", got, expect)
	}
	if b.Length() != 2 {
		t.Errorf("Concat changed the source list, length %d", b.Length())
	}

	// Self-concatenation doubles the list.
	a.Concat(a)
	if a.Length() != 10 {
		t.Errorf("Expected length 10 after self-concat, got %d", a.Length())
	}
	got = nil
	for _, v := range a.All() {
		got = append(got, v.S)
	}
	if expect := []string{"1", "2", "3", "4", "5", "1", "2", "3", "4", "5"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After self-concat got %v", got)
	}

	// Concatenating an empty/nil list is a no-op.
	c := newTestDll()
	a.Concat(c)
	a.Concat(nil)
	if a.Length() != 10 {
		t.Errorf("Expected length unchanged after empty concat, got %d", a.Length())
	}
}

// TestIndexFromTail verifies positional access counted from the tail.
func TestIndexFromTail(t *testing.T) {
	list := newTestDll()
	for i := range 6 {
		list.AppendAtTail(TestDllItem{S: fmt.Sprintf("%02d", i)})
	}

	for sub, want := range []string{"05", "04", "03", "02", "01", "00"} {
		el, err := list.IndexFromTail(sub)
		if err != nil {
			t.Errorf("IndexFromTail(%d): %v", sub, err)
		} else if el.GetData().S != want {
			t.Errorf("IndexFromTail(%d) = %s, expected %s", sub, el.GetData().S, want)
		}
	}

	if _, err := list.IndexFromTail(6); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from IndexFromTail(6), got %v", err)
	}
	if _, err := list.IndexFromTail(-1); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from IndexFromTail(-1), got %v", err)
	}

	empty := newTestDll()
	if _, err := empty.IndexFromTail(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from IndexFromTail on empty list, got %v", err)
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: builtin equality, equality functions, zero value, nil receiver
// -------------------------------------------------------------------------------------------------------

// TestNewDllBuiltin verifies the constructor for types comparable with ==
// — no interface, no boxing.
func TestNewDllBuiltin(t *testing.T) {
	list := NewDll[int]()
	if !list.InsertBeforeHead(42) {
		t.Errorf("Expected InsertBeforeHead to return true.")
	}
	list.Push(7)
	list.Push(99)

	if el, pos := list.Search(7); el == nil || pos != 1 {
		t.Errorf("Search(7) = (%v, %d), expected pos 1", el, pos)
	}
	if _, pos := list.Search(43); pos != -1 {
		t.Errorf("Search(43) pos = %d, expected -1", pos)
	}
	if err := list.Delete(99); err != nil {
		t.Errorf("Delete(99): %v", err)
	}
	if list.Length() != 2 {
		t.Errorf("Expected length 2, got %d", list.Length())
	}

	// == equality distinguishes every field of a comparable struct.
	type point struct{ X, Y int }
	pl := NewDll[point]()
	pl.Push(point{1, 2})
	pl.Push(point{1, 3})
	if el, _ := pl.Search(point{1, 3}); el == nil {
		t.Errorf("Expected to find {1,3} with builtin == equality.")
	}
	if _, pos := pl.Search(point{1, 4}); pos != -1 {
		t.Errorf("Expected not to find {1,4}.")
	}
}

// TestNewDllFunc verifies the constructor with a caller supplied equality
// function, including equality by a single field.
func TestNewDllFunc(t *testing.T) {
	byS := NewDllFunc(eqTestDllItem)
	byS.AppendAtTail(TestDllItem{S: "a"})
	byS.AppendAtTail(TestDllItem{S: "b"})
	if el, _ := byS.Search(TestDllItem{S: "b"}); el == nil {
		t.Errorf("Expected to find b with function equality.")
	}
	if _, pos := byS.Search(TestDllItem{S: "z"}); pos != -1 {
		t.Errorf("Expected not to find z.")
	}

	// Equality by a field other than the natural identity.
	type rec struct {
		ID   int
		Name string
	}
	byID := NewDllFunc(func(a, b rec) bool { return a.ID == b.ID })
	byID.AppendAtTail(rec{ID: 1, Name: "ada"})
	byID.AppendAtTail(rec{ID: 2, Name: "grace"})
	if el, _ := byID.Search(rec{ID: 2}); el == nil || el.GetData().Name != "grace" {
		t.Errorf("Expected field-based equality to find grace by ID alone.")
	}

	// Types that are not comparable with == work through the function.
	slices := NewDllFunc(func(a, b []int) bool {
		return len(a) == len(b)
	})
	slices.AppendAtTail([]int{1})
	slices.AppendAtTail([]int{1, 2})
	if el, _ := slices.Search([]int{9, 9}); el == nil {
		t.Errorf("Expected slice equality by length to find a match.")
	}
}

// TestNewDllFuncNil verifies that a nil equality function is rejected at
// construction time, not on first use.
func TestNewDllFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewDllFunc(nil) to panic.")
		}
	}()
	NewDllFunc[TestDllItem](nil)
}

// TestZeroValueList verifies that the zero value of Dll behaves as an
// empty list for every non-insert operation and that the insert family
// fails loudly because no equality function has been set.
func TestZeroValueList(t *testing.T) {
	var list Dll[TestDllItem]

	if !list.IsEmpty() {
		t.Errorf("Expected zero value list to be empty.")
	}
	if list.Len() != 0 || list.Length() != 0 {
		t.Errorf("Expected zero value list to have length 0.")
	}
	if _, pos := list.Search(TestDllItem{S: "x"}); pos != -1 {
		t.Errorf("Expected not-found from Search on zero value list.")
	}
	if _, pos := list.ReverseSearch(TestDllItem{S: "x"}); pos != -1 {
		t.Errorf("Expected not-found from ReverseSearch on zero value list.")
	}
	if err := list.Delete(TestDllItem{S: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from Delete on zero value list, got %v", err)
	}
	if _, err := list.Pop(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Pop on zero value list, got %v", err)
	}
	if _, err := list.PopTail(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from PopTail on zero value list, got %v", err)
	}
	if _, err := list.Peek(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Peek on zero value list, got %v", err)
	}
	if _, err := list.Index(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from Index on zero value list, got %v", err)
	}
	if err := list.Trim(3); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Trim on zero value list, got %v", err)
	}
	list.Truncate() // no-op, must not panic
	list.Reverse()  // no-op, must not panic
	if el, pos := list.Walk(func(pos int, data TestDllItem) bool { return true }); el != nil || pos != -1 {
		t.Errorf("Expected no visits from Walk on zero value list.")
	}
	if it := list.Front(); !it.Done() {
		t.Errorf("Expected Front on zero value list to be Done immediately.")
	}
	for range list.All() {
		t.Errorf("Expected no values from All on zero value list.")
	}

	// The insert family panics with a clear message naming the fix.
	for name, fx := range map[string]func(){
		"InsertBeforeHead": func() { list.InsertBeforeHead(TestDllItem{S: "x"}) },
		"Push":             func() { list.Push(TestDllItem{S: "x"}) },
		"AppendAtTail":     func() { list.AppendAtTail(TestDllItem{S: "x"}) },
		"Enqueue":          func() { list.Enqueue(TestDllItem{S: "x"}) },
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on zero value list to panic.", name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDll") {
					t.Errorf("%s: unexpected panic message: %v", name, r)
				}
			}()
			fx()
		}()
	}
}

// TestNilListTolerated verifies that every non-insert operation treats a
// nil list as an empty list, and that the insert family panics with a
// message naming the method.
func TestNilListTolerated(t *testing.T) {
	var list *Dll[TestDllItem]

	if !list.IsEmpty() {
		t.Errorf("Expected nil list to be empty.")
	}
	if list.Len() != 0 || list.Length() != 0 {
		t.Errorf("Expected nil list to have length 0.")
	}
	if _, pos := list.Search(TestDllItem{S: "x"}); pos != -1 {
		t.Errorf("Expected not-found from Search on nil list.")
	}
	if err := list.Delete(TestDllItem{S: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from Delete on nil list, got %v", err)
	}
	if _, err := list.Pop(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Pop on nil list, got %v", err)
	}
	if _, err := list.PeekTail(); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from PeekTail on nil list, got %v", err)
	}
	if _, err := list.Index(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Expected ErrOutOfRange from Index on nil list, got %v", err)
	}
	if err := list.Trim(3); !errors.Is(err, ErrEmptyDll) {
		t.Errorf("Expected ErrEmptyDll from Trim on nil list, got %v", err)
	}
	list.Reverse()   // no-op
	list.Lock()      // no-op
	list.Truncate()  // no-op
	list.Unlock()    // no-op
	list.Concat(nil) // no-op
	if el, pos := list.ReverseWalk(func(pos int, data TestDllItem) bool { return true }); el != nil || pos != -1 {
		t.Errorf("Expected no visits from ReverseWalk on nil list.")
	}
	if it := list.Rear(); !it.Done() {
		t.Errorf("Expected Rear on nil list to be Done immediately.")
	}
	for range list.All() {
		t.Errorf("Expected no values from All on nil list.")
	}
	for range list.Backward() {
		t.Errorf("Expected no values from Backward on nil list.")
	}

	// The insert family panics with a message naming the method.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Push on nil list to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Push") && !strings.Contains(msg, "InsertBeforeHead") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		list.Push(TestDllItem{S: "x"})
	}()
}
