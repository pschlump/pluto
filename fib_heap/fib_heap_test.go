// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package fib_heap

import "testing"

func TestNew(t *testing.T) {
	q := NewIndexFibonacciPQ[int](8)
	if q.Len() != 0 || !q.IsEmpty() {
		t.Fatalf("fresh queue not empty: Len=%d IsEmpty=%v", q.Len(), q.IsEmpty())
	}
	if _, _, ok := q.Peek(); ok {
		t.Fatal("Peek on a fresh queue reported found")
	}
	if _, _, ok := q.Pop(); ok {
		t.Fatal("Pop on a fresh queue reported found")
	}
	if q.Contains(0) {
		t.Fatal("Contains(0) true on a fresh queue")
	}
	n := 0
	for range q.All() {
		n++
	}
	if n != 0 {
		t.Fatalf("All on a fresh queue yielded %d items", n)
	}
}

func TestConstructorPanics(t *testing.T) {
	expectPanic(t, "NewIndexFibonacciPQFunc(nil)", func() { NewIndexFibonacciPQFunc[int](4, nil) })
	expectPanic(t, "NewIndexFibonacciPQ(-1)", func() { NewIndexFibonacciPQ[int](-1) })
	expectPanic(t, "NewIndexFibonacciPQFunc(0, fx)", func() { NewIndexFibonacciPQFunc(0, Compare[int]) })
}

func TestInsertPeekPop(t *testing.T) {
	q := NewIndexFibonacciPQ[int](6)
	for _, kv := range [][2]int{{0, 30}, {1, 10}, {2, 50}, {3, 20}, {4, 40}, {5, 0}} {
		if !q.Insert(kv[0], kv[1]) {
			t.Fatalf("Insert(%d, %d) = false, want true", kv[0], kv[1])
		}
	}
	if q.Len() != 6 {
		t.Fatalf("Len = %d, want 6", q.Len())
	}
	orderK := []int{5, 1, 3, 0, 4, 2} // values 0,10,20,30,40,50
	orderV := []int{0, 10, 20, 30, 40, 50}
	for i := range 6 {
		k, v, ok := q.Peek()
		if !ok || k != orderK[i] || v != orderV[i] {
			t.Fatalf("Peek %d = (%d, %d, %v), want (%d, %d, true)", i, k, v, ok, orderK[i], orderV[i])
		}
		k, v, ok = q.Pop()
		if !ok || k != orderK[i] || v != orderV[i] {
			t.Fatalf("Pop %d = (%d, %d, %v), want (%d, %d, true)", i, k, v, ok, orderK[i], orderV[i])
		}
	}
	if !q.IsEmpty() {
		t.Fatal("queue not empty after popping everything")
	}
}

func TestInsertOutOfRange(t *testing.T) {
	q := NewIndexFibonacciPQ[int](3)
	if q.Insert(-1, 1) {
		t.Fatal("Insert(-1) = true, want false")
	}
	if q.Insert(3, 1) {
		t.Fatal("Insert(3) = true, want false")
	}
	if q.Len() != 0 {
		t.Fatalf("Len = %d after rejected inserts, want 0", q.Len())
	}
	if q.Insert(2, 22) && !q.Contains(2) {
		t.Fatal("Insert(2, 22) accepted but Contains(2) false")
	}
}

func TestInsertReplaces(t *testing.T) {
	q := NewIndexFibonacciPQ[int](4)
	q.Insert(0, 40)
	q.Insert(1, 10)
	if !q.Insert(0, 5) { // replace with a decrease: now the minimum
		t.Fatal("Insert on a present index = false, want true")
	}
	k, v, _ := q.Peek()
	if k != 0 || v != 5 {
		t.Fatalf("Peek after replace-decrease = (%d, %d), want (0, 5)", k, v)
	}
	if !q.Insert(0, 50) { // replace with an increase: no longer the minimum
		t.Fatal("Insert on a present index = false, want true")
	}
	k, v, _ = q.Peek()
	if k != 1 || v != 10 {
		t.Fatalf("Peek after replace-increase = (%d, %d), want (1, 10)", k, v)
	}
	if got, _ := q.Value(0); got != 50 {
		t.Fatalf("Value(0) = %d, want 50", got)
	}
}

