package quicklist

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: a fixed-seed randomized oracle that cross-checks a
// QuickList against a plain []int model after every batch of a random op
// mix (push/pop/insert/delete/set/trim/range), plus segment-invariant
// checks after every single op — no empty segments, capacity bounds
// respected, no adjacent pair below the merge threshold, link integrity,
// and the compression depth windows when compression is enabled.

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// checkInvariants verifies the internal structure of q: link integrity,
// element accounting, no empty or over-capacity segments, no adjacent
// pair that should have merged, and the compression depth windows.
func checkInvariants[T any](t *testing.T, q *QuickList[T]) {
	t.Helper()
	if q.length == 0 {
		if q.head != nil || q.tail != nil {
			t.Fatal("empty list still links segments")
		}
		return
	}
	if q.head == nil || q.tail == nil || q.head.prev != nil || q.tail.next != nil {
		t.Fatal("broken head/tail links")
	}
	cap_ := q.segCap()
	sum, m := 0, 0
	var prev *segment[T]
	for s := q.head; s != nil; s = s.next {
		if s.prev != prev {
			t.Fatalf("segment %d: broken prev link", m)
		}
		if s.n <= 0 {
			t.Fatalf("segment %d: empty or negative count %d", m, s.n)
		}
		if s.n > cap_ {
			t.Fatalf("segment %d: %d elements over capacity %d", m, s.n, cap_)
		}
		if s.packed == nil {
			if s.start < 0 || s.start+s.n > len(s.data) {
				t.Fatalf("segment %d: live region [%d:%d] outside backing %d",
					m, s.start, s.start+s.n, len(s.data))
			}
		} else if q.codec == nil {
			t.Fatalf("segment %d: packed with compression disabled", m)
		}
		if prev != nil && prev.n+s.n <= q.mergeThreshold() {
			t.Fatalf("segments %d,%d: combined %d at/below merge threshold %d",
				m-1, m, prev.n+s.n, q.mergeThreshold())
		}
		sum += s.n
		m++
		prev = s
	}
	if prev != q.tail {
		t.Fatal("tail link does not end the walk")
	}
	if sum != q.length {
		t.Fatalf("segment counts sum to %d, length is %d", sum, q.length)
	}
	checkDepthWindow(t, q)
}

// checkModel verifies q's observable state against the model slice.
func checkModel(t *testing.T, q *QuickList[int], model []int) {
	t.Helper()
	if q.Len() != len(model) {
		t.Fatalf("Len: expected %d, got %d", len(model), q.Len())
	}
	if got := collect(q.All()); !slices.Equal(got, model) {
		t.Fatalf("All mismatch:\n got %v\nwant %v", got, model)
	}
	if got := collect(q.Backward()); !slices.Equal(got, reversed(model)) {
		t.Fatal("Backward mismatch")
	}
	for _, i := range []int{0, 1, len(model) / 2, len(model) - 2, len(model) - 1} {
		if i < 0 || i >= len(model) {
			continue
		}
		if v, ok := q.At(i); !ok || v != model[i] {
			t.Fatalf("At(%d): expected %d, got %d,%v", i, model[i], v, ok)
		}
		if v, ok := q.At(i - len(model)); !ok || v != model[i] {
			t.Fatalf("At(%d): expected %d, got %d,%v", i-len(model), model[i], v, ok)
		}
	}
	if len(model) > 0 {
		if v, _ := q.PeekHead(); v != model[0] {
			t.Fatalf("PeekHead: expected %d, got %d", model[0], v)
		}
		if v, _ := q.PeekTail(); v != model[len(model)-1] {
			t.Fatalf("PeekTail: expected %d, got %d", model[len(model)-1], v)
		}
	}
}

func reversed(model []int) []int {
	out := make([]int, len(model))
	for i, v := range model {
		out[len(model)-1-i] = v
	}
	return out
}

// modelInsert inserts v before index i in a slice.
func modelInsert(model []int, i int, v int) []int {
	return slices.Insert(model, i, v)
}

