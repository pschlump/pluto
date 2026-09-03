package dqueue_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, link-integrity checks,
// single-element and duplicate edge cases, value-copy semantics of the
// peeks, snapshot semantics of the iterators, truncate reuse, and a
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
func checkModel[T comparable](t *testing.T, q *Deque[T], model []T) {
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
			t.Fatalf("PeekFront: expected %v, got %v", model[0], front)
		}
		back, err := q.PeekBack()
		if err != nil {
			t.Fatalf("PeekBack on non-empty deque returned error: %v", err)
		}
		if back != model[len(model)-1] {
			t.Fatalf("PeekBack: expected %v, got %v", model[len(model)-1], back)
		}
	}
	// All must iterate front to back.  slices.Equal (not reflect.DeepEqual)
	// so that a nil result and an emptied model compare equal.
	var fwd []T
	for _, v := range q.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, model) {
		t.Fatalf("All: expected %v, got %v", model, fwd)
	}
	// Backward must iterate back to front.
	var bwd []T
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	var wantBwd []T
	for _, m := range slices.Backward(model) {
		wantBwd = append(wantBwd, m)
	}
	if !slices.Equal(bwd, wantBwd) {
		t.Fatalf("Backward: expected %v, got %v", wantBwd, bwd)
	}
}

// checkLinks verifies the structural invariants of the doubly linked
// list: the count matches Length, prev/next pointers are bidirectional,
// and head.prev / tail.next are nil.  Call it after structural changes;
// it must only be used from single-goroutine tests (it reads the
// internals without the lock).
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

// TestIterateSnapshot verifies that the All/Backward iterators operate on
// a snapshot taken when they are called: later modifications — even
// truncating the whole deque — are not observed, and mutating the deque
// from inside the loop is safe.
func TestIterateSnapshot(t *testing.T) {
	var q Deque[int]
	for i := range 5 {
		q.PushBack(i)
	}

	all := q.All()
	backward := q.Backward()

	q.Truncate() // the iterators above must not observe this

	var fwd []int
	for _, v := range all {
		fwd = append(fwd, v)
	}
	if expect := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All after Truncate error, expected %v got %v", expect, fwd)
	}

	var bwd []int
	for _, v := range backward {
		bwd = append(bwd, v)
	}
	if expect := []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(bwd, expect) {
		t.Errorf("Backward after Truncate error, expected %v got %v", expect, bwd)
	}

	// Popping from inside the loop is safe: the loop sees the snapshot.
	for i := range 3 {
		q.PushBack(i)
	}
	visited := 0
	for _, v := range q.All() {
		visited++
		_, _ = q.PopFront()
		_ = v
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while popping during iteration, got %d", visited)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty deque after popping during iteration.")
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against a
// slice reference model with a fixed seed, cross-checking every
// observable result (length, both peeks, pop order, iteration order)
// along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 42))
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

func TestMarshalJSON(t *testing.T) {
	// Exact array output, front to back.
	var q Deque[int]
	q.PushBack(1)
	q.PushFront(3)
	q.PushBack(2)
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
	var items Deque[item]
	items.PushBack(item{S: "a"})
	items.PushBack(item{S: "b"})
	if b, err := json.Marshal(&items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty (zero-value) deque encodes as [].
	var zero Deque[int]
	if b, err := json.Marshal(&zero); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty deque, got (%s, %v)", b, err)
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
	// Decoded order is preserved: element 0 becomes the front.
	var q Deque[int]
	if err := json.Unmarshal([]byte("[3,1,2]"), &q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkModel(t, &q, []int{3, 1, 2})
	checkLinks(t, &q)
	if front, err := q.PeekFront(); err != nil || front != 3 {
		t.Errorf("Expected front 3, got (%v, %v)", front, err)
	}

	// A round trip rebuilds a structurally sound deque that stays fully
	// usable from both ends.
	var orig Deque[int]
	for _, v := range []int{1, 2, 3} {
		orig.PushBack(v)
	}
	b, err := json.Marshal(&orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Deque[int]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkModel(t, &again, []int{1, 2, 3})
	checkLinks(t, &again)
	again.PushFront(0)
	again.PushBack(4)
	if v, err := again.PopBack(); err != nil || v != 4 {
		t.Errorf("PopBack after round trip = (%v, %v), expected 4", v, err)
	}
	checkModel(t, &again, []int{0, 1, 2, 3})
	checkLinks(t, &again)

	// Unmarshaling replaces the current contents, and [] / null clear.
	var full Deque[int]
	for _, v := range []int{8, 9} {
		full.PushBack(v)
	}
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), &full); err != nil {
			t.Errorf("Expected %s to clear the deque, got %v", data, err)
		}
		checkModel(t, &full, nil)
		checkLinks(t, &full)
		full.PushBack(8)
		full.PushBack(9)
	}

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
	var keep Deque[string]
	keep.PushBack("keep")
	for _, badData := range []string{"[1,", `[1]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		var got []string
		for _, v := range keep.All() {
			got = append(got, v)
		}
		if fmt.Sprint(got) != "[keep]" {
			t.Errorf("Deque changed after the error on %s: %s", badData, got)
		}
	}
	checkModel(t, &keep, []string{"keep"})
	checkLinks(t, &keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the push
// family: storing elements into a nil deque panics with a message naming
// the method, while [] and null — which store nothing — are tolerated
// everywhere.  Unlike dll, the zero value is a ready-to-use deque, so
// storing into one works without a panic.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Deque[int]
	for _, data := range []string{"[]", "null", "[1,2]"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value deque to be tolerated, got %v", data, err)
		}
	}
	checkModel(t, &zero, []int{1, 2})
	checkLinks(t, &zero)

	var nilDeque *Deque[int]
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
		_ = nilDeque.UnmarshalJSON([]byte("[1]"))
	}()
}

// TestJSONStructField marshals and unmarshals a Deque nested in a struct
// through the encoding/json package.  The zero value is usable, so the
// field needs no initialization before unmarshaling.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string        `json:"title"`
		Tags  Deque[string] `json:"tags"`
	}

	var d Doc
	d.Title = "pluto"
	d.Tags.PushBack("ds")
	d.Tags.PushBack("go")

	b, err := json.Marshal(&d) // &d so the Deque's pointer-receiver marshaler applies (and no lock is copied)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	var out Doc
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
}
