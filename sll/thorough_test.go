package sll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: structural invariants, single-element and duplicate
// edge cases, iterator edges, Dump, and a fixed-seed randomized property
// test cross-checked against a slice reference model.  Benchmarks at the
// bottom.

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// checkInvariants walks the list and verifies that the internal structure
// is consistent: node count equals Length(), head/tail are nil exactly
// when the list is empty, and tail is the last node reachable from head.
func checkInvariants(t *testing.T, ns *Sll[TestSllItem], context string) {
	t.Helper()
	n := 0
	var last *SllElement[TestSllItem]
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

// TestSingleElementList exercises every operation on a one-element list.
func TestSingleElementList(t *testing.T) {
	list := newTestSll()
	list.InsertAfterTail(TestSllItem{S: "solo"})

	if v, err := list.Peek(); err != nil || v.S != "solo" {
		t.Errorf("Peek = (%v, %v)", v, err)
	}
	if el, pos := list.Search(TestSllItem{S: "solo"}); el == nil || pos != 0 {
		t.Errorf("Search = (%v, %d)", el, pos)
	}

	// Reverse of a single element is a no-op.
	list.Reverse()
	if got := valuesOf(list); len(got) != 1 || got[0] != "solo" {
		t.Errorf("After single reverse got %v", got)
	}
	checkInvariants(t, list, "after single reverse")

	// Pop it and confirm the drained behavior.
	if v, err := list.Pop(); err != nil || v.S != "solo" {
		t.Errorf("Pop = (%v, %v)", v, err)
	}
	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after popping the only element.")
	}
	checkInvariants(t, list, "after popping the only element")
}

