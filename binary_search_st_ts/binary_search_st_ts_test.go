/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st_ts

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

// TestPutGetDelete exercises the core map verbs through the twin:
// duplicate puts replace, deletes report found, and the table stays
// sorted underneath (Keys, AtIndex).
func TestPutGetDelete(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	if !st.IsEmpty() || st.Len() != 0 {
		t.Fatalf("new table is not empty")
	}
	st.Put("banana", 2)
	st.Put("apple", 1)
	st.Put("cherry", 3)
	st.Put("apple", 5) // duplicate replaces the value
	if st.Len() != 3 {
		t.Fatalf("Len = %d, want 3", st.Len())
	}
	if v, ok := st.Get("apple"); !ok || v != 5 {
		t.Fatalf("Get(apple) = %d, %v; want 5, true (duplicates replace)", v, ok)
	}
	if _, ok := st.Get("durian"); ok {
		t.Fatalf("Get of a missing key reported found")
	}
	if !st.Delete("banana") {
		t.Fatalf("Delete of a present key reported false")
	}
	if st.Delete("banana") {
		t.Fatalf("second Delete of the same key reported true")
	}
	if keys := st.Keys(); !slices.Equal(keys, []string{"apple", "cherry"}) {
		t.Fatalf("Keys = %v, want [apple cherry] (ascending underneath)", keys)
	}
	if k, _, ok := st.AtIndex(1); !ok || k != "cherry" {
		t.Fatalf("AtIndex(1) = %q, %v; want cherry, true", k, ok)
	}
}

// TestOrderedRiders checks the skip_list-named riders through the
// twin, on the table a e i o u with values 1 5 9 15 21.
func TestOrderedRiders(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	for _, p := range []struct {
		k string
		v int
	}{{"a", 1}, {"e", 5}, {"i", 9}, {"o", 15}, {"u", 21}} {
		st.Put(p.k, p.v)
	}
	if k, v, ok := st.Floor("h"); !ok || k != "e" || v != 5 {
		t.Fatalf("Floor(h) = %q, %d, %v; want e, 5, true", k, v, ok)
	}
	if _, _, ok := st.Floor("0"); ok {
		t.Fatalf("Floor before the first key reported found")
	}
	if k, _, ok := st.Ceil("h"); !ok || k != "i" {
		t.Fatalf("Ceil(h) = %q, %v; want i, true", k, ok)
	}
	if _, _, ok := st.Ceil("z"); ok {
		t.Fatalf("Ceil after the last key reported found")
	}
	if rank, ok := st.Rank("o"); !ok || rank != 3 {
		t.Fatalf("Rank(o) = %d, %v; want 3, true (present-key semantics)", rank, ok)
	}
	if rank, ok := st.Rank("h"); ok || rank != 0 {
		t.Fatalf("Rank(h) = %d, %v; want 0, false (a missing key ranks 0, the skip_list convention)", rank, ok)
	}
	if got := st.CountRange("c", "p"); got != 3 {
		t.Fatalf("CountRange(c, p) = %d, want 3 (e, i, o)", got)
	}
	if k, _, ok := st.FindMin(); !ok || k != "a" {
		t.Fatalf("FindMin = %q, %v", k, ok)
	}
	if k, _, ok := st.FindMax(); !ok || k != "u" {
		t.Fatalf("FindMax = %q, %v", k, ok)
	}
	if n := st.DeleteRange("c", "p"); n != 3 {
		t.Fatalf("DeleteRange(c, p) deleted %d, want 3", n)
	}
	if keys := st.Keys(); !slices.Equal(keys, []string{"a", "u"}) {
		t.Fatalf("Keys after DeleteRange = %v, want [a u]", keys)
	}
	if !st.DeleteAtHead() || !st.DeleteAtTail() || !st.IsEmpty() {
		t.Fatalf("head/tail deletes did not empty the table")
	}
}

