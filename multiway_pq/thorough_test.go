package multiway_pq

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/pschlump/pluto/heap"
)

// -------------------------------------------------------------------------------------------------------
// Randomized model test: every branching factor cross-checked against a
// multiset model AND against pluto's binary heap on the identical op
// stream (a tie-safe value-for-value oracle: both heaps must pop the
// minimum VALUE of the same multiset even when the minimum is tied).
// -------------------------------------------------------------------------------------------------------

// TestDHeapRandomizedModel runs thousands of mixed operations (Push,
// Pop, Peek, Delete at a random index, Fix, AppendHeap + rebuild,
// Truncate) at a fixed seed against a map multiset model and a mirror
// pluto/heap, verifying the heap invariant and the multiset
// periodically.
func TestDHeapRandomizedModel(t *testing.T) {
	for _, d := range []int{2, 3, 4, 8, 16} {
		t.Run(fmt.Sprintf("d=%d", d), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(42, 7))
			const ops = 4000
			const keySpace = 50

			dh := NewDHeap[int](d)
			mirror := heap.NewHeap[int]()
			model := map[int]int{} // value -> count

			check := func(step int) {
				t.Helper()
				checkHeapInvariant(t, dh, Compare[int])
				if dh.Len() != countOf(model) {
					t.Fatalf("step %d: Len %d, model has %d", step, dh.Len(), countOf(model))
				}
				seen := map[int]int{}
				for v := range dh.All() {
					seen[v]++
				}
				if !reflect.DeepEqual(seen, model) {
					t.Fatalf("step %d: heap multiset %v != model %v", step, seen, model)
				}
			}

			for step := range ops {
				v := rng.IntN(keySpace)
				switch rng.IntN(8) {
				case 0, 1, 2: // Push (mirrored)
					dh.Push(v)
					mirror.Push(v)
					model[v]++
				case 3: // Pop (mirrored, value-for-value)
					got, found := dh.Pop()
					want, mFound := mirror.Pop()
					if len(model) == 0 {
						if found || mFound {
							t.Fatalf("step %d: Pop on empty reported true", step)
						}
					} else {
						if !found || !mFound || got != want {
							t.Fatalf("step %d: Pop = (%v, %v), binary-heap mirror popped %v", step, got, found, want)
						}
						model[got]--
						if model[got] == 0 {
							delete(model, got)
						}
					}
				case 4: // Peek (mirrored)
					got, found := dh.Peek()
					want, mFound := mirror.Peek()
					if (found != mFound) || (found && got != want) {
						t.Fatalf("step %d: Peek = (%v, %v), mirror (%v, %v)", step, got, found, want, mFound)
					}
				case 5: // Delete at a random index (mirror removes the same VALUE)
					if dh.Len() > 0 {
						idx := rng.IntN(dh.Len())
						old, _ := dh.GetValue(idx)
						got, found := dh.Delete(idx)
						if !found || got != old {
							t.Fatalf("step %d: Delete(%d) = (%v, %v), expected %v", step, idx, got, found, old)
						}
						_, mPos, mFound := mirror.Search(old)
						if !mFound {
							t.Fatalf("step %d: mirror does not contain %v that the d-heap deleted", step, old)
						}
						mirror.Delete(mPos)
						model[old]--
						if model[old] == 0 {
							delete(model, old)
						}
					}
				case 6: // Fix: replace a random index with a new value (model only —
					// the mirror's index space differs, so the mirror tracks via Search)
					if dh.Len() > 0 {
						idx := rng.IntN(dh.Len())
						old, _ := dh.GetValue(idx)
						if !dh.Fix(idx, v) {
							t.Fatalf("step %d: Fix(%d) reported false on a valid index", step, idx)
						}
						if _, mPos, mFound := mirror.Search(old); mFound {
							mirror.Fix(mPos, v)
						} else {
							t.Fatalf("step %d: mirror does not contain %v that the d-heap replaced", step, old)
						}
						model[old]--
						if model[old] == 0 {
							delete(model, old)
						}
						model[v]++
					}
				case 7: // AppendHeap a batch + rebuild (mirrored as Pushes)
					if rng.IntN(4) == 0 {
						var batch []int
						for range 1 + rng.IntN(8) {
							x := rng.IntN(keySpace)
							batch = append(batch, x)
							model[x]++
						}
						dh.AppendHeap(batch)
						for i := (dh.Len() - 2) / dh.D(); i >= 0; i-- {
							dh.Heapify(dh.Len(), i)
						}
						for _, x := range batch {
							mirror.Push(x)
						}
					} else if dh.Len() > 0 && rng.IntN(20) == 0 {
						// Occasional Truncate keeps lengths from saturating.
						dh.Truncate()
						mirror.Truncate()
						clear(model)
					}
				}
				if step%37 == 0 {
					check(step)
				}
			}
			check(ops)

			// Final drain: the d-heap and the binary heap must produce the
			// identical sorted sequence.
			var gotSorted, mirrorSorted []int
			for {
				v, found := dh.Pop()
				if !found {
					break
				}
				gotSorted = append(gotSorted, v)
			}
			for {
				v, found := mirror.Pop()
				if !found {
					break
				}
				mirrorSorted = append(mirrorSorted, v)
			}
			var expect []int
			for val, cnt := range model {
				for range cnt {
					expect = append(expect, val)
				}
			}
			slices.Sort(expect)
			if !reflect.DeepEqual(gotSorted, expect) || !reflect.DeepEqual(mirrorSorted, expect) {
				t.Fatalf("final drain mismatch: d-heap %d items, mirror %d items, model %d items",
					len(gotSorted), len(mirrorSorted), len(expect))
			}
		})
	}
}

