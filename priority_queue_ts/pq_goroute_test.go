// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package priority_queue_ts

import (
	"fmt"
	"sync"
	"testing"
)

// TestPQGoroutineConcurrency runs concurrent Inserters, Poppers, readers,
// Search/UpdatePriority/Delete users, Lock/Unlock batch users and an
// iterator against one shared queue.  It is meant to be run with the race
// detector (make race).  The point is to detect data races, deadlocks and
// panics, not exact interleavings.
func TestPQGoroutineConcurrency(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()

	// Seed the queue.
	for i := 1; i <= 100; i++ {
		pq.Insert(&PqTest{value: fmt.Sprintf("seed%d", i), priority: i})
	}

	var wg sync.WaitGroup

	// Writers: Insert.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				pq.Insert(&PqTest{value: "w", priority: base + i%97})
			}
		}(g * 1000)
	}

	// Readers: Pop.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				pq.Pop() // nil on empty is fine
			}
		}()
	}

	// Readers: Peek / Len / IsEmpty / Search / All.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			needle := &PqTest{priority: 42}
			for i := 0; i < 200; i++ {
				pq.Peek()
				pq.Len()
				pq.IsEmpty()
				pq.Search(needle)
				for range pq.All() {
					break // exercise the snapshot iterator, then stop early
				}
			}
		}()
	}

	// Search-then-update/delete users (positions may race; errors are fine).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			probe := &PqTest{priority: i % 97}
			if _, pos, err := pq.Search(probe); err == nil {
				pq.UpdatePriority(pos, &PqTest{value: "u", priority: i})
				pq.Delete(pos)
			}
		}
	}()

	// Atomic batch users of Lock/Unlock + Nl methods.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			pq.Lock()
			pq.NlInsert(&PqTest{value: "b", priority: i})
			if !pq.NlIsEmpty() {
				pq.NlPeek()
				pq.NlPop()
			}
			pq.Unlock()
		}
	}()

	wg.Wait()

	// The queue must still drain in sorted order.
	prev := -1
	for pq.Len() > 0 {
		got := pq.Pop()
		if got == nil {
			t.Fatal("Pop returned nil on a non-empty queue")
		}
		if got.priority < prev {
			t.Fatalf("drain out of order: %d after %d", got.priority, prev)
		}
		prev = got.priority
	}
}
