package dll_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: element accessors, delete branches, iterator edges,
// Dump, and a fixed-seed randomized property test cross-checked against a
// slice reference model.  Benchmarks at the bottom.

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

// checkList verifies the structural invariants: length matches a full
// walk, the head's prev and the tail's next are nil, and every link is
// bidirectional.
func checkList(t *testing.T, list *Dll[TestDllItem], where string) {
	t.Helper()

	// Forward walk reaches the tail and counts length nodes.
	n := 0
	var last *DllElement[TestDllItem]
	for p := list.head; p != nil; p = p.next {
		if p.prev != last {
			t.Fatalf("%s: broken prev link at node %d", where, n)
		}
		last = p
		n++
	}
	if n != list.length {
		t.Fatalf("%s: forward walk found %d nodes, length is %d", where, n, list.length)
	}
	if list.tail != last {
		t.Fatalf("%s: tail pointer does not match the last node of the forward walk", where)
	}
	if list.head != nil && list.head.prev != nil {
		t.Fatalf("%s: head.prev is non-nil", where)
	}
	if list.tail != nil && list.tail.next != nil {
		t.Fatalf("%s: tail.next is non-nil", where)
	}

	// Backward walk reaches the head.
	n = 0
	var first *DllElement[TestDllItem]
	for p := list.tail; p != nil; p = p.prev {
		first = p
		n++
	}
	if list.head != first {
		t.Fatalf("%s: backward walk does not reach the head", where)
	}
	if list.length == 0 && (list.head != nil || list.tail != nil) {
		t.Fatalf("%s: empty list has non-nil head/tail", where)
	}
}

// valuesOf returns the current contents of the list, head to tail.
func valuesOf(list *Dll[TestDllItem]) []string {
	var got []string
	for _, v := range list.All() {
		got = append(got, v.S)
	}
	return got
}

func TestElementAccessors(t *testing.T) {
	el := &DllElement[TestDllItem]{}
	el.SetData(TestDllItem{S: "42"})
	if d := el.GetData(); d.S != "42" {
		t.Errorf("Expected GetData to return 42, got %+v", d)
	}

	list := newTestDll()
	list.AppendAtTail(TestDllItem{S: "07"})
	found, pos := list.Search(TestDllItem{S: "07"})
	if found == nil || pos != 0 {
		t.Fatalf("Expected to find the appended item.")
	}
	found.SetData(TestDllItem{S: "08"})
	if v, err := list.Peek(); err != nil || v.S != "08" {
		t.Errorf("SetData on a found element should change the stored value, got (%v, %v)", v, err)
	}
}

