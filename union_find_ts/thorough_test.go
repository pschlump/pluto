package union_find_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the structural invariants of the forest:
// every parent chain terminates at a root within n steps (no cycles),
// Find returns that root, Connected agrees with Find on every pair,
// and Count equals the number of distinct roots.  It reads the
// internals without the lock — single-goroutine tests only.
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
		case 9: // Find — the root is arbitrary, but the two Finds must
			// agree with the model's connectivity for the pair.
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
// Concurrency (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestConcurrentUnionFind runs merger and observer goroutines against
// one shared union-find.  It is primarily a test for the race detector
// (`make race`): Find and Connected take the WRITE lock (path halving
// mutates the parent links), so a query racing a Union must not trip
// the detector.  The final accounting is deterministic: after the
// mergers finish, a sequential pass joins everything into one set.
func TestConcurrentUnionFind(t *testing.T) {
	const n = 512
	uf := NewUnionFind(n)

	var wg sync.WaitGroup

	// Mergers: each goroutine chains its own disjoint band of elements
	// (g*band .. g*band+band-1) into one set.  Bands never overlap, so
	// no assertion on intermediate Count is needed.
	const mergers = 8
	const band = n / mergers
	for g := range mergers {
		wg.Go(func() {
			base := g * band
			for i := 1; i < band; i++ {
				uf.Union(base, base+i)
			}
		})
	}

	// Observers hammer the query operations — including the write-locked
	// Find and Connected — while the mergers work.
	for range 2 {
		wg.Go(func() {
			for range 2000 {
				_, _ = uf.Find(rand.Intn(n))
				_ = uf.Connected(rand.Intn(n), rand.Intn(n))
				_ = uf.Count()
				_ = uf.Len()
			}
		})
	}

	wg.Wait()

	// Each band must now be one set.
	for g := range mergers {
		base := g * band
		for i := 1; i < band; i++ {
			if !uf.Connected(base, base+i) {
				t.Fatalf("expected %d and %d to be connected after concurrent merges", base, base+i)
			}
		}
	}

	// Join the bands sequentially; exactly one set must remain.
	for g := 1; g < mergers; g++ {
		if !uf.Union(0, g*band) {
			t.Fatalf("expected Union(0,%d) to merge two bands", g*band)
		}
	}
	if uf.Count() != 1 {
		t.Fatalf("expected Count()=1 after joining all bands, got %d", uf.Count())
	}
	if !uf.Connected(0, n-1) {
		t.Errorf("expected 0 and %d to be connected in the single remaining set", n-1)
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// checkSamePartition verifies that two same-sized union-finds agree on
// the connectivity of every pair of elements (and on Count).
func checkSamePartition(t *testing.T, context string, a, b *UnionFind) {
	t.Helper()
	n := a.Len()
	if b.Len() != n {
		t.Fatalf("%s: sizes differ: %d vs %d", context, n, b.Len())
	}
	if a.Count() != b.Count() {
		t.Errorf("%s: Count()=%d but the other union-find reports %d", context, a.Count(), b.Count())
	}
	for p := 0; p < n; p++ {
		for q := 0; q < n; q++ {
			if a.Connected(p, q) != b.Connected(p, q) {
				t.Fatalf("%s: Connected(%d,%d) differs between the two union-finds", context, p, q)
			}
		}
	}
}

func TestMarshalJSON(t *testing.T) {
	// A fresh union-find is n singleton sets.
	fresh := NewUnionFind(4)
	b, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != "[[0],[1],[2],[3]]" {
		t.Errorf("Expected [[0],[1],[2],[3]], got %s", b)
	}

	// Sets are ordered by smallest member, members ascending: the
	// encoding of a partition is deterministic regardless of the order
	// the unions ran in.
	uf := NewUnionFind(6)
	uf.Union(4, 5)
	uf.Union(0, 1)
	uf.Union(1, 2)
	b, err = json.Marshal(uf)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != "[[0,1,2],[3],[4,5]]" {
		t.Errorf("Expected [[0,1,2],[3],[4,5]], got %s", b)
	}

	// A zero-value union-find is a tolerated read: [].
	var zero UnionFind
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value union-find, got (%s, %v)", b, err)
	}

	// A direct call on a nil union-find encodes as []; json.Marshal on
	// a nil *UnionFind never reaches the method — the json package
	// writes null for nil pointers itself.
	var nilUF *UnionFind
	if b, err := nilUF.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil union-find call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilUF); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil union-find, got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded sets replace the current partition.
	uf := NewUnionFind(6)
	if err := json.Unmarshal([]byte("[[0,1,2],[3],[4,5]]"), uf); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if uf.Count() != 3 {
		t.Errorf("Expected Count()=3, got %d", uf.Count())
	}
	if !uf.Connected(0, 2) || !uf.Connected(4, 5) {
		t.Errorf("Expected the decoded sets to be merged.")
	}
	if uf.Connected(0, 3) || uf.Connected(2, 4) {
		t.Errorf("Expected distinct decoded sets to stay disconnected.")
	}
	checkInvariants(t, uf)

	// The set order in the document is irrelevant; so is member order
	// within a set.
	uf2 := NewUnionFind(6)
	if err := json.Unmarshal([]byte("[[5,4],[3],[1,2,0]]"), uf2); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkSamePartition(t, "same partition, different encoding order", uf, uf2)

	// A second unmarshal replaces the first partition.
	if err := json.Unmarshal([]byte("[[0,5],[1],[2],[3],[4]]"), uf); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if uf.Count() != 5 || !uf.Connected(0, 5) || uf.Connected(0, 1) {
		t.Errorf("Expected the second unmarshal to replace the partition.")
	}
	checkInvariants(t, uf)

	// An empty array resets to singletons; null is tolerated too.
	if err := json.Unmarshal([]byte("[]"), uf); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if uf.Count() != 6 {
		t.Errorf("Expected Count()=6 after unmarshaling [], got %d", uf.Count())
	}
	for p := range 6 {
		if root, ok := uf.Find(p); !ok || root != p {
			t.Errorf("Expected element %d to be a singleton after [], got root %d", p, root)
		}
	}
	if err := json.Unmarshal([]byte("null"), uf); err != nil {
		t.Errorf("Expected null to be tolerated, got %v", err)
	}
}

