/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq_ts

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

// TestNewDHeap checks the constructors and D().
func TestNewDHeap(t *testing.T) {
	h := NewDHeap[int](4)
	if h.D() != 4 || !h.IsEmpty() || h.Len() != 0 {
		t.Fatalf("fresh d=4 heap: D=%d IsEmpty=%v Len=%d", h.D(), h.IsEmpty(), h.Len())
	}
	hf := NewDHeapFunc(2, func(a, b string) int { return -Compare(a, b) }) // reversed: max-heap
	if hf.D() != 2 {
		t.Fatalf("DHeapFunc D = %d", hf.D())
	}
}

// TestPanicContract pins the four multiway_pq-shaped panics with the
// twin prefix.
func TestPanicContract(t *testing.T) {
	expectPanic(t, "multiway_pq_ts: NewDHeapFunc called with a nil comparison function", func() {
		_ = NewDHeapFunc[int](3, nil)
	})
	expectPanic(t, "multiway_pq_ts: NewDHeap/NewDHeapFunc called with d < 2", func() {
		_ = NewDHeap[int](1)
	})
	var nilHeap *DHeap[int]
	expectPanic(t, "multiway_pq_ts: Push called on a nil heap", func() {
		nilHeap.Push(1)
	})
	zero := &DHeap[int]{}
	expectPanic(t, "multiway_pq_ts: Push called on a heap with no comparison function", func() {
		zero.Push(1)
	})
	expectPanic(t, "multiway_pq_ts: AppendHeap called on a nil heap", func() {
		nilHeap.AppendHeap([]int{1})
	})
	expectPanic(t, "multiway_pq_ts: AppendHeap called on a heap with no comparison function", func() {
		zero.AppendHeap([]int{1})
	})
	expectPanic(t, "multiway_pq_ts: UnmarshalJSON called on a nil heap", func() {
		_ = nilHeap.UnmarshalJSON([]byte(`[1]`))
	})
	expectPanic(t, "multiway_pq_ts: UnmarshalJSON called on a heap with no comparison function", func() {
		_ = zero.UnmarshalJSON([]byte(`[1]`))
	})
	// Storing nothing is tolerated everywhere.
	_ = nilHeap.UnmarshalJSON([]byte(`[]`))
	_ = zero.UnmarshalJSON([]byte(`null`))
}

// TestPushPopPeek drives Push/Pop/Peek at several branching factors:
// the pop sequence must be nondecreasing (the sorted drain) and the
// invariant must hold after every step.
func TestPushPopPeek(t *testing.T) {
	rng := newRand()
	for _, d := range []int{2, 3, 4, 8} {
		h := NewDHeap[int](d)
		if h.D() != d {
			t.Fatalf("D() = %d, want %d", h.D(), d)
		}
		for range 200 {
			h.Push(rng.Intn(1000))
		}
		checkHeapInvariant(t, h)
		last := -1
		for !h.IsEmpty() {
			p, ok := h.Peek()
			if !ok {
				t.Fatalf("d=%d: Peek on a non-empty heap failed", d)
			}
			v, found := h.Pop()
			if !found {
				t.Fatalf("d=%d: Pop on a non-empty heap failed", d)
			}
			if p != v {
				t.Fatalf("d=%d: Peek %d != Pop %d", d, p, v)
			}
			if v < last {
				t.Fatalf("d=%d: pop sequence not sorted: %d after %d", d, v, last)
			}
			last = v
		}
		if _, ok := h.Pop(); ok {
			t.Fatalf("d=%d: Pop on the drained heap reported found", d)
		}
	}
}

