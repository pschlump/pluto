package binomial_queue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// TestItem is the test element type.  Ordering is supplied to the
// queue as a plain function (cmpTestItem below).
type TestItem struct {
	S string
}

// cmpTestItem orders TestItem by its S field.
func cmpTestItem(a, b TestItem) int {
	return strings.Compare(a.S, b.S)
}

// newTestQueue builds a min-first queue of TestItem ordered by S.
func newTestQueue() *BinomialQueue[TestItem] {
	return NewBinomialQueueFunc(cmpTestItem)
}

func TestNewBinomialQueue(t *testing.T) {
	q := newTestQueue()
	if q == nil {
		t.Fatalf("NewBinomialQueueFunc returned nil.")
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected length 0, got %d/%d", q.Len(), q.Length())
	}
	if _, found := q.Peek(); found {
		t.Errorf("Expected Peek on empty queue to report false.")
	}
	if _, found := q.FindMin(); found {
		t.Errorf("Expected FindMin on empty queue to report false.")
	}
}

func TestInsertAndDeleteMin(t *testing.T) {
	q := newTestQueue()

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		q.Insert(TestItem{S: s})
		checkInvariants(t, q, cmpTestItem)
	}

	if v, found := q.Peek(); !found || v.S != "00" {
		t.Errorf("Peek = (%v, %v), expected 00", v, found)
	}

	// DeleteMin drains in sorted order.
	for _, want := range []string{"00", "02", "03", "05", "09"} {
		v, found := q.DeleteMin()
		if !found {
			t.Fatalf("DeleteMin: unexpectedly empty.")
		}
		if v.S != want {
			t.Errorf("DeleteMin = %s, expected %s", v.S, want)
		}
		checkInvariants(t, q, cmpTestItem)
	}
	if _, found := q.DeleteMin(); found {
		t.Errorf("Expected DeleteMin on drained queue to report false.")
	}
	if q.trees != nil {
		t.Errorf("Expected nil forest after a full drain.")
	}
}

func TestPeekAndFindMin(t *testing.T) {
	q := newTestQueue()
	q.Insert(TestItem{S: "05"})
	q.Insert(TestItem{S: "02"})
	q.Insert(TestItem{S: "09"})

	// Peek/FindMin do not remove.
	for i := range 3 {
		if v, found := q.Peek(); !found || v.S != "02" {
			t.Errorf("Peek step %d = (%v, %v), expected 02", i, v, found)
		}
		if v, found := q.FindMin(); !found || v.S != "02" {
			t.Errorf("FindMin step %d = (%v, %v), expected 02", i, v, found)
		}
	}
	if q.Len() != 3 {
		t.Errorf("Expected Peek/FindMin to leave the length at 3, got %d", q.Len())
	}
}

func TestDeleteMinOnEmpty(t *testing.T) {
	q := newTestQueue()

	if _, found := q.DeleteMin(); found {
		t.Errorf("Expected DeleteMin on empty queue to report false.")
	}
	// Still usable.
	q.Insert(TestItem{S: "x"})
	if v, found := q.DeleteMin(); !found || v.S != "x" {
		t.Errorf("DeleteMin after empty-cycle = (%v, %v)", v, found)
	}
}

func TestWithDifferentElements(t *testing.T) {
	// A queue of ints with the natural ordering.
	q := NewBinomialQueue[int]()
	for _, v := range []int{42, 7, 13, 99, 55, 0} {
		q.Insert(v)
		checkInvariants(t, q, Compare[int])
	}
	var got []int
	for {
		v, found := q.DeleteMin()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{0, 7, 13, 42, 55, 99}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain got %v, expected %v", got, expect)
	}
}

// TestMerge verifies the signature operation: other is absorbed into the
// receiver and left empty.
func TestMerge(t *testing.T) {
	a := newTestQueue()
	for _, s := range []string{"05", "02", "09"} {
		a.Insert(TestItem{S: s})
	}
	b := newTestQueue()
	for _, s := range []string{"00", "03", "07"} {
		b.Insert(TestItem{S: s})
	}

	a.Merge(b)
	checkInvariants(t, a, cmpTestItem)
	if a.Len() != 6 {
		t.Errorf("Expected length 6 after Merge, got %d", a.Len())
	}
	if !b.IsEmpty() || b.Len() != 0 {
		t.Errorf("Expected other to be empty after Merge, got length %d", b.Len())
	}

	var got []string
	for {
		v, found := a.DeleteMin()
		if !found {
			break
		}
		got = append(got, v.S)
	}
	if expect := []string{"00", "02", "03", "05", "07", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain after Merge got %v, expected %v", got, expect)
	}

	// The absorbed queue is reusable.
	b.Insert(TestItem{S: "zz"})
	if v, found := b.Peek(); !found || v.S != "zz" {
		t.Errorf("Peek on reused other = (%v, %v), expected zz", v, found)
	}
}

