// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package heap_ts

import (
	"sync"
	"testing"
)

// TestGoroutineConcurrency runs concurrent Pushers, Poppers, Peeking
// readers, Searchers and an iterator against one shared heap.  It is meant
// to be run with the race detector (make race).  Correctness of the
// resulting data is checked loosely — the point is to detect data races,
// deadlocks and panics, not exact interleavings.
func TestGoroutineConcurrency(t *testing.T) {
	h := NewHeap[myHeap]()

	// Seed the heap so readers/poppers have something to work on.
	for i := 1; i <= 100; i++ {
		v := myHeap(i)
		h.Push(&v)
	}

	var wg sync.WaitGroup

	// Writers: Push.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				v := myHeap(base + i%97)
				h.Push(&v)
			}
		}(g * 1000)
	}

	// Readers: Pop.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				h.Pop() // nil on empty is fine
			}
		}()
	}

	// Readers: Peek / Len / Search / All.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			needle := myHeap(42)
			for i := 0; i < 200; i++ {
				h.Peek()
				h.Len()
				h.Length()
				h.Search(&needle)
				for range h.All() {
					break // exercise the snapshot iterator, then stop early
				}
			}
		}()
	}

	// Batch user of Lock/Unlock + Nl methods.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			h.Lock()
			v := myHeap(i)
			h.NlPush(&v)
			_ = h.NlLen()
			if h.NlLen() > 0 {
				h.NlPop()
			}
			h.Unlock()
		}
	}()

	// Delete/Fix on index 0 whenever the heap is non-empty.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if h.Len() > 0 {
				func() {
					defer func() { _ = recover() }() // index may race with a concurrent Pop
					v := myHeap(i)
					h.Fix(0, &v)
					h.Delete(0)
				}()
			}
		}
	}()

	wg.Wait()

	// The heap must still be a valid heap and drain in sorted order.
	h.verify(t, 0)
	prev := -1
	for h.Len() > 0 {
		got := h.Pop()
		if got == nil {
			t.Fatal("Pop returned nil on a non-empty heap")
		}
		if int(*got) < prev {
			t.Fatalf("drain out of order: %d after %d", int(*got), prev)
		}
		prev = int(*got)
	}
}
