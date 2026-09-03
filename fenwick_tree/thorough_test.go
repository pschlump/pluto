package fenwick_tree

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

	"github.com/pschlump/pluto/g_lib"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the 1-based
// internal array: for every k in 1..n, tree[k] holds the sum of the
// lowbit(k) values ending at slot k-1 — i.e. tree[k] == Sum(k-1) -
// Sum(k-1-lowbit(k)), where the Sums are reconstructed from the public
// API.  Call it after any structural change.
func checkInvariants[T g_lib.Numeric](t *testing.T, ft *FenwickTree[T]) {
	t.Helper()
	n := ft.Len()
	if len(ft.tree) != n+1 {
		t.Fatalf("internal tree has %d slots, expected %d", len(ft.tree), n+1)
	}
	var zero T
	if ft.tree[0] != zero {
		t.Fatalf("tree[0] is %v, expected the unused slot to stay zero", ft.tree[0])
	}
	for k := 1; k <= n; k++ {
		lo := k - (k & -k) // tree[k] covers slots lo..k-1 (0-based)
		want := ft.Sum(k-1) - ft.Sum(lo-1)
		if ft.tree[k] != want {
			t.Fatalf("tree[%d]=%v, expected %v (sum of slots %d..%d)", k, ft.tree[k], want, lo, k-1)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestFenwickRandomizedModel cross-checks the tree against a naive
// reference: a plain slice whose prefix and range sums are computed by
// brute-force loops.  O(n) per query and obviously correct — the
// property under test is that the binary-indexed bookkeeping computes
// exactly the same sums under interleaved Adds and Sets.
func TestFenwickRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64
	ft := NewFenwickTree[int](n)
	model := make([]int, n)

	naiveRange := func(lo, hi int) int {
		s := 0
		for i := lo; i <= hi; i++ {
			s += model[i]
		}
		return s
	}

	verify := func(step int) {
		for i := 0; i < n; i++ {
			if v, ok := ft.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
			if s := ft.Sum(i); s != naiveRange(0, i) {
				t.Fatalf("step %d: Sum(%d)=%d, model has %d", step, i, s, naiveRange(0, i))
			}
		}
		checkInvariants(t, ft)
	}

	for step := range 3000 {
		i := rng.Intn(n)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Add
			delta := rng.Intn(201) - 100 // in [-100, 100]
			if !ft.Add(i, delta) {
				t.Fatalf("step %d: Add(%d, %d) returned false for an in-range slot", step, i, delta)
			}
			model[i] += delta
		case 4, 5: // Set
			v := rng.Intn(201) - 100
			if !ft.Set(i, v) {
				t.Fatalf("step %d: Set(%d, %d) returned false for an in-range slot", step, i, v)
			}
			model[i] = v
		case 6, 7: // RangeSum
			lo := rng.Intn(n)
			hi := lo + rng.Intn(n-lo)
			if s, ok := ft.RangeSum(lo, hi); !ok || s != naiveRange(lo, hi) {
				t.Fatalf("step %d: RangeSum(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, naiveRange(lo, hi))
			}
		case 8: // Sum
			if s := ft.Sum(i); s != naiveRange(0, i) {
				t.Fatalf("step %d: Sum(%d)=%d, model has %d", step, i, s, naiveRange(0, i))
			}
		case 9: // Value
			if v, ok := ft.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
		}
		if step%251 == 0 {
			verify(step)
		}
	}
	verify(3000)
}

// TestFenwickFromMatchesAdd verifies that the O(n) build of
// NewFenwickTreeFrom produces the same internal array as n individual
// Adds, at a fixed seed.
func TestFenwickFromMatchesAdd(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 200
	data := make([]int, n)
	for i := range data {
		data[i] = rng.Intn(1000) - 500
	}

	fromData := NewFenwickTreeFrom(data)
	fromAdds := NewFenwickTree[int](n)
	for i, v := range data {
		fromAdds.Add(i, v)
	}

	for k := range fromData.tree {
		if fromData.tree[k] != fromAdds.tree[k] {
			t.Fatalf("tree[%d]: bulk build has %d, incremental build has %d",
				k, fromData.tree[k], fromAdds.tree[k])
		}
	}
	checkInvariants(t, fromData)
	checkInvariants(t, fromAdds)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// valuesOfJSON collects the per-index values of ft through the public
// Value API — what a JSON encoding of the tree must reproduce.
func valuesOfJSON(t *testing.T, ft *FenwickTree[int]) []int {
	t.Helper()
	var got []int
	for i := 0; i < ft.Len(); i++ {
		v, ok := ft.Value(i)
		if !ok {
			t.Fatalf("Value(%d) reported ok=false on a %d-slot tree", i, ft.Len())
		}
		got = append(got, v)
	}
	return got
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, per-index values in slot order.
	ft := NewFenwickTreeFrom([]int{3, 1, 2})
	b, err := json.Marshal(ft)
	if err != nil {
		t.Fatalf("json.Marshal(ft): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// A fresh tree encodes its zeros, one per slot.
	if b, err := json.Marshal(NewFenwickTree[int](3)); err != nil || string(b) != "[0,0,0]" {
		t.Errorf("Expected [0,0,0] for a fresh 3-slot tree, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero FenwickTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *FenwickTree never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilTree *FenwickTree[int]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Float values use their normal JSON encoding.
	f := NewFenwickTreeFrom([]float64{1.5, -2.25})
	if b, err := json.Marshal(f); err != nil || string(b) != "[1.5,-2.25]" {
		t.Errorf("Expected [1.5,-2.25], got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order becomes the slot order; the slot count follows the
	// array length.
	ft := NewFenwickTree[int](5)
	if err := json.Unmarshal([]byte("[3,1,2]"), ft); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, ft)), "[3 1 2]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
	if got, want := ft.Len(), 3; got != want {
		t.Errorf("Expected Len()=%d after unmarshal, got %d", want, got)
	}
	if s := ft.Sum(2); s != 6 {
		t.Errorf("Expected Sum(2)=6 after unmarshal, got %d", s)
	}
	checkInvariants(t, ft)

	// A round trip rebuilds a structurally sound tree with the same
	// per-index values.
	orig := NewFenwickTreeFrom([]int{3, 1, 4, 1, 5, 9, 2, 6})
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewFenwickTree[int](1)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, again)), "[3 1 4 1 5 9 2 6]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	checkInvariants(t, again)

	// A zero-value tree is resized to fit the decoded values.
	var zero FenwickTree[int]
	if err := json.Unmarshal([]byte("[7,8]"), &zero); err != nil {
		t.Fatalf("json.Unmarshal on a zero-value tree: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, &zero)), "[7 8]"; got != want {
		t.Errorf("Expected %s on the zero-value tree, got %s", want, got)
	}
	if s, ok := zero.RangeSum(0, 1); !ok || s != 15 {
		t.Errorf("Expected RangeSum(0,1)=(15, true) on the zero-value tree, got (%d, %v)", s, ok)
	}
	checkInvariants(t, &zero)

	// An empty array and null clear the tree to zero slots.
	full := NewFenwickTreeFrom([]int{9})
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !full.IsEmpty() || full.Len() != 0 {
			t.Errorf("Expected %s to clear the tree, got Len()=%d", data, full.Len())
		}
	}

	// Decode errors are returned and leave the tree untouched.
	keep := NewFenwickTreeFrom([]int{4, 2})
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
// package's write contract: storing elements into a nil tree panics,
// while [] and null — which store nothing — are tolerated everywhere,
// and a zero-value tree is resized rather than panicking.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilTree *FenwickTree[int]
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil FenwickTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilTree.UnmarshalJSON([]byte("[1,2]"))
	}()
}

// TestJSONStructField marshals and unmarshals a FenwickTree nested in a
// struct through the encoding/json package.  Unlike the packages with a
// constructor-set function, a nil *FenwickTree field unmarshals fine:
// the json package allocates the tree and UnmarshalJSON resizes it.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Name string            `json:"name"`
		Sums *FenwickTree[int] `json:"sums"`
	}

	d := Doc{Name: "pluto", Sums: NewFenwickTreeFrom([]int{1, 2, 3})}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"name":"pluto","sums":[1,2,3]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a nil *FenwickTree field: the json package
	// allocates the tree itself and the decoded values resize it.
	var out Doc
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(valuesOfJSON(t, out.Sums)), "[1 2 3]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}
	checkInvariants(t, out.Sums)

	// A nil tree field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created tree and never allocates.
	if b, err := json.Marshal(Doc{Name: "x"}); err != nil || string(b) != `{"name":"x","sums":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Name: "x", Sums: NewFenwickTreeFrom([]int{5})}
	if err := json.Unmarshal([]byte(`{"name":"x","sums":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal(null field): %v", err)
	}
	if !clearDoc.Sums.IsEmpty() {
		t.Errorf("Expected null to clear the tree field, got Len()=%d", clearDoc.Sums.Len())
	}
}