// TestRandomizedModel runs puts, gets, deletes and the riders against
// a map reference model at the fixed seed.
func TestRandomizedModel(t *testing.T) {
	rng := newRand()
	st := NewBinarySearchST[int, int]()
	model := make(map[int]int)
	for step := 0; step < 1500; step++ {
		k := rng.Intn(200)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // put
			v := rng.Intn(10000)
			st.Put(k, v)
			model[k] = v
		case 4: // delete
			if st.Delete(k) != containsKey(model, k) {
				t.Fatalf("step %d: Delete(%d) disagrees with the model", step, k)
			}
			delete(model, k)
		case 5:
			if got, ok := st.Get(k); ok != containsKey(model, k) || (ok && got != model[k]) {
				t.Fatalf("step %d: Get(%d) = %d, %v; want %d, %v", step, k, got, ok, model[k], containsKey(model, k))
			}
		case 6:
			if st.Contains(k) != containsKey(model, k) {
				t.Fatalf("step %d: Contains(%d) disagrees with the model", step, k)
			}
		case 7: // Len vs model size
			if st.Len() != len(model) {
				t.Fatalf("step %d: Len = %d, model size = %d", step, st.Len(), len(model))
			}
		case 8: // Rank: position when present, 0/false when missing
			if containsKey(model, k) {
				want := slices.Index(sortedKeys(model), k)
				if rank, ok := st.Rank(k); !ok || rank != want {
					t.Fatalf("step %d: Rank(%d) = %d, %v; want %d, true", step, k, rank, ok, want)
				}
			} else if rank, ok := st.Rank(k); ok || rank != 0 {
				t.Fatalf("step %d: Rank of missing %d = %d, %v; want 0, false", step, k, rank, ok)
			}
		case 9: // AtIndex over the sorted model keys
			keys := sortedKeys(model)
			i := rng.Intn(len(keys) + 1)
			gotK, _, ok := st.AtIndex(i)
			if i < len(keys) {
				if !ok || gotK != keys[i] {
					t.Fatalf("step %d: AtIndex(%d) = %d, %v; want %d, true", step, i, gotK, ok, keys[i])
				}
			} else if ok {
				t.Fatalf("step %d: AtIndex(%d) reported found past the end", step, i)
			}
		}
	}
	// Final full agreement: Keys equals the sorted model keys.
	if keys := st.Keys(); !slices.Equal(keys, sortedKeys(model)) {
		t.Fatalf("final Keys disagree with the model: %d vs %d entries", len(keys), len(model))
	}
}

// containsKey is the map's own ok return.
func containsKey(m map[int]int, k int) bool {
	_, ok := m[k]
	return ok
}

func sortedKeys(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestIteratorsSnapshot pins the twins' iterator rule: All, Backward
// and Range walk a SNAPSHOT — mutating the table from inside the loop
// body is safe and does not disturb the iteration.
func TestIteratorsSnapshot(t *testing.T) {
	st := NewBinarySearchST[int, int]()
	for i := range 10 {
		st.Put(i, i*i)
	}

	var seen []int
	for k, v := range st.All() {
		seen = append(seen, k)
		if v != k*k {
			t.Fatalf("All yielded (%d, %d); want value %d", k, v, k*k)
		}
		st.Delete(k)     // safe: snapshot iteration
		st.Put(100+k, k) // safe: never observed by this iteration
	}
	if !slices.Equal(seen, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("All visited %v, want 0..9 ascending", seen)
	}
	if st.Len() != 10 {
		t.Fatalf("Len after in-loop delete+put = %d, want 10", st.Len())
	}

	seen = seen[:0]
	for k := range st.Backward() {
		seen = append(seen, k)
		st.Truncate() // safe: snapshot iteration
	}
	if !slices.Equal(seen, []int{109, 108, 107, 106, 105, 104, 103, 102, 101, 100}) {
		t.Fatalf("Backward visited %v", seen)
	}
	if !st.IsEmpty() {
		t.Fatalf("Backward's in-loop Truncate did not stick")
	}

	for i := range 5 {
		st.Put(i, i)
	}
	seen = seen[:0]
	for k := range st.Range(1, 3) {
		seen = append(seen, k)
	}
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("Range(1, 3) visited %v, want [1 2 3] (closed range)", seen)
	}
	seen = seen[:0]
	for k := range st.Range(4, 2) { // lo after hi: empty
		seen = append(seen, k)
	}
	if len(seen) != 0 {
		t.Fatalf("inverted Range visited %v", seen)
	}
	var nilSt *BinarySearchST[string, int]
	for range nilSt.All() { // nil iterates as empty
		t.Fatalf("nil table's All yielded an entry")
	}
}

