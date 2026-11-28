/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestShardCountRule(t *testing.T) {
	cases := []struct {
		name    string
		options []Option[int, int]
		want    int
	}{
		{"unbounded default", nil, 16},
		{"unbounded level 1", []Option[int, int]{WithConcurrencyLevel[int, int](1)}, 1},
		{"level rounds up to a power of two", []Option[int, int]{WithConcurrencyLevel[int, int](5)}, 8},
		{"size 0", []Option[int, int]{WithMaximumSize[int, int](0)}, 1},
		{"size 2", []Option[int, int]{WithMaximumSize[int, int](2)}, 1},
		{"size 19", []Option[int, int]{WithMaximumSize[int, int](19)}, 1},
		{"size 20", []Option[int, int]{WithMaximumSize[int, int](20)}, 2},
		{"size 256", []Option[int, int]{WithMaximumSize[int, int](256)}, 16},
		{"weight bounds too", []Option[int, int]{WithMaximumSize[int, int](1000), WithMaximumWeight[int, int](40)}, 4}, // Guava checks before doubling: 4 shards of 10
	}
	for _, tc := range cases {
		value := New(tc.options...)
		if got := len(value.shards); got != tc.want {
			t.Errorf("%s: %d shards, want %d", tc.name, got, tc.want)
		}
		if value.maximumSize >= 0 {
			total := 0
			for _, shard := range value.shards {
				total += shard.maximumSize
			}
			if total != value.maximumSize {
				t.Errorf("%s: shard sizes sum to %d, want %d", tc.name, total, value.maximumSize)
			}
		}
		if value.maximumWeight >= 0 {
			total := int64(0)
			for _, shard := range value.shards {
				total += shard.maximumWeight
			}
			if total != value.maximumWeight {
				t.Errorf("%s: shard weights sum to %d, want %d", tc.name, total, value.maximumWeight)
			}
		}
	}
}

func TestConcurrencyLevelPanics(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil || !strings.Contains(fmt.Sprint(recovered), "concurrency level") {
			t.Fatalf("got %v, want concurrency level panic", recovered)
		}
	}()
	New(WithConcurrencyLevel[int, int](0))
}

func TestShardedSizeBoundAndSpread(t *testing.T) {
	value := New(WithMaximumSize[int, int](256))
	for key := range 10000 {
		value.Put(key, key)
		if key%997 == 0 && value.Len() > 256 {
			t.Fatalf("Len %d exceeds bound", value.Len())
		}
	}
	// Sequential integer keys must spread: every shard ends up full.
	for index, shard := range value.shards {
		if shard.length != shard.maximumSize {
			t.Errorf("shard %d holds %d of %d", index, shard.length, shard.maximumSize)
		}
	}
	if got := value.Len(); got != 256 {
		t.Fatalf("Len %d, want 256", got)
	}
	if got := value.Stats().Evictions; got != 10000-256 {
		t.Fatalf("evictions %d, want %d", got, 10000-256)
	}
}

func TestInvalidateExpiredReportsExpired(t *testing.T) {
	clock := newFakeClock()
	causes := []RemovalCause{}
	value := New(WithExpireAfterWrite[string, int](10), WithClock[string, int](clock.Now),
		WithRemovalListener(func(event Notification[string, int]) { causes = append(causes, event.Cause) }))
	value.Put("a", 1)
	clock.Advance(10)
	value.Invalidate("a")
	if !slices.Equal(causes, []RemovalCause{Expired}) {
		t.Fatal(causes)
	}
}

