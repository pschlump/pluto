package dqueue_ts

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

// TestSingleElement covers single-element edge cases: every combination of
// pushing on one end and popping from either end must leave an empty queue.
func TestSingleElement(t *testing.T) {
	for _, tc := range []struct {
		name string
		push func(q *Deque[TestDemo], v *TestDemo)
		pop  func(q *Deque[TestDemo]) (*TestDemo, error)
	}{
		{"PushFront/PopFront", (*Deque[TestDemo]).PushFront, (*Deque[TestDemo]).PopFront},
		{"PushFront/PopBack", (*Deque[TestDemo]).PushFront, (*Deque[TestDemo]).PopBack},
		{"PushBack/PopFront", (*Deque[TestDemo]).PushBack, (*Deque[TestDemo]).PopFront},
		{"PushBack/PopBack", (*Deque[TestDemo]).PushBack, (*Deque[TestDemo]).PopBack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var q Deque[TestDemo]
			tc.push(&q, &TestDemo{S: "x"})

			if q.Length() != 1 {
				t.Fatalf("Expected length 1 got %d", q.Length())
			}

			// Both peeks see the same single element.
			f, err := q.PeekFront()
			if err != nil || f.S != "x" {
				t.Errorf("PeekFront: expected x got %v, err %v", f, err)
			}
			b, err := q.PeekBack()
			if err != nil || b.S != "x" {
				t.Errorf("PeekBack: expected x got %v, err %v", b, err)
			}

			v, err := tc.pop(&q)
			if err != nil || v.S != "x" {
				t.Errorf("Pop: expected x got %v, err %v", v, err)
			}
			if !q.IsEmpty() {
				t.Errorf("Expected empty queue after popping the single element")
			}
			if q.Length() != 0 {
				t.Errorf("Expected length 0 got %d", q.Length())
			}
		})
	}
}

// TestDuplicateValues verifies the queue stores duplicate values without
// conflating them.
func TestDuplicateValues(t *testing.T) {
	var q Deque[TestDemo]
	for i := 0; i < 5; i++ {
		q.PushBack(&TestDemo{S: "dup"})
	}
	if q.Length() != 5 {
		t.Errorf("Expected length 5 got %d", q.Length())
	}
	n := 0
	for _, v := range q.All() {
		if v.S != "dup" {
			t.Errorf("Expected dup got %s", v.S)
		}
		n++
	}
	if n != 5 {
		t.Errorf("All: expected 5 elements got %d", n)
	}
	for i := 0; i < 5; i++ {
		v, err := q.PopFront()
		if err != nil || v.S != "dup" {
			t.Errorf("PopFront: expected dup got %v, err %v", v, err)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after popping all duplicates")
	}
}

// TestTruncateReuse verifies the queue is fully usable after Truncate.
func TestTruncateReuse(t *testing.T) {
	var q Deque[TestDemo]
	for i := 0; i < 3; i++ {
		q.PushBack(&TestDemo{S: fmt.Sprintf("a%d", i)})
	}
	q.Truncate()
	if !q.IsEmpty() || q.Length() != 0 {
		t.Fatalf("Expected empty queue after Truncate, length %d", q.Length())
	}

	q.PushBack(&TestDemo{S: "z"})
	v, err := q.PeekBack()
	if err != nil || v.S != "z" {
		t.Errorf("PeekBack after reuse: expected z got %v, err %v", v, err)
	}
	v, err = q.PopFront()
	if err != nil || v.S != "z" {
		t.Errorf("PopFront after reuse: expected z got %v, err %v", v, err)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after reuse drain")
	}
}

// TestIteratorEarlyBreak covers early termination of both iterators.
func TestIteratorEarlyBreak(t *testing.T) {
	var q Deque[TestDemo]
	for i := 0; i < 5; i++ {
		q.PushBack(&TestDemo{S: fmt.Sprintf("v%d", i)})
	}

	n := 0
	for range q.Backward() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("Backward: expected early break after 2 elements, got %d", n)
	}

	// Breaking immediately yields exactly one element.
	n = 0
	for range q.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("All: expected 1 element before break, got %d", n)
	}
	n = 0
	for range q.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Backward: expected 1 element before break, got %d", n)
	}
}

