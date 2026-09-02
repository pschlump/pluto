/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts_test

import (
	"sync"
	"testing"
	"time"

	"github.com/pschlump/pluto/lfu_ts"
)

// tsClock is a read-mostly clock for the race tests: written once
// before the goroutines start, only read inside them.
type tsClock struct{ min int64 }

func (c *tsClock) now() time.Time { return time.Unix(c.min*60, 0).UTC() }

// TestConcurrentTouchRead hammers the table from writers and readers
// at once (run with -race): 8 writers touch disjoint key ranges at
// logFactor 0 (deterministic counters), 4 readers poll Counter, Len
// and IdleMinutes on everything.  Every key's final counter must be
// exactly InitVal+its touch count — the write lock makes each Touch
// atomic, so nothing may be lost.
func TestConcurrentTouchRead(t *testing.T) {
	const writers, perWriter = 8, 500
	clk := &tsClock{min: 1000}
	l := lfu_ts.NewLfuWithClock[int](0, 0, clk.now) // no decay: counts exact

	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				l.Touch(w*perWriter + i)
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < 2000; i++ {
				l.Counter(i % (writers * perWriter))
				l.Len()
				l.IdleMinutes(i % (writers * perWriter))
			}
		}()
	}
	close(start)
	wg.Wait()

	if n := l.Len(); n != writers*perWriter {
		t.Fatalf("Len = %d, want %d", n, writers*perWriter)
	}
	for k := 0; k < writers*perWriter; k++ {
		v, ok := l.Counter(k)
		if !ok || v != lfu_ts.InitVal+1 { // each key touched exactly once
			t.Fatalf("Counter(%d) = (%d,%v), want (%d,true)", k, v, ok, lfu_ts.InitVal+1)
		}
	}
}

// TestConcurrentAddDelete races key creation against deletion; after
// the dust settles, Len must equal the surviving keys exactly.
func TestConcurrentAddDelete(t *testing.T) {
	clk := &tsClock{min: 2000}
	l := lfu_ts.NewLfuWithClock[int](0, 1, clk.now)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < 1000; i++ {
				k := (w*1000 + i) % 400
				l.Add(k)
				if i%3 == 0 {
					l.Delete(k)
				}
			}
		}(w)
	}
	close(start)
	wg.Wait()

	// Survivors are exactly the keys present — the table and Len must
	// agree (each Add/Delete was atomic).
	count := 0
	for k := 0; k < 400; k++ {
		if _, ok := l.Counter(k); ok {
			count++
		}
	}
	if count != l.Len() {
		t.Errorf("present keys %d != Len %d", count, l.Len())
	}
}

// TestLockNlCompound is the eviction-shaped compound under concurrency:
// holding the real Lock, scan the candidates with NlCounter and evict
// (NlDelete) the coldest — one consistent view for the whole decision,
// while concurrent writers keep touching the candidates.  Each evicted
// key must stay gone, and the section must be atomic per key.
func TestLockNlCompound(t *testing.T) {
	clk := &tsClock{min: 3000}
	l := lfu_ts.NewLfuWithClock[string](0, 0, clk.now)
	for i := 0; i < 50; i++ {
		l.Touch(string(rune('a' + i)))
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				l.Touch(string(rune('a' + w)))
			}
		}(w)
	}

	for round := 0; round < 20; round++ {
		l.Lock()
		coldest, coldestVal := "", uint8(255)
		// Candidates are the untouched keys 4..49 — the writers hammer
		// a..d, and an evicted-and-recreated key would blur the count.
		for i := 4; i < 50; i++ {
			k := string(rune('a' + i))
			if v, ok := l.NlCounter(k); ok && v < coldestVal {
				coldest, coldestVal = k, v
			}
		}
		l.NlDelete(coldest)
		if _, ok := l.NlCounter(coldest); ok {
			l.Unlock()
			t.Fatalf("round %d: evicted key %q still present", round, coldest)
		}
		l.Unlock()
	}
	close(stop)
	wg.Wait()

	if n := l.Len(); n != 30 {
		t.Errorf("Len after 20 evictions = %d, want 30", n)
	}
}
