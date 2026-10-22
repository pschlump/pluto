/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// stKey is a struct key ordered by S only, the TestTreeNode shape: S is
// the key the comparison function orders by, N is satellite data the
// comparison function ignores.
type stKey struct {
	S string
	N int
}

func keyCmp(a, b stKey) int { return strings.Compare(a.S, b.S) }

func TestPutGetDelete(t *testing.T) {
	st := NewBinarySearchST[string, int]()

	if got := st.Len(); got != 0 {
		t.Errorf("Len on a fresh table = %d, want 0", got)
	}
	if !st.IsEmpty() {
		t.Error("IsEmpty on a fresh table = false")
	}
	if _, found := st.Get("missing"); found {
		t.Error("Get on a fresh table found a key")
	}

	for i, k := range []string{"m", "a", "z", "c"} {
		st.Put(k, i+1)
	}
	if got := st.Len(); got != 4 {
		t.Errorf("Len after 4 puts = %d, want 4", got)
	}
	if got := st.Keys(); !slices.Equal(got, []string{"a", "c", "m", "z"}) {
		t.Errorf("Keys = %v, want [a c m z] (sorted regardless of put order)", got)
	}

	// A duplicate key replaces the value and does not grow the table.
	st.Put("m", 99)
	if got := st.Len(); got != 4 {
		t.Errorf("Len after a replacing put = %d, want 4", got)
	}
	if v, found := st.Get("m"); !found || v != 99 {
		t.Errorf("Get after replace = (%d, %v), want (99, true)", v, found)
	}

	if !st.Contains("a") || st.Contains("b") {
		t.Error("Contains disagrees with Get")
	}
	if st.Delete("b") {
		t.Error("Delete of a missing key returned true")
	}
	if !st.Delete("m") {
		t.Error("Delete of a present key returned false")
	}
	if got := st.Keys(); !slices.Equal(got, []string{"a", "c", "z"}) {
		t.Errorf("Keys after delete = %v, want [a c z]", got)
	}
	if _, found := st.Get("m"); found {
		t.Error("Get found a deleted key")
	}
}

func TestOrderedRiders(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	for i, k := range []string{"a", "c", "e", "g", "i"} {
		st.Put(k, i) // values track the sorted position
	}

	floorWant := []struct {
		key, want string
		found     bool
	}{
		{"a", "a", true}, {"b", "a", true}, {"c", "c", true},
		{"d", "c", true}, {"i", "i", true}, {"j", "i", true},
		{"", "", false}, // before every key
	}
	for _, tc := range floorWant {
		k, _, found := st.Floor(tc.key)
		if found != tc.found || (found && k != tc.want) {
			t.Errorf("Floor(%q) = (%q, %v), want (%q, %v)", tc.key, k, found, tc.want, tc.found)
		}
	}

	ceilWant := []struct {
		key, want string
		found     bool
	}{
		{"a", "a", true}, {"b", "c", true}, {"i", "i", true},
		{"j", "j", false}, {"", "a", true},
	}
	for _, tc := range ceilWant {
		k, _, found := st.Ceil(tc.key)
		if found != tc.found || (found && k != tc.want) {
			t.Errorf("Ceil(%q) = (%q, %v), want (%q, %v)", tc.key, k, found, tc.want, tc.found)
		}
	}

	if rank, found := st.Rank("e"); !found || rank != 2 {
		t.Errorf("Rank(e) = (%d, %v), want (2, true)", rank, found)
	}
	if rank, found := st.Rank("d"); found || rank != 0 {
		t.Errorf("Rank(d) = (%d, %v), want (0, false) — rank of a missing key is not defined", rank, found)
	}
	for i, k := range []string{"a", "c", "e", "g", "i"} {
		key, val, found := st.AtIndex(i)
		if !found || key != k || val != i {
			t.Errorf("AtIndex(%d) = (%q, %d, %v), want (%q, %d, true) — AtIndex inverts Rank", i, key, val, found, k, i)
		}
	}
	if _, _, found := st.AtIndex(-1); found {
		t.Error("AtIndex(-1) found")
	}
	if _, _, found := st.AtIndex(5); found {
		t.Error("AtIndex(5) found on a 5-entry table")
	}

	if got := st.CountRange("b", "g"); got != 3 {
		t.Errorf("CountRange(b, g) = %d, want 3 ([c e g])", got)
	}
	if got := st.CountRange("g", "b"); got != 0 {
		t.Errorf("CountRange(g, b) = %d, want 0 (lo after hi)", got)
	}
	if got := st.CountRange("x", "z"); got != 0 {
		t.Errorf("CountRange(x, z) = %d, want 0", got)
	}

	if k, v, found := st.FindMin(); !found || k != "a" || v != 0 {
		t.Errorf("FindMin = (%q, %d, %v), want (a, 0, true)", k, v, found)
	}
	if k, v, found := st.FindMax(); !found || k != "i" || v != 4 {
		t.Errorf("FindMax = (%q, %d, %v), want (i, 4, true)", k, v, found)
	}

	if got := st.DeleteRange("c", "g"); got != 3 {
		t.Errorf("DeleteRange(c, g) deleted %d, want 3", got)
	}
	if got := st.Keys(); !slices.Equal(got, []string{"a", "i"}) {
		t.Errorf("Keys after DeleteRange = %v, want [a i]", got)
	}
	if got := st.DeleteRange("x", "z"); got != 0 {
		t.Errorf("DeleteRange(x, z) deleted %d, want 0", got)
	}

	if !st.DeleteAtHead() || !st.DeleteAtTail() {
		t.Error("DeleteAtHead/DeleteAtTail on a 2-entry table returned false")
	}
	if !st.IsEmpty() {
		t.Error("table not empty after draining head and tail")
	}
	if st.DeleteAtHead() {
		t.Error("DeleteAtHead on an empty table returned true")
	}
	if _, _, found := st.FindMin(); found {
		t.Error("FindMin on an empty table found")
	}
}