// TestDuplicates verifies that duplicates coexist, that Search finds the
// first, and that Delete removes one at a time.
func TestDuplicates(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"x", "y", "x", "z", "x"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	if _, pos := list.Search(TestSllItem{S: "x"}); pos != 0 {
		t.Errorf("Search(x) pos = %d, expected 0", pos)
	}

	if err := list.Delete(TestSllItem{S: "x"}); err != nil {
		t.Fatalf("Delete(x): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[y x z x]"; got != want {
		t.Errorf("After delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after duplicate delete")
}

// TestDeleteFoundEdgeCases exercises the delete paths: head, tail,
// middle, not-found, and the special error cases.
func TestDeleteFoundEdgeCases(t *testing.T) {
	list := newTestSll()

	// Empty list.
	if err := list.DeleteFound(nil); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from DeleteFound on empty list, got %v", err)
	}

	for _, s := range []string{"a", "b", "c", "d"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	// Head.
	el, _ := list.Search(TestSllItem{S: "a"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(head): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b c d]"; got != want {
		t.Errorf("After head delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after head delete")

	// Tail.
	el, _ = list.Search(TestSllItem{S: "d"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(tail): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b c]"; got != want {
		t.Errorf("After tail delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after tail delete")

	// Middle.
	list.InsertAfterTail(TestSllItem{S: "e"})
	el, _ = list.Search(TestSllItem{S: "c"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(middle): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b e]"; got != want {
		t.Errorf("After middle delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after middle delete")

	// A stale element whose data no longer matches anything.
	if err := list.DeleteFound(el); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from DeleteFound of stale element, got %v", err)
	}
}

// TestCursorIteratorEdgeCases covers the cursor iterator's edges:
// Value/Next on an exhausted iterator, and starting positions.
func TestCursorIteratorEdgeCases(t *testing.T) {
	// Empty list: Front is immediately Done; Next is a no-op.
	empty := newTestSll()
	it := empty.Front()
	if !it.Done() {
		t.Errorf("Expected Front on empty list to be Done.")
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value on empty list iterator.")
	}
	it.Next() // must not panic
	if !it.Done() {
		t.Errorf("Expected Done to hold after Next on exhausted iterator.")
	}

	list := newTestSll()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	// Exhaust the iterator, then keep calling Next.
	it = list.Front()
	steps := 0
	for !it.Done() {
		it.Next()
		steps++
	}
	if steps != 3 {
		t.Errorf("Expected 3 steps to exhaust, got %d", steps)
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value after exhaustion.")
	}
	it.Next() // no-op, not a panic
	if !it.Done() {
		t.Errorf("Expected Done to hold after extra Next.")
	}
	if it.Pos() != 3 {
		t.Errorf("Expected Pos 3 after exhaustion, got %d", it.Pos())
	}

	// Current with a nil element starts done.
	if it := list.Current(nil, 0); !it.Done() {
		t.Errorf("Expected Current(nil, 0) to be Done immediately.")
	}
}

// TestDump verifies the debugging output.
func TestDump(t *testing.T) {
	list := newTestSll()
	var buf bytes.Buffer
	list.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty list, got %q", buf.String())
	}

	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	buf.Reset()
	list.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines from Dump, got %d: %q", len(lines), out)
	}
	for i, want := range []string{"0: {S:a}", "1: {S:b}", "2: {S:c}"} {
		if lines[i] != want {
			t.Errorf("Dump line %d: expected %q, got %q", i, want, lines[i])
		}
	}
}

// TestTruncateReuse verifies that the list is fully reusable after a
// truncate, including the insert-at-tail path.
func TestTruncateReuse(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	list.Truncate()

	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after Truncate.")
	}
	checkInvariants(t, list, "after Truncate")

	// Reusable after the drain, with both insertion paths.
	list.Push(TestSllItem{S: "p"})
	list.InsertAfterTail(TestSllItem{S: "t"})
	if got, want := fmt.Sprint(valuesOf(list)), "[p t]"; got != want {
		t.Errorf("After refill got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after refill")

	// Truncating an already-empty list is fine.
	list.Truncate()
	list.Truncate()
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after double Truncate.")
	}
}

// TestModelRandomized runs thousands of mixed operations against a plain
// slice reference model with a fixed seed, cross-checking after every
// step.
func TestModelRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 7))
	const ops = 4000
	const keySpace = 40 // small space forces duplicates

	list := newTestSll()
	var model []string // head at index 0

	check := func(step int) {
		t.Helper()
		if list.Length() != len(model) {
			t.Fatalf("step %d: length %d, model has %d", step, list.Length(), len(model))
		}
		if got, want := fmt.Sprint(valuesOf(list)), fmt.Sprint(model); got != want {
			t.Fatalf("step %d: contents %s, model %s", step, got, want)
		}
		checkInvariants(t, list, fmt.Sprintf("step %d", step))
	}

	for step := range ops {
		s := fmt.Sprintf("%02d", rng.IntN(keySpace))
		switch rng.IntN(6) {
		case 0: // Push (head)
			list.Push(TestSllItem{S: s})
			model = append([]string{s}, model...)
		case 1: // InsertAfterTail
			list.InsertAfterTail(TestSllItem{S: s})
			model = append(model, s)
		case 2: // Pop
			v, err := list.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptySll) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("step %d: Pop = (%v, %v), model head %s", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // Delete by value (first match)
			err := list.Delete(TestSllItem{S: s})
			idx := -1
			for i, m := range model {
				if m == s {
					idx = i
					break
				}
			}
			if idx < 0 {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("step %d: Delete(%s) = %v, model says absent", step, s, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Delete(%s): %v", step, s, err)
				}
				model = append(model[:idx], model[idx+1:]...)
			}
		case 4: // Search position
			_, pos := list.Search(TestSllItem{S: s})
			want := -1
			for i, m := range model {
				if m == s {
					want = i
					break
				}
			}
			if pos != want {
				t.Fatalf("step %d: Search(%s) pos %d, model says %d", step, s, pos, want)
			}
		case 5: // Reverse
			list.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		}
		if step%50 == 0 {
			check(step)
		}
	}
	check(ops)
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkListSize = 4096

func BenchmarkInsertBeforeHead(b *testing.B) {
	list := NewSll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.InsertBeforeHead(i)
	}
}

func BenchmarkInsertAfterTail(b *testing.B) {
	list := NewSll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.InsertAfterTail(i)
	}
}

func BenchmarkPop(b *testing.B) {
	list := NewSll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if list.IsEmpty() {
			list.Truncate()
		}
		_, _ = list.Pop()
	}
}

func BenchmarkSearch(b *testing.B) {
	list := NewSll[int]()
	for i := range benchmarkListSize {
		list.InsertAfterTail(i)
	}
	find := benchmarkListSize / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Search(find)
	}
}

func BenchmarkIterateOver(b *testing.B) {
	list := NewSll[int]()
	for i := range benchmarkListSize {
		list.InsertAfterTail(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range list.IterateOver() {
		}
	}
}
