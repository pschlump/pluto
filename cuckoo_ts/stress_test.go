/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// A concurrent stress test for the thread-safe cuckoo table.  Sixteen
// goroutines hammer one shared table with inserts, deletes and searches
// while reader goroutines walk it and drain the snapshot iterators, a band
// of goroutines runs compound Lock+Nl sections on a shared key range, and
// the background resizer grows and shrinks the table underneath all of
// them.  The run is profiled: a CPU profile over the whole stress phase is
// written to stress_cpu.pprof and a heap profile of the loaded table to
// stress_mem.pprof (view with `go tool pprof`).  The standard test flags
// (-cpuprofile, -memprofile) must not be combined with this test — the test
// starts its own CPU profile.
//
// What constitutes success (the evaluation criteria):
//
//  1. No data races or panics — run under `go test -race`.
//  2. Model agreement: each worker owns a disjoint key range and keeps a
//     local model of it, so every Insert/Delete/Search result is checked
//     against ground truth even mid-flight — a lost update, a resurrected
//     element or a duplicated slot shows up immediately.
//  3. Exact final state: after the workers and the resizer quiesce, Len
//     equals the model's count, every model key is found with its value,
//     every other key is absent, and Walk visits exactly Len elements.
//  4. The resizer actually ran and converged: the exact resize counters
//     from Info show at least one grow under load and one shrink after the
//     drain, and the settled saturation is at or below the grow threshold.
//     (Forced collision-loop resizes are expected in small numbers — growth
//     is asynchronous, so insert bursts can hit the kick limit before the
//     background rebuild lands — and are reported, not failed on.)
//  5. Progress: the stress phase finishes within a hard deadline (a
//     deadlock or lock convoy fails the test) and sustains a conservative
//     throughput floor.
//
// Run it with:
//
//	go test -run TestConcurrentStress -v
//	go test -race -run TestConcurrentStress -v
//
// `go test -short` runs a reduced workload.
package cuckoo_ts_test

import (
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pschlump/charon/cuckoo_ts"
)

const (
	stressWorkers       = 16
	stressKeysPerWorker = 5000
	stressChurnPerKey   = 10 // phase-B rounds per worker = keysPerWorker * this
	stressBandSize      = 64 // shared key range for the Lock+Nl hammer
	stressDeadline      = 3 * time.Minute
	stressMinOpsPerSec  = 10_000 // conservative progress floor; healthy runs do 100x this
)

// stressWorker runs the three phases against the worker's own key range
// [base, base+kpw) and returns its model of the range (model[i] means key
// base+i should be in the table) with the model's element count.  No other
// goroutine touches the range, so the model is ground truth and every
// operation's result is checked against it.
func stressWorker(t *testing.T, ht *cuckoo_ts.HashTab[int], w, kpw, churn int,
	errCount *atomic.Int64, opCount *atomic.Uint64) (model []bool, expected int) {

	base := w * kpw
	model = make([]bool, kpw)
	rng := xorshift64(0x9E3779B97F4A7C15 + uint64(w)*0x100000001B3)

	report := func(format string, args ...any) {
		if errCount.Add(1) <= 20 { // report the first few, count the rest
			args = append([]any{w}, args...)
			t.Errorf("worker %d: "+format, args...)
		}
	}

	// Phase A — fill: insert every key in a pseudo-random order.  All
	// inserts are of new keys; a "replaced" report means the table already
	// holds a key it should not.
	perm := make([]int, kpw)
	for i := range perm {
		perm[i] = i
	}
	for i := kpw - 1; i > 0; i-- {
		j := int(rng.next() % uint64(i+1))
		perm[i], perm[j] = perm[j], perm[i]
	}
	for _, i := range perm {
		k := base + i
		if !ht.Insert(k) {
			report("Insert(%d) reported replaced, want added", k)
		}
		model[i] = true
		opCount.Add(1)
		if i&63 == 0 { // spot-check: an inserted key is found immediately
			if v, found := ht.Search(k); !found || v != k {
				report("Search(%d) right after Insert = %d, %v", k, v, found)
			}
			opCount.Add(1)
		}
	}

	// Phase B — churn: a random mix of inserts, deletes and searches over
	// the worker's range, each checked against the model.  This crosses
	// the grow and shrink thresholds repeatedly while the other workers do
	// the same, so the background resizer is continuously in flight.
	for r := 0; r < kpw*churn; r++ {
		i := int(rng.next() % uint64(kpw))
		k := base + i
		switch x := rng.next() % 100; {
		case x < 40:
			added := ht.Insert(k)
			if added == model[i] { // added iff the model says absent
				report("Insert(%d) returned added=%v, model says present=%v", k, added, model[i])
			}
			model[i] = true
		case x < 55:
			found := ht.Delete(k)
			if found != model[i] {
				report("Delete(%d) returned found=%v, model says present=%v", k, found, model[i])
			}
			model[i] = false
		default:
			v, found := ht.Search(k)
			if found != model[i] {
				report("Search(%d) returned found=%v, model says present=%v", k, found, model[i])
			} else if found && v != k {
				report("Search(%d) returned value %d", k, v)
			}
		}
		opCount.Add(1)
	}

	// Phase C — drain: delete three of every four present keys.  This
	// leaves a deterministic final state and pushes the saturation below
	// the shrink threshold, so the shrink resizer runs alongside the
	// drains.
	for i := 0; i < kpw; i++ {
		if i%4 != 0 && model[i] {
			k := base + i
			if !ht.Delete(k) {
				report("Delete(%d) missed a present key", k)
			}
			model[i] = false
			opCount.Add(1)
		}
		if model[i] {
			expected++
		}
	}
	return model, expected
}