func TestIteration(t *testing.T) {
	st := NewBinarySearchST[int, string]()
	for i := range 10 {
		st.Put(i, fmt.Sprintf("v%d", i))
	}

	var fwd []string
	for _, v := range st.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, []string{"v0", "v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9"}) {
		t.Errorf("All yielded %v, want v0..v9 in order", fwd)
	}

	var back []string
	for _, v := range st.Backward() {
		back = append(back, v)
	}
	slices.Reverse(fwd)
	if !slices.Equal(back, fwd) {
		t.Errorf("Backward yielded %v, want the reverse of All", back)
	}

	var in []int
	for k, v := range st.Range(3, 6) {
		if v != fmt.Sprintf("v%d", k) {
			t.Errorf("Range pair mismatch: %d -> %q", k, v)
		}
		in = append(in, k)
	}
	if !slices.Equal(in, []int{3, 4, 5, 6}) {
		t.Errorf("Range(3, 6) yielded %v, want [3 4 5 6] (closed bounds)", in)
	}
	for k := range st.Range(6, 3) { // lo after hi yields nothing
		t.Errorf("Range(6, 3) yielded %d", k)
	}

	// Early break stops the walk.
	count := 0
	for range st.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("break out of All visited %d entries, want 1", count)
	}
}

// TestRandomizedModel drives Put/Delete against a map reference model
// with a fixed seed, then cross-checks the ordered riders against a
// slices.BinarySearch oracle on the model's sorted keys.
func TestRandomizedModel(t *testing.T) {
	rng := newRand()
	st := NewBinarySearchST[string, int]()
	model := make(map[string]int)

	randKey := func() string {
		return string(rune('a'+rng.Intn(26))) + string(rune('a'+rng.Intn(26)))
	}

	for step := range 800 {
		k := randKey()
		switch rng.Intn(3) {
		case 0, 1: // puts twice as often as deletes
			v := rng.Intn(1000)
			st.Put(k, v)
			model[k] = v
		case 2:
			stDel := st.Delete(k)
			_, inModel := model[k]
			if stDel != inModel {
				t.Fatalf("step %d: Delete(%q) = %v, model says present=%v", step, k, stDel, inModel)
			}
			delete(model, k)
		}

		if step%40 == 0 {
			if got := st.Len(); got != len(model) {
				t.Fatalf("step %d: Len = %d, model has %d", step, got, len(model))
			}
			keys := make([]string, 0, len(model))
			for k := range model {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if got := st.Keys(); !slices.Equal(got, keys) {
				t.Fatalf("step %d: Keys = %v, model sorted keys = %v", step, got, keys)
			}
			i := 0
			for k, v := range st.All() {
				if k != keys[i] {
					t.Fatalf("step %d: All position %d = %q, want %q", step, i, k, keys[i])
				}
				if v != model[k] {
					t.Fatalf("step %d: value of %q = %d, model %d", step, k, v, model[k])
				}
				i++
			}

			probe := randKey()
			for range 8 {
				fk, _, ffound := st.Floor(probe)
				idx, ok := slices.BinarySearch(keys, probe)
				switch {
				case ok:
					if !ffound || fk != keys[idx] {
						t.Fatalf("step %d: Floor(%q) = (%q, %v), want (%q, true)", step, probe, fk, ffound, keys[idx])
					}
				case idx > 0:
					if !ffound || fk != keys[idx-1] {
						t.Fatalf("step %d: Floor(%q) = (%q, %v), want (%q, true)", step, probe, fk, ffound, keys[idx-1])
					}
				default:
					if ffound {
						t.Fatalf("step %d: Floor(%q) found %q, want not-found", step, probe, fk)
					}
				}

				ck, _, cfound := st.Ceil(probe)
				if idx < len(keys) {
					want := keys[idx]
					if !cfound || ck != want {
						t.Fatalf("step %d: Ceil(%q) = (%q, %v), want (%q, true)", step, probe, ck, cfound, want)
					}
				} else if cfound {
					t.Fatalf("step %d: Ceil(%q) found %q, want not-found", step, probe, ck)
				}

				if idx < len(keys) {
					rank, rf := st.Rank(keys[idx])
					if !rf || rank != idx {
						t.Fatalf("step %d: Rank(%q) = (%d, %v), want (%d, true)", step, keys[idx], rank, rf, idx)
					}
					ak, av, af := st.AtIndex(idx)
					if !af || ak != keys[idx] || av != model[keys[idx]] {
						t.Fatalf("step %d: AtIndex(%d) = (%q, %d, %v), want (%q, %d, true)", step, idx, ak, av, af, keys[idx], model[keys[idx]])
					}
				}
				probe = randKey()
			}
		}
	}
}

// TestNonComparableKey proves no interface and no == is ever applied to
// keys: []string is not comparable at all, yet it orders fine through a
// caller supplied comparison function.
func TestNonComparableKey(t *testing.T) {
	st := NewBinarySearchSTFunc[[]string, int](func(a, b []string) int { return slices.Compare(a, b) })
	st.Put([]string{"b", "x"}, 1)
	st.Put([]string{"a"}, 2)
	st.Put([]string{"b", "a"}, 3)
	if got := st.Len(); got != 3 {
		t.Fatalf("Len = %d, want 3", got)
	}
	v, found := st.Get([]string{"b", "x"})
	if !found || v != 1 {
		t.Errorf("Get([b x]) = (%d, %v), want (1, true)", v, found)
	}
	if k, _, found := st.Floor([]string{"b", "m"}); !found || slices.Compare(k, []string{"b", "a"}) != 0 {
		t.Errorf("Floor([b m]) wrong: (%v, %v)", k, found)
	}
	if !st.Delete([]string{"a"}) {
		t.Error("Delete([a]) returned false")
	}
}

func TestStructKeyFunc(t *testing.T) {
	st := NewBinarySearchSTFunc[stKey, int](keyCmp)

	// N is satellite data the comparison function ignores: two keys with
	// the same S are the same key, and the replacement stores the new
	// satellite data.
	st.Put(stKey{S: "k", N: 1}, 10)
	st.Put(stKey{S: "k", N: 2}, 20)
	if got := st.Len(); got != 1 {
		t.Errorf("Len after two same-S puts = %d, want 1", got)
	}
	v, found := st.Get(stKey{S: "k"})
	if !found || v != 20 {
		t.Errorf("Get(k) = (%d, %v), want (20, true) — the replacement wins", v, found)
	}
	st.Put(stKey{S: "a", N: 5}, 1)
	st.Put(stKey{S: "z", N: 6}, 2)
	if got := st.Len(); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}
	if k, _, found := st.FindMax(); !found || k.S != "z" || k.N != 6 {
		t.Errorf("FindMax = (%v, %v), want ({z 6}, true)", k, found)
	}
}

