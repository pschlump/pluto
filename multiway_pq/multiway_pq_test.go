package multiway_pq

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestDHeapItem is the test element type.  Ordering is supplied to the
// heap as a plain function (cmpTestDHeapItem below).
type TestDHeapItem struct {
	S string
}

// cmpTestDHeapItem orders TestDHeapItem by its S field.
func cmpTestDHeapItem(a, b TestDHeapItem) int {
	return strings.Compare(a.S, b.S)
}

// newTestDHeap builds a 4-ary min-heap of TestDHeapItem ordered by S.
func newTestDHeap() *DHeap[TestDHeapItem] {
	return NewDHeapFunc(4, cmpTestDHeapItem)
}

// checkHeapInvariant verifies the min-heap property (every node sorts at
// or before all of its d children) and the length.
func checkHeapInvariant[T any](t *testing.T, dh *DHeap[T], cmp func(a, b T) int) {
	t.Helper()
	n := len(dh.data)
	d := dh.D()
	for i := 1; i < n; i++ {
		if cmp(dh.data[i], dh.data[(i-1)/d]) < 0 {
			t.Fatalf("heap invariant violated: data[%d]=%v sorts before its parent data[%d]=%v",
				i, dh.data[i], (i-1)/d, dh.data[(i-1)/d])
		}
	}
	if dh.Len() != n || dh.Length() != n {
		t.Fatalf("Len/Length %d/%d do not match internal size %d", dh.Len(), dh.Length(), n)
	}
}

func TestNewDHeap(t *testing.T) {
	for _, d := range []int{2, 3, 4, 8, 16, 64} {
		dh := NewDHeapFunc(d, Compare[int])
		if dh == nil {
			t.Fatalf("NewDHeapFunc(d=%d) returned nil.", d)
		}
		if !dh.IsEmpty() {
			t.Errorf("d=%d: expected empty heap.", d)
		}
		if dh.Len() != 0 || dh.Length() != 0 {
			t.Errorf("d=%d: expected length 0, got %d/%d", d, dh.Len(), dh.Length())
		}
		if _, found := dh.Peek(); found {
			t.Errorf("d=%d: expected Peek on empty heap to report false.", d)
		}
		if got := dh.D(); got != d {
			t.Errorf("D() = %d, expected %d", got, d)
		}
	}
}

func TestPushAndPop(t *testing.T) {
	for _, d := range []int{2, 3, 4, 8, 16} {
		dh := NewDHeapFunc(d, cmpTestDHeapItem)

		for _, s := range []string{"05", "02", "09", "00", "03"} {
			dh.Push(TestDHeapItem{S: s})
			checkHeapInvariant(t, dh, cmpTestDHeapItem)
		}

		if v, found := dh.Peek(); !found || v.S != "00" {
			t.Errorf("d=%d: Peek = (%v, %v), expected 00", d, v, found)
		}

		// Pops come out in sorted order.
		for _, want := range []string{"00", "02", "03", "05", "09"} {
			v, found := dh.Pop()
			if !found {
				t.Fatalf("d=%d: Pop: unexpectedly empty.", d)
			}
			if v.S != want {
				t.Errorf("d=%d: Pop = %s, expected %s", d, v.S, want)
			}
			checkHeapInvariant(t, dh, cmpTestDHeapItem)
		}
		if _, found := dh.Pop(); found {
			t.Errorf("d=%d: expected Pop on drained heap to report false.", d)
		}
	}
}

func TestSearch(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		dh.Push(TestDHeapItem{S: s})
	}

	for _, s := range []string{"00", "02", "03", "05", "09"} {
		v, pos, found := dh.Search(TestDHeapItem{S: s})
		if !found || pos < 0 || pos >= dh.Len() {
			t.Errorf("Search(%s) = (%v, %d, %v)", s, v, pos, found)
		} else if v.S != s {
			t.Errorf("Search(%s) returned %s", s, v.S)
		}
	}

	if _, pos, found := dh.Search(TestDHeapItem{S: "42"}); found || pos != -1 {
		t.Errorf("Search(42) = (%d, %v), expected (-1, false)", pos, found)
	}
}

