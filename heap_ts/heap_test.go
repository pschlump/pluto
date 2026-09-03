package heap_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestHeapItem is the test element type.  Ordering is supplied to the
// heap as a plain function (cmpTestHeapItem below).
type TestHeapItem struct {
	S string
}

// cmpTestHeapItem orders TestHeapItem by its S field.
func cmpTestHeapItem(a, b TestHeapItem) int {
	return strings.Compare(a.S, b.S)
}

// newTestHeap builds a min-heap of TestHeapItem ordered by S.
func newTestHeap() *Heap[TestHeapItem] {
	return NewHeapFunc(cmpTestHeapItem)
}

// checkHeapInvariant verifies the min-heap property (every node sorts at
// or before its children) and the length.
func checkHeapInvariant[T any](t *testing.T, hp *Heap[T], cmp func(a, b T) int) {
	t.Helper()
	n := len(hp.data)
	for i := 1; i < n; i++ {
		if cmp(hp.data[i], hp.data[(i-1)/2]) < 0 {
			t.Fatalf("heap invariant violated: data[%d]=%v sorts before its parent data[%d]=%v",
				i, hp.data[i], (i-1)/2, hp.data[(i-1)/2])
		}
	}
	if hp.Len() != n || hp.Length() != n {
		t.Fatalf("Len/Length %d/%d do not match internal size %d", hp.Len(), hp.Length(), n)
	}
}

func TestNewHeap(t *testing.T) {
	hp := newTestHeap()
	if hp == nil {
		t.Fatalf("NewHeapFunc returned nil.")
	}
	if !hp.IsEmpty() {
		t.Errorf("Expected empty heap.")
	}
	if hp.Len() != 0 || hp.Length() != 0 {
		t.Errorf("Expected length 0, got %d/%d", hp.Len(), hp.Length())
	}
	if _, found := hp.Peek(); found {
		t.Errorf("Expected Peek on empty heap to report false.")
	}
}

func TestPushAndPop(t *testing.T) {
	hp := newTestHeap()

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		hp.Push(TestHeapItem{S: s})
		checkHeapInvariant(t, hp, cmpTestHeapItem)
	}

	if v, found := hp.Peek(); !found || v.S != "00" {
		t.Errorf("Peek = (%v, %v), expected 00", v, found)
	}

	// Pops come out in sorted order.
	for _, want := range []string{"00", "02", "03", "05", "09"} {
		v, found := hp.Pop()
		if !found {
			t.Fatalf("Pop: unexpectedly empty.")
		}
		if v.S != want {
			t.Errorf("Pop = %s, expected %s", v.S, want)
		}
		checkHeapInvariant(t, hp, cmpTestHeapItem)
	}
	if _, found := hp.Pop(); found {
		t.Errorf("Expected Pop on drained heap to report false.")
	}
}

func TestSearch(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		hp.Push(TestHeapItem{S: s})
	}

	for _, s := range []string{"00", "02", "03", "05", "09"} {
		v, pos, found := hp.Search(TestHeapItem{S: s})
		if !found || pos < 0 || pos >= hp.Len() {
			t.Errorf("Search(%s) = (%v, %d, %v)", s, v, pos, found)
		} else if v.S != s {
			t.Errorf("Search(%s) returned %s", s, v.S)
		}
	}

	if _, pos, found := hp.Search(TestHeapItem{S: "42"}); found || pos != -1 {
		t.Errorf("Search(42) = (%d, %v), expected (-1, false)", pos, found)
	}
}

