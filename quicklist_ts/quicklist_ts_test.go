package quicklist_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Tests: API parity with the plain package, snapshot-iterator
// semantics (mutate-while-iterating), the Lock/Nl* compound pattern,
// MoveHeadToTail between two lists in both lock orders, and a
// concurrent hammer for -race.

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/pschlump/pluto/quicklist"
)

func build(n int) *QuickList[int] {
	q := NewQuickList(quicklist.WithSegmentFill[int](8))
	for i := 0; i < n; i++ {
		q.PushTail(i)
	}
	return q
}

func TestAPIParity(t *testing.T) {
	q := build(100)
	if q.Len() != 100 {
		t.Fatalf("Len: got %d", q.Len())
	}
	if v, ok := q.At(50); !ok || v != 50 {
		t.Fatalf("At(50): got %d,%v", v, ok)
	}
	if v, ok := q.At(-1); !ok || v != 99 {
		t.Fatalf("At(-1): got %d,%v", v, ok)
	}
	if !q.Set(0, 1000) {
		t.Fatal("Set failed")
	}
	if !q.InsertBefore(1, 999) || !q.InsertAfter(-1, 1001) {
		t.Fatal("inserts failed")
	}
	if !q.Delete(0) || !q.Delete(-1) {
		t.Fatal("deletes failed")
	}
	if n := q.DeleteRange(0, 9); n != 10 {
		t.Fatalf("DeleteRange: got %d", n)
	}
	q.Trim(10, 19)
	if q.Len() != 10 {
		t.Fatalf("after Trim: Len %d", q.Len())
	}
	// List is now [20 21 ... 29].
	if v, _ := q.PeekHead(); v != 20 {
		t.Fatalf("PeekHead: got %d", v)
	}
	if v, _ := q.PeekTail(); v != 29 {
		t.Fatalf("PeekTail: got %d", v)
	}
	if v, ok := q.PopHead(); !ok || v != 20 {
		t.Fatalf("PopHead: got %d,%v", v, ok)
	}
	if v, ok := q.PopTail(); !ok || v != 29 {
		t.Fatalf("PopTail: got %d,%v", v, ok)
	}
}

func TestSnapshotIterators(t *testing.T) {
	q := build(10)
	// Mutating from inside the loop is safe and the iterator keeps
	// walking the snapshot.
	var got []int
	for i, v := range q.All() {
		got = append(got, v)
		if i == 4 {
			q.Delete(0)
			q.PushTail(99)
		}
	}
	if len(got) != 10 {
		t.Fatalf("snapshot All yielded %d elements, want 10", len(got))
	}
	if !slices.Equal(got, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("snapshot All: got %v", got)
	}
	// Backward indexes count down from Len()-1 of the snapshot.
	var idx []int
	for i := range q.Backward() {
		idx = append(idx, i)
	}
	if !slices.IsSortedFunc(idx, func(a, b int) int { return b - a }) {
		t.Fatalf("Backward indexes not descending: %v", idx)
	}
	// Range over a clamped/negative window.
	var r []int
	for _, v := range q.Range(-3, 100) {
		r = append(r, v)
	}
	if len(r) != 3 {
		t.Fatalf("Range(-3,100): got %v", r)
	}
}

func TestLockNlCompound(t *testing.T) {
	q := build(5)
	q.Lock()
	// Atomic batch: replace every element with its square, atomically.
	for i := 0; i < q.NlLen(); i++ {
		v, _ := q.NlAt(i)
		q.NlSet(i, v*v)
	}
	q.Unlock()
	want := []int{0, 1, 4, 9, 16}
	var got []int
	for _, v := range q.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("compound: got %v, want %v", got, want)
	}
	// Nl inserts and range ops under the lock: [0,2,4,6,8] + 25 at the
	// tail, drop the first two, then drop the new tail.
	q.Lock()
	q.NlInsertAfter(-1, 25)
	q.NlDeleteRange(0, 1)
	q.NlTrim(0, -2)
	q.Unlock()
	if q.Len() != 3 {
		t.Fatalf("after Nl ops: Len %d", q.Len())
	}
	var rest []int
	for _, v := range q.All() {
		rest = append(rest, v)
	}
	if !slices.Equal(rest, []int{4, 9, 16}) {
		t.Fatalf("after Nl ops: got %v, want [4 9 16]", rest)
	}
}

func TestMoveHeadToTail(t *testing.T) {
	src := NewQuickList[int]()
	dst := NewQuickList[int]()
	src.PushTail(1)
	src.PushTail(2)
	dst.PushTail(9)
	v, ok := MoveHeadToTail(src, dst)
	if !ok || v != 1 {
		t.Fatalf("MoveHeadToTail: got %d,%v", v, ok)
	}
	// Reverse direction locks the pair in the opposite caller order;
	// pointer ordering keeps it deadlock-free.
	v, ok = MoveHeadToTail(dst, src)
	if !ok || v != 9 {
		t.Fatalf("reverse MoveHeadToTail: got %d,%v", v, ok)
	}
	// Self-rotation.
	if _, ok := MoveHeadToTail(src, src); !ok {
		t.Fatal("self-rotation failed")
	}
	if _, ok := MoveHeadToTail(NewQuickList[int](), dst); ok {
		t.Fatal("move from empty returned ok")
	}
}

func TestConcurrentHammer(t *testing.T) {
	q := NewQuickList(quicklist.WithSegmentFill[int](16))
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				switch i % 6 {
				case 0:
					q.PushTail(i)
				case 1:
					q.PushHead(i)
				case 2:
					q.PopHead()
				case 3:
					q.PopTail()
				case 4:
					q.At(i % 50)
				case 5:
					q.Len()
				}
			}
		}(g)
	}
	// Concurrent snapshot iterations.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				n := 0
				for range q.All() {
					n++
				}
				_ = n
			}
		}()
	}
	wg.Wait()
	// Drained of nothing in particular — just verify consistency: Len
	// agrees with a full walk.
	n := 0
	for range q.All() {
		n++
	}
	if n != q.Len() {
		t.Fatalf("inconsistent after hammer: walk %d, Len %d", n, q.Len())
	}
}

func TestConcurrentMoveHeadToTail(t *testing.T) {
	a := NewQuickList[int]()
	b := NewQuickList[int]()
	for i := 0; i < 100; i++ {
		a.PushTail(i)
		b.PushTail(i)
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				MoveHeadToTail(a, b)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				MoveHeadToTail(b, a)
			}
		}()
	}
	wg.Wait()
	if got := a.Len() + b.Len(); got != 200 {
		t.Fatalf("elements lost: total %d, want 200", got)
	}
}

func TestNilToleratedAndPanics(t *testing.T) {
	var q *QuickList[int]
	if q.Len() != 0 {
		t.Fatal("nil Len")
	}
	if _, ok := q.At(0); ok {
		t.Fatal("nil At")
	}
	if _, ok := q.PopHead(); ok {
		t.Fatal("nil PopHead")
	}
	if q.Set(0, 1) || q.Delete(0) {
		t.Fatal("nil write returned true")
	}
	q.Trim(0, -1)
	q.Lock()
	q.Unlock()
	for range q.All() {
		t.Fatal("nil All yielded")
	}
	if _, ok := MoveHeadToTail[int](q, q); ok {
		t.Fatal("nil MoveHeadToTail")
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from nil PushTail")
		}
		if msg := fmt.Sprint(r); msg != "quicklist_ts: PushTail called on a nil QuickList" {
			t.Fatalf("panic message: %q", msg)
		}
	}()
	q.PushTail(1)
}
