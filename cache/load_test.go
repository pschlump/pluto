/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadCoalescingAndUnrelatedKeys(t *testing.T) {
	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	value := NewLoading(func(ctx context.Context, key string) (int, error) {
		loads.Add(1)
		once.Do(func() { close(started) })
		select {
		case <-release:
			return len(key), nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})
	const workers = 16
	var ready, wait sync.WaitGroup
	ready.Add(workers)
	wait.Add(workers)
	errorsSeen := make(chan error, workers)
	for range workers {
		go func() {
			ready.Done()
			got, err := value.Get(context.Background(), "same")
			if err != nil {
				errorsSeen <- err
			} else if got != 4 {
				errorsSeen <- errors.New("wrong value")
			}
			wait.Done()
		}()
	}
	ready.Wait()
	<-started
	close(release)
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
	if loads.Load() != 1 {
		t.Fatal("loader calls", loads.Load())
	}
	stats := value.Stats()
	if stats.Loads != 1 || stats.LoadSuccesses != 1 || stats.Hits+stats.Misses != workers {
		t.Fatal(stats)
	}

	twoStarted := make(chan string, 2)
	twoRelease := make(chan struct{})
	unrelated := NewLoading(func(ctx context.Context, key string) (string, error) {
		twoStarted <- key
		<-twoRelease
		return key, nil
	})
	wait.Add(2)
	go func() { _, _ = unrelated.Get(context.Background(), "a"); wait.Done() }()
	go func() { _, _ = unrelated.Get(context.Background(), "b"); wait.Done() }()
	<-twoStarted
	<-twoStarted
	close(twoRelease)
	wait.Wait()
}

func TestLoadCancellationRecursionPanicAndErrorNotification(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	value := NewLoading(func(ctx context.Context, key string) (int, error) {
		close(started)
		<-release
		return 7, nil
	})
	firstDone := make(chan error, 1)
	go func() { _, err := value.Get(context.Background(), "a"); firstDone <- err }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := value.Get(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}

	var recursive *LoadingCache[string, int]
	recursive = NewLoading(func(ctx context.Context, key string) (int, error) {
		_, err := recursive.Get(ctx, key)
		return 0, err
	})
	if _, err := recursive.Get(context.Background(), "a"); !errors.Is(err, ErrRecursiveLoad) {
		t.Fatal(err)
	}

	panicCache := NewLoading(func(context.Context, string) (int, error) { panic("boom") })
	if _, err := panicCache.Get(context.Background(), "a"); err == nil {
		t.Fatal("loader panic was not returned")
	}

	causes := make(chan RemovalCause, 1)
	failed := NewLoading(func(context.Context, string) (int, error) { return 0, errors.New("load") },
		WithRemovalListener[string, int](func(event Notification[string, int]) { causes <- event.Cause }))
	if _, err := failed.Get(context.Background(), "a"); err == nil {
		t.Fatal("load error")
	}
	if cause := <-causes; cause != LoaderError {
		t.Fatal(cause)
	}
}

func TestRefreshReturnsOldAndRetainsOnFailure(t *testing.T) {
	clock := newFakeClock()
	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	replaced := make(chan Notification[string, int], 2)
	value := NewLoading(func(ctx context.Context, key string) (int, error) {
		call := loads.Add(1)
		if call == 2 {
			close(started)
			<-release
		}
		return int(call), nil
	},
		WithRefreshAfterWrite[string, int](10), WithClock[string, int](clock.Now),
		WithRemovalListener[string, int](func(event Notification[string, int]) { replaced <- event }))
	if got, err := value.Get(context.Background(), "a"); err != nil || got != 1 {
		t.Fatal(got, err)
	}
	clock.Advance(10)
	if got, err := value.Get(context.Background(), "a"); err != nil || got != 1 {
		t.Fatal("refresh did not return old", got, err)
	}
	<-started
	close(release)
	event := <-replaced
	if event.Cause != Replaced || event.Value != 1 {
		t.Fatal(event)
	}
	if got, _ := value.Peek("a"); got != 2 {
		t.Fatal(got)
	}

	var failingLoads atomic.Int32
	failedRefresh := make(chan struct{})
	failure := NewLoading(func(context.Context, string) (int, error) {
		if failingLoads.Add(1) == 1 {
			return 5, nil
		}
		close(failedRefresh)
		return 0, errors.New("refresh")
	}, WithRefreshAfterWrite[string, int](10), WithClock[string, int](clock.Now))
	if got, _ := failure.Get(context.Background(), "a"); got != 5 {
		t.Fatal(got)
	}
	clock.Advance(10)
	if got, _ := failure.Get(context.Background(), "a"); got != 5 {
		t.Fatal(got)
	}
	<-failedRefresh
	deadline := time.After(time.Second)
	for failure.Stats().LoadFailures != 1 {
		select {
		case <-deadline:
			t.Fatal("refresh failure not recorded")
		default:
			runtime.Gosched()
		}
	}
	if got, _ := failure.Peek("a"); got != 5 {
		t.Fatal("failed refresh replaced old value")
	}
}

func TestDirectWriteDuringRefreshWins(t *testing.T) {
	clock := newFakeClock()
	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	value := NewLoading(func(context.Context, string) (int, error) {
		if loads.Add(1) == 2 {
			close(started)
			<-release
		}
		return 2, nil
	}, WithRefreshAfterWrite[string, int](10), WithClock[string, int](clock.Now))
	_, _ = value.Get(context.Background(), "a")
	clock.Advance(10)
	_, _ = value.Get(context.Background(), "a")
	<-started
	value.Put("a", 9)
	close(release)
	deadline := time.After(time.Second)
	for value.Stats().LoadSuccesses != 2 {
		select {
		case <-deadline:
			t.Fatal("refresh did not finish")
		default:
			runtime.Gosched()
		}
	}
	if got, _ := value.Peek("a"); got != 9 {
		t.Fatal("late refresh overwrote direct write", got)
	}
}

func TestBulkLoadsAndPartialResults(t *testing.T) {
	value := NewLoading(func(_ context.Context, key int) (int, error) { return key * 2, nil },
		WithBulkLoader[int, int](func(_ context.Context, keys []int) ([]Entry[int, int], error) {
			entries := make([]Entry[int, int], len(keys))
			for index, key := range keys {
				entries[index] = Entry[int, int]{Key: key, Value: 0}
			}
			return entries, nil
		}))
	entries, err := value.GetAll(context.Background(), []int{1, 2, 1})
	if err != nil || len(entries) != 3 {
		t.Fatal(entries, err)
	}
	for _, entry := range entries {
		if entry.Value != 0 {
			t.Fatal(entries)
		}
	}
	if got, found := value.Peek(2); !found || got != 0 {
		t.Fatal("zero bulk value missing", got, found)
	}

	partial := NewLoading(func(_ context.Context, key int) (int, error) { return key, nil },
		WithBulkLoader[int, int](func(_ context.Context, keys []int) ([]Entry[int, int], error) {
			return []Entry[int, int]{{Key: keys[0], Value: keys[0]}}, errors.New("partial")
		}))
	entries, err = partial.GetAll(context.Background(), []int{1, 2})
	if err == nil || len(entries) != 2 || entries[0].Value != 1 {
		t.Fatal(entries, err)
	}
	if _, found := partial.Peek(1); !found {
		t.Fatal("partial successful entry not retained")
	}

	missing := NewLoading(func(_ context.Context, key int) (int, error) { return key, nil },
		WithBulkLoader[int, int](func(_ context.Context, keys []int) ([]Entry[int, int], error) { return nil, nil }))
	if _, err := missing.GetAll(context.Background(), []int{1}); !errors.Is(err, ErrBulkLoad) {
		t.Fatal(err)
	}
}

func TestCacheConcurrentStress(t *testing.T) {
	value := NewLoading(func(_ context.Context, key int) (int, error) { return key * 2, nil },
		WithMaximumSize[int, int](64), WithExpireAfterWrite[int, int](1000))
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				key := (worker + iteration) % 100
				_, _ = value.Get(context.Background(), key)
				value.GetIfPresent(key)
				value.Put(key, iteration)
				value.Snapshot()
				value.All()
				if iteration%11 == 0 {
					value.Invalidate(key)
				}
				if iteration%17 == 0 {
					value.CleanUp()
				}
			}
		}(worker)
	}
	wait.Wait()
	if value.Len() > 64 {
		t.Fatal("bound exceeded")
	}
}