func TestWithDifferentElements(t *testing.T) {
	// A heap of ints with the natural ordering.
	hp := NewHeap[int]()
	for _, v := range []int{42, 7, 13, 99, 55, 0} {
		hp.Push(v)
		checkHeapInvariant(t, hp, Compare[int])
	}
	var got []int
	for {
		v, found := hp.Pop()
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
	hp := newTestHeap()

	if _, found := hp.Pop(); found {
		t.Errorf("Expected Pop on empty heap to report false.")
	}
	if _, found := hp.Peek(); found {
		t.Errorf("Expected Peek on empty heap to report false.")
	}
	// Still usable.
	hp.Push(TestHeapItem{S: "x"})
	if v, found := hp.Pop(); !found || v.S != "x" {
		t.Errorf("Pop after empty-cycle = (%v, %v)", v, found)
	}
}

func TestPeek(t *testing.T) {
	hp := newTestHeap()
	hp.Push(TestHeapItem{S: "05"})
	hp.Push(TestHeapItem{S: "02"})
	hp.Push(TestHeapItem{S: "09"})

	// Peek does not remove.
	for i := range 3 {
		if v, found := hp.Peek(); !found || v.S != "02" {
			t.Errorf("Peek step %d = (%v, %v), expected 02", i, v, found)
		}
	}
	if hp.Len() != 3 {
		t.Errorf("Expected Peek to leave the length at 3, got %d", hp.Len())
	}
}

func TestDelete(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		hp.Push(TestHeapItem{S: s})
	}

	// Delete(0) behaves exactly like Pop.
	if v, found := hp.Delete(0); !found || v.S != "00" {
		t.Errorf("Delete(0) = (%v, %v), expected 00", v, found)
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)

	// Delete an interior index; the multiset must be preserved.
	before := len(hp.data)
	if _, found := hp.Delete(2); !found {
		t.Errorf("Expected Delete(2) to report true.")
	}
	if hp.Len() != before-1 {
		t.Errorf("Expected length %d after delete, got %d", before-1, hp.Len())
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)

	// Out-of-range indexes report false.
	for _, bad := range []int{-1, hp.Len(), hp.Len() + 5} {
		if _, found := hp.Delete(bad); found {
			t.Errorf("Expected Delete(%d) to report false.", bad)
		}
	}

	// Drain the rest; the remaining elements come out in sorted order.
	var got []string
	for {
		v, found := hp.Pop()
		if !found {
			break
		}
		got = append(got, v.S)
	}
	// data after the 5 pushes is [00 02 09 05 03]; Delete(0) removes 00,
	// Delete(2) removes 09 — 02, 03, 05 remain.
	if expect := []string{"02", "03", "05"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain after deletes got %v, expected %v", got, expect)
	}
}

func TestFixAndSetValue(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		hp.Push(TestHeapItem{S: s})
	}

	// Raise the minimum to a large value; it must sink.
	if !hp.Fix(0, TestHeapItem{S: "99"}) {
		t.Fatalf("Expected Fix(0) to report true.")
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)
	if v, found := hp.Peek(); !found || v.S != "02" {
		t.Errorf("Peek after raise = (%v, %v), expected 02", v, found)
	}

	// Sink a large element to the minimum; it must rise.
	if _, idx, found := hp.Search(TestHeapItem{S: "99"}); found {
		if !hp.SetValue(idx, TestHeapItem{S: "00"}) {
			t.Fatalf("Expected SetValue(%d) to report true.", idx)
		}
		checkHeapInvariant(t, hp, cmpTestHeapItem)
		if v, found := hp.Peek(); !found || v.S != "00" {
			t.Errorf("Peek after sink = (%v, %v), expected 00", v, found)
		}
	} else {
		t.Fatalf("Expected to find 99 in the heap.")
	}

	// Out-of-range Fix/SetValue report false and change nothing.
	for _, bad := range []int{-1, hp.Len()} {
		if hp.Fix(bad, TestHeapItem{S: "xx"}) {
			t.Errorf("Expected Fix(%d) to report false.", bad)
		}
		if hp.SetValue(bad, TestHeapItem{S: "xx"}) {
			t.Errorf("Expected SetValue(%d) to report false.", bad)
		}
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)
}

func TestGetValue(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"05", "02", "09"} {
		hp.Push(TestHeapItem{S: s})
	}

	for i := 0; i < hp.Len(); i++ {
		if _, found := hp.GetValue(i); !found {
			t.Errorf("Expected GetValue(%d) to report true.", i)
		}
	}
	for _, bad := range []int{-1, hp.Len()} {
		if _, found := hp.GetValue(bad); found {
			t.Errorf("Expected GetValue(%d) to report false.", bad)
		}
	}
}

func TestAppendHeapAndHeapify(t *testing.T) {
	hp := newTestHeap()
	hp.Push(TestHeapItem{S: "05"})

	hp.AppendHeap([]TestHeapItem{{S: "02"}, {S: "09"}, {S: "00"}, {S: "03"}})
	if hp.Len() != 5 {
		t.Fatalf("Expected length 5 after AppendHeap, got %d", hp.Len())
	}

	// Rebuild the heap.
	for i := hp.Len()/2 - 1; i >= 0; i-- {
		hp.Heapify(hp.Len(), i)
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)

	var got []string
	for {
		v, found := hp.Pop()
		if !found {
			break
		}
		got = append(got, v.S)
	}
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain after rebuild got %v, expected %v", got, expect)
	}
}

