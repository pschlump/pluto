package index_pq

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"strings"
	"testing"
)

// expectPanic runs fx and fails the test unless it panics; when want is
// non-empty the panic message must contain it.
func expectPanic(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if want != "" {
			if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
				t.Errorf("Unexpected panic message from %s: %v (expected it to contain %q)", name, r, want)
			}
		}
	}()
	fx()
}

// -------------------------------------------------------------------------------------------------------
// Constructors and the panic contract
// -------------------------------------------------------------------------------------------------------

func TestNewIndexPQ(t *testing.T) {
	q := NewIndexPQ[int](10)
	if q == nil {
		t.Fatalf("NewIndexPQ returned nil.")
	}
	if !q.IsEmpty() || q.Len() != 0 {
		t.Errorf("Expected a new queue to be empty, got Len=%d", q.Len())
	}
	if _, _, found := q.Peek(); found {
		t.Errorf("Expected Peek on a new queue to report false.")
	}
	if _, _, found := q.Pop(); found {
		t.Errorf("Expected Pop on a new queue to report false.")
	}
	for k := range 10 {
		if q.Contains(k) {
			t.Errorf("Expected Contains(%d) on a new queue to be false.", k)
		}
	}
}

func TestCompare(t *testing.T) {
	if c := Compare(1, 2); c != -1 {
		t.Errorf("Compare(1,2) = %d, expected -1", c)
	}
	if c := Compare(2, 1); c != 1 {
		t.Errorf("Compare(2,1) = %d, expected +1", c)
	}
	if c := Compare(1, 1); c != 0 {
		t.Errorf("Compare(1,1) = %d, expected 0", c)
	}
	if c := Compare("abc", "abd"); c != -1 {
		t.Errorf("Compare(abc,abd) = %d, expected -1", c)
	}
}

func TestNewIndexPQFuncNilPanics(t *testing.T) {
	expectPanic(t, "NewIndexPQFunc(10, nil)", "nil comparison function", func() {
		NewIndexPQFunc[int](10, nil)
	})
}

func TestNewIndexPQBadN(t *testing.T) {
	expectPanic(t, "NewIndexPQ(0)", "n < 1", func() { NewIndexPQ[int](0) })
	expectPanic(t, "NewIndexPQ(-3)", "n < 1", func() { NewIndexPQ[int](-3) })
	expectPanic(t, "NewIndexPQFunc(0, fx)", "n < 1", func() {
		NewIndexPQFunc(0, Compare[int])
	})
}

// TestIndexPQNilPanics verifies the documented panics when Insert or
// Change are called on a nil queue — writes with no sane answer.
func TestIndexPQNilPanics(t *testing.T) {
	var nilQ *IndexPQ[int]
	expectPanic(t, "Insert", "Insert", func() { nilQ.Insert(0, 1) })
	expectPanic(t, "Change", "Change", func() { nilQ.Change(0, 1) })
}

// TestIndexPQNilTolerated verifies that every operation other than
// Insert and Change treats a nil queue as an empty queue.
func TestIndexPQNilTolerated(t *testing.T) {
	var nilQ *IndexPQ[int]

	if !nilQ.IsEmpty() || nilQ.Len() != 0 {
		t.Errorf("Expected nil queue to be empty.")
	}
	if _, _, found := nilQ.Peek(); found {
		t.Errorf("Expected Peek on nil queue to report false.")
	}
	if _, _, found := nilQ.Pop(); found {
		t.Errorf("Expected Pop on nil queue to report false.")
	}
	if nilQ.Contains(0) {
		t.Errorf("Expected Contains on nil queue to be false.")
	}
	if _, found := nilQ.Value(0); found {
		t.Errorf("Expected Value on nil queue to report false.")
	}
	if nilQ.Delete(0) {
		t.Errorf("Expected Delete on nil queue to report false.")
	}
	nilQ.Lock()     // no-op, must not panic
	nilQ.Unlock()   // no-op, must not panic
	nilQ.Truncate() // no-op, must not panic
	for range nilQ.All() {
		t.Errorf("Expected no pairs from All on nil queue.")
	}
}