// modelTrim keeps the Redis-normalized inclusive range [start, stop].
func modelTrim(model []int, start, stop int) []int {
	n := len(model)
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n || stop < 0 {
		return model[:0]
	}
	return model[start : stop+1]
}

// TestRandomizedModel runs 100k random operations against a []int model
// with a small fill target to stress segment splits and merges.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	q := NewQuickList(WithSegmentFill[int](8))
	var model []int

	const ops = 100_000
	for k := 0; k < ops; k++ {
		n := len(model)
		switch rng.IntN(12) {
		case 0, 1, 2: // PushTail
			v := rng.IntN(1000)
			q.PushTail(v)
			model = append(model, v)
		case 3, 4: // PushHead
			v := rng.IntN(1000)
			q.PushHead(v)
			model = append([]int{v}, model...)
		case 5: // PopHead
			if _, ok := q.PopHead(); ok != (n > 0) {
				t.Fatalf("op %d: PopHead ok=%v with %d elements", k, ok, n)
			}
			if n > 0 {
				model = model[1:]
			}
		case 6: // PopTail
			if _, ok := q.PopTail(); ok != (n > 0) {
				t.Fatalf("op %d: PopTail ok=%v with %d elements", k, ok, n)
			}
			if n > 0 {
				model = model[:n-1]
			}
		case 7: // InsertBefore / InsertAfter at a random valid index
			i := rng.IntN(n + 1)
			v := rng.IntN(1000)
			if i == n {
				if ok := q.InsertAfter(n-1, v); ok != (n > 0) {
					t.Fatalf("op %d: InsertAfter(%d) ok=%v", k, n-1, ok)
				}
				if n > 0 {
					model = append(model, v)
				}
			} else {
				if !q.InsertBefore(i, v) {
					t.Fatalf("op %d: InsertBefore(%d) failed", k, i)
				}
				model = modelInsert(model, i, v)
			}
		case 8: // Delete — probes negative and out-of-range indexes too
			i := rng.IntN(2*n+3) - (n + 1) // -(n+1)..n+1
			abs, valid := modelNorm(n, i)
			if ok := q.Delete(i); ok != valid {
				t.Fatalf("op %d: Delete(%d) ok=%v with %d elements", k, i, ok, n)
			}
			if valid {
				model = slices.Delete(model, abs, abs+1)
			}
		case 9: // Set
			i := rng.IntN(2*n+3) - (n + 1)
			v := rng.IntN(1000)
			abs, valid := modelNorm(n, i)
			if ok := q.Set(i, v); ok != valid {
				t.Fatalf("op %d: Set(%d) ok=%v", k, i, ok)
			}
			if valid {
				model[abs] = v
			}
		case 10: // DeleteRange with raw (unclamped) bounds
			a, b := rng.IntN(n+4)-2, rng.IntN(n+4)-2
			if a > b {
				a, b = b, a
			}
			lo, hi, ok := normRangeModel(n, a, b)
			want := 0
			if ok {
				want = hi - lo + 1
			}
			if got := q.DeleteRange(a, b); got != want {
				t.Fatalf("op %d: DeleteRange(%d,%d) = %d, want %d", k, a, b, got, want)
			}
			if ok {
				model = modelDeleteRange(model, lo, hi)
			}
		case 11: // Trim with raw bounds
			a, b := rng.IntN(n+4)-2, rng.IntN(n+4)-2
			if a > b {
				a, b = b, a
			}
			q.Trim(a, b)
			model = modelTrim(model, a, b)
		}
		checkInvariants(t, q)
		if k%997 == 0 {
			checkModel(t, q, model)
		}
	}
	checkModel(t, q, model)
}

// modelNorm mirrors the package's norm: negative indexes count back from
// the tail; it returns the absolute index and whether i was in range.
func modelNorm(n, i int) (int, bool) {
	if i < 0 {
		i += n
	}
	if i < 0 || i >= n {
		return 0, false
	}
	return i, true
}

