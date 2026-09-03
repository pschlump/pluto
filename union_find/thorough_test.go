package union_find

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the structural invariants of the forest:
// every parent chain terminates at a root within n steps (no cycles),
// Find returns that root, Connected agrees with Find on every pair,
// and Count equals the number of distinct roots.  Call it after any
// structural change.
func checkInvariants(t *testing.T, uf *UnionFind) {
	t.Helper()
	n := uf.Len()

	roots := make(map[int]bool, n)
	for p := 0; p < n; p++ {
		// Walk the raw parent chain: it must reach a root (parent[r] == r)
		// within n links — a longer walk means a cycle.
		r := p
		for steps := 0; ; steps++ {
			if steps > n {
				t.Fatalf("parent chain from %d did not terminate within %d steps (cycle in the forest)", p, n)
			}
			if uf.parent[r] == r {
				break
			}
			r = uf.parent[r]
		}
		// Find must agree with the raw walk, and Find's path halving must
		// not corrupt the forest.
		got, ok := uf.Find(p)
		if !ok {
			t.Fatalf("Find(%d) returned ok=false for an in-range element", p)
		}
		if got != r {
			t.Fatalf("Find(%d)=%d but the raw parent chain ends at %d", p, got, r)
		}
		roots[r] = true
	}

	if uf.Count() != len(roots) {
		t.Fatalf("Count()=%d but there are %d distinct roots", uf.Count(), len(roots))
	}

	// Connected must agree with Find on every pair.
	for p := 0; p < n; p++ {
		for q := 0; q < n; q++ {
			rp, _ := uf.Find(p)
			rq, _ := uf.Find(q)
			if uf.Connected(p, q) != (rp == rq) {
				t.Fatalf("Connected(%d,%d)=%v but Find roots are %d and %d", p, q, rp == rq, rp, rq)
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// naiveUF is the reference model: a map from element to set label,
// where Union relabels every element of one set.  O(n) per operation
// and obviously correct — the property under test is that the ranked,
// path-halving forest computes exactly the same partition.
type naiveUF struct {
	label map[int]int
	count int
}

func newNaiveUF(n int) *naiveUF {
	m := &naiveUF{label: make(map[int]int, n), count: n}
	for i := 0; i < n; i++ {
		m.label[i] = i
	}
	return m
}

func (m *naiveUF) union(p, q int) bool {
	lp, lq := m.label[p], m.label[q]
	if lp == lq {
		return false
	}
	for k, v := range m.label {
		if v == lp {
			m.label[k] = lq
		}
	}
	m.count--
	return true
}

func (m *naiveUF) connected(p, q int) bool {
	return m.label[p] == m.label[q]
}

func TestUnionFindRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 24 // small element space so merges and reconnects are common
	uf := NewUnionFind(n)
	model := newNaiveUF(n)

	verify := func(step int) {
		if uf.Count() != model.count {
			t.Fatalf("step %d: Count()=%d, model has %d sets", step, uf.Count(), model.count)
		}
		// Full pairwise comparison of the connectivity relation.
		for p := 0; p < n; p++ {
			for q := 0; q < n; q++ {
				if got := uf.Connected(p, q); got != model.connected(p, q) {
					t.Fatalf("step %d: Connected(%d,%d)=%v, model says %v",
						step, p, q, got, model.connected(p, q))
				}
			}
		}
		checkInvariants(t, uf)
	}

	for step := range 800 {
		p := rng.Intn(n)
		q := rng.Intn(n)
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4: // Union
			got := uf.Union(p, q)
			want := model.union(p, q)
			if got != want {
				t.Fatalf("step %d: Union(%d,%d)=%v, model says %v", step, p, q, got, want)
			}
		case 5, 6, 7, 8: // Connected
			got := uf.Connected(p, q)
			want := model.connected(p, q)
			if got != want {
				t.Fatalf("step %d: Connected(%d,%d)=%v, model says %v", step, p, q, got, want)
			}
		case 9: // Find — the root is arbitrary, but it must be a set member:
			// Connected(p, root) must hold, and repeated Finds must agree.
			r1, ok1 := uf.Find(p)
			r2, ok2 := uf.Find(q)
			if !ok1 || !ok2 {
				t.Fatalf("step %d: Find returned ok=false for in-range elements", step)
			}
			if (r1 == r2) != model.connected(p, q) {
				t.Fatalf("step %d: Find roots (%d,%d) disagree with model connectivity for (%d,%d)",
					step, r1, r2, p, q)
			}
		}
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)

	// Union every element into one set; the forest must stay consistent.
	for i := 1; i < n; i++ {
		uf.Union(0, i)
	}
	if uf.Count() != 1 {
		t.Errorf("Expected Count()=1 after a full merge, got %d", uf.Count())
	}
	checkInvariants(t, uf)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// samePartition reports whether uf and the naive reference model agree
// on the connectivity of every pair.
func samePartition(t *testing.T, uf *UnionFind, model *naiveUF, n int, context string) {
	t.Helper()
	if uf.Count() != model.count {
		t.Errorf("%s: Count()=%d, model has %d sets", context, uf.Count(), model.count)
	}
	for p := 0; p < n; p++ {
		for q := 0; q < n; q++ {
			if got := uf.Connected(p, q); got != model.connected(p, q) {
				t.Errorf("%s: Connected(%d,%d)=%v, model says %v", context, p, q, got, model.connected(p, q))
			}
		}
	}
}

func TestMarshalJSON(t *testing.T) {
	// Exact output: sets ordered by smallest member, members ascending.
	uf := NewUnionFind(6)
	uf.Union(4, 3)
	uf.Union(3, 2)
	uf.Union(1, 0)
	b, err := json.Marshal(uf)
	if err != nil {
		t.Fatalf("json.Marshal(uf): %v", err)
	}
	if string(b) != `[[0,1],[2,3,4],[5]]` {
		t.Errorf("Expected [[0,1],[2,3,4],[5]], got %s", b)
	}

	// A fresh union-find encodes as n singleton sets.
	if b, err := json.Marshal(NewUnionFind(3)); err != nil || string(b) != `[[0],[1],[2]]` {
		t.Errorf("Expected [[0],[1],[2]] for a fresh union-find, got (%s, %v)", b, err)
	}

	// A zero-value union-find is a tolerated read: [].
	var zero UnionFind
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value union-find, got (%s, %v)", b, err)
	}

	// A direct call on a nil union-find encodes as []; json.Marshal on a
	// nil *UnionFind never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilUF *UnionFind
	if b, err := nilUF.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilUF); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil union-find, got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// A decoded partition replaces the current one; reconstruction goes
	// through Union, so the rebuilt forest is structurally sound.
	uf := NewUnionFind(6)
	if err := json.Unmarshal([]byte(`[[0,1],[2,3,4],[5]]`), uf); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, uf)
	if uf.Count() != 3 {
		t.Errorf("Expected Count()=3 after unmarshal, got %d", uf.Count())
	}
	if !uf.Connected(4, 2) || uf.Connected(0, 5) {
		t.Errorf("Unexpected connectivity after unmarshal.")
	}

	// Unmarshaling replaces the partition; it does not merge into it.
	if err := json.Unmarshal([]byte(`[[0,1,2,3,4,5]]`), uf); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if uf.Count() != 1 {
		t.Errorf("Expected replacement to Count()=1, got %d", uf.Count())
	}
	checkInvariants(t, uf)

	// A round trip preserves the partition exactly.
	b, err := json.Marshal(uf)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewUnionFind(6)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal round trip: %v", err)
	}
	model := newNaiveUF(6)
	for i := 1; i < 6; i++ {
		model.union(0, i)
	}
	samePartition(t, again, model, 6, "after round trip")
	checkInvariants(t, again)

	// An empty array and null reset to singletons.
	merged := NewUnionFind(4)
	merged.Union(0, 1)
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), merged); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if merged.Count() != 4 {
			t.Errorf("Expected %s to reset to 4 singletons, got Count()=%d", data, merged.Count())
		}
		checkInvariants(t, merged)
		merged.Union(0, 1)
	}

	// Decode errors are returned and leave the union-find untouched.
	keep := NewUnionFind(4)
	keep.Union(0, 1)
	for _, badData := range []string{"[[0,", `[["x"]]`, `{"0":[0]}`, "7", `[[0,1],[2,"a"]]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if !keep.Connected(0, 1) || keep.Count() != 3 {
			t.Errorf("Union-find changed after the error on %s.", badData)
		}
		checkInvariants(t, keep)
	}

	// Partition errors — out-of-range, duplicated, or missing elements —
	// are returned and leave the union-find untouched.
	for _, badData := range []string{`[[0,1],[2,4]]`, `[[0,1],[1,2],[3]]`, `[[0,1],[2]]`, `[[0,1],[2,3,-1]]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if !keep.Connected(0, 1) || keep.Count() != 3 {
			t.Errorf("Union-find changed after the error on %s.", badData)
		}
		checkInvariants(t, keep)
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON follows the Union
// contract: storing sets into a nil or zero-value union-find panics
// with a message naming the method, while [] and null — which store
// nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero UnionFind
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value union-find to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with sets to panic on a zero-value union-find.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewUnionFind") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[[0]]`))
	}()

	var nilUF *UnionFind
	if err := nilUF.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil union-find to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with sets to panic on a nil union-find.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil UnionFind") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilUF.UnmarshalJSON([]byte(`[[0]]`))
	}()
}

