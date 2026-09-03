package index_pq_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// checkInvariants verifies the structural invariants of the queue (heap
// property, inverse position map, length).  It reads the internals
// WITHOUT taking the lock — single-goroutine tests only (or call it only
// once all concurrent work is known to be quiescent).
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
			continue
		}
		count++
		if q.qp[k] < 0 || q.qp[k] >= q.length {
			t.Fatalf("qp[%d] = %d is not a heap position (length %d)", k, q.qp[k], q.length)
		}
		if q.pq[q.qp[k]] != k {
			t.Fatalf("pq[qp[%d]] = pq[%d] = %d, expected %d", k, q.qp[k], q.pq[q.qp[k]], k)
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

// TestIndexPQRandomizedModel runs 800 mixed operations against a map
// reference model with a fixed seed.  It is single-goroutine, so
// checkInvariants may read the internals directly.
func TestIndexPQRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64        // index space 0..63
	const valSpace = 40 // small value space so ties are common

	q := NewIndexPQ[int](n)
	model := make(map[int]int) // index -> value the queue should hold

	modelMin := func() (minV int, ok bool) {
		for _, v := range model {
			if !ok || v < minV {
				minV, ok = v, true
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
			minV, _ := modelMin()
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
			minV, _ := modelMin()
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

// -------------------------------------------------------------------------------------------------------
// Thread-safety: run under -race
// -------------------------------------------------------------------------------------------------------

// TestConcurrentInsertChangePop hammers one shared queue from many
// goroutines: writers Insert and Change striped (disjoint) indices while
// an observer iterates the snapshot All and reads Peek/Len, then all
// goroutines Pop concurrently — every inserted index must come out
// exactly once.
func TestConcurrentInsertChangePop(t *testing.T) {
	const n = 1000
	const workers = 8

	q := NewIndexPQ[int](n)

	var stop atomic.Bool
	var observerWG sync.WaitGroup
	observerWG.Add(1)
	go func() {
		defer observerWG.Done()
		for !stop.Load() {
			// The snapshot is a valid queue: each pass yields
			// non-decreasing values regardless of concurrent mutation.
			prev := -1
			first := true
			for _, v := range q.All() {
				if !first && v < prev {
					t.Errorf("All yielded %d after %d — snapshot not in priority order", v, prev)
					return
				}
				first = false
				prev = v
			}
			q.Peek()
			q.Len()
			q.IsEmpty()
		}
	}()

	// Writers on disjoint stripes: no two goroutines touch the same index.
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := w; k < n; k += workers {
				q.Insert(k, (k*37)%n)
				q.Change(k, (k*91)%n) // always present: own stripe
				q.Contains(k)
				q.Value(k)
			}
		}(w)
	}
	wg.Wait()

	if q.Len() != n {
		t.Fatalf("Expected Len %d after concurrent fills, got %d", n, q.Len())
	}
	checkInvariants(t, q, Compare[int]) // writers are done: quiescent

	// Concurrent pops: every index comes out exactly once.
	var mu sync.Mutex
	seen := make(map[int]bool, n)
	total := 0
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				k, _, found := q.Pop()
				if !found {
					return
				}
				mu.Lock()
				if seen[k] {
					t.Errorf("index %d popped twice", k)
				}
				seen[k] = true
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	stop.Store(true)
	observerWG.Wait()

	if total != n {
		t.Errorf("Expected %d popped indices, got %d", n, total)
	}
	if q.Len() != 0 || !q.IsEmpty() {
		t.Errorf("Expected drained queue, got Len %d", q.Len())
	}
}

// TestConcurrentCompound exercises the Lock + Nl* compound surface:
// atomic decrease-key-if-greater clamps from many goroutines must leave
// every value at or below every clamp threshold ever applied, with no
// torn read-then-write.
func TestConcurrentCompound(t *testing.T) {
	const n = 200
	const workers = 8

	q := NewIndexPQ[int](n)
	for k := range n {
		q.Insert(k, 1000+k)
	}

	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 100 {
				k := (w*100 + i) % n
				threshold := i
				q.Lock()
				if q.NlContains(k) {
					if v, found := q.NlValue(k); found && v > threshold {
						q.NlChange(k, threshold)
					}
				}
				q.Unlock()
			}
		}(w)
	}
	wg.Wait()

	// Every threshold 0..99 was applied to every index, so every value is
	// at most 99.
	if q.Len() != n {
		t.Fatalf("Expected Len %d after compound clamps, got %d", n, q.Len())
	}
	for k, v := range q.All() {
		if v > 99 {
			t.Errorf("index %d has value %d after clamps to 0..99 — torn compound", k, v)
		}
	}
	checkInvariants(t, q, Compare[int])

	// Concurrent Lock + NlPop: each pop is atomic, no index pops twice.
	var mu sync.Mutex
	seen := make(map[int]bool, n)
	total := 0
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				q.Lock()
				if q.NlLen() == 0 {
					q.Unlock()
					return
				}
				k, _, _ := q.NlPop()
				q.Unlock()
				mu.Lock()
				if seen[k] {
					t.Errorf("index %d popped twice", k)
				}
				seen[k] = true
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if total != n {
		t.Errorf("Expected %d popped indices via NlPop, got %d", n, total)
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// jsonItem is a struct value type, to verify that element-level JSON
// encoding is honored through the queue.
type jsonItem struct {
	S string
}

func jsonItemCmp(a, b jsonItem) int { return Compare(a.S, b.S) }

// jsonUpperString is a string with its own JSON representation, to
// verify that value-level marshalers are honored through the queue.
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

// jsonPairsOf drains the queue via All into "k:v" strings in priority
// order — the wire order of MarshalJSON.
func jsonPairsOf[T any](q *IndexPQ[T]) []string {
	var out []string
	for k, v := range q.All() {
		out = append(out, fmt.Sprintf("%d:%v", k, v))
	}
	return out
}

func TestIndexPQMarshalJSON(t *testing.T) {
	// Exact array output in priority order, minimum value first.
	q := NewIndexPQ[int](4)
	q.Insert(0, 30)
	q.Insert(1, 10)
	q.Insert(2, 20)
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("json.Marshal(q): %v", err)
	}
	want := `[{"index":1,"value":10},{"index":2,"value":20},{"index":0,"value":30}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// Struct values use their normal JSON encoding.
	items := NewIndexPQFunc[jsonItem](4, jsonItemCmp)
	items.Insert(0, jsonItem{S: "b"})
	items.Insert(1, jsonItem{S: "a"})
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"index":1,"value":{"S":"a"}},{"index":0,"value":{"S":"b"}}]` {
		t.Errorf(`Unexpected struct-value encoding: (%s, %v)`, b, err)
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

	// Value-level marshalers are honored.
	custom := NewIndexPQFunc[jsonUpperString](4, func(a, b jsonUpperString) int {
		return Compare(string(a), string(b))
	})
	custom.Insert(0, "x")
	custom.Insert(1, "y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `[{"index":0,"value":"X"},{"index":1,"value":"Y"}]` {
		t.Errorf(`Unexpected custom-value encoding: (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewIndexPQFunc[chan int](2, func(a, b chan int) int { return 0 })
	bad.Insert(0, make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a queue of channels.")
	}
}

func TestIndexPQUnmarshalJSON(t *testing.T) {
	// The decoded pairs rebuild the queue; priority order comes back out.
	q := NewIndexPQ[int](4)
	if err := json.Unmarshal([]byte(`[{"index":0,"value":30},{"index":1,"value":10},{"index":2,"value":20}]`), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, q, Compare[int])
	if got, want := fmt.Sprint(jsonPairsOf(q)), "[1:10 2:20 0:30]"; got != want {
		t.Errorf("Expected %s after unmarshal, got %s", want, got)
	}
	if k, v, found := q.Peek(); !found || k != 1 || v != 10 {
		t.Errorf("Expected Peek (1, 10), got (%d, %d, %v)", k, v, found)
	}

	// A round trip rebuilds a structurally sound queue and keeps the
	// comparison function (Insert works on the rebuilt queue).
	items := NewIndexPQFunc[jsonItem](4, jsonItemCmp)
	items.Insert(0, jsonItem{S: "b"})
	items.Insert(1, jsonItem{S: "a"})
	items.Insert(2, jsonItem{S: "c"})
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewIndexPQFunc[jsonItem](4, jsonItemCmp)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again, jsonItemCmp)
	if got, want := fmt.Sprint(jsonPairsOf(again)), "[1:{a} 0:{b} 2:{c}]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if !again.Insert(3, jsonItem{S: "d"}) {
		t.Errorf("Expected Insert to work after unmarshal (comparison function kept).")
	}

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte(`[{"index":3,"value":7}]`), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := q.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}
	if q.Contains(0) || !q.Contains(3) {
		t.Errorf("Expected only index 3 after replacement, pairs %v", jsonPairsOf(q))
	}

	// A repeated index keeps the last value (Insert semantics).
	rep := NewIndexPQ[int](4)
	if err := json.Unmarshal([]byte(`[{"index":1,"value":5},{"index":1,"value":2}]`), rep); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, _ := rep.Value(1); v != 2 || rep.Len() != 1 {
		t.Errorf("Expected a repeated index to keep the last value, got value %d length %d", v, rep.Len())
	}

	// An empty array and null clear the queue.
	full := NewIndexPQ[int](4)
	full.Insert(0, 9)
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the queue.")
	}
	full.Insert(0, 9)
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the queue.")
	}
	checkInvariants(t, full, Compare[int])

	// Value-level unmarshalers are honored.
	custom := NewIndexPQFunc[jsonUpperString](4, func(a, b jsonUpperString) int {
		return Compare(string(a), string(b))
	})
	if err := json.Unmarshal([]byte(`[{"index":0,"value":"x"},{"index":1,"value":"y"}]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(jsonPairsOf(custom)), "[0:x 1:y]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}

	// Decode errors are returned and leave the queue untouched.
	keep := NewIndexPQ[int](4)
	keep.Insert(0, 42)
	for _, badData := range []string{"[1,", `[{"index":0,"value":"x"}]`, `{"index":0}`, "7"} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(jsonPairsOf(keep)), "[0:42]"; got != want {
			t.Errorf("Queue changed after the error on %s: %s", badData, got)
		}
	}

	// An index outside the queue's index space is rejected with an
	// error, also with the queue untouched.
	if err := json.Unmarshal([]byte(`[{"index":9,"value":1}]`), keep); err == nil {
		t.Errorf("Expected an error for an out-of-range index.")
	} else if !strings.Contains(err.Error(), "out of the queue's index space") {
		t.Errorf("Unexpected out-of-range error: %v", err)
	}
	if got, want := fmt.Sprint(jsonPairsOf(keep)), "[0:42]"; got != want {
		t.Errorf("Queue changed after the out-of-range error: %s", got)
	}
	if err := json.Unmarshal([]byte(`[{"index":-1,"value":1}]`), keep); err == nil {
		t.Errorf("Expected an error for a negative index.")
	}
	checkInvariants(t, keep, Compare[int])
}

// TestIndexPQUnmarshalJSONPanics verifies that UnmarshalJSON joins the
// insert family: storing values into a nil or zero-value queue panics
// with a message naming the method and the fix, while [] and null —
// which store nothing — are tolerated everywhere.
func TestIndexPQUnmarshalJSONPanics(t *testing.T) {
	var zero IndexPQ[int]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value queue to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON on a zero-value queue", "NewIndexPQ", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"index":0,"value":1}]`))
	})

	var nilQ *IndexPQ[int]
	if err := nilQ.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil queue to be tolerated, got %v", err)
	}
	expectPanic(t, "UnmarshalJSON on a nil queue", "nil queue", func() {
		_ = nilQ.UnmarshalJSON([]byte(`[{"index":0,"value":1}]`))
	})
}

// TestIndexPQJSONStructField marshals and unmarshals an IndexPQ nested
// in a struct through the encoding/json package.  The queue must be
// created with NewIndexPQ/NewIndexPQFunc before unmarshaling: for a nil
// *IndexPQ field the json package allocates a zero-value queue itself
// (no comparison function), so non-empty data panics with the
// insert-family message.
func TestIndexPQJSONStructField(t *testing.T) {
	type Doc struct {
		Title string           `json:"title"`
		Queue *IndexPQ[string] `json:"queue"`
	}

	d := Doc{Title: "pluto", Queue: NewIndexPQ[string](4)}
	d.Queue.Insert(0, "ds")
	d.Queue.Insert(1, "go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"title":"pluto","queue":[{"index":0,"value":"ds"},{"index":1,"value":"go"}]}`
	if string(b) != want {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created queue field.
	var out Doc
	out.Queue = NewIndexPQ[string](4)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := fmt.Sprint(jsonPairsOf(out.Queue)); got != "[0:ds 1:go]" {
		t.Errorf("Unexpected queue after unmarshal: %s", got)
	}
	checkInvariants(t, out.Queue, Compare[string])
}