// normRangeModel mirrors the package's normRange for a model length.
func normRangeModel(n, start, stop int) (int, int, bool) {
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n || stop < 0 {
		return 0, 0, false
	}
	return start, stop, true
}

// modelDeleteRange applies the normalized inclusive removal to a model.
func modelDeleteRange(model []int, lo, hi int) []int {
	if lo > hi {
		return model
	}
	return slices.Delete(model, lo, hi+1)
}

// TestRandomizedModelCompressed runs the same oracle shape against a
// compressed string list, verifying transparency and the depth windows.
func TestRandomizedModelCompressed(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 1))
	q := NewQuickList(
		WithSegmentFill[string](8),
		WithCompression[string](LZWCodec(), 2, EncodeStringSegment, DecodeStringSegment))
	var model []string

	const ops = 20_000
	for k := 0; k < ops; k++ {
		n := len(model)
		switch rng.IntN(8) {
		case 0, 1, 2:
			v := fmt.Sprintf("v%06d", rng.IntN(10000))
			q.PushTail(v)
			model = append(model, v)
		case 3:
			v := fmt.Sprintf("v%06d", rng.IntN(10000))
			q.PushHead(v)
			model = append([]string{v}, model...)
		case 4:
			if _, ok := q.PopHead(); ok != (n > 0) {
				t.Fatalf("op %d: PopHead ok=%v", k, ok)
			}
			if n > 0 {
				model = model[1:]
			}
		case 5:
			if _, ok := q.PopTail(); ok != (n > 0) {
				t.Fatalf("op %d: PopTail ok=%v", k, ok)
			}
			if n > 0 {
				model = model[:n-1]
			}
		case 6:
			if n == 0 {
				continue
			}
			i := rng.IntN(n)
			v := fmt.Sprintf("v%06d", rng.IntN(10000))
			if !q.InsertBefore(i, v) {
				t.Fatalf("op %d: InsertBefore(%d) failed", k, i)
			}
			model = slices.Insert(model, i, v)
		case 7:
			i := rng.IntN(2*n+3) - (n + 1)
			abs, valid := modelNorm(n, i)
			if ok := q.Delete(i); ok != valid {
				t.Fatalf("op %d: Delete(%d) ok=%v", k, i, ok)
			}
			if valid {
				model = slices.Delete(model, abs, abs+1)
			}
		}
		checkInvariants(t, q)
		if k%499 == 0 {
			if q.Len() != len(model) {
				t.Fatalf("op %d: Len %d, want %d", k, q.Len(), len(model))
			}
			if got := collect(q.All()); !slices.Equal(got, model) {
				t.Fatalf("op %d: content mismatch through compression", k)
			}
		}
	}
	if got := collect(q.All()); !slices.Equal(got, model) {
		t.Fatal("final content mismatch through compression")
	}
}

// TestRepeatedInsertDeleteAtOnePosition hammers a single index to prove
// the split/merge policy does not degenerate into many tiny segments.
func TestRepeatedInsertDeleteAtOnePosition(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 0))
	q := build(64, WithSegmentFill[int](16))
	for k := 0; k < 10_000; k++ {
		i := q.Len() / 2
		if rng.IntN(2) == 0 {
			if !q.InsertBefore(i, k) {
				t.Fatalf("op %d: InsertBefore(%d) failed", k, i)
			}
		} else {
			if !q.Delete(i) {
				t.Fatalf("op %d: Delete(%d) failed", k, i)
			}
		}
		checkInvariants(t, q)
	}
	m := 0
	for s := q.head; s != nil; s = s.next {
		m++
	}
	// Ideal is Len/fill segments; the merge threshold (fill/2) allows at
	// most ~2x that.  Without merging this workload would leave ~1
	// segment per element — 3x ideal proves no such degeneration.
	ideal := (q.Len() + 15) / 16
	if m > 3*ideal+2 {
		t.Fatalf("fragmented into %d segments for %d elements (ideal %d)", m, q.Len(), ideal)
	}
}
