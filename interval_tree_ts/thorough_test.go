package interval_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
)

// checkInvariants verifies every structural invariant of the tree —
// BST order by (Lo, Hi), the maxHi augmentation, the five red-black
// properties, parent pointers and the node count.  It reads internals
// WITHOUT the lock, so it is for single-goroutine tests only.
// Call it after any structural change in a test.
func checkInvariants[K any, V any](t *testing.T, tt *IntervalTree[K, V]) {
	t.Helper()
	if tt.Len() != tt.Length() {
		t.Errorf("Len %d != Length %d", tt.Len(), tt.Length())
	}
	if tt.root == nil {
		if tt.length != 0 {
			t.Fatalf("nil root but length %d", tt.length)
		}
		return
	}
	if tt.root.parent != nil {
		t.Errorf("root has a parent %v", tt.root.parent.iv)
	}
	if tt.root.red {
		t.Errorf("root is red")
	}

	count := 0
	var check func(n *itNode[K, V]) int // returns subtree black-height
	check = func(n *itNode[K, V]) int {
		if n == nil {
			return 1 // nil children are black
		}
		count++

		if n.left != nil && n.left.parent != n {
			t.Errorf("left child of %v has wrong parent", n.iv)
		}
		if n.right != nil && n.right.parent != n {
			t.Errorf("right child of %v has wrong parent", n.iv)
		}

		// BST order by (Lo, Hi) over the whole subtree.
		var walkLower, walkHigher func(x *itNode[K, V])
		walkLower = func(x *itNode[K, V]) {
			if x == nil {
				return
			}
			if tt.ivCmp(x.iv, n.iv) >= 0 {
				t.Errorf("BST order violated: %v not < %v (left subtree)", x.iv, n.iv)
			}
			walkLower(x.left)
			walkLower(x.right)
		}
		walkHigher = func(x *itNode[K, V]) {
			if x == nil {
				return
			}
			if tt.ivCmp(x.iv, n.iv) <= 0 {
				t.Errorf("BST order violated: %v not > %v (right subtree)", x.iv, n.iv)
			}
			walkHigher(x.left)
			walkHigher(x.right)
		}
		walkLower(n.left)
		walkHigher(n.right)

		// maxHi augmentation.
		want := n.iv.Hi
		if l := n.left; l != nil && tt.cmp(l.maxHi, want) > 0 {
			want = l.maxHi
		}
		if r := n.right; r != nil && tt.cmp(r.maxHi, want) > 0 {
			want = r.maxHi
		}
		if tt.cmp(n.maxHi, want) != 0 {
			t.Errorf("maxHi of %v is %v, expected %v", n.iv, n.maxHi, want)
		}

		if isRed(n) && (isRed(n.left) || isRed(n.right)) {
			t.Errorf("red node %v has a red child", n.iv)
		}

		lb := check(n.left)
		rb := check(n.right)
		if lb != rb {
			t.Errorf("black-height mismatch under %v: left %d, right %d", n.iv, lb, rb)
		}
		if n.red {
			return lb
		}
		return lb + 1
	}
	check(tt.root)

	if count != tt.length {
		t.Errorf("node count %d != Length %d", count, tt.length)
	}
}

// modelEntry is one (interval, value) pair of the reference model.
type modelEntry struct {
	iv  Interval[int]
	val int
}

func modelLess(a, b modelEntry) int {
	if a.iv.Lo != b.iv.Lo {
		return a.iv.Lo - b.iv.Lo
	}
	return a.iv.Hi - b.iv.Hi
}

// overlapsInt is the plain-int reference overlap predicate.
func overlapsInt(a, b Interval[int]) bool {
	return a.Lo <= b.Hi && b.Lo <= a.Hi
}

// modelOverlaps returns the model's overlapping pairs in ascending
// order — the O(n) scan oracle.
func modelOverlaps(model []modelEntry, probe Interval[int]) []modelEntry {
	var out []modelEntry
	for _, e := range model {
		if overlapsInt(e.iv, probe) {
			out = append(out, e)
		}
	}
	return out
}

