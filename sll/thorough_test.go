package sll

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// checkInvariants walks the list and verifies that the internal structure is
// consistent: node count equals Length(), head/tail are nil exactly when the
// list is empty, and tail is the last node reachable from head.
func checkInvariants[T comparable.Equality](t *testing.T, ns *Sll[T], context string) {
	t.Helper()
	n := 0
	var last *SllElement[T]
	for p := ns.head; p != nil; p = p.next {
		last = p
		n++
	}
	if n != ns.length {
		t.Errorf("%s: walked %d nodes but Length() reports %d", context, n, ns.length)
	}
	if ns.length == 0 {
		if ns.head != nil || ns.tail != nil {
			t.Errorf("%s: empty list must have nil head and tail", context)
		}
	} else {
		if ns.head == nil || ns.tail == nil {
			t.Errorf("%s: non-empty list must have non-nil head and tail", context)
		}
		if ns.tail != last {
			t.Errorf("%s: tail pointer does not point at the last node", context)
		}
		if ns.tail.next != nil {
			t.Errorf("%s: tail node must have nil next", context)
		}
	}
}

func TestNewSllZeroValue(t *testing.T) {
	// Both the constructor and a bare zero-value declaration must yield a
	// usable, empty list.
	lists := map[string]*Sll[TestDemo]{
		"NewSll":    NewSll[TestDemo](),
		"zeroValue": &Sll[TestDemo]{},
	}
	for name, list := range lists {
		if !list.IsEmpty() {
			t.Errorf("%s: expected new list to be empty", name)
		}
		if got := list.Length(); got != 0 {
			t.Errorf("%s: expected length 0, got %d", name, got)
		}
		if _, err := list.Pop(); err != ErrEmptySll {
			t.Errorf("%s: expected ErrEmptySll from Pop, got %v", name, err)
		}
		if _, err := list.Peek(); err != ErrEmptySll {
			t.Errorf("%s: expected ErrEmptySll from Peek, got %v", name, err)
		}
		if err := list.DeleteFound(nil); err != ErrEmptySll {
			t.Errorf("%s: expected ErrEmptySll from DeleteFound, got %v", name, err)
		}
		// Mutating an empty list in harmless ways must keep it usable.
		list.Truncate()
		list.Reverse()
		checkInvariants(t, list, name+" after Truncate/Reverse")
		list.Push(&TestDemo{S: "x"})
		if got := list.Length(); got != 1 {
			t.Errorf("%s: expected length 1 after Push, got %d", name, got)
		}
		checkInvariants(t, list, name+" after Push")
	}
}

func TestSingleElementList(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "only"})

	v, err := list.Peek()
	if err != nil || v.S != "only" {
		t.Errorf("Peek: expected 'only', got %v err %v", v, err)
	}

	list.Reverse() // reversing a single element must be a no-op
	checkInvariants(t, &list, "after single Reverse")
	v, err = list.Pop()
	if err != nil || v.S != "only" {
		t.Errorf("Pop: expected 'only', got %v err %v", v, err)
	}
	checkInvariants(t, &list, "after single Pop")
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after popping the only element")
	}
}

func TestDuplicates(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "dup"})
	list.InsertAfterTail(&TestDemo{S: "mid"})
	list.InsertAfterTail(&TestDemo{S: "dup"})
	if got := list.Length(); got != 3 {
		t.Fatalf("Expected length 3, got %d", got)
	}

	// Search finds the first duplicate.
	_, pos := list.Search(&TestDemo{S: "dup"})
	if pos != 0 {
		t.Errorf("Search: expected first duplicate at pos 0, got %d", pos)
	}

	// Delete removes only the first occurrence.
	if err := list.Delete(&TestDemo{S: "dup"}); err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}
	checkInvariants(t, &list, "after deleting first duplicate")
	got := []string{}
	for _, v := range list.IterateOver() {
		got = append(got, v.S)
	}
	want := []string{"mid", "dup"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("Expected %v, got %v", want, got)
	}
}

