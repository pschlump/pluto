package interval_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// checkInvariants verifies every structural invariant of the tree:
//
//  1. BST order by (Lo, Hi) — every left subtree key < node key < every
//     right subtree key, under the tree's own ivCmp.
//  2. maxHi augmentation — node.maxHi is the max of the subtree's His.
//  3. The five red-black properties: root black, red nodes have no red
//     children, nil children are black, every root-to-nil path carries
//     the same number of black nodes, and nodes are red or black.
//  4. Parent-pointer integrity.
//  5. Node count == Length.
//
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

		// Parent pointers.
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

		// No red-red parent/child pair.
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

// modelEntry is one (interval, value) pair of the reference model; the
// slice is kept sorted by (Lo, Hi) so it doubles as the expected All()
// output.
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

// TestTreeRandomizedModel is the property test: a few thousand random
// operations (insert with replacement, delete, search, AnyOverlap,
// Overlaps, Stab, Min/Max) against a sorted-slice model, cross-checking
// every return value and the full post-state each step.  Fixed seed —
// keep it deterministic.
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
		case 0, 1, 2, 3: // Insert (frequent — grows the tree).
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

		case 7: // AnyOverlap: returned hit must be in the model and
			// overlap; a miss requires the model to have no overlap.
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

		case 8: // Overlaps: exact ascending set equality with the scan.
			probe := randIV()
			entriesEqual(t, "Overlaps", collect(tt.Overlaps(probe)), modelOverlaps(model, probe))

		case 9: // Stab at a point == Overlaps over [p, p], checked both
			// against the model and against the tree's own Overlaps.
			p := rng.IntN(70)
			got := collect(tt.Stab(p))
			entriesEqual(t, "Stab", got, modelOverlaps(model, Interval[int]{Lo: p, Hi: p}))
			entriesEqual(t, "Stab vs Overlaps", collect(tt.Overlaps(Interval[int]{Lo: p, Hi: p})), modelOverlaps(model, Interval[int]{Lo: p, Hi: p}))
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

// TestDepthBalance verifies the balancing guarantee: sequential
// inserts (the adversarial shape for a plain BST) and sequential
// deletes must leave the depth at O(log₂ n) — within the textbook
// 2·log₂(n+1) bound.
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

	// Deleting the first half ascending must keep it balanced.
	for i := range n / 2 {
		if !tt.Delete(iv(i, i+3)) {
			t.Fatalf("Delete(%d) returned false", i)
		}
	}
	if d := tt.Depth(); float64(d) > 2*math.Log2(float64(n)+1) {
		t.Errorf("depth %d after deleting the first half exceeds 2·log₂(n+1)", d)
	}
	if tt.Len() != n-n/2 {
		t.Errorf("Len %d, expected %d", tt.Len(), n-n/2)
	}
}

// TestMaxHiAfterEveryMutation hammers insert/delete interleavings at
// small sizes with the invariant checker after every single operation —
// the maxHi maintenance is what this catches drifting.
func TestMaxHiAfterEveryMutation(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 42))
	for round := 0; round < 60; round++ {
		tt := NewIntervalTree[int, int]()
		for op := 0; op < 40; op++ {
			lo := rng.IntN(12)
			probe := iv(lo, lo+rng.IntN(3))
			if op%3 == 2 && tt.Len() > 0 {
				tt.Delete(probe)
			} else {
				tt.Insert(probe, op)
			}
			checkInvariants(t, tt)
		}
	}
}

// TestOverlapsSortedOutput verifies the query iterators yield matches
// in ascending (Lo, Hi) order even under a scrambled insertion order.
func TestOverlapsSortedOutput(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tt := NewIntervalTree[int, int]()
	for range 500 {
		lo := rng.IntN(100)
		tt.Insert(iv(lo, lo+rng.IntN(30)), 0)
	}
	probe := iv(40, 60)
	var got []Interval[int]
	for k := range tt.Overlaps(probe) {
		got = append(got, k)
	}
	if !slices.IsSortedFunc(got, func(a, b Interval[int]) int {
		if a.Lo != b.Lo {
			return a.Lo - b.Lo
		}
		return a.Hi - b.Hi
	}) {
		t.Errorf("Overlaps output not ascending by (Lo, Hi): %v", got)
	}
	if len(got) == 0 {
		t.Errorf("no overlaps found for %v in a 500-interval tree", probe)
	}
}

func BenchmarkInsert(b *testing.B) {
	base := make([]Interval[int], 1000)
	rng := uint32(42)
	for i := range base {
		lo := int(rng>>16) % 10_000
		if lo < 0 {
			lo = -lo
		}
		base[i] = iv(lo, lo+int(rng%97))
		rng = rng*1664525 + 1013904223
	}
	b.ResetTimer()
	for range b.N {
		tt := NewIntervalTree[int, int]()
		for _, e := range base {
			tt.Insert(e, 0)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	tt := NewIntervalTree[int, int]()
	rng := uint32(42)
	probes := make([]Interval[int], 0, 1000)
	for i := 0; i < 1000; i++ {
		lo := int(rng>>16) % 10_000
		if lo < 0 {
			lo = -lo
		}
		e := iv(lo, lo+int(rng%97))
		tt.Insert(e, i)
		probes = append(probes, e)
		rng = rng*1664525 + 1013904223
	}
	b.ResetTimer()
	for range b.N {
		for _, p := range probes {
			tt.Search(p)
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	type ivPair = Interval[int]
	base := make([]ivPair, 1000)
	rng := uint32(42)
	for i := range base {
		lo := int(rng>>16) % 10_000
		if lo < 0 {
			lo = -lo
		}
		base[i] = iv(lo, lo+int(rng%97))
		rng = rng*1664525 + 1013904223
	}
	b.ResetTimer()
	for range b.N {
		tt := NewIntervalTree[int, int]()
		for _, e := range base {
			tt.Insert(e, 0)
		}
		for _, e := range base {
			tt.Delete(e)
		}
	}
}

func BenchmarkAnyOverlap(b *testing.B) {
	tt := NewIntervalTree[int, int]()
	rng := uint32(42)
	for i := 0; i < 1000; i++ {
		lo := int(rng>>16) % 10_000
		if lo < 0 {
			lo = -lo
		}
		tt.Insert(iv(lo, lo+int(rng%97)), i)
		rng = rng*1664525 + 1013904223
	}
	b.ResetTimer()
	for range b.N {
		_, _, _ = tt.AnyOverlap(iv(4_000, 4_050))
	}
}

func BenchmarkStab(b *testing.B) {
	tt := NewIntervalTree[int, int]()
	rng := uint32(42)
	for i := 0; i < 1000; i++ {
		lo := int(rng>>16) % 10_000
		if lo < 0 {
			lo = -lo
		}
		tt.Insert(iv(lo, lo+int(rng%97)), i)
		rng = rng*1664525 + 1013904223
	}
	b.ResetTimer()
	for range b.N {
		for e := range tt.Stab(4_020) {
			_ = e
		}
	}
}