// TestSearchProbePartialFields verifies the Search probe only needs the
// fields the comparison function reads.
func TestSearchProbePartialFields(t *testing.T) {
	type task struct {
		priority int
		name     string
	}
	dh := NewDHeapFunc(4, func(a, b task) int { return Compare(a.priority, b.priority) })
	for _, n := range []string{"a", "b", "c"} {
		dh.Push(task{priority: 3, name: n})
	}
	if v, pos, found := dh.Search(task{priority: 3}); !found || pos < 0 || v.name == "" {
		t.Errorf("Search by priority-only probe = (%v, %d, %v)", v, pos, found)
	}
	if _, _, found := dh.Search(task{priority: 4}); found {
		t.Errorf("Expected Search by absent priority to report false.")
	}
}

func TestWithDifferentElements(t *testing.T) {
	// A heap of ints with the natural ordering.
	dh := NewDHeap[int](3)
	for _, v := range []int{42, 7, 13, 99, 55, 0} {
		dh.Push(v)
		checkHeapInvariant(t, dh, Compare[int])
	}
	var got []int
	for {
		v, found := dh.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{0, 7, 13, 42, 55, 99}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain got %v, expected %v", got, expect)
	}
}

func TestPopPeekOnEmpty(t *testing.T) {
	dh := newTestDHeap()

	if _, found := dh.Pop(); found {
		t.Errorf("Expected Pop on empty heap to report false.")
	}
	if _, found := dh.Peek(); found {
		t.Errorf("Expected Peek on empty heap to report false.")
	}
	// Still usable.
	dh.Push(TestDHeapItem{S: "x"})
	if v, found := dh.Pop(); !found || v.S != "x" {
		t.Errorf("Pop after empty-cycle = (%v, %v)", v, found)
	}
}

func TestPeek(t *testing.T) {
	dh := newTestDHeap()
	dh.Push(TestDHeapItem{S: "05"})
	dh.Push(TestDHeapItem{S: "02"})
	dh.Push(TestDHeapItem{S: "09"})

	// Peek does not remove.
	for i := range 3 {
		if v, found := dh.Peek(); !found || v.S != "02" {
			t.Errorf("Peek step %d = (%v, %v), expected 02", i, v, found)
		}
	}
	if dh.Len() != 3 {
		t.Errorf("Expected Peek to leave the length at 3, got %d", dh.Len())
	}
}

func TestDelete(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		dh.Push(TestDHeapItem{S: s})
	}

	// Delete(0) behaves exactly like Pop.
	if v, found := dh.Delete(0); !found || v.S != "00" {
		t.Errorf("Delete(0) = (%v, %v), expected 00", v, found)
	}
	checkHeapInvariant(t, dh, cmpTestDHeapItem)

	// Delete an interior index; the multiset must be preserved.
	before := dh.Len()
	if _, found := dh.Delete(2); !found {
		t.Errorf("Expected Delete(2) to report true.")
	}
	if dh.Len() != before-1 {
		t.Errorf("Expected length %d after delete, got %d", before-1, dh.Len())
	}
	checkHeapInvariant(t, dh, cmpTestDHeapItem)

	// Out-of-range indexes report false.
	for _, bad := range []int{-1, dh.Len(), dh.Len() + 5} {
		if _, found := dh.Delete(bad); found {
			t.Errorf("Expected Delete(%d) to report false.", bad)
		}
	}

	// Drain the rest; the remaining elements come out in sorted order.
	var got []string
	for {
		v, found := dh.Pop()
		if !found {
			break
		}
		got = append(got, v.S)
	}
	// The five pushes leave {02, 03, 05, 09} after Delete(0); the 4-ary
	// internal layout puts 09 at index 2, so Delete(2) removes it and
	// {02, 03, 05} remain.
	if expect := []string{"02", "03", "05"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain after deletes got %v, expected %v", got, expect)
	}
}