// TestZeroValueIndexPQ verifies that the zero value behaves as an empty
// queue for every read operation and that Insert/Change fail loudly
// because no comparison function has been set.
func TestZeroValueIndexPQ(t *testing.T) {
	var q IndexPQ[int]

	if !q.IsEmpty() || q.Len() != 0 {
		t.Errorf("Expected zero-value queue to be empty.")
	}
	if _, _, found := q.Peek(); found {
		t.Errorf("Expected Peek on zero-value queue to report false.")
	}
	if _, _, found := q.Pop(); found {
		t.Errorf("Expected Pop on zero-value queue to report false.")
	}
	if q.Contains(0) {
		t.Errorf("Expected Contains on zero-value queue to be false.")
	}
	if _, found := q.Value(0); found {
		t.Errorf("Expected Value on zero-value queue to report false.")
	}
	if q.Delete(0) {
		t.Errorf("Expected Delete on zero-value queue to report false.")
	}
	q.Truncate() // no-op, must not panic
	for range q.All() {
		t.Errorf("Expected no pairs from All on zero-value queue.")
	}

	expectPanic(t, "Insert on zero-value queue", "NewIndexPQ", func() { q.Insert(0, 1) })
	expectPanic(t, "Change on zero-value queue", "NewIndexPQ", func() { q.Change(0, 1) })
}

// -------------------------------------------------------------------------------------------------------
// Core operations
// -------------------------------------------------------------------------------------------------------

func TestIndexPQInsertAndPopOrder(t *testing.T) {
	q := NewIndexPQ[int](8)
	// Values shuffled relative to the indices.
	vals := []int{50, 20, 90, 10, 40, 70, 30, 60}
	for k, v := range vals {
		if !q.Insert(k, v) {
			t.Fatalf("Insert(%d, %d) reported false.", k, v)
		}
	}
	if q.Len() != 8 {
		t.Fatalf("Expected Len 8, got %d", q.Len())
	}

	if k, v, found := q.Peek(); !found || k != 3 || v != 10 {
		t.Errorf("Peek = (%d, %d, %v), expected (3, 10, true)", k, v, found)
	}

	// Pops come out in ascending value order.
	wantVal := []int{10, 20, 30, 40, 50, 60, 70, 90}
	wantKey := []int{3, 1, 6, 4, 0, 7, 5, 2}
	for i := range wantVal {
		k, v, found := q.Pop()
		if !found {
			t.Fatalf("Pop %d: unexpectedly empty.", i)
		}
		if k != wantKey[i] || v != wantVal[i] {
			t.Errorf("Pop %d = (%d, %d), expected (%d, %d)", i, k, v, wantKey[i], wantVal[i])
		}
	}
	if _, _, found := q.Pop(); found {
		t.Errorf("Expected Pop on drained queue to report false.")
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after draining.")
	}
}

// TestIndexPQInsertReplaces verifies that Insert on an already-present
// index replaces the value (and re-orders the heap) instead of failing.
func TestIndexPQInsertReplaces(t *testing.T) {
	q := NewIndexPQ[int](5)
	q.Insert(0, 50)
	q.Insert(1, 20)
	q.Insert(2, 90)

	if !q.Insert(2, 5) { // in range: true, and the value is replaced
		t.Errorf("Expected Insert of a present in-range index to report true.")
	}
	if q.Len() != 3 {
		t.Errorf("Expected Len to stay 3 after replace, got %d", q.Len())
	}
	if v, found := q.Value(2); !found || v != 5 {
		t.Errorf("Value(2) = (%d, %v), expected (5, true)", v, found)
	}
	if k, v, found := q.Peek(); !found || k != 2 || v != 5 {
		t.Errorf("Peek after replace = (%d, %d, %v), expected (2, 5, true)", k, v, found)
	}
}

