package index_pq

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// checkInvariants verifies the structural invariants of the queue:
//   - the heap property: the value at every heap position orders no
//     earlier than the value at its parent position;
//   - the inverse position map: qp[pq[i]] == i for every heap position;
//   - qp[k] == -1 exactly when index k is absent, and otherwise it
//     points back at k (pq[qp[k]] == k);
//   - length equals the number of indices with qp[k] != -1, and Len
//     agrees.
func checkInvariants[T any](t *testing.T, q *IndexPQ[T], cmp func(a, b T) int) {
	t.Helper()

	// Heap property over pq via cmp.
	for i := 1; i < q.length; i++ {
		parent := (i - 1) / 2
		if cmp(q.vals[q.pq[i]], q.vals[q.pq[parent]]) < 0 {
			t.Fatalf("heap invariant violated: vals[pq[%d]]=%v sorts before its parent vals[pq[%d]]=%v",
				i, q.vals[q.pq[i]], parent, q.vals[q.pq[parent]])
		}
	}

	// Every heap position holds a valid index, and qp is its exact inverse.
	for i := 0; i < q.length; i++ {
		k := q.pq[i]
		if k < 0 || k >= q.n {
			t.Fatalf("pq[%d] = %d is out of the index space 0..%d", i, k, q.n-1)
		}
		if q.qp[k] != i {
			t.Fatalf("qp[pq[%d]] = qp[%d] = %d, expected %d", i, k, q.qp[k], i)
		}
	}

	// qp[k] == -1 ⟺ k is absent; count of present indices == length.
	count := 0
	for k := 0; k < q.n; k++ {
		if q.qp[k] == -1 {
			if q.Contains(k) {
				t.Fatalf("qp[%d] is -1 but Contains(%d) reports present", k, k)
			}
			continue
		}
		count++
		if q.qp[k] < 0 || q.qp[k] >= q.length {
			t.Fatalf("qp[%d] = %d is not a heap position (length %d)", k, q.qp[k], q.length)
		}
		if q.pq[q.qp[k]] != k {
			t.Fatalf("pq[qp[%d]] = pq[%d] = %d, expected %d", k, q.qp[k], q.pq[q.qp[k]], k)
		}
		if !q.Contains(k) {
			t.Fatalf("qp[%d] = %d but Contains(%d) reports absent", k, q.qp[k], k)
		}
	}
	if count != q.length {
		t.Fatalf("length is %d but %d indices have qp[k] != -1", q.length, count)
	}
	if q.Len() != q.length {
		t.Fatalf("Len() = %d does not match internal length %d", q.Len(), q.length)
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestIndexPQRandomizedModel runs 800 mixed operations (Insert, Change,
// Delete, Pop, Peek, Value/Contains) against a map reference model with
// a fixed seed, verifying the structural invariants, the length, and —
// every few steps — that All drains exactly the model in priority order.
func TestIndexPQRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64        // index space 0..63
	const valSpace = 40 // small value space so ties are common

	q := NewIndexPQ[int](n)
	model := make(map[int]int) // index -> value the queue should hold

	modelMin := func() (minK, minV int, ok bool) {
		for k, v := range model {
			if !ok || v < minV {
				minK, minV, ok = k, v, true
			}
		}
		return
	}

	verify := func(step int) {
		if q.Len() != len(model) {
			t.Fatalf("step %d: Len()=%d, model has %d indices", step, q.Len(), len(model))
		}
		checkInvariants(t, q, Compare[int])

		// All must yield exactly the model, in non-decreasing value order.
		seen := make(map[int]int, len(model))
		prev := -1
		for k, v := range q.All() {
			if prev >= 0 && v < prev {
				t.Fatalf("step %d: All yielded %d after %d — not in priority order", step, v, prev)
			}
			prev = v
			seen[k] = v
		}
		if len(seen) != len(model) {
			t.Fatalf("step %d: All yielded %d pairs, model has %d", step, len(seen), len(model))
		}
		for k, v := range model {
			if sv, ok := seen[k]; !ok || sv != v {
				t.Fatalf("step %d: All pair for %d = (%d, %v), model says %d", step, k, sv, ok, v)
			}
		}

		// Peek must match a model minimum (ties allowed between equal values).
		if len(model) == 0 {
			if _, _, found := q.Peek(); found {
				t.Fatalf("step %d: Peek on empty queue reported true", step)
			}
		} else {
			_, minV, _ := modelMin()
			k, v, found := q.Peek()
			if !found || v != minV || model[k] != v {
				t.Fatalf("step %d: Peek = (%d, %d, %v), model min value %d", step, k, v, found, minV)
			}
		}
	}

	for step := range 800 {
		// Occasionally probe just outside the index space.
		k := rng.Intn(n+2) - 1 // -1..n
		v := rng.Intn(valSpace)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert (in-range always true; present indices replace)
			got := q.Insert(k, v)
			if k < 0 || k >= n {
				if got {
					t.Fatalf("step %d: Insert(%d) reported true for an out-of-range index", step, k)
				}
				break
			}
			if !got {
				t.Fatalf("step %d: Insert(%d) reported false for an in-range index", step, k)
			}
			model[k] = v
		case 4, 5: // Delete
			_, present := model[k]
			if got := q.Delete(k); got != present {
				t.Fatalf("step %d: Delete(%d)=%v, model said present=%v", step, k, got, present)
			}
			delete(model, k)
		case 6, 7: // Change
			_, present := model[k]
			if got := q.Change(k, v); got != present {
				t.Fatalf("step %d: Change(%d)=%v, model said present=%v", step, k, got, present)
			}
			if present {
				model[k] = v
			}
		case 8: // Pop
			k, v, found := q.Pop()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: Pop on empty queue reported true", step)
				}
				break
			}
			_, minV, _ := modelMin()
			mv, present := model[k]
			if !found || !present || mv != v || v != minV {
				t.Fatalf("step %d: Pop = (%d, %d, %v), model min value %d", step, k, v, found, minV)
			}
			delete(model, k)
		case 9: // Value / Contains
			mv, present := model[k]
			if got := q.Contains(k); got != present {
				t.Fatalf("step %d: Contains(%d)=%v, model said present=%v", step, k, got, present)
			}
			gv, found := q.Value(k)
			if found != present || (present && gv != mv) {
				t.Fatalf("step %d: Value(%d)=(%d, %v), model said (%d, %v)", step, k, gv, found, mv, present)
			}
		}
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)
}

