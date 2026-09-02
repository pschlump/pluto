package sharded_hash_ts

// Thorough tests for the striped hash table: structural invariants, growth
// and the deterministic doubling split, the Redis SCAN guarantee (frozen
// coverage, coverage across mid-scan growth and Truncate, cursor edges, the
// reverse-binary order itself), a randomized cross-check against a map
// model, LockKey compound sections, and concurrent access against a mutex
// guarded oracle.  TestData and newTestHash are defined in
// sharded_hash_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// checkInvariants verifies the structural invariants of the table: every
// node sits in the bucket its raw hash masks to, every stripe's live count
// matches its chain length, the stripes sum to the atomic Len, and Search
// finds every stored element.  Call it after structural changes — in
// single-goroutine tests only (it reads the internals without the locks).
func checkInvariants(t *testing.T, h *ShardedHash[TestData]) {
	t.Helper()
	total := 0
	for i, s := range h.stripes {
		live := 0
		seen := make(map[string]bool)
		for b, head := range s.tab.heads {
			m := uint64(len(s.tab.heads) - 1)
			for n := head; n != nil; n = n.next {
				if n.h&m != uint64(b) {
					t.Fatalf("stripe %d: node %q (h=%d) chained in bucket %d, want %d", i, n.data.S, n.h, b, n.h&m)
				}
				if seen[n.data.S] {
					t.Fatalf("stripe %d: key %q chained twice", i, n.data.S)
				}
				seen[n.data.S] = true
				if _, found := h.Search(n.data); !found {
					t.Fatalf("stripe %d: element %q not found by Search", i, n.data.S)
				}
				live++
			}
		}
		if live != s.tab.live {
			t.Fatalf("stripe %d: chain length %d, live counter %d", i, live, s.tab.live)
		}
		if h.StripeLen(i) != live {
			t.Fatalf("stripe %d: StripeLen %d, chain length %d", i, h.StripeLen(i), live)
		}
		total += live
		if len(s.tab.heads)&(len(s.tab.heads)-1) != 0 {
			t.Fatalf("stripe %d: heads %d is not a power of two", i, len(s.tab.heads))
		}
	}
	if total != h.Len() {
		t.Fatalf("stripes sum to %d elements, Len() = %d", total, h.Len())
	}
}

// TestGrowthDeterministicSplit fills stripes past their thresholds and
// verifies every element survives doubling, lands in the bucket its hash
// masks to, and stays findable.
func TestGrowthDeterministicSplit(t *testing.T) {
	h := newTestHash(4, 8, 0.75)
	const n = 2000
	for i := range n {
		h.Insert(TestData{S: fmt.Sprintf("g%05d", i), N: i})
	}
	grew := false
	for i := range h.StripeCount() {
		if len(h.stripes[i].tab.heads) > 16 { // past the normalized initial 16
			grew = true
		}
	}
	if !grew {
		t.Fatalf("expected stripes to double past 16 heads")
	}
	if h.Len() != n {
		t.Fatalf("expected length %d, got %d", n, h.Len())
	}
	for i := range n {
		v, found := h.Search(TestData{S: fmt.Sprintf("g%05d", i)})
		if !found || v.N != i {
			t.Fatalf("g%05d should survive growth with satellite %d, got %v found=%v", i, i, v, found)
		}
	}
	// Deleting half and re-filling keeps the invariants (unlink never moves
	// another node).
	for i := range n {
		if i%2 == 0 {
			if !h.Delete(TestData{S: fmt.Sprintf("g%05d", i)}) {
				t.Fatalf("expected to delete g%05d", i)
			}
		}
	}
	for i := range n {
		h.Insert(TestData{S: fmt.Sprintf("g%05d", i), N: i + 1})
	}
	if h.Len() != n {
		t.Fatalf("expected length %d after refill, got %d", n, h.Len())
	}
	checkInvariants(t, h)
}

// scanAll runs a Scan loop to completion (bounded), returning every element
// returned and failing the test if the iteration does not terminate.
func scanAll(t *testing.T, h *ShardedHash[TestData], count int) map[string]int {
	t.Helper()
	got := make(map[string]int)
	cursor := uint64(0)
	for calls := 0; ; calls++ {
		if calls > 1_000_000 {
			t.Fatalf("scan did not terminate within a million calls")
		}
		items, next := h.Scan(cursor, count)
		for _, it := range items {
			got[it.S]++
		}
		cursor = next
		if cursor == 0 {
			return got
		}
	}
}