func countOf(m map[int]int) int {
	n := 0
	for _, c := range m {
		n += c
	}
	return n
}

// TestHeapifyEquivalence verifies that bulk-loading via AppendHeap +
// the bottom-up Heapify rebuild produces the same sorted drain as
// pushing the same data one element at a time (and that the rebuild is
// O(n): it must handle a large batch).
func TestHeapifyEquivalence(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for _, d := range []int{2, 3, 4, 8, 16} {
		var data []int
		for range 5000 {
			data = append(data, rng.IntN(2000))
		}

		bulk := NewDHeap[int](d)
		bulk.AppendHeap(data)
		for i := (bulk.Len() - 2) / bulk.D(); i >= 0; i-- {
			bulk.Heapify(bulk.Len(), i)
		}
		checkHeapInvariant(t, bulk, Compare[int])

		one := NewDHeap[int](d)
		for _, v := range data {
			one.Push(v)
		}

		var bulkDrain, oneDrain []int
		for {
			v, found := bulk.Pop()
			if !found {
				break
			}
			bulkDrain = append(bulkDrain, v)
		}
		for {
			v, found := one.Pop()
			if !found {
				break
			}
			oneDrain = append(oneDrain, v)
		}
		if !slices.IsSorted(bulkDrain) {
			t.Fatalf("d=%d: bulk drain not sorted", d)
		}
		if !reflect.DeepEqual(bulkDrain, oneDrain) {
			t.Fatalf("d=%d: bulk-load drain differs from one-at-a-time drain", d)
		}
	}
}

// TestDeleteEveryIndex deletes through every index position.
func TestDeleteEveryIndex(t *testing.T) {
	for _, d := range []int{2, 4, 8} {
		dh := NewDHeap[int](d)
		for i := range 20 {
			dh.Push(i * 3 % 7) // duplicates included
		}
		for dh.Len() > 0 {
			for idx := dh.Len() - 1; idx >= 0; idx -= 3 {
				if _, found := dh.Delete(idx); !found {
					t.Fatalf("d=%d: Delete(%d) on len %d reported false", d, idx, dh.Len())
				}
				checkHeapInvariant(t, dh, Compare[int])
			}
		}
	}
}