func TestFixAndSetValue(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		dh.Push(TestDHeapItem{S: s})
	}

	// Raise the minimum to a large value; it must sink.
	if !dh.Fix(0, TestDHeapItem{S: "99"}) {
		t.Fatalf("Expected Fix(0) to report true.")
	}
	checkHeapInvariant(t, dh, cmpTestDHeapItem)
	if v, found := dh.Peek(); !found || v.S != "02" {
		t.Errorf("Peek after raise = (%v, %v), expected 02", v, found)
	}

	// Sink a large element to the minimum; it must rise.
	if _, idx, found := dh.Search(TestDHeapItem{S: "99"}); found {
		if !dh.SetValue(idx, TestDHeapItem{S: "00"}) {
			t.Fatalf("Expected SetValue(%d) to report true.", idx)
		}
		checkHeapInvariant(t, dh, cmpTestDHeapItem)
		if v, found := dh.Peek(); !found || v.S != "00" {
			t.Errorf("Peek after sink = (%v, %v), expected 00", v, found)
		}
	} else {
		t.Fatalf("Expected to find 99 in the heap.")
	}

	// Out-of-range Fix/SetValue report false and change nothing.
	for _, bad := range []int{-1, dh.Len()} {
		if dh.Fix(bad, TestDHeapItem{S: "xx"}) {
			t.Errorf("Expected Fix(%d) to report false.", bad)
		}
		if dh.SetValue(bad, TestDHeapItem{S: "xx"}) {
			t.Errorf("Expected SetValue(%d) to report false.", bad)
		}
	}
	checkHeapInvariant(t, dh, cmpTestDHeapItem)
}

func TestGetValue(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"05", "02", "09"} {
		dh.Push(TestDHeapItem{S: s})
	}

	for i := 0; i < dh.Len(); i++ {
		if _, found := dh.GetValue(i); !found {
			t.Errorf("Expected GetValue(%d) to report true.", i)
		}
	}
	for _, bad := range []int{-1, dh.Len()} {
		if _, found := dh.GetValue(bad); found {
			t.Errorf("Expected GetValue(%d) to report false.", bad)
		}
	}
}

func TestAppendHeapAndHeapify(t *testing.T) {
	for _, d := range []int{2, 3, 4, 8} {
		dh := NewDHeapFunc(d, cmpTestDHeapItem)
		dh.Push(TestDHeapItem{S: "05"})

		dh.AppendHeap([]TestDHeapItem{{S: "02"}, {S: "09"}, {S: "00"}, {S: "03"}})
		if dh.Len() != 5 {
			t.Fatalf("d=%d: expected length 5 after AppendHeap, got %d", d, dh.Len())
		}

		// Rebuild the heap with the d-ary rebuild loop.
		for i := (dh.Len() - 2) / dh.D(); i >= 0; i-- {
			dh.Heapify(dh.Len(), i)
		}
		checkHeapInvariant(t, dh, cmpTestDHeapItem)

		var got []string
		for {
			v, found := dh.Pop()
			if !found {
				break
			}
			got = append(got, v.S)
		}
		if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(got, expect) {
			t.Errorf("d=%d: drain after rebuild got %v, expected %v", d, got, expect)
		}
	}
}

// TestAppendHeapInterleaved verifies that each AppendHeap batch can be
// followed by ordinary Pushes and another rebuild — the batches may be
// interleaved, each rebuild fixing the combined data.
func TestAppendHeapInterleaved(t *testing.T) {
	dh := NewDHeap[int](4)
	dh.AppendHeap([]int{5, 1, 9})
	dh.Push(3) // a lone Push between batches re-establishes order for its own element
	dh.AppendHeap([]int{7, 0})
	for i := (dh.Len() - 2) / dh.D(); i >= 0; i-- {
		dh.Heapify(dh.Len(), i)
	}
	checkHeapInvariant(t, dh, Compare[int])

	var got []int
	for {
		v, found := dh.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{0, 1, 3, 5, 7, 9}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain got %v, expected %v", got, expect)
	}
}

