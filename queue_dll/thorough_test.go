package queue_dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, link-integrity checks,
// duplicate edge cases, value-copy semantics of Peek, live-walk
// semantics of the iterators, truncate reuse, and a fixed-seed
// randomized property test.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// checkModel verifies the queue's observable state against a reference
// model (a plain slice where the head is element 0).
func checkModel(t *testing.T, q *Queue[int], model []int) {
	t.Helper()
	if got, want := q.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := q.IsEmpty(), len(model) == 0; got != want {
		t.Fatalf("IsEmpty: expected %v, got %v", want, got)
	}
	if len(model) > 0 {
		head, err := q.Peek()
		if err != nil {
			t.Fatalf("Peek on non-empty queue returned error: %v", err)
		}
		if head != model[0] {
			t.Fatalf("Peek: expected %d, got %d", model[0], head)
		}
	}
	// All must iterate head to tail.  slices.Equal (not reflect.DeepEqual)
	// so that a nil result and an emptied model compare equal.
	var fwd []int
	for _, v := range q.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, model) {
		t.Fatalf("All: expected %v, got %v", model, fwd)
	}
	// Backward must iterate tail to head.
	var bwd []int
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	var wantBwd []int
	for _, m := range slices.Backward(model) {
		wantBwd = append(wantBwd, m)
	}
	if !slices.Equal(bwd, wantBwd) {
		t.Fatalf("Backward: expected %v, got %v", wantBwd, bwd)
	}
}

// checkLinks verifies the structural invariants of the doubly linked
// list: the count matches Length, prev/next pointers are bidirectional,
// and head.prev / tail.next are nil.  Call it after structural changes.
func checkLinks[T any](t *testing.T, q *Queue[T]) {
	t.Helper()

	// Walk forward from the head, verifying every prev pointer on the way.
	n := 0
	var last *queueElement[T]
	for p := q.head; p != nil; p = p.next {
		if p.prev != last {
			t.Fatalf("link %d: p.prev = %v, expected %v", n, p.prev, last)
		}
		last = p
		n++
	}
	if last != q.tail {
		t.Fatalf("forward walk ended at %v, tail is %v", last, q.tail)
	}
	if n != q.length {
		t.Fatalf("forward walk counted %d nodes, length is %d", n, q.length)
	}

	// Walk backward from the tail, verifying every next pointer.
	n = 0
	var first *queueElement[T]
	for p := q.tail; p != nil; p = p.prev {
		if p.next != first && first != nil {
			t.Fatalf("link %d: p.next = %v, expected %v", n, p.next, first)
		}
		first = p
		n++
	}
	if first != q.head {
		t.Fatalf("backward walk ended at %v, head is %v", first, q.head)
	}
	if n != q.length {
		t.Fatalf("backward walk counted %d nodes, length is %d", n, q.length)
	}
}

// TestSingleElement covers the single-element edge case for both push
// aliases: pushing one element and dequeuing it must leave an empty
// queue with nil head and tail.
func TestSingleElement(t *testing.T) {
	for _, tc := range []struct {
		name string
		push func(q *Queue[string], v string)
	}{
		{"Push", (*Queue[string]).Push},
		{"Enqueue", (*Queue[string]).Enqueue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var q Queue[string]
			tc.push(&q, "x")

			if q.Length() != 1 {
				t.Fatalf("Expected length 1 got %d", q.Length())
			}
			checkLinks(t, &q)

			// Peek sees the single element.
			if p, err := q.Peek(); err != nil || p != "x" {
				t.Errorf("Peek = (%q, %v)", p, err)
			}

			v, err := q.Dequeue()
			if err != nil || v != "x" {
				t.Errorf("Dequeue = (%q, %v)", v, err)
			}
			if !q.IsEmpty() {
				t.Errorf("Expected empty queue after dequeuing the single element")
			}
			if q.head != nil || q.tail != nil {
				t.Errorf("Expected nil head and tail after draining, got %v/%v", q.head, q.tail)
			}
			checkLinks(t, &q)
		})
	}
}