// TestConcurrentStress is the stress test; see the file comment for the
// workload and the success criteria.
func TestConcurrentStress(t *testing.T) {
	workers, kpw, churn := stressWorkers, stressKeysPerWorker, stressChurnPerKey
	if testing.Short() {
		workers, kpw, churn = 8, 1000, 4
	}
	bandBase := workers * kpw
	totalKeys := bandBase + stressBandSize

	// CPU profile over the whole stress phase.
	cpuFile, err := os.Create("stress_cpu.pprof")
	if err != nil {
		t.Fatalf("cannot create stress_cpu.pprof: %v", err)
	}
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		t.Fatalf("cannot start the CPU profile: %v", err)
	}

	start := time.Now()
	ht := cuckoo_ts.NewHashTab[int](5, 0, 0) // minimum size, default thresholds
	initialCap := ht.Capacity()

	var opCount atomic.Uint64
	var errCount atomic.Int64

	// Readers: hammer the O(1) reads (Search, Len, Saturation) and bounded
	// walks while the writers run; a full snapshot drain (All copies the
	// whole table under the read lock) runs only occasionally — four
	// goroutines draining O(n) snapshots in a tight loop would hold the
	// read lock almost continuously and starve every writer.  A snapshot
	// or bounded walk can never yield more elements than were ever
	// inserted.
	readStop := make(chan struct{})
	var readWG sync.WaitGroup
	for r := range 4 {
		readWG.Go(func() {
			rng := xorshift64(0xFEEDFACE + uint64(r)*0x9E3779B9)
			lastSnap := time.Now()
			for {
				select {
				case <-readStop:
					return
				default:
				}
				ht.Search(int(rng.next() % uint64(totalKeys))) // hit or miss: both legal mid-flight
				ht.Len()
				ht.Saturation()
				m := 0
				ht.Walk(func(pos int, data int) bool { m++; return m < 256 }) // bounded scan
				if m > totalKeys {
					errCount.Add(1)
					t.Errorf("Walk visited %d elements, more than the %d ever inserted", m, totalKeys)
					return
				}
				if time.Since(lastSnap) > 100*time.Millisecond {
					lastSnap = time.Now()
					n := 0
					for range ht.All() {
						n++
					}
					if n > totalKeys {
						errCount.Add(1)
						t.Errorf("All yielded %d elements, more than the %d ever inserted", n, totalKeys)
						return
					}
				}
			}
		})
	}

	// Band: three goroutines hammer a small shared key range with compound
	// Lock+Nl sections — writer-writer contention on the write lock while
	// inserts elsewhere trigger resizes.
	bandStop := make(chan struct{})
	var bandWG sync.WaitGroup
	for b := range 3 {
		bandWG.Go(func() {
			rng := xorshift64(0xC0FFEE + uint64(b)*0x9E3779B9)
			for {
				select {
				case <-bandStop:
					return
				default:
				}
				k := bandBase + int(rng.next()%stressBandSize)
				ht.Lock()
				switch rng.next() % 4 {
				case 0:
					ht.NlInsert(k)
				case 1:
					ht.NlDelete(k)
				default:
					ht.NlSearch(k)
				}
				ht.Unlock()
				opCount.Add(1)
			}
		})
	}

	// The workers, under a hard deadline: a deadlock or a lock convoy
	// fails the test instead of hanging it.
	models := make([][]bool, workers)
	expected := make([]int, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			models[w], expected[w] = stressWorker(t, ht, w, kpw, churn, &errCount, &opCount)
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(stressDeadline):
		t.Fatalf("stress phase did not finish within %v — deadlock or lock convoy suspected", stressDeadline)
	}

	close(bandStop)
	bandWG.Wait()
	close(readStop)
	readWG.Wait()

	// Quiesce the resizer, then normalize the band single-threaded (delete
	// whatever the band goroutines left) so the expected state is exact.
	settle(ht)
	for k := bandBase; k < bandBase+stressBandSize; k++ {
		if _, found := ht.Search(k); found {
			ht.Delete(k)
		}
	}
	settle(ht)

	pprof.StopCPUProfile()
	cpuFile.Close()
	elapsed := time.Since(start)

	// Heap profile of the settled, loaded table.
	runtime.GC()
	memFile, err := os.Create("stress_mem.pprof")
	if err != nil {
		t.Fatalf("cannot create stress_mem.pprof: %v", err)
	}
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		t.Fatalf("cannot write the heap profile: %v", err)
	}
	memFile.Close()

	// Criterion 2: nothing observed mid-flight contradicted a model.
	if n := errCount.Load(); n > 0 {
		t.Errorf("%d model/observer mismatches during the stress phase (first 20 reported above)", n)
	}

	// Criterion 3: the final state is exactly the union of the models.
	totalExpected := 0
	verifyMisses := 0
	for w := 0; w < workers; w++ {
		base := w * kpw
		totalExpected += expected[w]
		for i := 0; i < kpw; i++ {
			k := base + i
			v, found := ht.Search(k)
			if models[w][i] {
				if !found || v != k {
					if verifyMisses < 20 {
						t.Errorf("final check: key %d should be present, Search = %d, %v", k, v, found)
					}
					verifyMisses++
				}
			} else if found {
				if verifyMisses < 20 {
					t.Errorf("final check: key %d should be absent", k)
				}
				verifyMisses++
			}
		}
	}
	if verifyMisses > 20 {
		t.Errorf("final check: %d keys in the wrong state in total", verifyMisses)
	}
	if n := ht.Len(); n != totalExpected {
		t.Errorf("final length = %d, want %d", n, totalExpected)
	}
	walked := 0
	ht.Walk(func(pos int, data int) bool { walked++; return true })
	if walked != totalExpected {
		t.Errorf("Walk visited %d elements, want %d", walked, totalExpected)
	}

	// Criterion 4: the resizer grew the table under load, shrank it after
	// the drain, and converged — a settled table is at or below the grow
	// threshold.  The Info counters are exact (every successful rebuild is
	// counted under the write lock), unlike the capacity polling an
	// external observer has to resort to.  Forced (collision-loop) resizes
	// are not an error here: growth is asynchronous, so insert bursts run
	// against the saturated table and can hit the kick limit before the
	// background rebuild lands — the counter is reported for information.
	info := ht.Info()
	if info.Forced > info.Grows {
		t.Errorf("Forced = %d exceeds Grows = %d — Forced is a subset of Grows", info.Forced, info.Grows)
	}
	if info.Grows == 0 {
		t.Errorf("no growth resize under an %d-key load", workers*kpw)
	}
	if info.Shrinks == 0 {
		t.Errorf("no shrink resize after the drain to %d elements", totalExpected)
	}
	if info.Capacity <= initialCap {
		t.Errorf("final capacity %d, never grew past the initial %d", info.Capacity, initialCap)
	}
	deadline := time.Now().Add(10 * time.Second)
	for ht.Saturation() > 0.85 {
		if time.Now().After(deadline) {
			t.Fatalf("the resizer did not converge: saturation still %.4f after settling", ht.Saturation())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if c := ht.Capacity(); c < 256 || c&(c-1) != 0 {
		t.Errorf("final capacity %d is not a power of two at or above 256", c)
	}

	// Criterion 5: throughput floor (the deadline above covers the
	// no-progress case).
	ops := opCount.Load()
	opsPerSec := float64(ops) / elapsed.Seconds()
	if opsPerSec < stressMinOpsPerSec {
		t.Errorf("throughput %.0f ops/s is below the %d ops/s floor", opsPerSec, stressMinOpsPerSec)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("stress: %d ops in %v (%.0f ops/s), %d workers x %d keys", ops, elapsed.Round(time.Millisecond), opsPerSec, workers, kpw)
	t.Logf("resizes: %d grows (%d forced), %d shrinks, capacity %d -> final %d",
		info.Grows, info.Forced, info.Shrinks, initialCap, info.Capacity)
	t.Logf("final: len %d, saturation %.4f, heap in use %.1f MB", ht.Len(), ht.Saturation(), float64(m.Alloc)/(1<<20))
	t.Logf("profiles: stress_cpu.pprof, stress_mem.pprof (go tool pprof stress_cpu.pprof)")
}