func TestChangeBothDirections(t *testing.T) {
	q := NewIndexFibonacciPQ[string](4)
	q.Insert(0, "delta")
	q.Insert(1, "alpha")
	q.Insert(2, "charlie")
	q.Insert(3, "bravo")

	if !q.Change(0, "aaa") { // decrease: 0 becomes the minimum
		t.Fatal("Change(0, aaa) = false, want true")
	}
	k, v, _ := q.Peek()
	if k != 0 || v != "aaa" {
		t.Fatalf("Peek after decrease = (%d, %s), want (0, aaa)", k, v)
	}

	if !q.Change(0, "zzz") { // increase: back to the tail
		t.Fatal("Change(0, zzz) = false, want true")
	}
	k, v, _ = q.Peek()
	if k != 1 || v != "alpha" {
		t.Fatalf("Peek after increase = (%d, %s), want (1, alpha)", k, v)
	}

	if q.Change(9, "x") {
		t.Fatal("Change(9, x) = true, want false (out of range)")
	}
	q.Delete(0)
	if q.Change(0, "x") {
		t.Fatal("Change on an absent index = true, want false")
	}
}

func TestDecreaseKey(t *testing.T) {
	q := NewIndexFibonacciPQ[int](5)
	for i := range 5 {
		q.Insert(i, (i+1)*10) // 10,20,30,40,50
	}
	if q.DecreaseKey(9, 1) {
		t.Fatal("DecreaseKey out of range = true, want false")
	}
	if q.DecreaseKey(4, 50) {
		t.Fatal("DecreaseKey equal value = true, want false")
	}
	if q.DecreaseKey(4, 60) {
		t.Fatal("DecreaseKey increase = true, want false")
	}
	if got, _ := q.Value(4); got != 50 {
		t.Fatalf("Value(4) changed by a rejected DecreaseKey: %d", got)
	}
	if !q.DecreaseKey(4, 5) {
		t.Fatal("DecreaseKey(4, 5) = false, want true")
	}
	k, v, _ := q.Peek()
	if k != 4 || v != 5 {
		t.Fatalf("Peek after DecreaseKey = (%d, %d), want (4, 5)", k, v)
	}
}

func TestDelete(t *testing.T) {
	q := NewIndexFibonacciPQ[int](5)
	for i := range 5 {
		q.Insert(i, i*10)
	}
	if q.Delete(9) {
		t.Fatal("Delete out of range = true, want false")
	}
	if !q.Delete(0) { // the current minimum
		t.Fatal("Delete(0) = false, want true")
	}
	if q.Contains(0) {
		t.Fatal("Contains(0) still true after Delete")
	}
	if k, _, _ := q.Peek(); k != 1 {
		t.Fatalf("Peek after deleting the minimum = %d, want 1", k)
	}
	// Drain interleaved with deletes: pop 1, delete 3, pop 2 and 4.
	q.Pop()
	q.Delete(3)
	var got []int
	for {
		k, _, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, k)
	}
	if len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Fatalf("remaining pops = %v, want [2 4]", got)
	}
}

func TestAllPriorityOrder(t *testing.T) {
	q := NewIndexFibonacciPQ[int](5)
	for i := range 5 {
		q.Insert(i, (5-i)*10) // 50,40,30,20,10
	}
	var vs []int
	for _, v := range q.All() {
		vs = append(vs, v)
	}
	want := []int{10, 20, 30, 40, 50}
	for i := range want {
		if vs[i] != want[i] {
			t.Fatalf("All values = %v, want %v", vs, want)
		}
	}
	if q.Len() != 5 {
		t.Fatalf("All drained the queue: Len = %d, want 5", q.Len())
	}
	// Mutating from inside the loop is safe (All drains a snapshot).
	for k, v := range q.All() {
		if k == 2 && v == 30 {
			q.Delete(2)
			q.Insert(2, 5)
		}
	}
	if got, _ := q.Value(2); got != 5 {
		t.Fatalf("Value(2) = %d after mutation inside All, want 5", got)
	}
}

func TestTruncate(t *testing.T) {
	q := NewIndexFibonacciPQ[int](4)
	for i := range 4 {
		q.Insert(i, i)
	}
	q.Truncate()
	if q.Len() != 0 || !q.IsEmpty() {
		t.Fatalf("queue not empty after Truncate: Len=%d", q.Len())
	}
	if q.Contains(0) {
		t.Fatal("Contains(0) true after Truncate")
	}
	if !q.Insert(2, 22) { // index space and ordering kept
		t.Fatal("Insert after Truncate = false")
	}
	k, v, ok := q.Peek()
	if !ok || k != 2 || v != 22 {
		t.Fatalf("Peek after Truncate+Insert = (%d, %d, %v)", k, v, ok)
	}
}