func TestNilAndZero(t *testing.T) {
	var nilST *BinarySearchST[string, int]

	// Reads tolerate nil exactly as an empty table.
	if nilST.Len() != 0 || !nilST.IsEmpty() {
		t.Error("nil table is not empty")
	}
	if _, found := nilST.Get("a"); found {
		t.Error("Get on nil found")
	}
	if nilST.Contains("a") {
		t.Error("Contains on nil true")
	}
	if nilST.Delete("a") {
		t.Error("Delete on nil returned true")
	}
	if nilST.DeleteAtHead() || nilST.DeleteAtTail() {
		t.Error("DeleteAtHead/DeleteAtTail on nil returned true")
	}
	if _, _, found := nilST.FindMin(); found {
		t.Error("FindMin on nil found")
	}
	if _, _, found := nilST.Floor("a"); found {
		t.Error("Floor on nil found")
	}
	if _, _, found := nilST.Ceil("a"); found {
		t.Error("Ceil on nil found")
	}
	if rank, found := nilST.Rank("a"); found || rank != 0 {
		t.Errorf("Rank on nil = (%d, %v), want (0, false)", rank, found)
	}
	if _, _, found := nilST.AtIndex(0); found {
		t.Error("AtIndex on nil found")
	}
	if got := nilST.CountRange("a", "z"); got != 0 {
		t.Errorf("CountRange on nil = %d, want 0", got)
	}
	if got := nilST.DeleteRange("a", "z"); got != 0 {
		t.Errorf("DeleteRange on nil = %d, want 0", got)
	}
	if got := nilST.DeleteFunc(func(string, int) bool { return true }); got {
		t.Error("DeleteFunc on nil returned true")
	}
	if got := nilST.Keys(); got != nil {
		t.Errorf("Keys on nil = %v, want nil", got)
	}
	for range nilST.All() {
		t.Error("All on nil yielded an entry")
	}
	for range nilST.Backward() {
		t.Error("Backward on nil yielded an entry")
	}
	for range nilST.Range("a", "z") {
		t.Error("Range on nil yielded an entry")
	}
	nilST.Truncate() // no-op, must not panic
	if nilST.Clone() != nil {
		t.Error("Clone on nil is not nil")
	}
	blob, err := nilST.MarshalJSON()
	if err != nil || string(blob) != "[]" {
		t.Errorf("MarshalJSON on nil = (%q, %v), want ([], nil)", blob, err)
	}

	// The zero value behaves the same way for reads.
	var zero BinarySearchST[string, int]
	if zero.Len() != 0 || !zero.IsEmpty() || zero.Contains("a") {
		t.Error("zero-value table does not read as empty")
	}

	// The exact panic surface.
	expectPanic(t, "binary_search_st: NewBinarySearchSTFunc called with a nil comparison function", func() {
		NewBinarySearchSTFunc[string, int](nil)
	})
	expectPanic(t, "binary_search_st: Put called on a nil symbol table", func() {
		nilST.Put("a", 1)
	})
	expectPanic(t, "binary_search_st: Put called on a zero-value symbol table with no comparison function", func() {
		zero.Put("a", 1)
	})
	expectPanic(t, "binary_search_st: PutIter called on a nil symbol table", func() {
		nilST.PutIter(nil)
	})
	expectPanic(t, "binary_search_st: UnmarshalJSON called on a nil symbol table", func() {
		_ = nilST.UnmarshalJSON([]byte(`[{"k":"a","v":1}]`))
	})
	expectPanic(t, "binary_search_st: UnmarshalJSON called on a zero-value symbol table with no comparison function", func() {
		_ = zero.UnmarshalJSON([]byte(`[{"k":"a","v":1}]`))
	})
	// An empty document stores nothing — tolerated everywhere.
	if err := nilST.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("UnmarshalJSON([]) on nil = %v, want nil", err)
	}
	if err := zero.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Errorf("UnmarshalJSON(null) on zero value = %v, want nil", err)
	}
}

