package dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: element accessors, delete branches, iterator edges,
// Dump, and a fixed-seed randomized property test cross-checked against a
// slice reference model.  Benchmarks at the bottom.

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// checkList verifies the structural invariants: length matches a full
// walk, the head's prev and the tail's next are nil, and every link is
// bidirectional.
func checkList(t *testing.T, list *Dll[TestDllItem], where string) {
	t.Helper()

	// Forward walk reaches the tail and counts length nodes.
	n := 0
	var last *DllElement[TestDllItem]
	for p := list.head; p != nil; p = p.next {
		if p.prev != last {
			t.Fatalf("%s: broken prev link at node %d", where, n)
		}
		last = p
		n++
	}
	if n != list.length {
		t.Fatalf("%s: forward walk found %d nodes, length is %d", where, n, list.length)
	}
	if list.tail != last {
		t.Fatalf("%s: tail pointer does not match the last node of the forward walk", where)
	}
	if list.head != nil && list.head.prev != nil {
		t.Fatalf("%s: head.prev is non-nil", where)
	}
	if list.tail != nil && list.tail.next != nil {
		t.Fatalf("%s: tail.next is non-nil", where)
	}

	// Backward walk reaches the head.
	n = 0
	var first *DllElement[TestDllItem]
	for p := list.tail; p != nil; p = p.prev {
		first = p
		n++
	}
	if list.head != first {
		t.Fatalf("%s: backward walk does not reach the head", where)
	}
	if list.length == 0 && (list.head != nil || list.tail != nil) {
		t.Fatalf("%s: empty list has non-nil head/tail", where)
	}
}

// valuesOf returns the current contents of the list, head to tail.
func valuesOf(list *Dll[TestDllItem]) []string {
	var got []string
	for _, v := range list.All() {
		got = append(got, v.S)
	}
	return got
}

func TestElementAccessors(t *testing.T) {
	el := &DllElement[TestDllItem]{}
	el.SetData(TestDllItem{S: "42"})
	if d := el.GetData(); d.S != "42" {
		t.Errorf("Expected GetData to return 42, got %+v", d)
	}

	list := newTestDll()
	list.AppendAtTail(TestDllItem{S: "07"})
	found, pos := list.Search(TestDllItem{S: "07"})
	if found == nil || pos != 0 {
		t.Fatalf("Expected to find the appended item.")
	}
	found.SetData(TestDllItem{S: "08"})
	if v, err := list.Peek(); err != nil || v.S != "08" {
		t.Errorf("SetData on a found element should change the stored value, got (%v, %v)", v, err)
	}
}

// TestDeleteFoundAllBranches exercises every branch of DeleteFound:
// single element, head, tail, middle, and a nil element.
func TestDeleteFoundAllBranches(t *testing.T) {
	// Nil element.
	list := newTestDll()
	if err := list.DeleteFound(nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from DeleteFound(nil), got %v", err)
	}

	// Single element.
	list.AppendAtTail(TestDllItem{S: "only"})
	el, _ := list.Search(TestDllItem{S: "only"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(single): %v", err)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting the single element.")
	}
	checkList(t, list, "after single delete")

	// Head and tail of a longer list.
	for _, s := range []string{"a", "b", "c", "d"} {
		list.AppendAtTail(TestDllItem{S: s})
	}
	el, _ = list.Search(TestDllItem{S: "a"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(head): %v", err)
	}
	el, _ = list.Search(TestDllItem{S: "d"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(tail): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b c]"; got != want {
		t.Errorf("After head+tail deletes got %s, expected %s", got, want)
	}
	checkList(t, list, "after head+tail deletes")

	// Middle of a longer list.
	list.AppendAtTail(TestDllItem{S: "x"})
	list.AppendAtTail(TestDllItem{S: "y"})
	el, _ = list.Search(TestDllItem{S: "c"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(middle): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b x y]"; got != want {
		t.Errorf("After middle delete got %s, expected %s", got, want)
	}
	checkList(t, list, "after middle delete")

	// Elements from Index/IndexFromTail work with DeleteFound.
	el, err := list.Index(1)
	if err != nil {
		t.Fatalf("Index(1): %v", err)
	}
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(Index(1)): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b y]"; got != want {
		t.Errorf("After Index-based delete got %s, expected %s", got, want)
	}
}

