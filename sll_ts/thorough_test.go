package sll_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: structural invariants, single-element and duplicate
// edge cases, iterator edges, Dump, and a fixed-seed randomized property
// test cross-checked against a slice reference model.  Benchmarks at the
// bottom.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// checkInvariants walks the list and verifies that the internal structure
// is consistent: node count equals Length(), head/tail are nil exactly
// when the list is empty, and tail is the last node reachable from head.
func checkInvariants(t *testing.T, ns *Sll[TestSllItem], context string) {
	t.Helper()
	n := 0
	var last *SllElement[TestSllItem]
	for p := ns.head; p != nil; p = p.next {
		last = p
		n++
	}
	if n != ns.length {
		t.Errorf("%s: walked %d nodes but Length() reports %d", context, n, ns.length)
	}
	if ns.length == 0 {
		if ns.head != nil || ns.tail != nil {
			t.Errorf("%s: empty list must have nil head and tail", context)
		}
	} else {
		if ns.head == nil || ns.tail == nil {
			t.Errorf("%s: non-empty list must have non-nil head and tail", context)
		}
		if ns.tail != last {
			t.Errorf("%s: tail pointer does not point at the last node", context)
		}
		if ns.tail.next != nil {
			t.Errorf("%s: tail node must have nil next", context)
		}
	}
}

// TestSingleElementList exercises every operation on a one-element list.
func TestSingleElementList(t *testing.T) {
	list := newTestSll()
	list.InsertAfterTail(TestSllItem{S: "solo"})

	if v, err := list.Peek(); err != nil || v.S != "solo" {
		t.Errorf("Peek = (%v, %v)", v, err)
	}
	if el, pos := list.Search(TestSllItem{S: "solo"}); el == nil || pos != 0 {
		t.Errorf("Search = (%v, %d)", el, pos)
	}

	// Reverse of a single element is a no-op.
	list.Reverse()
	if got := valuesOf(list); len(got) != 1 || got[0] != "solo" {
		t.Errorf("After single reverse got %v", got)
	}
	checkInvariants(t, list, "after single reverse")

	// Pop it and confirm the drained behavior.
	if v, err := list.Pop(); err != nil || v.S != "solo" {
		t.Errorf("Pop = (%v, %v)", v, err)
	}
	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after popping the only element.")
	}
	checkInvariants(t, list, "after popping the only element")
}