// TestDeleteFoundAllBranches exercises every branch of DeleteFound:
// single element, head, tail, middle, and a nil element.
func TestDeleteFoundAllBranches(t *testing.T) {
	// Nil element.
	list := newTestDll()
	if err := list.DeleteFound(nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound from DeleteFound(nil), got %v", err)
	}

	// Single element.
	list.AppendAtTail(TestDllItem{S: "only"})
	el, _ := list.Search(TestDllItem{S: "only"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(single): %v", err)
	}
	if !list.IsEmpty() {
		t.Errorf("Expected empty list after deleting the single element.")
	}
	checkList(t, list, "after single delete")

	// Head and tail of a longer list.
	for _, s := range []string{"a", "b", "c", "d"} {
		list.AppendAtTail(TestDllItem{S: s})
	}
	el, _ = list.Search(TestDllItem{S: "a"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(head): %v", err)
	}
	el, _ = list.Search(TestDllItem{S: "d"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(tail): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b c]"; got != want {
		t.Errorf("After head+tail deletes got %s, expected %s", got, want)
	}
	checkList(t, list, "after head+tail deletes")

	// Middle of a longer list.
	list.AppendAtTail(TestDllItem{S: "x"})
	list.AppendAtTail(TestDllItem{S: "y"})
	el, _ = list.Search(TestDllItem{S: "c"})
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(middle): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b x y]"; got != want {
		t.Errorf("After middle delete got %s, expected %s", got, want)
	}
	checkList(t, list, "after middle delete")

	// Elements from Index/IndexFromTail work with DeleteFound.
	el, err := list.Index(1)
	if err != nil {
		t.Fatalf("Index(1): %v", err)
	}
	if err := list.DeleteFound(el); err != nil {
		t.Fatalf("DeleteFound(Index(1)): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[b y]"; got != want {
		t.Errorf("After Index-based delete got %s, expected %s", got, want)
	}
}

// TestSingleElementEdgeCases covers every operation on a one-element list.
func TestSingleElementEdgeCases(t *testing.T) {
	list := newTestDll()
	list.AppendAtTail(TestDllItem{S: "solo"})

	if v, err := list.Peek(); err != nil || v.S != "solo" {
		t.Errorf("Peek = (%v, %v)", v, err)
	}
	if v, err := list.PeekTail(); err != nil || v.S != "solo" {
		t.Errorf("PeekTail = (%v, %v)", v, err)
	}
	if el, pos := list.Search(TestDllItem{S: "solo"}); el == nil || pos != 0 {
		t.Errorf("Search = (%v, %d)", el, pos)
	}
	if el, err := list.Index(0); err != nil || el.GetData().S != "solo" {
		t.Errorf("Index(0) = (%v, %v)", el, err)
	}
	if el, err := list.IndexFromTail(0); err != nil || el.GetData().S != "solo" {
		t.Errorf("IndexFromTail(0) = (%v, %v)", el, err)
	}

	// Reverse of a single element is a no-op.
	list.Reverse()
	if got := valuesOf(list); !reflect.DeepEqual(got, []string{"solo"}) {
		t.Errorf("After reverse got %v", got)
	}
	checkList(t, list, "after single reverse")

	// Pop it and confirm the drained behavior.
	if v, err := list.Pop(); err != nil || v.S != "solo" {
		t.Errorf("Pop = (%v, %v)", v, err)
	}
	if !list.IsEmpty() || list.Length() != 0 {
		t.Errorf("Expected empty list after popping the only element.")
	}
	checkList(t, list, "after popping the only element")
}

// TestDuplicateValues verifies that duplicates coexist and that Search
// finds the first while ReverseSearch finds the last.
func TestDuplicateValues(t *testing.T) {
	list := newTestDll()
	for _, s := range []string{"x", "y", "x", "z", "x"} {
		list.AppendAtTail(TestDllItem{S: s})
	}

	if _, pos := list.Search(TestDllItem{S: "x"}); pos != 0 {
		t.Errorf("Search(x) pos = %d, expected 0", pos)
	}
	if _, pos := list.ReverseSearch(TestDllItem{S: "x"}); pos != 4 {
		t.Errorf("ReverseSearch(x) pos = %d, expected 4", pos)
	}

	// Deleting by value removes one at a time.
	if err := list.Delete(TestDllItem{S: "x"}); err != nil {
		t.Fatalf("Delete(x): %v", err)
	}
	if got, want := fmt.Sprint(valuesOf(list)), "[y x z x]"; got != want {
		t.Errorf("After delete got %s, expected %s", got, want)
	}
	checkList(t, list, "after duplicate delete")
}

// TestIteratorEdgeCases covers the legacy iterator's edges: Value/Next/
// Prev on an exhausted iterator, and Prev off the head.
func TestIteratorEdgeCases(t *testing.T) {
	// Empty list: Front is immediately Done; Next/Prev are no-ops.
	empty := newTestDll()
	it := empty.Front()
	if !it.Done() {
		t.Errorf("Expected Front on empty list to be Done.")
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value on empty list iterator.")
	}
	it.Next()
	it.Prev()
	if !it.Done() {
		t.Errorf("Expected Done to hold after Next/Prev on exhausted iterator.")
	}

	list := newTestDll()
	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(TestDllItem{S: s})
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

	// Prev off the head ends the iteration with a negative Pos.
	it = list.Front()
	it.Prev()
	if !it.Done() {
		t.Errorf("Expected Done after Prev off the head.")
	}
	if it.Pos() != -1 {
		t.Errorf("Expected Pos -1 after Prev off the head, got %d", it.Pos())
	}
}

// TestSearchWalkOnEmpty verifies every read on an empty list.
func TestSearchWalkOnEmpty(t *testing.T) {
	list := newTestDll()

	if el, pos := list.Search(TestDllItem{S: "a"}); el != nil || pos != -1 {
		t.Errorf("Search on empty: (%v, %d)", el, pos)
	}
	if el, pos := list.ReverseSearch(TestDllItem{S: "a"}); el != nil || pos != -1 {
		t.Errorf("ReverseSearch on empty: (%v, %d)", el, pos)
	}
	if el, pos := list.Walk(func(pos int, data TestDllItem) bool { return true }); el != nil || pos != -1 {
		t.Errorf("Walk on empty: (%v, %d)", el, pos)
	}
	if el, pos := list.ReverseWalk(func(pos int, data TestDllItem) bool { return true }); el != nil || pos != -1 {
		t.Errorf("ReverseWalk on empty: (%v, %d)", el, pos)
	}
	if _, err := list.Index(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Index on empty: %v", err)
	}
	if _, err := list.IndexFromTail(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("IndexFromTail on empty: %v", err)
	}
}

// TestDump verifies the debugging output.
func TestDump(t *testing.T) {
	list := newTestDll()
	var buf bytes.Buffer
	list.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty list, got %q", buf.String())
	}

	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(TestDllItem{S: s})
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

// TestModelRandomized runs thousands of mixed operations against a plain
// slice reference model with a fixed seed, cross-checking after every
// step.
func TestModelRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 42))
	const ops = 4000
	const keySpace = 40 // small space forces duplicates

	list := newTestDll()
	var model []string // head at index 0

	check := func(step int) {
		t.Helper()
		if list.Length() != len(model) {
			t.Fatalf("step %d: length %d, model has %d", step, list.Length(), len(model))
		}
		if got, want := fmt.Sprint(valuesOf(list)), fmt.Sprint(model); got != want {
			t.Fatalf("step %d: contents %s, model %s", step, got, want)
		}
		checkList(t, list, fmt.Sprintf("step %d", step))
	}

	for step := range ops {
		s := fmt.Sprintf("%02d", rng.IntN(keySpace))
		switch rng.IntN(8) {
		case 0: // Push (head)
			list.Push(TestDllItem{S: s})
			model = append([]string{s}, model...)
		case 1: // AppendAtTail
			list.AppendAtTail(TestDllItem{S: s})
			model = append(model, s)
		case 2: // Pop
			v, err := list.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyDll) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil || v.S != model[0] {
					t.Fatalf("step %d: Pop = (%v, %v), model head %s", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // PopTail
			v, err := list.PopTail()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyDll) {
					t.Fatalf("step %d: PopTail on empty returned %v", step, err)
				}
			} else {
				if err != nil || v.S != model[len(model)-1] {
					t.Fatalf("step %d: PopTail = (%v, %v), model tail %s", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case 4: // Delete by value (first match)
			err := list.Delete(TestDllItem{S: s})
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
		case 5: // Search position
			_, pos := list.Search(TestDllItem{S: s})
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
		case 6: // Index round trip
			if len(model) > 0 {
				sub := rng.IntN(len(model))
				el, err := list.Index(sub)
				if err != nil || el.GetData().S != model[sub] {
					t.Fatalf("step %d: Index(%d) = (%v, %v), model %s", step, sub, el, err, model[sub])
				}
			}
		case 7: // Trim to a random prefix
			if len(model) > 0 {
				n := rng.IntN(len(model) + 2)
				if err := list.Trim(n); err != nil {
					t.Fatalf("step %d: Trim(%d): %v", step, n, err)
				}
				if n <= 0 {
					model = nil
				} else if n < len(model) {
					model = model[:n]
				}
			}
		}
		if step%50 == 0 {
			check(step)
		}
	}
	check(ops)
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestDllIterateSnapshot verifies that the All/Backward iterators operate
// on a snapshot taken when they are called: later modifications — even
// truncating the whole list — are not observed, and mutating the list from
// inside the loop is safe.
func TestDllIterateSnapshot(t *testing.T) {
	list := newTestDll()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		list.AppendAtTail(TestDllItem{S: s})
	}

	all := list.All()
	backward := list.Backward()

	list.Truncate() // the iterators above must not observe this

	// A list preserves insertion order (unlike the trees, which sort).
	expect := []string{"05", "02", "09", "00", "03"}

	var got []string
	for _, v := range all {
		got = append(got, v.S)
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("All after Truncate error, expected %v got %v", expect, got)
	}

	var gotB []string
	for _, v := range backward {
		gotB = append(gotB, v.S)
	}
	for i := range expect {
		if gotB[i] != expect[len(expect)-1-i] {
			t.Fatalf("Backward after Truncate error, expected reverse of %v got %v", expect, gotB)
		}
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	list = newTestDll()
	for _, s := range []string{"05", "02", "09"} {
		list.AppendAtTail(TestDllItem{S: s})
	}
	visited := 0
	for _, v := range list.All() {
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

// TestDllConcurrent runs writers (each owning a disjoint key range)
// against a reader that iterates snapshots and queries metadata in a
// tight loop.  It is primarily a test for the race detector (`make
// race`); it also verifies that every operation reports success and that
// the list ends up empty and structurally sound.
func TestDllConcurrent(t *testing.T) {
	list := NewDllFunc(eqTestDllItem)

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
			for range list.All() {
			}
			for range list.Backward() {
			}
			_ = list.Len()
			_ = list.IsEmpty()
			_, _ = list.Peek()
			_, _ = list.PeekTail()
			_, _ = list.Index(0)
		}
	}()

	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range perWorker {
				k := TestDllItem{S: fmt.Sprintf("%02d-%04d", w, i)}
				if !list.AppendAtTail(k) {
					t.Errorf("worker %d: AppendAtTail(%s) returned false", w, k.S)
					return
				}
			}
			for i := range perWorker {
				k := TestDllItem{S: fmt.Sprintf("%02d-%04d", w, i)}
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
	checkList(t, list, "after concurrent drain")
}

// TestDllConcurrentLock covers the exposed Lock/Unlock pair: a compound
// search-and-insert sequence under the manual lock must be atomic with
// respect to other writers.  The compound operation uses the in-package
// lock-free internals because the locked public methods must not be
// called while the lock is held.
func TestDllConcurrentLock(t *testing.T) {
	list := NewDll[int]()

	const workers = 8
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 100 {
				list.Lock()
				found := false
				for p := list.head; p != nil; p = p.next {
					if list.equal(p.data, i) {
						found = true
						break
					}
				}
				if !found {
					list.noLockInsertBeforeHead(i)
				}
				list.Unlock()
			}
		}(w)
	}
	wg.Wait()

	if list.Length() != 100 {
		t.Errorf("Expected exactly 100 distinct insertions under the manual lock, got %d", list.Length())
	}
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
	list := NewDll[int]()
	for _, v := range []int{3, 1, 2} {
		list.AppendAtTail(v)
	}
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("json.Marshal(list): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := newTestDll()
	for _, s := range []string{"a", "b"} {
		items.AppendAtTail(TestDllItem{S: s})
	}
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty list encodes as [].
	if b, err := json.Marshal(NewDll[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty list, got (%s, %v)", b, err)
	}

	// A zero-value list is a tolerated read: [].
	var zero Dll[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value list, got (%s, %v)", b, err)
	}

	// A direct call on a nil list encodes as []; json.Marshal on a nil
	// *Dll never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilList *Dll[int]
	if b, err := nilList.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-list call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilList); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil list, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewDll[upperString]()
	custom.AppendAtTail("x")
	custom.AppendAtTail("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewDll[chan int]()
	bad.AppendAtTail(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a list of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the head.
	list := NewDll[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), list); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []int
	for _, v := range list.All() {
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
	items := newTestDll()
	for _, s := range []string{"a", "b", "c"} {
		items.AppendAtTail(TestDllItem{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestDll()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkList(t, again, "after unmarshal")
	if got, want := fmt.Sprint(valuesOf(again)), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if _, pos := again.Search(TestDllItem{S: "b"}); pos != 1 {
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
	full := newTestDll()
	full.AppendAtTail(TestDllItem{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the list.")
	}
	full.AppendAtTail(TestDllItem{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the list.")
	}
	checkList(t, full, "after null")

	// Element-level unmarshalers are honored.
	custom := NewDll[upperString]()
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

	// Decode errors are returned and leave the list untouched.
	keep := newTestDll()
	keep.AppendAtTail(TestDllItem{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(valuesOf(keep)), "[keep]"; got != want {
			t.Errorf("List changed after the error on %s: %s", badData, got)
		}
	}
	checkList(t, keep, "after decode errors")
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value list panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Dll[TestDllItem]
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewDll") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilList *Dll[TestDllItem]
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

// TestJSONStructField marshals and unmarshals a Dll nested in a struct
// through the encoding/json package.  The list must be created with
// NewDll/NewDllFunc before unmarshaling: for a nil *Dll field the json
// package allocates a zero-value list itself (no equality function), so
// non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string       `json:"title"`
		Tags  *Dll[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewDll[string]()}
	d.Tags.AppendAtTail("ds")
	d.Tags.AppendAtTail("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created list field.
	var out Doc
	out.Tags = NewDll[string]()
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

	// A nil list field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created list and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewDll[string]()}
	clearDoc.Tags.AppendAtTail("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the list.")
	}

	// Non-empty data into a nil *Dll field: the json package allocates a
	// zero-value list, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated list field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDll") {
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
	rng := rand.New(rand.NewPCG(20260902, 42))
	const ops = 500

	list := NewDll[int]()
	model := []int{} // non-nil, so an emptied model marshals as [] like the list

	for step := range ops {
		switch rng.IntN(4) {
		case 0:
			v := rng.IntN(100)
			list.AppendAtTail(v)
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
			if len(model) > 0 {
				v, err := list.PopTail()
				if err != nil || v != model[len(model)-1] {
					t.Fatalf("step %d: PopTail = (%v, %v), model %d", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
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
		fresh := NewDll[int]()
		if err := json.Unmarshal(got, fresh); err != nil {
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

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with writers and a marshaling reader; every output must be a valid
// JSON array.  Run under -race (make race).
func TestJSONConcurrent(t *testing.T) {
	list := NewDllFunc(eqTestDllItem)

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
			var probe []TestDllItem
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
				item := TestDllItem{S: fmt.Sprintf("%02d-%03d", w, i)}
				b, err := json.Marshal([]TestDllItem{item})
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
	checkList(t, list, "after concurrent JSON")
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkListSize = 4096

func BenchmarkPush(b *testing.B) {
	list := NewDll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Push(i)
	}
}

func BenchmarkAppendAtTail(b *testing.B) {
	list := NewDll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.AppendAtTail(i)
	}
}

func BenchmarkPop(b *testing.B) {
	list := NewDll[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if list.IsEmpty() {
			list.Truncate()
		}
		_, _ = list.Pop()
	}
}

func BenchmarkSearch(b *testing.B) {
	list := NewDll[int]()
	for i := range benchmarkListSize {
		list.AppendAtTail(i)
	}
	find := benchmarkListSize / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Search(find)
	}
}

func BenchmarkAll(b *testing.B) {
	list := NewDll[int]()
	for i := range benchmarkListSize {
		list.AppendAtTail(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range list.All() {
		}
	}
}
