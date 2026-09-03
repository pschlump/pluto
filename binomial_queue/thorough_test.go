package binomial_queue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Structural invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the binomial-queue structure: the forest is
// in strictly increasing order of degree, every tree is a true binomial
// tree of its degree (min-heap-ordered per cmp, children of degrees
// 0..k-1, exactly 2^k nodes), and the total node count matches Length.
// Call it after every structural change.
func checkInvariants[T any](t *testing.T, q *BinomialQueue[T], cmp func(a, b T) int) {
	t.Helper()
	if q == nil {
		return
	}
	total := 0
	prevDegree := -1
	for _, tr := range q.trees {
		degree := len(tr.children)
		if degree <= prevDegree {
			t.Errorf("forest invariant violated: tree of degree %d follows degree %d (not strictly increasing)",
				degree, prevDegree)
		}
		prevDegree = degree
		total += checkBinomialTree(t, tr, cmp)
	}
	if total != q.Length() {
		t.Errorf("Length mismatch: Length()=%d but the forest has %d nodes", q.Length(), total)
	}
	if total == 0 && !q.IsEmpty() {
		t.Errorf("IsEmpty() is false but the queue has no nodes")
	}
	if total > 0 && q.IsEmpty() {
		t.Errorf("IsEmpty() is true but the queue has %d nodes", total)
	}
}

// checkBinomialTree verifies that n roots a true binomial tree of degree
// len(n.children): child i has degree i (so the tree has 2^degree
// nodes), and every child sorts at or after its parent per cmp.  It
// returns the number of nodes in the tree.
func checkBinomialTree[T any](t *testing.T, n *bqNode[T], cmp func(a, b T) int) int {
	t.Helper()
	degree := len(n.children)
	count := 1
	for i, c := range n.children {
		if len(c.children) != i {
			t.Errorf("binomial tree invariant violated: child %d has degree %d, expected %d",
				i, len(c.children), i)
		}
		if cmp(c.value, n.value) < 0 {
			t.Errorf("heap-order invariant violated: child %v sorts before its parent %v", c.value, n.value)
		}
		count += checkBinomialTree(t, c, cmp)
	}
	if count != 1<<degree {
		t.Errorf("degree-%d binomial tree has %d nodes, expected %d", degree, count, 1<<degree)
	}
	return count
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestRandomizedModel runs 800 mixed Insert/DeleteMin/Peek/Merge steps
// against a multiset reference model with a fixed seed, verifying that
// DeleteMin returns the true minimum each time and that the structural
// invariants hold after every step.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	q := NewBinomialQueue[int]()
	model := map[int]int{} // value -> count

	modelMin := func() int {
		min := -1
		for k := range model {
			if min == -1 || k < min {
				min = k
			}
		}
		return min
	}
	modelCount := func() int {
		n := 0
		for _, c := range model {
			n += c
		}
		return n
	}
	removeFromModel := func(v int) {
		model[v]--
		if model[v] == 0 {
			delete(model, v)
		}
	}

	verify := func(step int) {
		t.Helper()
		checkInvariants(t, q, Compare[int])
		if q.Len() != modelCount() {
			t.Fatalf("step %d: Len %d, model has %d", step, q.Len(), modelCount())
		}
		// Multiset equality.
		seen := map[int]int{}
		for _, v := range q.All() {
			seen[v]++
		}
		if !reflect.DeepEqual(seen, model) {
			t.Fatalf("step %d: queue multiset %v != model %v", step, seen, model)
		}
		// Peek/FindMin must agree with the model minimum.
		if len(model) == 0 {
			if _, found := q.Peek(); found {
				t.Fatalf("step %d: Peek on empty queue reported true", step)
			}
		} else if v, found := q.FindMin(); !found || v != modelMin() {
			t.Fatalf("step %d: FindMin = (%v, %v), model min %d", step, v, found, modelMin())
		}
	}

	const keySpace = 100
	for step := range 800 {
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert
			v := rng.Intn(keySpace)
			q.Insert(v)
			model[v]++
		case 4, 5, 6: // DeleteMin must return the true minimum
			got, found := q.DeleteMin()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: DeleteMin on empty reported true", step)
				}
			} else {
				min := modelMin()
				if !found || got != min {
					t.Fatalf("step %d: DeleteMin = (%v, %v), model min %d", step, got, found, min)
				}
				removeFromModel(min)
			}
		case 7, 8: // Merge a freshly built queue in
			other := NewBinomialQueue[int]()
			m := rng.Intn(20)
			for range m {
				v := rng.Intn(keySpace)
				other.Insert(v)
				model[v]++
			}
			q.Merge(other)
			if !other.IsEmpty() || other.Len() != 0 {
				t.Fatalf("step %d: other not empty after Merge (Len %d)", step, other.Len())
			}
		case 9: // Peek only
		}
		verify(step)
	}

	// Drain: the rest comes out in non-decreasing order and matches the model.
	prev := -1
	for {
		v, found := q.DeleteMin()
		if !found {
			break
		}
		if v < prev {
			t.Fatalf("drain: %d after %d, not in ascending order", v, prev)
		}
		prev = v
		removeFromModel(v)
		checkInvariants(t, q, Compare[int])
	}
	if len(model) != 0 {
		t.Errorf("model not drained: %v", model)
	}
}

