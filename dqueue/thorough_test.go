package dqueue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, link-integrity checks,
// single-element and duplicate edge cases, value-copy semantics of the
// peeks, live-walk semantics of the iterators, truncate reuse, and a
// fixed-seed randomized property test.

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

// checkModel verifies the deque's observable state against a reference
// model (a plain slice where the front is element 0).
func checkModel(t *testing.T, q *Deque[int], model []int) {
	t.Helper()
	if got, want := q.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := q.IsEmpty(), len(model) == 0; got != want {
		t.Fatalf("IsEmpty: expected %v, got %v", want, got)
	}
	if len(model) > 0 {
		front, err := q.PeekFront()
		if err != nil {
			t.Fatalf("PeekFront on non-empty deque returned error: %v", err)
		}
		if front != model[0] {
			t.Fatalf("PeekFront: expected %d, got %d", model[0], front)
		}
		back, err := q.PeekBack()
		if err != nil {
			t.Fatalf("PeekBack on non-empty deque returned error: %v", err)
		}
		if back != model[len(model)-1] {
			t.Fatalf("PeekBack: expected %d, got %d", model[len(model)-1], back)
		}
	}
	// All must iterate front to back.  slices.Equal (not reflect.DeepEqual)
	// so that a nil result and an emptied model compare equal.
	var fwd []int
	for _, v := range q.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, model) {
		t.Fatalf("All: expected %v, got %v", model, fwd)
	}
	// Backward must iterate back to front.
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
func checkLinks[T any](t *testing.T, q *Deque[T]) {
	t.Helper()

	// Walk forward from the head, verifying every prev pointer on the way.
	n := 0
	var last *dequeElement[T]
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
	var first *dequeElement[T]
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

// TestSingleElement covers single-element edge cases: every combination
// of pushing on one end and popping from either end must leave an empty
// deque.
func TestSingleElement(t *testing.T) {
	for _, tc := range []struct {
		name string
		push func(q *Deque[string], v string)
		pop  func(q *Deque[string]) (string, error)
	}{
		{"PushFront/PopFront", (*Deque[string]).PushFront, (*Deque[string]).PopFront},
		{"PushFront/PopBack", (*Deque[string]).PushFront, (*Deque[string]).PopBack},
		{"PushBack/PopFront", (*Deque[string]).PushBack, (*Deque[string]).PopFront},
		{"PushBack/PopBack", (*Deque[string]).PushBack, (*Deque[string]).PopBack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var q Deque[string]
			tc.push(&q, "x")

			if q.Length() != 1 {
				t.Fatalf("Expected length 1 got %d", q.Length())
			}
			checkLinks(t, &q)

			// Both peeks see the same single element.
			if f, err := q.PeekFront(); err != nil || f != "x" {
				t.Errorf("PeekFront = (%q, %v)", f, err)
			}
			if b, err := q.PeekBack(); err != nil || b != "x" {
				t.Errorf("PeekBack = (%q, %v)", b, err)
			}

			v, err := tc.pop(&q)
			if err != nil || v != "x" {
				t.Errorf("Pop = (%q, %v)", v, err)
			}
			if !q.IsEmpty() {
				t.Errorf("Expected empty deque after popping the single element")
			}
			if q.head != nil || q.tail != nil {
				t.Errorf("Expected nil head and tail after draining, got %v/%v", q.head, q.tail)
			}
			checkLinks(t, &q)
		})
	}
}

// TestDuplicateValues verifies the deque stores duplicate values without
// conflating them.
func TestDuplicateValues(t *testing.T) {
	var q Deque[int]
	for range 5 {
		q.PushBack(7)
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
		if v, err := q.PopFront(); err != nil || v != 7 {
			t.Errorf("PopFront step %d = (%d, %v)", i, v, err)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty deque after popping all duplicates")
	}
}

// TestPeekReturnsValue verifies that the peeks return independent
// values: mutating one cannot affect the deque.
func TestPeekReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var q Deque[item]
	q.PushBack(item{S: "a", N: 1})
	q.PushBack(item{S: "b", N: 2})

	v, err := q.PeekBack()
	if err != nil {
		t.Fatalf("PeekBack: %v", err)
	}
	v.N = 99 // mutate the returned value

	if got, err := q.PeekBack(); err != nil || got.N != 2 {
		t.Errorf("PeekBack after mutation = (%v, %v), expected N=2 unaffected", got, err)
	}
	if got, err := q.PeekFront(); err != nil || got.S != "a" {
		t.Errorf("PeekFront = (%v, %v), expected the front unaffected", got, err)
	}
	if q.Length() != 2 {
		t.Errorf("Expected the peeks to leave the length at 2, got %d", q.Length())
	}
}

// TestPushPopInterleaved cross-checks against the model between every
// operation.
func TestPushPopInterleaved(t *testing.T) {
	var q Deque[int]
	var model []int

	for _, v := range []int{1, 2, 3} {
		q.PushBack(v)
		model = append(model, v)
		checkModel(t, &q, model)
	}

	q.PushFront(0)
	model = append([]int{0}, model...)
	checkModel(t, &q, model)

	if v, err := q.PopBack(); err != nil || v != 3 {
		t.Fatalf("PopBack = (%v, %v), expected 3", v, err)
	}
	model = model[:len(model)-1]
	checkModel(t, &q, model)
	checkLinks(t, &q)

	if v, err := q.PopFront(); err != nil || v != 0 {
		t.Fatalf("PopFront = (%v, %v), expected 0", v, err)
	}
	model = model[1:]
	checkModel(t, &q, model)
}

