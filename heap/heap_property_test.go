// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package heap

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// checkInvariant verifies the min-heap ordering of every parent/child pair.
func checkInvariant(t *testing.T, h *Heap[myHeap], context string) {
	t.Helper()
	n := h.Len()
	for i := 0; i < n; i++ {
		for _, j := range []int{2*i + 1, 2*i + 2} {
			if j < n && (*(h.data[j])).Compare(*(h.data[i])) < 0 {
				t.Fatalf("%s: heap invariant violated: data[%d]=%d > data[%d]=%d",
					context, i, int(*h.data[i]), j, int(*h.data[j]))
			}
		}
	}
}

// checkContents verifies that the heap holds exactly the values in want
// (as a multiset, duplicates included).
func checkContents(t *testing.T, h *Heap[myHeap], want map[int]int, context string) {
	t.Helper()
	got := make(map[int]int, h.Len())
	total := 0
	for _, v := range h.data {
		got[int(*v)]++
		total++
	}
	if total != h.Len() {
		t.Fatalf("%s: Len()=%d but backing slice has %d elements", context, h.Len(), total)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%s: contents mismatch: got %v, want %v", context, got, want)
	}
}

func refMin(want map[int]int) (int, bool) {
	keys := make([]int, 0, len(want))
	for k, c := range want {
		if c > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return 0, false
	}
	sort.Ints(keys)
	return keys[0], true
}

func refDel(want map[int]int, v int) {
	want[v]--
	if want[v] <= 0 {
		delete(want, v)
	}
}

// TestRandomOpsAgainstReference hammers Push/Pop/Peek/Delete/Fix/SetValue in
// random order (with duplicates) and cross-checks every operation against a
// multiset reference model.  This is the test that decides whether Delete
// and Fix are actually broken.
func TestRandomOpsAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for trial := 0; trial < 200; trial++ {
		h := NewHeap[myHeap]()
		want := make(map[int]int)

		for op := 0; op < 200; op++ {
			ctx := fmt.Sprintf("trial %d op %d", trial, op)
			n := h.Len()
			choice := rng.Intn(100)
			switch {
			case choice < 35 || n == 0:
				// Push
				v := myHeap(rng.Intn(50)) // small range => many duplicates
				h.Push(&v)
				want[int(v)]++
			case choice < 55:
				// Pop
				got := h.Pop()
				mn, ok := refMin(want)
				if !ok {
					t.Fatalf("%s: Pop on non-empty heap returned nil", ctx)
				}
				if got == nil || int(*got) != mn {
					t.Fatalf("%s: Pop got %v, want min %d", ctx, got, mn)
				}
				refDel(want, mn)
			case choice < 60:
				// Peek
				got := h.Peek()
				mn, _ := refMin(want)
				if got == nil || int(*got) != mn {
					t.Fatalf("%s: Peek got %v, want %d", ctx, got, mn)
				}
				if h.Len() != n {
					t.Fatalf("%s: Peek changed length %d -> %d", ctx, n, h.Len())
				}
			case choice < 80:
				// Delete at a random valid index
				idx := rng.Intn(n)
				expectVal := int(*h.GetValue(idx))
				got := h.Delete(idx)
				if got == nil || int(*got) != expectVal {
					t.Fatalf("%s: Delete(%d) returned %v, want %d", ctx, idx, got, expectVal)
				}
				refDel(want, expectVal)
			default:
				// Fix / SetValue at a random valid index
				idx := rng.Intn(n)
				oldVal := int(*h.GetValue(idx))
				nv := myHeap(rng.Intn(50))
				if rng.Intn(2) == 0 {
					h.Fix(idx, &nv)
				} else {
					h.SetValue(idx, &nv)
				}
				refDel(want, oldVal)
				want[int(nv)]++
			}
			checkInvariant(t, h, ctx)
			checkContents(t, h, want, ctx)
		}

		// Drain: must come out in non-decreasing order.
		prev := -1
		for h.Len() > 0 {
			got := h.Pop()
			if got == nil {
				t.Fatalf("trial %d drain: Pop returned nil", trial)
			}
			if int(*got) < prev {
				t.Fatalf("trial %d drain: Pop out of order: %d after %d", trial, int(*got), prev)
			}
			prev = int(*got)
			refDel(want, int(*got))
			checkInvariant(t, h, "drain")
		}
		if len(want) != 0 {
			t.Fatalf("trial %d: reference not empty after drain: %v", trial, want)
		}
		if got := h.Pop(); got != nil {
			t.Fatalf("trial %d: Pop on emptied heap returned %v", trial, *got)
		}
	}
}

// TestDeleteEveryIndex deletes every index of a fresh heap in turn and
// verifies the returned value and the remaining contents.
func TestDeleteEveryIndex(t *testing.T) {
	for size := 1; size <= 64; size++ {
		for idx := 0; idx < size; idx++ {
			h := NewHeap[myHeap]()
			for i := 1; i <= size; i++ {
				v := myHeap(i)
				h.Push(&v)
			}
			expectVal := int(*h.GetValue(idx))
			got := h.Delete(idx)
			ctx := fmt.Sprintf("size %d Delete(%d)", size, idx)
			if got == nil || int(*got) != expectVal {
				t.Fatalf("%s: returned %v, want %d", ctx, got, expectVal)
			}
			if h.Len() != size-1 {
				t.Fatalf("%s: length %d, want %d", ctx, h.Len(), size-1)
			}
			want := make(map[int]int, size)
			for i := 1; i <= size; i++ {
				if i != expectVal {
					want[i] = 1
				}
			}
			checkInvariant(t, h, ctx)
			checkContents(t, h, want, ctx)
		}
	}
}