// TestSingleElementEdgeCases covers every operation on a one-element list.
func TestSingleElementEdgeCases(t *testing.T) {
	list := newTestDll()
	list.AppendAtTail(TestDllItem{S: "solo"})

	if v, err := list.Peek(); err != nil || v.S != "solo" {
		t.Errorf("Peek = (%v, %v)", v, err)
	}
	if v, err := list.PeekTail(); err != nil || v.S != "solo" {
		t.Errorf("PeekTail = (%v, %v)", v, err)
	}
	if el, pos := list.Search(TestDllItem{S: "solo"}); el == nil || pos != 0 {
		t.Errorf("Search = (%v, %d)", el, pos)
	}
	if el, err := list.Index(0); err != nil || el.GetData().S != "solo" {
		t.Errorf("Index(0) = (%v, %v)", el, err)
	}
	if el, err := list.IndexFromTail(0); err != nil || el.GetData().S != "solo" {
		t.Errorf("IndexFromTail(0) = (%v, %v)", el, err)
	}

	// Reverse of a single element is a no-op.
	list.Reverse()
	if got := valuesOf(list); !reflect.DeepEqual(got, []string{"solo"}) {
		t.Errorf("After reverse got %v", got)
	}
	checkList(t, list, "after single reverse")

	// Pop it and confirm the drained behavior.
	if v, err := list.Pop(); err != nil || v.S != "solo" {
		t.Errorf("Pop = (%v, %v)", v, err)
	}
	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after popping the only element.")
	}
	checkList(t, list, "after popping the only element")
}

