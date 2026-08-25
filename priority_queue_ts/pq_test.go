package priority_queue_ts

import (
	"fmt"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// PqTest is a "priority queue of int" test type.
type PqTest struct {
	value    string // The value of the item; arbitrary.
	priority int    // The priority of the item in the queue.
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*PqTest)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa PqTest) Compare(x comparable.Comparable) int {
	if bb, ok := x.(PqTest); ok {
		return aa.priority - bb.priority
	} else if bb, ok := x.(*PqTest); ok {
		return aa.priority - bb.priority
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
}

func newTestPQ(items ...PqTest) *PriorityQueue[PqTest] {
	pq := NewPriorityQueue[PqTest]()
	for i := range items {
		pq.Insert(&items[i])
	}
	return pq
}

func TestNewEmptyQueue(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()
	if pq == nil {
		t.Fatal("NewPriorityQueue returned nil")
	}
	if !pq.IsEmpty() {
		t.Error("new queue should be empty")
	}
	if pq.Len() != 0 {
		t.Errorf("Len() = %d, want 0", pq.Len())
	}
	if v := pq.Peek(); v != nil {
		t.Errorf("Peek() on empty queue = %v, want nil", v)
	}
	if v := pq.Pop(); v != nil {
		t.Errorf("Pop() on empty queue = %v, want nil", v)
	}
	for range pq.All() {
		t.Error("All() on empty queue should yield nothing")
	}
}

func TestInsertPeekPop(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "banana", priority: 3},
		PqTest{value: "apple", priority: 2},
		PqTest{value: "pear", priority: 4},
	)

	if pq.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", pq.Len())
	}
	if pq.IsEmpty() {
		t.Error("queue with 3 items should not be empty")
	}

	if top := pq.Peek(); top == nil || top.value != "apple" {
		t.Fatalf("Peek() = %v, want apple (priority 2)", top)
	}
	// Peek must not remove the element.
	if pq.Len() != 3 {
		t.Fatalf("Len() after Peek() = %d, want 3", pq.Len())
	}

	// Pop must return elements in ascending priority order (min-heap).
	want := []string{"apple", "banana", "pear"}
	for i, w := range want {
		got := pq.Pop()
		if got == nil {
			t.Fatalf("Pop() #%d returned nil, want %q", i, w)
		}
		if got.value != w {
			t.Errorf("Pop() #%d = %q, want %q", i, got.value, w)
		}
	}
	if !pq.IsEmpty() {
		t.Error("queue should be empty after popping all elements")
	}
	if v := pq.Pop(); v != nil {
		t.Errorf("Pop() on drained queue = %v, want nil", v)
	}
}

func TestSearch(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "banana", priority: 3},
		PqTest{value: "apple", priority: 2},
		PqTest{value: "pear", priority: 4},
	)

	find := PqTest{priority: 3}
	rv, pos, err := pq.Search(&find)
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if rv == nil || rv.value != "banana" {
		t.Errorf("Search() = %v, want banana", rv)
	}
	if pos < 0 || pos >= pq.Len() {
		t.Errorf("Search() pos = %d, want in range [0..%d)", pos, pq.Len())
	}

	missing := PqTest{priority: 99}
	rv, _, err = pq.Search(&missing)
	if err == nil {
		t.Error("Search() for missing element should return an error")
	}
	if rv != nil {
		t.Errorf("Search() for missing element = %v, want nil", rv)
	}
}

func TestUpdatePriority(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "banana", priority: 3},
		PqTest{value: "apple", priority: 2},
		PqTest{value: "pear", priority: 4},
	)

	// Demote "apple" to the lowest priority; "banana" becomes the new min.
	_, pos, err := pq.Search(&PqTest{priority: 2})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if ok := pq.UpdatePriority(pos, &PqTest{value: "apple", priority: 9}); !ok {
		t.Fatal("UpdatePriority() = false, want true for valid position")
	}
	if top := pq.Peek(); top == nil || top.value != "banana" {
		t.Errorf("Peek() after UpdatePriority = %v, want banana", top)
	}

	if ok := pq.UpdatePriority(-1, &PqTest{priority: 1}); ok {
		t.Error("UpdatePriority(-1) = true, want false")
	}
	if ok := pq.UpdatePriority(pq.Len(), &PqTest{priority: 1}); ok {
		t.Error("UpdatePriority(Len()) = true, want false")
	}
}

