/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadBufferOfferDrain(t *testing.T) {
	var buffer readBuffer[int, int]
	entries := make([]*cacheEntry[int, int], readBufferSize+1)
	for index := range entries {
		entries[index] = &cacheEntry[int, int]{key: index}
	}
	for index := range readBufferSize {
		accepted, pending := buffer.offer(entries[index], int64(index))
		if !accepted || pending != uint64(index+1) {
			t.Fatalf("offer %d: %v %d", index, accepted, pending)
		}
	}
	if accepted, _ := buffer.offer(entries[readBufferSize], 0); accepted {
		t.Fatal("offer into a full ring accepted")
	}
	next := 0
	buffer.drain(func(entry *cacheEntry[int, int], at int64) {
		if entry.key != next || at != int64(next) {
			t.Fatalf("drained %d@%d, want %d", entry.key, at, next)
		}
		next++
	})
	if next != readBufferSize {
		t.Fatal(next)
	}
	// A claimed-but-unpublished slot stops the drain; it resumes later.
	buffer.tail.Add(1)
	if accepted, _ := buffer.offer(entries[0], 7); !accepted {
		t.Fatal("offer after drain rejected")
	}
	drained := 0
	buffer.drain(func(*cacheEntry[int, int], int64) { drained++ })
	if drained != 0 {
		t.Fatal("drain passed an unpublished slot")
	}
	slot := &buffer.slots[readBufferSize&(readBufferSize-1)]
	slot.entry = entries[1]
	slot.seq.Store(readBufferSize + 1)
	buffer.drain(func(*cacheEntry[int, int], int64) { drained++ })
	if drained != 2 {
		t.Fatal(drained)
	}
}

// TestHitStatsExact checks that buffered lock-free hits are never lost:
// Stats drains every shard's buffer, including partially filled ones and the
// full-ring lock fallback.
func TestHitStatsExact(t *testing.T) {
	value := New[int, int]()
	for key := range 100 {
		value.Put(key, key)
	}
	const workers, reads = 8, 5000
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := range reads {
				key := (index*7 + worker) % 120 // keys ≥ 100 miss
				if got, found := value.GetIfPresent(key); found && got != key {
					panic(fmt.Sprint("wrong value ", got, " for ", key))
				}
			}
		}()
	}
	wait.Wait()
	stats := value.Stats()
	if stats.Hits+stats.Misses != workers*reads {
		t.Fatalf("hits %d + misses %d != %d", stats.Hits, stats.Misses, workers*reads)
	}
	wantMisses := uint64(0)
	for worker := range workers {
		for index := range reads {
			if (index*7+worker)%120 >= 100 {
				wantMisses++
			}
		}
	}
	if stats.Misses != wantMisses {
		t.Fatalf("misses %d, want %d", stats.Misses, wantMisses)
	}
}

// TestLockFreeReadersDuringGrowthAndChurn hammers lock-free reads of stable
// keys while writers grow the tables, churn tombstones, and replace values.
// A stable key must never be reported missing, and every value read must
// belong to the key that was asked for.
func TestLockFreeReadersDuringGrowthAndChurn(t *testing.T) {
	value := New[int, string](WithConcurrencyLevel[int, string](2))
	const stable = 64
	for key := range stable {
		value.Put(key, fmt.Sprint(key, ":0"))
	}
	var stop atomic.Bool
	var wait sync.WaitGroup
	for range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; !stop.Load(); index++ {
				key := index % stable
				got, found := value.GetIfPresent(key)
				if !found {
					panic(fmt.Sprint("stable key ", key, " missing"))
				}
				var gotKey, version int
				if _, err := fmt.Sscanf(got, "%d:%d", &gotKey, &version); err != nil || gotKey != key {
					panic(fmt.Sprint("key ", key, " read value ", got))
				}
				if _, found := value.Peek(key); !found {
					panic(fmt.Sprint("stable key ", key, " missing from Peek"))
				}
			}
		}()
	}
	for round := 1; round <= 20; round++ {
		for key := stable; key < stable+2000; key++ { // growth, then tombstones
			value.Put(key, fmt.Sprint(key, ":", round))
		}
		for key := stable; key < stable+2000; key++ {
			value.Invalidate(key)
		}
		for key := range stable { // replacement makes the old entries dead
			value.Put(key, fmt.Sprint(key, ":", round))
		}
	}
	stop.Store(true)
	wait.Wait()
	if value.Len() != stable {
		t.Fatal(value.Len())
	}
	// Tombstone churn must not grow the tables without bound.
	for index, shard := range value.shards {
		if size := len(shard.table.Load().slots); size > 8192 {
			t.Errorf("shard %d table grew to %d slots for %d live entries", index, size, shard.live)
		}
	}
}

// TestBufferedReadsKeepLRUOrder checks that lock-free hits still protect an
// entry from eviction: buffered reads are applied before the next write evicts.
func TestBufferedReadsKeepLRUOrder(t *testing.T) {
	value := New[string, int](WithMaximumSize[string, int](3))
	value.Put("a", 1)
	value.Put("b", 2)
	value.Put("c", 3)
	if _, found := value.GetIfPresent("a"); !found { // buffered, not yet applied
		t.Fatal("a missing")
	}
	value.Put("d", 4) // must drain the read first, then evict b
	if _, found := value.Peek("b"); found {
		t.Fatal("b should have been evicted")
	}
	if _, found := value.Peek("a"); !found {
		t.Fatal("recently read a was evicted")
	}
}

