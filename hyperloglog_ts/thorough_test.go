/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog_ts

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"
)

// TestConcurrentAddCount hammers the sketch from writers adding
// disjoint ranges while readers call Count and Bytes — the race
// detector's exercise of the read-lock Count against write-lock Adds
// (the atomic estimate cache is what makes that pairing safe).
func TestConcurrentAddCount(t *testing.T) {
	const writers = 8
	const perWriter = 40_000
	h := NewHll()
	var wg sync.WaitGroup
	start := make(chan struct{})

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var key [8]byte
			<-start
			for i := uint64(0); i < perWriter; i++ {
				binary.LittleEndian.PutUint64(key[:], uint64(w)<<32|i)
				h.Add(key[:])
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range 500 {
				h.Count()
				h.IsEmpty()
				if b := h.Bytes(); len(b) != DenseSize {
					t.Errorf("Bytes length %d during concurrent adds", len(b))
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	total := uint64(writers * perWriter)
	if got := h.Count(); got < total*97/100 || got > total*103/100 {
		t.Errorf("Count after %d concurrent adds = %d, want ~%d", total, got, total)
	}
}

// TestConcurrentMerge merges in opposite directions concurrently — the
// snapshot-under-operand-lock pattern must be deadlock-free (A merges B
// while B merges A) — and the final union must approximate the true
// cardinality.
func TestConcurrentMerge(t *testing.T) {
	const sketches = 4
	const perSketch = 20_000
	hlls := make([]*Hll, sketches)
	for s := range hlls {
		hlls[s] = NewHll()
		for i := uint64(0); i < perSketch; i++ {
			var key [8]byte
			binary.LittleEndian.PutUint64(key[:], uint64(s)<<32|i)
			hlls[s].Add(key[:])
		}
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	// Every sketch merges every other, all concurrently.
	for a := range hlls {
		for b := range hlls {
			if a == b {
				continue
			}
			wg.Add(1)
			go func(dst, src *Hll) {
				defer wg.Done()
				<-start
				dst.Merge(src)
			}(hlls[a], hlls[b])
		}
	}
	// Concurrent writers on the sources during the merges.
	for s := range hlls {
		wg.Add(1)
		go func(s *Hll) {
			defer wg.Done()
			var key [8]byte
			<-start
			for i := uint64(0); i < 1000; i++ {
				binary.LittleEndian.PutUint64(key[:], uint64(1)<<62|i)
				s.Add(key[:])
			}
		}(hlls[s])
	}
	close(start)
	wg.Wait()

	// After every sketch merged every other, all sketches must agree
	// bit-for-bit (max-union is commutative and idempotent over the
	// pre-merge snapshots; the late writers' elements are absorbed by
	// the same max).
	first := hlls[0].Bytes()
	for s := 1; s < sketches; s++ {
		if !bytes.Equal(first, hlls[s].Bytes()) {
			// Not strictly guaranteed (a merge may snapshot a source
			// before that source's own last merge landed) — but within
			// tolerance the union must hold.  Compare estimates.
			a, b := hlls[0].Count(), hlls[s].Count()
			lo, hi := uint64(float64(a)*0.97), uint64(float64(a)*1.03)
			if b < lo || b > hi {
				t.Errorf("sketches 0 and %d diverged: %d vs %d", s, a, b)
			}
		}
	}
	total := uint64(sketches * perSketch)
	if got := hlls[0].Count(); got < total*97/100 || got > total*103/100 {
		t.Errorf("merged Count = %d, want ~%d", got, total)
	}
}

// TestLockNlCompound exercises the compound surface: NlAdd/NlCount
// under a held Lock racing regular Adds from other goroutines.
func TestLockNlCompound(t *testing.T) {
	h := NewHll()
	var wg sync.WaitGroup
	start := make(chan struct{})

	// One goroutine runs locked add-batches via the Nl* surface.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var key [8]byte
		<-start
		for batch := uint64(0); batch < 200; batch++ {
			h.Lock()
			for i := uint64(0); i < 100; i++ {
				binary.LittleEndian.PutUint64(key[:], 1<<63|batch<<32|i)
				h.NlAdd(key[:])
			}
			h.NlCount()
			h.Unlock()
		}
	}()

	// Racers use the regular locked API.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			var key [8]byte
			<-start
			for i := uint64(0); i < 5_000; i++ {
				binary.LittleEndian.PutUint64(key[:], uint64(r)<<40|i)
				h.Add(key[:])
			}
		}(r)
	}
	close(start)
	wg.Wait()

	// 20000 + 20000 distinct elements.
	if got := h.Count(); got < 39_000 || got > 41_000 {
		t.Errorf("Count after compound writes = %d, want ~40000", got)
	}
}

// TestConcurrentReset hammers Reset against Adds on the same sketch —
// the final state must be a sane sketch whatever the interleaving.
func TestConcurrentReset(t *testing.T) {
	h := NewHll()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var key [8]byte
			<-start
			for i := uint64(0); i < 2_000; i++ {
				binary.LittleEndian.PutUint64(key[:], uint64(w)<<32|i)
				h.Add(key[:])
				if i%500 == 0 {
					h.Reset()
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for range 200 {
			h.Count() // must never observe a torn cache
		}
	}()
	close(start)
	wg.Wait()
	if got := h.Count(); got > 8_000 {
		t.Errorf("Count after reset race = %d, want ≤ 8000", got)
	}
}