func TestTruncate(t *testing.T) {
	st := NewBinarySearchST[int, string]()
	for i := range 5 {
		st.Put(i, "x")
	}
	st.Truncate()
	if !st.IsEmpty() {
		t.Fatal("table not empty after Truncate")
	}
	// The comparison function is kept, so the table refills.
	st.Put(3, "three")
	if v, found := st.Get(3); !found || v != "three" {
		t.Errorf("Get after Truncate+Put = (%q, %v)", v, found)
	}
	var nilST *BinarySearchST[int, string]
	nilST.Truncate() // no-op, must not panic
}

func TestCapabilities(t *testing.T) {
	build := func() *BinarySearchST[string, int] {
		st := NewBinarySearchST[string, int]()
		st.Put("b", 2)
		st.Put("a", 1)
		st.Put("c", 3)
		return st
	}
	st := build()

	if got := st.String(); got != "[a:1 b:2 c:3]" {
		t.Errorf("String = %q, want [a:1 b:2 c:3]", got)
	}
	got := st.StringFunc(func(k string, v int) string { return fmt.Sprintf("%s=%d", k, v) })
	if got != "[a=1 b=2 c=3]" {
		t.Errorf("StringFunc = %q", got)
	}
	if empty := NewBinarySearchST[int, int]().String(); empty != "[]" {
		t.Errorf("String on empty = %q, want []", empty)
	}

	// Clone: no backing array shared.
	clone := st.Clone()
	clone.Put("d", 4)
	clone.Delete("a")
	if st.Len() != 3 || !st.Contains("a") || st.Contains("d") {
		t.Errorf("mutating the clone changed the original: %v", st.Keys())
	}
	var nilST *BinarySearchST[string, int]
	if nilST.Clone() != nil {
		t.Error("Clone on nil is not nil")
	}
	var zero BinarySearchST[string, int]
	zeroClone := zero.Clone()
	if zeroClone.Len() != 0 {
		t.Error("Clone of a zero value is not empty")
	}

	// PutIter stores every pair; equal keys keep the last.
	pairs := []struct {
		k string
		v int
	}{{"b", 2}, {"a", 1}}
	st2 := NewBinarySearchST[string, int]()
	st2.PutIter(func(yield func(string, int) bool) {
		for _, p := range pairs {
			if !yield(p.k, p.v) {
				return
			}
		}
	})
	st2.PutIter(func(yield func(string, int) bool) { yield("a", 11) })
	if got := st2.String(); got != "[a:11 b:2]" {
		t.Errorf("after PutIter = %q, want [a:11 b:2] (last pair wins)", got)
	}

	// DeleteFunc removes the matches and keeps the order.
	st3 := build()
	if !st3.DeleteFunc(func(k string, v int) bool { return v%2 == 1 }) {
		t.Error("DeleteFunc reported no deletion")
	}
	if got := st3.String(); got != "[b:2]" {
		t.Errorf("after DeleteFunc = %q, want [b:2]", got)
	}
	if st3.DeleteFunc(func(string, int) bool { return false }) {
		t.Error("DeleteFunc with no matches returned true")
	}
}

