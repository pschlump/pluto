/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom_ts

import (
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
)

// TestConcurrent hammers the filter from writers and readers at once
// (run with -race).  The exact laws afterwards: Added equals the total
// number of write calls (a torn counter would read low) and every
// element any goroutine added reports present (no false negatives).
func TestConcurrent(t *testing.T) {
	const goroutines = 8
	const perG = 500
	b := NewBloom(goroutines*perG, 0.01)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(g int) { // writer: TestAndSet and Add over its own band
			defer wg.Done()
			var key [12]byte
			binary.LittleEndian.PutUint64(key[:8], uint64(g))
			for i := 0; i < perG; i++ {
				binary.LittleEndian.PutUint32(key[8:], uint32(i))
				b.TestAndSet(key[:])
				b.Add(key[:]) // deliberate duplicate
			}
		}(g)
		go func(g int) { // reader: membership and counters while writing
			defer wg.Done()
			var key [12]byte
			for i := 0; i < perG; i++ {
				binary.LittleEndian.PutUint64(key[:8], uint64(g))
				binary.LittleEndian.PutUint32(key[8:], uint32(i))
				b.MayContain(key[:])
				_ = b.Count()
				_ = b.Saturation()
				_ = b.IsEmpty()
			}
		}(g)
	}
	wg.Wait()

	if got, want := b.Added(), uint64(2*goroutines*perG); got != want {
		t.Errorf("Added = %d, want exactly %d (a torn counter reads low)", got, want)
	}
	var key [12]byte
	for g := 0; g < goroutines; g++ {
		for i := 0; i < perG; i++ {
			binary.LittleEndian.PutUint64(key[:8], uint64(g))
			binary.LittleEndian.PutUint32(key[8:], uint32(i))
			if !b.MayContain(key[:]) {
				t.Fatalf("false negative for g=%d i=%d after concurrent adds", g, i)
			}
		}
	}
}

// TestConcurrentMerge runs merges in both directions between two
// filters while writers keep adding (run with -race) — the operand-
// snapshot pattern's proof: no nested locks, so opposite-way merges
// cannot deadlock, and the union never loses an element.
func TestConcurrentMerge(t *testing.T) {
	a := NewBloom(20_000, 0.01)
	b := NewBloom(20_000, 0.01)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { // a.Merge(b) repeatedly
		defer wg.Done()
		for i := 0; i < 200; i++ {
			a.Merge(b)
		}
	}()
	go func() { // b.Merge(a) repeatedly — the opposite direction
		defer wg.Done()
		for i := 0; i < 200; i++ {
			b.Merge(a)
		}
	}()
	go func() { // writer on a
		defer wg.Done()
		var key [8]byte
		for i := uint64(0); i < 5000; i++ {
			binary.LittleEndian.PutUint64(key[:], i)
			a.Add(key[:])
		}
	}()
	go func() { // writer on b
		defer wg.Done()
		var key [8]byte
		for i := uint64(0); i < 5000; i++ {
			binary.LittleEndian.PutUint64(key[:], i+100_000)
			b.Add(key[:])
		}
	}()
	wg.Wait()

	a.Merge(b) // settle: a is the union of both bands
	var key [8]byte
	for i := uint64(0); i < 5000; i++ {
		binary.LittleEndian.PutUint64(key[:], i)
		if !a.MayContain(key[:]) {
			t.Fatalf("union lost element %d — a false negative", i)
		}
		binary.LittleEndian.PutUint64(key[:], i+100_000)
		if !a.MayContain(key[:]) {
			t.Fatalf("union lost element %d — a false negative", i+100_000)
		}
	}
}

// TestConcurrentCompound races Lock + Nl* compounds against plain
// writers (run with -race): each compound atomically admits a batch of
// keys and records a local tally; the exact laws afterwards are the
// total Added (compounds' adds + plain writes) and no false negatives.
func TestConcurrentCompound(t *testing.T) {
	const goroutines = 8
	const batch = 100
	b := NewBloom(50_000, 0.01)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		go func(g int) { // compound: Lock + Nl adds of a per-g batch
			defer wg.Done()
			b.Lock()
			defer b.Unlock()
			for i := 0; i < batch; i++ {
				b.NlAdd([]byte(fmt.Sprintf("compound-%d-%d", g, i)))
			}
		}(g)
		go func(g int) { // plain writer interleaving
			defer wg.Done()
			for i := 0; i < batch; i++ {
				b.Add([]byte(fmt.Sprintf("plain-%d-%d", g, i)))
			}
		}(g)
	}
	wg.Wait()

	if got, want := b.Added(), uint64(2*goroutines*batch); got != want {
		t.Errorf("Added = %d, want exactly %d", got, want)
	}
	for g := 0; g < goroutines; g++ {
		for i := 0; i < batch; i++ {
			if !b.MayContain([]byte(fmt.Sprintf("compound-%d-%d", g, i))) {
				t.Fatalf("false negative on compound-%d-%d", g, i)
			}
			if !b.MayContain([]byte(fmt.Sprintf("plain-%d-%d", g, i))) {
				t.Fatalf("false negative on plain-%d-%d", g, i)
			}
		}
	}
}