// TestMergeThenDeleteMin is a focused Merge check: two queues built from
// interleaved value ranges must drain as one sorted stream.
func TestMergeThenDeleteMin(t *testing.T) {
	a := NewBinomialQueue[int]()
	b := NewBinomialQueue[int]()
	for i := range 50 {
		a.Insert(2 * i) // evens
		checkInvariants(t, a, Compare[int])
	}
	for i := range 50 {
		b.Insert(2*i + 1) // odds
		checkInvariants(t, b, Compare[int])
	}
	a.Merge(b)
	checkInvariants(t, a, Compare[int])
	if a.Len() != 100 {
		t.Fatalf("Expected length 100 after Merge, got %d", a.Len())
	}
	for i := range 100 {
		v, found := a.DeleteMin()
		if !found || v != i {
			t.Fatalf("DeleteMin = (%v, %v), expected %d", v, found, i)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// shoutString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the queue.
type shoutString string

func (u shoutString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *shoutString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = shoutString(s)
	return nil
}

// drainJSON drains q with DeleteMin, returning the elements in sorted
// (min-first) order — the order that must survive a JSON round trip.
func drainJSON[T any](q *BinomialQueue[T]) []T {
	var out []T
	for {
		v, found := q.DeleteMin()
		if !found {
			return out
		}
		out = append(out, v)
	}
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output in internal forest order (the order of All) —
	// inserting 1, 2, 3 builds the forest [B0(3), B1(1->[2])], which
	// iterates as 3, 1, 2.  This is NOT sorted order.
	q := NewBinomialQueue[int]()
	for _, v := range []int{1, 2, 3} {
		q.Insert(v)
	}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("json.Marshal(q): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := newTestQueue()
	for _, s := range []string{"a", "b"} {
		items.Insert(TestItem{S: s})
	}
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty queue encodes as [].
	if b, err := json.Marshal(NewBinomialQueue[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty queue, got (%s, %v)", b, err)
	}

	// A zero-value queue is a tolerated read: [].
	var zero BinomialQueue[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value queue, got (%s, %v)", b, err)
	}

	// A direct call on a nil queue encodes as []; json.Marshal on a nil
	// *BinomialQueue never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilQueue *BinomialQueue[int]
	if b, err := nilQueue.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-queue call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilQueue); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil queue, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewBinomialQueueFunc(func(a, b shoutString) int { return strings.Compare(string(a), string(b)) })
	custom.Insert("x")
	custom.Insert("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewBinomialQueueFunc(func(a, b chan int) int { return 0 })
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a queue of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Every element lands in the queue and drains in sorted order.
	q := NewBinomialQueue[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, q, Compare[int])
	if q.Len() != 3 {
		t.Errorf("Expected length 3, got %d", q.Len())
	}
	if v, found := q.FindMin(); !found || v != 1 {
		t.Errorf("Expected minimum 1, got (%v, %v)", v, found)
	}
	if got, want := fmt.Sprint(drainJSON(q)), "[1 2 3]"; got != want {
		t.Errorf("Expected drain %s, got %s", want, got)
	}

	// A round trip preserves the multiset and the drain order, and keeps
	// the comparison function (FindMin/DeleteMin work on the rebuilt
	// queue).
	items := newTestQueue()
	for _, s := range []string{"c", "a", "b"} {
		items.Insert(TestItem{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestQueue()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again, cmpTestItem)
	if got, want := again.Len(), items.Len(); got != want {
		t.Errorf("Expected length %d after round trip, got %d", want, got)
	}
	var drained []string
	for {
		v, found := again.DeleteMin()
		if !found {
			break
		}
		drained = append(drained, v.S)
	}
	if got, want := fmt.Sprint(drained), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte("[7]"), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := q.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the queue.
	full := newTestQueue()
	full.Insert(TestItem{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the queue.")
	}
	full.Insert(TestItem{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the queue.")
	}
	checkInvariants(t, full, cmpTestItem)

	// Element-level unmarshalers are honored.
	custom := NewBinomialQueueFunc(func(a, b shoutString) int { return strings.Compare(string(a), string(b)) })
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
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
	keep := newTestQueue()
	keep.Insert(TestItem{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := keep.Len(), 1; got != want {
			t.Errorf("Queue changed after the error on %s: length %d, want %d", badData, got, want)
		}
		if v, found := keep.FindMin(); !found || v.S != "keep" {
			t.Errorf("Queue changed after the error on %s: min (%v, %v)", badData, v, found)
		}
	}
	checkInvariants(t, keep, cmpTestItem)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value queue panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero BinomialQueue[TestItem]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value queue to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value queue.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewBinomialQueue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilQueue *BinomialQueue[TestItem]
	if err := nilQueue.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil queue to be tolerated, got %v", err)
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

// TestJSONStructField marshals and unmarshals a BinomialQueue nested in
// a struct through the encoding/json package.  The queue must be created
// with NewBinomialQueue/NewBinomialQueueFunc before unmarshaling: for a
// nil *BinomialQueue field the json package allocates a zero-value queue
// itself (no comparison function), so non-empty data panics with the
// insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string                 `json:"title"`
		Tags  *BinomialQueue[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewBinomialQueue[string]()}
	d.Tags.Insert("ds")
	d.Tags.Insert("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Unmarshal into a pre-created queue field.
	var out Doc
	out.Tags = NewBinomialQueue[string]()
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

	// A nil queue field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created queue and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewBinomialQueue[string]()}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the queue.")
	}

	// Non-empty data into a nil *BinomialQueue field: the json package
	// allocates a zero-value queue, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated queue field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBinomialQueue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a multiset reference model at fixed seed: a round trip must
// preserve the element multiset and the sorted drain order, whatever the
// internal forest shape.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run
	const ops = 500

	q := NewBinomialQueue[int]()
	model := map[int]int{} // value -> count

	removeFromModel := func(v int) {
		model[v]--
		if model[v] == 0 {
			delete(model, v)
		}
	}

	for step := range ops {
		switch rng.Intn(3) {
		case 0, 1: // Insert
			v := rng.Intn(100)
			q.Insert(v)
			model[v]++
		case 2: // DeleteMin
			v, found := q.DeleteMin()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: DeleteMin on empty reported true", step)
				}
			} else {
				if !found {
					t.Fatalf("step %d: DeleteMin on non-empty reported false", step)
				}
				removeFromModel(v)
			}
		}

		// Round trip into a fresh queue must reproduce the multiset, the
		// minimum, and the sorted drain order.
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		fresh := NewBinomialQueue[int]()
		if err := json.Unmarshal(b, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if fresh.Len() != q.Len() {
			t.Fatalf("step %d: round trip length %d, want %d", step, fresh.Len(), q.Len())
		}
		seen := map[int]int{}
		for _, v := range fresh.All() {
			seen[v]++
		}
		if !reflect.DeepEqual(seen, model) {
			t.Fatalf("step %d: round trip multiset %v != model %v", step, seen, model)
		}
		prev := -1
		for {
			v, found := fresh.DeleteMin()
			if !found {
				break
			}
			if v < prev {
				t.Fatalf("step %d: drain %d after %d, not in ascending order", step, v, prev)
			}
			prev = v
		}
		checkInvariants(t, q, Compare[int])
	}
}