func TestDelete(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "banana", priority: 3},
		PqTest{value: "apple", priority: 2},
		PqTest{value: "pear", priority: 4},
		PqTest{value: "plum", priority: 1},
	)

	_, pos, err := pq.Search(&PqTest{priority: 3})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if err := pq.Delete(pos); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if pq.Len() != 3 {
		t.Fatalf("Len() after Delete() = %d, want 3", pq.Len())
	}

	// The deleted element must be gone and the rest must pop in order.
	want := []string{"plum", "apple", "pear"}
	for i, w := range want {
		got := pq.Pop()
		if got == nil || got.value != w {
			t.Fatalf("Pop() #%d = %v, want %q", i, got, w)
		}
	}

	// Out-of-range positions must return an error, not panic.
	if err := pq.Delete(0); err == nil {
		t.Error("Delete(0) on empty queue should return an out-of-range error")
	}
	pq.Insert(&PqTest{value: "fig", priority: 5})
	if err := pq.Delete(-1); err == nil {
		t.Error("Delete(-1) should return an error")
	}
	if err := pq.Delete(1); err == nil {
		t.Error("Delete(Len()) should return an error")
	}
	if err := pq.Delete(0); err != nil {
		t.Errorf("Delete(0) error = %v, want nil", err)
	}
	if !pq.IsEmpty() {
		t.Error("queue should be empty after deleting last element")
	}
}

func TestTruncate(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "banana", priority: 3},
		PqTest{value: "apple", priority: 2},
	)
	pq.Truncate()
	if !pq.IsEmpty() || pq.Len() != 0 {
		t.Errorf("after Truncate: Len() = %d, want 0", pq.Len())
	}
	if v := pq.Peek(); v != nil {
		t.Errorf("Peek() after Truncate = %v, want nil", v)
	}
	// Queue must still be usable after Truncate.
	pq.Insert(&PqTest{value: "fig", priority: 5})
	if pq.Len() != 1 {
		t.Errorf("Len() after Insert post-Truncate = %d, want 1", pq.Len())
	}
}

func TestAllIterator(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "banana", priority: 3},
		PqTest{value: "apple", priority: 2},
		PqTest{value: "pear", priority: 4},
		PqTest{value: "plum", priority: 1},
	)

	var got []string
	for item := range pq.All() {
		got = append(got, item.value)
	}
	want := []string{"plum", "apple", "banana", "pear"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("All() yielded %v, want %v", got, want)
	}

	// Iteration must be non-destructive.
	if pq.Len() != 4 {
		t.Errorf("Len() after All() = %d, want 4", pq.Len())
	}

	// Early break must stop iteration.
	count := 0
	for range pq.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("early break: iterated %d elements, want 1", count)
	}
}

// ExamplePriorityQueue is adapted from the classic container/heap priority
// queue example: items are inserted, one priority is updated, and the items
// come out in ascending priority order.
func ExamplePriorityQueue() {
	pq := NewPriorityQueue[PqTest]()
	pq.Insert(&PqTest{value: "banana", priority: 3})
	pq.Insert(&PqTest{value: "apple", priority: 2})
	pq.Insert(&PqTest{value: "pear", priority: 4})

	// Insert a new item and then lower its priority below all the others.
	item := &PqTest{value: "orange", priority: 1}
	pq.Insert(item)
	_, pos, err := pq.Search(&PqTest{priority: 1})
	if err != nil {
		panic(err)
	}
	pq.UpdatePriority(pos, &PqTest{value: "orange", priority: 5})

	// Take the items out; they arrive in increasing priority order.
	for !pq.IsEmpty() {
		item := pq.Pop()
		fmt.Printf("%.2d:%s ", item.priority, item.value)
	}
	// Output:
	// 02:apple 03:banana 04:pear 05:orange
}

func BenchmarkInsert(b *testing.B) {
	pq := NewPriorityQueue[PqTest]()
	items := make([]PqTest, b.N)
	for i := range items {
		items[i] = PqTest{value: "x", priority: i * 2654435761 % 1000003}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Insert(&items[i])
	}
}

func BenchmarkPop(b *testing.B) {
	pq := NewPriorityQueue[PqTest]()
	for i := 0; i < b.N; i++ {
		item := PqTest{value: "x", priority: i * 2654435761 % 1000003}
		pq.Insert(&item)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Pop()
	}
}

func BenchmarkPeek(b *testing.B) {
	pq := NewPriorityQueue[PqTest]()
	item := PqTest{value: "x", priority: 1}
	pq.Insert(&item)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Peek()
	}
}