// TestDuplicates verifies that duplicates coexist, that Search finds the
// first, and that Delete removes one at a time.
func TestDuplicates(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"x", "y", "x", "z", "x"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	if _, pos := list.Search(TestSllItem{S: "x"}); pos != 0 {
		t.Errorf("Search(x) pos = %d, expected 0", pos)
	}

	if err := list.Delete(TestSllItem{S: "x"}); err != nil {
		t.Fatalf("Delete(x): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[y x z x]"; got != want {
		t.Errorf("After delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after duplicate delete")
}

// TestDeleteFoundEdgeCases exercises the delete paths: head, tail,
// middle, not-found, and the special error cases.
func TestDeleteFoundEdgeCases(t *testing.T) {
	list := newTestSll()

	// Empty list.
	if err := list.DeleteFound(nil); !errors.Is(err, ErrEmptySll) {
		t.Errorf("Expected ErrEmptySll from DeleteFound on empty list, got %v", err)
	}

	for _, s := range []string{"a", "b", "c", "d"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	// Head.
	el, _ := list.Search(TestSllItem{S: "a"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(head): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b c d]"; got != want {
		t.Errorf("After head delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after head delete")

	// Tail.
	el, _ = list.Search(TestSllItem{S: "d"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(tail): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b c]"; got != want {
		t.Errorf("After tail delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after tail delete")

	// Middle.
	list.InsertAfterTail(TestSllItem{S: "e"})
	el, _ = list.Search(TestSllItem{S: "c"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(middle): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b e]"; got != want {
		t.Errorf("After middle delete got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after middle delete")

	// A stale element whose data no longer matches anything.
	if err := list.DeleteFound(el); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from DeleteFound of stale element, got %v", err)
	}
}

// TestCursorIteratorEdgeCases covers the cursor iterator's edges:
// Value/Next on an exhausted iterator, and starting positions.
func TestCursorIteratorEdgeCases(t *testing.T) {
	// Empty list: Front is immediately Done; Next is a no-op.
	empty := newTestSll()
	it := empty.Front()
	if !it.Done() {
		t.Errorf("Expected Front on empty list to be Done.")
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value on empty list iterator.")
	}
	it.Next() // must not panic
	if !it.Done() {
		t.Errorf("Expected Done to hold after Next on exhausted iterator.")
	}

	list := newTestSll()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	// Exhaust the iterator, then keep calling Next.
	it = list.Front()
	steps := 0
	for !it.Done() {
		it.Next()
		steps++
	}
	if steps != 3 {
		t.Errorf("Expected 3 steps to exhaust, got %d", steps)
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value after exhaustion.")
	}
	it.Next() // no-op, not a panic
	if !it.Done() {
		t.Errorf("Expected Done to hold after extra Next.")
	}
	if it.Pos() != 3 {
		t.Errorf("Expected Pos 3 after exhaustion, got %d", it.Pos())
	}

	// Current with a nil element starts done.
	if it := list.Current(nil, 0); !it.Done() {
		t.Errorf("Expected Current(nil, 0) to be Done immediately.")
	}
}

// TestDump verifies the debugging output.
func TestDump(t *testing.T) {
	list := newTestSll()
	var buf bytes.Buffer
	list.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty list, got %q", buf.String())
	}

	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	buf.Reset()
	list.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines from Dump, got %d: %q", len(lines), out)
	}
	for i, want := range []string{"0: {S:a}", "1: {S:b}", "2: {S:c}"} {
		if lines[i] != want {
			t.Errorf("Dump line %d: expected %q, got %q", i, want, lines[i])
		}
	}
}

// TestTruncateReuse verifies that the list is fully reusable after a
// truncate, including the insert-at-tail path.
func TestTruncateReuse(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	list.Truncate()

	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after Truncate.")
	}
	checkInvariants(t, list, "after Truncate")

	// Reusable after the drain, with both insertion paths.
	list.Push(TestSllItem{S: "p"})
	list.InsertAfterTail(TestSllItem{S: "t"})
	if got, want := fmt.Sprint(valuesOf(list)), "[p t]"; got != want {
		t.Errorf("After refill got %s, expected %s", got, want)
	}
	checkInvariants(t, list, "after refill")

	// Truncating an already-empty list is fine.
	list.Truncate()
	list.Truncate()
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after double Truncate.")
	}
}

// TestModelRandomized runs thousands of mixed operations against a plain
// slice reference model with a fixed seed, cross-checking after every
// step.
func TestModelRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 7))
	const ops = 4000
	const keySpace = 40 // small space forces duplicates

	list := newTestSll()
	var model []string // head at index 0

	check := func(step int) {
		t.Helper()
		if list.Length() != len(model) {
			t.Fatalf("step %d: length %d, model has %d", step, list.Length(), len(model))
		}
		if got, want := fmt.Sprint(valuesOf(list)), fmt.Sprint(model); got != want {
			t.Fatalf("step %d: contents %s, model %s", step, got, want)
		}
		checkInvariants(t, list, fmt.Sprintf("step %d", step))
	}

	for step := range ops {
		s := fmt.Sprintf("%02d", rng.IntN(keySpace))
		switch rng.IntN(6) {
		case 0: // Push (head)
			list.Push(TestSllItem{S: s})
			model = append([]string{s}, model...)
		case 1: // InsertAfterTail
			list.InsertAfterTail(TestSllItem{S: s})
			model = append(model, s)
		case 2: // Pop
			v, err := list.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptySll) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("step %d: Pop = (%v, %v), model head %s", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // Delete by value (first match)
			err := list.Delete(TestSllItem{S: s})
			idx := -1
			for i, m := range model {
				if m == s {
					idx = i
					break
				}
			}
			if idx < 0 {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("step %d: Delete(%s) = %v, model says absent", step, s, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Delete(%s): %v", step, s, err)
				}
				model = append(model[:idx], model[idx+1:]...)
			}
		case 4: // Search position
			_, pos := list.Search(TestSllItem{S: s})
			want := -1
			for i, m := range model {
				if m == s {
					want = i
					break
				}
			}
			if pos != want {
				t.Fatalf("step %d: Search(%s) pos %d, model says %d", step, s, pos, want)
			}
		case 5: // Reverse
			list.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		}
		if step%50 == 0 {
			check(step)
		}
	}
	check(ops)
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterator and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestSllIterateSnapshot verifies that IterateOver operates on a snapshot
// taken when it is called: later modifications — even truncating the whole
// list — are not observed, and mutating the list from inside the loop is
// safe.
func TestSllIterateSnapshot(t *testing.T) {
	list := newTestSll()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}

	over := list.IterateOver()

	list.Truncate() // the iterator above must not observe this

	// A list preserves insertion order (unlike the trees, which sort).
	expect := []string{"05", "02", "09", "00", "03"}

	var got []string
	for _, v := range over {
		got = append(got, v.S)
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("IterateOver after Truncate error, expected %v got %v", expect, got)
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	list = newTestSll()
	for _, s := range []string{"05", "02", "09"} {
		list.InsertAfterTail(TestSllItem{S: s})
	}
	visited := 0
	for _, v := range list.IterateOver() {
		visited++
		if err := list.Delete(v); err != nil {
			t.Errorf("Delete(%s) during iteration: %v", v.S, err)
		}
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while deleting during iteration, got %d", visited)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting every visited element.")
	}
}

