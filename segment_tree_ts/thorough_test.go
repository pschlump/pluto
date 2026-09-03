package segment_tree_ts

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
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the array-backed
// tree: every internal node holds the combine of its two children,
// every leaf at size+i (i < n) holds Value(i), and every padding leaf
// holds the identity.  It reads the internals without the lock —
// single-goroutine tests only.
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

// TestSegmentTreeRandomizedModel cross-checks three trees over the
// SAME data — a sum tree (NewSegmentTree), a min tree, and a max tree
// (both via NewSegmentTreeFunc) — against brute-force range loops over
// a naive reference slice, with random interleaved updates and
// queries.  The same property as in the plain package, exercised
// through the locked API.
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
	sum := func(a, b int) int { return a + b }

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
			if s, ok := sumTree.Query(lo, hi); !ok || s != brute(sum, 0, lo, hi) {
				t.Fatalf("step %d: sum Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(sum, 0, lo, hi))
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
			if s, ok := sumTree.Query(0, n-1); !ok || s != brute(sum, 0, 0, n-1) {
				t.Fatalf("step %d: sum full Query=(%d,%v), model has %d",
					step, s, ok, brute(sum, 0, 0, n-1))
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

// TestConcurrentSegmentTree runs writer and reader goroutines against
// one shared tree.  It is primarily a test for the race detector
// (`make race`): each writer owns a disjoint band of slots (the
// internal node updates of different bands still overlap near the
// root, so a missing lock would corrupt the queries), and the final
// accounting is deterministic.
func TestConcurrentSegmentTree(t *testing.T) {
	const n = 512
	data := make([]int, n)
	st := NewSegmentTree(data)

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
				_, _ = st.Query(0, i)
				_, _ = st.Value(i)
				_ = st.Len()
				_ = st.IsEmpty()
			}
		})
	}

	// Writers: each goroutine Updates its own disjoint band of slots
	// (g*band .. g*band+band-1), so the final value of every slot is
	// deterministic.
	var writersWG sync.WaitGroup
	const writers = 8
	const band = n / writers
	for g := range writers {
		writersWG.Go(func() {
			base := g * band
			for i := 0; i < band; i++ {
				st.Update(base+i, 1)
				st.Update(base+i, 5) // final value: 5 per slot
			}
		})
	}

	writersWG.Wait()
	close(stop)
	readersWG.Wait()

	// Every slot must hold exactly 5, so Query(0, n-1) == 5n.
	for i := 0; i < n; i++ {
		if v, ok := st.Value(i); !ok || v != 5 {
			t.Fatalf("expected Value(%d)=5 after concurrent writes, got (%d, %v)", i, v, ok)
		}
	}
	if s, ok := st.Query(0, n-1); !ok || s != 5*n {
		t.Fatalf("expected Query(0,%d)=(%d, true) after concurrent writes, got (%d, %v)", n-1, 5*n, s, ok)
	}
	checkInvariants(t, st)
}