// TestScanFrozenCoverage freezes a table, scans it to completion at several
// counts (including the default and counts larger than the table), and
// asserts exact set equality with no duplicates — no growth happens, so
// every bucket is visited exactly once.
func TestScanFrozenCoverage(t *testing.T) {
	h := newTestHash(8, 64, 0)
	want := make(map[string]bool)
	for i := range 1000 {
		k := fmt.Sprintf("f%05d", i)
		h.Insert(TestData{S: k, N: i})
		want[k] = true
	}
	for _, count := range []int{1, 7, 10, 100, 100000} {
		got := scanAll(t, h, count)
		if len(got) != len(want) {
			t.Fatalf("count %d: scan returned %d distinct keys, want %d", count, len(got), len(want))
		}
		for k, times := range got {
			if !want[k] {
				t.Fatalf("count %d: scan returned unknown key %q", count, k)
			}
			if times != 1 {
				t.Fatalf("count %d: key %q returned %d times on a frozen table, want 1", count, k, times)
			}
		}
	}
	// count <= 0 selects the default of 10 — must still cover everything.
	if got := scanAll(t, h, 0); len(got) != len(want) {
		t.Errorf("default count: scan returned %d distinct keys, want %d", len(got), len(want))
	}
	if got := scanAll(t, h, -5); len(got) != len(want) {
		t.Errorf("negative count: scan returned %d distinct keys, want %d", len(got), len(want))
	}
}

// TestScanGrowthMidScan is the core SCAN-contract test: while a scan is in
// progress, inserts force per-stripe doublings (small per-stripe tables, so
// growth happens on already-scanned and not-yet-scanned stripes alike).
// Every element present for the entire scan must be returned at least once.
func TestScanGrowthMidScan(t *testing.T) {
	for _, stripes := range []int{1, 2, 16} {
		for _, scanCount := range []int{1, 3} {
			t.Run(fmt.Sprintf("stripes=%d,count=%d", stripes, scanCount), func(t *testing.T) {
				h := newTestHash(stripes, 8, 0.75) // tiny tables: constant doubling
				const frozen = 300
				for i := range frozen {
					h.Insert(TestData{S: fmt.Sprintf("p%04d", i), N: i})
				}

				got := make(map[string]bool)
				cursor := uint64(0)
				churn := 0
				for calls := 0; ; calls++ {
					if calls > 200_000 {
						t.Fatalf("scan did not terminate")
					}
					items, next := h.Scan(cursor, scanCount)
					for _, it := range items {
						got[it.S] = true
					}
					cursor = next
					if cursor == 0 {
						break
					}
					// Between the first 50 calls: churn 20 new elements into
					// random stripes.  They force doublings mid-scan (on
					// already-scanned and not-yet-scanned stripes alike).
					// Churn is capped so growth is finite and the scan is
					// guaranteed to wrap — uncapped churn would add buckets
					// faster than a count=1 scan visits them.
					if calls < 50 {
						for range 20 {
							h.Insert(TestData{S: fmt.Sprintf("c%05d", churn), N: churn})
							churn++
						}
					}
				}

				// Every frozen element was present for the entire scan.
				for i := range frozen {
					k := fmt.Sprintf("p%04d", i)
					if !got[k] {
						t.Fatalf("element %q present throughout the scan was never returned (returned %d of %d frozen)", k, len(got), frozen)
					}
				}
				if h.Len() != frozen+churn {
					t.Fatalf("expected length %d, got %d", frozen+churn, h.Len())
				}
			})
		}
	}
}

// TestScanTruncateMidScan verifies the documented Truncate-during-scan
// behavior: the scan proceeds cleanly (no panic, no deadlock, terminates),
// the emptied table scans to a quick end, and a refilled table scans
// completely.
func TestScanTruncateMidScan(t *testing.T) {
	h := newTestHash(4, 16, 0)
	for i := range 500 {
		h.Insert(TestData{S: fmt.Sprintf("d%04d", i)})
	}
	cursor := uint64(0)
	calls := 0
	for {
		if calls > 100_000 {
			t.Fatalf("scan did not terminate after Truncate")
		}
		items, next := h.Scan(cursor, 10)
		_ = items
		cursor = next
		calls++
		if cursor == 0 {
			break
		}
		if calls == 5 {
			h.Truncate() // mid-scan wipe
		}
	}
	if h.Len() != 0 {
		t.Fatalf("expected an empty table, got length %d", h.Len())
	}
	// Refill and scan the new contents completely.
	for i := range 200 {
		h.Insert(TestData{S: fmt.Sprintf("r%04d", i)})
	}
	got := scanAll(t, h, 13)
	if len(got) != 200 {
		t.Fatalf("post-refill scan returned %d distinct keys, want 200", len(got))
	}
}

