package skip_list_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization, deterministic rendering, deep copy, membership
// batches, predicate bulk delete, the nil/zero-value read contract, and
// the span machinery surviving the batch paths.  Tower heights come from
// the global RNG and are never asserted here.  The suite mirrors
// skip_list's capabilities_test.go; the concurrency tests at the bottom
// are the twin's additions (`go test -race` is the real assertion).

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// checkCapLevel0 verifies what the capability paths can disturb for any
// element type: the level-0 chain is strictly ascending and holds exactly
// Length nodes.  (The full tower/span check is thorough_test.go's typed
// checkInvariant; heights are RNG-drawn and never asserted.)
func checkCapLevel0[T any](t *testing.T, tt *SkipList[T], where string) {
	t.Helper()
	if tt == nil {
		return
	}
	n := 0
	var pv T
	have := false
	if tt.head != nil {
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			if have && tt.cmp(pv, cur.data) >= 0 {
				t.Errorf("%s: level-0 order violated at element %d", where, n)
				return
			}
			pv = cur.data
			have = true
			n++
		}
	}
	if n != tt.Length() {
		t.Errorf("%s: level-0 chain has %d nodes, Length=%d", where, n, tt.Length())
	}
}

func TestCapabilitiesContains(t *testing.T) {
	list := NewSkipList[string]()
	list.Insert("alpha")
	if !list.Contains("alpha") {
		t.Errorf("Contains should find a stored element")
	}
	if list.Contains("beta") {
		t.Errorf("Contains should not find a missing element")
	}
	var nilList *SkipList[string]
	if nilList.Contains("alpha") {
		t.Errorf("Contains on a nil list should be false")
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	list := newTestList()
	list.Insert(TestSkipListNode{S: "00000020"})
	list.Insert(TestSkipListNode{S: "00000010"})
	list.Insert(TestSkipListNode{S: "00000030"})
	list.Insert(TestSkipListNode{S: "00000010", N: 7}) // replace: still 3 elements

	got := list.Slice()
	if len(got) != 3 {
		t.Fatalf("Slice length = %d, want 3", len(got))
	}
	// Slice is the level-0 lane: ascending without any sorting.
	if got[0].S != "00000010" || got[1].S != "00000020" || got[2].S != "00000030" {
		t.Errorf("Slice order = [%s %s %s], want ascending", got[0].S, got[1].S, got[2].S)
	}
	// Replacement keeps the last value.
	if got[0].N != 7 {
		t.Errorf("Slice should carry the replaced value, got N=%d, want 7", got[0].N)
	}

	// The slice is a copy: mutating it must not affect the list.
	got[0] = TestSkipListNode{S: "zzzzzzzz"}
	if _, found := list.Search(TestSkipListNode{S: "zzzzzzzz"}); found {
		t.Errorf("mutating the Slice result changed the list")
	}

	if empty := newTestList().Slice(); len(empty) != 0 {
		t.Errorf("Slice of an empty list should be empty, got %d", len(empty))
	}
	var nilList *SkipList[TestSkipListNode]
	if s := nilList.Slice(); s != nil {
		t.Errorf("Slice of a nil list should be nil, got %v", s)
	}

	// A drained list snapshots to nil — the twin's toSlice returns nil
	// for any empty list (unlike the plain package's drained sentinel).
	drained := NewSkipList[int]()
	drained.Insert(1)
	drained.Delete(1)
	if s := drained.Slice(); s != nil {
		t.Errorf("Slice of a drained list should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed lists holding the same elements must
	// render identically — the level-0 walk is tower-independent.
	a := NewSkipList[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		a.Insert(s)
	}
	b := NewSkipList[string]()
	for _, s := range []string{"fig", "pear", "apple"} {
		b.Insert(s)
	}
	if a.String() != b.String() {
		t.Errorf("same elements must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[apple fig pear]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewSkipList[string]().String(); got != "[]" {
		t.Errorf("String of an empty list = %q, want []", got)
	}
	var nilList *SkipList[string]
	if got := nilList.String(); got != "[]" {
		t.Errorf("String of a nil list = %q, want []", got)
	}

	numbers := NewSkipList[int]()
	numbers.Insert(9)
	numbers.Insert(2)
	if got, want := numbers.String(), "[2 9]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	list := NewSkipList[int]()
	list.Insert(5)
	list.Insert(8)
	got := list.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := list.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesClone(t *testing.T) {
	list := NewSkipList[int]()
	for i := 50; i > 0; i -= 7 {
		list.Insert(i)
	}
	checkCapLevel0(t, list, "Clone source")

	dup := list.Clone()
	if !dup.Equal(list) || !list.Equal(dup) {
		t.Errorf("clone must be equal to the receiver")
	}
	checkCapLevel0(t, dup, "Clone result")

	// The clone's spans are real: the positional queries agree with the
	// receiver's on every index.
	for i := 0; i < list.Len(); i++ {
		want, _ := list.AtIndex(i)
		got, found := dup.AtIndex(i)
		if !found || got != want {
			t.Fatalf("clone AtIndex(%d) = %d,%v; want %d", i, got, found, want)
		}
	}
	if _, found := dup.Rank(50); !found {
		t.Errorf("clone Rank(50) should be found")
	}

	// Deep copy: mutate the clone, the receiver must not move.
	dup.Insert(3)
	dup.Delete(50)
	if list.Len() != 8 || !list.Contains(50) || list.Contains(3) {
		t.Errorf("mutating the clone changed the receiver: len=%d", list.Len())
	}

	// And the other direction.
	list.Insert(2)
	if dup.Contains(2) {
		t.Errorf("mutating the receiver changed the clone")
	}

	// The clone carries the comparison function (usable for further
	// inserts) — a fresh zero-value list would panic here.
	dup.Insert(1000)

	var nilList *SkipList[int]
	if c := nilList.Clone(); c != nil {
		t.Errorf("Clone of a nil list should be nil, got %v", c)
	}
	var zero SkipList[int]
	if c := zero.Clone(); c.Len() != 0 {
		t.Errorf("Clone of a zero-value list should be empty")
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// Same elements inserted in different orders: equal.
	a := NewSkipList[int]()
	for _, k := range []int{1, 2, 3, 4} {
		a.Insert(k)
	}
	b := NewSkipList[int]()
	for _, k := range []int{4, 3, 2, 1} {
		b.Insert(k)
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("lists with the same elements must be Equal (both directions)")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Different lengths.
	c := NewSkipList[int]()
	c.Insert(1)
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("different lengths must not be Equal")
	}

	// Same length, different elements.
	d := NewSkipList[int]()
	for _, k := range []int{1, 2, 3, 99} {
		d.Insert(k)
	}
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("same length but different elements must not be Equal")
	}

	// Empty lists, nil lists.
	empty := NewSkipList[int]()
	if !empty.Equal(NewSkipList[int]()) {
		t.Errorf("two empty lists must be Equal")
	}
	var nilList *SkipList[int]
	if !empty.Equal(nilList) || !nilList.Equal(empty) {
		t.Errorf("an empty list and a nil list must be Equal")
	}
	if !nilList.Equal(nilList) {
		t.Errorf("nil must Equal nil")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	list := NewSkipList[int]()
	for _, v := range []int{1, 2, 3} {
		list.Insert(v)
	}
	if !list.ContainsSlice([]int{1, 2, 3}) {
		t.Errorf("ContainsSlice should be true when all present")
	}
	if !list.ContainsSlice([]int{3, 1}) {
		t.Errorf("ContainsSlice should be true for a subset")
	}
	if list.ContainsSlice([]int{1, 9}) {
		t.Errorf("ContainsSlice should be false when one is missing")
	}
	if !list.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice of an empty slice is vacuously true")
	}
	if !list.ContainsAny([]int{9, 2}) {
		t.Errorf("ContainsAny should be true when one is present")
	}
	if list.ContainsAny([]int{9, 8}) {
		t.Errorf("ContainsAny should be false when none present")
	}
	if list.ContainsAny([]int{}) {
		t.Errorf("ContainsAny of an empty slice should be false")
	}
	var nilList *SkipList[int]
	if nilList.ContainsSlice([]int{1}) || !nilList.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice on a nil list: non-empty false, empty true")
	}
	if nilList.ContainsAny([]int{1}) {
		t.Errorf("ContainsAny on a nil list should be false")
	}
}

func TestCapabilitiesInsertBatches(t *testing.T) {
	list := NewSkipList[int]()
	if changed := list.InsertSlice([]int{1, 2, 3}); !changed {
		t.Errorf("InsertSlice of new elements should report true")
	}
	if changed := list.InsertSlice([]int{2, 3}); changed {
		t.Errorf("InsertSlice of only existing elements should report false")
	}
	if changed := list.InsertSlice([]int{4, 4}); !changed || list.Len() != 4 {
		t.Errorf("InsertSlice with duplicates: changed=%v len=%d, want true 4", changed, list.Len())
	}
	if changed := list.InsertSlice(nil); changed {
		t.Errorf("InsertSlice of nothing should report false")
	}

	// InsertIter feeds directly from another list's All (an iter.Seq here,
	// no adaptation needed).
	other := NewSkipList[int]()
	other.Insert(10)
	other.Insert(20)
	if changed := list.InsertIter(other.All()); !changed || !list.Contains(20) {
		t.Errorf("InsertIter from another list: changed=%v", changed)
	}
	if changed := list.InsertIter(other.All()); changed {
		t.Errorf("InsertIter of only existing elements should report false")
	}
	if changed := list.InsertIter(nil); changed {
		t.Errorf("InsertIter(nil) should report false")
	}

	// Self-feeding is safe: the iterator is drained before inserting.
	self := NewSkipList[int]()
	self.InsertSlice([]int{1, 2, 3})
	if changed := self.InsertIter(self.All()); changed || self.Len() != 3 {
		t.Errorf("self-feed InsertIter: changed=%v len=%d, want false 3", changed, self.Len())
	}

	// Ascending batch input is the skip list's home turf: no degradation.
	list2 := newTestList()
	keys := make([]TestSkipListNode, 0, 10)
	for i := 1; i <= 10; i++ {
		keys = append(keys, TestSkipListNode{S: fmt.Sprintf("%08d", i)})
	}
	if changed := list2.InsertSlice(keys); !changed || list2.Len() != 10 {
		t.Fatalf("ascending batch: changed=%v len=%d, want true 10", changed, list2.Len())
	}
	checkInvariant(t, list2, "after ascending batch insert")
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	list := NewSkipList[int]()
	list.InsertSlice([]int{1, 2, 3, 4, 5})
	if changed := list.DeleteSlice([]int{2, 9, 4}); !changed || list.Len() != 3 {
		t.Errorf("DeleteSlice: changed=%v len=%d, want true 3", changed, list.Len())
	}
	if changed := list.DeleteSlice([]int{9, 8}); changed {
		t.Errorf("DeleteSlice of nothing present should report false")
	}
	if changed := list.DeleteSlice(nil); changed {
		t.Errorf("DeleteSlice(nil) should report false")
	}

	// DeleteFunc removes exactly the matches and leaves the rest.
	list2 := NewSkipList[int]()
	for i := 0; i < 200; i++ {
		list2.Insert(i)
	}
	if changed := list2.DeleteFunc(func(v int) bool { return v%2 == 0 }); !changed {
		t.Errorf("DeleteFunc that matches half should report true")
	}
	if list2.Len() != 100 {
		t.Fatalf("after DeleteFunc len=%d, want 100", list2.Len())
	}
	for i := 0; i < 200; i++ {
		_, found := list2.Search(i)
		if i%2 == 0 && found {
			t.Errorf("even element %d should be deleted", i)
		}
		if i%2 == 1 && !found {
			t.Errorf("odd element %d should remain", i)
		}
	}
	checkCapLevel0(t, list2, "after DeleteFunc")

	if changed := list2.DeleteFunc(func(int) bool { return false }); changed {
		t.Errorf("DeleteFunc matching nothing should report false")
	}
	if changed := list2.DeleteFunc(nil); changed {
		t.Errorf("DeleteFunc(nil) should report false")
	}

	var nilList *SkipList[int]
	if nilList.DeleteSlice([]int{1}) || nilList.DeleteFunc(func(int) bool { return true }) {
		t.Errorf("deletes on a nil list should report false")
	}
}

// TestCapabilitiesDeleteFuncMassDelete exercises bulk deletion hard
// enough to expose span damage: fill, delete roughly half by predicate,
// then re-verify every membership, the chain invariants and the
// positional queries.
func TestCapabilitiesDeleteFuncMassDelete(t *testing.T) {
	list := NewSkipList[int]()
	model := map[int]bool{}
	for i := 0; i < 1000; i++ {
		list.Insert(i)
		model[i] = true
	}
	if !list.DeleteFunc(func(v int) bool { return v%3 == 0 || v%5 == 0 }) {
		t.Fatalf("DeleteFunc should report true")
	}
	for i := range model {
		want := i%3 != 0 && i%5 != 0
		if list.Contains(i) != want {
			t.Fatalf("after mass DeleteFunc, Contains(%d)=%v, want %v", i, list.Contains(i), want)
		}
	}
	if list.Len() != 533 { // 1000 - |div by 3 or 5| = 1000 - (333+200-66)
		t.Errorf("after mass DeleteFunc len=%d, want 533", list.Len())
	}
	checkCapLevel0(t, list, "after mass DeleteFunc")

	// The span machinery survived: Rank of a survivor is its sorted
	// position among the survivors (the model still holds every key).
	survivors := make([]int, 0, list.Len())
	for i := 0; i < 1000; i++ {
		if i%3 != 0 && i%5 != 0 {
			survivors = append(survivors, i)
		}
	}
	for probe := 0; probe < len(survivors); probe += 37 {
		if rank, found := list.Rank(survivors[probe]); !found || rank != probe {
			t.Errorf("Rank(%d) = %d,%v; want %d", survivors[probe], rank, found, probe)
		}
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilList *SkipList[string]
	var zero SkipList[string]
	for name, ls := range map[string]*SkipList[string]{"nil list": nilList, "zero value": &zero} {
		if ls.Contains("x") {
			t.Errorf("%s: Contains should be false", name)
		}
		if ls.String() != "[]" {
			t.Errorf("%s: String should be []", name)
		}
		if ls.Len() != 0 || !ls.IsEmpty() {
			t.Errorf("%s: should be empty", name)
		}
		if changed := ls.DeleteSlice([]string{"x"}); changed {
			t.Errorf("%s: DeleteSlice should report false", name)
		}
		if changed := ls.DeleteFunc(func(string) bool { return true }); changed {
			t.Errorf("%s: DeleteFunc should report false", name)
		}
		if !ls.Equal(nilList) {
			t.Errorf("%s: should Equal a nil list", name)
		}
		if s := ls.Slice(); s != nil {
			t.Errorf("%s: Slice should be nil (never-filled list)", name)
		}
	}

	// The insert contract fires only when an element would be stored.
	if changed := nilList.InsertSlice([]string{}); changed {
		t.Errorf("InsertSlice of nothing on a nil list should report false")
	}
	if changed := nilList.InsertIter(nil); changed {
		t.Errorf("InsertIter(nil) on a nil list should report false")
	}
	expectPanicMessage(t, "InsertSlice on nil", "InsertSlice called on a nil list", func() {
		nilList.InsertSlice([]string{"x"})
	})
	expectPanicMessage(t, "InsertIter on nil", "InsertIter called on a nil list", func() {
		nilList.InsertIter(func(yield func(string) bool) { yield("x") })
	})
	expectPanicMessage(t, "InsertSlice on zero value", "no comparison function", func() {
		zero.InsertSlice([]string{"x"})
	})
	expectPanicMessage(t, "InsertIter on zero value", "no comparison function", func() {
		zero.InsertIter(func(yield func(string) bool) { yield("x") })
	})
}

// TestCapabilitiesRandomizedModel cross-checks the batch and predicate
// operations against a map reference model with a fixed seed.
func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	list := NewSkipList[int]()
	model := map[int]bool{}

	key := func() int { return rng.IntN(200) }

	for step := 0; step < 800; step++ {
		switch rng.IntN(6) {
		case 0, 1:
			k := key()
			model[k] = true
			list.Insert(k)
		case 2:
			k := key()
			delete(model, k)
			list.Delete(k)
		case 3:
			// Batch insert of 1..3 keys.
			items := make([]int, rng.IntN(4))
			for i := range items {
				items[i] = key()
			}
			for _, k := range items {
				model[k] = true
			}
			list.InsertSlice(items)
		case 4:
			// Predicate delete on a digit class.
			d := rng.IntN(10)
			for k := range model {
				if k%10 == d {
					delete(model, k)
				}
			}
			list.DeleteFunc(func(v int) bool { return v%10 == d })
		case 5:
			// Batch delete of 1..3 keys.
			items := make([]int, rng.IntN(4))
			for i := range items {
				items[i] = key()
			}
			for _, k := range items {
				delete(model, k)
			}
			list.DeleteSlice(items)
		}

		if list.Len() != len(model) {
			t.Fatalf("step %d: len=%d, model=%d", step, list.Len(), len(model))
		}

		if step%100 == 99 {
			checkCapLevel0(t, list, "randomized")
			for k, want := range model {
				if list.Contains(k) != want {
					t.Fatalf("step %d: Contains(%d)=%v, want %v", step, k, list.Contains(k), want)
				}
			}
			snap := list.Slice()
			sortedModel := capabilitiesModelKeys(model)
			sort.Ints(sortedModel)
			if !g_lib.EqualSlice(snap, sortedModel) {
				t.Fatalf("step %d: Slice is not the sorted model (it must be ascending)", step)
			}
		}
	}

	// Final full verification, plus Equal and deterministic String
	// against a fresh list built from the model.
	want := capabilitiesModelKeys(model)
	sort.Ints(want)
	fresh := NewSkipList[int]()
	fresh.InsertSlice(want)
	if !list.Equal(fresh) || !fresh.Equal(list) {
		t.Errorf("final list must Equal a fresh list built from the model")
	}
	if list.String() != fresh.String() {
		t.Errorf("same elements must render the same String: %q vs %q", list.String(), fresh.String())
	}
	if !list.ContainsSlice(want) {
		t.Errorf("final list must contain every model key")
	}
	if list.ContainsSlice(append(want, 1000000)) {
		t.Errorf("ContainsSlice with a missing key must be false")
	}
	if !list.ContainsAny(append(want, 1000000)) {
		t.Errorf("ContainsAny with one present key must be true")
	}
}

// capabilitiesModelKeys returns the keys of m in unspecified order (test
// helper).
func capabilitiesModelKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestCapabilitiesConcurrent runs the new methods from several goroutines
// at once — `go test -race` is the real assertion here (no data race and
// no deadlock), the final-state check just verifies the writes all
// landed.
func TestCapabilitiesConcurrent(t *testing.T) {
	list := NewSkipList[int]()
	list.InsertSlice([]int{-1, -2}) // a little initial content

	const workers = 8
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				k := base + i*workers
				switch i % 8 {
				case 0, 1, 2:
					list.Insert(k)
				case 3:
					list.InsertSlice([]int{k, k})
				case 4:
					list.InsertIter(func(yield func(int) bool) {
						yield(k)
						yield(k + 1)
					})
				case 5:
					list.Delete(k)
					list.DeleteSlice([]int{k + 1})
				case 6:
					_ = list.Contains(k)
					_ = list.ContainsSlice([]int{k, k + 1})
					_ = list.ContainsAny([]int{k, k + 1})
				case 7:
					_ = list.Len()
					_ = list.IsEmpty()
					_ = list.String()
					_ = list.Slice()
					_ = list.Clone().Len()
					_ = list.DeleteFunc(func(v int) bool { return v == k+2 })
				}
			}
		}(w * 100000)
	}
	wg.Wait()

	if list.Len() == 0 {
		t.Errorf("expected residual content after the concurrent mix")
	}
	for v := range list.All() { // snapshot iterator: safe after the wait
		_ = v
	}
}

// TestCapabilitiesBatchAtomicity verifies that a batch insert holds the
// write lock for the whole batch: a concurrent reader observes either
// none or all of the batch, never a partial application.
func TestCapabilitiesBatchAtomicity(t *testing.T) {
	list := NewSkipList[int]()
	const batch = 5000
	items := make([]int, batch)
	for i := range items {
		items[i] = i
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		list.InsertSlice(items)
	}()

	observed := map[int]int{}
	for running := true; running; {
		n := list.Len() // read lock; blocks while the batch holds the write lock
		observed[n]++
		select {
		case <-done:
			running = false
		default:
		}
	}
	if observed[batch] == 0 {
		t.Errorf("the final size %d was never observed (observations: %v)", batch, observed)
	}
	for n := range observed {
		if n != 0 && n != batch {
			t.Errorf("observed partial batch state len=%d (observations: %v)", n, observed)
		}
	}
}

// TestCapabilitiesDeleteFuncCallback verifies that the DeleteFunc
// predicate runs without the lock held: it may call back into the list.
func TestCapabilitiesDeleteFuncCallback(t *testing.T) {
	list := NewSkipList[int]()
	for i := 0; i < 50; i++ {
		list.Insert(i)
	}
	changed := list.DeleteFunc(func(v int) bool {
		return v%2 == 0 && list.Contains(v+1000) == false && list.Len() >= 0 // call-backs under the predicate
	})
	if !changed || list.Len() != 25 {
		t.Errorf("DeleteFunc with a call-back predicate: changed=%v len=%d, want true 25", changed, list.Len())
	}
}

// TestCapabilitiesEqualNoDeadlock compares two lists from two goroutines
// in opposite directions at once — Equal must never hold both locks.
func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := NewSkipList[int]()
	b := NewSkipList[int]()
	for i := 0; i < 500; i++ {
		a.Insert(i)
		b.Insert(i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
}
