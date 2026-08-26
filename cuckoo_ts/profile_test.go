/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// A profiling benchmark: a mixed workload of 100,000 inserts, 10,000
// deletes and 1,000,000 searches against the cuckoo table, with a share of
// the deletes and searches aimed at keys that are not in the table.  It is
// meant as a baseline for comparing alternative hashing schemes: it reports
// the elapsed time, the number of memory allocations, the total bytes
// allocated, the live memory at the end, and how many times the table grew
// (and shrank) while the workload ran.
//
// Run it with:
//
//	go test -bench Profile -benchmem -benchtime 1x -v
package cuckoo_ts_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/pschlump/charon/cuckoo_ts"
)

const (
	profileInserts  = 100_000
	profileDeletes  = 10_000 // one delete per insertEveryDelete-th insert
	profileSearches = 1_000_000
	insertEveryDel  = profileInserts / profileDeletes
	searchPerInsert = profileSearches / profileInserts
)

// xorshift64 is a tiny deterministic PRNG so every run (and every hashing
// scheme measured against this baseline) sees the same workload.
type xorshift64 uint64

func (x *xorshift64) next() uint64 {
	*x ^= *x << 13
	*x ^= *x >> 7
	*x ^= *x << 17
	return uint64(*x)
}

// readMem returns an approximation of the total bytes allocated and the
// number of allocations since program start.
func readMem() (totalAlloc uint64, mallocs uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.TotalAlloc, m.Mallocs
}

// settle waits for the background resizer to finish: the capacity is
// re-read until it has been unchanged for three consecutive polls.
func settle(ht *cuckoo_ts.HashTab[int]) {
	last, stable := -1, 0
	for stable < 3 {
		c := ht.Capacity()
		if c == last {
			stable++
		} else {
			stable = 0
			last = c
		}
		if stable < 3 {
			time.Sleep(time.Millisecond)
		}
	}
}

// BenchmarkProfileMix runs the baseline mixed workload once.  Insert keys
// are a fixed permutation of 0..profileInserts-1; every tenth insert is
// followed by a delete and every insert by ten searches.  Deletes probe
// keys spread over a range wider than the inserted keys, and half of the
// searches probe keys that were never inserted, so a good share of both
// miss.
func BenchmarkProfileMix(b *testing.B) {
	for iter := 0; iter < b.N; iter++ {
		runtime.GC()
		totalAlloc0, mallocs0 := readMem()

		ht := cuckoo_ts.NewHashTab[int](5, 0, 0) // minimum size, default thresholds

		// The insert order is a fixed pseudo-random permutation of the keys.
		perm := make([]int, profileInserts)
		for i := range perm {
			perm[i] = i
		}
		rng := xorshift64(0x9E3779B97F4A7C15)
		for i := profileInserts - 1; i > 0; i-- {
			j := int(rng.next() % uint64(i+1))
			perm[i], perm[j] = perm[j], perm[i]
		}

		var searchHit, searchMiss, deleteHit, deleteMiss int
		var grows, shrinks int
		lastCap := ht.Capacity()

		b.ResetTimer()
		for i := 0; i < profileInserts; i++ {
			ht.Insert(perm[i])

			// 10 searches per insert: 5 over the live key range (hit once
			// the key is in), 5 over keys that are never inserted (always
			// a miss).
			for s := 0; s < searchPerInsert; s++ {
				var key int
				if s%2 == 0 {
					key = int(rng.next() % uint64(profileInserts))
				} else {
					key = profileInserts + int(rng.next()%uint64(profileInserts))
				}
				if _, found := ht.Search(key); found {
					searchHit++
				} else {
					searchMiss++
				}
			}

			// One delete every tenth insert: probe a key anywhere in the
			// live range plus 20% beyond, so early deletes and the tail
			// range mostly miss.
			if (i+1)%insertEveryDel == 0 {
				key := int(rng.next() % uint64(profileInserts*12/10))
				if ht.Delete(key) {
					deleteHit++
				} else {
					deleteMiss++
				}
			}

			if c := ht.Capacity(); c != lastCap {
				if c > lastCap {
					grows++
				} else {
					shrinks++
				}
				lastCap = c
			}
		}
		b.StopTimer()

		settle(ht) // let a pending background resize land before measuring

		totalAlloc1, mallocs1 := readMem()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Re-poll once more after the GC: a resizer woken by the GC pause
		// is harmless, but keep the reported capacity the settled one.
		finalCap := ht.Capacity()
		for lastCap != finalCap {
			if finalCap > lastCap {
				grows++
			} else {
				shrinks++
			}
			lastCap = finalCap
			settle(ht)
			finalCap = ht.Capacity()
		}

		b.ReportMetric(float64(searchHit), "search-hits")
		b.ReportMetric(float64(searchMiss), "search-misses")
		b.ReportMetric(float64(deleteHit), "delete-hits")
		b.ReportMetric(float64(deleteMiss), "delete-misses")
		b.ReportMetric(float64(grows), "table-grows")
		b.ReportMetric(float64(shrinks), "table-shrinks")
		b.ReportMetric(float64(finalCap), "final-capacity")
		b.ReportMetric(float64(ht.Len()), "final-len")
		b.ReportMetric(float64(mallocs1-mallocs0), "allocs")
		b.ReportMetric(float64(totalAlloc1-totalAlloc0), "alloc-bytes")
		b.ReportMetric(float64(m.Alloc), "live-bytes")
	}
}
