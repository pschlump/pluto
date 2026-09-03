package cuckoo_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Scan tests: frozen-table coverage, edge cases, and a scan running
// while an insert storm saturates the table and drives the background
// resize goroutine — no lock is held across Scan calls, so the resizer
// rebuilds strictly between batches.  Coverage is asserted on the
// quiescent table afterwards (a cuckoo insert's displacement chain can
// carry a present element across the cursor mid-churn — see scan.go).
// Run with -race.

import (
	"math/rand/v2"
	"sync/atomic"
	"testing"
	"time"
)

// scanAll drains a full iteration into a set, failing if it does not
// terminate within maxSteps batches.
func scanAll[T comparable](t *testing.T, tt *HashTab[T], count int, maxSteps int) map[T]int {
	t.Helper()
	got := make(map[T]int)
	cursor := uint64(0)
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
	tt := NewHashTab[int](256, 0, 0)
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

func TestScanEdgeCases(t *testing.T) {
	tt := NewHashTab[int](256, 0, 0)
	if items, next := tt.Scan(0, 10); items != nil || next != 0 {
		t.Fatalf("empty table: got (%v, %d)", items, next)
	}
	var niltt *HashTab[int]
	if items, next := niltt.Scan(0, 10); items != nil || next != 0 {
		t.Fatalf("nil table: got (%v, %d)", items, next)
	}
	tt.Insert(1)
	tt.Insert(2)
	if got := scanAll(t, tt, 0, 1000); len(got) != 2 {
		t.Fatalf("default count: scanned %d distinct, want 2", len(got))
	}
	if got := scanAll(t, tt, -3, 1000); len(got) != 2 {
		t.Fatalf("negative count: scanned %d distinct, want 2", len(got))
	}
	// Foreign cursors: no panic, no corruption.
	tt.Scan(0xFFFFFFFFFFFFFFFF, 1)
	if _, next := tt.Scan(encodeScanCursor(tt.generation, 1_000_000), 10); next != 0 {
		t.Fatal("out-of-range cursor did not end the iteration")
	}
	if tt.Len() != 2 {
		t.Fatalf("foreign cursors corrupted the table: Len %d", tt.Len())
	}
	// Scan across Truncate: the stale cursor restarts against the
	// emptied table and finishes.
	_, cursor := tt.Scan(0, 1)
	tt.Truncate()
	if items, next := tt.Scan(cursor, 1); items != nil || next != 0 {
		t.Fatalf("Scan across Truncate: got (%v, %d)", items, next)
	}
}

// TestScanDuringBackgroundResize scans continuously while an insert
// storm saturates the table and drives the background resize goroutine.
// The scanner must keep making progress (no deadlock, no lost wakeup
// between the read-locked batches and the resizer's write lock); exact
// coverage is asserted on the quiescent table at the end.
func TestScanDuringBackgroundResize(t *testing.T) {
	tt := NewHashTab[int](256, 0, 0)
	const base = 500
	for i := 0; i < base; i++ {
		tt.Insert(i)
	}
	var stop atomic.Bool
	var iterations atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			cursor := uint64(0)
			for steps := 0; ; steps++ {
				_, next := tt.Scan(cursor, 17)
				if next == 0 {
					break
				}
				cursor = next
				if steps > 10_000_000 {
					return // something wedged; main reports the timeout
				}
			}
			iterations.Add(1)
		}
	}()
	// The storm forces repeated background growth (each insert past the
	// grow threshold spawns the resizer).
	const storm = 20_000
	for i := 0; i < storm; i++ {
		tt.Insert(1_000_000 + i)
	}
	stop.Store(true)
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("scanner wedged during background resize")
	}
	if iterations.Load() == 0 {
		t.Fatal("scanner completed no full iteration during the storm")
	}
	// Quiescent coverage: nothing was ever deleted, so the final scan
	// must return every element exactly once as a set.
	final := scanAll(t, tt, 100, 10_000_000)
	if len(final) != tt.Len() {
		t.Fatalf("final scan: %d distinct, Len %d", len(final), tt.Len())
	}
	for i := 0; i < base; i++ {
		if final[i] == 0 {
			t.Fatalf("pre-existing element %d missing from final scan", i)
		}
	}
	for i := 0; i < storm; i++ {
		if final[1_000_000+i] == 0 {
			t.Fatalf("storm element %d missing from final scan", 1_000_000+i)
		}
	}
}