// TestDuplicateValues verifies the queue stores duplicate values without
// conflating them.
func TestDuplicateValues(t *testing.T) {
	var q Queue[int]
	for range 5 {
		q.Push(7)
	}
	if q.Length() != 5 {
		t.Errorf("Expected length 5 got %d", q.Length())
	}
	n := 0
	for _, v := range q.All() {
		if v != 7 {
			t.Errorf("Expected 7 got %d", v)
		}
		n++
	}
	if n != 5 {
		t.Errorf("All: expected 5 elements got %d", n)
	}
	for i := range 5 {
		if v, err := q.Dequeue(); err != nil || v != 7 {
			t.Errorf("Dequeue step %d = (%d, %v)", i, v, err)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing all duplicates")
	}
}

// TestPeekReturnsValue verifies that Peek returns an independent value:
// mutating it cannot affect the queue.
func TestPeekReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var q Queue[item]
	q.Push(item{S: "a", N: 1})
	q.Push(item{S: "b", N: 2})

	v, err := q.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	v.N = 99 // mutate the returned value

	if got, err := q.Peek(); err != nil || got.N != 1 {
		t.Errorf("Peek after mutation = (%v, %v), expected N=1 unaffected", got, err)
	}
	if q.Length() != 2 {
		t.Errorf("Expected the peek to leave the length at 2, got %d", q.Length())
	}
}

// TestPushDequeueInterleaved cross-checks against the model between
// every operation.
func TestPushDequeueInterleaved(t *testing.T) {
	var q Queue[int]
	var model []int

	for _, v := range []int{1, 2, 3} {
		q.Push(v)
		model = append(model, v)
		checkModel(t, &q, model)
	}

	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Fatalf("Dequeue = (%v, %v), expected 1", v, err)
	}
	model = model[1:]
	checkModel(t, &q, model)
	checkLinks(t, &q)

	if err := q.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	model = model[1:]
	checkModel(t, &q, model)
	checkLinks(t, &q)
}

// TestTruncateReuse verifies that the queue is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	var q Queue[int]
	for i := range 10 {
		q.Push(i)
	}
	q.Truncate()

	if !q.IsEmpty() || q.Length() != 0 {
		t.Errorf("Expected empty queue after Truncate.")
	}
	checkModel(t, &q, nil)
	checkLinks(t, &q)

	// Reusable after the drain.
	q.Push(7)
	if v, err := q.Peek(); err != nil || v != 7 {
		t.Errorf("Peek after Truncate = (%v, %v), expected 7", v, err)
	}

	// Truncating an already-empty queue is fine.
	q.Truncate()
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after double Truncate.")
	}
}

// TestIteratorReflectsLiveQueue is the opposite of a snapshot iterator:
// the All/Backward iterators walk the live list — All through the next
// pointers, Backward through the prev pointers — so modifications made
// between iterations are visible, and the queue must not be modified
// while an iterator is running.
func TestIteratorReflectsLiveQueue(t *testing.T) {
	var q Queue[int]
	q.Push(1)
	q.Push(2)

	var seen []int
	for _, v := range q.All() {
		seen = append(seen, v)
		break // stop after the head element
	}
	if expect := []int{1}; !reflect.DeepEqual(seen, expect) {
		t.Fatalf("First visit: expected %v got %v", expect, seen)
	}

	// Push, then iterate again: the new tail is visible.
	q.Push(3)
	seen = nil
	for _, v := range q.All() {
		seen = append(seen, v)
	}
	if expect := []int{1, 2, 3}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("All after push: expected %v got %v", expect, seen)
	}

	if _, err := q.Dequeue(); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	seen = nil
	for _, v := range q.Backward() {
		seen = append(seen, v)
	}
	if expect := []int{3, 2}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("Backward after Dequeue: expected %v got %v", expect, seen)
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against
// a slice reference model with a fixed seed, cross-checking every
// observable result (length, peek, dequeue order, iteration order)
// along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260826, 7))
	const ops = 4000
	const keySpace = 50

	var q Queue[int]
	var model []int

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(4) {
		case 0, 1, 2: // Push (weighted so the queue grows)
			q.Push(v)
			model = append(model, v)
		case 3: // Dequeue
			got, err := q.Dequeue()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: Dequeue on empty returned %v", step, err)
				}
			} else {
				if err != nil || got != model[0] {
					t.Fatalf("step %d: Dequeue = (%v, %v), model head %d", step, got, err, model[0])
				}
				model = model[1:]
			}
		}
		if step%50 == 0 {
			checkModel(t, &q, model)
			checkLinks(t, &q)
		}
	}
	checkModel(t, &q, model)
	checkLinks(t, &q)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// jsonUpperString is a string with its own JSON representation, to
// verify that element-level marshalers are honored through the queue.
type jsonUpperString string

func (u jsonUpperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *jsonUpperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = jsonUpperString(s)
	return nil
}