// TestDeleteFixGetValue covers the indexed operations: out-of-range
// reports false, deleting every index drains in sorted order, and Fix
// restores the invariant in both directions.
func TestDeleteFixGetValue(t *testing.T) {
	h := NewDHeap[int](3)
	if _, ok := h.Delete(-1); ok {
		t.Fatalf("negative Delete reported found")
	}
	if _, ok := h.GetValue(0); ok {
		t.Fatalf("GetValue past the end reported found")
	}
	if h.Fix(0, 9) {
		t.Fatalf("Fix past the end reported true")
	}
	for i := range 50 {
		h.Push((i*37)%50 + 1) // shuffled 1..50 with duplicates
	}
	checkHeapInvariant(t, h)

	// Fix in both directions.
	if !h.Fix(0, -100) { // decrease the root: stays the root
		t.Fatalf("Fix at 0 reported false")
	}
	checkHeapInvariant(t, h)
	if v, _ := h.Peek(); v != -100 {
		t.Fatalf("after decreasing the root, Peek = %d", v)
	}
	if !h.Fix(0, 1000) { // increase past everything: sinks away
		t.Fatalf("second Fix at 0 reported false")
	}
	checkHeapInvariant(t, h)

	// Delete every index, in a shuffled order: the multiset must come
	// out nondecreasing only via Pop afterwards; here we check the
	// count and the invariant instead (values are removed by index, so
	// order is unspecified).
	n := h.Len()
	for i := n - 1; i >= 0; i-- {
		if _, ok := h.Delete(i); !ok {
			t.Fatalf("Delete(%d) on a live index failed", i)
		}
		if i%7 == 0 {
			checkHeapInvariant(t, h)
		}
	}
	if !h.IsEmpty() || h.Len() != 0 {
		t.Fatalf("heap not empty after deleting every index")
	}

	// SetValue is Fix's alias.
	h.Push(5)
	h.Push(3)
	if !h.SetValue(1, 1) {
		t.Fatalf("SetValue reported false")
	}
	checkHeapInvariant(t, h)
}

// TestSearch checks the linear scan and the partial-field probe shape
// (the probe only needs the fields the comparison function reads).
func TestSearch(t *testing.T) {
	type task struct {
		name     string
		priority int
	}
	h := NewDHeapFunc(3, func(a, b task) int { return a.priority - b.priority })
	for _, x := range []task{{"a", 5}, {"b", 3}, {"c", 5}, {"d", 1}} {
		h.Push(x)
	}
	if v, pos, ok := h.Search(task{priority: 3}); !ok || v.name != "b" || pos < 0 {
		t.Fatalf("Search({priority:3}) = %+v, %d, %v", v, pos, ok)
	}
	if _, _, ok := h.Search(task{priority: 9}); ok {
		t.Fatalf("Search for a missing priority reported found")
	}
}

// TestRandomizedModel runs a mixed operation stream at the fixed seed
// against a map multiset model, at several branching factors, checking
// the heap invariant and the multiset periodically.
func TestRandomizedModel(t *testing.T) {
	for _, d := range []int{2, 3, 5} {
		rng := newRand()
		h := NewDHeap[int](d)
		model := make(map[int]int)
		size := 0
		for step := range 2000 {
			switch rng.Intn(8) {
			case 0, 1, 2:
				v := rng.Intn(50)
				h.Push(v)
				model[v]++
				size++
			case 3:
				v, ok := h.Pop()
				if ok != (size > 0) {
					t.Fatalf("d=%d step %d: Pop found=%v, size=%d", d, step, ok, size)
				}
				if ok {
					if model[v] == 0 {
						t.Fatalf("d=%d step %d: popped %d not in the model", d, step, v)
					}
					// v must be a minimum of the model multiset.
					for k := range model {
						if k < v {
							t.Fatalf("d=%d step %d: popped %d but %d is smaller", d, step, v, k)
						}
					}
					model[v]--
					if model[v] == 0 {
						delete(model, v)
					}
					size--
				}
			case 4:
				if size > 0 {
					i := rng.Intn(size)
					v, ok := h.Delete(i)
					if !ok {
						t.Fatalf("d=%d step %d: Delete(%d) failed", d, step, i)
					}
					model[v]--
					if model[v] == 0 {
						delete(model, v)
					}
					size--
				}
			case 5:
				if size > 0 {
					i := rng.Intn(size)
					old, _ := h.GetValue(i)
					fresh := rng.Intn(50)
					if !h.Fix(i, fresh) {
						t.Fatalf("d=%d step %d: Fix failed", d, step)
					}
					model[old]--
					if model[old] == 0 {
						delete(model, old)
					}
					model[fresh]++
				}
			case 6:
				if rng.Intn(4) == 0 {
					h.Truncate()
					clear(model)
					size = 0
				}
			case 7:
				if h.Len() != size {
					t.Fatalf("d=%d step %d: Len=%d size=%d", d, step, h.Len(), size)
				}
			}
			if step%97 == 0 {
				checkHeapInvariant(t, h)
				if h.Len() != size {
					t.Fatalf("d=%d step %d: Len=%d size=%d", d, step, h.Len(), size)
				}
			}
		}
	}
}