// TestScanCursorEdges covers the cursor contract directly: an invalid
// cursor (a stripe index past the end) restarts at 0 rather than panicking
// or skipping, and the cursor sequence of an unmutated scan never repeats.
func TestScanCursorEdges(t *testing.T) {
	h := newTestHash(4, 16, 0)
	for i := range 100 {
		h.Insert(TestData{S: fmt.Sprintf("e%03d", i)})
	}

	// Garbage cursors (stripe field past the end, wild slot bits) restart.
	for _, bad := range []uint64{1 << 63, uint64(len(h.stripes)) << slotBits, ^uint64(0)} {
		items, next := h.Scan(bad, 10)
		if next == bad {
			t.Errorf("cursor %d: Scan must not echo an invalid cursor", bad)
		}
		if len(items) == 0 && h.Len() > 0 {
			t.Errorf("cursor %d: a restart should return items from a non-empty table", bad)
		}
	}

	// The cursor sequence over a frozen table is duplicate-free and finite.
	seen := make(map[uint64]bool)
	cursor := uint64(0)
	for {
		if seen[cursor] {
			t.Fatalf("cursor %d repeated on a frozen table", cursor)
		}
		seen[cursor] = true
		_, next := h.Scan(cursor, 7)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if len(seen) < 10 {
		t.Errorf("expected a multi-call scan, got %d calls", len(seen))
	}
}

// TestRevBinNext directly verifies the reverse-binary bucket order for
// masks 3 and 7 against the sequences Redis's dictScan produces, plus the
// size-1 degenerate case.
func TestRevBinNext(t *testing.T) {
	sequences := map[uint64][]uint64{
		0: {0, 0},                      // single bucket: wraps immediately
		3: {0, 2, 1, 3, 0},             // size 4
		7: {0, 4, 2, 6, 1, 5, 3, 7, 0}, // size 8
	}
	for m, want := range sequences {
		v := uint64(0)
		for i, wantV := range want {
			if v != wantV {
				t.Fatalf("mask %d: position %d got %d, want %d", m, i, v, wantV)
			}
			v = revBinNext(v, m)
		}
	}
}

// TestScanCoverageUnderConcurrentMutation freezes a key set, then scans to
// completion while churn goroutines add and remove a disjoint key set (and
// one goroutine repeatedly Truncates nothing — it runs on a second table).
// Asserts no panic, no deadlock (bounded by the test timeout), and that
// every frozen key — present throughout — is returned at least once.
// Race-detector target (`make race`).
func TestScanCoverageUnderConcurrentMutation(t *testing.T) {
	h := newTestHash(8, 16, 0)
	const frozen = 400
	for i := range frozen {
		h.Insert(TestData{S: fmt.Sprintf("F%04d", i)})
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				k := fmt.Sprintf("M%d-%05d", w, i%3000)
				h.Insert(TestData{S: k})
				h.Search(TestData{S: k})
				h.Delete(TestData{S: k})
				i++
			}
		}(w)
	}

	got := make(map[string]bool)
	cursor := uint64(0)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not terminate under mutation")
		}
		items, next := h.Scan(cursor, 25)
		for _, it := range items {
			got[it.S] = true
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	close(stop)
	wg.Wait()

	for i := range frozen {
		k := fmt.Sprintf("F%04d", i)
		if !got[k] {
			t.Fatalf("frozen key %q was never returned by the scan under mutation", k)
		}
	}
}