// TestFixEveryDirection replaces every index in turn with a new minimum
// (must rise) and a new maximum (must sink).
func TestFixEveryDirection(t *testing.T) {
	const size = 33
	for idx := 0; idx < size; idx++ {
		for _, nv := range []int{0, 1000} { // below min (1), above max (size)
			h := NewHeap[myHeap]()
			for i := 1; i <= size; i++ {
				v := myHeap(i)
				h.Push(&v)
			}
			oldVal := int(*h.GetValue(idx))
			v := myHeap(nv)
			h.Fix(idx, &v)
			ctx := fmt.Sprintf("Fix(%d, %d)", idx, nv)
			want := make(map[int]int, size)
			for i := 1; i <= size; i++ {
				if i != oldVal {
					want[i] = 1
				}
			}
			want[nv] = 1
			checkInvariant(t, h, ctx)
			checkContents(t, h, want, ctx)
		}
	}
}

// TestSingleElementOps exercises the size-1 heap edge cases.
func TestSingleElementOps(t *testing.T) {
	h := NewHeap[myHeap]()
	v := myHeap(7)
	h.Push(&v)
	checkInvariant(t, h, "single push")

	nv := myHeap(3)
	h.Fix(0, &nv)
	checkInvariant(t, h, "single fix")
	if got := h.Peek(); got == nil || int(*got) != 3 {
		t.Fatalf("after Fix(0): Peek got %v, want 3", got)
	}

	got := h.Delete(0)
	if got == nil || int(*got) != 3 {
		t.Fatalf("Delete(0) on size-1 heap got %v, want 3", got)
	}
	if h.Len() != 0 {
		t.Fatalf("after Delete(0): length %d, want 0", h.Len())
	}

	h.Push(&v)
	if got := h.Pop(); got == nil || int(*got) != 7 {
		t.Fatalf("Pop on size-1 heap got %v, want 7", got)
	}
	if got := h.Pop(); got != nil {
		t.Fatalf("second Pop got %v, want nil", got)
	}
}

// TestDuplicatesStress pushes many copies of a few distinct values and
// drains them in order.
func TestDuplicatesStress(t *testing.T) {
	h := NewHeap[myHeap]()
	rng := rand.New(rand.NewSource(7))
	want := make(map[int]int)
	for i := 0; i < 1000; i++ {
		v := myHeap(rng.Intn(3)) // only 0,1,2
		h.Push(&v)
		want[int(v)]++
	}
	checkInvariant(t, h, "duplicates")
	checkContents(t, h, want, "duplicates")
	prev := -1
	for h.Len() > 0 {
		got := h.Pop()
		if int(*got) < prev {
			t.Fatalf("Pop out of order with duplicates: %d after %d", int(*got), prev)
		}
		prev = int(*got)
	}
}

// TestSearchDuplicates verifies Search returns a valid position for a
// matching element and nil/-1 for a missing one.
func TestSearchDuplicates(t *testing.T) {
	h := NewHeap[myHeap]()
	for i := 0; i < 10; i++ {
		v := myHeap(5)
		h.Push(&v)
	}
	needle := myHeap(5)
	rv, pos, err := h.Search(&needle)
	if err != nil || rv == nil || pos < 0 || pos >= h.Len() {
		t.Fatalf("Search(5): rv=%v pos=%d err=%v", rv, pos, err)
	}
	if int(*h.GetValue(pos)) != 5 {
		t.Fatalf("Search(5): pos %d holds %d", pos, int(*h.GetValue(pos)))
	}
	missing := myHeap(99)
	rv, pos, _ = h.Search(&missing)
	if rv != nil || pos != -1 {
		t.Fatalf("Search(99): rv=%v pos=%d, want nil/-1", rv, pos)
	}
}

// TestOutOfRangePanics covers GetValue/SetValue/Fix bounds checks.
func TestOutOfRangePanics(t *testing.T) {
	h := NewHeap[myHeap]()
	v := myHeap(1)
	h.Push(&v)
	nv := myHeap(2)
	for name, fn := range map[string]func(int){
		"GetValue": func(i int) { h.GetValue(i) },
		"SetValue": func(i int) { h.SetValue(i, &nv) },
		"Fix":      func(i int) { h.Fix(i, &nv) },
		"Delete":   func(i int) { h.Delete(i) },
	} {
		for _, idx := range []int{-1, 1, 100} {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("%s(%d): expected panic, got none", name, idx)
					}
				}()
				fn(idx)
			}()
		}
	}
}

// TestHeapifyRebuildWithDuplicates rebuilds via AppendHeap+Heapify with
// duplicate values and verifies the drain order.
func TestHeapifyRebuildWithDuplicates(t *testing.T) {
	h := NewHeap[myHeap]()
	v0 := myHeap(1)
	h.Push(&v0) // start non-empty so AppendHeap mixes with existing data
	rng := rand.New(rand.NewSource(99))
	data := make([]*myHeap, 0, 300)
	want := map[int]int{1: 1}
	for i := 0; i < 300; i++ {
		v := myHeap(rng.Intn(20))
		data = append(data, &v)
		want[int(v)]++
	}
	h.AppendHeap(data)
	for i := h.Len()/2 - 1; i >= 0; i-- {
		h.Heapify(h.Len(), i)
	}
	checkInvariant(t, h, "heapify rebuild")
	checkContents(t, h, want, "heapify rebuild")
	prev := -1
	for h.Len() > 0 {
		got := h.Pop()
		if int(*got) < prev {
			t.Fatalf("drain out of order: %d after %d", int(*got), prev)
		}
		prev = int(*got)
	}
}