// TestJSONStructField marshals and unmarshals a UnionFind nested in a
// struct through the encoding/json package.  The union-find must be
// created with NewUnionFind before unmarshaling: for a nil *UnionFind
// field the json package allocates a zero-value structure itself (no
// elements), so non-empty data panics with the standard message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string     `json:"title"`
		Sets  *UnionFind `json:"sets"`
	}

	d := Doc{Title: "pluto", Sets: NewUnionFind(4)}
	d.Sets.Union(0, 1)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","sets":[[0,1],[2],[3]]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created union-find field.
	var out Doc
	out.Sets = NewUnionFind(4)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !out.Sets.Connected(0, 1) || out.Sets.Count() != 3 {
		t.Errorf("Unexpected partition after struct round trip.")
	}

	// A nil union-find field marshals as null (the json package's own
	// nil pointer rule); null sets a pointer field to nil itself — it
	// never calls UnmarshalJSON and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","sets":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Sets: NewUnionFind(2)}
	clearDoc.Sets.Union(0, 1)
	if err := json.Unmarshal([]byte(`{"title":"x","sets":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null sets: %v", err)
	}
	if clearDoc.Sets != nil {
		t.Errorf("Expected null sets to nil the field, got Count()=%d", clearDoc.Sets.Count())
	}

	// Non-empty data into a nil *UnionFind field: the json package
	// allocates a zero-value structure, and the write contract panics
	// through json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated union-find field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewUnionFind") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","sets":[[0]]}`), &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against the naive reference model at fixed seed: the JSON round trip
// must preserve the partition exactly.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run

	const n = 24
	uf := NewUnionFind(n)
	model := newNaiveUF(n)

	for step := range 800 {
		p := rng.Intn(n)
		q := rng.Intn(n)
		got := uf.Union(p, q)
		want := model.union(p, q)
		if got != want {
			t.Fatalf("step %d: Union(%d,%d)=%v, model says %v", step, p, q, got, want)
		}

		if step%37 == 0 {
			b, err := json.Marshal(uf)
			if err != nil {
				t.Fatalf("step %d: json.Marshal: %v", step, err)
			}
			back := NewUnionFind(n)
			if err := json.Unmarshal(b, back); err != nil {
				t.Fatalf("step %d: json.Unmarshal of own output: %v", step, err)
			}
			samePartition(t, back, model, n, "round trip")
			checkInvariants(t, back)
		}
	}

	// The final state must round trip too, byte-for-byte stable.
	b1, err := json.Marshal(uf)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	back := NewUnionFind(n)
	if err := json.Unmarshal(b1, back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	samePartition(t, back, model, n, "final round trip")
	checkInvariants(t, back)
	if b2, err := json.Marshal(back); err != nil || string(b2) != string(b1) {
		t.Errorf("Round trip not stable: %s then (%s, %v)", b1, b2, err)
	}
}