// TestTruncateReuse verifies that the deque is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	var q Deque[int]
	for i := range 10 {
		q.PushBack(i)
	}
	q.Truncate()

	if !q.IsEmpty() || q.Length() != 0 {
		t.Errorf("Expected empty deque after Truncate.")
	}
	checkModel(t, &q, nil)
	checkLinks(t, &q)

	// Reusable after the drain, from both ends.
	q.PushBack(7)
	if v, err := q.PeekBack(); err != nil || v != 7 {
		t.Errorf("PeekBack after Truncate = (%v, %v), expected 7", v, err)
	}
	q.PushFront(6)
	if v, err := q.PeekFront(); err != nil || v != 6 {
		t.Errorf("PeekFront after reuse = (%v, %v), expected 6", v, err)
	}

	// Truncating an already-empty deque is fine.
	q.Truncate()
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty deque after double Truncate.")
	}
}

// TestIteratorReflectsLiveDeque is the opposite of the dqueue_ts
// snapshot test: the All/Backward iterators walk the live list — All
// through the next pointers, Backward through the prev pointers — so
// modifications made between iterations are visible, and the deque must
// not be modified while an iterator is running.
func TestIteratorReflectsLiveDeque(t *testing.T) {
	var q Deque[int]
	q.PushBack(1)
	q.PushBack(2)

	var seen []int
	for _, v := range q.All() {
		seen = append(seen, v)
		break // stop after the front element
	}
	if expect := []int{1}; !reflect.DeepEqual(seen, expect) {
		t.Fatalf("First visit: expected %v got %v", expect, seen)
	}

	// Mutate at both ends, then iterate again: the changes are visible.
	q.PushFront(3)
	q.PushBack(4)
	seen = nil
	for _, v := range q.All() {
		seen = append(seen, v)
	}
	if expect := []int{3, 1, 2, 4}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("All after pushes: expected %v got %v", expect, seen)
	}

	if _, err := q.PopBack(); err != nil {
		t.Fatalf("PopBack: %v", err)
	}
	seen = nil
	for _, v := range q.Backward() {
		seen = append(seen, v)
	}
	if expect := []int{2, 1, 3}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("Backward after PopBack: expected %v got %v", expect, seen)
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against a
// slice reference model with a fixed seed, cross-checking every
// observable result (length, both peeks, pop order, iteration order)
// along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260826, 42))
	const ops = 4000
	const keySpace = 50

	var q Deque[int]
	var model []int

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(8) {
		case 0, 1, 2: // PushBack (weighted so the deque grows)
			q.PushBack(v)
			model = append(model, v)
		case 3, 4: // PushFront
			q.PushFront(v)
			model = append([]int{v}, model...)
		case 5: // PopFront
			got, err := q.PopFront()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyDeque) {
					t.Fatalf("step %d: PopFront on empty returned %v", step, err)
				}
			} else {
				if err != nil || got != model[0] {
					t.Fatalf("step %d: PopFront = (%v, %v), model front %d", step, got, err, model[0])
				}
				model = model[1:]
			}
		case 6: // PopBack
			got, err := q.PopBack()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyDeque) {
					t.Fatalf("step %d: PopBack on empty returned %v", step, err)
				}
			} else {
				if err != nil || got != model[len(model)-1] {
					t.Fatalf("step %d: PopBack = (%v, %v), model back %d", step, got, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case 7: // Truncate
			q.Truncate()
			model = model[:0]
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

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the deque.
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

// jsonItem is a struct element type, to verify that struct elements use
// their normal JSON encoding through the deque.
type jsonItem struct {
	S string
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, front to back.
	var q Deque[int]
	for _, v := range []int{3, 1, 2} {
		q.PushBack(v)
	}
	b, err := json.Marshal(&q)
	if err != nil {
		t.Fatalf("json.Marshal(&q): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Pushes at both ends still marshal front to back.
	var both Deque[int]
	both.PushBack(2)
	both.PushFront(1)
	both.PushBack(3)
	if b, err := json.Marshal(&both); err != nil || string(b) != "[1,2,3]" {
		t.Errorf("Expected [1,2,3], got (%s, %v)", b, err)
	}

	// Struct elements use their normal JSON encoding.
	var items Deque[jsonItem]
	for _, s := range []string{"a", "b"} {
		items.PushBack(jsonItem{S: s})
	}
	if b, err := json.Marshal(&items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty deque encodes as [].
	if b, err := json.Marshal(&Deque[int]{}); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty deque, got (%s, %v)", b, err)
	}

	// A zero-value deque is a tolerated read: [].
	var zero Deque[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value deque, got (%s, %v)", b, err)
	}

	// A direct call on a nil deque encodes as []; json.Marshal on a nil
	// *Deque never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilDeque *Deque[int]
	if b, err := nilDeque.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-deque call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilDeque); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil deque, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	var custom Deque[upperString]
	custom.PushBack("x")
	custom.PushBack("y")
	if b, err := json.Marshal(&custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	var bad Deque[chan int]
	bad.PushBack(make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a deque of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the front.  The zero
	// value needs no constructor, so unmarshaling into it just works.
	var q Deque[int]
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
	if front, err := q.PeekFront(); err != nil || front != 3 {
		t.Errorf("Expected front 3, got (%v, %v)", front, err)
	}
	checkLinks(t, &q)

	// A round trip rebuilds a structurally sound deque.
	var items Deque[string]
	for _, s := range []string{"a", "b", "c"} {
		items.PushBack(s)
	}
	b, err := json.Marshal(&items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Deque[string]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkLinks(t, &again)
	var vals []string
	for _, v := range again.All() {
		vals = append(vals, v)
	}
	if got, want := fmt.Sprint(vals), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte("[7]"), &q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := q.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the deque.
	var full Deque[string]
	full.PushBack("z")
	if err := json.Unmarshal([]byte("[]"), &full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the deque.")
	}
	full.PushBack("z")
	if err := json.Unmarshal([]byte("null"), &full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the deque.")
	}
	checkLinks(t, &full)

	// Element-level unmarshalers are honored.
	var custom Deque[upperString]
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

	// Decode errors are returned and leave the deque untouched.
	var keep Deque[jsonItem]
	keep.PushBack(jsonItem{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, err := json.Marshal(&keep); err != nil || string(got) != `[{"S":"keep"}]` {
			t.Errorf("Deque changed after the error on %s: (%s, %v)", badData, got, err)
		}
	}
	checkLinks(t, &keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil deque panics with a message naming
// the method, while [] and null — which store nothing — are tolerated
// everywhere.  Unlike dll, a zero-value deque is fully usable (there is
// no equality function to miss), so storing into one must work.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Deque[string]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value deque to be tolerated, got %v", data, err)
		}
	}

	// Storing into the zero value works — the deque needs no constructor.
	if err := zero.UnmarshalJSON([]byte(`["a","b"]`)); err != nil {
		t.Fatalf("UnmarshalJSON on a zero-value deque: %v", err)
	}
	if got, want := zero.Len(), 2; got != want {
		t.Errorf("Expected length %d after unmarshaling into a zero-value deque, got %d", want, got)
	}
	checkLinks(t, &zero)

	var nilDeque *Deque[string]
	for _, data := range []string{"[]", "null"} {
		if err := nilDeque.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil deque to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil deque.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil deque") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilDeque.UnmarshalJSON([]byte(`["a"]`))
	}()
}

// TestJSONStructField marshals and unmarshals a Deque nested in a struct
// through the encoding/json package.  Because the deque has no
// constructor, even a nil *Deque field works: the json package allocates
// a zero-value deque and unmarshaling stores into it.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string         `json:"title"`
		Tags  *Deque[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: &Deque[string]{}}
	d.Tags.PushBack("ds")
	d.Tags.PushBack("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created deque field.
	out := Doc{Tags: &Deque[string]{}}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for _, v := range out.Tags.All() {
		tags = append(tags, v)
	}
	if fmt.Sprint(tags) != "[ds go]" {
		t.Errorf("Expected [ds go], got %v", tags)
	}

	// Unmarshal into a nil *Deque field: the json package allocates the
	// deque itself, and with no constructor requirement it just works.
	var fresh Doc
	if err := json.Unmarshal(b, &fresh); err != nil {
		t.Fatalf("json.Unmarshal into a nil deque field: %v", err)
	}
	if fresh.Tags == nil || fresh.Tags.Len() != 2 {
		t.Errorf("Expected an allocated 2-element deque field, got %v", fresh.Tags)
	}

	// A nil deque field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created deque and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: &Deque[string]{}}
	clearDoc.Tags.PushBack("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the deque.")
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260902, 42))
	const ops = 500

	var q Deque[int]
	model := []int{} // non-nil, so an emptied model marshals as [] like the deque

	for step := range ops {
		switch rng.IntN(4) {
		case 0:
			v := rng.IntN(100)
			q.PushBack(v)
			model = append(model, v)
		case 1:
			v := rng.IntN(100)
			q.PushFront(v)
			model = append([]int{v}, model...)
		case 2:
			if len(model) > 0 {
				v, err := q.PopFront()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: PopFront = (%v, %v), model %d", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3:
			if len(model) > 0 {
				v, err := q.PopBack()
				if err != nil || v != model[len(model)-1] {
					t.Fatalf("step %d: PopBack = (%v, %v), model %d", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
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

		// Unmarshaling into a fresh deque must reproduce the model.
		var fresh Deque[int]
		if err := json.Unmarshal(got, &fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		var vals []int
		for _, v := range fresh.All() {
			vals = append(vals, v)
		}
		if fmt.Sprint(vals) != fmt.Sprint(model) {
			t.Fatalf("step %d: round trip got %v, model %v", step, vals, model)
		}
	}
}