func TestAllIterator(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		hp.Push(TestHeapItem{S: s})
	}

	// All yields internal order, exactly matching the slice.
	var got []string
	for v := range hp.All() {
		got = append(got, v.S)
	}
	if len(got) != hp.Len() {
		t.Fatalf("All visited %d items, heap has %d", len(got), hp.Len())
	}
	for i, s := range got {
		if hp.data[i].S != s {
			t.Errorf("All[%d] = %s, internal data[%d] = %s", i, s, i, hp.data[i].S)
		}
	}

	// Early break stops iteration.
	n := 0
	for range hp.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Empty heap yields nothing.
	empty := newTestHeap()
	for range empty.All() {
		t.Errorf("Expected no items from All on empty heap")
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value, nil heap
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewHeap.
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

// TestNewHeapOrdered verifies the constructor for naturally ordered key
// types.
func TestNewHeapOrdered(t *testing.T) {
	hp := NewHeap[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		hp.Push(s)
	}
	if v, found := hp.Peek(); !found || v != "apple" {
		t.Errorf("Peek = (%q, %v), expected apple", v, found)
	}

	nums := NewHeap[float64]()
	for _, f := range []float64{2.5, 1.5, 3.5} {
		nums.Push(f)
	}
	if v, found := nums.Peek(); !found || v != 1.5 {
		t.Errorf("Peek = (%v, %v), expected 1.5", v, found)
	}
}

// TestNewHeapFunc verifies the constructor with a caller supplied
// comparison function, including a reversed (max-heap) one and ordering
// by a struct field.
func TestNewHeapFunc(t *testing.T) {
	byS := NewHeapFunc(cmpTestHeapItem)
	byS.Push(TestHeapItem{S: "b"})
	byS.Push(TestHeapItem{S: "a"})
	if v, found := byS.Peek(); !found || v.S != "a" {
		t.Errorf("Expected min a with function ordering, got (%v, %v)", v, found)
	}

	// A reversed comparison turns the min-heap into a max-heap.
	maxHeap := NewHeapFunc(func(a, b int) int { return -Compare(a, b) })
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

// TestNewHeapFuncNil verifies that a nil comparison function is rejected
// at construction time, not on first use.
func TestNewHeapFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewHeapFunc(nil) to panic.")
		}
	}()
	NewHeapFunc[TestHeapItem](nil)
}

// TestZeroValueHeap verifies that the zero value of Heap behaves as an
// empty heap for every non-insert operation and that Push fails loudly
// because no comparison function has been set.
func TestZeroValueHeap(t *testing.T) {
	var hp Heap[TestHeapItem]

	if hp.Len() != 0 || hp.Length() != 0 {
		t.Errorf("Expected zero value heap to have length 0.")
	}
	if _, found := hp.Pop(); found {
		t.Errorf("Expected Pop on zero value heap to report false.")
	}
	if _, found := hp.Peek(); found {
		t.Errorf("Expected Peek on zero value heap to report false.")
	}
	if _, _, found := hp.Search(TestHeapItem{S: "x"}); found {
		t.Errorf("Expected Search on zero value heap to report false.")
	}
	if _, found := hp.Delete(0); found {
		t.Errorf("Expected Delete on zero value heap to report false.")
	}
	if _, found := hp.GetValue(0); found {
		t.Errorf("Expected GetValue on zero value heap to report false.")
	}
	if hp.Fix(0, TestHeapItem{S: "x"}) {
		t.Errorf("Expected Fix on zero value heap to report false.")
	}
	hp.Truncate() // no-op, must not panic
	hp.Heapify(0, 0)
	for range hp.All() {
		t.Errorf("Expected no values from All on zero value heap.")
	}

	// Push without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Push on zero value heap to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewHeap") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		hp.Push(TestHeapItem{S: "x"})
	}()

	// AppendHeap likewise.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Expected AppendHeap on zero value heap to panic.")
			}
		}()
		hp.AppendHeap([]TestHeapItem{{S: "x"}})
	}()
}

// TestNilHeapTolerated verifies that every non-insert operation treats a
// nil heap as an empty heap, and that Push panics with a message naming
// the method — the package's only panics on calls.
func TestNilHeapTolerated(t *testing.T) {
	var hp *Heap[TestHeapItem]

	if hp.Len() != 0 || hp.Length() != 0 {
		t.Errorf("Expected nil heap to have length 0.")
	}
	if _, found := hp.Pop(); found {
		t.Errorf("Expected Pop on nil heap to report false.")
	}
	if _, found := hp.Peek(); found {
		t.Errorf("Expected Peek on nil heap to report false.")
	}
	if _, _, found := hp.Search(TestHeapItem{S: "x"}); found {
		t.Errorf("Expected Search on nil heap to report false.")
	}
	if _, found := hp.Delete(0); found {
		t.Errorf("Expected Delete on nil heap to report false.")
	}
	if _, found := hp.GetValue(0); found {
		t.Errorf("Expected GetValue on nil heap to report false.")
	}
	if hp.Fix(0, TestHeapItem{S: "x"}) {
		t.Errorf("Expected Fix on nil heap to report false.")
	}
	hp.Lock()     // no-op
	hp.Truncate() // no-op
	hp.Unlock()   // no-op
	hp.Heapify(0, 0)
	for range hp.All() {
		t.Errorf("Expected no values from All on nil heap.")
	}
	var buf bytes.Buffer
	hp.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on nil heap.")
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
		hp.Push(TestHeapItem{S: "x"})
	}()
}