// TestIndexPQFullDrainAndRefill stresses the queue at its capacity: fill
// all n slots, change every key, delete half, pop the rest, refill.
func TestIndexPQFullDrainAndRefill(t *testing.T) {
	const n = 128
	q := NewIndexPQ[int](n)

	for k := 0; k < n; k++ {
		if !q.Insert(k, (k*37)%n) { // shuffled values, with ties
			t.Fatalf("Insert(%d) into a non-full queue reported false", k)
		}
		checkInvariants(t, q, Compare[int])
	}
	if q.Len() != n {
		t.Fatalf("Expected Len %d on a full queue, got %d", n, q.Len())
	}

	// Change every key to its own index value (strictly increasing).
	for k := 0; k < n; k++ {
		if !q.Change(k, k) {
			t.Fatalf("Change(%d) on a full queue reported false", k)
		}
	}
	checkInvariants(t, q, Compare[int])

	// Delete the even indices.
	for k := 0; k < n; k += 2 {
		if !q.Delete(k) {
			t.Fatalf("Delete(%d) reported false", k)
		}
		checkInvariants(t, q, Compare[int])
	}

	// The odd indices pop in ascending order (value == index now).
	for want := 1; want < n; want += 2 {
		k, v, found := q.Pop()
		if !found || k != want || v != want {
			t.Fatalf("Pop = (%d, %d, %v), expected (%d, %d, true)", k, v, found, want, want)
		}
	}
	if !q.IsEmpty() {
		t.Fatalf("Expected empty queue after draining.")
	}

	// Refill after a full drain.
	for k := 0; k < n; k++ {
		q.Insert(k, n-k)
	}
	checkInvariants(t, q, Compare[int])
	if k, v, found := q.Peek(); !found || k != n-1 || v != 1 {
		t.Errorf("Peek after refill = (%d, %d, %v), expected (%d, 1, true)", k, v, found, n-1)
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

// drainPairs pops the queue dry, returning the (index, value) pairs in
// priority order.
func drainPairs[T any](q *IndexPQ[T]) [][2]any {
	var out [][2]any
	for {
		k, v, found := q.Pop()
		if !found {
			return out
		}
		out = append(out, [2]any{k, v})
	}
}

func TestMarshalJSON(t *testing.T) {
	// Exact output: priority order, minimum value first.
	q := NewIndexPQ[int](4)
	q.Insert(0, 30)
	q.Insert(1, 10)
	q.Insert(2, 50)
	q.Insert(3, 20)
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("json.Marshal(q): %v", err)
	}
	if want := `[{"k":1,"v":10},{"k":3,"v":20},{"k":0,"v":30},{"k":2,"v":50}]`; string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// The queue is unchanged by marshaling (All drains a snapshot).
	if q.Len() != 4 {
		t.Errorf("Expected Len 4 after Marshal, got %d", q.Len())
	}

	// An empty queue encodes as [].
	if b, err := json.Marshal(NewIndexPQ[int](4)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty queue, got (%s, %v)", b, err)
	}

	// A zero-value queue is a tolerated read: [].
	var zero IndexPQ[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value queue, got (%s, %v)", b, err)
	}

	// A direct call on a nil queue encodes as []; json.Marshal on a nil
	// *IndexPQ never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilQ *IndexPQ[int]
	if b, err := nilQ.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-queue call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilQ); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil queue, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewIndexPQ[upperString](2)
	custom.Insert(0, "y")
	custom.Insert(1, "x")
	if b, err := json.Marshal(custom); err != nil || string(b) != `[{"k":1,"v":"X"},{"k":0,"v":"Y"}]` {
		t.Errorf(`Expected [{"k":1,"v":"X"},{"k":0,"v":"Y"}], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewIndexPQFunc(1, func(a, b chan int) int { return 0 })
	bad.Insert(0, make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a queue of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// The decoded pairs become the queue's contents, regardless of the
	// document order: Peek and Pop follow the priority.
	q := NewIndexPQ[string](3)
	if err := json.Unmarshal([]byte(`[{"k":0,"v":"pear"},{"k":1,"v":"fig"},{"k":2,"v":"apple"}]`), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if q.Len() != 3 {
		t.Fatalf("Expected Len 3, got %d", q.Len())
	}
	if k, v, found := q.Peek(); !found || k != 2 || v != "apple" {
		t.Errorf("Peek = (%d, %q, %v), expected (2, apple, true)", k, v, found)
	}

	// A round trip rebuilds a sound queue and keeps the comparison
	// function (Insert and Pop work on the rebuilt queue).
	src := NewIndexPQ[int](4)
	src.Insert(0, 30)
	src.Insert(1, 10)
	src.Insert(2, 50)
	src.Insert(3, 20)
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewIndexPQ[int](4)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again, Compare[int])
	got := drainPairs(again)
	want := [][2]any{{1, 10}, {3, 20}, {0, 30}, {2, 50}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Expected %v after round trip, got %v", want, got)
	}
	if !again.Insert(2, 5) { // cmp was kept: still usable
		t.Errorf("Expected Insert to work after unmarshal.")
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`[{"k":1,"v":"seven"}]`), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if q.Len() != 1 || q.Contains(0) {
		t.Errorf("Expected replacement, got Len %d with 0 present=%v", q.Len(), q.Contains(0))
	}

	// Duplicate indices in the document follow the Insert convention:
	// the last pair wins.
	dup := NewIndexPQ[int](2)
	if err := json.Unmarshal([]byte(`[{"k":0,"v":9},{"k":0,"v":3}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, found := dup.Value(0); !found || v != 3 || dup.Len() != 1 {
		t.Errorf("Expected last duplicate to win: Value(0)=(%d, %v), Len=%d", v, found, dup.Len())
	}

	// An empty array and null clear the queue.
	full := NewIndexPQ[int](2)
	full.Insert(0, 1)
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the queue.")
	}
	full.Insert(0, 1)
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the queue.")
	}
	checkInvariants(t, full, Compare[int])

	// Element-level unmarshalers are honored.
	custom := NewIndexPQ[upperString](2)
	if err := json.Unmarshal([]byte(`[{"k":0,"v":"X"},{"k":1,"v":"Y"}]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, found := custom.Value(0); !found || string(v) != "X" {
		t.Errorf("Expected Value(0) = X, got (%q, %v)", v, found)
	}

	// Decode errors are returned and leave the queue untouched.
	keep := NewIndexPQ[int](4)
	keep.Insert(1, 42)
	for _, badData := range []string{"[1,", `[{"k":0,"v":"x"}]`, `{"k":0,"v":1}`, "7"} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.Len() != 1 {
			t.Errorf("Queue changed after the error on %s: Len=%d", badData, keep.Len())
		}
		if v, found := keep.Value(1); !found || v != 42 {
			t.Errorf("Queue changed after the error on %s: Value(1)=(%d, %v)", badData, v, found)
		}
	}

	// An out-of-range index is an error and leaves the queue untouched.
	if err := json.Unmarshal([]byte(`[{"k":4,"v":1}]`), keep); err == nil {
		t.Errorf("Expected an error for an out-of-range index.")
	}
	if keep.Len() != 1 {
		t.Errorf("Queue changed after the out-of-range error: Len=%d", keep.Len())
	}
	checkInvariants(t, keep, Compare[int])
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing values into a nil or zero-value queue panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero IndexPQ[int]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value queue to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON on zero-value queue", "UnmarshalJSON", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"k":0,"v":1}]`))
	})
	expectPanic(t, "UnmarshalJSON on zero-value queue", "NewIndexPQ", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"k":0,"v":1}]`))
	})

	var nilQ *IndexPQ[int]
	if err := nilQ.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil queue to be tolerated, got %v", err)
	}
	expectPanic(t, "UnmarshalJSON on nil queue", "nil queue", func() {
		_ = nilQ.UnmarshalJSON([]byte(`[{"k":0,"v":1}]`))
	})
}

// TestJSONStructField marshals and unmarshals an IndexPQ nested in a
// struct through the encoding/json package.  The queue must be created
// with NewIndexPQ/NewIndexPQFunc before unmarshaling: for a nil
// *IndexPQ field the json package allocates a zero-value queue itself
// (no comparison function), so non-empty data panics with the
// insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string           `json:"title"`
		Prio  *IndexPQ[string] `json:"prio"`
	}

	d := Doc{Title: "pluto", Prio: NewIndexPQ[string](2)}
	d.Prio.Insert(0, "zzz")
	d.Prio.Insert(1, "aaa")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"title":"pluto","prio":[{"k":1,"v":"aaa"},{"k":0,"v":"zzz"}]}`; string(b) != want {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created queue field.
	var out Doc
	out.Prio = NewIndexPQ[string](2)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if k, v, found := out.Prio.Peek(); !found || k != 1 || v != "aaa" {
		t.Errorf("Peek after unmarshal = (%d, %q, %v), expected (1, aaa, true)", k, v, found)
	}

	// A nil queue field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created queue and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","prio":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Prio: NewIndexPQ[string](2)}
	clearDoc.Prio.Insert(0, "gone")
	if err := json.Unmarshal([]byte(`{"title":"x","prio":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null prio: %v", err)
	}
	if !clearDoc.Prio.IsEmpty() {
		t.Errorf("Expected null prio to clear the queue.")
	}

	// Non-empty data into a nil *IndexPQ field: the json package
	// allocates a zero-value queue, and the insert contract panics
	// through json.Unmarshal (it does not recover panics).
	expectPanic(t, "json.Unmarshal into an uncreated queue field", "NewIndexPQ", func() {
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","prio":[{"k":0,"v":"a"}]}`), &bad)
	})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map reference model at fixed seed: every few steps the queue