// TestIndexPQChange verifies decrease-key and increase-key: changing the
// value of an index re-orders the queue.
func TestIndexPQChange(t *testing.T) {
	q := NewIndexPQ[int](5)
	q.Insert(0, 50)
	q.Insert(1, 20)
	q.Insert(2, 90)

	// Decrease-key: index 2 becomes the minimum.
	if !q.Change(2, 1) {
		t.Fatalf("Expected Change(2, 1) to report true.")
	}
	if k, v, found := q.Peek(); !found || k != 2 || v != 1 {
		t.Errorf("Peek after decrease-key = (%d, %d, %v), expected (2, 1, true)", k, v, found)
	}

	// Increase-key: index 2 sinks back below the others.
	if !q.Change(2, 99) {
		t.Fatalf("Expected Change(2, 99) to report true.")
	}
	if k, v, found := q.Peek(); !found || k != 1 || v != 20 {
		t.Errorf("Peek after increase-key = (%d, %d, %v), expected (1, 20, true)", k, v, found)
	}

	// Change on an absent index reports false and changes nothing.
	if q.Change(3, 1) {
		t.Errorf("Expected Change on an absent index to report false.")
	}
	if q.Len() != 3 {
		t.Errorf("Expected Len 3 after failed Change, got %d", q.Len())
	}
}

func TestIndexPQDelete(t *testing.T) {
	q := NewIndexPQ[int](6)
	for k, v := range []int{50, 20, 90, 10, 40, 70} {
		q.Insert(k, v)
	}

	if !q.Delete(2) { // 90 — not the min
		t.Fatalf("Expected Delete(2) to report true.")
	}
	if q.Contains(2) {
		t.Errorf("Expected Contains(2) to be false after Delete.")
	}
	if q.Len() != 5 {
		t.Errorf("Expected Len 5 after Delete, got %d", q.Len())
	}
	if q.Delete(2) {
		t.Errorf("Expected second Delete(2) to report false.")
	}

	// The remaining values drain in ascending order.
	var got []int
	for {
		_, v, found := q.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{10, 20, 40, 50, 70}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain after Delete got %v, expected %v", got, expect)
	}
}

// TestIndexPQOutOfRange verifies that out-of-range indices report false
// instead of panicking.
func TestIndexPQOutOfRange(t *testing.T) {
	q := NewIndexPQ[int](4)
	q.Insert(0, 10)

	for _, bad := range []int{-1, -100, 4, 5, 1000} {
		if q.Insert(bad, 1) {
			t.Errorf("Expected Insert(%d) to report false.", bad)
		}
		if q.Change(bad, 1) {
			t.Errorf("Expected Change(%d) to report false.", bad)
		}
		if q.Delete(bad) {
			t.Errorf("Expected Delete(%d) to report false.", bad)
		}
		if q.Contains(bad) {
			t.Errorf("Expected Contains(%d) to be false.", bad)
		}
		if _, found := q.Value(bad); found {
			t.Errorf("Expected Value(%d) to report false.", bad)
		}
	}
	if q.Len() != 1 {
		t.Errorf("Out-of-range operations changed the queue: Len = %d", q.Len())
	}
}

func TestIndexPQValueAndContains(t *testing.T) {
	q := NewIndexPQ[string](3)
	q.Insert(0, "pear")
	q.Insert(2, "fig")

	if v, found := q.Value(0); !found || v != "pear" {
		t.Errorf("Value(0) = (%q, %v), expected (pear, true)", v, found)
	}
	if _, found := q.Value(1); found {
		t.Errorf("Expected Value(1) to report false (absent).")
	}
	if !q.Contains(2) || q.Contains(1) {
		t.Errorf("Contains mismatch: expected 2 present, 1 absent.")
	}

	// The queue is full at n; inserting the last slot still works.
	if !q.Insert(1, "apple") {
		t.Errorf("Expected Insert into the final slot to report true.")
	}
	if k, v, found := q.Peek(); !found || k != 1 || v != "apple" {
		t.Errorf("Peek = (%d, %q, %v), expected (1, apple, true)", k, v, found)
	}
}

