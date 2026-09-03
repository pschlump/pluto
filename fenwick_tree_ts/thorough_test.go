package fenwick_tree_ts

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

	"github.com/pschlump/pluto/g_lib"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the 1-based
// internal array: for every k in 1..n, tree[k] holds the sum of the
// lowbit(k) values ending at slot k-1.  It reads the internals without
// the lock — single-goroutine tests only.
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
// reference slice with brute-force range sums — the same property as in
// the plain package, exercised through the locked API.
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
			delta := rng.Intn(201) - 100
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

// -------------------------------------------------------------------------------------------------------
// Concurrency (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestConcurrentFenwick runs writer and reader goroutines against one
// shared tree.  It is primarily a test for the race detector
// (`make race`): each writer owns a disjoint band of slots (the
// internal tree-array updates of different bands still overlap, so a
// missing lock would corrupt the sums), and the final accounting is
// deterministic.
func TestConcurrentFenwick(t *testing.T) {
	const n = 512
	ft := NewFenwickTree[int](n)

	stop := make(chan struct{})
	var readersWG sync.WaitGroup

	// Readers hammer the query operations until the writers finish.
	for range 4 {
		readersWG.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				i := rand.Intn(n)
				_ = ft.Sum(i)
				_, _ = ft.RangeSum(0, i)
				_, _ = ft.Value(i)
				_ = ft.Len()
				_ = ft.IsEmpty()
			}
		})
	}

	// Writers: each goroutine Adds to and Sets its own disjoint band of
	// slots (g*band .. g*band+band-1), so the final value of every slot
	// is deterministic.
	var writersWG sync.WaitGroup
	const writers = 8
	const band = n / writers
	for g := range writers {
		writersWG.Go(func() {
			base := g * band
			for i := 0; i < band; i++ {
				ft.Add(base+i, 1)
				ft.Set(base+i, 2)
				ft.Add(base+i, 3) // final value: 5 per slot
			}
		})
	}

	writersWG.Wait()
	close(stop)
	readersWG.Wait()

	// Every slot must hold exactly 5, so Sum(n-1) == 5n.
	for i := 0; i < n; i++ {
		if v, ok := ft.Value(i); !ok || v != 5 {
			t.Fatalf("expected Value(%d)=5 after concurrent writes, got (%d, %v)", i, v, ok)
		}
	}
	if s := ft.Sum(n - 1); s != 5*n {
		t.Fatalf("expected Sum(%d)=%d after concurrent writes, got %d", n-1, 5*n, s)
	}
	checkInvariants(t, ft)
}

