/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stable_pq

import (
	"slices"
	"testing"
)

// task is a key-with-satellite element: the comparator orders by prio
// only, so prio values identify the tie groups in which stability is
// observed through the insertion-ordered tag.
type task struct {
	prio int
	tag  string
}

func byPrio(a, b task) int {
	switch {
	case a.prio < b.prio:
		return -1
	case a.prio > b.prio:
		return 1
	}
	return 0
}

func TestBasicQueue(t *testing.T) {
	pq := NewStablePQ[int]()
	if pq.Len() != 0 || !pq.IsEmpty() {
		t.Fatalf("new queue is not empty: Len=%d IsEmpty=%v", pq.Len(), pq.IsEmpty())
	}
	if _, found := pq.Peek(); found {
		t.Fatalf("Peek on an empty queue reported found")
	}
	if _, found := pq.Pop(); found {
		t.Fatalf("Pop on an empty queue reported found")
	}

	for _, v := range []int{42, 7, 13, 7, 99, 1} {
		pq.Insert(v)
	}
	if pq.Len() != 6 {
		t.Fatalf("Len after 6 inserts = %d, want 6", pq.Len())
	}
	if v, _ := pq.Peek(); v != 1 {
		t.Fatalf("Peek = %d, want 1", v)
	}
	var got []int
	for {
		v, found := pq.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if want := []int{1, 7, 7, 13, 42, 99}; !slices.Equal(got, want) {
		t.Fatalf("Pop sequence = %v, want %v", got, want)
	}
	if !pq.IsEmpty() {
		t.Fatalf("queue not empty after draining")
	}
}

// TestStabilityWitness pins the defining property on a small
// hand-traced sequence: keys come out ascending, and equal keys come
// out in insertion order (tags a, b, c — never a later permutation).
func TestStabilityWitness(t *testing.T) {
	pq := NewStablePQFunc(byPrio)
	for _, x := range []task{
		{1, "a"}, {0, "z"}, {1, "b"}, {1, "c"}, {0, "y"}, {2, "m"},
	} {
		pq.Insert(x)
	}
	var got []string
	for !pq.IsEmpty() {
		x, _ := pq.Pop()
		got = append(got, x.tag)
	}
	if want := []string{"z", "y", "a", "b", "c", "m"}; !slices.Equal(got, want) {
		t.Fatalf("stable pop order = %v, want %v", got, want)
	}
}

// TestStabilityLongTies makes one tie group the whole queue: n elements
// with the SAME key must come out in exact insertion order, which a
// plain heap is free to shuffle.
func TestStabilityLongTies(t *testing.T) {
	const n = 500
	pq := NewStablePQFunc(byPrio)
	for i := range n {
		pq.Insert(task{prio: 5, tag: string(rune('a'+i%26)) + "-" + itoa(i)})
	}
	i := 0
	for {
		x, found := pq.Pop()
		if !found {
			break
		}
		if x.tag != string(rune('a'+i%26))+"-"+itoa(i) {
			t.Fatalf("all-equal keys: pop %d = %q, want the element inserted at %d", i, x.tag, i)
		}
		i++
	}
	if i != n {
		t.Fatalf("popped %d elements, want %d", i, n)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

func TestPeekDoesNotConsume(t *testing.T) {
	pq := NewStablePQ[string]()
	pq.Insert("m")
	pq.Insert("a")
	for range 3 {
		if v, found := pq.Peek(); !found || v != "a" {
			t.Fatalf("Peek = (%q, %v), want (a, true)", v, found)
		}
	}
	if pq.Len() != 2 {
		t.Fatalf("Len after repeated Peek = %d, want 2", pq.Len())
	}
}

func TestAllSnapshot(t *testing.T) {
	pq := NewStablePQFunc(byPrio)
	for _, x := range []task{{2, "w"}, {1, "x"}, {2, "v"}, {1, "y"}} {
		pq.Insert(x)
	}
	var got []string
	for x := range pq.All() {
		got = append(got, x.tag)
	}
	if want := []string{"x", "y", "w", "v"}; !slices.Equal(got, want) {
		t.Fatalf("All order = %v, want %v", got, want)
	}
	if pq.Len() != 4 {
		t.Fatalf("All drained the queue: Len = %d, want 4", pq.Len())
	}

	// The snapshot is taken at call time: later inserts do not appear,
	// and mutating from inside the loop is safe.
	count := 0
	for range pq.All() {
		count++
		pq.Insert(task{prio: 0, tag: "new"})
	}
	if count != 4 {
		t.Fatalf("first All yielded %d elements, want 4", count)
	}
	if pq.Len() != 8 {
		t.Fatalf("Len after inserting inside All loop = %d, want 8", pq.Len())
	}
}

func TestTruncateAndRefill(t *testing.T) {
	pq := NewStablePQ[int]()
	for _, v := range []int{3, 1, 2} {
		pq.Insert(v)
	}
	pq.Truncate()
	if pq.Len() != 0 || !pq.IsEmpty() {
		t.Fatalf("Truncate left Len=%d", pq.Len())
	}
	if _, found := pq.Pop(); found {
		t.Fatalf("Pop after Truncate reported found")
	}
	// Refill: the kept comparison function (and the still-counting
	// stamp) keep the queue correct after the reset.
	for _, v := range []int{30, 10, 20, 10} {
		pq.Insert(v)
	}
	var got []int
	for v := range pq.All() {
		got = append(got, v)
	}
	if want := []int{10, 10, 20, 30}; !slices.Equal(got, want) {
		t.Fatalf("refilled All = %v, want %v", got, want)
	}
}

// TestMaxHeapViaReversedComparator checks the reversed-comparator
// idiom: ties must STILL be FIFO (the reversal applies to the key only,
// not the stamp) — the oldest of the equal maxima comes out first.
func TestMaxHeapViaReversedComparator(t *testing.T) {
	maxPq := NewStablePQFunc(func(a, b int) int { return -byInt(a, b) })
	for _, v := range []int{5, 9, 5, 7, 9, 1} {
		maxPq.Insert(v)
	}
	var got []int
	for {
		v, found := maxPq.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if want := []int{9, 9, 7, 5, 5, 1}; !slices.Equal(got, want) {
		t.Fatalf("max-heap pop order = %v, want %v", got, want)
	}
}

func byInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func TestNilAndZeroValueTolerance(t *testing.T) {
	var nilPq *StablePQ[int]
	if nilPq.Len() != 0 || !nilPq.IsEmpty() {
		t.Fatalf("nil queue: Len=%d IsEmpty=%v", nilPq.Len(), nilPq.IsEmpty())
	}
	if nilPq.Length() != 0 {
		t.Fatalf("nil queue: Length=%d", nilPq.Length())
	}
	if _, found := nilPq.Peek(); found {
		t.Fatalf("nil queue: Peek reported found")
	}
	if _, found := nilPq.Pop(); found {
		t.Fatalf("nil queue: Pop reported found")
	}
	nilPq.Truncate() // must not panic
	nilPq.Lock()     // no-op compatibility pair
	nilPq.Unlock()
	count := 0
	for range nilPq.All() {
		count++
	}
	if count != 0 {
		t.Fatalf("nil queue: All yielded %d elements", count)
	}

	var zero StablePQ[string] // zero value: no heap, no comparator
	if zero.Len() != 0 || !zero.IsEmpty() {
		t.Fatalf("zero queue: Len=%d", zero.Len())
	}
	if _, found := zero.Peek(); found {
		t.Fatalf("zero queue: Peek reported found")
	}
	if _, found := zero.Pop(); found {
		t.Fatalf("zero queue: Pop reported found")
	}
	zero.Truncate()
	for range zero.All() {
		count++
	}
	if count != 0 {
		t.Fatalf("zero queue: All yielded elements")
	}
}

func TestPanics(t *testing.T) {
	expectPanic(t, "NewStablePQFunc(nil)", func() {
		NewStablePQFunc[int](nil)
	})

	var nilPq *StablePQ[int]
	expectPanic(t, "Insert on a nil queue", func() {
		nilPq.Insert(1)
	})

	var zero StablePQ[int]
	expectPanic(t, "Insert on a zero-value queue", func() {
		zero.Insert(1)
	})
}

func expectPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", what)
		}
	}()
	fn()
}

// TestCustomTypeStability exercises the NewStablePQFunc path with a
// struct keyed by a float priority and string sub-order ignored by the
// comparator — equal priorities must keep satellite insertion order.
func TestCustomTypeStability(t *testing.T) {
	type patient struct {
		severity float64
		name     string
	}
	pq := NewStablePQFunc(func(a, b patient) int {
		switch {
		case a.severity < b.severity:
			return -1
		case a.severity > b.severity:
			return 1
		}
		return 0
	})
	for _, p := range []patient{
		{1.0, "first-1.0"}, {3.5, "first-of-3.5"}, {2.0, "only-2.0"},
		{3.5, "second-of-3.5"}, {3.5, "third-of-3.5"},
	} {
		pq.Insert(p)
	}
	var names []string
	for p := range pq.All() {
		names = append(names, p.name)
	}
	want := []string{"first-1.0", "only-2.0", "first-of-3.5", "second-of-3.5", "third-of-3.5"}
	if !slices.Equal(names, want) {
		t.Fatalf("custom-type stable order = %v, want %v", names, want)
	}
}