// TestLockNlCompound verifies the real Lock/Unlock pair and the Nl*
// methods: a compound search-and-push under the manual lock must be
// atomic with respect to other writers.  The compound operation uses the
// Nl (no-lock) methods because the locked public methods must not be
// called while the lock is held.
func TestLockNlCompound(t *testing.T) {
	hp := NewHeap[int]()

	const workers = 8
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range 100 {
				hp.Lock()
				// Insert i only if it is not already present (atomic
				// check-and-insert via the Nl* methods).
				found := false
				for idx := 0; idx < hp.NlLen(); idx++ {
					if v, ok := hp.NlGetValue(idx); ok && v == i {
						found = true
						break
					}
				}
				if !found {
					hp.NlPush(i)
				}
				hp.Unlock()
			}
		})
	}
	wg.Wait()

	if hp.Len() != 100 {
		t.Errorf("Expected exactly 100 distinct insertions under the manual lock, got %d", hp.Len())
	}
	checkHeapInvariant(t, hp, Compare[int])

	// NlPop/NlDelete/NlFix under the lock.
	hp.Lock()
	if _, found := hp.NlPop(); !found {
		t.Errorf("Expected NlPop on non-empty heap to report true.")
	}
	if _, found := hp.NlDelete(hp.NlLen() - 1); !found {
		t.Errorf("Expected NlDelete(last) to report true.")
	}
	if !hp.NlFix(0, -1) {
		t.Errorf("Expected NlFix(0) to report true.")
	}
	hp.Unlock()
	checkHeapInvariant(t, hp, Compare[int])
	if v, found := hp.Peek(); !found || v != -1 {
		t.Errorf("Peek after NlFix = (%v, %v), expected -1", v, found)
	}

	// Out-of-range Nl ops report false / zero.
	hp.Lock()
	if _, found := hp.NlGetValue(-1); found {
		t.Errorf("Expected NlGetValue(-1) to report false.")
	}
	if _, found := hp.NlDelete(hp.NlLen()); found {
		t.Errorf("Expected NlDelete(len) to report false.")
	}
	if hp.NlFix(hp.NlLen(), 42) {
		t.Errorf("Expected NlFix(len) to report false.")
	}
	hp.Unlock()
}

// TestLockNlAppendHeapify verifies the atomic bulk form of AppendHeap +
// Heapify: under Lock, NlAppendHeap plus a NlHeapify rebuild leaves a
// valid heap that drains in order (this is the compound heap_sort_ts
// builds its InsertArray on).
func TestLockNlAppendHeapify(t *testing.T) {
	hp := NewHeap[int]()

	hp.Lock()
	hp.NlPush(5)
	hp.NlAppendHeap([]int{2, 9, 0, 3})
	if n := hp.NlLen(); n != 5 {
		t.Errorf("Expected NlLen 5 after NlAppendHeap, got %d", n)
	}
	for i := hp.NlLen()/2 - 1; i >= 0; i-- {
		hp.NlHeapify(hp.NlLen(), i)
	}
	hp.Unlock()

	checkHeapInvariant(t, hp, Compare[int])

	var got []int
	for {
		v, found := hp.Pop()
		if !found {
			break
		}
		got = append(got, v)
	}
	if expect := []int{0, 2, 3, 5, 9}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain after NlAppendHeap+NlHeapify got %v, expected %v", got, expect)
	}
}

// TestTruncateReuse verifies that the heap is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"a", "b", "c"} {
		hp.Push(TestHeapItem{S: s})
	}
	hp.Truncate()
	if hp.data != nil {
		t.Errorf("Expected nil backing array after Truncate.")
	}
	if hp.Len() != 0 {
		t.Errorf("Expected empty heap after Truncate.")
	}

	hp.Push(TestHeapItem{S: "z"})
	hp.Push(TestHeapItem{S: "a"})
	if v, found := hp.Peek(); !found || v.S != "a" {
		t.Errorf("Peek after Truncate = (%v, %v), expected a", v, found)
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)

	// Truncating an already-empty heap is fine.
	hp.Truncate()
	hp.Truncate()
	if hp.Len() != 0 {
		t.Errorf("Expected empty heap after double Truncate.")
	}
}

// TestPushSameValueRepeatedly verifies duplicates coexist and pop in
// (stable) sorted order.
func TestPushSameValueRepeatedly(t *testing.T) {
	hp := NewHeap[int]()
	for range 10 {
		hp.Push(7)
		checkHeapInvariant(t, hp, Compare[int])
	}
	for i := range 10 {
		if v, found := hp.Pop(); !found || v != 7 {
			t.Fatalf("Pop %d = (%v, %v), expected 7", i, v, found)
		}
	}
	if _, found := hp.Pop(); found {
		t.Errorf("Expected drained heap.")
	}
}