// TestFixEveryDirection raises and sinks the root repeatedly.
func TestFixEveryDirection(t *testing.T) {
	dh := NewDHeap[int](4)
	for i := range 20 {
		dh.Push(i)
	}
	for i := range 10 {
		if !dh.Fix(0, 100+i) { // raise the root; it must sink
			t.Fatalf("Fix(0) reported false")
		}
		checkHeapInvariant(t, dh, Compare[int])
	}
	for i := range 10 {
		if !dh.Fix(0, -1-i) { // sink the root; it stays
			t.Fatalf("Fix(0) reported false")
		}
		checkHeapInvariant(t, dh, Compare[int])
	}
	if v, found := dh.Peek(); !found || v != -10 {
		t.Errorf("Peek = (%v, %v), expected -10", v, found)
	}
}

// TestLargeSequentialDrain pushes 100k sequential values and drains
// them — the sift paths at their longest.
func TestLargeSequentialDrain(t *testing.T) {
	const n = 100_000
	dh := NewDHeap[int](8)
	for i := range n {
		dh.Push(n - 1 - i) // descending input exercises up() maximally
	}
	if v, found := dh.Peek(); !found || v != 0 {
		t.Fatalf("Peek = (%v, %v), expected 0", v, found)
	}
	for i := range n {
		v, found := dh.Pop()
		if !found || v != i {
			t.Fatalf("Pop %d = (%v, %v)", i, v, found)
		}
	}
	if !dh.IsEmpty() || dh.data != nil {
		t.Errorf("Expected a fully drained heap with a nil backing array.")
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a multiset reference model at fixed seed: a round trip must
// preserve the multiset exactly and produce a valid heap.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const ops = 500
	const keySpace = 100

	dh := NewDHeap[int](4)
	model := map[int]int{} // value -> count

	for step := range ops {
		switch rng.IntN(3) {
		case 0, 1: // Push
			v := rng.IntN(keySpace)
			dh.Push(v)
			model[v]++
		case 2: // Pop
			got, found := dh.Pop()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: Pop on empty reported true", step)
				}
			} else {
				min := minKey(model)
				if !found || got != min {
					t.Fatalf("step %d: Pop = (%v, %v), model min %d", step, got, found, min)
				}
				model[min]--
				if model[min] == 0 {
					delete(model, min)
				}
			}
		}

		gotJSON, err := json.Marshal(dh)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if len(model) == 0 && string(gotJSON) != "[]" {
			t.Fatalf("step %d: an empty heap must marshal as [], got %s", step, gotJSON)
		}
		fresh := NewDHeap[int](4)
		if err := json.Unmarshal(gotJSON, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		checkHeapInvariant(t, fresh, Compare[int])
		seen := map[int]int{}
		var drained []int
		for {
			v, found := fresh.Pop()
			if !found {
				break
			}
			seen[v]++
			drained = append(drained, v)
		}
		if !reflect.DeepEqual(seen, model) {
			t.Fatalf("step %d: round-tripped multiset %v != model %v", step, seen, model)
		}
		if !sort.IntsAreSorted(drained) {
			t.Fatalf("step %d: round-tripped heap drained out of order: %v", step, drained)
		}
	}
}

func minKey(m map[int]int) int {
	min := -1
	for k := range m {
		if min == -1 || k < min {
			min = k
		}
	}
	return min
}

// TestCustomKeyStruct verifies ordering by a struct field with satellite
// data the comparator ignores (the TestTreeNode shape).
func TestCustomKeyStruct(t *testing.T) {
	type task struct {
		priority int
		name     string
	}
	dh := NewDHeapFunc(4, func(a, b task) int { return Compare(a.priority, b.priority) })
	for _, tk := range []task{
		{3, "c"}, {1, "a"}, {2, "b"}, {1, "a-prime"},
	} {
		dh.Push(tk)
	}
	checkHeapInvariant(t, dh, func(a, b task) int { return Compare(a.priority, b.priority) })

	// Both priority-1 tasks come out before the rest; their relative order
	// is unspecified (the drain is by priority, not stable).
	var prios []int
	for {
		v, found := dh.Pop()
		if !found {
			break
		}
		prios = append(prios, v.priority)
	}
	if expect := []int{1, 1, 2, 3}; !reflect.DeepEqual(prios, expect) {
		t.Errorf("Drain priorities got %v, expected %v", prios, expect)
	}
}
