package sll_ts

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// collect is a helper that drains IterateOver into a slice of strings.
func collect(list *Sll[TestDemo]) []string {
	out := []string{}
	for _, v := range list.IterateOver() {
		out = append(out, v.S)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// checkInvariant verifies that length, iteration order, and Pop order agree.
func checkInvariant(t *testing.T, list *Sll[TestDemo], want []string) {
	t.Helper()
	if got := list.Length(); got != len(want) {
		t.Fatalf("Length: expected %d got %d", len(want), got)
	}
	if got := list.IsEmpty(); got != (len(want) == 0) {
		t.Fatalf("IsEmpty: expected %v got %v", len(want) == 0, got)
	}
	if got := collect(list); !sameStrings(got, want) {
		t.Fatalf("IterateOver: expected %v got %v", want, got)
	}
}

// TestCurrentIterator covers Current: start an iteration from a Search result.
func TestCurrentIterator(t *testing.T) {
	list := NewSll[TestDemo]()
	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	list.InsertAfterTail(&TestDemo{S: "03"})

	el, pos := list.Search(&TestDemo{S: "02"})
	if el == nil || pos != 1 {
		t.Fatalf("Search: expected 02 at pos 1, got el %v pos %d", el, pos)
	}

	it := list.Current(el, pos)
	if it.Pos() != 1 {
		t.Errorf("Current: expected starting Pos 1, got %d", it.Pos())
	}
	got := []string{}
	for ; !it.Done(); it.Next() {
		got = append(got, it.Value().S)
	}
	want := []string{"02", "03"}
	if !sameStrings(got, want) {
		t.Errorf("Current iteration: expected %v got %v", want, got)
	}
	// Pos must have tracked the two Next calls.
	if it.Pos() != 3 {
		t.Errorf("Current: expected final Pos 3, got %d", it.Pos())
	}
}

// TestCursorEdgeCases covers the cursor iterator on an empty list and the
// nil-current branches of Value and Next.
func TestCursorEdgeCases(t *testing.T) {
	list := NewSll[TestDemo]()

	it := list.Front()
	if !it.Done() {
		t.Errorf("Front on empty list: expected Done() true")
	}
	if v := it.Value(); v != nil {
		t.Errorf("Value on empty iterator: expected nil, got %v", v)
	}
	if p := it.Pos(); p != 0 {
		t.Errorf("Pos on empty iterator: expected 0, got %d", p)
	}
	// Next on an exhausted/empty cursor must be a no-op, not a panic.
	it.Next()
	if !it.Done() {
		t.Errorf("Done after Next on empty iterator: expected true")
	}

	// A cursor that walks off the end of a non-empty list.
	list.Push(&TestDemo{S: "01"})
	it = list.Front()
	if it.Done() {
		t.Errorf("Done at head of non-empty list: expected false")
	}
	if v := it.Value(); v == nil || v.S != "01" {
		t.Errorf("Value at head: expected 01, got %v", v)
	}
	it.Next()
	if !it.Done() {
		t.Errorf("Done after walking past single element: expected true")
	}
	if v := it.Value(); v != nil {
		t.Errorf("Value past end: expected nil, got %v", v)
	}
	it.Next() // no-op past the end
	if p := it.Pos(); p != 1 {
		t.Errorf("Pos past end: expected 1, got %d", p)
	}
}

// TestNilAndEmptyErrors covers the error-return branches that the happy-path
// tests miss.
func TestNilAndEmptyErrors(t *testing.T) {
	list := NewSll[TestDemo]()

	// Delete on an empty list reports ErrEmptySll.
	if err := list.Delete(&TestDemo{S: "01"}); err != ErrEmptySll {
		t.Errorf("Delete on empty list: expected ErrEmptySll, got %v", err)
	}

	// Search(nil) reports not found without dereferencing nil.
	if el, pos := list.Search(nil); el != nil || pos != -1 {
		t.Errorf("Search(nil): expected (nil, -1), got (%v, %d)", el, pos)
	}

	// DeleteFound(nil) reports ErrNotFound.
	if err := list.DeleteFound(nil); err != ErrNotFound {
		t.Errorf("DeleteFound(nil): expected ErrNotFound, got %v", err)
	}

	// DeleteFound on an element with nil data reports ErrNotFound.
	if err := list.DeleteFound(&SllElement[TestDemo]{}); err != ErrNotFound {
		t.Errorf("DeleteFound(element with nil data): expected ErrNotFound, got %v", err)
	}

	// Delete(nil) on a non-empty list reports ErrNotFound.
	list.Push(&TestDemo{S: "01"})
	if err := list.Delete(nil); err != ErrNotFound {
		t.Errorf("Delete(nil): expected ErrNotFound, got %v", err)
	}
	if got := list.Length(); got != 1 {
		t.Errorf("Delete(nil) must not modify the list: expected length 1, got %d", got)
	}
}

// TestDeleteFoundEmpty covers DeleteFound on an empty list with a valid element.
func TestDeleteFoundEmpty(t *testing.T) {
	list := NewSll[TestDemo]()
	v := TestDemo{S: "01"}
	if err := list.DeleteFound(&SllElement[TestDemo]{data: &v}); err != ErrEmptySll {
		t.Errorf("DeleteFound on empty list: expected ErrEmptySll, got %v", err)
	}
}

// TestDump verifies the debug dump output format and content.
func TestDump(t *testing.T) {
	list := NewSll[TestDemo]()

	var buf bytes.Buffer
	list.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Dump of empty list: expected no output, got %q", buf.String())
	}

	list.InsertAfterTail(&TestDemo{S: "01"})
	list.InsertAfterTail(&TestDemo{S: "02"})
	buf.Reset()
	list.Dump(&buf)
	want := "0: {S:01}\n1: {S:02}\n"
	if buf.String() != want {
		t.Errorf("Dump: expected %q got %q", want, buf.String())
	}
}

// TestIterateEarlyBreak covers the !yield return path in both range-over-func
// iterators.
func TestIterateEarlyBreak(t *testing.T) {
	list := NewSll[TestDemo]()
	for i := 1; i <= 5; i++ {
		list.InsertAfterTail(&TestDemo{S: fmt.Sprintf("%02d", i)})
	}

	n := 0
	for _, v := range list.IterateOver() {
		n++
		if v.S == "02" {
			break
		}
	}
	if n != 2 {
		t.Errorf("IterateOver early break: expected 2 yields, got %d", n)
	}

	n = 0
	for _, v := range list.IteratePtr() {
		n++
		if v.S == "03" {
			break
		}
	}
	if n != 3 {
		t.Errorf("IteratePtr early break: expected 3 yields, got %d", n)
	}

	// IteratePtr on an empty list yields nothing.
	empty := NewSll[TestDemo]()
	for range empty.IteratePtr() {
		t.Errorf("IteratePtr on empty list: expected no elements")
	}
}

// TestIteratePtrAliases verifies that IteratePtr hands out pointers that alias
// the stored data (mutating through them changes what Pop returns).
func TestIteratePtrAliases(t *testing.T) {
	list := NewSll[TestDemo]()
	list.Push(&TestDemo{S: "01"})
	for _, v := range list.IteratePtr() {
		v.S = "changed"
	}
	got, err := list.Pop()
	if err != nil {
		t.Fatalf("Pop: unexpected error %v", err)
	}
	if got.S != "changed" {
		t.Errorf("IteratePtr must alias stored data: expected changed, got %s", got.S)
	}
}

// TestDuplicates verifies that duplicate values are allowed and that Delete
// removes only the first equal element.
func TestDuplicates(t *testing.T) {
	list := NewSll[TestDemo]()
	list.InsertAfterTail(&TestDemo{S: "x"})
	list.InsertAfterTail(&TestDemo{S: "y"})
	list.InsertAfterTail(&TestDemo{S: "x"})
	checkInvariant(t, list, []string{"x", "y", "x"})

	if err := list.Delete(&TestDemo{S: "x"}); err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}
	checkInvariant(t, list, []string{"y", "x"})

	// Search finds the first duplicate.
	el, pos := list.Search(&TestDemo{S: "x"})
	if el == nil || pos != 1 {
		t.Errorf("Search for duplicate: expected pos 1, got el %v pos %d", el, pos)
	}

	if err := list.Delete(&TestDemo{S: "x"}); err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}
	checkInvariant(t, list, []string{"y"})
}