// TestAppendHeapCompound is the heap_sort_ts-shaped canonical bulk
// build: NlAppendHeap leaves a non-heap, and the bottom-up NlHeapify
// sweep under the SAME hold of Lock makes the whole build atomic.
func TestAppendHeapCompound(t *testing.T) {
	h := NewDHeap[int](4)
	x := []int{9, 4, 7, 1, 8, 2, 6, 3, 5}
	h.Lock()
	h.NlAppendHeap(x)
	for i := (h.NlLen() - 2) / h.D(); i >= 0; i-- {
		h.NlHeapify(h.NlLen(), i)
	}
	h.Unlock()
	checkHeapInvariant(t, h)
	want := append([]int(nil), x...)
	slices.Sort(want)
	for _, w := range want {
		if v, ok := h.Pop(); !ok || v != w {
			t.Fatalf("pop after compound build = %d, %v; want %d", v, ok, w)
		}
	}
}

// TestAllSnapshot pins the twin's iterator rule: All walks a
// SNAPSHOT — mutating the heap from inside the loop body is safe.
func TestAllSnapshot(t *testing.T) {
	h := NewDHeap[int](3)
	for _, v := range []int{5, 1, 4, 2, 3} {
		h.Push(v)
	}
	seen := make([]int, 0, 5)
	for v := range h.All() {
		seen = append(seen, v)
		h.Pop() // safe: snapshot iteration
	}
	if len(seen) != 5 {
		t.Fatalf("All visited %d elements, want 5 (the snapshot)", len(seen))
	}
	if !h.IsEmpty() {
		t.Fatalf("in-loop Pops did not stick")
	}
	for v := range h.All() { // empty now
		t.Fatalf("All on the empty heap yielded %d", v)
	}
	var nilHeap *DHeap[int]
	for v := range nilHeap.All() { // nil iterates as empty
		t.Fatalf("nil heap's All yielded %d", v)
	}
}

