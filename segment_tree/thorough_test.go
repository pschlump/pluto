package segment_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the array-backed
// tree: every internal node holds the combine of its two children,
// every leaf at size+i (i < n) holds Value(i), and every padding leaf
// holds the identity.  Call it after any structural change.
func checkInvariants(t *testing.T, st *SegmentTree[int]) {
	t.Helper()
	if len(st.tree) != 2*st.size {
		t.Fatalf("internal tree has %d slots, expected 2*%d", len(st.tree), st.size)
	}
	if st.size < st.n || st.size >= 2*st.n && st.n > 1 {
		t.Fatalf("size=%d is not the smallest power of two >= n=%d", st.size, st.n)
	}
	for k := 1; k < st.size; k++ {
		want := st.combine(st.tree[2*k], st.tree[2*k+1])
		if st.tree[k] != want {
			t.Fatalf("tree[%d]=%d, expected combine of children = %d", k, st.tree[k], want)
		}
	}
	for i := st.n; i < st.size; i++ {
		if st.tree[st.size+i] != st.identity {
			t.Fatalf("padding leaf %d is %d, expected the identity %d",
				i, st.tree[st.size+i], st.identity)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against reference models (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestSegmentTreeRandomizedModel cross-checks three trees over the SAME
// data — a sum tree (NewSegmentTree), a min tree, and a max tree (both
// via NewSegmentTreeFunc) — against brute-force range loops over a
// naive reference slice, with random interleaved updates and queries.
func TestSegmentTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}

	model := make([]int, n)
	for i := range model {
		model[i] = rng.Intn(201) - 100
	}
	sumTree := NewSegmentTree(model)
	minTree := NewSegmentTreeFunc(model, min, math.MaxInt)
	maxTree := NewSegmentTreeFunc(model, max, math.MinInt)

	brute := func(combine func(a, b int) int, identity, lo, hi int) int {
		acc := identity
		for i := lo; i <= hi; i++ {
			acc = combine(acc, model[i])
		}
		return acc
	}

	verify := func(step int) {
		for i := 0; i < n; i++ {
			if v, ok := sumTree.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: sumTree.Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
			if v, ok := minTree.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: minTree.Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
			if v, ok := maxTree.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: maxTree.Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
		}
		checkInvariants(t, sumTree)
		checkInvariants(t, minTree)
		checkInvariants(t, maxTree)
	}

	for step := range 3000 {
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Update
			i := rng.Intn(n)
			v := rng.Intn(201) - 100
			if !sumTree.Update(i, v) || !minTree.Update(i, v) || !maxTree.Update(i, v) {
				t.Fatalf("step %d: Update(%d, %d) returned false for an in-range slot", step, i, v)
			}
			model[i] = v
		case 4, 5, 6, 7: // Query all three trees over one random range
			lo := rng.Intn(n)
			hi := lo + rng.Intn(n-lo)
			if s, ok := sumTree.Query(lo, hi); !ok || s != brute(func(a, b int) int { return a + b }, 0, lo, hi) {
				t.Fatalf("step %d: sum Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(func(a, b int) int { return a + b }, 0, lo, hi))
			}
			if s, ok := minTree.Query(lo, hi); !ok || s != brute(min, math.MaxInt, lo, hi) {
				t.Fatalf("step %d: min Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(min, math.MaxInt, lo, hi))
			}
			if s, ok := maxTree.Query(lo, hi); !ok || s != brute(max, math.MinInt, lo, hi) {
				t.Fatalf("step %d: max Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(max, math.MinInt, lo, hi))
			}
		case 8: // single-slot query
			i := rng.Intn(n)
			if s, ok := sumTree.Query(i, i); !ok || s != model[i] {
				t.Fatalf("step %d: sum Query(%d,%d)=(%d,%v), model has %d", step, i, i, s, ok, model[i])
			}
		case 9: // full-range query
			if s, ok := sumTree.Query(0, n-1); !ok || s != brute(func(a, b int) int { return a + b }, 0, 0, n-1) {
				t.Fatalf("step %d: sum full Query=(%d,%v), model has %d",
					step, s, ok, brute(func(a, b int) int { return a + b }, 0, 0, n-1))
			}
		}
		if step%251 == 0 {
			verify(step)
		}
	}
	verify(3000)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// valuesOfJSON collects the per-slot values of st through the public
// Value API — what a JSON encoding of the tree must reproduce.
func valuesOfJSON(t *testing.T, st *SegmentTree[int]) []int {
	t.Helper()
	var got []int
	for i := 0; i < st.Len(); i++ {
		v, ok := st.Value(i)
		if !ok {
			t.Fatalf("Value(%d) reported ok=false on a %d-slot tree", i, st.Len())
		}
		got = append(got, v)
	}
	return got
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, per-slot values in slot order — the leaf row,
	// not the internal 2*size combine array.
	st := NewSegmentTree([]int{3, 1, 2})
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("json.Marshal(st): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// A zero-value tree is a tolerated read: [].
	var zero SegmentTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *SegmentTree never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilTree *SegmentTree[int]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Float values use their normal JSON encoding.
	f := NewSegmentTree([]float64{1.5, -2.25})
	if b, err := json.Marshal(f); err != nil || string(b) != "[1.5,-2.25]" {
		t.Errorf("Expected [1.5,-2.25], got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order becomes the slot order; the slot count follows the
	// array length, and the combine function is kept — Query still works.
	st := NewSegmentTree([]int{9, 9, 9, 9, 9})
	if err := json.Unmarshal([]byte("[3,1,2]"), st); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, st)), "[3 1 2]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
	if got, want := st.Len(), 3; got != want {
		t.Errorf("Expected Len()=%d after unmarshal, got %d", want, got)
	}
	if s, ok := st.Query(0, 2); !ok || s != 6 {
		t.Errorf("Expected Query(0,2)=(6, true) after unmarshal, got (%d, %v)", s, ok)
	}
	checkInvariants(t, st)

	// A round trip rebuilds a structurally sound tree with the same
	// per-slot values.
	orig := NewSegmentTree([]int{3, 1, 4, 1, 5, 9, 2, 6})
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewSegmentTree([]int{0})
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, again)), "[3 1 4 1 5 9 2 6]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	checkInvariants(t, again)

	// A non-sum combine survives the round trip too: the min tree still
	// answers range-min queries after a rebuild.
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	minTree := NewSegmentTreeFunc([]int{5, 4, 3, 2, 1}, min, math.MaxInt)
	mb, err := json.Marshal(minTree)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	minAgain := NewSegmentTreeFunc([]int{0}, min, math.MaxInt)
	if err := json.Unmarshal(mb, minAgain); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := minAgain.Query(0, 4); !ok || v != 1 {
		t.Errorf("Expected min Query(0,4)=(1, true) after round trip, got (%d, %v)", v, ok)
	}
	checkInvariants(t, minAgain)

	// An empty array and null clear the tree to zero slots.
	full := NewSegmentTree([]int{9})
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !full.IsEmpty() || full.Len() != 0 {
			t.Errorf("Expected %s to clear the tree, got Len()=%d", data, full.Len())
		}
		checkInvariants(t, full)
		// The tree stays usable: the combine function is kept, and a
		// fresh unmarshal rebuilds it.
		if err := json.Unmarshal([]byte("[7]"), full); err != nil {
			t.Fatalf("json.Unmarshal([7]) after %s: %v", data, err)
		}
		if s, ok := full.Query(0, 0); !ok || s != 7 {
			t.Errorf("Expected Query(0,0)=(7, true) after rebuild, got (%d, %v)", s, ok)
		}
	}

	// Decode errors are returned and leave the tree untouched.
	keep := NewSegmentTree([]int{4, 2})
	for _, badData := range []string{"[1,", `["x"]`, `{"v":1}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(valuesOfJSON(t, keep)), "[4 2]"; got != want {
			t.Errorf("Tree changed after the error on %s: got %s, want %s", badData, got, want)
		}
	}
	checkInvariants(t, keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON follows the
// package's write contract: storing elements into a nil tree or a
// zero-value tree (no combine function) panics, while [] and null —
// which store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilTree *SegmentTree[int]
	for _, data := range []string{"[]", "null"} {
		if err := nilTree.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil tree to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil SegmentTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilTree.UnmarshalJSON([]byte("[1,2]"))
	}()

	var zero SegmentTree[int]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value tree to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewSegmentTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte("[1,2]"))
	}()
}

// TestJSONStructField marshals and unmarshals a SegmentTree nested in a
// struct through the encoding/json package.  The tree must be created
// with NewSegmentTree/NewSegmentTreeFunc before unmarshaling: for a nil
// *SegmentTree field the json package allocates a zero-value tree
// itself (no combine function), so non-empty data panics with the
// write-contract message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Name string            `json:"name"`
		Mods *SegmentTree[int] `json:"mods"`
	}

	d := Doc{Name: "pluto", Mods: NewSegmentTree([]int{1, 2, 3})}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"name":"pluto","mods":[1,2,3]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created tree field.
	var out Doc
	out.Mods = NewSegmentTree([]int{0})
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, out.Mods)), "[1 2 3]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
	checkInvariants(t, out.Mods)

	// A nil tree field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created tree and never allocates.
	if b, err := json.Marshal(Doc{Name: "x"}); err != nil || string(b) != `{"name":"x","mods":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Name: "x", Mods: NewSegmentTree([]int{5})}
	if err := json.Unmarshal([]byte(`{"name":"x","mods":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null mods: %v", err)
	}
	if !clearDoc.Mods.IsEmpty() {
		t.Errorf("Expected null mods to clear the tree.")
	}

	// Non-empty data into a nil *SegmentTree field: the json package
	// allocates a zero-value tree, and the write contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated tree field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSegmentTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"name":"x","mods":[1]}`), &bad)
	}()
}