func TestAllIterator(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		dh.Push(TestDHeapItem{S: s})
	}

	// All yields internal order, exactly matching the slice.
	var got []string
	for v := range dh.All() {
		got = append(got, v.S)
	}
	if len(got) != dh.Len() {
		t.Fatalf("All visited %d items, heap has %d", len(got), dh.Len())
	}
	for i, s := range got {
		if dh.data[i].S != s {
			t.Errorf("All[%d] = %s, internal data[%d] = %s", i, s, i, dh.data[i].S)
		}
	}

	// Early break stops iteration.
	n := 0
	for range dh.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Empty heap yields nothing.
	empty := newTestDHeap()
	for range empty.All() {
		t.Errorf("Expected no items from All on empty heap")
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value, nil heap
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewDHeap.
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

// TestNewDHeapOrdered verifies the constructor for naturally ordered key
// types.
func TestNewDHeapOrdered(t *testing.T) {
	dh := NewDHeap[string](4)
	for _, s := range []string{"pear", "apple", "fig"} {
		dh.Push(s)
	}
	if v, found := dh.Peek(); !found || v != "apple" {
		t.Errorf("Peek = (%q, %v), expected apple", v, found)
	}

	nums := NewDHeap[float64](2)
	for _, f := range []float64{2.5, 1.5, 3.5} {
		nums.Push(f)
	}
	if v, found := nums.Peek(); !found || v != 1.5 {
		t.Errorf("Peek = (%v, %v), expected 1.5", v, found)
	}
}

// TestNewDHeapFunc verifies the constructor with a caller supplied
// comparison function, including a reversed (max-heap) one and ordering
// by a struct field.
func TestNewDHeapFunc(t *testing.T) {
	byS := NewDHeapFunc(3, cmpTestDHeapItem)
	byS.Push(TestDHeapItem{S: "b"})
	byS.Push(TestDHeapItem{S: "a"})
	if v, found := byS.Peek(); !found || v.S != "a" {
		t.Errorf("Expected min a with function ordering, got (%v, %v)", v, found)
	}

	// A reversed comparison turns the min-heap into a max-heap.
	maxHeap := NewDHeapFunc(4, func(a, b int) int { return -Compare(a, b) })
	for _, v := range []int{5, 1, 9, 3} {
		maxHeap.Push(v)
		checkHeapInvariant(t, maxHeap, func(a, b int) int { return -Compare(a, b) })
	}
	var got []int
	for {
		v, found := maxHeap.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{9, 5, 3, 1}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Max-heap drain got %v, expected %v", got, expect)
	}
}

// expectPanic runs fn and verifies it panics with exactly wantMsg.
func expectPanic(t *testing.T, wantMsg string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected panic %q, got none.", wantMsg)
			return
		}
		if msg, ok := r.(string); !ok || msg != wantMsg {
			t.Errorf("Panic message:\n got  %q\n want %q", r, wantMsg)
		}
	}()
	fn()
}

// TestPanicMessages pins every contracted panic message exactly.
func TestPanicMessages(t *testing.T) {
	expectPanic(t, "multiway_pq: NewDHeapFunc called with a nil comparison function",
		func() { NewDHeapFunc[int](4, nil) })

	for _, d := range []int{1, 0, -3} {
		expectPanic(t, "multiway_pq: NewDHeap/NewDHeapFunc called with d < 2 (a heap needs at least two children per node)",
			func() { NewDHeap[int](d) })
		expectPanic(t, "multiway_pq: NewDHeap/NewDHeapFunc called with d < 2 (a heap needs at least two children per node)",
			func() { NewDHeapFunc(d, Compare[int]) })
	}

	var nilHeap *DHeap[int]
	expectPanic(t, "multiway_pq: Push called on a nil heap", func() { nilHeap.Push(1) })
	expectPanic(t, "multiway_pq: AppendHeap called on a nil heap", func() { nilHeap.AppendHeap([]int{1}) })
	expectPanic(t, "multiway_pq: UnmarshalJSON called on a nil heap",
		func() { _ = nilHeap.UnmarshalJSON([]byte("[1]")) })

	var zero DHeap[int]
	expectPanic(t, "multiway_pq: Push called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)",
		func() { zero.Push(1) })
	expectPanic(t, "multiway_pq: AppendHeap called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)",
		func() { zero.AppendHeap([]int{1}) })
	expectPanic(t, "multiway_pq: UnmarshalJSON called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)",
		func() { _ = zero.UnmarshalJSON([]byte("[1]")) })
}