// TestCapabilities covers the §4.5 menu through the twin: Slice/String
// (backing order), Equal (multiset, d-blind, non-destructive, ties)
// and Clone (preserves d, layout and pop sequence).
func TestCapabilities(t *testing.T) {
	h := NewDHeap[int](3)
	for _, v := range []int{3, 1, 2} {
		h.Push(v)
	}
	if got := h.String(); got != "[1 3 2]" {
		t.Fatalf("String = %q (internal backing order, NOT sorted)", got)
	}
	if !slices.Equal(h.Slice(), []int{1, 3, 2}) {
		t.Fatalf("Slice = %v", h.Slice())
	}

	// StringFunc's format callback may call back into the heap.
	got := h.StringFunc(func(v int) string {
		_, _ = h.Peek() // a read from inside the format callback
		return fmt.Sprintf("#%d", v)
	})
	if got != "[#1 #3 #2]" {
		t.Fatalf("StringFunc = %q", got)
	}

	// Equal: multiset, d-blind, insertion-order-blind, non-destructive.
	a := NewDHeap[int](2)
	for _, v := range []int{5, 1, 5, 3} {
		a.Push(v)
	}
	b := NewDHeap[int](8)
	for _, v := range []int{1, 3, 5, 5} {
		b.Push(v)
	}
	if !a.Equal(b) {
		t.Fatalf("equal multisets at d=2 vs d=8 reported not equal (Equal is d-blind)")
	}
	if got := a.String(); got == "" || got[0] != '[' {
		t.Fatalf("Equal destroyed the receiver: %q", got)
	}
	c := NewDHeap[int](2)
	for _, v := range []int{5, 1, 5, 4} {
		c.Push(v)
	}
	if a.Equal(c) {
		t.Fatalf("different multisets reported equal")
	}
	if a.Len() != 4 {
		t.Fatalf("Equal changed the length")
	}

	// Clone preserves d, the exact layout and the pop sequence.
	cl := a.Clone()
	if cl.D() != a.D() || !slices.Equal(cl.Slice(), a.Slice()) {
		t.Fatalf("clone lost d or layout: d=%d slice=%v", cl.D(), cl.Slice())
	}
	for {
		va, oka := a.Pop()
		vb, okb := cl.Pop()
		if oka != okb || va != vb {
			t.Fatalf("clone diverged in the pop sequence: %d/%v vs %d/%v", va, oka, vb, okb)
		}
		if !oka {
			break
		}
	}

	// Nil/zero-value totals.
	var nilHeap *DHeap[int]
	if s := nilHeap.String(); s != "[]" {
		t.Fatalf("nil String = %q", s)
	}
	if nilHeap.Slice() != nil {
		t.Fatalf("nil Slice not nil")
	}
	if !nilHeap.Equal(&DHeap[int]{d: 2}) {
		t.Fatalf("nil heap not equal to an empty heap")
	}
	if nilHeap.Clone() != nil {
		t.Fatalf("nil Clone not nil")
	}
	if z := (&DHeap[int]{}).D(); z != 2 {
		t.Fatalf("zero-value D = %d, want the d=2 fallback", z)
	}
}