// TestMergeEdgeCases covers the tolerant and adopting paths of Merge.
func TestMergeEdgeCases(t *testing.T) {
	// Merging an empty other is a no-op.
	a := newTestQueue()
	a.Insert(TestItem{S: "05"})
	empty := newTestQueue()
	a.Merge(empty)
	checkInvariants(t, a, cmpTestItem)
	if a.Len() != 1 {
		t.Errorf("Expected length 1 after merging empty other, got %d", a.Len())
	}

	// Merging a nil other is a no-op.
	a.Merge(nil)
	if a.Len() != 1 {
		t.Errorf("Expected length 1 after merging nil other, got %d", a.Len())
	}

	// Merging a queue into itself is a no-op.
	a.Merge(a)
	checkInvariants(t, a, cmpTestItem)
	if a.Len() != 1 {
		t.Errorf("Expected length 1 after self-merge, got %d", a.Len())
	}

	// A zero-value receiver adopts other's comparison function.
	b := newTestQueue()
	b.Insert(TestItem{S: "02"})
	b.Insert(TestItem{S: "01"})
	var z BinomialQueue[TestItem]
	z.Merge(b)
	if z.cmp == nil {
		t.Fatalf("Expected zero-value receiver to adopt other's comparison function.")
	}
	if v, found := z.DeleteMin(); !found || v.S != "01" {
		t.Errorf("DeleteMin on adopting receiver = (%v, %v), expected 01", v, found)
	}

	// Merge on a nil receiver with a non-empty other panics.
	full := newTestQueue()
	full.Insert(TestItem{S: "x"})
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Merge on nil receiver to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Merge") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var nq *BinomialQueue[TestItem]
		nq.Merge(full)
	}()

	// Merge on a nil receiver with an empty other is tolerated.
	var nq *BinomialQueue[TestItem]
	nq.Merge(empty)
	nq.Merge(nil)
}

func TestInsertSameValueRepeatedly(t *testing.T) {
	q := NewBinomialQueue[int]()
	for range 10 {
		q.Insert(7)
		checkInvariants(t, q, Compare[int])
	}
	if q.Len() != 10 {
		t.Fatalf("Expected length 10, got %d", q.Len())
	}
	for i := range 10 {
		if v, found := q.DeleteMin(); !found || v != 7 {
			t.Fatalf("DeleteMin %d = (%v, %v), expected 7", i, v, found)
		}
	}
	if _, found := q.DeleteMin(); found {
		t.Errorf("Expected drained queue.")
	}
}

// TestInsertSizes exercises every forest shape up to 64 elements — the
// carry chains across all degree boundaries.
func TestInsertSizes(t *testing.T) {
	q := NewBinomialQueue[int]()
	for i := range 64 {
		q.Insert(i)
		checkInvariants(t, q, Compare[int])
	}
	if q.Len() != 64 {
		t.Fatalf("Expected length 64, got %d", q.Len())
	}
	for i := range 64 {
		v, found := q.DeleteMin()
		if !found || v != i {
			t.Fatalf("DeleteMin = (%v, %v), expected %d", v, found, i)
		}
		checkInvariants(t, q, Compare[int])
	}
}

func TestAllIterator(t *testing.T) {
	q := newTestQueue()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		q.Insert(TestItem{S: s})
	}

	// All visits every element exactly once (internal order, not sorted).
	var got []string
	for i, v := range q.All() {
		if i != len(got) {
			t.Errorf("All index %d, expected %d", i, len(got))
		}
		got = append(got, v.S)
	}
	if len(got) != q.Len() {
		t.Fatalf("All visited %d items, queue has %d", len(got), q.Len())
	}
	seen := map[string]int{}
	for _, s := range got {
		seen[s]++
	}
	if expect := map[string]int{"00": 1, "02": 1, "03": 1, "05": 1, "09": 1}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("All visited %v, expected %v", seen, expect)
	}

	// A single-variable range yields the index.
	var indexes []int
	for i := range q.All() {
		indexes = append(indexes, i)
	}
	if expect := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(indexes, expect) {
		t.Errorf("All indexes got %v, expected %v", indexes, expect)
	}

	// Early break stops iteration.
	n := 0
	for range q.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Empty queue yields nothing.
	empty := newTestQueue()
	for range empty.All() {
		t.Errorf("Expected no items from All on empty queue")
	}
}