// TestConcurrentCompound exercises Lock + Nl* read-modify-write
// sequences from multiple goroutines against one shared tree.  Each
// goroutine increments its own slot 100 times as a locked
// NlValue/NlUpdate pair — without the lock the pair would not be
// atomic, and the shared internal array would race under -race.
func TestConcurrentCompound(t *testing.T) {
	const writers = 8
	st := NewSegmentTree(make([]int, writers))

	var wg sync.WaitGroup
	for g := range writers {
		wg.Go(func() {
			for range 100 {
				st.Lock()
				v, ok := st.NlValue(g)
				if ok {
					st.NlUpdate(g, v+1)
				}
				st.Unlock()
			}
		})
	}
	wg.Wait()

	for g := 0; g < writers; g++ {
		if v, ok := st.Value(g); !ok || v != 100 {
			t.Fatalf("expected Value(%d)=100 after 100 locked increments, got (%d, %v)", g, v, ok)
		}
	}
	if s, _ := st.Query(0, writers-1); s != 100*writers {
		t.Errorf("expected Query(0,%d)=%d, got %d", writers-1, 100*writers, s)
	}
	checkInvariants(t, st)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the tree.
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

// slotValues returns the values at slots 0..Len()-1 in order.
func slotValues(st *SegmentTree[int]) []int {
	var got []int
	for i := 0; i < st.Len(); i++ {
		v, _ := st.Value(i)
		got = append(got, v)
	}
	return got
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, slot 0 first.
	st := NewSegmentTree([]int{3, 1, 2})
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("json.Marshal(st): %v", err)
	}
	if string(b) != "[3,1,2]" {
		t.Errorf("Expected [3,1,2], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	type pair struct {
		S string
	}
	pts := NewSegmentTreeFunc([]pair{{"a"}, {"b"}},
		func(a, b pair) pair { return pair{a.S + b.S} }, pair{})
	if b, err := json.Marshal(pts); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero SegmentTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *SegmentTree never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilST *SegmentTree[int]
	if b, err := nilST.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilST); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewSegmentTreeFunc([]upperString{"x", "y"},
		func(a, b upperString) upperString { return a + b }, upperString(""))
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewSegmentTreeFunc([]chan int{make(chan int)},
		func(a, b chan int) chan int { return a }, nil)
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a tree of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the value at slot 0.
	st := NewSegmentTree([]int{0, 0, 0})
	if err := json.Unmarshal([]byte("[3,1,2]"), st); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := fmt.Sprint(slotValues(st)); got != "[3 1 2]" {
		t.Errorf("Expected [3 1 2], got %v", got)
	}
	if s, ok := st.Query(0, 2); !ok || s != 6 {
		t.Errorf("Expected Query(0,2)=(6, true) after unmarshal, got (%d, %v)", s, ok)
	}
	checkInvariants(t, st)

	// A round trip rebuilds a structurally sound tree — resized to the
	// decoded length — and keeps the combine function (a min tree stays
	// a min tree).
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	src := NewSegmentTreeFunc([]int{3, 1, 4, 1, 5, 9, 2, 6}, min, math.MaxInt)
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewSegmentTreeFunc([]int{0, 0}, min, math.MaxInt) // a different size
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(slotValues(again)), "[3 1 4 1 5 9 2 6]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if again.Len() != 8 {
		t.Errorf("Expected the tree to be resized to 8 slots, got %d", again.Len())
	}
	if v, ok := again.Query(0, 7); !ok || v != 1 {
		t.Errorf("Expected the kept min combine to give Query(0,7)=(1, true), got (%d, %v)", v, ok)
	}
	checkInvariants(t, again)

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte("[7]"), st); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := st.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}
	if v, ok := st.Value(0); !ok || v != 7 {
		t.Errorf("Expected Value(0)=(7, true) after replacement, got (%d, %v)", v, ok)
	}

	// An empty array and null clear the tree to empty, keeping the
	// combine function so the tree stays usable.
	if err := json.Unmarshal([]byte("[]"), st); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !st.IsEmpty() {
		t.Errorf("Expected [] to clear the tree.")
	}
	if err := json.Unmarshal([]byte("[4,5]"), st); err != nil {
		t.Fatalf("json.Unmarshal after clearing: %v", err)
	}
	if got := fmt.Sprint(slotValues(st)); got != "[4 5]" {
		t.Errorf("Expected [4 5] after clearing and refilling, got %v", got)
	}
	if err := json.Unmarshal([]byte("null"), st); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !st.IsEmpty() {
		t.Errorf("Expected null to clear the tree.")
	}
	checkInvariants(t, st)

	// Element-level unmarshalers are honored.
	custom := NewSegmentTreeFunc([]upperString{"?"},
		func(a, b upperString) upperString { return a + b }, upperString(""))
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for i := 0; i < custom.Len(); i++ {
		v, _ := custom.Value(i)
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the tree untouched.
	keep := NewSegmentTree([]int{1, 2, 3})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[1,"a"]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(slotValues(keep)), "[1 2 3]"; got != want {
			t.Errorf("Tree changed after the error on %s: %s", badData, got)
		}
	}
	checkInvariants(t, keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value tree panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
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
		_ = zero.UnmarshalJSON([]byte("[1]"))
	}()

	var nilST *SegmentTree[int]
	if err := nilST.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil tree to be tolerated, got %v", err)
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
		_ = nilST.UnmarshalJSON([]byte("[1]"))
	}()
}