// TestJSON round-trips through the twin: marshal is the snapshot in
// backing order, unmarshal decodes outside the lock and rebuilds
// atomically in the O(n) sweep.
func TestJSON(t *testing.T) {
	h := NewDHeap[int](4)
	for _, v := range []int{3, 1, 2} {
		h.Push(v)
	}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "[1,3,2]" {
		t.Fatalf("marshal = %s (backing order)", data)
	}
	if err := h.UnmarshalJSON([]byte(`[9,4,7,1,8,2,6,3,5]`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	checkHeapInvariant(t, h)
	if h.Len() != 9 {
		t.Fatalf("Len after unmarshal = %d", h.Len())
	}
	last := -1
	for range 9 {
		x, ok := h.Pop()
		if !ok || x < last {
			t.Fatalf("unmarshaled heap not a heap: pop %d after %d", x, last)
		}
		last = x
	}
	// Decode error leaves the heap untouched.
	h.Push(1)
	before := h.String()
	if err := h.UnmarshalJSON([]byte(`[1,`)); err == nil {
		t.Fatalf("malformed document did not error")
	}
	if h.String() != before {
		t.Fatalf("failed decode changed the heap: %s", h.String())
	}
	// Empty array and null clear.
	for _, doc := range []string{`[]`, `null`} {
		if err := h.UnmarshalJSON([]byte(doc)); err != nil {
			t.Fatalf("unmarshal %s: %v", doc, err)
		}
		if !h.IsEmpty() {
			t.Fatalf("unmarshal %s did not clear", doc)
		}
	}
	var nilHeap *DHeap[int]
	data, err = nilHeap.MarshalJSON()
	if err != nil || string(data) != "[]" {
		t.Fatalf("nil marshal = %s, %v", data, err)
	}
}

// TestNilTolerated checks the sane empty answers on a nil heap.
func TestNilTolerated(t *testing.T) {
	var h *DHeap[int]
	if h.Len() != 0 || h.Length() != 0 || !h.IsEmpty() {
		t.Fatalf("nil Len/IsEmpty not empty")
	}
	if _, ok := h.Pop(); ok {
		t.Fatalf("nil Pop found")
	}
	if _, ok := h.Peek(); ok {
		t.Fatalf("nil Peek found")
	}
	if _, ok := h.Delete(0); ok {
		t.Fatalf("nil Delete found")
	}
	if h.Fix(0, 1) {
		t.Fatalf("nil Fix true")
	}
	if _, ok := h.GetValue(0); ok {
		t.Fatalf("nil GetValue found")
	}
	if _, _, ok := h.Search(1); ok {
		t.Fatalf("nil Search found")
	}
	if h.D() != 2 {
		t.Fatalf("nil D = %d", h.D())
	}
	for v := range h.All() {
		t.Fatalf("nil All yielded %d", v)
	}
	h.Truncate()
	h.Lock()
	h.Unlock()
	if _, err := h.MarshalJSON(); err != nil {
		t.Fatalf("nil MarshalJSON errored")
	}
}

// TestDHeapConcurrent runs writers against one shared heap, then
// drains it with concurrent poppers and a reader — the heap_ts battery
// shape, primarily a test for the race detector (`make race`); the
// accounting must balance exactly.
func TestDHeapConcurrent(t *testing.T) {
	for _, d := range []int{2, 4} {
		h := NewDHeap[int](d)
		const producers = 8
		const per = 250
		const total = producers * per
		var producedSum = int64(total) * int64(total+1) / 2

		var wg sync.WaitGroup
		for p := range producers {
			wg.Go(func() {
				for i := range per {
					h.Push(p*per + i + 1)
				}
			})
		}
		// A reader iterates snapshots and queries while the writers work.
		stop := make(chan struct{})
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				for range h.All() {
				}
				_ = h.Len()
				_, _ = h.Peek()
				_, _, _ = h.Search(42)
				_ = h.String()
			}
		}()
		wg.Wait()

		// Concurrent poppers drain; exit on the COUNT of pops (exactly
		// total items were pushed).  Popped values are collected and
		// checked sorted afterwards.
		var poppedCount atomic.Int64
		var poppedSum atomic.Int64
		var popped []int
		var mu sync.Mutex
		for range 4 {
			wg.Go(func() {
				for {
					v, found := h.Pop()
					if !found {
						if poppedCount.Load() >= total {
							return
						}
						continue
					}
					poppedCount.Add(1)
					poppedSum.Add(int64(v))
					mu.Lock()
					popped = append(popped, v)
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		close(stop)

		if got := poppedCount.Load(); got != total {
			t.Errorf("d=%d: expected %d pops, got %d", d, total, got)
		}
		if got := poppedSum.Load(); got != producedSum {
			t.Errorf("d=%d: pop sum %d, expected %d", d, got, producedSum)
		}
		if !h.IsEmpty() {
			t.Errorf("d=%d: heap not empty after the drain: Len = %d", d, h.Len())
		}
		slices.Sort(popped)
		for i := 1; i <= total; i++ {
			if popped[i-1] != i {
				t.Fatalf("d=%d: popped multiset wrong at %d: %d", d, i, popped[i-1])
			}
		}
	}
}

// TestEqualNoDeadlock compares the same pair of heaps from two
// goroutines in opposite directions — Equal must never hold two locks
// at once.
func TestEqualNoDeadlock(t *testing.T) {
	a := NewDHeap[int](2)
	b := NewDHeap[int](5)
	for i := range 100 {
		a.Push(i)
		b.Push(99 - i)
	}
	done := make(chan bool)
	go func() {
		_ = a.Equal(b)
		done <- true
	}()
	go func() {
		_ = b.Equal(a)
		done <- true
	}()
	<-done
	<-done
}
