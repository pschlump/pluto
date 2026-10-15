package bag_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the snapshot-iterator semantics (inverted from the
// plain bag's live walk), a fixed-seed model cross-check, and the
// concurrent hammer under -race.

import (
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
)

// checkModel verifies the bag's observable state against a reference
// model (a plain slice in insertion order).
func checkModel(t *testing.T, b *Bag[int], model []int) {
	t.Helper()
	if got, want := b.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	var fwd []int
	for _, v := range b.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, model) {
		t.Fatalf("All: expected %v, got %v", model, fwd)
	}
	var bwd []int
	var bwdIdx []int
	for i, v := range b.Backward() {
		bwd = append(bwd, v)
		bwdIdx = append(bwdIdx, i)
	}
	var wantBwd []int
	for _, m := range slices.Backward(model) {
		wantBwd = append(wantBwd, m)
	}
	if !slices.Equal(bwd, wantBwd) {
		t.Fatalf("Backward: expected %v, got %v", wantBwd, bwd)
	}
	for k, i := range bwdIdx {
		if i != len(model)-1-k {
			t.Fatalf("Backward index %d = %d, expected %d", k, i, len(model)-1-k)
		}
	}
}

// TestIteratorSnapshotSemantics verifies the ts contract, inverted from
// the plain bag's live walk: the iterators copy a snapshot at call
// time, so mutations after the call (and from inside the loop) are not
// visible to the running iteration.
func TestIteratorSnapshotSemantics(t *testing.T) {
	var b Bag[int]
	for i := range 3 {
		b.Add(i + 1)
	}

	// A mutation after All() returns is invisible to the loop.
	it := b.All()
	b.Add(99)
	var seen []int
	for _, v := range it {
		seen = append(seen, v)
	}
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("snapshot iterator observed a later mutation: %v", seen)
	}

	// Mutating from inside the loop is safe (no deadlock) and the loop
	// still walks the call-time snapshot.
	seen = nil
	for _, v := range b.All() {
		seen = append(seen, v)
		if v == 1 {
			b.Truncate()
		}
	}
	if !slices.Equal(seen, []int{1, 2, 3, 99}) {
		t.Fatalf("snapshot iterator observed an in-loop mutation: %v", seen)
	}
	if !b.IsEmpty() {
		t.Fatalf("Truncate from inside the loop did not stick: %d", b.Length())
	}
}

// TestBackwardSnapshotSemantics is the Backward half of the snapshot
// contract.
func TestBackwardSnapshotSemantics(t *testing.T) {
	var b Bag[int]
	for i := range 3 {
		b.Add(i + 1)
	}
	it := b.Backward()
	b.Add(99)
	var seen []int
	for _, v := range it {
		seen = append(seen, v)
	}
	if !slices.Equal(seen, []int{3, 2, 1}) {
		t.Fatalf("Backward snapshot observed a later mutation: %v", seen)
	}
}

// TestRandomizedAgainstModel runs interleaved Add/Truncate rounds
// against a slice model.  Fixed PCG seed for reproducibility.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	var b Bag[int]
	var model []int
	for range 4000 {
		switch rng.IntN(8) {
		case 0:
			b.Truncate()
			model = nil
		default:
			v := rng.IntN(100) // duplicates are common
			b.Add(v)
			model = append(model, v)
		}
	}
	checkModel(t, &b, model)
}

// TestBagConcurrent hammers the bag from writers, readers and snapshot
// iterators at once.  The exact accounting: every writer adds a known
// number of distinct values, so the final length and the union of every
// collected snapshot observation must be right.
func TestBagConcurrent(t *testing.T) {
	const writers = 8
	const perWriter = 500

	var b Bag[int]
	var wg sync.WaitGroup
	for g := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				b.Add(g*perWriter + i)
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				_ = b.Len()
				_ = b.IsEmpty()
				for range b.All() {
				}
				for range b.Backward() {
				}
			}
		}()
	}
	wg.Wait()

	if got, want := b.Length(), writers*perWriter; got != want {
		t.Fatalf("after the storm: Length = %d, expected %d", got, want)
	}

	// Every distinct value must appear exactly once in the final bag.
	seen := make(map[int]int)
	for _, v := range b.All() {
		seen[v]++
	}
	if len(seen) != writers*perWriter {
		t.Fatalf("distinct values: got %d, expected %d", len(seen), writers*perWriter)
	}
	for _, n := range seen {
		if n != 1 {
			t.Fatalf("a value was observed %d times — adds were lost or duplicated", n)
		}
	}
}

// TestBagConcurrentTruncate races Truncate against Adds and snapshot
// iterators: no panic, no race, and the final state must agree with a
// final Truncate.
func TestBagConcurrentTruncate(t *testing.T) {
	var b Bag[int]

	// The truncator fights the writers until every writer has finished;
	// the iterator hammers snapshots throughout.
	var writers, truncator, iterators sync.WaitGroup
	stop := make(chan struct{})

	truncator.Add(1)
	go func() {
		defer truncator.Done()
		for {
			select {
			case <-stop:
				return
			default:
				b.Truncate()
			}
		}
	}()

	for g := range 4 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := range 1000 {
				b.Add(g*1000 + i)
			}
		}()
	}

	iterators.Add(1)
	go func() {
		defer iterators.Done()
		for range 500 {
			for range b.All() {
			}
		}
	}()

	writers.Wait()
	iterators.Wait()
	close(stop)
	truncator.Wait()

	// The truncator could have been descheduled between the writers'
	// final Adds and its exit, so pin the end state deterministically.
	b.Truncate()
	if !b.IsEmpty() || b.Length() != 0 {
		t.Fatalf("final Truncate left Length = %d", b.Length())
	}
	var fwd []int
	for _, v := range b.All() {
		fwd = append(fwd, v)
	}
	if len(fwd) != 0 {
		t.Fatalf("final bag is not empty: %v", fwd)
	}
}

// TestBagConcurrentJSON races the JSON encoders against writers: the
// snapshot is taken under the read lock, so marshaling is safe.
func TestBagConcurrentJSON(t *testing.T) {
	var b Bag[int]
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 1000 {
				b.Add(g*1000 + i)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 200 {
			if _, err := b.MarshalJSON(); err != nil {
				t.Errorf("MarshalJSON under load: %v", err)
				return
			}
		}
	}()
	wg.Wait()
	if b.Length() != 4000 {
		t.Fatalf("Length = %d, expected 4000", b.Length())
	}
}