// TestUnmarshalJSONErrors verifies that malformed JSON and invalid
// partitions are reported and leave the union-find untouched.
func TestUnmarshalJSONErrors(t *testing.T) {
	uf := NewUnionFind(5)
	uf.Union(0, 1)
	uf.Union(2, 3)

	before, err := json.Marshal(uf)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	for _, data := range []string{
		`[[0,1],`,                  // malformed JSON
		`{"sets":[[0,1]]}`,         // not an array
		`[0,1,2,3,4]`,              // not an array of arrays
		`[[0,1],[2],[3]]`,          // element 4 missing
		`[[0,1],[2],[3],[4],[4]]`,  // too many elements / duplicate
		`[[0,0],[1],[2],[3],[4]]`,  // duplicate element
		`[[0,1],[2],[3],[4],[5]]`,  // 5 out of range
		`[[-1,0],[1],[2],[3],[4]]`, // -1 out of range
	} {
		if err := json.Unmarshal([]byte(data), uf); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", data)
		}
		after, err := json.Marshal(uf)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if string(after) != string(before) {
			t.Errorf("Unmarshal of %s changed the union-find: %s -> %s", data, before, after)
		}
	}
	if uf.Count() != 3 {
		t.Errorf("Expected Count()=3 after the decode errors, got %d", uf.Count())
	}
	checkInvariants(t, uf)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value union-find panics
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
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value union-find.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewUnionFind") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[[0,1]]`))
	}()

	var nilUF *UnionFind
	for _, data := range []string{"[]", "null"} {
		if err := nilUF.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil union-find to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil union-find.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil UnionFind") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilUF.UnmarshalJSON([]byte(`[[0,1]]`))
	}()
}

// TestJSONRoundTrip cross-checks marshal/unmarshal round-trips against
// the naive reference model at a fixed seed.
func TestJSONRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run

	const n = 20
	uf := NewUnionFind(n)
	model := newNaiveUF(n)
	for range 100 {
		p, q := rng.Intn(n), rng.Intn(n)
		if got, want := uf.Union(p, q), model.union(p, q); got != want {
			t.Fatalf("Union(%d,%d)=%v, model says %v", p, q, got, want)
		}
	}

	b, err := json.Marshal(uf)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// The decoded copy must compute exactly the same partition.
	copy := NewUnionFind(n)
	if err := json.Unmarshal(b, copy); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkSamePartition(t, "round-trip", uf, copy)
	checkInvariants(t, copy)

	// Marshaling the copy must reproduce the same document byte for
	// byte (deterministic encoding), and decoding into the copy again
	// must be a no-op.
	b2, err := json.Marshal(copy)
	if err != nil {
		t.Fatalf("json.Marshal of the copy: %v", err)
	}
	if string(b) != string(b2) {
		t.Errorf("Expected a stable encoding, got %s then %s", b, b2)
	}
	if err := json.Unmarshal(b2, copy); err != nil {
		t.Fatalf("second json.Unmarshal: %v", err)
	}
	checkSamePartition(t, "second round-trip", uf, copy)
}