// entriesEqual compares tree iterator output against the model.
func entriesEqual(t *testing.T, what string, got []entryInt, want []modelEntry) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries, expected %d\n got: %v\nwant: %v", what, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i].iv != want[i].iv || got[i].val != want[i].val {
			t.Errorf("%s: entry %d = %+v, expected %+v", what, i, got[i], want[i])
		}
	}
}

// TestTreeRandomizedModel is the plain package's model test, ported
// with the identical seed and step count so the twins walk the same
// operation stream.  Fixed seed — keep it deterministic.
func TestTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tt := NewIntervalTree[int, int]()
	var model []modelEntry

	randIV := func() Interval[int] {
		lo := rng.IntN(60)
		return Interval[int]{Lo: lo, Hi: lo + rng.IntN(12)}
	}

	for step := 0; step < 4000; step++ {
		switch rng.IntN(10) {
		case 0, 1, 2, 3: // Insert.
			probe := randIV()
			val := rng.IntN(1000)
			wantAdded := true
			for _, e := range model {
				if e.iv == probe {
					wantAdded = false
					break
				}
			}
			if got := tt.Insert(probe, val); got != wantAdded {
				t.Fatalf("step %d: Insert(%v) = %v, expected %v", step, probe, got, wantAdded)
			}
			replaced := false
			for i := range model {
				if model[i].iv == probe {
					model[i].val = val
					replaced = true
					break
				}
			}
			if !replaced {
				model = append(model, modelEntry{probe, val})
				slices.SortFunc(model, modelLess)
			}

		case 4, 5: // Delete.
			probe := randIV()
			want := false
			for _, e := range model {
				if e.iv == probe {
					want = true
					break
				}
			}
			if got := tt.Delete(probe); got != want {
				t.Fatalf("step %d: Delete(%v) = %v, expected %v", step, probe, got, want)
			}
			if want {
				for i := range model {
					if model[i].iv == probe {
						model = slices.Delete(model, i, i+1)
						break
					}
				}
			}

		case 6: // Search exact.
			probe := randIV()
			wantVal, wantFound := 0, false
			for _, e := range model {
				if e.iv == probe {
					wantVal, wantFound = e.val, true
					break
				}
			}
			gotVal, gotFound := tt.Search(probe)
			if gotFound != wantFound || (gotFound && gotVal != wantVal) {
				t.Fatalf("step %d: Search(%v) = (%d,%v), expected (%d,%v)", step, probe, gotVal, gotFound, wantVal, wantFound)
			}

		case 7: // AnyOverlap.
			probe := randIV()
			gotIV, _, ok := tt.AnyOverlap(probe)
			wantAny := len(modelOverlaps(model, probe)) > 0
			if ok != wantAny {
				t.Fatalf("step %d: AnyOverlap(%v) ok=%v, model says %v", step, probe, ok, wantAny)
			}
			if ok {
				inModel := slices.ContainsFunc(model, func(e modelEntry) bool { return e.iv == gotIV })
				if !inModel || !overlapsInt(gotIV, probe) {
					t.Fatalf("step %d: AnyOverlap(%v) returned %v — not a model overlap", step, probe, gotIV)
				}
			}

		case 8: // Overlaps.
			probe := randIV()
			entriesEqual(t, "Overlaps", collect(tt.Overlaps(probe)), modelOverlaps(model, probe))

		case 9: // Stab.
			p := rng.IntN(70)
			entriesEqual(t, "Stab", collect(tt.Stab(p)), modelOverlaps(model, Interval[int]{Lo: p, Hi: p}))
		}

		if step%37 == 0 {
			checkInvariants(t, tt)
			if tt.Len() != len(model) {
				t.Fatalf("step %d: Len %d != model %d", step, tt.Len(), len(model))
			}
			entriesEqual(t, "All", collect(tt.All()), model)
			back := collect(tt.Backward())
			slices.Reverse(back)
			entriesEqual(t, "Backward", back, model)
			if len(model) > 0 {
				if got, _, _ := tt.Min(); got != model[0].iv {
					t.Fatalf("step %d: Min %v != model %v", step, got, model[0].iv)
				}
				if got, _, _ := tt.Max(); got != model[len(model)-1].iv {
					t.Fatalf("step %d: Max %v != model %v", step, got, model[len(model)-1].iv)
				}
			}
		}
	}
	checkInvariants(t, tt)
}