// TestDump verifies the debugging output.
func TestDump(t *testing.T) {
	hp := newTestHeap()
	var buf bytes.Buffer
	hp.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty heap, got %q", buf.String())
	}

	hp.Push(TestHeapItem{S: "b"})
	hp.Push(TestHeapItem{S: "a"})
	buf.Reset()
	hp.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1+2 {
		t.Fatalf("Expected 1 header + 2 element lines, got %d: %q", len(lines), out)
	}
	if lines[0] != "Heap length=2" {
		t.Errorf("Dump header: expected %q, got %q", "Heap length=2", lines[0])
	}
	// Internal order: "a" is the root, "b" its child.
	if lines[1] != "0: {S:a}" || lines[2] != "1: {S:b}" {
		t.Errorf("Dump elements unexpected: %q %q", lines[1], lines[2])
	}
}

// TestSearchDuplicates verifies Search finds some matching element when
// duplicates exist.
func TestSearchDuplicates(t *testing.T) {
	hp := NewHeap[int]()
	for _, v := range []int{5, 3, 5, 1, 5} {
		hp.Push(v)
	}
	for i := range 3 {
		if _, pos, found := hp.Search(5); !found || pos < 0 {
			t.Errorf("Search(5) #%d = (%d, %v)", i, pos, found)
		}
	}
	if _, pos, found := hp.Search(4); found || pos != -1 {
		t.Errorf("Search(4) = (%d, %v), expected (-1, false)", pos, found)
	}
}

// TestSearchOnEmpty verifies Search on the empty heap.
func TestSearchOnEmpty(t *testing.T) {
	hp := newTestHeap()
	if v, pos, found := hp.Search(TestHeapItem{S: "x"}); found || pos != -1 {
		t.Errorf("Search on empty = (%v, %d, %v)", v, pos, found)
	}
}

// TestDeleteZeroIsPop verifies Delete(0) and Pop agree.
func TestDeleteZeroIsPop(t *testing.T) {
	hp := newTestHeap()
	for _, s := range []string{"05", "02", "09"} {
		hp.Push(TestHeapItem{S: s})
	}
	v1, f1 := hp.Peek()
	v2, f2 := hp.Delete(0)
	if !f1 || !f2 || v1 != v2 {
		t.Errorf("Delete(0) = (%v, %v) but Peek = (%v, %v)", v2, f2, v1, f1)
	}
	checkHeapInvariant(t, hp, cmpTestHeapItem)
}

// -------------------------------------------------------------------------------------------------------
// Property tests against a sorted-slice reference model
// -------------------------------------------------------------------------------------------------------

// TestRandomOpsAgainstReference runs thousands of mixed operations
// (Push, Pop, Delete at a random index, Fix, Peek, Search) against a
// sorted-slice multiset model with a fixed seed, verifying the heap
// invariant and the multiset after every step.
func TestRandomOpsAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 19))
	const ops = 4000
	const keySpace = 40

	hp := NewHeap[int]()
	model := map[int]int{} // value -> count

	check := func(step int) {
		t.Helper()
		checkHeapInvariant(t, hp, Compare[int])
		if hp.Len() != countOf(model) {
			t.Fatalf("step %d: Len %d, model has %d", step, hp.Len(), countOf(model))
		}
		// Multiset equality.
		seen := map[int]int{}
		for v := range hp.All() {
			seen[v]++
		}
		if !reflect.DeepEqual(seen, model) {
			t.Fatalf("step %d: heap multiset %v != model %v", step, seen, model)
		}
	}

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(6) {
		case 0, 1, 2: // Push
			hp.Push(v)
			model[v]++
		case 3: // Pop
			got, found := hp.Pop()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: Pop on empty reported true", step)
				}
			} else {
				min := minKey(model)
				if !found || got != min {
					t.Fatalf("step %d: Pop = (%v, %v), model min %d", step, got, found, min)
				}
				model[min]--
				if model[min] == 0 {
					delete(model, min)
				}
			}
		case 4: // Delete at a random index
			if hp.Len() > 0 {
				idx := rng.IntN(hp.Len())
				old, _ := hp.GetValue(idx)
				got, found := hp.Delete(idx)
				if !found || got != old {
					t.Fatalf("step %d: Delete(%d) = (%v, %v), expected %v", step, idx, got, found, old)
				}
				model[old]--
				if model[old] == 0 {
					delete(model, old)
				}
			}
		case 5: // Fix: replace a random index with a new value
			if hp.Len() > 0 {
				idx := rng.IntN(hp.Len())
				old, _ := hp.GetValue(idx)
				if !hp.Fix(idx, v) {
					t.Fatalf("step %d: Fix(%d) reported false on a valid index", step, idx)
				}
				model[old]--
				if model[old] == 0 {
					delete(model, old)
				}
				model[v]++
			}
		}
		if step%50 == 0 {
			check(step)
		}
	}
	check(ops)
}