// TestConcurrentCompound exercises Lock + Nl* read-modify-write
// sequences from multiple goroutines against one shared tree.  Each
// goroutine increments its own slot 100 times as a locked
// NlValue/NlSet pair — without the lock the pair would not be atomic,
// and the shared internal array would race under -race.
func TestConcurrentCompound(t *testing.T) {
	const writers = 8
	ft := NewFenwickTree[int](writers)

	var wg sync.WaitGroup
	for g := range writers {
		wg.Go(func() {
			for range 100 {
				ft.Lock()
				v, ok := ft.NlValue(g)
				if ok {
					ft.NlSet(g, v+1)
				}
				ft.Unlock()
			}
		})
	}
	wg.Wait()

	for g := 0; g < writers; g++ {
		if v, ok := ft.Value(g); !ok || v != 100 {
			t.Fatalf("expected Value(%d)=100 after 100 locked increments, got (%d, %v)", g, v, ok)
		}
	}
	if s := ft.Sum(writers - 1); s != 100*writers {
		t.Errorf("expected Sum(%d)=%d, got %d", writers-1, 100*writers, s)
	}
	checkInvariants(t, ft)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

func TestMarshalJSON(t *testing.T) {
	// Exact array output: element i is Value(i), in index order.
	ft := NewFenwickTreeFrom([]int{3, 1, 2})
	b, err := json.Marshal(ft)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// The encoding carries the per-index values, not the internal
	// prefix-sum array: after Add/Set the values are still exact.
	if !ft.Add(0, 10) || !ft.Set(2, 30) {
		t.Fatalf("Add/Set returned false for in-range slots.")
	}
	if b, err := json.Marshal(ft); err != nil || string(b) != "[13,1,30]" {
		t.Errorf("Expected [13,1,30] after Add/Set, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero FenwickTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *FenwickTree never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilFT *FenwickTree[int]
	if b, err := nilFT.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilFT); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element i becomes Value(i), and the
	// prefix sums work on the rebuilt tree.
	ft := NewFenwickTree[int](1)
	if err := json.Unmarshal([]byte("[3,1,2]"), ft); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ft.Len() != 3 {
		t.Errorf("Expected Len()=3 after unmarshal, got %d", ft.Len())
	}
	for i, want := range []int{3, 1, 2} {
		if v, ok := ft.Value(i); !ok || v != want {
			t.Errorf("Expected Value(%d)=%d, got (%d, %v)", i, want, v, ok)
		}
	}
	if s := ft.Sum(2); s != 6 {
		t.Errorf("Expected Sum(2)=6 after unmarshal, got %d", s)
	}
	checkInvariants(t, ft)

	// A round trip rebuilds a structurally sound tree.
	src := NewFenwickTreeFrom([]int{4, -1, 7, 0, 5})
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewFenwickTree[int](8)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if again.Len() != 5 {
		t.Errorf("Expected Len()=5 after round trip, got %d", again.Len())
	}
	if s, ok := again.RangeSum(1, 3); !ok || s != 6 {
		t.Errorf("Expected RangeSum(1,3)=(6, true) after round trip, got (%d, %v)", s, ok)
	}
	checkInvariants(t, again)

	// Unmarshaling replaces the contents — including the size; it does
	// not add deltas.
	if err := json.Unmarshal([]byte("[7]"), ft); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ft.Len() != 1 {
		t.Errorf("Expected replacement, got Len()=%d, want 1", ft.Len())
	}
	if v, ok := ft.Value(0); !ok || v != 7 {
		t.Errorf("Expected Value(0)=7 after replacement, got (%d, %v)", v, ok)
	}

	// An empty array and null clear the tree.
	full := NewFenwickTreeFrom([]int{1, 2, 3})
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !full.IsEmpty() || full.Len() != 0 {
			t.Errorf("Expected %s to clear the tree, got Len()=%d", data, full.Len())
		}
		if s := full.Sum(0); s != 0 {
			t.Errorf("Expected Sum(0)=0 on the cleared tree, got %d", s)
		}
	}

	// A zero-value tree can be unmarshaled into: a Fenwick tree has no
	// constructor-set functions, so a rebuilt tree is fully usable.
	var zero FenwickTree[int]
	if err := json.Unmarshal([]byte("[1,2,3]"), &zero); err != nil {
		t.Fatalf("json.Unmarshal into a zero value: %v", err)
	}
	if s := zero.Sum(2); s != 6 {
		t.Errorf("Expected Sum(2)=6 on a tree built from the zero value, got %d", s)
	}
	if !zero.Add(0, 1) {
		t.Errorf("Expected Add to work on a tree built from the zero value.")
	}

	// Decode errors are returned and leave the tree untouched.
	keep := NewFenwickTreeFrom([]int{9, 8})
	for _, badData := range []string{"[1,", `["x"]`, `{"0":1}`, "7", `[1,"a"]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.Len() != 2 {
			t.Errorf("Len changed after the error on %s: %d", badData, keep.Len())
		}
		if s := keep.Sum(1); s != 17 {
			t.Errorf("Tree changed after the error on %s: Sum(1)=%d", badData, s)
		}
	}
	checkInvariants(t, keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the
// insert family: storing values into a nil tree panics with a message
// naming the method, while [] and null — which store nothing — are
// tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilFT *FenwickTree[int]
	for _, data := range []string{"[]", "null"} {
		if err := nilFT.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil tree to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with values to panic on a nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "fenwick_tree_ts: UnmarshalJSON") || !strings.Contains(msg, "nil FenwickTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilFT.UnmarshalJSON([]byte("[1]"))
	}()
}