func TestJSON(t *testing.T) {
	st := NewBinarySearchST[string, int]()
	st.Put("b", 2)
	st.Put("a", 1)
	st.Put("c", 3)

	blob, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `[{"k":"a","v":1},{"k":"b","v":2},{"k":"c","v":3}]`; string(blob) != want {
		t.Errorf("marshal = %s, want %s (ascending)", blob, want)
	}

	// Round trip into a fresh table.
	round := NewBinarySearchST[string, int]()
	if err := json.Unmarshal(blob, round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := round.String(); got != st.String() {
		t.Errorf("round trip = %s, want %s", got, st.String())
	}

	// Unsorted input and duplicate keys land sorted, last pair wins.
	if err := round.UnmarshalJSON([]byte(`[{"k":"z","v":9},{"k":"a","v":100},{"k":"z","v":-1}]`)); err != nil {
		t.Fatalf("Unmarshal unsorted: %v", err)
	}
	if got := round.String(); got != "[a:100 z:-1]" {
		t.Errorf("after unsorted unmarshal = %s, want [a:100 z:-1]", got)
	}

	// A decode error leaves the table untouched.
	if err := round.UnmarshalJSON([]byte(`{"k":"a"}`)); err == nil {
		t.Error("unmarshal of a non-array document succeeded")
	}
	if got := round.String(); got != "[a:100 z:-1]" {
		t.Errorf("failed unmarshal changed the table: %s", got)
	}
	if err := round.UnmarshalJSON([]byte(`[{"k":3,"v":1}]`)); err == nil {
		t.Error("unmarshal of wrong key type succeeded")
	}

	// null clears, [] clears.
	if err := round.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !round.IsEmpty() {
		t.Error("null did not clear the table")
	}
	if err := round.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Fatalf("unmarshal []: %v", err)
	}
}