// TestReverseEdgeCases covers Reverse on empty, single, and two element lists,
// and that a double reverse restores the original order.
func TestReverseEdgeCases(t *testing.T) {
	empty := NewSll[TestDemo]()
	empty.Reverse()
	checkInvariant(t, empty, []string{})

	one := NewSll[TestDemo]()
	one.Push(&TestDemo{S: "01"})
	one.Reverse()
	checkInvariant(t, one, []string{"01"})

	two := NewSll[TestDemo]()
	two.InsertAfterTail(&TestDemo{S: "01"})
	two.InsertAfterTail(&TestDemo{S: "02"})
	two.Reverse()
	checkInvariant(t, two, []string{"02", "01"})
	two.Reverse()
	checkInvariant(t, two, []string{"01", "02"})

	// After a reverse, both ends must still work.
	two.InsertBeforeHead(&TestDemo{S: "00"})
	two.InsertAfterTail(&TestDemo{S: "03"})
	checkInvariant(t, two, []string{"00", "01", "02", "03"})
}

// TestTruncateReuse verifies the list is fully usable after Truncate.
func TestTruncateReuse(t *testing.T) {
	list := NewSll[TestDemo]()
	for i := 0; i < 10; i++ {
		list.InsertAfterTail(&TestDemo{S: fmt.Sprintf("%02d", i)})
	}
	list.Truncate()
	checkInvariant(t, list, []string{})

	if _, err := list.Pop(); err != ErrEmptySll {
		t.Errorf("Pop after Truncate: expected ErrEmptySll, got %v", err)
	}
	if _, err := list.Peek(); err != ErrEmptySll {
		t.Errorf("Peek after Truncate: expected ErrEmptySll, got %v", err)
	}

	// Both insertion ends must work on the truncated (stale-tail-free) list.
	list.InsertAfterTail(&TestDemo{S: "t1"})
	list.InsertBeforeHead(&TestDemo{S: "h1"})
	checkInvariant(t, list, []string{"h1", "t1"})
}