func TestDeleteFoundEdgeCases(t *testing.T) {
	// Empty list reports ErrEmptySll.
	var list Sll[TestDemo]
	if err := list.DeleteFound(&SllElement[TestDemo]{}); err != ErrEmptySll {
		t.Errorf("Expected ErrEmptySll on empty list, got %v", err)
	}

	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})

	// A nil element reports ErrNotFound.
	if err := list.DeleteFound(nil); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for nil element, got %v", err)
	}
	// An element with nil data reports ErrNotFound.
	if err := list.DeleteFound(&SllElement[TestDemo]{}); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for element with nil data, got %v", err)
	}
	// An element whose data matches nothing reports ErrNotFound.
	stray := &SllElement[TestDemo]{data: &TestDemo{S: "zz"}}
	if err := list.DeleteFound(stray); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for non-matching element, got %v", err)
	}
	if got := list.Length(); got != 2 {
		t.Errorf("Failed deletes must not change length; expected 2, got %d", got)
	}

	// Deleting the tail via DeleteFound must update the tail pointer.
	el, pos := list.Search(&TestDemo{S: "02"})
	if pos != 1 || el == nil {
		t.Fatalf("Search: expected 02 at pos 1")
	}
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound: unexpected error %v", err)
	}
	checkInvariants(t, &list, "after DeleteFound of tail")

	// The deleted element is unlinked and cleared.
	if el.GetData() != nil {
		t.Errorf("Expected deleted element's data to be cleared")
	}

	// List is still usable at both ends.
	list.InsertAfterTail(&TestDemo{S: "03"})
	checkInvariants(t, &list, "after InsertAfterTail post-delete")
	v, err := list.Pop()
	if err != nil || v.S != "01" {
		t.Errorf("Expected to pop 01, got %v err %v", v, err)
	}
}

func TestCursorIteratorEdgeCases(t *testing.T) {
	// A cursor on an empty list is immediately done and yields nil values.
	var empty Sll[TestDemo]
	it := empty.Front()
	if !it.Done() {
		t.Errorf("Expected Done() on empty list iterator")
	}
	if v := it.Value(); v != nil {
		t.Errorf("Expected nil Value() on empty list iterator, got %v", v)
	}
	if p := it.Pos(); p != 0 {
		t.Errorf("Expected Pos() 0 on fresh iterator, got %d", p)
	}
	// Next on an exhausted iterator must be a safe no-op.
	it.Next()
	if !it.Done() {
		t.Errorf("Expected Done() after Next() on empty iterator")
	}

	// Current starts iteration from a found element, preserving its position.
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.InsertAfterTail(&TestDemo{S: "03"})
	el, pos := list.Search(&TestDemo{S: "02"})
	if pos != 1 || el == nil {
		t.Fatalf("Search: expected 02 at pos 1")
	}
	ci := list.Current(el, pos)
	if ci.Pos() != 1 {
		t.Errorf("Expected Current iterator Pos() 1, got %d", ci.Pos())
	}
	got := []string{}
	for ; !ci.Done(); ci.Next() {
		got = append(got, ci.Value().S)
	}
	want := []string{"02", "03"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("Current iteration: expected %v, got %v", want, got)
	}

	// Value must return nil after the cursor runs off the end.
	if v := ci.Value(); v != nil {
		t.Errorf("Expected nil Value() past end of list, got %v", v)
	}
}

func TestDump(t *testing.T) {
	var buf bytes.Buffer

	var empty Sll[TestDemo]
	empty.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty list, got %q", buf.String())
	}

	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "aa"})
	list.InsertAfterTail(&TestDemo{S: "bb"})
	list.Dump(&buf)
	want := "0: {S:aa}\n1: {S:bb}\n"
	if buf.String() != want {
		t.Errorf("Dump: expected %q, got %q", want, buf.String())
	}
}

func TestReverseEdgeCases(t *testing.T) {
	// Reverse on empty list is a no-op.
	var list Sll[TestDemo]
	list.Reverse()
	checkInvariants(t, &list, "after Reverse on empty")

	// Build head-inserted list, reverse twice, expect original order.
	for _, s := range []string{"01", "02", "03", "04"} {
		list.InsertAfterTail(&TestDemo{S: s})
	}
	before := []string{}
	for _, v := range list.IterateOver() {
		before = append(before, v.S)
	}
	list.Reverse()
	checkInvariants(t, &list, "after first Reverse")
	list.Reverse()
	checkInvariants(t, &list, "after second Reverse")
	after := []string{}
	for _, v := range list.IterateOver() {
		after = append(after, v.S)
	}
	if fmt.Sprintf("%v", before) != fmt.Sprintf("%v", after) {
		t.Errorf("Double reverse must restore order: before %v after %v", before, after)
	}
}