// TestNewDHeapFuncNil verifies that a nil comparison function is rejected
// at construction time, not on first use.
func TestNewDHeapFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewDHeapFunc(nil) to panic.")
		}
	}()
	NewDHeapFunc[TestDHeapItem](4, nil)
}

// TestNewDHeapDSmall verifies that d < 2 is rejected at construction
// time — there is no such thing as a 1-ary heap.
func TestNewDHeapDSmall(t *testing.T) {
	for _, d := range []int{1, 0, -3} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewDHeap(d=%d) to panic.", d)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "d < 2") {
					t.Errorf("Unexpected panic message: %v", r)
				}
			}()
			NewDHeap[int](d)
		}()
	}
}

// TestZeroValueHeap verifies that the zero value of DHeap behaves as an
// empty heap for every non-insert operation and that Push fails loudly
// because no comparison function has been set.
func TestZeroValueHeap(t *testing.T) {
	var dh DHeap[TestDHeapItem]

	if dh.Len() != 0 || dh.Length() != 0 {
		t.Errorf("Expected zero value heap to have length 0.")
	}
	if _, found := dh.Pop(); found {
		t.Errorf("Expected Pop on zero value heap to report false.")
	}
	if _, found := dh.Peek(); found {
		t.Errorf("Expected Peek on zero value heap to report false.")
	}
	if _, _, found := dh.Search(TestDHeapItem{S: "x"}); found {
		t.Errorf("Expected Search on zero value heap to report false.")
	}
	if _, found := dh.Delete(0); found {
		t.Errorf("Expected Delete on zero value heap to report false.")
	}
	if _, found := dh.GetValue(0); found {
		t.Errorf("Expected GetValue on zero value heap to report false.")
	}
	if dh.Fix(0, TestDHeapItem{S: "x"}) {
		t.Errorf("Expected Fix on zero value heap to report false.")
	}
	dh.Truncate() // no-op, must not panic
	dh.Heapify(0, 0)
	if got := dh.D(); got != 2 {
		t.Errorf("D() on zero value heap = %d, expected 2 (the tolerated fallback)", got)
	}
	for range dh.All() {
		t.Errorf("Expected no values from All on zero value heap.")
	}

	// Push without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Push on zero value heap to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDHeap") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		dh.Push(TestDHeapItem{S: "x"})
	}()

	// AppendHeap likewise.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected AppendHeap on zero value heap to panic.")
			}
		}()
		dh.AppendHeap([]TestDHeapItem{{S: "x"}})
	}()
}

// TestNilHeapTolerated verifies that every non-insert operation treats a
// nil heap as an empty heap, and that Push panics with a message naming
// the method — the package's only panics on calls.
func TestNilHeapTolerated(t *testing.T) {
	var dh *DHeap[TestDHeapItem]

	if dh.Len() != 0 || dh.Length() != 0 {
		t.Errorf("Expected nil heap to have length 0.")
	}
	if _, found := dh.Pop(); found {
		t.Errorf("Expected Pop on nil heap to report false.")
	}
	if _, found := dh.Peek(); found {
		t.Errorf("Expected Peek on nil heap to report false.")
	}
	if _, _, found := dh.Search(TestDHeapItem{S: "x"}); found {
		t.Errorf("Expected Search on nil heap to report false.")
	}
	if _, found := dh.Delete(0); found {
		t.Errorf("Expected Delete on nil heap to report false.")
	}
	if _, found := dh.GetValue(0); found {
		t.Errorf("Expected GetValue on nil heap to report false.")
	}
	if dh.Fix(0, TestDHeapItem{S: "x"}) {
		t.Errorf("Expected Fix on nil heap to report false.")
	}
	dh.Lock()     // no-op
	dh.Truncate() // no-op
	dh.Unlock()   // no-op
	dh.Heapify(0, 0)
	for range dh.All() {
		t.Errorf("Expected no values from All on nil heap.")
	}
	var buf bytes.Buffer
	dh.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on nil heap.")
	}
	if got := dh.D(); got != 2 {
		t.Errorf("D() on nil heap = %d, expected 2 (the tolerated fallback)", got)
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Push on nil heap to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Push") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		dh.Push(TestDHeapItem{S: "x"})
	}()
}