func TestZeroValue(t *testing.T) {
	var q IndexFibonacciPQ[int]
	if q.Len() != 0 || !q.IsEmpty() {
		t.Fatal("zero-value queue not empty")
	}
	if _, _, ok := q.Peek(); ok {
		t.Fatal("Peek on zero value reported found")
	}
	if _, _, ok := q.Pop(); ok {
		t.Fatal("Pop on zero value reported found")
	}
	if q.Contains(0) || q.Delete(0) || q.DecreaseKey(0, 1) {
		t.Fatal("index ops on zero value reported true")
	}
	if _, ok := q.Value(0); ok {
		t.Fatal("Value on zero value reported found")
	}
	n := 0
	for range q.All() {
		n++
	}
	if n != 0 {
		t.Fatalf("All on zero value yielded %d items", n)
	}
	q.Truncate() // tolerated
}

func TestNilAndZeroPanics(t *testing.T) {
	var nilQ *IndexFibonacciPQ[int]
	var zero IndexFibonacciPQ[int]
	expectPanic(t, "Insert on nil", func() { nilQ.Insert(0, 1) })
	expectPanic(t, "Insert on zero value", func() { zero.Insert(0, 1) })
	expectPanic(t, "Change on nil", func() { nilQ.Change(0, 1) })
	expectPanic(t, "Change on zero value", func() { zero.Change(0, 1) })
	// Everything else tolerates nil.
	if nilQ.Len() != 0 || nilQ.Contains(0) || nilQ.Delete(0) || nilQ.DecreaseKey(0, 1) {
		t.Fatal("nil queue did not behave as an empty queue")
	}
	if _, _, ok := nilQ.Peek(); ok {
		t.Fatal("nil queue Peek reported found")
	}
	if _, _, ok := nilQ.Pop(); ok {
		t.Fatal("nil queue Pop reported found")
	}
}

func TestMaxQueueViaReversedComparator(t *testing.T) {
	q := NewIndexFibonacciPQFunc(4, func(a, b int) int { return -Compare(a, b) })
	for i, v := range []int{10, 40, 20, 30} {
		q.Insert(i, v)
	}
	want := []int{40, 30, 20, 10}
	for i := range 4 {
		_, v, ok := q.Pop()
		if !ok || v != want[i] {
			t.Fatalf("Pop %d of max queue = (%d, %v), want %d", i, v, ok, want[i])
		}
	}
}

func expectPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected a panic", what)
		}
	}()
	fn()
}

func TestPanicMessages(t *testing.T) {
	msgs := []struct {
		fn   func()
		want string
	}{
		{func() { NewIndexFibonacciPQFunc[int](4, nil) },
			"fib_heap: NewIndexFibonacciPQFunc called with a nil comparison function"},
		{func() { NewIndexFibonacciPQ[int](0) },
			"fib_heap: NewIndexFibonacciPQ/NewIndexFibonacciPQFunc called with n < 1 (no usable index space)"},
	}
	for _, m := range msgs {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("no panic, want %q", m.want)
					return
				}
				if got, ok := r.(string); !ok || got != m.want {
					t.Errorf("panic = %v, want %q", r, m.want)
				}
			}()
			m.fn()
		}()
	}
	// The Insert/Change panic messages, checked once each (string form).
	var zero IndexFibonacciPQ[int]
	checkMsg := func(fn func(), want string) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("no panic, want %q", want)
				return
			}
			if got, ok := r.(string); !ok || got != want {
				t.Errorf("panic = %v, want %q", r, want)
			}
		}()
		fn()
	}
	checkMsg(func() { zero.Insert(0, 1) },
		"fib_heap: Insert called on a queue with no comparison function (create the queue with NewIndexFibonacciPQ or NewIndexFibonacciPQFunc)")
	checkMsg(func() { zero.Change(0, 1) },
		"fib_heap: Change called on a queue with no comparison function (create the queue with NewIndexFibonacciPQ or NewIndexFibonacciPQFunc)")
	var nilQ *IndexFibonacciPQ[int]
	checkMsg(func() { nilQ.Insert(0, 1) }, "fib_heap: Insert called on a nil queue")
	checkMsg(func() { nilQ.Change(0, 1) }, "fib_heap: Change called on a nil queue")
}
