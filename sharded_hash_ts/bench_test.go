package sharded_hash_ts

// Throughput benchmarks for the striped table against the single-lock
// thread-safe tables (hash_grow_ts, cuckoo_ts) and against the manual
// N-independent-tables-plus-routing-glue arrangement this package replaces.
// All workloads run through b.RunParallel (GOMAXPROCS goroutines); the note
// asks for 1/4/16 stripes — 256 (the default stripe count) is included to
// show the plateau.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/pschlump/pluto/cuckoo_ts"
	"github.com/pschlump/pluto/hash_grow_ts"
)

// benchN is the preloaded keyspace every benchmark searches and churns over.
const benchN = 100_000

var benchKeys [benchN]string  // the preloaded, search-heavy keyspace
var benchChurn [benchN]string // the insert/delete churn keyspace

func init() {
	for i := range benchN {
		benchKeys[i] = fmt.Sprintf("key-%06d", i)
		benchChurn[i] = fmt.Sprintf("churn-%06d", i)
	}
}

// stringTable is the surface every contender exposes; used as a Go generic
// constraint (not an interface value), so each instantiation is compiled
// and called directly — no dynamic dispatch in the measured loops.
type stringTable interface {
	Insert(string) bool
	Search(string) (string, bool)
	Delete(string) bool
}

// preload fills a contender with the search keyspace (untimed).
func preload[T stringTable](b *testing.B, tt T) {
	b.Helper()
	for _, k := range benchKeys {
		tt.Insert(k)
	}
}

// runReadHeavy: 90% searches, 5% inserts, 5% deletes over the churn keys.
func runReadHeavy[T stringTable](b *testing.B, tt T) {
	b.Helper()
	preload(b, tt)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		n := uint64(0)
		for pb.Next() {
			k := n % benchN
			switch {
			case n%100 < 90:
				_, _ = tt.Search(benchKeys[k])
			case n%2 == 0:
				_ = tt.Insert(benchChurn[k])
			default:
				_ = tt.Delete(benchChurn[k])
			}
			n++
		}
	})
}

// runMixed: 50% searches, 25% inserts, 25% deletes.
func runMixed[T stringTable](b *testing.B, tt T) {
	b.Helper()
	preload(b, tt)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		n := uint64(0)
		for pb.Next() {
			k := n % benchN
			switch {
			case n%2 == 0:
				_, _ = tt.Search(benchKeys[k])
			case n%4 == 1:
				_ = tt.Insert(benchChurn[k])
			default:
				_ = tt.Delete(benchChurn[k])
			}
			n++
		}
	})
}

// newBenchSharded builds the striped table under test with string keys
// (the maphash-comparable constructor — the Ultima keyspace shape).
func newBenchSharded(stripes int) *ShardedHash[string] {
	return NewShardedHash[string](stripes, 16, 0)
}

// BenchmarkShardedReadHeavy demonstrates stripe-count scaling: 1 stripe is
// the degenerate single-lock case, and each doubling of the stripe count
// removes lock contention until the stripe count passes the core count.
func BenchmarkShardedReadHeavy(b *testing.B) {
	for _, stripes := range []int{1, 4, 16, 256} {
		b.Run(fmt.Sprintf("stripes=%d", stripes), func(b *testing.B) {
			runReadHeavy(b, newBenchSharded(stripes))
		})
	}
}

// BenchmarkShardedMixed is the 50/50 read/write version of the scaling run.
func BenchmarkShardedMixed(b *testing.B) {
	for _, stripes := range []int{1, 4, 16, 256} {
		b.Run(fmt.Sprintf("stripes=%d", stripes), func(b *testing.B) {
			runMixed(b, newBenchSharded(stripes))
		})
	}
}

// BenchmarkHashGrowTsReadHeavy is the single-lock baseline: pluto's fastest
// single-threaded table behind one RWMutex.
func BenchmarkHashGrowTsReadHeavy(b *testing.B) {
	runReadHeavy(b, hash_grow_ts.NewHashTab[string](1024, 0))
}

// BenchmarkHashGrowTsMixed is the single-lock hash_grow_ts 50/50 baseline.
func BenchmarkHashGrowTsMixed(b *testing.B) {
	runMixed(b, hash_grow_ts.NewHashTab[string](1024, 0))
}

// BenchmarkCuckooTsReadHeavy is the cuckoo single-lock baseline.
func BenchmarkCuckooTsReadHeavy(b *testing.B) {
	runReadHeavy(b, cuckoo_ts.NewHashTab[string](256, 0, 0))
}

// BenchmarkCuckooTsMixed is the cuckoo single-lock 50/50 baseline.
func BenchmarkCuckooTsMixed(b *testing.B) {
	runMixed(b, cuckoo_ts.NewHashTab[string](256, 0, 0))
}

// manualCuckoo16 is the arrangement the package replaces: sixteen
// independent cuckoo_ts tables plus caller-side FNV routing — the glue code
// (and the sixteen separate cursor spaces) a native striped table makes
// unnecessary.
type manualCuckoo16 struct {
	tabs [16]*cuckoo_ts.HashTab[string]
}

func newManualCuckoo16() *manualCuckoo16 {
	m := &manualCuckoo16{}
	for i := range m.tabs {
		m.tabs[i] = cuckoo_ts.NewHashTab[string](256, 0, 0)
	}
	return m
}

// route is the caller-side glue: hash the key, pick a table.
func (m *manualCuckoo16) route(key string) *cuckoo_ts.HashTab[string] {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return m.tabs[h.Sum64()&15]
}

func (m *manualCuckoo16) Insert(s string) bool { return m.route(s).Insert(s) }

func (m *manualCuckoo16) Search(s string) (string, bool) { return m.route(s).Search(s) }

func (m *manualCuckoo16) Delete(s string) bool { return m.route(s).Delete(s) }

// BenchmarkManualCuckoo16ReadHeavy is the 16-independent-tables baseline
// (the note's "N x cuckoo_ts").
func BenchmarkManualCuckoo16ReadHeavy(b *testing.B) {
	runReadHeavy(b, newManualCuckoo16())
}

// BenchmarkManualCuckoo16Mixed is the 16-independent-tables 50/50 baseline.
func BenchmarkManualCuckoo16Mixed(b *testing.B) {
	runMixed(b, newManualCuckoo16())
}

// BenchmarkShardedScan times full reverse-binary scans (count 100) of the
// preloaded 100k table, sequential — the SCAN family's per-element cost.
func BenchmarkShardedScan(b *testing.B) {
	h := newBenchSharded(256)
	for _, k := range benchKeys {
		h.Insert(k)
	}
	b.ResetTimer()
	for b.Loop() {
		cursor := uint64(0)
		for {
			items, next := h.Scan(cursor, 100)
			if len(items) == 0 && next == 0 {
				b.Fatalf("scan of a 100k table returned nothing")
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
}
