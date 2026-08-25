package sll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"math/rand"
	"testing"
)

// checkInvariants verifies the structural invariants of the list: length
// matches the actual node count, head/tail are nil exactly when the list is
// empty, and tail is the last node of the chain.
func checkInvariants(t *testing.T, ns *Sll[TestDemo]) {
	t.Helper()
	n := 0
	var last *SllElement[TestDemo]
	for p := ns.head; p != nil; p = p.next {
		n++
		last = p
		if n > ns.length {
			t.Fatalf("list chain is longer than length %d (cycle or stale nodes)", ns.length)
		}
	}
	if n != ns.length {
		t.Errorf("invariant: walked %d nodes but Length() is %d", n, ns.length)
	}
	if ns.length == 0 {
		if ns.head != nil || ns.tail != nil {
			t.Errorf("invariant: empty list must have nil head and tail, got head=%v tail=%v", ns.head, ns.tail)
		}
	} else {
		if ns.head == nil || ns.tail == nil {
			t.Errorf("invariant: non-empty list must have non-nil head and tail")
		}
		if ns.tail != last {
			t.Errorf("invariant: tail is not the last node of the chain")
		}
		if ns.tail != nil && ns.tail.next != nil {
			t.Errorf("invariant: tail.next must be nil")
		}
	}
}

func toSlice(ns *Sll[TestDemo]) []string {
	var got []string
	for p := ns.head; p != nil; p = p.next {
		got = append(got, p.data.S)
	}
	return got
}

// TestZeroValue verifies that a zero-value Sll behaves as an empty list.
func TestZeroValue(t *testing.T) {
	var ns Sll[TestDemo]
	checkInvariants(t, &ns)

	if !ns.IsEmpty() {
		t.Errorf("Expected zero-value list to be empty")
	}
	if got := ns.Length(); got != 0 {
		t.Errorf("Expected Length 0, got %d", got)
	}
	if v, err := ns.Pop(); err != ErrEmptySll || v != nil {
		t.Errorf("Expected (nil, ErrEmptySll) from Pop on zero-value list, got (%v, %v)", v, err)
	}
	if !errors.Is(errors.Join(errors.New("wrap"), ErrEmptySll), ErrEmptySll) {
		t.Errorf("errors.Is sanity check failed")
	}
	if v, err := ns.Peek(); err != ErrEmptySll || v != nil {
		t.Errorf("Expected (nil, ErrEmptySll) from Peek on zero-value list, got (%v, %v)", v, err)
	}
	if el, pos := ns.Search(&TestDemo{S: "x"}); el != nil || pos != -1 {
		t.Errorf("Expected (nil, -1) from Search on zero-value list, got (%v, %d)", el, pos)
	}
	if err := ns.Delete(nil); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound deleting nil element, got %v", err)
	}

	// Old-style iterator on an empty list.
	it := ns.Front()
	if !it.Done() {
		t.Errorf("Expected Done() true on empty list")
	}
	if v := it.Value(); v != nil {
		t.Errorf("Expected Value() nil on empty list, got %v", v)
	}
	if p := it.Pos(); p != 0 {
		t.Errorf("Expected Pos() 0 on empty list, got %d", p)
	}
	it.Next() // must be a no-op, not a panic
	if !it.Done() {
		t.Errorf("Expected Done() still true after Next() on exhausted iterator")
	}

	// Range-over-func iterators on an empty list yield nothing.
	n := 0
	for range ns.IterateOver() {
		n++
	}
	for range ns.IteratePtr() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected 0 iterations over empty list, got %d", n)
	}

	// Truncate on an empty list must be a no-op.
	ns.Truncate()
	checkInvariants(t, &ns)
	if !ns.IsEmpty() {
		t.Errorf("Expected list still empty after Truncate on zero-value list")
	}
}