// TestSingleElement covers operations on a one-element list, where head == tail.
func TestSingleElement(t *testing.T) {
	list := NewSll[TestDemo]()
	list.InsertAfterTail(&TestDemo{S: "only"})

	v, err := list.Peek()
	if err != nil || v.S != "only" {
		t.Errorf("Peek: expected only, got %v err %v", v, err)
	}

	el, pos := list.Search(&TestDemo{S: "only"})
	if el == nil || pos != 0 {
		t.Errorf("Search: expected (el, 0), got (%v, %d)", el, pos)
	}
	if el.GetData().S != "only" {
		t.Errorf("GetData: expected only, got %s", el.GetData().S)
	}

	// Delete the single element by value; head and tail must both clear.
	if err := list.Delete(&TestDemo{S: "only"}); err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}
	checkInvariant(t, list, []string{})

	// Reinsert at the tail after deleting the last element.
	list.InsertAfterTail(&TestDemo{S: "again"})
	checkInvariant(t, list, []string{"again"})
}

// TestRandomizedModel cross-checks the list against a slice reference model
// over hundreds of mixed operations with a fixed seed.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	list := NewSll[TestDemo]()
	model := []string{} // head-to-tail order

	removeFirst := func(s string) bool {
		for i, v := range model {
			if v == s {
				model = append(model[:i], model[i+1:]...)
				return true
			}
		}
		return false
	}

	const ops = 800
	for op := 0; op < ops; op++ {
		// Keep the value space small so duplicates and deletes are frequent.
		val := fmt.Sprintf("%02d", rng.Intn(20))
		switch rng.Intn(9) {
		case 0:
			list.InsertBeforeHead(&TestDemo{S: val})
			model = append([]string{val}, model...)
		case 1:
			list.InsertAfterTail(&TestDemo{S: val})
			model = append(model, val)
		case 2:
			list.Push(&TestDemo{S: val})
			model = append([]string{val}, model...)
		case 3:
			v, err := list.Pop()
			if len(model) == 0 {
				if err != ErrEmptySll {
					t.Fatalf("op %d: Pop on empty: expected ErrEmptySll, got %v", op, err)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: Pop: unexpected error %v", op, err)
				}
				if v.S != model[0] {
					t.Fatalf("op %d: Pop: expected %s got %s", op, model[0], v.S)
				}
				model = model[1:]
			}
		case 4:
			err := list.Delete(&TestDemo{S: val})
			found := removeFirst(val)
			if len(model) == 0 && !found {
				if err != ErrEmptySll {
					t.Fatalf("op %d: Delete on empty: expected ErrEmptySll, got %v", op, err)
				}
			} else if found {
				if err != nil {
					t.Fatalf("op %d: Delete %s: unexpected error %v", op, val, err)
				}
			} else {
				if err != ErrNotFound {
					t.Fatalf("op %d: Delete missing %s: expected ErrNotFound, got %v", op, val, err)
				}
			}
		case 5:
			v, err := list.Peek()
			if len(model) == 0 {
				if err != ErrEmptySll {
					t.Fatalf("op %d: Peek on empty: expected ErrEmptySll, got %v", op, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("op %d: Peek: expected %s got %v err %v", op, model[0], v, err)
				}
			}
		case 6:
			el, pos := list.Search(&TestDemo{S: val})
			want := -1
			for i, m := range model {
				if m == val {
					want = i
					break
				}
			}
			if pos != want {
				t.Fatalf("op %d: Search %s: expected pos %d got %d", op, val, want, pos)
			}
			if want == -1 && el != nil {
				t.Fatalf("op %d: Search %s: expected nil element", op, val)
			}
			if want >= 0 && (el == nil || el.GetData().S != val) {
				t.Fatalf("op %d: Search %s: bad element %v", op, val, el)
			}
		case 7:
			list.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		case 8:
			list.Truncate()
			model = []string{}
		}

		// Structural invariant after every operation: length, iteration
		// order (via the snapshot iterator), and head/tail behavior (via
		// Peek/Pop order) must match the model.
		if got := list.Length(); got != len(model) {
			t.Fatalf("op %d: Length: expected %d got %d", op, len(model), got)
		}
		if got := collect(list); !sameStrings(got, model) {
			t.Fatalf("op %d: contents: expected %v got %v", op, model, got)
		}
	}

	// Drain: Pop order must equal head-to-tail model order, then list is empty.
	for i, want := range model {
		v, err := list.Pop()
		if err != nil || v.S != want {
			t.Fatalf("drain %d: expected %s got %v err %v", i, want, v, err)
		}
	}
	checkInvariant(t, list, []string{})
}