// TestRefreshMovesToWriteQueueTail checks that a completed refresh restarts the
// entry's expire-after-write lifetime in queue order, not only in its timestamp.
func TestRefreshMovesToWriteQueueTail(t *testing.T) {
	clock := newFakeClock()
	value := NewLoading(func(_ context.Context, key string) (string, error) { return key + "!", nil },
		WithExpireAfterWrite[string, string](100), WithRefreshAfterWrite[string, string](10),
		WithClock[string, string](clock.Now), WithConcurrencyLevel[string, string](1),
		WithExecutor[string, string](func(operation func()) { operation() }))
	value.Put("a", "old")
	clock.Advance(5)
	value.Put("b", "b")
	clock.Advance(10) // a is due for refresh; b is not
	if got, err := value.Get(context.Background(), "a"); err != nil || got != "old" {
		t.Fatal(got, err)
	}
	clock.Advance(90) // b written at 5 expires at 105; refreshed a lives to 115
	if _, found := value.Peek("b"); found {
		t.Fatal("b should have expired")
	}
	if got, found := value.Peek("a"); !found || got != "a!" {
		t.Fatal("refreshed a should be live", got, found)
	}
	if value.Len() != 1 {
		t.Fatal(value.Len())
	}
}

// TestExpirationQueueModel drives random operations against a brute-force
// reference that applies the expiration rule to every key on every step (the
// former O(n) scan), at one shard and at the default shard count.
func TestExpirationQueueModel(t *testing.T) {
	type modelEntry struct {
		value             int
		written, accessed time.Duration
	}
	for _, level := range []int{1, 16} {
		for _, mode := range []string{"write", "access", "both"} {
			t.Run(fmt.Sprintf("%s/level%d", mode, level), func(t *testing.T) {
				clock := newFakeClock()
				options := []Option[int, int]{WithClock[int, int](clock.Now), WithConcurrencyLevel[int, int](level)}
				expireWrite, expireAccess := time.Duration(0), time.Duration(0)
				if mode != "access" {
					expireWrite = 50
					options = append(options, WithExpireAfterWrite[int, int](expireWrite))
				}
				if mode != "write" {
					expireAccess = 20
					options = append(options, WithExpireAfterAccess[int, int](expireAccess))
				}
				value := New(options...)
				model := map[int]modelEntry{}
				elapsed := time.Duration(0)
				live := func(entry modelEntry) bool {
					if expireWrite > 0 && elapsed-entry.written >= expireWrite {
						return false
					}
					return expireAccess == 0 || elapsed-entry.accessed < expireAccess
				}
				random := rand.New(rand.NewPCG(42, 1729))
				for step := range 5000 {
					key := random.IntN(64)
					switch random.IntN(6) {
					case 0, 1:
						value.Put(key, step)
						model[key] = modelEntry{value: step, written: elapsed, accessed: elapsed}
					case 2:
						got, found := value.GetIfPresent(key)
						entry, ok := model[key]
						want := ok && live(entry)
						if found != want || found && got != entry.value {
							t.Fatalf("step %d GetIfPresent(%d) = %d,%v want %d,%v", step, key, got, found, entry.value, want)
						}
						if want {
							entry.accessed = elapsed
							model[key] = entry
						}
					case 3:
						got, found := value.Peek(key)
						entry, ok := model[key]
						want := ok && live(entry)
						if found != want || found && got != entry.value {
							t.Fatalf("step %d Peek(%d) = %d,%v want %d,%v", step, key, got, found, entry.value, want)
						}
					case 4:
						value.Invalidate(key)
						delete(model, key)
					case 5:
						advance := time.Duration(random.IntN(8))
						clock.Advance(advance)
						elapsed += advance
					}
					if step%50 == 0 {
						want := []int{}
						for key, entry := range model {
							if live(entry) {
								want = append(want, key)
							}
						}
						got := []int{}
						for _, entry := range value.Snapshot() {
							got = append(got, entry.Key)
						}
						slices.Sort(want)
						slices.Sort(got)
						if !slices.Equal(got, want) {
							t.Fatalf("step %d snapshot keys\n got %v\nwant %v", step, got, want)
						}
						if value.Len() != len(want) {
							t.Fatalf("step %d Len %d want %d", step, value.Len(), len(want))
						}
					}
				}
			})
		}
	}
}