func TestIndexPQTruncateReuse(t *testing.T) {
	q := NewIndexPQ[int](4)
	for k, v := range []int{3, 1, 4, 1} {
		q.Insert(k, v)
	}
	q.Truncate()
	if !q.IsEmpty() || q.Len() != 0 {
		t.Errorf("Expected empty queue after Truncate.")
	}
	for k := range 4 {
		if q.Contains(k) {
			t.Errorf("Expected Contains(%d) to be false after Truncate.", k)
		}
	}

	// Fully reusable, same index space and ordering.
	q.Insert(2, 9)
	q.Insert(0, 7)
	if k, v, found := q.Peek(); !found || k != 0 || v != 7 {
		t.Errorf("Peek after reuse = (%d, %d, %v), expected (0, 7, true)", k, v, found)
	}

	// Truncating an already-empty queue is fine.
	q.Truncate()
	q.Truncate()
	if q.Len() != 0 {
		t.Errorf("Expected empty queue after double Truncate.")
	}
}

// TestIndexPQMaxPQ verifies that a reversed comparison function turns
// the queue into a max-first priority queue.
func TestIndexPQMaxPQ(t *testing.T) {
	q := NewIndexPQFunc(6, func(a, b int) int { return -Compare(a, b) })
	for k, v := range []int{5, 1, 9, 3, 7, 2} {
		q.Insert(k, v)
	}
	var got []int
	for {
		_, v, found := q.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{9, 7, 5, 3, 2, 1}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Max-first drain got %v, expected %v", got, expect)
	}
}

// -------------------------------------------------------------------------------------------------------
// All iterator
// -------------------------------------------------------------------------------------------------------

func TestIndexPQAll(t *testing.T) {
	q := NewIndexPQ[int](6)
	vals := []int{50, 20, 90, 10, 40, 70}
	for k, v := range vals {
		q.Insert(k, v)
	}

	// Priority order, minimum first.
	var gotV []int
	var gotK []int
	for k, v := range q.All() {
		gotK = append(gotK, k)
		gotV = append(gotV, v)
	}
	if expect := []int{10, 20, 40, 50, 70, 90}; !reflect.DeepEqual(gotV, expect) {
		t.Errorf("All values = %v, expected %v", gotV, expect)
	}
	if expect := []int{3, 1, 4, 0, 5, 2}; !reflect.DeepEqual(gotK, expect) {
		t.Errorf("All indices = %v, expected %v", gotK, expect)
	}

	// Non-destructive: the queue is unchanged.
	if q.Len() != 6 {
		t.Errorf("Expected Len 6 after All, got %d", q.Len())
	}
	if k, _, found := q.Peek(); !found || k != 3 {
		t.Errorf("Peek after All = (%d, %v), expected (3, true)", k, found)
	}

	// Mutating from inside the loop is safe (snapshot semantics).
	n := 0
	for k := range q.All() {
		q.Delete(k)
		n++
	}
	if n != 6 {
		t.Errorf("Expected 6 pairs while deleting inside the loop, got %d", n)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected the deletes inside the loop to drain the queue.")
	}

	// Early break stops iteration.
	for k, v := range vals {
		q.Insert(k, v)
	}
	n = 0
	for range q.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 pair, got %d", n)
	}

	// Empty queue yields nothing.
	empty := NewIndexPQ[int](2)
	for range empty.All() {
		t.Errorf("Expected no pairs from All on an empty queue.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkIndexPQSize = 4096

func BenchmarkIndexPQInsert(b *testing.B) {
	q := NewIndexPQ[int](benchmarkIndexPQSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.Len() >= benchmarkIndexPQSize {
			q.Truncate()
		}
		q.Insert(i%benchmarkIndexPQSize, i)
	}
}

func BenchmarkIndexPQInsertPop(b *testing.B) {
	q := NewIndexPQ[int](benchmarkIndexPQSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := i % benchmarkIndexPQSize
		q.Insert(k, i)
		if _, _, found := q.Pop(); !found {
			b.Fatalf("Pop: unexpectedly empty")
		}
	}
}