// TestBufferedAccessExtendsExpiry checks that an access recorded only in the
// read buffer still extends expire-after-access. The lock-free check sees the
// stale access time, and the locked retry must drain the buffer and find the
// entry live.
func TestBufferedAccessExtendsExpiry(t *testing.T) {
	clock := newFakeClock()
	value := New(WithExpireAfterAccess[string, int](10), WithClock[string, int](clock.Now))
	value.Put("a", 1)
	clock.Advance(8)
	if _, found := value.GetIfPresent("a"); !found { // access at 8, buffered
		t.Fatal("a missing at 8")
	}
	clock.Advance(8) // 16: stale access time 0 looks expired, buffered 8 does not
	if _, found := value.GetIfPresent("a"); !found {
		t.Fatal("buffered access did not extend expiry")
	}
	clock.Advance(10)
	if _, found := value.Peek("a"); found {
		t.Fatal("a should have expired at 26")
	}
	if stats := value.Stats(); stats.Hits != 2 || stats.Misses != 0 {
		t.Fatalf("stats %+v", stats)
	}
}

// TestConcurrentRefreshReaders races lock-free readers against refreshes that
// replace entries, under the race detector.
func TestConcurrentRefreshReaders(t *testing.T) {
	clock := newFakeClock()
	var loads atomic.Int64
	value := NewLoading(func(_ context.Context, key int) (int, error) {
		loads.Add(1)
		return key * 10, nil
	}, WithRefreshAfterWrite[int, int](5), WithClock[int, int](clock.Now),
		WithExecutor[int, int](func(operation func()) { operation() }))
	var stop atomic.Bool
	var wait sync.WaitGroup
	for range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; !stop.Load(); index++ {
				key := index % 16
				got, err := value.Get(context.Background(), key)
				if err != nil || got != key*10 {
					panic(fmt.Sprint("Get ", key, " = ", got, err))
				}
			}
		}()
	}
	for range 200 {
		clock.Advance(3)
		time.Sleep(50 * time.Microsecond)
	}
	stop.Store(true)
	wait.Wait()
	if loads.Load() <= 16 {
		t.Fatalf("expected refresh reloads, got %d loads", loads.Load())
	}
}

// TestInFlightPlaceholderInvisibleToLockFreeReads checks that a loading
// placeholder, which sits in the table before it is live, is never returned
// by the lock-free paths.
func TestInFlightPlaceholderInvisibleToLockFreeReads(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	value := NewLoading(func(_ context.Context, key string) (string, error) {
		close(started)
		<-release
		return key + "!", nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := value.Get(context.Background(), "a"); err != nil {
			panic(err)
		}
	}()
	<-started
	if got, found := value.GetIfPresent("a"); found {
		t.Fatalf("GetIfPresent saw placeholder %q", got)
	}
	if got, found := value.Peek("a"); found {
		t.Fatalf("Peek saw placeholder %q", got)
	}
	close(release)
	<-done
	if got, found := value.GetIfPresent("a"); !found || got != "a!" {
		t.Fatal(got, found)
	}
}

// TestStaleTableDeadEntryNotReturned simulates a reader still probing a table
// that was replaced by a rebuild: the frozen table still holds an entry that
// has since been invalidated, and the entry's own state must reject it.
func TestStaleTableDeadEntryNotReturned(t *testing.T) {
	value := New[int, int](WithConcurrencyLevel[int, int](1))
	value.Put(0, 100)
	shard := value.shards[0]
	stale := shard.table.Load()
	for key := 1; key < 64; key++ { // forces at least one rebuild
		value.Put(key, key)
	}
	if shard.table.Load() == stale {
		t.Fatal("expected a rebuild")
	}
	value.Invalidate(0)
	current := shard.table.Load()
	shard.table.Store(stale) // the reader's view
	_, foundGet := value.GetIfPresent(0)
	_, foundPeek := value.Peek(0)
	shard.table.Store(current)
	if foundGet || foundPeek {
		t.Fatalf("stale table returned an invalidated entry (GetIfPresent %v, Peek %v)", foundGet, foundPeek)
	}
}

// TestFullReadBufferIsLossless fills a shard's read ring while the shard lock
// is held (so TryLock drains fail). The next reader must block on the lock
// rather than drop its event, and every hit must be counted.
func TestFullReadBufferIsLossless(t *testing.T) {
	value := New[int, int](WithConcurrencyLevel[int, int](1), WithMaximumSize[int, int](1000))
	value.Put(1, 1)
	shard := value.shards[0]
	const reads = readBufferSize + 36
	shard.lock.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range reads {
			value.GetIfPresent(1)
		}
	}()
	for shard.reads.tail.Load() < readBufferSize {
		runtime.Gosched()
	}
	select {
	case <-done:
		shard.lock.Unlock()
		t.Fatal("reader finished while the ring was full and the lock held: events were dropped")
	default:
	}
	shard.lock.Unlock()
	<-done
	if hits := value.Stats().Hits; hits != reads {
		t.Fatalf("hits %d, want %d", hits, reads)
	}
}