// TestBackwardIterator verifies Backward is the exact reverse of All,
// with matching indexes.
func TestBackwardIterator(t *testing.T) {
	q := newTestQueue()
	for _, s := range []string{"05", "02", "09", "00", "03", "07", "01"} {
		q.Insert(TestItem{S: s})
	}

	var fwd []string
	for _, v := range q.All() {
		fwd = append(fwd, v.S)
	}
	var bwd []string
	for i, v := range q.Backward() {
		if i != q.Len()-1-len(bwd) {
			t.Errorf("Backward index %d, expected %d", i, q.Len()-1-len(bwd))
		}
		bwd = append(bwd, v.S)
	}
	if len(bwd) != len(fwd) {
		t.Fatalf("Backward visited %d items, All visited %d", len(bwd), len(fwd))
	}
	for i := range fwd {
		if fwd[i] != bwd[len(bwd)-1-i] {
			t.Fatalf("Backward is not the reverse of All: %v vs %v", fwd, bwd)
		}
	}

	// Early break.
	n := 0
	for range q.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Empty queue yields nothing.
	empty := newTestQueue()
	for range empty.Backward() {
		t.Errorf("Expected no items from Backward on empty queue")
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value, nil queue
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewBinomialQueue.
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
	if c := Compare(2.5, 2.25); c != 1 {
		t.Errorf("Compare(2.5,2.25) = %d, expected +1", c)
	}
}

// TestNewBinomialQueueOrdered verifies the constructor for naturally
// ordered key types.
func TestNewBinomialQueueOrdered(t *testing.T) {
	q := NewBinomialQueue[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		q.Insert(s)
	}
	if v, found := q.Peek(); !found || v != "apple" {
		t.Errorf("Peek = (%q, %v), expected apple", v, found)
	}

	nums := NewBinomialQueue[float64]()
	for _, f := range []float64{2.5, 1.5, 3.5} {
		nums.Insert(f)
	}
	if v, found := nums.Peek(); !found || v != 1.5 {
		t.Errorf("Peek = (%v, %v), expected 1.5", v, found)
	}
}

// TestNewBinomialQueueFunc verifies the constructor with a caller
// supplied comparison function, including a reversed (max-first) one and
// ordering by a struct field.
func TestNewBinomialQueueFunc(t *testing.T) {
	byS := NewBinomialQueueFunc(cmpTestItem)
	byS.Insert(TestItem{S: "b"})
	byS.Insert(TestItem{S: "a"})
	if v, found := byS.Peek(); !found || v.S != "a" {
		t.Errorf("Expected min a with function ordering, got (%v, %v)", v, found)
	}

	// A reversed comparison turns the min-first queue into a max-first queue.
	maxQ := NewBinomialQueueFunc(func(a, b int) int { return -Compare(a, b) })
	for _, v := range []int{5, 1, 9, 3} {
		maxQ.Insert(v)
		checkInvariants(t, maxQ, func(a, b int) int { return -Compare(a, b) })
	}
	var got []int
	for {
		v, found := maxQ.DeleteMin()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{9, 5, 3, 1}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Max-first drain got %v, expected %v", got, expect)
	}
}

// TestNewBinomialQueueFuncNil verifies that a nil comparison function is
// rejected at construction time, not on first use.
func TestNewBinomialQueueFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewBinomialQueueFunc(nil) to panic.")
		}
	}()
	NewBinomialQueueFunc[TestItem](nil)
}

// TestZeroValueQueue verifies that the zero value of BinomialQueue
// behaves as an empty queue for every non-insert operation and that
// Insert fails loudly because no comparison function has been set.
func TestZeroValueQueue(t *testing.T) {
	var q BinomialQueue[TestItem]

	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected zero value queue to have length 0.")
	}
	if !q.IsEmpty() {
		t.Errorf("Expected zero value queue to be empty.")
	}
	if _, found := q.DeleteMin(); found {
		t.Errorf("Expected DeleteMin on zero value queue to report false.")
	}
	if _, found := q.Peek(); found {
		t.Errorf("Expected Peek on zero value queue to report false.")
	}
	if _, found := q.FindMin(); found {
		t.Errorf("Expected FindMin on zero value queue to report false.")
	}
	q.Truncate() // no-op, must not panic
	q.Merge(nil) // no-op, must not panic
	for range q.All() {
		t.Errorf("Expected no values from All on zero value queue.")
	}
	for range q.Backward() {
		t.Errorf("Expected no values from Backward on zero value queue.")
	}
	var buf bytes.Buffer
	q.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on zero value queue.")
	}

	// Insert without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Insert on zero value queue to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBinomialQueue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		q.Insert(TestItem{S: "x"})
	}()
}