// TestLockUnlockNoop verifies the compatibility shims exist.
func TestLockUnlockNoop(t *testing.T) {
	dh := newTestDHeap()
	dh.Lock()
	dh.Push(TestDHeapItem{S: "a"}) // the shims are no-ops, so Push works inside the "critical section"
	dh.Unlock()
	if dh.Len() != 1 {
		t.Errorf("Expected length 1, got %d", dh.Len())
	}
}

// TestTruncateReuse verifies that the heap is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"a", "b", "c"} {
		dh.Push(TestDHeapItem{S: s})
	}
	dh.Truncate()
	if dh.data != nil {
		t.Errorf("Expected nil backing array after Truncate.")
	}
	if dh.Len() != 0 {
		t.Errorf("Expected empty heap after Truncate.")
	}

	dh.Push(TestDHeapItem{S: "z"})
	dh.Push(TestDHeapItem{S: "a"})
	if v, found := dh.Peek(); !found || v.S != "a" {
		t.Errorf("Peek after Truncate = (%v, %v), expected a", v, found)
	}
	checkHeapInvariant(t, dh, cmpTestDHeapItem)

	// Truncating an already-empty heap is fine.
	dh.Truncate()
	dh.Truncate()
	if dh.Len() != 0 {
		t.Errorf("Expected empty heap after double Truncate.")
	}
}

// TestPushSameValueRepeatedly verifies duplicates coexist and pop in
// sorted order.
func TestPushSameValueRepeatedly(t *testing.T) {
	dh := NewDHeap[int](8)
	for range 10 {
		dh.Push(7)
		checkHeapInvariant(t, dh, Compare[int])
	}
	for i := range 10 {
		if v, found := dh.Pop(); !found || v != 7 {
			t.Fatalf("Pop %d = (%v, %v), expected 7", i, v, found)
		}
	}
	if _, found := dh.Pop(); found {
		t.Errorf("Expected drained heap.")
	}
}

// TestPopReleasesBackingArray verifies the memory contract: a full drain
// releases the backing array rather than holding a zero-length window
// into it.
func TestPopReleasesBackingArray(t *testing.T) {
	dh := NewDHeap[int](4)
	dh.Push(1)
	dh.Push(2)
	dh.Pop()
	if _, found := dh.Pop(); !found {
		t.Fatalf("Expected the second Pop to report true.")
	}
	if dh.data != nil {
		t.Errorf("Expected nil backing array after a full drain, got %v", dh.data)
	}
}

func TestDump(t *testing.T) {
	dh := newTestDHeap()
	var buf bytes.Buffer
	dh.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty heap, got %q", buf.String())
	}

	dh.Push(TestDHeapItem{S: "b"})
	dh.Push(TestDHeapItem{S: "a"})
	buf.Reset()
	dh.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1+2 {
		t.Fatalf("Expected 1 header + 2 element lines, got %d: %q", len(lines), out)
	}
	if lines[0] != "DHeap(d=4) length=2" {
		t.Errorf("Dump header: expected %q, got %q", "DHeap(d=4) length=2", lines[0])
	}
	// Internal order: "a" is the root, "b" its child.
	if lines[1] != "0: {S:a}" || lines[2] != "1: {S:b}" {
		t.Errorf("Dump elements unexpected: %q %q", lines[1], lines[2])
	}
}

