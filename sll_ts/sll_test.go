package sll_ts

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

// TestSllItem is the test element type.  Equality is
// supplied to the list as a plain function (eqTestSllItem below).
type TestSllItem struct {
	S string
}

// eqTestSllItem reports equality of TestSllItem by its S field.
func eqTestSllItem(a, b TestSllItem) bool {
	return a.S == b.S
}

// newTestSll builds an Sll of TestSllItem with equality by S.
func newTestSll() *Sll[TestSllItem] {
	return NewSllFunc(eqTestSllItem)
}

// valuesOf returns the current contents of the list, head to tail.
func valuesOf(list *Sll[TestSllItem]) []string {
	var got []string
	for _, v := range list.IterateOver() {
		got = append(got, v.S)
	}
	return got
}

func TestStack(t *testing.T) {
	list := newTestSll()

	if !list.IsEmpty() {
		t.Errorf("Expected empty list after declaration.")
	}

	if _, err := list.Pop(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Pop on empty list, got %v", err)
	}
	if _, err := list.Peek(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Peek on empty list, got %v", err)
	}

	list.Push(TestSllItem{S: "01"})
	list.Push(TestSllItem{S: "02"})
	list.Push(TestSllItem{S: "03"})

	if list.Length() != 3 {
		t.Errorf("Expected length 3, got %d", list.Length())
	}
	if v, err := list.Peek(); err != nil || v.S != "03" {
		t.Errorf("Peek = (%v, %v), expected 03", v, err)
	}

	// Stack order: last pushed is first popped.
	for _, want := range []string{"03", "02", "01"} {
		v, err := list.Pop()
		if err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if v.S != want {
			t.Errorf("Pop = %s, expected %s", v.S, want)
		}
	}
	if _, err := list.Pop(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll after draining, got %v", err)
	}
	if !list.IsEmpty() || list.Len() != 0 {
		t.Errorf("Expected empty drained list.")
	}
}

// TestInsertAfterTail verifies tail insertion builds head-to-tail order.
func TestInsertAfterTail(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"01", "02", "03"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[01 02 03]"; got != want {
		t.Errorf("After tail inserts got %s, expected %s", got, want)
	}

	// Pop-then-insert keeps head and tail consistent.
	if v, err := list.Pop(); err != nil || v.S != "01" {
		t.Errorf("Pop = (%v, %v), expected 01", v, err)
	}
	list.InsertAfterTail(TestSllItem{S: "04"})
	if got, want := fmt.Sprint(valuesOf(list)), "[02 03 04]"; got != want {
		t.Errorf("After pop+insert got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after pop+insert")
}

func TestReverse(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"01", "02", "03"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	list.Reverse()
	if got, want := fmt.Sprint(valuesOf(list)), "[03 02 01]"; got != want {
		t.Errorf("After reverse got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after reverse")

	// Reverse is its own inverse.
	list.Reverse()
	if got, want := fmt.Sprint(valuesOf(list)), "[01 02 03]"; got != want {
		t.Errorf("After double reverse got %s, expected %s", got, want)
	}

	// Reverse of empty and single-element lists are no-ops.
	empty := newTestSll()
	empty.Reverse()
	if !empty.IsEmpty() {
		t.Errorf("Expected empty list to stay empty after Reverse.")
	}
	single := newTestSll()
	single.InsertAfterTail(TestSllItem{S: "x"})
	single.Reverse()
	if got := valuesOf(single); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("Single-element reverse got %v", got)
	}
	checkInvariants(t, single, "after single reverse")
}

func TestIterateOver(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"01", "02", "03"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	var got []string
	for i, v := range list.IterateOver() {
		if i != len(got) {
			t.Fatalf("IterateOver: unexpected index %d at step %d", i, len(got))
		}
		got = append(got, v.S)
	}
	if expect := []string{"01", "02", "03"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("IterateOver got %v, expected %v", got, expect)
	}

	// Early break stops iteration.
	n := 0
	for range list.IterateOver() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Empty list yields nothing.
	empty := newTestSll()
	for range empty.IterateOver() {
		t.Errorf("Expected no items from IterateOver on empty list")
	}
}