// TestRandomized runs hundreds of mixed operations against the queue with a
// fixed seed and cross-checks every observable result (length, front, back,
// pop order, iteration order) against a plain slice reference model.
func TestRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	var q Deque[TestDemo]
	var model []*TestDemo

	check := func(step int) {
		t.Helper()
		if q.Length() != len(model) {
			t.Fatalf("step %d: length mismatch: got %d want %d", step, q.Length(), len(model))
		}
		if q.IsEmpty() != (len(model) == 0) {
			t.Fatalf("step %d: IsEmpty mismatch: got %v want %v", step, q.IsEmpty(), len(model) == 0)
		}
		f, ferr := q.PeekFront()
		b, berr := q.PeekBack()
		if len(model) == 0 {
			if ferr == nil || berr == nil {
				t.Fatalf("step %d: expected errors peeking empty queue", step)
			}
			return
		}
		if ferr != nil || f.S != model[0].S {
			t.Fatalf("step %d: PeekFront got %v, err %v; want %s", step, f, ferr, model[0].S)
		}
		if berr != nil || b.S != model[len(model)-1].S {
			t.Fatalf("step %d: PeekBack got %v, err %v; want %s", step, b, berr, model[len(model)-1].S)
		}
	}

	for step := 0; step < 2000; step++ {
		op := rng.Intn(10)
		switch {
		case op < 3: // PushBack
			v := &TestDemo{S: fmt.Sprintf("v%d", rng.Intn(100))}
			q.PushBack(v)
			model = append(model, v)
		case op < 6: // PushFront
			v := &TestDemo{S: fmt.Sprintf("v%d", rng.Intn(100))}
			q.PushFront(v)
			model = append([]*TestDemo{v}, model...)
		case op < 7: // PopFront
			v, err := q.PopFront()
			if len(model) == 0 {
				if err == nil {
					t.Fatalf("step %d: PopFront on empty: expected error", step)
				}
			} else {
				if err != nil || v != model[0] {
					t.Fatalf("step %d: PopFront got %v, err %v; want %v", step, v, err, model[0])
				}
				model = model[1:]
			}
		case op < 8: // PopBack
			v, err := q.PopBack()
			if len(model) == 0 {
				if err == nil {
					t.Fatalf("step %d: PopBack on empty: expected error", step)
				}
			} else {
				if err != nil || v != model[len(model)-1] {
					t.Fatalf("step %d: PopBack got %v, err %v; want %v", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case op < 9: // verify full iteration order against the model
			i := 0
			for idx, v := range q.All() {
				if idx != i || v != model[i] {
					t.Fatalf("step %d: All[%d] got %v; want %v", step, i, v, model[i])
				}
				i++
			}
			if i != len(model) {
				t.Fatalf("step %d: All yielded %d elements; want %d", step, i, len(model))
			}
			for idx, v := range q.Backward() {
				if v != model[idx] {
					t.Fatalf("step %d: Backward[%d] got %v; want %v", step, idx, v, model[idx])
				}
			}
		default: // Truncate
			q.Truncate()
			model = model[:0]
		}
		check(step)
	}
}

// TestConcurrentIterators runs concurrent writers, poppers, peekers and
// iterators against one shared queue.  Run with `go test -race`; the race
// detector validates the locking.  Correctness of the data is checked
// loosely — the point is to detect data races, deadlocks and panics, not
// exact interleavings.
func TestConcurrentIterators(t *testing.T) {
	var q Deque[TestDemo]

	// Seed the queue so readers and iterators have something to work on.
	for i := 0; i < 100; i++ {
		q.PushBack(&TestDemo{S: fmt.Sprintf("seed-%d", i)})
	}

	var wg sync.WaitGroup

	// Writers: PushFront / PushBack.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				v := &TestDemo{S: fmt.Sprintf("w%d-%d", base, i)}
				if i%2 == 0 {
					q.PushFront(v)
				} else {
					q.PushBack(v)
				}
			}
		}(g)
	}

	// Consumers: PopFront / PopBack.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(even bool) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if even {
					_, _ = q.PopFront()
				} else {
					_, _ = q.PopBack()
				}
			}
		}(g%2 == 0)
	}

	// Readers: Peek / IsEmpty / Length.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_, _ = q.PeekFront()
				_, _ = q.PeekBack()
				_ = q.IsEmpty()
				_ = q.Length()
			}
		}()
	}

	// Iterators: full and early-break passes, forward and backward.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(full bool) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if full {
					for range q.All() {
					}
					for range q.Backward() {
					}
				} else {
					for range q.All() {
						break
					}
					for range q.Backward() {
						break
					}
				}
			}
		}(g%2 == 0)
	}

	wg.Wait()

	// The queue must still be consistent: length matches a final drain.
	drained := 0
	for {
		if _, err := q.PopFront(); err != nil {
			break
		}
		drained++
	}
	if q.Length() != 0 || !q.IsEmpty() {
		t.Errorf("Expected empty queue after drain, length %d", q.Length())
	}
	t.Logf("drained %d remaining elements", drained)
}