// TestNilQueueTolerated verifies that every non-insert operation treats
// a nil queue as an empty queue, and that Insert panics with a message
// naming the method — the package's only panics on calls.
func TestNilQueueTolerated(t *testing.T) {
	var q *BinomialQueue[TestItem]

	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected nil queue to have length 0.")
	}
	if !q.IsEmpty() {
		t.Errorf("Expected nil queue to be empty.")
	}
	if _, found := q.DeleteMin(); found {
		t.Errorf("Expected DeleteMin on nil queue to report false.")
	}
	if _, found := q.Peek(); found {
		t.Errorf("Expected Peek on nil queue to report false.")
	}
	if _, found := q.FindMin(); found {
		t.Errorf("Expected FindMin on nil queue to report false.")
	}
	q.Truncate() // no-op
	q.Merge(nil) // no-op
	for range q.All() {
		t.Errorf("Expected no values from All on nil queue.")
	}
	for range q.Backward() {
		t.Errorf("Expected no values from Backward on nil queue.")
	}
	var buf bytes.Buffer
	q.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on nil queue.")
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Insert on nil queue to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		q.Insert(TestItem{S: "x"})
	}()
}

// TestTruncateReuse verifies that the queue is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	q := newTestQueue()
	for _, s := range []string{"a", "b", "c"} {
		q.Insert(TestItem{S: s})
	}
	q.Truncate()
	if q.trees != nil {
		t.Errorf("Expected nil forest after Truncate.")
	}
	if q.Len() != 0 || !q.IsEmpty() {
		t.Errorf("Expected empty queue after Truncate.")
	}

	q.Insert(TestItem{S: "z"})
	q.Insert(TestItem{S: "a"})
	if v, found := q.Peek(); !found || v.S != "a" {
		t.Errorf("Peek after Truncate = (%v, %v), expected a", v, found)
	}
	checkInvariants(t, q, cmpTestItem)

	// Truncating an already-empty queue is fine.
	q.Truncate()
	q.Truncate()
	if q.Len() != 0 {
		t.Errorf("Expected empty queue after double Truncate.")
	}
}

// TestDump verifies the debugging output.
func TestDump(t *testing.T) {
	q := newTestQueue()
	var buf bytes.Buffer
	q.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty queue, got %q", buf.String())
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		q.Insert(TestItem{S: s})
	}
	buf.Reset()
	q.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1+5 {
		t.Fatalf("Expected 1 header + 5 element lines, got %d: %q", len(lines), out)
	}
	if lines[0] != "BinomialQueue length=5 trees=2" {
		t.Errorf("Dump header: got %q", lines[0])
	}
	// Every element appears exactly once, and the roots ("00" and "03")
	// are the unindented lines.
	var roots int
	seen := map[string]int{}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, " ") {
			roots++
		}
		fields := strings.Fields(line)
		if len(fields) != 1 {
			t.Fatalf("Unexpected Dump line: %q", line)
		}
		seen[strings.Trim(fields[0], "{}")]++
	}
	if roots != 2 {
		t.Errorf("Expected 2 tree roots in Dump, got %d:\n%s", roots, out)
	}
	if expect := map[string]int{"S:00": 1, "S:02": 1, "S:03": 1, "S:05": 1, "S:09": 1}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("Dump elements got %v, expected %v", seen, expect)
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkQueueSize = 1000

func BenchmarkInsert(b *testing.B) {
	q := NewBinomialQueue[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.Len() >= benchmarkQueueSize {
			q.Truncate()
		}
		q.Insert(i)
	}
}

func BenchmarkDeleteMin(b *testing.B) {
	q := NewBinomialQueue[int]()
	for i := range benchmarkQueueSize {
		q.Insert(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.IsEmpty() {
			for j := range benchmarkQueueSize {
				q.Insert(j)
			}
		}
		if _, found := q.DeleteMin(); !found {
			b.Fatalf("DeleteMin: unexpectedly empty")
		}
	}
}

func BenchmarkInsertDeleteMin(b *testing.B) {
	q := NewBinomialQueue[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Insert(i)
		if _, found := q.DeleteMin(); !found {
			b.Fatalf("DeleteMin: unexpectedly empty")
		}
	}
}

func BenchmarkMerge(b *testing.B) {
	dst := NewBinomialQueue[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := NewBinomialQueue[int]()
		for j := 0; j < benchmarkQueueSize; j++ {
			src.Insert(j)
		}
		dst.Merge(src)
		dst.Truncate()
	}
}
