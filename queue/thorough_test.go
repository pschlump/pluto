package queue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: single-element and duplicate edge cases, value-copy
// semantics, iterators after a partial drain, and a fixed-seed randomized
// FIFO property test cross-checked against a slice reference model.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// contents returns the current contents of the queue, head to tail.  It
// always returns a non-nil slice so empty-queue comparisons work with
// reflect.DeepEqual.
func contents[T any](q *Queue[T]) []T {
	got := []T{}
	for _, v := range q.All() {
		got = append(got, v)
	}
	return got
}

// TestSingleElement exercises every operation on a one-element queue.
func TestSingleElement(t *testing.T) {
	var q Queue[string]

	q.Push("only")
	if q.IsEmpty() || q.Length() != 1 {
		t.Errorf("Expected single-element queue, length %d", q.Length())
	}
	if v, err := q.Peek(); err != nil || v != "only" {
		t.Errorf("Peek = (%q, %v)", v, err)
	}
	if v, err := q.Dequeue(); err != nil || v != "only" {
		t.Errorf("Dequeue = (%q, %v)", v, err)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeueing the only element.")
	}
	if q.data != nil {
		t.Errorf("Expected nil backing array after draining.")
	}
}

// TestDuplicateValues verifies that duplicates coexist and dequeue in
// FIFO order.
func TestDuplicateValues(t *testing.T) {
	var q Queue[int]
	for _, v := range []int{5, 5, 5, 7} {
		q.Push(v)
	}
	for i, want := range []int{5, 5, 5, 7} {
		if v, err := q.Dequeue(); err != nil || v != want {
			t.Errorf("Dequeue step %d = (%v, %v), expected %d", i, v, err, want)
		}
	}
}

// TestDequeueReturnsValue verifies that the dequeued element is an
// independent value: mutating a dequeued struct does not affect the queue
// (or vice versa), because elements are stored and returned by value.
func TestDequeueReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var q Queue[item]
	q.Push(item{S: "a", N: 1})
	q.Push(item{S: "b", N: 2})

	v, err := q.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	v.N = 99 // mutate the returned value

	if rest := contents(&q); !reflect.DeepEqual(rest, []item{{S: "b", N: 2}}) {
		t.Errorf("Mutating a dequeued value affected the queue: %v", rest)
	}
	if next, err := q.Dequeue(); err != nil || next.N != 2 {
		t.Errorf("Dequeue = (%v, %v), expected N=2 unaffected", next, err)
	}
}

// TestPeekDoesNotRemove verifies that Peek leaves the queue unchanged and
// returns the same head each time.
func TestPeekDoesNotRemove(t *testing.T) {
	var q Queue[int]
	q.Push(1)
	q.Push(2)

	for i := range 3 {
		if v, err := q.Peek(); err != nil || v != 1 {
			t.Errorf("Peek step %d = (%v, %v), expected 1", i, v, err)
		}
	}
	if q.Length() != 2 {
		t.Errorf("Expected Peek to leave the length at 2, got %d", q.Length())
	}
	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Errorf("Dequeue after Peeks = (%v, %v), expected 1", v, err)
	}
}

// TestIteratorsAfterPartialDrain verifies that the iterators reflect the
// remaining window after some elements have been dequeued, with head at
// index 0.
func TestIteratorsAfterPartialDrain(t *testing.T) {
	var q Queue[int]
	for i := range 6 {
		q.Push(i)
	}
	for i := range 3 {
		if _, err := q.Dequeue(); err != nil {
			t.Fatalf("Dequeue(%d): %v", i, err)
		}
	}

	var fwd []int
	for _, v := range q.All() {
		fwd = append(fwd, v)
	}
	if expect := []int{3, 4, 5}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All after partial drain got %v, expected %v", fwd, expect)
	}

	var bwd []int
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	if expect := []int{5, 4, 3}; !reflect.DeepEqual(bwd, expect) {
		t.Errorf("Backward after partial drain got %v, expected %v", bwd, expect)
	}

	// Backward must be the exact reverse of All.
	for i := range fwd {
		if bwd[len(bwd)-1-i] != fwd[i] {
			t.Fatalf("Backward does not mirror All at %d", i)
		}
	}
}

// TestTruncateOnEmpty verifies truncating an empty queue is a no-op that
// leaves the queue usable.
func TestTruncateOnEmpty(t *testing.T) {
	var q Queue[int]
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after Truncate on empty queue.")
	}
	q.Push(7)
	if v, err := q.Dequeue(); err != nil || v != 7 {
		t.Errorf("Dequeue after truncate-on-empty = (%v, %v), expected 7", v, err)
	}
}