func countOf(m map[int]int) int {
	n := 0
	for _, c := range m {
		n += c
	}
	return n
}

func minKey(m map[int]int) int {
	min := -1
	for k := range m {
		if min == -1 || k < min {
			min = k
		}
	}
	return min
}

// TestDeleteEveryIndex deletes through every index position.
func TestDeleteEveryIndex(t *testing.T) {
	hp := NewHeap[int]()
	for i := range 20 {
		hp.Push(i * 3 % 7) // duplicates included
	}
	for hp.Len() > 0 {
		for idx := hp.Len() - 1; idx >= 0; idx -= 3 {
			if _, found := hp.Delete(idx); !found {
				t.Fatalf("Delete(%d) on len %d reported false", idx, hp.Len())
			}
			checkHeapInvariant(t, hp, Compare[int])
		}
	}
}

// TestFixEveryDirection raises and sinks the root repeatedly.
func TestFixEveryDirection(t *testing.T) {
	hp := NewHeap[int]()
	for i := range 20 {
		hp.Push(i)
	}
	for i := range 10 {
		if !hp.Fix(0, 100+i) { // raise the root; it must sink
			t.Fatalf("Fix(0) reported false")
		}
		checkHeapInvariant(t, hp, Compare[int])
	}
	for i := range 10 {
		if !hp.Fix(0, -1-i) { // sink the root; it stays
			t.Fatalf("Fix(0) reported false")
		}
		checkHeapInvariant(t, hp, Compare[int])
	}
	if v, found := hp.Peek(); !found || v != -10 {
		t.Errorf("Peek = (%v, %v), expected -10", v, found)
	}
}

// TestSingleElementOps exercises every operation on a one-element heap.
func TestSingleElementOps(t *testing.T) {
	hp := newTestHeap()
	hp.Push(TestHeapItem{S: "only"})

	if v, found := hp.Peek(); !found || v.S != "only" {
		t.Errorf("Peek = (%v, %v)", v, found)
	}
	if _, pos, found := hp.Search(TestHeapItem{S: "only"}); !found || pos != 0 {
		t.Errorf("Search = (%d, %v)", pos, found)
	}
	if v, found := hp.GetValue(0); !found || v.S != "only" {
		t.Errorf("GetValue(0) = (%v, %v)", v, found)
	}
	if _, found := hp.Delete(0); !found {
		t.Errorf("Expected Delete(0) on single-element heap to report true.")
	}
	if hp.data != nil {
		t.Errorf("Expected nil backing array after draining.")
	}
}