// TestSllConcurrent runs writers (each owning a disjoint key range)
// against a reader that iterates snapshots and queries metadata in a
// tight loop.  It is primarily a test for the race detector (`make
// race`); it also verifies that every operation reports success and that
// the list ends up empty and structurally sound.
func TestSllConcurrent(t *testing.T) {
	list := NewSllFunc(eqTestSllItem)

	const workers = 8
	const perWorker = 200

	stop := make(chan struct{})
	var writers sync.WaitGroup

	// Reader: iterate snapshots and query metadata while the writers work.
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for range list.IterateOver() {
			}
			_ = list.Len()
			_ = list.IsEmpty()
			_, _ = list.Peek()
		}
	}()

	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range perWorker {
				k := TestSllItem{S: fmt.Sprintf("%02d-%04d", w, i)}
				list.InsertAfterTail(k)
			}
			for i := range perWorker {
				k := TestSllItem{S: fmt.Sprintf("%02d-%04d", w, i)}
				if _, pos := list.Search(k); pos < 0 {
					t.Errorf("worker %d: Search(%s) not found", w, k.S)
				}
				if err := list.Delete(k); err != nil {
					t.Errorf("worker %d: Delete(%s): %v", w, k.S, err)
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)

	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after concurrent insert/delete, got length %d", list.Length())
	}
	checkInvariants(t, list, "after concurrent drain")
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the list.
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
	list := NewSll[int]()
	for _, v := range []int{3, 1, 2} {
		list.InsertAfterTail(v)
	}
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("json.Marshal(list): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := newTestSll()
	for _, s := range []string{"a", "b"} {
		items.InsertAfterTail(TestSllItem{S: s})
	}
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty list encodes as [].
	if b, err := json.Marshal(NewSll[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty list, got (%s, %v)", b, err)
	}

	// A zero-value list is a tolerated read: [].
	var zero Sll[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value list, got (%s, %v)", b, err)
	}

	// A direct call on a nil list encodes as []; json.Marshal on a nil
	// *Sll never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilList *Sll[int]
	if b, err := nilList.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-list call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilList); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil list, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewSll[upperString]()
	custom.InsertAfterTail("x")
	custom.InsertAfterTail("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewSll[chan int]()
	bad.InsertAfterTail(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a list of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the head.
	list := NewSll[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), list); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []int
	for _, v := range list.IterateOver() {
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[3 1 2]" {
		t.Errorf("Expected [3 1 2], got %v", got)
	}
	if head, err := list.Peek(); err != nil || head != 3 {
		t.Errorf("Expected head 3, got (%v, %v)", head, err)
	}

	// A round trip rebuilds a structurally sound list and keeps the
	// equality function (Search works on the rebuilt list).
	items := newTestSll()
	for _, s := range []string{"a", "b", "c"} {
		items.InsertAfterTail(TestSllItem{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestSll()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again, "after unmarshal")
	if got, want := fmt.Sprint(valuesOf(again)), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if _, pos := again.Search(TestSllItem{S: "b"}); pos != 1 {
		t.Errorf("Expected Search to work after unmarshal, got pos %d", pos)
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte("[7]"), list); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := list.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the list.
	full := newTestSll()
	full.InsertAfterTail(TestSllItem{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the list.")
	}
	full.InsertAfterTail(TestSllItem{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the list.")
	}
	checkInvariants(t, full, "after null")

	// Element-level unmarshalers are honored.
	custom := NewSll[upperString]()
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for _, v := range custom.IterateOver() {
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the list untouched.
	keep := newTestSll()
	keep.InsertAfterTail(TestSllItem{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(valuesOf(keep)), "[keep]"; got != want {
			t.Errorf("List changed after the error on %s: %s", badData, got)
		}
	}
	checkInvariants(t, keep, "after decode errors")
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value list panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Sll[TestSllItem]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value list to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value list.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewSll") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilList *Sll[TestSllItem]
	if err := nilList.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil list to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil list.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil list") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilList.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()
}

// TestJSONStructField marshals and unmarshals a Sll nested in a struct
// through the encoding/json package.  The list must be created with
// NewSll/NewSllFunc before unmarshaling: for a nil *Sll field the json
// package allocates a zero-value list itself (no equality function), so
// non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string       `json:"title"`
		Tags  *Sll[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewSll[string]()}
	d.Tags.InsertAfterTail("ds")
	d.Tags.InsertAfterTail("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created list field.
	var out Doc
	out.Tags = NewSll[string]()
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for _, v := range out.Tags.IterateOver() {
		tags = append(tags, v)
	}
	if fmt.Sprint(tags) != "[ds go]" {
		t.Errorf("Expected [ds go], got %v", tags)
	}

	// A nil list field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created list and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewSll[string]()}
	clearDoc.Tags.InsertAfterTail("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the list.")
	}

	// Non-empty data into a nil *Sll field: the json package allocates a
	// zero-value list, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated list field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSll") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 500

	list := NewSll[int]()
	model := []int{} // non-nil, so an emptied model marshals as [] like the list

	for step := range ops {
		switch rng.IntN(4) {
		case 0:
			v := rng.IntN(100)
			list.InsertAfterTail(v)
			model = append(model, v)
		case 1:
			v := rng.IntN(100)
			list.Push(v)
			model = append([]int{v}, model...)
		case 2:
			if len(model) > 0 {
				v, err := list.Pop()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: Pop = (%v, %v), model %d", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3:
			list.Reverse()
			for i, j := 0, len(model)-1; i < j; i, j = i+1, j-1 {
				model[i], model[j] = model[j], model[i]
			}
		}

		// Marshal must equal the model marshaled as a plain slice.
		got, err := json.Marshal(list)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(model)
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh list must reproduce the model.
		fresh := NewSll[int]()
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		var vals []int
		for _, v := range fresh.IterateOver() {
			vals = append(vals, v)
		}
		if fmt.Sprint(vals) != fmt.Sprint(model) {
			t.Fatalf("step %d: round trip got %v, model %v", step, vals, model)
		}
	}
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with writers and a marshaling reader; every output must be a valid
// JSON array.  Run under -race (go test -race).
func TestJSONConcurrent(t *testing.T) {
	list := NewSllFunc(eqTestSllItem)

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
			b, err := list.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON: %v", err)
				return
			}
			var probe []TestSllItem
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
				item := TestSllItem{S: fmt.Sprintf("%02d-%03d", w, i)}
				b, err := json.Marshal([]TestSllItem{item})
				if err != nil {
					t.Errorf("worker %d: %v", w, err)
					return
				}
				if err := list.UnmarshalJSON(b); err != nil {
					t.Errorf("worker %d: UnmarshalJSON: %v", w, err)
					return
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)
	readers.Wait()
	checkInvariants(t, list, "after concurrent JSON")
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkListSize = 4096

func BenchmarkInsertBeforeHead(b *testing.B) {
	list := NewSll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.InsertBeforeHead(i)
	}
}

func BenchmarkInsertAfterTail(b *testing.B) {
	list := NewSll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.InsertAfterTail(i)
	}
}

func BenchmarkPop(b *testing.B) {
	list := NewSll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if list.IsEmpty() {
			list.Truncate()
		}
		_, _ = list.Pop()
	}
}

func BenchmarkSearch(b *testing.B) {
	list := NewSll[int]()
	for i := range benchmarkListSize {
		list.InsertAfterTail(i)
	}
	find := benchmarkListSize / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Search(find)
	}
}

func BenchmarkIterateOver(b *testing.B) {
	list := NewSll[int]()
	for i := range benchmarkListSize {
		list.InsertAfterTail(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range list.IterateOver() {
		}
	}
}