func TestSearchDuplicates(t *testing.T) {
	dh := NewDHeap[int](4)
	for _, v := range []int{5, 3, 5, 1, 5} {
		dh.Push(v)
	}
	for i := range 3 {
		if _, pos, found := dh.Search(5); !found || pos < 0 {
			t.Errorf("Search(5) #%d = (%d, %v)", i, pos, found)
		}
	}
	if _, pos, found := dh.Search(4); found || pos != -1 {
		t.Errorf("Search(4) = (%d, %v), expected (-1, false)", pos, found)
	}
}

func TestSearchOnEmpty(t *testing.T) {
	dh := newTestDHeap()
	if v, pos, found := dh.Search(TestDHeapItem{S: "x"}); found || pos != -1 {
		t.Errorf("Search on empty = (%v, %d, %v)", v, pos, found)
	}
}

// TestDeleteZeroIsPop verifies Delete(0) and Pop agree.
func TestDeleteZeroIsPop(t *testing.T) {
	dh := newTestDHeap()
	for _, s := range []string{"05", "02", "09"} {
		dh.Push(TestDHeapItem{S: s})
	}
	v1, f1 := dh.Peek()
	v2, f2 := dh.Delete(0)
	if !f1 || !f2 || v1 != v2 {
		t.Errorf("Delete(0) = (%v, %v) but Peek = (%v, %v)", v2, f2, v1, f1)
	}
	checkHeapInvariant(t, dh, cmpTestDHeapItem)
}

// TestSingleElementOps exercises every operation on a one-element heap.
func TestSingleElementOps(t *testing.T) {
	dh := newTestDHeap()
	dh.Push(TestDHeapItem{S: "only"})

	if v, found := dh.Peek(); !found || v.S != "only" {
		t.Errorf("Peek = (%v, %v)", v, found)
	}
	if _, pos, found := dh.Search(TestDHeapItem{S: "only"}); !found || pos != 0 {
		t.Errorf("Search = (%d, %v)", pos, found)
	}
	if v, found := dh.GetValue(0); !found || v.S != "only" {
		t.Errorf("GetValue(0) = (%v, %v)", v, found)
	}
	if _, found := dh.Delete(0); !found {
		t.Errorf("Expected Delete(0) on single-element heap to report true.")
	}
	if dh.data != nil {
		t.Errorf("Expected nil backing array after draining.")
	}
}