// TestCapabilities covers Keys, String/StringFunc (callback unlocked),
// Clone (deep copy), PutIter (self-feed safe) and DeleteFunc
// (predicate unlocked, atomic rebuild).
func TestCapabilities(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	for _, p := range []struct {
		k string
		v int
	}{{"mango", 2}, {"apple", 0}, {"kiwi", 1}} {
		st.Put(p.k, p.v)
	}
	if got := st.String(); got != "[apple:0 kiwi:1 mango:2]" {
		t.Fatalf("String = %q", got)
	}
	// StringFunc's format function may call back into the table.
	got := st.StringFunc(func(k string, v int) string {
		_, ok := st.Get(k) // a read from inside the format callback
		if !ok {
			t.Fatalf("format callback lost key %q", k)
		}
		return fmt.Sprintf("%s=%d", k, v)
	})
	if got != "[apple=0 kiwi=1 mango=2]" {
		t.Fatalf("StringFunc = %q", got)
	}

	// Clone is deep: writes to either side never reach the other.
	c := st.Clone()
	c.Put("pear", 9)
	st.Put("plum", 8)
	if _, ok := st.Get("pear"); ok {
		t.Fatalf("clone's Put reached the original")
	}
	if _, ok := c.Get("plum"); ok {
		t.Fatalf("original's Put reached the clone")
	}

	// PutIter may be fed the table's own All (drained before the lock).
	c.PutIter(st.All())
	for k := range st.All() {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("PutIter(self All) lost key %q", k)
		}
	}
	if c.Len() != st.Len()+1 { // + pear
		t.Fatalf("Len after self-feed PutIter = %d, want %d", c.Len(), st.Len()+1)
	}

	// DeleteFunc's predicate may call back into the table; the rebuild
	// is atomic and the survivors keep their order.
	if !st.DeleteFunc(func(k string, v int) bool {
		_ = st.Len() // a read from inside the predicate
		return v%2 == 1
	}) {
		t.Fatalf("DeleteFunc reported nothing deleted")
	}
	if keys := st.Keys(); !slices.Equal(keys, []string{"apple", "mango", "plum"}) {
		t.Fatalf("Keys after DeleteFunc = %v (odd values 1 deleted, survivors ascending)", keys)
	}
	if st.DeleteFunc(func(string, int) bool { return false }) {
		t.Fatalf("DeleteFunc with no matches reported true")
	}
}