func TestSearchDelete(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	el, pos := list.Search(TestSllItem{S: "b"})
	if el == nil || pos != 1 {
		t.Fatalf("Search(b) = (%v, %d), expected pos 1", el, pos)
	}
	if got := el.GetData().S; got != "b" {
		t.Errorf("GetData = %s, expected b", got)
	}
	if _, pos := list.Search(TestSllItem{S: "z"}); pos != -1 {
		t.Errorf("Search(z) pos = %d, expected -1", pos)
	}

	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound: %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[a c]"; got != want {
		t.Errorf("After DeleteFound got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after DeleteFound")

	if err := list.Delete(TestSllItem{S: "a"}); err != nil {
		t.Fatalf("Delete(a): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[c]"; got != want {
		t.Errorf("After Delete got %s, expected %s", got, want)
	}
	if err := list.Delete(TestSllItem{S: "z"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from Delete of absent item, got %v", err)
	}
}

// TestDeleteFoundSingleElement removes the only element through every
// delete path.
func TestDeleteFoundSingleElement(t *testing.T) {
	list := newTestSll()
	list.InsertAfterTail(TestSllItem{S: "only"})

	el, _ := list.Search(TestSllItem{S: "only"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(single): %v", err)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting the single element.")
	}
	checkInvariants(t, list, "after single delete")

	// DeleteFound on the emptied list.
	if err := list.DeleteFound(el); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from DeleteFound on empty list, got %v", err)
	}
	// DeleteFound(nil).
	list.InsertAfterTail(TestSllItem{S: "x"})
	if err := list.DeleteFound(nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from DeleteFound(nil), got %v", err)
	}
}

func TestPeek(t *testing.T) {
	list := newTestSll()
	if _, err := list.Peek(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Peek on empty list, got %v", err)
	}
	list.Push(TestSllItem{S: "head"})
	list.InsertAfterTail(TestSllItem{S: "tail"})
	if v, err := list.Peek(); err != nil || v.S != "head" {
		t.Errorf("Peek = (%v, %v), expected head", v, err)
	}
	// Peek does not remove.
	if list.Length() != 2 {
		t.Errorf("Expected length 2 after Peek, got %d", list.Length())
	}
}

func TestIter(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"01", "02", "03"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	expected := []string{"01", "02", "03"}

	for ii := list.Front(); !ii.Done(); ii.Next() {
		j := ii.Pos()
		if j < 0 || j >= len(expected) {
			t.Errorf("Unexpected position %d", j)
			continue
		}
		v, found := ii.Value()
		if !found {
			t.Fatalf("Value not found while not Done.")
		}
		if expected[j] != v.S {
			t.Errorf("Value got %s expected %s at pos %d", v.S, expected[j], j)
		}
	}

	// Current starts an iteration from a found position.
	rv, pos := list.Search(TestSllItem{S: "02"})
	if rv == nil {
		t.Fatalf("Expected to find 02.")
	}
	var got []string
	for it := list.Current(rv, pos); !it.Done(); it.Next() {
		v, _ := it.Value()
		got = append(got, v.S)
	}
	if expect := []string{"02", "03"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Current iteration got %v, expected %v", got, expect)
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: builtin equality, equality functions, zero value, nil list
// -------------------------------------------------------------------------------------------------------

// TestNewSllBuiltin verifies the constructor for types comparable with ==.
func TestNewSllBuiltin(t *testing.T) {
	list := NewSll[int]()
	list.Push(42)
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
	pl := NewSll[point]()
	pl.Push(point{1, 2})
	pl.Push(point{1, 3})
	if el, _ := pl.Search(point{1, 3}); el == nil {
		t.Errorf("Expected to find {1,3} with builtin == equality.")
	}
	if _, pos := pl.Search(point{1, 4}); pos != -1 {
		t.Errorf("Expected not to find {1,4}.")
	}
}

// TestNewSllFunc verifies the constructor with a caller supplied equality
// function, including equality by a single field.
func TestNewSllFunc(t *testing.T) {
	byS := NewSllFunc(eqTestSllItem)
	byS.InsertAfterTail(TestSllItem{S: "a"})
	byS.InsertAfterTail(TestSllItem{S: "b"})
	if el, _ := byS.Search(TestSllItem{S: "b"}); el == nil {
		t.Errorf("Expected to find b with function equality.")
	}
	if _, pos := byS.Search(TestSllItem{S: "z"}); pos != -1 {
		t.Errorf("Expected not to find z.")
	}

	// Equality by a field other than the natural identity.
	type rec struct {
		ID   int
		Name string
	}
	byID := NewSllFunc(func(a, b rec) bool { return a.ID == b.ID })
	byID.InsertAfterTail(rec{ID: 1, Name: "ada"})
	byID.InsertAfterTail(rec{ID: 2, Name: "grace"})
	if el, _ := byID.Search(rec{ID: 2}); el == nil || el.GetData().Name != "grace" {
		t.Errorf("Expected field-based equality to find grace by ID alone.")
	}

	// Types that are not comparable with == work through the function.
	slices := NewSllFunc(func(a, b []int) bool { return len(a) == len(b) })
	slices.InsertAfterTail([]int{1})
	slices.InsertAfterTail([]int{1, 2})
	if el, _ := slices.Search([]int{9, 9}); el == nil {
		t.Errorf("Expected slice equality by length to find a match.")
	}
}

// TestNewSllFuncNil verifies that a nil equality function is rejected at
// construction time, not on first use.
func TestNewSllFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewSllFunc(nil) to panic.")
		}
	}()
	NewSllFunc[TestSllItem](nil)
}

// TestZeroValueList verifies that the zero value of Sll behaves as an
// empty list for every non-insert operation and that the insert family
// fails loudly because no equality function has been set.
func TestZeroValueList(t *testing.T) {
	var list Sll[TestSllItem]

	if !list.IsEmpty() {
		t.Errorf("Expected zero value list to be empty.")
	}
	if list.Len() != 0 || list.Length() != 0 {
		t.Errorf("Expected zero value list to have length 0.")
	}
	if _, pos := list.Search(TestSllItem{S: "x"}); pos != -1 {
		t.Errorf("Expected not-found from Search on zero value list.")
	}
	if err := list.Delete(TestSllItem{S: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from Delete on zero value list, got %v", err)
	}
	if err := list.DeleteFound(nil); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from DeleteFound on zero value list, got %v", err)
	}
	if _, err := list.Pop(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Pop on zero value list, got %v", err)
	}
	if _, err := list.Peek(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Peek on zero value list, got %v", err)
	}
	list.Truncate() // no-op, must not panic
	list.Reverse()  // no-op, must not panic
	if it := list.Front(); !it.Done() {
		t.Errorf("Expected Front on zero value list to be Done immediately.")
	}
	for range list.IterateOver() {
		t.Errorf("Expected no values from IterateOver on zero value list.")
	}

	// The insert family panics with a clear message naming the fix.
	for name, fx := range map[string]func(){
		"InsertBeforeHead": func() { list.InsertBeforeHead(TestSllItem{S: "x"}) },
		"InsertAfterTail":  func() { list.InsertAfterTail(TestSllItem{S: "x"}) },
		"Push":             func() { list.Push(TestSllItem{S: "x"}) },
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on zero value list to panic.", name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSll") {
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
	var list *Sll[TestSllItem]

	if !list.IsEmpty() {
		t.Errorf("Expected nil list to be empty.")
	}
	if list.Len() != 0 || list.Length() != 0 {
		t.Errorf("Expected nil list to have length 0.")
	}
	if _, pos := list.Search(TestSllItem{S: "x"}); pos != -1 {
		t.Errorf("Expected not-found from Search on nil list.")
	}
	if err := list.Delete(TestSllItem{S: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from Delete on nil list, got %v", err)
	}
	if _, err := list.Pop(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Pop on nil list, got %v", err)
	}
	if _, err := list.Peek(); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from Peek on nil list, got %v", err)
	}
	list.Truncate() // no-op
	list.Reverse()  // no-op
	if it := list.Front(); !it.Done() {
		t.Errorf("Expected Front on nil list to be Done immediately.")
	}
	for range list.IterateOver() {
		t.Errorf("Expected no values from IterateOver on nil list.")
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
		list.Push(TestSllItem{S: "x"})
	}()
}