// is marshaled, unmarshaled into a fresh queue, and the rebuilt queue
// must contain exactly the model — with working invariants.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run

	const n = 32
	q := NewIndexPQ[int](n)
	model := make(map[int]int)

	verify := func(step int) {
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("step %d: json.Marshal: %v", step, err)
		}

		// The marshaled pairs are in non-decreasing value order and match
		// the model exactly.
		var pairs []jsonPair[int]
		if err := json.Unmarshal(b, &pairs); err != nil {
			t.Fatalf("step %d: json.Unmarshal of pairs: %v", step, err)
		}
		if len(pairs) != len(model) {
			t.Fatalf("step %d: marshaled %d pairs, model has %d", step, len(pairs), len(model))
		}
		prev := -1 << 60
		for _, p := range pairs {
			if p.V < prev {
				t.Fatalf("step %d: marshaled pairs not in priority order: %d after %d", step, p.V, prev)
			}
			prev = p.V
			if mv, ok := model[p.K]; !ok || mv != p.V {
				t.Fatalf("step %d: marshaled pair (%d, %d), model says (%d, %v)", step, p.K, p.V, mv, ok)
			}
		}

		// Unmarshaling into a fresh queue must reproduce the model.
		fresh := NewIndexPQ[int](n)
		if err := json.Unmarshal(b, fresh); err != nil {
			t.Fatalf("step %d: json.Unmarshal: %v", step, err)
		}
		checkInvariants(t, fresh, Compare[int])
		if fresh.Len() != len(model) {
			t.Fatalf("step %d: rebuilt Len=%d, model has %d", step, fresh.Len(), len(model))
		}
		for k := 0; k < n; k++ {
			mv, present := model[k]
			gv, found := fresh.Value(k)
			if found != present || (present && gv != mv) {
				t.Fatalf("step %d: rebuilt Value(%d)=(%d, %v), model said (%d, %v)", step, k, gv, found, mv, present)
			}
		}
	}

	for step := range 400 {
		k := rng.Intn(n)
		v := rng.Intn(50)
		switch rng.Intn(4) {
		case 0, 1:
			q.Insert(k, v)
			model[k] = v
		case 2:
			q.Delete(k)
			delete(model, k)
		case 3:
			if _, present := model[k]; present {
				q.Change(k, v)
				model[k] = v
			}
		}
		if step%17 == 0 {
			verify(step)
		}
	}
	verify(400)
}