// TestJSON round-trips the pair array through the twin, checks the
// empty/null clear and that a decode error leaves the table untouched.
func TestJSON(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	st.Put("b", 2)
	st.Put("a", 1)
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `[{"k":"a","v":1},{"k":"b","v":2}]` {
		t.Fatalf("marshal = %s", data)
	}
	if err := st.UnmarshalJSON([]byte(`[{"k":"c","v":3},{"k":"a","v":9}]`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, _ := st.Get("a"); v != 9 {
		t.Fatalf("unmarshal did not replace (last pair wins): a = %d", v)
	}
	if v, _ := st.Get("c"); v != 3 {
		t.Fatalf("unmarshal lost c")
	}
	// A decode error leaves the table untouched.
	before := st.String()
	if err := st.UnmarshalJSON([]byte(`[{"k":`)); err == nil {
		t.Fatalf("malformed document did not error")
	}
	if st.String() != before {
		t.Fatalf("failed decode changed the table: %s", st.String())
	}
	// Empty array and null clear, everywhere.
	for _, doc := range []string{`[]`, `null`} {
		if err := st.UnmarshalJSON([]byte(doc)); err != nil {
			t.Fatalf("unmarshal %s: %v", doc, err)
		}
		if !st.IsEmpty() {
			t.Fatalf("unmarshal %s did not clear", doc)
		}
	}
	// A direct marshal on a nil table encodes as [].
	var nilSt *BinarySearchST[string, int]
	data, err = nilSt.MarshalJSON()
	if err != nil || string(data) != "[]" {
		t.Fatalf("nil marshal = %s, %v", data, err)
	}
}

// TestPanicContract pins the twin-named panic surface: a nil
// comparison function at construction, and storing on a nil table or
// a zero-value table.
func TestPanicContract(t *testing.T) {
	expectPanic(t, "binary_search_st_ts: NewBinarySearchSTFunc called with a nil comparison function", func() {
		_ = NewBinarySearchSTFunc[string, int](nil)
	})
	var nilSt *BinarySearchST[string, int]
	expectPanic(t, "binary_search_st_ts: Put called on a nil symbol table", func() {
		nilSt.Put("k", 1)
	})
	zero := &BinarySearchST[string, int]{}
	expectPanic(t, "binary_search_st_ts: Put called on a zero-value symbol table", func() {
		zero.Put("k", 1)
	})
	expectPanic(t, "binary_search_st_ts: PutIter called on a nil symbol table", func() {
		nilSt.PutIter(nil) // the contract fires even on an empty seq
	})
	expectPanic(t, "binary_search_st_ts: UnmarshalJSON called on a nil symbol table", func() {
		_ = nilSt.UnmarshalJSON([]byte(`[{"k":"a","v":1}]`))
	})
	expectPanic(t, "binary_search_st_ts: UnmarshalJSON called on a zero-value symbol table", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"k":"a","v":1}]`))
	})
	// Storing nothing is tolerated everywhere.
	_ = nilSt.UnmarshalJSON([]byte(`[]`))
	_ = zero.UnmarshalJSON([]byte(`null`))
}

// TestNilTolerated checks that every read on a nil table has the sane
// empty answer.
func TestNilTolerated(t *testing.T) {
	var st *BinarySearchST[string, int]
	if st.IsEmpty() != true || st.Len() != 0 || st.Length() != 0 {
		t.Fatalf("nil Len/IsEmpty not empty")
	}
	if _, ok := st.Get("a"); ok {
		t.Fatalf("nil Get found")
	}
	if st.Contains("a") {
		t.Fatalf("nil Contains true")
	}
	if _, _, ok := st.FindMin(); ok {
		t.Fatalf("nil FindMin found")
	}
	if _, _, ok := st.FindMax(); ok {
		t.Fatalf("nil FindMax found")
	}
	if _, _, ok := st.Floor("a"); ok {
		t.Fatalf("nil Floor found")
	}
	if _, _, ok := st.Ceil("a"); ok {
		t.Fatalf("nil Ceil found")
	}
	if _, ok := st.Rank("a"); ok {
		t.Fatalf("nil Rank found")
	}
	if _, _, ok := st.AtIndex(0); ok {
		t.Fatalf("nil AtIndex found")
	}
	if st.CountRange("a", "z") != 0 {
		t.Fatalf("nil CountRange nonzero")
	}
	if st.DeleteRange("a", "z") != 0 {
		t.Fatalf("nil DeleteRange nonzero")
	}
	if st.Delete("a") || st.DeleteAtHead() || st.DeleteAtTail() {
		t.Fatalf("nil deletes reported true")
	}
	if keys := st.Keys(); keys != nil {
		t.Fatalf("nil Keys = %v", keys)
	}
	if got := st.String(); got != "[]" {
		t.Fatalf("nil String = %q", got)
	}
	if c := st.Clone(); c != nil {
		t.Fatalf("nil Clone = %v", c)
	}
	if st.DeleteFunc(func(string, int) bool { return true }) {
		t.Fatalf("DeleteFunc on a nil table reported true (it reports false, no panic)")
	}
	st.Truncate() // no-op, no panic
	if st.NlIsEmpty() != true {
		t.Fatalf("NlIsEmpty on nil not true (the Nl forms read the empty inner)")
	}
	st.Lock() // no-op on nil
	st.Unlock()
}

// TestNlCompound is the canonical compound: a check-and-insert pair
// atomic under one hold of Lock.
func TestNlCompound(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	for i := range 100 {
		key := fmt.Sprintf("k%03d", i)
		st.Lock()
		if _, found := st.NlGet(key); !found {
			st.NlPut(key, i) // the check-and-insert pair is one atomic step
		}
		st.Unlock()
	}
	if st.NlLen() != 100 || st.Len() != 100 {
		t.Fatalf("NlLen = %d, Len = %d; want 100, 100", st.NlLen(), st.Len())
	}
	st.Lock()
	if !st.NlDelete("k042") || st.NlContains("k042") {
		t.Fatalf("NlDelete/NlContains disagree")
	}
	if st.NlIsEmpty() {
		t.Fatalf("NlIsEmpty true on a 99-entry table")
	}
	st.Unlock()
}

// TestConcurrent runs concurrent writers against a reader that
// iterates snapshots and queries, then drains the table with
// concurrent head/tail deleters.  It is primarily a test for the race
// detector (`make race`); the accounting must balance exactly.
func TestConcurrent(t *testing.T) {
	st := NewBinarySearchST[int, string]()
	const producers, per = 8, 250
	const total = producers * per

	var wg sync.WaitGroup
	for p := range producers {
		wg.Go(func() {
			for i := range per {
				k := p*per + i + 1
				st.Put(k, fmt.Sprint(k))
			}
		})
	}
	// A reader iterates snapshots and queries while the writers work.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for range st.All() {
			}
			_ = st.Len()
			_, _ = st.Get(42)
			_ = st.Contains(7)
			_, _, _ = st.Floor(1000)
			_, _ = st.MarshalJSON()
		}
	}()
	wg.Wait()

	if st.Len() != total {
		t.Fatalf("Len after concurrent puts = %d, want %d", st.Len(), total)
	}
	for k := 1; k <= total; k++ {
		if v, ok := st.Get(k); !ok || v != fmt.Sprint(k) {
			t.Fatalf("Get(%d) = %q, %v after the writers finished", k, v, ok)
		}
	}

	// Concurrent head/tail drain; each delete that reports true
	// accounts for exactly one entry.
	var drained atomic.Int64
	for range 4 {
		wg.Go(func() {
			for {
				if st.DeleteAtHead() {
					drained.Add(1)
					continue
				}
				if st.DeleteAtTail() {
					drained.Add(1)
					continue
				}
				if drained.Load() >= total {
					return
				}
			}
		})
	}
	wg.Wait()
	close(stop)

	if got := drained.Load(); got != total {
		t.Errorf("drained %d entries, want exactly %d", got, total)
	}
	if !st.IsEmpty() {
		t.Errorf("table not empty after the drain: Len = %d", st.Len())
	}
}
