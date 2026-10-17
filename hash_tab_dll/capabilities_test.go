package hash_tab_dll

// Tests for the set-convenience capabilities (capabilities.go):
// materialization, deterministic rendering, deep copy, membership
// batches, predicate bulk delete, and the nil/zero-value read contract.
// The same file (package name aside) runs against the hash_tab_ts twin —
// the twins must behave identically.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

func TestCapabilitiesContains(t *testing.T) {
	ht := NewHashTab[string](8)
	ht.Insert("alpha")
	if !ht.Contains("alpha") {
		t.Errorf("Contains should find a stored element")
	}
	if ht.Contains("beta") {
		t.Errorf("Contains should not find a missing element")
	}
	var nilTab *HashTab[string]
	if nilTab.Contains("alpha") {
		t.Errorf("Contains on a nil table should be false")
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	ht := newTestTab(7)
	ht.Insert(TestData{S: "a", N: 1})
	ht.Insert(TestData{S: "b", N: 2})
	ht.Insert(TestData{S: "a", N: 3}) // replace: still 2 elements

	got := ht.Slice()
	if len(got) != 2 {
		t.Fatalf("Slice length = %d, want 2", len(got))
	}
	keys := []string{}
	for _, v := range got {
		keys = append(keys, v.S)
	}
	sort.Strings(keys)
	if keys[0] != "a" || keys[1] != "b" {
		t.Errorf("Slice contents = %v, want [a b]", keys)
	}
	// Replacement keeps the last value.
	for _, v := range got {
		if v.S == "a" && v.N != 3 {
			t.Errorf("Slice should carry the replaced value, got N=%d, want 3", v.N)
		}
	}

	// The slice is a copy: mutating it must not affect the table.
	got[0] = TestData{S: "zzz"}
	if _, found := ht.Search(TestData{S: "zzz"}); found {
		t.Errorf("mutating the Slice result changed the table")
	}

	if empty := newTestTab(7).Slice(); len(empty) != 0 {
		t.Errorf("Slice of an empty table should be empty, got %d", len(empty))
	}
	var nilTab *HashTab[TestData]
	if s := nilTab.Slice(); s != nil {
		t.Errorf("Slice of a nil table should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed tables (each with its own random
	// maphash seed) holding the same elements must render identically —
	// that is the whole point of the sorted-format rule.
	a := NewHashTab[string](8)
	for _, s := range []string{"pear", "apple", "fig"} {
		a.Insert(s)
	}
	b := NewHashTab[string](5)
	for _, s := range []string{"fig", "pear", "apple"} { // different order, different bucket count
		b.Insert(s)
	}
	if a.String() != b.String() {
		t.Errorf("same elements must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[apple fig pear]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewHashTab[string](8).String(); got != "[]" {
		t.Errorf("String of an empty table = %q, want []", got)
	}
	var nilTab *HashTab[string]
	if got := nilTab.String(); got != "[]" {
		t.Errorf("String of a nil table = %q, want []", got)
	}

	// fmt.Sprint is the default element rendering; note the sort is over
	// the formatted strings, so "[11 3]"-style lexicographic order is
	// correct for numbers of different digit counts.
	c := NewHashTab[int](8)
	c.Insert(9)
	c.Insert(2)
	if got, want := c.String(), "[2 9]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	ht := NewHashTab[int](8)
	ht.Insert(5)
	ht.Insert(8)
	got := ht.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := ht.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesClone(t *testing.T) {
	ht := newTestTab(7)
	ht.Insert(TestData{S: "a", N: 1})
	ht.Insert(TestData{S: "b", N: 2})
	ht.Insert(TestData{S: "c", N: 3})

	dup := ht.Clone()
	if !dup.Equal(ht) || !ht.Equal(dup) {
		t.Errorf("clone must be equal to the receiver")
	}
	checkInvariants(t, dup)

	// Deep copy: mutate the clone, the receiver must not move.
	dup.Insert(TestData{S: "d", N: 4})
	dup.Delete(TestData{S: "a"})
	if ht.Len() != 3 || !ht.Contains(TestData{S: "a"}) || !ht.Contains(TestData{S: "b"}) || ht.Contains(TestData{S: "d"}) {
		t.Errorf("mutating the clone changed the receiver: len=%d", ht.Len())
	}

	// And the other direction.
	ht.Insert(TestData{S: "e", N: 5})
	if dup.Len() != 3 || dup.Contains(TestData{S: "e"}) {
		t.Errorf("mutating the receiver changed the clone: len=%d", dup.Len())
	}

	// The eq/hash functions carried over: the clone still matches by S only.
	dup.Insert(TestData{S: "b", N: 99})
	if el, found := dup.Search(TestData{S: "b"}); !found || el.GetData().N != 99 {
		t.Errorf("clone should keep the equality function (replace by S), got N=%d found=%v", el.GetData().N, found)
	}

	var nilTab *HashTab[TestData]
	if c := nilTab.Clone(); c != nil {
		t.Errorf("Clone of a nil table should be nil, got %v", c)
	}
	var zero HashTab[TestData]
	if c := zero.Clone(); c.Len() != 0 {
		t.Errorf("Clone of a zero-value table should be empty")
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// Same elements inserted in different orders, same functions: equal.
	a := newTestTab(7)
	for _, s := range []string{"a", "b", "c", "d"} {
		a.Insert(TestData{S: s})
	}
	b := newTestTab(7)
	for _, s := range []string{"d", "c", "b", "a"} {
		b.Insert(TestData{S: s})
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("tables with the same elements must be Equal (both directions)")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Different lengths.
	c := newTestTab(7)
	c.Insert(TestData{S: "a"})
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("different lengths must not be Equal")
	}

	// Same length, different elements.
	d := newTestTab(7)
	for _, s := range []string{"a", "b", "c", "x"} {
		d.Insert(TestData{S: s})
	}
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("same length but different elements must not be Equal")
	}

	// Empty tables, nil tables.
	empty := newTestTab(7)
	if !empty.Equal(newTestTab(7)) {
		t.Errorf("two empty tables must be Equal")
	}
	var nilTab *HashTab[TestData]
	if !empty.Equal(nilTab) || !nilTab.Equal(empty) {
		t.Errorf("an empty table and a nil table must be Equal")
	}
	if !nilTab.Equal(nilTab) {
		t.Errorf("nil must Equal nil")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	ht := NewHashTab[int](8)
	for _, v := range []int{1, 2, 3} {
		ht.Insert(v)
	}
	if !ht.ContainsSlice([]int{1, 2, 3}) {
		t.Errorf("ContainsSlice should be true when all present")
	}
	if !ht.ContainsSlice([]int{3, 1}) {
		t.Errorf("ContainsSlice should be true for a subset")
	}
	if ht.ContainsSlice([]int{1, 9}) {
		t.Errorf("ContainsSlice should be false when one is missing")
	}
	if !ht.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice of an empty slice is vacuously true")
	}
	if !ht.ContainsAny([]int{9, 2}) {
		t.Errorf("ContainsAny should be true when one is present")
	}
	if ht.ContainsAny([]int{9, 8}) {
		t.Errorf("ContainsAny should be false when none present")
	}
	if ht.ContainsAny([]int{}) {
		t.Errorf("ContainsAny of an empty slice should be false")
	}
	var nilTab *HashTab[int]
	if nilTab.ContainsSlice([]int{1}) || !nilTab.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice on a nil table: non-empty false, empty true")
	}
	if nilTab.ContainsAny([]int{1}) {
		t.Errorf("ContainsAny on a nil table should be false")
	}
}

func TestCapabilitiesInsertBatches(t *testing.T) {
	ht := NewHashTab[int](8)
	if changed := ht.InsertSlice([]int{1, 2, 3}); !changed {
		t.Errorf("InsertSlice of new elements should report true")
	}
	if changed := ht.InsertSlice([]int{2, 3}); changed {
		t.Errorf("InsertSlice of only existing elements should report false")
	}
	if changed := ht.InsertSlice([]int{4, 4}); !changed || ht.Len() != 4 {
		t.Errorf("InsertSlice with duplicates: changed=%v len=%d, want true 4", changed, ht.Len())
	}
	if changed := ht.InsertSlice(nil); changed {
		t.Errorf("InsertSlice of nothing should report false")
	}

	// InsertIter fed from another table's Values — the interface-free
	// batch bridge.
	other := NewHashTab[int](8)
	other.Insert(10)
	other.Insert(20)
	if changed := ht.InsertIter(other.Values()); !changed || !ht.Contains(20) {
		t.Errorf("InsertIter from another table: changed=%v", changed)
	}
	if changed := ht.InsertIter(other.Values()); changed {
		t.Errorf("InsertIter of only existing elements should report false")
	}
	if changed := ht.InsertIter(nil); changed {
		t.Errorf("InsertIter(nil) should report false")
	}

	// Self-feeding is safe: the iterator is drained before inserting.
	self := NewHashTab[int](8)
	self.InsertSlice([]int{1, 2, 3})
	if changed := self.InsertIter(self.Values()); changed || self.Len() != 3 {
		t.Errorf("self-feed InsertIter: changed=%v len=%d, want false 3", changed, self.Len())
	}
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	ht := NewHashTab[int](8)
	ht.InsertSlice([]int{1, 2, 3, 4, 5})
	if changed := ht.DeleteSlice([]int{2, 9, 4}); !changed || ht.Len() != 3 {
		t.Errorf("DeleteSlice: changed=%v len=%d, want true 3", changed, ht.Len())
	}
	if changed := ht.DeleteSlice([]int{9, 8}); changed {
		t.Errorf("DeleteSlice of nothing present should report false")
	}
	if changed := ht.DeleteSlice(nil); changed {
		t.Errorf("DeleteSlice(nil) should report false")
	}

	// DeleteFunc removes exactly the matches and leaves the rest.
	ht2 := newTestTab(53)
	for i := 0; i < 200; i++ {
		ht2.Insert(TestData{S: fmt.Sprintf("k%03d", i), N: i})
	}
	if changed := ht2.DeleteFunc(func(v TestData) bool { return v.N%2 == 0 }); !changed {
		t.Errorf("DeleteFunc that matches half should report true")
	}
	if ht2.Len() != 100 {
		t.Fatalf("after DeleteFunc len=%d, want 100", ht2.Len())
	}
	for i := 0; i < 200; i++ {
		_, found := ht2.Search(TestData{S: fmt.Sprintf("k%03d", i)})
		if i%2 == 0 && found {
			t.Errorf("even element %d should be deleted", i)
		}
		if i%2 == 1 && !found {
			t.Errorf("odd element %d should remain", i)
		}
	}
	checkInvariants(t, ht2)

	if changed := ht2.DeleteFunc(func(v TestData) bool { return false }); changed {
		t.Errorf("DeleteFunc matching nothing should report false")
	}
	if changed := ht2.DeleteFunc(nil); changed {
		t.Errorf("DeleteFunc(nil) should report false")
	}

	var nilTab *HashTab[int]
	if nilTab.DeleteSlice([]int{1}) || nilTab.DeleteFunc(func(int) bool { return true }) {
		t.Errorf("deletes on a nil table should report false")
	}
}

// TestCapabilitiesDeleteFuncMassDelete exercises bulk deletion hard
// enough to expose probe-chain or chain-relinking damage: fill, delete
// roughly half by predicate, then re-verify every membership.
func TestCapabilitiesDeleteFuncMassDelete(t *testing.T) {
	ht := NewHashTab[int](127)
	model := map[int]bool{}
	for i := 0; i < 1000; i++ {
		ht.Insert(i)
		model[i] = true
	}
	if !ht.DeleteFunc(func(v int) bool { return v%3 == 0 || v%5 == 0 }) {
		t.Fatalf("DeleteFunc should report true")
	}
	for i := range model {
		want := i%3 != 0 && i%5 != 0
		if ht.Contains(i) != want {
			t.Fatalf("after mass DeleteFunc, Contains(%d)=%v, want %v", i, ht.Contains(i), want)
		}
	}
	if ht.Len() != 533 { // 1000 - |div by 3 or 5| = 1000 - (333+200-66)
		t.Errorf("after mass DeleteFunc len=%d, want 533", ht.Len())
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilTab *HashTab[string]
	var zero HashTab[string]
	for name, tab := range map[string]*HashTab[string]{"nil table": nilTab, "zero value": &zero} {
		if tab.Contains("x") {
			t.Errorf("%s: Contains should be false", name)
		}
		if tab.String() != "[]" {
			t.Errorf("%s: String should be []", name)
		}
		if tab.Len() != 0 || !tab.IsEmpty() {
			t.Errorf("%s: should be empty", name)
		}
		if changed := tab.DeleteSlice([]string{"x"}); changed {
			t.Errorf("%s: DeleteSlice should report false", name)
		}
		if changed := tab.DeleteFunc(func(string) bool { return true }); changed {
			t.Errorf("%s: DeleteFunc should report false", name)
		}
		if !tab.Equal(nilTab) {
			t.Errorf("%s: should Equal a nil table", name)
		}
		if s := tab.Slice(); name == "nil table" && s != nil {
			t.Errorf("nil table: Slice should be nil")
		} else if name == "zero value" && len(s) != 0 {
			t.Errorf("zero value: Slice should be empty")
		}
	}

	// The insert contract fires only when an element would be stored.
	if changed := nilTab.InsertSlice([]string{}); changed {
		t.Errorf("InsertSlice of nothing on a nil table should report false")
	}
	if changed := nilTab.InsertIter(nil); changed {
		t.Errorf("InsertIter(nil) on a nil table should report false")
	}
	expectPanicMessage(t, "InsertSlice on nil", "InsertSlice called on a nil table", func() {
		nilTab.InsertSlice([]string{"x"})
	})
	expectPanicMessage(t, "InsertIter on nil", "InsertIter called on a nil table", func() {
		nilTab.InsertIter(func(yield func(string) bool) { yield("x") })
	})
	expectPanicMessage(t, "InsertSlice on zero value", "no equality/hash functions", func() {
		zero.InsertSlice([]string{"x"})
	})
	expectPanicMessage(t, "InsertIter on zero value", "no equality/hash functions", func() {
		zero.InsertIter(func(yield func(string) bool) { yield("x") })
	})
}

// TestCapabilitiesRandomizedModel cross-checks the batch and predicate
// operations against a map reference model with a fixed seed.
func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ht := NewHashTab[int](53)
	model := map[int]bool{}

	key := func() int { return rng.Intn(200) }

	for step := 0; step < 800; step++ {
		switch rng.Intn(6) {
		case 0, 1:
			k := key()
			model[k] = true
			ht.Insert(k)
		case 2:
			k := key()
			delete(model, k)
			ht.Delete(k)
		case 3:
			// Batch insert of 1..3 keys.
			items := make([]int, rng.Intn(4))
			for i := range items {
				items[i] = key()
			}
			for _, k := range items {
				model[k] = true
			}
			ht.InsertSlice(items)
		case 4:
			// Predicate delete on a digit class.
			d := rng.Intn(10)
			for k := range model {
				if k%10 == d {
					delete(model, k)
				}
			}
			ht.DeleteFunc(func(v int) bool { return v%10 == d })
		case 5:
			// Batch delete of 1..3 keys.
			items := make([]int, rng.Intn(4))
			for i := range items {
				items[i] = key()
			}
			for _, k := range items {
				delete(model, k)
			}
			ht.DeleteSlice(items)
		}

		if ht.Len() != len(model) {
			t.Fatalf("step %d: len=%d, model=%d", step, ht.Len(), len(model))
		}

		if step%100 == 99 {
			for k, want := range model {
				if ht.Contains(k) != want {
					t.Fatalf("step %d: Contains(%d)=%v, want %v", step, k, ht.Contains(k), want)
				}
			}
			snap := ht.Slice()
			sortedSnap := append([]int(nil), snap...)
			sort.Ints(sortedSnap)
			sortedModel := modelKeys(model)
			sort.Ints(sortedModel)
			if !g_lib.EqualSlice(sortedSnap, sortedModel) {
				t.Fatalf("step %d: Slice contents disagree with the model", step)
			}
		}
	}

	// Final full verification, plus Equal and deterministic String
	// against a fresh table built from the model.
	for k := range model {
		if !ht.Contains(k) {
			t.Fatalf("final: missing %d", k)
		}
	}
	want := make([]int, 0, len(model))
	for k := range model {
		want = append(want, k)
	}
	fresh := NewHashTab[int](53)
	fresh.InsertSlice(want)
	if !ht.Equal(fresh) || !fresh.Equal(ht) {
		t.Errorf("final table must Equal a fresh table built from the model")
	}
	if ht.String() != fresh.String() {
		t.Errorf("same elements must render the same String: %q vs %q", ht.String(), fresh.String())
	}
	if !ht.ContainsSlice(want) {
		t.Errorf("final table must contain every model key")
	}
	missing := append(want, 1000000)
	if ht.ContainsSlice(missing) {
		t.Errorf("ContainsSlice with a missing key must be false")
	}
	if !ht.ContainsAny(missing) {
		t.Errorf("ContainsAny with one present key must be true")
	}
}

// modelKeys returns the keys of m in unspecified order (test helper).
func modelKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