// TestDuplicateValues verifies that duplicates coexist and that Search
// finds the first while ReverseSearch finds the last.
func TestDuplicateValues(t *testing.T) {
	list := newTestDll()
	for _, s := range []string{"x", "y", "x", "z", "x"} {
		list.AppendAtTail(TestDllItem{S: s})
	}

	if _, pos := list.Search(TestDllItem{S: "x"}); pos != 0 {
		t.Errorf("Search(x) pos = %d, expected 0", pos)
	}
	if _, pos := list.ReverseSearch(TestDllItem{S: "x"}); pos != 4 {
		t.Errorf("ReverseSearch(x) pos = %d, expected 4", pos)
	}

	// Deleting by value removes one at a time.
	if err := list.Delete(TestDllItem{S: "x"}); err != nil {
		t.Fatalf("Delete(x): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[y x z x]"; got != want {
		t.Errorf("After delete got %s, expected %s", got, want)
	}
	checkList(t, list, "after duplicate delete")
}

// TestIteratorEdgeCases covers the legacy iterator's edges: Value/Next/
// Prev on an exhausted iterator, and Prev off the head.
func TestIteratorEdgeCases(t *testing.T) {
	// Empty list: Front is immediately Done; Next/Prev are no-ops.
	empty := newTestDll()
	it := empty.Front()
	if !it.Done() {
		t.Errorf("Expected Front on empty list to be Done.")
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value on empty list iterator.")
	}
	it.Next()
	it.Prev()
	if !it.Done() {
		t.Errorf("Expected Done to hold after Next/Prev on exhausted iterator.")
	}

	list := newTestDll()
	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(TestDllItem{S: s})
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

	// Prev off the head ends the iteration with a negative Pos.
	it = list.Front()
	it.Prev()
	if !it.Done() {
		t.Errorf("Expected Done after Prev off the head.")
	}
	if it.Pos() != -1 {
		t.Errorf("Expected Pos -1 after Prev off the head, got %d", it.Pos())
	}
}

// TestSearchWalkOnEmpty verifies every read on an empty list.
func TestSearchWalkOnEmpty(t *testing.T) {
	list := newTestDll()

	if el, pos := list.Search(TestDllItem{S: "a"}); el != nil || pos != -1 {
		t.Errorf("Search on empty: (%v, %d)", el, pos)
	}
	if el, pos := list.ReverseSearch(TestDllItem{S: "a"}); el != nil || pos != -1 {
		t.Errorf("ReverseSearch on empty: (%v, %d)", el, pos)
	}
	if el, pos := list.Walk(func(pos int, data TestDllItem) bool { return true }); el != nil || pos != -1 {
		t.Errorf("Walk on empty: (%v, %d)", el, pos)
	}
	if el, pos := list.ReverseWalk(func(pos int, data TestDllItem) bool { return true }); el != nil || pos != -1 {
		t.Errorf("ReverseWalk on empty: (%v, %d)", el, pos)
	}
	if _, err := list.Index(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Index on empty: %v", err)
	}
	if _, err := list.IndexFromTail(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("IndexFromTail on empty: %v", err)
	}
}

// TestDump verifies the debugging output.
func TestDump(t *testing.T) {
	list := newTestDll()
	var buf bytes.Buffer
	list.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty list, got %q", buf.String())
	}

	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(TestDllItem{S: s})
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

// TestModelRandomized runs thousands of mixed operations against a plain
// slice reference model with a fixed seed, cross-checking after every
// step.
func TestModelRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 42))
	const ops = 4000
	const keySpace = 40 // small space forces duplicates

	list := newTestDll()
	var model []string // head at index 0

	check := func(step int) {
		t.Helper()
		if list.Length() != len(model) {
			t.Fatalf("step %d: length %d, model has %d", step, list.Length(), len(model))
		}
		if got, want := fmt.Sprint(valuesOf(list)), fmt.Sprint(model); got != want {
			t.Fatalf("step %d: contents %s, model %s", step, got, want)
		}
		checkList(t, list, fmt.Sprintf("step %d", step))
	}

	for step := range ops {
		s := fmt.Sprintf("%02d", rng.IntN(keySpace))
		switch rng.IntN(8) {
		case 0: // Push (head)
			list.Push(TestDllItem{S: s})
			model = append([]string{s}, model...)
		case 1: // AppendAtTail
			list.AppendAtTail(TestDllItem{S: s})
			model = append(model, s)
		case 2: // Pop
			v, err := list.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyDll) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("step %d: Pop = (%v, %v), model head %s", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // PopTail
			v, err := list.PopTail()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyDll) {
					t.Fatalf("step %d: PopTail on empty returned %v", step, err)
				}
			} else {
				if err != nil || v.S != model[len(model)-1] {
					t.Fatalf("step %d: PopTail = (%v, %v), model tail %s", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case 4: // Delete by value (first match)
			err := list.Delete(TestDllItem{S: s})
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
		case 5: // Search position
			_, pos := list.Search(TestDllItem{S: s})
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
		case 6: // Index round trip
			if len(model) > 0 {
				sub := rng.IntN(len(model))
				el, err := list.Index(sub)
				if err != nil || el.GetData().S != model[sub] {
					t.Fatalf("step %d: Index(%d) = (%v, %v), model %s", step, sub, el, err, model[sub])
				}
			}
		case 7: // Trim to a random prefix
			if len(model) > 0 {
				n := rng.IntN(len(model) + 2)
				if err := list.Trim(n); err != nil {
					t.Fatalf("step %d: Trim(%d): %v", step, n, err)
				}
				if n <= 0 {
					model = nil
				} else if n < len(model) {
					model = model[:n]
				}
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

func BenchmarkPush(b *testing.B) {
	list := NewDll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Push(i)
	}
}

func BenchmarkAppendAtTail(b *testing.B) {
	list := NewDll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.AppendAtTail(i)
	}
}

func BenchmarkPop(b *testing.B) {
	list := NewDll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if list.IsEmpty() {
			list.Truncate()
		}
		_, _ = list.Pop()
	}
}

func BenchmarkSearch(b *testing.B) {
	list := NewDll[int]()
	for i := range benchmarkListSize {
		list.AppendAtTail(i)
	}
	find := benchmarkListSize / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Search(find)
	}
}

func BenchmarkAll(b *testing.B) {
	list := NewDll[int]()
	for i := range benchmarkListSize {
		list.AppendAtTail(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range list.All() {
		}
	}
}