// jsonTestItem is a plain struct element, to verify that struct field
// encoding is honored through the queue.
type jsonTestItem struct {
	S string
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, head (next to be dequeued) to tail.
	var q Queue[int]
	for _, v := range []int{3, 1, 2} {
		q.Push(v)
	}
	b, err := json.Marshal(&q)
	if err != nil {
		t.Fatalf("json.Marshal(queue): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	var items Queue[jsonTestItem]
	items.Push(jsonTestItem{S: "a"})
	items.Push(jsonTestItem{S: "b"})
	if b, err := json.Marshal(&items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty queue encodes as [].
	var empty Queue[int]
	if b, err := json.Marshal(&empty); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty queue, got (%s, %v)", b, err)
	}

	// A direct call on a nil queue encodes as []; json.Marshal on a nil
	// *Queue never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilQueue *Queue[int]
	if b, err := nilQueue.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-queue call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilQueue); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil queue, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	var custom Queue[jsonUpperString]
	custom.Push("x")
	custom.Push("y")
	if b, err := json.Marshal(&custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	var bad Queue[chan int]
	bad.Push(make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a queue of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the head, the next
	// element returned by Dequeue.
	var q Queue[int]
	if err := json.Unmarshal([]byte("[3,1,2]"), &q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []int
	for _, v := range q.All() {
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[3 1 2]" {
		t.Errorf("Expected [3 1 2], got %v", got)
	}
	if head, err := q.Peek(); err != nil || head != 3 {
		t.Errorf("Expected head 3, got (%v, %v)", head, err)
	}

	// A round trip rebuilds a structurally sound queue.
	var items Queue[jsonTestItem]
	for _, s := range []string{"a", "b", "c"} {
		items.Push(jsonTestItem{S: s})
	}
	b, err := json.Marshal(&items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Queue[jsonTestItem]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkLinks(t, &again)
	var values []string
	for _, v := range again.All() {
		values = append(values, v.S)
	}
	if got, want := fmt.Sprint(values), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte("[7]"), &q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := q.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the queue.
	var full Queue[jsonTestItem]
	full.Push(jsonTestItem{S: "z"})
	if err := json.Unmarshal([]byte("[]"), &full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the queue.")
	}
	full.Push(jsonTestItem{S: "z"})
	if err := json.Unmarshal([]byte("null"), &full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the queue.")
	}
	checkLinks(t, &full)

	// Element-level unmarshalers are honored.
	var custom Queue[jsonUpperString]
	if err := json.Unmarshal([]byte(`["X","Y"]`), &custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for _, v := range custom.All() {
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the queue untouched.
	var keep Queue[jsonTestItem]
	keep.Push(jsonTestItem{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		var vals []string
		for _, v := range keep.All() {
			vals = append(vals, v.S)
		}
		if got, want := fmt.Sprint(vals), "[keep]"; got != want {
			t.Errorf("Queue changed after the error on %s: %s", badData, got)
		}
	}
	checkLinks(t, &keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil queue panics with a message naming
// the method, while [] and null — which store nothing — are tolerated
// everywhere.  Unlike dll, the zero value is a usable queue (there is no
// equality function to be missing), so storing into a zero-value queue
// works.
func TestUnmarshalJSONPanics(t *testing.T) {
	// A zero-value queue is usable: unmarshaling elements into it works.
	var zero Queue[jsonTestItem]
	if err := zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)); err != nil {
		t.Errorf("Expected a zero-value queue to accept elements, got %v", err)
	}
	if got, want := zero.Len(), 1; got != want {
		t.Errorf("Expected length %d on the zero-value queue, got %d", want, got)
	}
	checkLinks(t, &zero)

	var nilQueue *Queue[jsonTestItem]
	for _, data := range []string{"[]", "null"} {
		if err := nilQueue.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil queue to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil queue.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil queue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilQueue.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()
}

// TestJSONStructField marshals and unmarshals a Queue nested in a struct
// through the encoding/json package.  Because the zero value of Queue is
// usable, even a nil *Queue field — which the json package allocates as
// a zero-value queue itself — unmarshals without any constructor.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string      `json:"title"`
		Jobs  *Queue[int] `json:"jobs"`
	}

	doc := Doc{Title: "run", Jobs: &Queue[int]{}}
	doc.Jobs.Push(10)
	doc.Jobs.Push(20)

	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if got, want := string(b), `{"title":"run","jobs":[10,20]}`; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}

	var back Doc
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back.Jobs == nil {
		t.Fatalf("Expected the json package to allocate the queue field.")
	}
	var got []int
	for _, v := range back.Jobs.All() {
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[10 20]" {
		t.Errorf("Expected [10 20], got %v", got)
	}
	checkLinks(t, back.Jobs)
}