// TestHeapifyRebuildWithDuplicates verifies the AppendHeap + Heapify
// rebuild with many duplicates.
func TestHeapifyRebuildWithDuplicates(t *testing.T) {
	for _, d := range []int{2, 4, 8} {
		dh := NewDHeap[int](d)
		var bulk []int
		for i := range 100 {
			bulk = append(bulk, i%5) // 20 of each of 0..4
		}
		dh.AppendHeap(bulk)
		for i := (dh.Len() - 2) / dh.D(); i >= 0; i-- {
			dh.Heapify(dh.Len(), i)
		}
		checkHeapInvariant(t, dh, Compare[int])

		var got []int
		for {
			v, found := dh.Pop()
			if !found {
				break
			}
			got = append(got, v)
		}
		var expect []int
		for v := range 5 {
			for range 20 {
				expect = append(expect, v)
			}
		}
		if !reflect.DeepEqual(got, expect) {
			t.Errorf("d=%d: drain after rebuild mismatch (len %d vs %d)", d, len(got), len(expect))
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the heap.
type upperString string

func (u upperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperString(s)
	return nil
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, internal heap order: pushing 3, 1, 2 into a
	// 4-ary heap leaves the internal slice as [1 3 2].
	dh := NewDHeap[int](4)
	for _, v := range []int{3, 1, 2} {
		dh.Push(v)
	}
	b, err := json.Marshal(dh)
	if err != nil {
		t.Fatalf("json.Marshal(heap): %v", err)
	}
	if string(b) != "[1,3,2]" {
		t.Errorf("Expected [1,3,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := newTestDHeap()
	items.Push(TestDHeapItem{S: "b"})
	items.Push(TestDHeapItem{S: "a"})
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty heap encodes as [].
	if b, err := json.Marshal(NewDHeap[int](4)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty heap, got (%s, %v)", b, err)
	}

	// A zero-value heap is a tolerated read: [].
	var zero DHeap[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value heap, got (%s, %v)", b, err)
	}

	// A direct call on a nil heap encodes as []; json.Marshal on a nil
	// *DHeap never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilHeap *DHeap[int]
	if b, err := nilHeap.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-heap call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilHeap); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil heap, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewDHeap[upperString](4)
	custom.Push("y")
	custom.Push("x")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewDHeapFunc(4, func(a, b chan int) int { return 0 })
	bad.Push(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a heap of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded elements form a valid heap that pops in sorted order.
	dh := NewDHeap[int](4)
	if err := json.Unmarshal([]byte("[3,1,2]"), dh); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkHeapInvariant(t, dh, Compare[int])
	var got []int
	for {
		v, found := dh.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("Expected [1 2 3], got %v", got)
	}

	// A round trip rebuilds a structurally sound heap and keeps the
	// comparison function (Search works on the rebuilt heap).
	items := newTestDHeap()
	for _, s := range []string{"c", "a", "b"} {
		items.Push(TestDHeapItem{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestDHeap()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkHeapInvariant(t, again, cmpTestDHeapItem)
	if v, found := again.Peek(); !found || v.S != "a" {
		t.Errorf("Expected min a after round trip, got (%v, %v)", v, found)
	}
	if _, pos, found := again.Search(TestDHeapItem{S: "b"}); !found || pos < 0 {
		t.Errorf("Expected Search to work after unmarshal, got pos %d", pos)
	}

	// Unmarshaling replaces the contents; it does not append.
	dh = NewDHeap[int](4)
	dh.Push(1)
	dh.Push(2)
	if err := json.Unmarshal([]byte("[7]"), dh); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := dh.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the heap.
	full := newTestDHeap()
	full.Push(TestDHeapItem{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the heap.")
	}
	full.Push(TestDHeapItem{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the heap.")
	}
	if full.data != nil {
		t.Errorf("Expected nil backing array after clearing with null.")
	}

	// Element-level unmarshalers are honored.
	custom := NewDHeap[upperString](4)
	if err := json.Unmarshal([]byte(`["Y","X"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for {
		v, found := custom.Pop()
		if !found {
			break
		}
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the heap untouched.
	keep := newTestDHeap()
	keep.Push(TestDHeapItem{S: "keep"})
	for _, badData := range []string{"[1,", `[1]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := keep.Len(), 1; got != want {
			t.Errorf("Heap changed after the error on %s: length %d, want %d", badData, got, want)
		}
		if v, found := keep.Peek(); !found || v.S != "keep" {
			t.Errorf("Heap changed after the error on %s: Peek = (%v, %v)", badData, v, found)
		}
	}
}

// TestJSONStructField marshals and unmarshals a DHeap nested in a struct
// through the encoding/json package.  The heap must be created with
// NewDHeap/NewDHeapFunc before unmarshaling: for a nil *DHeap field the
// json package allocates a zero-value heap itself (no comparison
// function), so non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string      `json:"title"`
		Tags  *DHeap[int] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewDHeap[int](4)}
	d.Tags.Push(3)
	d.Tags.Push(1)
	d.Tags.Push(2)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":[1,3,2]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created heap field.
	var out Doc
	out.Tags = NewDHeap[int](4)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := out.Tags.Len(), 3; got != want {
		t.Fatalf("Expected 3 tags, got %d", got)
	}
	if v, found := out.Tags.Peek(); !found || v != 1 {
		t.Errorf("Expected min tag 1, got (%v, %v)", v, found)
	}

	// A nil heap field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created heap and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewDHeap[int](4)}
	clearDoc.Tags.Push(9)
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the heap.")
	}

	// Non-empty data into a nil *DHeap field: the json package allocates
	// a zero-value heap, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated heap field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDHeap") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":[1]}`), &bad)
	}()
}