// TestRandomizedModel runs a fixed-seed pseudo-random mix of Insert, Delete,
// Search and full-scan operations, cross-checking every result (including
// the satellite value of the latest insert) against a map model and
// periodically validating structural invariants.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	h := newTestHash(4, 8, 0.75)        // small tables: growth exercised early
	model := make(map[string]int)       // key -> satellite value of the latest insert

	key := func(i int) string { return fmt.Sprintf("r%05d", i) }

	for op := range 4000 {
		k := key(rng.Intn(300))
		switch rng.Intn(6) {
		case 0, 1, 2: // insert (weighted heavier)
			_, wasPresent := model[k]
			model[k] = op
			if got := h.Insert(TestData{S: k, N: op}); got == wasPresent {
				t.Fatalf("op %d: Insert(%q) = %v, model says previously present = %v", op, k, got, wasPresent)
			}
		case 3: // delete
			_, want := model[k]
			if got := h.Delete(TestData{S: k}); got != want {
				t.Fatalf("op %d: Delete(%q) = %v, model says %v", op, k, got, want)
			}
			delete(model, k)
		case 4: // search
			wantN, want := model[k]
			it, found := h.Search(TestData{S: k})
			if found != want {
				t.Fatalf("op %d: Search(%q) found=%v, model says %v", op, k, found, want)
			}
			if found && it.N != wantN {
				t.Fatalf("op %d: Search(%q) returned stale satellite %d, want %d", op, k, it.N, wantN)
			}
		case 5: // full scan — exact coverage of the model right now (no
			// concurrent mutation in this test, so the guarantee is exact)
			got := scanAll(t, h, 1+rng.Intn(50))
			if len(got) != len(model) {
				t.Fatalf("op %d: scan returned %d distinct keys, model has %d", op, len(got), len(model))
			}
			for mk := range model {
				if got[mk] != 1 {
					t.Fatalf("op %d: scan returned key %q %d times, want exactly 1", op, mk, got[mk])
				}
			}
		}
		if h.Len() != len(model) {
			t.Fatalf("op %d: Len() = %d, model has %d", op, h.Len(), len(model))
		}
		if op%250 == 0 {
			checkInvariants(t, h)
		}
	}
	checkInvariants(t, h)

	// Drain the table through Values and confirm it empties cleanly.
	toDelete := make([]string, 0, len(model))
	for item := range h.Values() {
		toDelete = append(toDelete, item.S)
	}
	for _, k := range toDelete {
		if !h.Delete(TestData{S: k}) {
			t.Fatalf("expected to delete %q during drain", k)
		}
	}
	if !h.IsEmpty() || h.Len() != 0 {
		t.Fatalf("table should be empty after drain, got length %d", h.Len())
	}
	checkInvariants(t, h)
}

// TestLockKeyCompound verifies the LockKey + Nl* escape hatch for compound
// operations: a search followed by a replace or delete runs atomically under
// one lock hold, and the Nl methods see a consistent stripe.
func TestLockKeyCompound(t *testing.T) {
	h := newTestHash(4, 16, 0)
	for i := range 100 {
		h.Insert(TestData{S: fmt.Sprintf("c%03d", i), N: i})
	}

	unlock := h.LockKey(TestData{S: "c042"})
	if v, found := h.NlSearch(TestData{S: "c042"}); found {
		if v.N != 42 {
			t.Errorf("NlSearch returned stale satellite %d, want 42", v.N)
		}
		if !h.NlDelete(TestData{S: "c042"}) {
			t.Errorf("NlDelete inside the held stripe lock should succeed")
		}
	} else {
		t.Errorf("NlSearch should have found c042")
	}
	if h.NlLen() != 99 || h.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty should report 99/false, got %d/%v", h.NlLen(), h.NlIsEmpty())
	}
	// A bulk insert of same-stripe keys under the same hold.
	for i := 200; i < 250; i++ {
		h.NlInsert(TestData{S: fmt.Sprintf("c%03d", i), N: i})
	}
	unlock()

	if h.Len() != 149 {
		t.Fatalf("expected length 149 after the compound section, got %d", h.Len())
	}
	if _, found := h.Search(TestData{S: "c042"}); found {
		t.Errorf("c042 should be gone")
	}
	for i := 200; i < 250; i++ {
		if v, found := h.Search(TestData{S: fmt.Sprintf("c%03d", i)}); !found || v.N != i {
			t.Errorf("expected to find c%03d with satellite %d, got %v found=%v", i, i, v, found)
		}
	}
	checkInvariants(t, h)
}

