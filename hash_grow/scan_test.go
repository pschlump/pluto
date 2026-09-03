package hash_grow

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Scan tests: frozen-table coverage at several batch sizes, coverage
// across growth mid-iteration, edge cases (empty table, default count,
// foreign cursors, Truncate), and a termination proof under adversarial
// insert/delete churn.

import (
	"math/rand/v2"
	"testing"
)

// scanAll drains a full iteration into a set, failing if it does not
// terminate within maxSteps batches.
func scanAll[T comparable](t *testing.T, tt *HashTab[T], count int, maxSteps int) map[T]int {
	t.Helper()
	return scanFrom(t, tt, 0, count, maxSteps)
}

// scanFrom drains an iteration starting at an arbitrary cursor.
func scanFrom[T comparable](t *testing.T, tt *HashTab[T], cursor uint64, count int, maxSteps int) map[T]int {
	t.Helper()
	got := make(map[T]int)
	for steps := 0; ; steps++ {
		if steps > maxSteps {
			t.Fatalf("Scan did not terminate within %d batches", maxSteps)
		}
		items, next := tt.Scan(cursor, count)
		for _, v := range items {
			got[v]++
		}
		if next == 0 {
			return got
		}
		if next == cursor {
			t.Fatal("Scan returned the same cursor twice without finishing")
		}
		cursor = next
	}
}

func TestScanFrozenCoverage(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	tt := NewHashTab[int](64, 0.5)
	want := make(map[int]bool)
	for len(want) < 500 {
		v := rng.IntN(1_000_000)
		if !want[v] {
			want[v] = true
			tt.Insert(v)
		}
	}
	for _, count := range []int{1, 7, 1000} {
		got := scanAll(t, tt, count, 100_000)
		if len(got) != len(want) {
			t.Fatalf("count=%d: scanned %d distinct elements, table holds %d", count, len(got), len(want))
		}
		for v := range want {
			if got[v] == 0 {
				t.Fatalf("count=%d: element %d never returned", count, v)
			}
		}
	}
}

func TestScanResizeDuringScan(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 1))
	tt := NewHashTab[int](64, 0.5)
	base := make(map[int]bool)
	for len(base) < 60 {
		v := rng.IntN(1_000_000)
		if !base[v] {
			base[v] = true
			tt.Insert(v)
		}
	}
	// First batch against the size-128 table, then force several
	// doublings mid-iteration.
	got := make(map[int]bool)
	items, cursor := tt.Scan(0, 3)
	for _, v := range items {
		got[v] = true
	}
	for i := 0; i < 500; i++ {
		tt.Insert(1_000_000 + i)
	}
	for steps := 0; cursor != 0; steps++ {
		if steps > 100_000 {
			t.Fatal("Scan did not terminate after growth")
		}
		items, cursor = tt.Scan(cursor, 3)
		for _, v := range items {
			got[v] = true
		}
	}
	for v := range base {
		if !got[v] {
			t.Fatalf("pre-growth element %d lost across the resize", v)
		}
	}
}

func TestScanEdgeCases(t *testing.T) {
	// Empty table scans as (nil, 0).
	tt := NewHashTab[int](64, 0)
	if items, next := tt.Scan(0, 10); items != nil || next != 0 {
		t.Fatalf("empty table: got (%v, %d)", items, next)
	}
	// Nil table tolerates Scan.
	var niltt *HashTab[int]
	if items, next := niltt.Scan(0, 10); items != nil || next != 0 {
		t.Fatalf("nil table: got (%v, %d)", items, next)
	}
	tt.Insert(1)
	tt.Insert(2)
	// count <= 0 selects the default and still completes.
	if got := scanAll(t, tt, 0, 1000); len(got) != 2 {
		t.Fatalf("default count: scanned %d distinct, want 2", len(got))
	}
	if got := scanAll(t, tt, -5, 1000); len(got) != 2 {
		t.Fatalf("negative count: scanned %d distinct, want 2", len(got))
	}
	// Foreign cursors: no panic, no corruption.  A stale generation
	// restarts the walk (so it covers the table); an out-of-range
	// position with the current generation simply ends the iteration.
	if got := scanFrom(t, tt, 1<<40, 10, 1000); len(got) != 2 {
		t.Fatalf("restart after stale generation: %d distinct, want 2", len(got))
	}
	if _, next := tt.Scan(encodeScanCursor(tt.generation, 1_000_000), 10); next != 0 {
		t.Fatal("out-of-range cursor did not end the iteration")
	}
	tt.Scan(0xFFFFFFFFFFFFFFFF, 1)
	tt.Scan(42, 1)
	if tt.Len() != 2 {
		t.Fatalf("foreign cursors corrupted the table: Len %d", tt.Len())
	}
	// A full iteration still works afterwards.
	if got := scanAll(t, tt, 1, 1000); len(got) != 2 {
		t.Fatalf("post-foreign scan: %d distinct, want 2", len(got))
	}
}

func TestScanConcurrentTruncate(t *testing.T) {
	tt := NewHashTab[int](64, 0.5)
	for i := 0; i < 100; i++ {
		tt.Insert(i)
	}
	_, cursor := tt.Scan(0, 5)
	tt.Truncate()
	// The stale cursor restarts against the emptied table and finishes.
	items, next := tt.Scan(cursor, 5)
	if items != nil || next != 0 {
		t.Fatalf("Scan across Truncate: got (%v, %d)", items, next)
	}
	// And a refilled table scans normally again.
	for i := 0; i < 10; i++ {
		tt.Insert(i)
	}
	if got := scanAll(t, tt, 3, 1000); len(got) != 10 {
		t.Fatalf("refilled table: %d distinct, want 10", len(got))
	}
}

// TestScanTerminationUnderChurn interleaves Scan batches with
// adversarial inserts and deletes (including table doublings) and proves
// the iteration completes within a bounded number of batches: resizes
// are the only source of restarts, and each pass costs at most
// ceil(size/count) batches.
func TestScanTerminationUnderChurn(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 2))
	tt := NewHashTab[int](64, 0.5)
	live := make(map[int]bool)
	nextKey := 0
	for i := 0; i < 100; i++ {
		tt.Insert(nextKey)
		live[nextKey] = true
		nextKey++
	}
	const count = 7
	cursor := uint64(0)
	batches := 0
	got := make(map[int]bool)
	// 4000 churn ops interleaved one-per-batch with the iteration; churn
	// is bounded so resizes (and therefore restarts) are bounded.  After
	// the churn runs out the iteration drains with no further mutation.
	for op := 0; ; {
		items, next := tt.Scan(cursor, count)
		batches++
		for _, v := range items {
			got[v] = true
		}
		if next == 0 {
			break
		}
		cursor = next
		if batches > 1_000_000 {
			t.Fatal("iteration did not terminate under churn")
		}
		if op >= 4000 {
			continue
		}
		op++
		// Adversarial churn: mostly inserts (forcing doublings), some
		// deletes of live elements.
		switch rng.IntN(4) {
		case 0, 1, 2:
			tt.Insert(nextKey)
			live[nextKey] = true
			nextKey++
		case 3:
			for v := range live {
				if rng.IntN(2) == 0 {
					tt.Delete(v)
					delete(live, v)
				}
				break
			}
		}
	}
	// The final table (no longer mutating) must scan cleanly and exactly.
	final := scanAll(t, tt, count, 100_000)
	if len(final) != tt.Len() {
		t.Fatalf("final scan: %d distinct, Len %d", len(final), tt.Len())
	}
	for v := range live {
		if final[v] == 0 {
			t.Fatalf("live element %d missing from final scan", v)
		}
	}
}
