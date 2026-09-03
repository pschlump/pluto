package hash_grow_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Scan tests: frozen-table coverage, edge cases, and a concurrent
// iteration running while inserts force growth — hash_grow inserts never
// relocate existing elements, so every pre-existing element must be
// covered even under the concurrent insert storm.  Run with -race.

import (
	"math/rand/v2"
	"sync"
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

func TestScanEdgeCases(t *testing.T) {
	tt := NewHashTab[int](64, 0)
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

// TestScanConcurrentInserts runs one full iteration while another
// goroutine's inserts force several growths.  hash_grow inserts only
// ever fill empty slots — they never relocate an existing element — so
// every pre-existing element must be returned, even with the restarts
// each growth causes.
func TestScanConcurrentInserts(t *testing.T) {
	tt := NewHashTab[int](64, 0.5)
	base := make(map[int]bool)
	for i := 0; i < 200; i++ {
		tt.Insert(i)
		base[i] = true
	}
	var (
		mu   sync.Mutex
		got  = make(map[int]bool)
		done = make(chan struct{})
	)
	go func() {
		defer close(done)
		cursor := uint64(0)
		for steps := 0; ; steps++ {
			items, next := tt.Scan(cursor, 7)
			mu.Lock()
			for _, v := range items {
				got[v] = true
			}
			mu.Unlock()
			if next == 0 {
				return
			}
			cursor = next
			if steps > 1_000_000 {
				return // the main goroutine reports the missing done channel
			}
		}
	}()
	// Insert storm: forces repeated doublings under the running scan.
	for i := 0; i < 5000; i++ {
		tt.Insert(1_000_000 + i)
	}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent Scan did not terminate after the insert storm")
	}
	mu.Lock()
	defer mu.Unlock()
	for v := range base {
		if !got[v] {
			t.Fatalf("pre-existing element %d lost under concurrent inserts", v)
		}
	}
	// The now-quiescent table scans exactly.
	final := scanAll(t, tt, 10, 1_000_000)
	if len(final) != tt.Len() {
		t.Fatalf("final scan: %d distinct, Len %d", len(final), tt.Len())
	}
}