// TestErrorIdentity verifies the exported sentinel errors can be matched
// with errors.Is.
func TestErrorIdentity(t *testing.T) {
	var ns Sll[TestDemo]
	_, err := ns.Pop()
	if !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected errors.Is(err, ErrEmptySll), got %v", err)
	}
	_, err = ns.Peek()
	if !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected errors.Is(err, ErrEmptySll), got %v", err)
	}
	if err := ns.Delete(&SllElement[TestDemo]{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected errors.Is(err, ErrNotFound), got %v", err)
	}
}

// TestSingleElement exercises every operation on a one-element list.
func TestSingleElement(t *testing.T) {
	var ns Sll[TestDemo]
	ns.Push(&TestDemo{S: "only"})
	checkInvariants(t, &ns)

	if got, want := collect(t, &ns), []string{"only"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
	if v, err := ns.Peek(); err != nil || v.S != "only" {
		t.Errorf("Expected (only, nil) from Peek, got (%v, %v)", v, err)
	}
	el, pos := ns.Search(&TestDemo{S: "only"})
	if el == nil || pos != 0 {
		t.Errorf("Expected (el, 0) from Search, got (%v, %d)", el, pos)
	}

	// Delete the only element via Search+Delete; head and tail must reset.
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error from Delete: %v", err)
	}
	checkInvariants(t, &ns)
	if !ns.IsEmpty() {
		t.Errorf("Expected empty list after deleting only element")
	}

	// Rebuild and delete via Pop instead.
	ns.InsertAfterTail(&TestDemo{S: "again"})
	checkInvariants(t, &ns)
	v, err := ns.Pop()
	if err != nil || v.S != "again" {
		t.Errorf("Expected (again, nil) from Pop, got (%v, %v)", v, err)
	}
	checkInvariants(t, &ns)
}

// TestDuplicates verifies that duplicate values are permitted and that
// Search finds the first occurrence from the head.
func TestDuplicates(t *testing.T) {
	ns := buildList("a", "b", "a", "a")
	checkInvariants(t, ns)

	el, pos := ns.Search(&TestDemo{S: "a"})
	if el == nil || pos != 0 {
		t.Errorf("Expected first occurrence at pos 0, got (%v, %d)", el, pos)
	}

	// Deleting the found element removes only that one occurrence.
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error from Delete: %v", err)
	}
	checkInvariants(t, ns)
	if got, want := collect(t, ns), []string{"b", "a", "a"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
	if got := ns.Length(); got != 3 {
		t.Errorf("Expected length 3 after deleting one duplicate, got %d", got)
	}
}

// TestDeletePositions covers Delete at the head, middle, and tail of a
// multi-element list, plus deleting a foreign node.
func TestDeletePositions(t *testing.T) {
	// Delete tail of a multi-element list: tail must move to the predecessor.
	ns := buildList("a", "b", "c", "d")
	el, pos := ns.Search(&TestDemo{S: "d"})
	if el == nil || pos != 3 {
		t.Fatalf("Expected (el, 3) from Search, got (%v, %d)", el, pos)
	}
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error from Delete: %v", err)
	}
	checkInvariants(t, ns)
	if got, want := collect(t, ns), []string{"a", "b", "c"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
	// Appending must still work off the new tail.
	ns.InsertAfterTail(&TestDemo{S: "e"})
	checkInvariants(t, ns)
	if got, want := collect(t, ns), []string{"a", "b", "c", "e"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}

	// Delete head of a multi-element list.
	el, _ = ns.Search(&TestDemo{S: "a"})
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error from Delete: %v", err)
	}
	checkInvariants(t, ns)
	if got, want := collect(t, ns), []string{"b", "c", "e"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}

	// Delete middle.
	el, _ = ns.Search(&TestDemo{S: "c"})
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error from Delete: %v", err)
	}
	checkInvariants(t, ns)
	if got, want := collect(t, ns), []string{"b", "e"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}

	// Deleting an already-deleted (unlinked) node must fail with ErrNotFound
	// and leave the list unchanged.
	if err := ns.Delete(el); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound re-deleting unlinked node, got %v", err)
	}
	checkInvariants(t, ns)
	if got := ns.Length(); got != 2 {
		t.Errorf("Expected length 2 unchanged, got %d", got)
	}

	// A node from a different list must not be deletable.
	other := buildList("x")
	fel, _ := other.Search(&TestDemo{S: "x"})
	if err := ns.Delete(fel); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound deleting foreign node, got %v", err)
	}
	checkInvariants(t, ns)
}