// TestDepthBalance is the plain package's balance gate, ported.
func TestDepthBalance(t *testing.T) {
	const n = 100_000
	tt := NewIntervalTree[int, int]()
	for i := range n {
		tt.Insert(iv(i, i+3), i)
	}
	limit := 2 * math.Log2(float64(n)+1)
	if d := tt.Depth(); float64(d) > limit {
		t.Errorf("depth %d after %d sequential inserts exceeds 2·log₂(n+1) = %.1f", d, n, limit)
	}
}

// TestIntervalTreeConcurrent is the race trio's storm: concurrent
// writers, deleters and readers (Search, AnyOverlap, snapshot
// iterators) hammering one tree.  Afterwards every surviving interval
// is present exactly once and the structure passes the invariant
// checker (run single-goroutine, after the fact).
func TestIntervalTreeConcurrent(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	const writers, perWriter = 8, 500

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(w), 7))
			for i := 0; i < perWriter; i++ {
				lo := rng.IntN(2000)
				tt.Insert(Interval[int]{Lo: lo, Hi: lo + rng.IntN(20)}, w*1000+i)
			}
		}(w)
	}
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(w)+100, 7))
			for i := 0; i < perWriter; i++ {
				lo := rng.IntN(2000)
				tt.Delete(Interval[int]{Lo: lo, Hi: lo + rng.IntN(20)})
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_, _ = tt.Search(Interval[int]{Lo: i, Hi: i + 5})
				_, _, _ = tt.AnyOverlap(Interval[int]{Lo: i, Hi: i + 9})
				for range tt.Stab(i) {
				}
				for range tt.All() {
					break
				}
			}
		}()
	}
	wg.Wait()

	// Every surviving interval appears exactly once in All (snapshot)
	// and Search finds it; the invariant checker closes the loop.
	seen := make(map[Interval[int]]int)
	for _, e := range collect(tt.All()) {
		seen[e.iv]++
		if seen[e.iv] > 1 {
			t.Fatalf("interval %v yielded more than once", e.iv)
		}
		if _, ok := tt.Search(e.iv); !ok {
			t.Fatalf("surviving interval %v not found by Search", e.iv)
		}
	}
	if len(seen) != tt.Len() {
		t.Errorf("All yielded %d distinct intervals, Len says %d", len(seen), tt.Len())
	}
	checkInvariants(t, tt)
}

// TestConcurrentDelete is the delete-focused race: two goroutines
// deleting the same intervals must not double-remove.
func TestConcurrentDelete(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	const n = 2000
	for i := 0; i < n; i++ {
		tt.Insert(Interval[int]{Lo: i, Hi: i + 3}, i)
	}

	var wg sync.WaitGroup
	var removed sync.Map // Interval -> bool, only counted once
	countTrue := 0       // guarded by the mutex below
	var mu sync.Mutex
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				if tt.Delete(Interval[int]{Lo: i, Hi: i + 3}) {
					if _, loaded := removed.LoadOrStore(i, true); !loaded {
						mu.Lock()
						countTrue++
						mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()

	if countTrue != n {
		t.Errorf("%d intervals counted removed, expected %d", countTrue, n)
	}
	if !tt.IsEmpty() {
		t.Errorf("tree not empty after both goroutines deleted everything, len=%d", tt.Len())
	}
}