// TestQueuePropertyRandomized runs thousands of mixed operations against
// a slice reference model with a fixed seed, cross-checking after every
// step: FIFO order, length, and Peek agreement.
func TestQueuePropertyRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 11))
	const ops = 4000
	const keySpace = 50

	var q Queue[int]
	var model []int

	check := func(step int) {
		t.Helper()
		if q.Length() != len(model) {
			t.Fatalf("step %d: length %d, model has %d", step, q.Length(), len(model))
		}
		if got := contents(&q); !reflect.DeepEqual(got, model) {
			t.Fatalf("step %d: contents %v, model %v", step, got, model)
		}
		if len(model) > 0 {
			if v, err := q.Peek(); err != nil || v != model[0] {
				t.Fatalf("step %d: Peek = (%v, %v), model head %d", step, v, err, model[0])
			}
		}
	}

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(3) {
		case 0: // Push
			q.Push(v)
			model = append(model, v)
		case 1: // Dequeue
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
		case 2: // Pop (discard)
			err := q.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Pop: %v", step, err)
				}
				model = model[1:]
			}
		}
		if step%50 == 0 {
			check(step)
		}
	}
	check(ops)

	// Final Backward cross-check.
	var bwd []int
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	for i := range model {
		if bwd[len(model)-1-i] != model[i] {
			t.Fatalf("Final Backward mismatch at %d", i)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the queue.
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
	// Exact array output, head to tail.
	var q Queue[int]
	for _, v := range []int{3, 1, 2} {
		q.Push(v)
	}
	b, err := json.Marshal(&q)
	if err != nil {
		t.Fatalf("json.Marshal(&q): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	type item struct {
		S string
	}
	var items Queue[item]
	for _, s := range []string{"a", "b"} {
		items.Push(item{S: s})
	}
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
	var custom Queue[upperString]
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
	// Decoded order is preserved: element 0 becomes the head.
	var q Queue[int]
	if err := json.Unmarshal([]byte("[3,1,2]"), &q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := contents(&q); !reflect.DeepEqual(got, []int{3, 1, 2}) {
		t.Errorf("Expected [3 1 2], got %v", got)
	}
	if head, err := q.Peek(); err != nil || head != 3 {
		t.Errorf("Expected head 3, got (%v, %v)", head, err)
	}

	// A round trip reproduces the queue, including after a partial
	// drain (the slice window may have walked off the front).
	var items Queue[string]
	for _, s := range []string{"a", "b", "c", "d"} {
		items.Push(s)
	}
	if _, err := items.Dequeue(); err != nil { // now [b c d] with a walked window
		t.Fatalf("Dequeue: %v", err)
	}
	b, err := json.Marshal(&items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Queue[string]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := contents(&again); !reflect.DeepEqual(got, []string{"b", "c", "d"}) {
		t.Errorf("Expected [b c d] after round trip, got %v", got)
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte("[7]"), &q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := q.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the queue.
	var full Queue[string]
	full.Push("z")
	if err := json.Unmarshal([]byte("[]"), &full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the queue.")
	}
	full.Push("z")
	if err := json.Unmarshal([]byte("null"), &full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the queue.")
	}

	// Element-level unmarshalers are honored.
	var custom Queue[upperString]
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
	var keep Queue[string]
	keep.Push("keep")
	for _, badData := range []string{"[1,", `[1]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got := contents(&keep); !reflect.DeepEqual(got, []string{"keep"}) {
			t.Errorf("Queue changed after the error on %s: %v", badData, got)
		}
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil queue panics with a message naming
// the method, while [] and null — which store nothing — are tolerated
// everywhere.  A zero-value queue is ready to use, so it accepts
// elements without complaint.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilQueue *Queue[string]
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "queue:") || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil queue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilQueue.UnmarshalJSON([]byte(`["a"]`))
	}()

	// The zero value is usable: no constructor, no panic.
	var zero Queue[string]
	if err := zero.UnmarshalJSON([]byte(`["a"]`)); err != nil {
		t.Fatalf("UnmarshalJSON on a zero-value queue: %v", err)
	}
	if v, err := zero.Peek(); err != nil || v != "a" {
		t.Errorf("Expected a zero-value queue to hold the decoded element, got (%q, %v)", v, err)
	}
}

// TestJSONStructField marshals and unmarshals a Queue nested in a struct
// through the encoding/json package.  A nil *Queue field marshals as
// null, and because the zero value of Queue is ready to use, non-empty
// data unmarshaled into an uncreated field works — the json package
// allocates the queue itself.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string         `json:"title"`
		Tags  *Queue[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: &Queue[string]{}}
	d.Tags.Push("ds")
	d.Tags.Push("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a nil *Queue field: the json package allocates the
	// queue, and the zero value accepts the elements.
	var out Doc
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := contents(out.Tags); !reflect.DeepEqual(got, []string{"ds", "go"}) {
		t.Errorf("Expected [ds go], got %v", got)
	}

	// A nil queue field marshals as null (the json package's own nil
	// pointer rule); null clears a populated queue and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: &Queue[string]{}}
	clearDoc.Tags.Push("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the queue.")
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 500

	var q Queue[int]
	model := []int{} // non-nil, so an emptied model marshals as [] like the queue

	for step := range ops {
		switch rng.IntN(2) {
		case 0:
			v := rng.IntN(100)
			q.Push(v)
			model = append(model, v)
		case 1:
			if len(model) > 0 {
				v, err := q.Dequeue()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: Dequeue = (%v, %v), model %d", step, v, err, model[0])
				}
				model = model[1:]
			}
		}

		// Marshal must equal the model marshaled as a plain slice.
		got, err := json.Marshal(&q)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(model)
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh queue must reproduce the model.
		var fresh Queue[int]
		if err := json.Unmarshal(got, &fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if got := contents(&fresh); !reflect.DeepEqual(got, model) {
			t.Fatalf("step %d: unmarshaled %v, model %v", step, got, model)
		}
	}
}