// TestCurrentIterator covers the Current constructor: an iterator that
// starts partway through the list at the position reported by Search.
func TestCurrentIterator(t *testing.T) {
	ns := buildList("a", "b", "c", "d")

	el, pos := ns.Search(&TestDemo{S: "c"})
	if el == nil {
		t.Fatalf("Expected to find element c")
	}
	it := ns.Current(el, pos)
	if p := it.Pos(); p != 2 {
		t.Errorf("Expected Pos() 2 at start, got %d", p)
	}
	var got []string
	for ; !it.Done(); it.Next() {
		got = append(got, it.Value().S)
	}
	if want := []string{"c", "d"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
	// Next on an exhausted iterator is a no-op and Value is nil.
	it.Next()
	if !it.Done() {
		t.Errorf("Expected Done() after walking past the end")
	}
	if v := it.Value(); v != nil {
		t.Errorf("Expected Value() nil after Done, got %v", v)
	}

	// Current positioned at a nil element is immediately done.
	it2 := ns.Current(nil, 0)
	if !it2.Done() {
		t.Errorf("Expected Done() for iterator on nil element")
	}
}

// TestFrontIteratorFull walks the whole list with the old-style iterator and
// checks positions and values.
func TestFrontIteratorFull(t *testing.T) {
	ns := buildList("a", "b", "c")
	expected := []string{"a", "b", "c"}
	i := 0
	for it := ns.Front(); !it.Done(); it.Next() {
		if it.Pos() != i {
			t.Errorf("Expected Pos() %d, got %d", i, it.Pos())
		}
		if v := it.Value(); v == nil || v.S != expected[i] {
			t.Errorf("Expected %s got %+v at pos %d", expected[i], v, i)
		}
		i++
	}
	if i != len(expected) {
		t.Errorf("Expected %d iterations, got %d", len(expected), i)
	}
}

// TestIteratePtrEmptyAndBreak covers IteratePtr on an empty list and early
// exit (the !yield branch).
func TestIteratePtrEmptyAndBreak(t *testing.T) {
	var empty Sll[TestDemo]
	n := 0
	for range empty.IteratePtr() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected 0 iterations over empty list, got %d", n)
	}

	ns := buildList("a", "b", "c")
	count := 0
	for range ns.IteratePtr() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected early exit after 1 iteration, got %d", count)
	}

	// Mutation through the yielded pointer is visible in the list.
	for _, v := range ns.IteratePtr() {
		v.S = v.S + "!"
	}
	if got, want := collect(t, ns), []string{"a!", "b!", "c!"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
}