func TestIteratePtrEarlyBreak(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.InsertAfterTail(&TestDemo{S: "03"})

	n := 0
	for i, v := range list.IteratePtr() {
		if i != n {
			t.Errorf("Unexpected index %d at iteration %d", i, n)
		}
		if v == nil {
			t.Fatalf("IteratePtr must never yield a nil pointer")
		}
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early exit after 1 element, got %d", n)
	}

	// Empty list yields nothing.
	var empty Sll[TestDemo]
	for range empty.IteratePtr() {
		t.Errorf("Expected no elements from empty list")
	}
}

func TestTruncateReuse(t *testing.T) {
	var list Sll[TestDemo]
	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.Truncate()
	checkInvariants(t, &list, "after Truncate")

	// Both insertion ends must work after truncation.
	list.InsertBeforeHead(&TestDemo{S: "h"})
	checkInvariants(t, &list, "after InsertBeforeHead post-Truncate")
	list.Truncate()
	list.InsertAfterTail(&TestDemo{S: "t"})
	checkInvariants(t, &list, "after InsertAfterTail post-Truncate")
	v, err := list.Pop()
	if err != nil || v.S != "t" {
		t.Errorf("Expected to pop 't', got %v err %v", v, err)
	}
}

// TestRandomizedModel cross-checks the list against a plain slice reference
// model over hundreds of mixed operations with a fixed seed.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	var list Sll[TestDemo]
	model := []string{}
	const ops = 2000

	valueFor := func(i int) string {
		// Small value space so duplicates and search hits are frequent.
		return fmt.Sprintf("%02d", rng.Intn(8))
	}

	for op := 0; op < ops; op++ {
		switch rng.Intn(9) {
		case 0: // Push (prepend)
			s := valueFor(op)
			list.Push(&TestDemo{S: s})
			model = append([]string{s}, model...)
		case 1: // InsertAfterTail (append)
			s := valueFor(op)
			list.InsertAfterTail(&TestDemo{S: s})
			model = append(model, s)
		case 2: // InsertBeforeHead (prepend)
			s := valueFor(op)
			list.InsertBeforeHead(&TestDemo{S: s})
			model = append([]string{s}, model...)
		case 3: // Pop
			v, err := list.Pop()
			if len(model) == 0 {
				if err != ErrEmptySll {
					t.Fatalf("op %d: Pop on empty: expected ErrEmptySll, got %v", op, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("op %d: Pop: expected %s, got %v err %v", op, model[0], v, err)
				}
				model = model[1:]
			}
		case 4: // Peek
			v, err := list.Peek()
			if len(model) == 0 {
				if err != ErrEmptySll {
					t.Fatalf("op %d: Peek on empty: expected ErrEmptySll, got %v", op, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("op %d: Peek: expected %s, got %v err %v", op, model[0], v, err)
				}
			}
		case 5: // Delete by value
			s := valueFor(op)
			err := list.Delete(&TestDemo{S: s})
			idx := -1
			for i, m := range model {
				if m == s {
					idx = i
					break
				}
			}
			if idx < 0 {
				if err != ErrNotFound {
					t.Fatalf("op %d: Delete(%s): expected ErrNotFound, got %v", op, s, err)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: Delete(%s): unexpected error %v", op, s, err)
				}
				model = append(model[:idx], model[idx+1:]...)
			}
		case 6: // Search
			s := valueFor(op)
			_, pos := list.Search(&TestDemo{S: s})
			wantPos := -1
			for i, m := range model {
				if m == s {
					wantPos = i
					break
				}
			}
			if pos != wantPos {
				t.Fatalf("op %d: Search(%s): expected pos %d, got %d", op, s, wantPos, pos)
			}
		case 7: // Reverse
			list.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		case 8: // Truncate
			list.Truncate()
			model = model[:0]
		}

		// Length and emptiness must agree with the model after every op.
		if got := list.Length(); got != len(model) {
			t.Fatalf("op %d: expected length %d, got %d", op, len(model), got)
		}
		if list.IsEmpty() != (len(model) == 0) {
			t.Fatalf("op %d: IsEmpty() disagrees with model", op)
		}
		checkInvariants(t, &list, fmt.Sprintf("op %d", op))
	}

	// Final full comparison of list contents against the model.
	i := 0
	for _, v := range list.IterateOver() {
		if i >= len(model) || v.S != model[i] {
			t.Fatalf("final content mismatch at %d: model %v", i, model)
		}
		i++
	}
	if i != len(model) {
		t.Fatalf("final content length mismatch: walked %d, model has %d", i, len(model))
	}
}