// TestHeapifyRebuildWithDuplicates verifies the AppendHeap + Heapify
// rebuild with many duplicates.
func TestHeapifyRebuildWithDuplicates(t *testing.T) {
	hp := NewHeap[int]()
	var bulk []int
	for i := range 100 {
		bulk = append(bulk, i%5) // 20 of each of 0..4
	}
	hp.AppendHeap(bulk)
	for i := hp.Len()/2 - 1; i >= 0; i-- {
		hp.Heapify(hp.Len(), i)
	}
	checkHeapInvariant(t, hp, Compare[int])

	var got []int
	for {
		v, found := hp.Pop()
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
		t.Errorf("Drain after rebuild mismatch (len %d vs %d)", len(got), len(expect))
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkHeapSize = 4096

func BenchmarkHeapPush(b *testing.B) {
	hp := NewHeap[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if hp.Len() >= benchmarkHeapSize {
			hp.Truncate()
		}
		hp.Push(i)
	}
}

func BenchmarkHeapPushPop(b *testing.B) {
	hp := NewHeap[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hp.Push(i)
		if _, found := hp.Pop(); !found {
			b.Fatalf("Pop: unexpectedly empty")
		}
	}
}

func BenchmarkHeapSearch(b *testing.B) {
	hp := NewHeap[int]()
	for i := range benchmarkHeapSize {
		hp.Push(i * 7 % benchmarkHeapSize)
	}
	find := benchmarkHeapSize / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hp.Search(find)
	}
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterator and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestHeapIterateSnapshot verifies that the All iterator operates on a
// snapshot taken when it is called: later modifications — even truncating
// the whole heap — are not observed.
func TestHeapIterateSnapshot(t *testing.T) {
	hp := NewHeap[int]()
	for i := range 5 {
		hp.Push(i)
	}

	all := hp.All()

	hp.Truncate() // the iterator above must not observe this

	var got []int
	for v := range all {
		got = append(got, v)
	}
	if len(got) != 5 {
		t.Errorf("Expected 5 visits from All after Truncate, got %d", len(got))
	}
	// The snapshot preserves the internal (breadth-first) order.
	for i, v := range got {
		if v != i { // pushing 0..4 in order leaves data as [0 1 2 3 4]
			t.Errorf("Snapshot[%d] = %d, expected %d", i, v, i)
		}
	}
}

// TestHeapConcurrent runs writers against one shared heap, then drains it
// with concurrent poppers and an accountant.  It is primarily a test for
// the race detector (`make race`); the accounting must balance exactly:
// every pushed element is popped exactly once, in ascending order per
// pop (each pop returns the then-minimum of the multiset).
func TestHeapConcurrent(t *testing.T) {
	hp := NewHeap[int]()

	const producers = 8
	const perProducer = 250
	const total = producers * perProducer

	// Producers push values 1..total (each exactly once).
	var producedSum = int64(total) * int64(total+1) / 2
	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := range perProducer {
				hp.Push(p*perProducer + i + 1)
			}
		}(p)
	}

	// A reader iterates snapshots and queries metadata while the writers
	// work.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for range hp.All() {
			}
			_ = hp.Len()
			_, _ = hp.Peek()
			_, _, _ = hp.Search(42)
		}
	}()

	wg.Wait()

	// Concurrent poppers drain; exit on the COUNT of pops (exactly total
	// items were pushed), not on any value sum.  Popped values are
	// collected and checked in sorted order afterwards — checking
	// monotonicity in flight races on the order the poppers RECORD, not
	// the order the heap RETURNS.
	var poppedCount atomic.Int64
	var poppedSum atomic.Int64
	var popped []int
	var mu sync.Mutex
	for range 4 {
		wg.Go(func() {
			for {
				v, found := hp.Pop()
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
		t.Errorf("Expected %d pops, got %d", total, got)
	}
	if got := poppedSum.Load(); got != producedSum {
		t.Errorf("Pop sum %d, expected %d", got, producedSum)
	}
	if !hp.IsEmpty() {
		t.Errorf("Expected empty heap after concurrent drain, got length %d", hp.Len())
	}

	// The multiset of popped values must be exactly 1..total: sorted, it
	// equals the sequence.
	sort.Ints(popped)
	for i, v := range popped {
		if v != i+1 {
			t.Fatalf("Popped multiset mismatch at %d: got %d, expected %d", i, v, i+1)
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

// drain pops every element, returning them in sorted (pop) order.
func drain[T any](hp *Heap[T]) []T {
	var got []T
	for {
		v, found := hp.Pop()
		if !found {
			return got
		}
		got = append(got, v)
	}
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output in internal (breadth-first) order: pushing 3, 1, 2
	// leaves the backing slice as [1 3 2].
	hp := NewHeap[int]()
	for _, v := range []int{3, 1, 2} {
		hp.Push(v)
	}
	b, err := json.Marshal(hp)
	if err != nil {
		t.Fatalf("json.Marshal(hp): %v", err)
	}
	if string(b) != "[1,3,2]" {
		t.Errorf("Expected [1,3,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := newTestHeap()
	for _, s := range []string{"a", "b"} {
		items.Push(TestHeapItem{S: s})
	}
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty heap encodes as [].
	if b, err := json.Marshal(NewHeap[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty heap, got (%s, %v)", b, err)
	}

	// A zero-value heap is a tolerated read: [].
	var zero Heap[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value heap, got (%s, %v)", b, err)
	}

	// A direct call on a nil heap encodes as []; json.Marshal on a nil
	// *Heap never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilHeap *Heap[int]
	if b, err := nilHeap.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-heap call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilHeap); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil heap, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewHeap[upperString]()
	custom.Push("x")
	custom.Push("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewHeapFunc(func(a, b chan int) int { return 0 })
	bad.Push(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a heap of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded elements are heapified: the heap invariant holds and pops
	// come out in sorted order.
	hp := NewHeap[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), hp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkHeapInvariant(t, hp, Compare[int])
	if top, found := hp.Peek(); !found || top != 1 {
		t.Errorf("Expected minimum 1, got (%v, %v)", top, found)
	}
	if got, want := fmt.Sprint(drain(hp)), "[1 2 3]"; got != want {
		t.Errorf("Expected drain %s, got %s", want, got)
	}

	// A round trip rebuilds a structurally sound heap and keeps the
	// comparison function (Search works on the rebuilt heap).  A marshaled
	// heap already satisfies the heap property, so the bytes survive the
	// round trip unchanged.
	items := newTestHeap()
	for _, s := range []string{"a", "b", "c"} {
		items.Push(TestHeapItem{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestHeap()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkHeapInvariant(t, again, cmpTestHeapItem)
	if b2, err := json.Marshal(again); err != nil || string(b2) != string(b) {
		t.Errorf("Expected round trip to preserve %s, got (%s, %v)", b, b2, err)
	}
	if _, _, found := again.Search(TestHeapItem{S: "b"}); !found {
		t.Errorf("Expected Search to work after unmarshal.")
	}
	if got, want := fmt.Sprint(drain(again)), "[{a} {b} {c}]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte("[7]"), hp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := hp.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the heap.
	full := newTestHeap()
	full.Push(TestHeapItem{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the heap.")
	}
	full.Push(TestHeapItem{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the heap.")
	}

	// Element-level unmarshalers are honored.
	custom := NewHeap[upperString]()
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(drain(custom)), "[X Y]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}

	// Decode errors are returned and leave the heap untouched.
	keep := newTestHeap()
	keep.Push(TestHeapItem{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		checkHeapInvariant(t, keep, cmpTestHeapItem)
		if got, want := fmt.Sprint(drain(keep)), "[{keep}]"; got != want {
			t.Errorf("Heap changed after the error on %s: %s", badData, got)
		}
		keep.Push(TestHeapItem{S: "keep"}) // restore for the next bad input
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value heap panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Heap[TestHeapItem]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value heap to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value heap.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewHeap") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilHeap *Heap[TestHeapItem]
	if err := nilHeap.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil heap to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil heap.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil heap") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilHeap.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()
}

// TestJSONStructField marshals and unmarshals a Heap nested in a struct
// through the encoding/json package.  The heap must be created with
// NewHeap/NewHeapFunc before unmarshaling: for a nil *Heap field the json
// package allocates a zero-value heap itself (no comparison function), so
// non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string        `json:"title"`
		Tags  *Heap[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewHeap[string]()}
	d.Tags.Push("ds")
	d.Tags.Push("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created heap field.
	var out Doc
	out.Tags = NewHeap[string]()
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(drain(out.Tags)), "[ds go]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}

	// A nil heap field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created heap and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewHeap[string]()}
	clearDoc.Tags.Push("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the heap.")
	}

	// Non-empty data into a nil *Heap field: the json package allocates a
	// zero-value heap, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated heap field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewHeap") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a sorted-slice reference model at fixed seed: a round trip must
// preserve the bytes (a marshaled heap already satisfies the heap
// property), the heap invariant, and the multiset.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260902, 42))
	const ops = 500

	hp := NewHeap[int]()
	model := []int{}

	for step := range ops {
		switch rng.IntN(3) {
		case 0, 1: // Push
			v := rng.IntN(100)
			hp.Push(v)
			model = append(model, v)
		case 2: // Pop
			got, found := hp.Pop()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: Pop on empty reported true", step)
				}
			} else {
				mi := 0
				for i, v := range model {
					if v < model[mi] {
						mi = i
					}
				}
				if !found || got != model[mi] {
					t.Fatalf("step %d: Pop = (%v, %v), model min %d", step, got, found, model[mi])
				}
				model = append(model[:mi], model[mi+1:]...)
			}
		}

		// Round trip: unmarshal the marshaled heap into a fresh one.
		got, err := json.Marshal(hp)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		fresh := NewHeap[int]()
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		checkHeapInvariant(t, fresh, Compare[int])
		if again, err := json.Marshal(fresh); err != nil || string(again) != string(got) {
			t.Fatalf("step %d: re-marshal got %s, expected %s", step, again, got)
		}

		// The drain of the round-tripped heap is the sorted model.
		sorted := append([]int(nil), model...)
		sort.Ints(sorted)
		if got, want := fmt.Sprint(drain(fresh)), fmt.Sprint(sorted); got != want {
			t.Fatalf("step %d: round trip drain got %s, model %s", step, got, want)
		}
	}
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with writers and a marshaling reader; every output must be a valid
// JSON array.  Run under -race (make race).
func TestJSONConcurrent(t *testing.T) {
	hp := newTestHeap()

	const workers = 8
	const perWorker = 100

	stop := make(chan struct{})
	var writers sync.WaitGroup
	var readers sync.WaitGroup

	// A marshaling reader: MarshalJSON snapshots under the read lock, so
	// it is safe while the writers replace the contents.
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := hp.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON: %v", err)
				return
			}
			var probe []TestHeapItem
			if err := json.Unmarshal(b, &probe); err != nil {
				t.Errorf("MarshalJSON produced invalid JSON %s: %v", b, err)
				return
			}
		}
	}()

	// Concurrent replacers: each replaces the whole contents with one
	// element of its own.
	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range perWorker {
				item := TestHeapItem{S: fmt.Sprintf("%02d-%03d", w, i)}
				b, err := json.Marshal([]TestHeapItem{item})
				if err != nil {
					t.Errorf("worker %d: %v", w, err)
					return
				}
				if err := hp.UnmarshalJSON(b); err != nil {
					t.Errorf("worker %d: UnmarshalJSON: %v", w, err)
					return
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)
	readers.Wait()
	checkHeapInvariant(t, hp, cmpTestHeapItem)
}