// TestTruncate verifies that Truncate empties the list and that the list is
// fully reusable afterwards.
func TestTruncate(t *testing.T) {
	ns := buildList("a", "b", "c")
	ns.Truncate()
	checkInvariants(t, ns)
	if !ns.IsEmpty() {
		t.Errorf("Expected empty list after Truncate")
	}
	if _, err := ns.Pop(); err != ErrEmptySll {
		t.Errorf("Expected ErrEmptySll after Truncate, got %v", err)
	}
	ns.InsertBeforeHead(&TestDemo{S: "x"})
	ns.InsertAfterTail(&TestDemo{S: "y"})
	checkInvariants(t, ns)
	if got, want := collect(t, ns), []string{"x", "y"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
}

// TestMixedOpsRandom is a randomized property test with a fixed seed.  It
// cross-checks the list against a plain slice model over hundreds of mixed
// operations, verifying contents, length, and structural invariants after
// every operation.
func TestMixedOpsRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic
	var ns Sll[TestDemo]
	var model []string

	check := func(op string) {
		t.Helper()
		checkInvariants(t, &ns)
		got := toSlice(&ns)
		if !equalStrings(got, model) {
			t.Fatalf("after %s: expected %v got %v", op, model, got)
		}
		if ns.Length() != len(model) {
			t.Fatalf("after %s: expected Length %d got %d", op, len(model), ns.Length())
		}
		if (ns.Length() == 0) != ns.IsEmpty() {
			t.Fatalf("after %s: IsEmpty disagrees with Length %d", op, ns.Length())
		}
	}

	mk := func(s string) *TestDemo { return &TestDemo{S: s} }

	for i := 0; i < 2000; i++ {
		switch rng.Intn(8) {
		case 0: // Push (insert before head)
			v := string(rune('a' + rng.Intn(6)))
			ns.Push(mk(v))
			model = append([]string{v}, model...)
			check("Push")
		case 1: // InsertAfterTail
			v := string(rune('a' + rng.Intn(6)))
			ns.InsertAfterTail(mk(v))
			model = append(model, v)
			check("InsertAfterTail")
		case 2: // Pop
			v, err := ns.Pop()
			if len(model) == 0 {
				if err != ErrEmptySll || v != nil {
					t.Fatalf("Pop on empty: expected (nil, ErrEmptySll), got (%v, %v)", v, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Pop: unexpected error %v", err)
				}
				if v.S != model[0] {
					t.Fatalf("Pop: expected %s got %s", model[0], v.S)
				}
				model = model[1:]
			}
			check("Pop")
		case 3: // Peek
			v, err := ns.Peek()
			if len(model) == 0 {
				if err != ErrEmptySll {
					t.Fatalf("Peek on empty: expected ErrEmptySll, got %v", err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("Peek: expected (%s, nil), got (%v, %v)", model[0], v, err)
				}
			}
			check("Peek")
		case 4: // Search for a possibly-present value; verify pos and value
			v := string(rune('a' + rng.Intn(8)))
			el, pos := ns.Search(mk(v))
			wantPos := -1
			for j, m := range model {
				if m == v {
					wantPos = j
					break
				}
			}
			if pos != wantPos {
				t.Fatalf("Search %q: expected pos %d got %d (model %v)", v, wantPos, pos, model)
			}
			if wantPos == -1 && el != nil {
				t.Fatalf("Search %q: expected nil element, got %v", v, el)
			}
			if wantPos >= 0 && (el == nil || el.data.S != v) {
				t.Fatalf("Search %q: bad element %v", v, el)
			}
		case 5: // Delete via Search of the value at a random position; Search
			// finds the first occurrence, so remove that from the model.
			if len(model) > 0 {
				j := rng.Intn(len(model))
				el, pos := ns.Search(mk(model[j]))
				if el == nil {
					t.Fatalf("Delete setup: could not find %q in %v", model[j], model)
				}
				if err := ns.Delete(el); err != nil {
					t.Fatalf("Delete: unexpected error %v", err)
				}
				model = append(model[:pos], model[pos+1:]...)
				check("Delete")
			}
		case 6: // Truncate
			ns.Truncate()
			model = model[:0]
			check("Truncate")
		case 7: // Full iteration matches model, both iterator styles
			k := 0
			for idx, v := range ns.IterateOver() {
				if idx != k || v.S != model[k] {
					t.Fatalf("IterateOver: at %d expected (%d, %s) got (%d, %s)", k, k, model[k], idx, v.S)
				}
				k++
			}
			if k != len(model) {
				t.Fatalf("IterateOver: expected %d values, got %d", len(model), k)
			}
			k = 0
			for it := ns.Front(); !it.Done(); it.Next() {
				if it.Pos() != k || it.Value().S != model[k] {
					t.Fatalf("Front iter: at %d expected (%d, %s) got (%d, %s)", k, k, model[k], it.Pos(), it.Value().S)
				}
				k++
			}
			if k != len(model) {
				t.Fatalf("Front iter: expected %d values, got %d", len(model), k)
			}
		}
	}
}
