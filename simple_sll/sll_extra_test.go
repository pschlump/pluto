package sll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"testing"
)

// buildList returns a list with the given values appended at the tail,
// in order: l[0] is at the head.
func buildList(vals ...string) *Sll[TestDemo] {
	ns := NewSll[TestDemo]()
	for _, v := range vals {
		ns.InsertAfterTail(&TestDemo{S: v})
	}
	return ns
}

func collect(t *testing.T, ns *Sll[TestDemo]) []string {
	t.Helper()
	var got []string
	for _, v := range ns.IterateOver() {
		got = append(got, v.S)
	}
	return got
}

func equalStrings(a, b []string) bool {
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

// Regression test: Pop of the last element must reset tail, otherwise a
// subsequent InsertAfterTail appends to a detached node and corrupts the list.
func TestPopResetsTail(t *testing.T) {
	var ns Sll[TestDemo]
	ns.Push(&TestDemo{S: "a"})
	if _, err := ns.Pop(); err != nil {
		t.Fatalf("Unexpected error on pop: %v", err)
	}
	if !ns.IsEmpty() {
		t.Errorf("Expected empty list after popping last element")
	}
	if ns.tail != nil {
		t.Errorf("Expected tail to be nil after popping last element")
	}
	ns.InsertAfterTail(&TestDemo{S: "b"})
	ns.InsertAfterTail(&TestDemo{S: "c"})
	if got, want := ns.Length(), 2; got != want {
		t.Errorf("Expected length of %d got %d", want, got)
	}
	if got, want := collect(t, &ns), []string{"b", "c"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
}

func TestInsertAfterTailOrder(t *testing.T) {
	ns := buildList("a", "b", "c")
	if got, want := ns.Length(), 3; got != want {
		t.Errorf("Expected length of %d got %d", want, got)
	}
	if got, want := collect(t, ns), []string{"a", "b", "c"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
}

func TestSearchPeek(t *testing.T) {
	var ns Sll[TestDemo]

	// Search/Peek on an empty list.
	if el, pos := ns.Search(&TestDemo{S: "x"}); el != nil || pos != -1 {
		t.Errorf("Expected (nil, -1) searching empty list, got (%v, %d)", el, pos)
	}
	if _, err := ns.Peek(); err == nil {
		t.Errorf("Expected error peeking empty list")
	}

	ns = *buildList("a", "b", "c")

	if v, err := ns.Peek(); err != nil {
		t.Errorf("Unexpected error on peek: %v", err)
	} else if v.S != "a" {
		t.Errorf("Expected %s got %s", "a", v.S)
	}

	el, pos := ns.Search(&TestDemo{S: "b"})
	if el == nil || pos != 1 {
		t.Errorf("Expected (el, 1) got (%v, %d)", el, pos)
	}
	if el, pos := ns.Search(&TestDemo{S: "missing"}); el != nil || pos != -1 {
		t.Errorf("Expected (nil, -1) got (%v, %d)", el, pos)
	}
}

func TestDelete(t *testing.T) {
	// Delete from an empty list.
	var ns Sll[TestDemo]
	if err := ns.Delete(&SllElement[TestDemo]{}); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound deleting from empty list, got %v", err)
	}

	// Delete head.
	ns = *buildList("a", "b", "c")
	el, _ := ns.Search(&TestDemo{S: "a"})
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got, want := collect(t, &ns), []string{"b", "c"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}

	// Delete middle.
	el, _ = ns.Search(&TestDemo{S: "c"})
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got, want := collect(t, &ns), []string{"b"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}

	// Delete tail; tail pointer must be updated so InsertAfterTail still works.
	el, _ = ns.Search(&TestDemo{S: "b"})
	if err := ns.Delete(el); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !ns.IsEmpty() {
		t.Errorf("Expected empty list, got length %d", ns.Length())
	}
	ns.InsertAfterTail(&TestDemo{S: "z"})
	if got, want := collect(t, &ns), []string{"z"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}

	// Delete a node that is not in the list.
	if err := ns.Delete(&SllElement[TestDemo]{}); err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestIterateOver(t *testing.T) {
	var empty Sll[TestDemo]
	n := 0
	for range empty.IterateOver() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected 0 iterations over empty list, got %d", n)
	}

	ns := buildList("a", "b", "c")
	expected := []string{"a", "b", "c"}
	for i, v := range ns.IterateOver() {
		if i < 0 || i >= len(expected) {
			t.Fatalf("Unexpected index %d", i)
		}
		if v.S != expected[i] {
			t.Errorf("Expected %s got %s at %d", expected[i], v.S, i)
		}
	}

	// Early exit must stop the iteration.
	count := 0
	for range ns.IterateOver() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected early exit after 1 iteration, got %d", count)
	}
}

func TestIteratePtr(t *testing.T) {
	ns := buildList("a", "b", "c")
	expected := []string{"a", "b", "c"}
	for i, v := range ns.IteratePtr() {
		if i < 0 || i >= len(expected) {
			t.Fatalf("Unexpected index %d", i)
		}
		if v == nil || v.S != expected[i] {
			t.Errorf("Expected %s got %+v at %d", expected[i], v, i)
		}
	}
}

// Mutating the value through the pointer returned by IteratePtr/Search
// must be visible in the list.
func TestMutateThroughPtr(t *testing.T) {
	ns := buildList("a")
	el, _ := ns.Search(&TestDemo{S: "a"})
	el.data.S = "changed"
	if got, want := collect(t, ns), []string{"changed"}; !equalStrings(got, want) {
		t.Errorf("Expected %v got %v", want, got)
	}
}

func BenchmarkPush(b *testing.B) {
	var ns Sll[TestDemo]
	v := &TestDemo{S: "x"}
	for b.Loop() {
		ns.Push(v)
	}
}

func BenchmarkPushPop(b *testing.B) {
	var ns Sll[TestDemo]
	v := &TestDemo{S: "x"}
	for b.Loop() {
		ns.Push(v)
		if _, err := ns.Pop(); err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkInsertAfterTail(b *testing.B) {
	var ns Sll[TestDemo]
	v := &TestDemo{S: "x"}
	for b.Loop() {
		ns.InsertAfterTail(v)
	}
}

func BenchmarkSearch(b *testing.B) {
	var ns Sll[TestDemo]
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		ns.InsertAfterTail(&TestDemo{S: s})
	}
	needle := &TestDemo{S: "h"} // worst case: last element
	b.ResetTimer()
	for b.Loop() {
		if el, _ := ns.Search(needle); el == nil {
			b.Fatal("Expected to find element")
		}
	}
}

func BenchmarkIterateOver(b *testing.B) {
	var ns Sll[TestDemo]
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		ns.InsertAfterTail(&TestDemo{S: s})
	}
	b.ResetTimer()
	for b.Loop() {
		for range ns.IterateOver() {
		}
	}
}