// TestLockKeyRMWAtomicity hammers one key with concurrent read-modify-write
// sections (LockKey + NlSearch + NlInsert of the bumped satellite).  The
// final satellite must equal the number of increments exactly — a lost
// update would show a smaller value.  Race-detector target (`make race`).
func TestLockKeyRMWAtomicity(t *testing.T) {
	h := newTestHash(4, 16, 0)
	h.Insert(TestData{S: "counter", N: 0})

	const writers = 8
	const each = 250
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for range each {
				unlock := h.LockKey(TestData{S: "counter"})
				if v, found := h.NlSearch(TestData{S: "counter"}); found {
					h.NlInsert(TestData{S: "counter", N: v.N + 1})
				}
				unlock()
			}
		}(w)
	}
	wg.Wait()

	v, found := h.Search(TestData{S: "counter"})
	if !found || v.N != writers*each {
		t.Fatalf("counter should be exactly %d (atomic RMW), got %v found=%v", writers*each, v, found)
	}
	if h.Len() != 1 {
		t.Fatalf("expected length 1, got %d", h.Len())
	}
}

// TestConcurrentOracle runs a random concurrent op mix (inserts, deletes,
// searches) against a mutex-guarded map oracle, cross-checking every return
// value in real time — the note's oracle test.  The oracle mutex is held
// across each predict-table-op-compare step, so the oracle and the table are
// updated atomically together and exact per-op verification is sound; the
// fully unsynchronized churn lives in TestConcurrentEverything.  Race-
// detector target (`make race`).
func TestConcurrentOracle(t *testing.T) {
	h := newTestHash(8, 16, 0)
	var mu sync.Mutex
	model := make(map[string]int) // key -> satellite of the latest insert

	const workers = 8
	const each = 500
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(1000 + w)))
			for range each {
				k := fmt.Sprintf("o%03d", rng.Intn(150))
				mu.Lock()
				switch rng.Intn(3) {
				case 0: // insert
					_, wasPresent := model[k]
					got := h.Insert(TestData{S: k, N: w})
					model[k] = w
					if got == wasPresent {
						t.Errorf("Insert(%q) = %v, oracle says previously present = %v", k, got, wasPresent)
					}
				case 1: // delete
					_, want := model[k]
					if got := h.Delete(TestData{S: k}); got != want {
						t.Errorf("Delete(%q) = %v, oracle says %v", k, got, want)
					}
					delete(model, k)
				case 2: // search
					wantN, want := model[k]
					it, found := h.Search(TestData{S: k})
					if found != want {
						t.Errorf("Search(%q) found=%v, oracle says %v", k, found, want)
					} else if found && it.N != wantN {
						t.Errorf("Search(%q) returned satellite %d, oracle says %d", k, it.N, wantN)
					}
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()

	// Final state: exact set equality with the oracle.
	if h.Len() != len(model) {
		t.Fatalf("Len() = %d, oracle has %d", h.Len(), len(model))
	}
	for k, n := range model {
		if v, found := h.Search(TestData{S: k}); !found || v.N != n {
			t.Fatalf("key %q should hold satellite %d, got %v found=%v", k, n, v, found)
		}
	}
}

// TestConcurrentEverything mixes every read path against one shared table —
// churn writers on a disjoint keyspace, full scans in flight, snapshot
// iterators, and the metrics readers.  It is primarily a race-detector and
// deadlock target (`make race`); the final state must equal the surviving
// insert set.
func TestConcurrentEverything(t *testing.T) {
	h := newTestHash(8, 16, 0)
	const keepers = 300
	for i := range keepers {
		h.Insert(TestData{S: fmt.Sprintf("K%04d", i)})
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Churn writers on a disjoint keyspace.
	for w := range 4 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				k := fmt.Sprintf("X%d-%05d", w, i)
				h.Insert(TestData{S: k})
				h.Delete(TestData{S: k})
				i++
			}
		}(w)
	}
	// Scanners.
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				cursor := uint64(0)
				for {
					_, next := h.Scan(cursor, 50)
					cursor = next
					if cursor == 0 {
						break
					}
				}
			}
		}()
	}
	// Snapshot iterators and a Truncate-free reader of the metrics.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			n := 0
			for _, v := range h.All() {
				_ = v
				n++
			}
			if n < keepers {
				t.Errorf("All saw %d elements, fewer than the %d never-deleted keepers", n, keepers)
				return
			}
			h.Len()
			h.StripeLen(3)
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

	if h.Len() != keepers {
		t.Fatalf("expected length %d after churn drains, got %d", keepers, h.Len())
	}
	got := scanAll(t, h, 31)
	if len(got) != keepers {
		t.Fatalf("final scan returned %d distinct keys, want %d", len(got), keepers)
	}
	checkInvariants(t, h)
}
