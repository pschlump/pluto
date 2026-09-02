/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts_test

import (
	"math/rand"
	"reflect"
	"sync"
	"testing"

	"github.com/pschlump/pluto/lru_ts"
)

// model is the reference implementation: a map for the contents and a
// recency slice (most-recently-used first) for the order.
type model struct {
	contents map[int]int
	recency  []int // MRU first
	capacity int
}

func newModel(capacity int) *model {
	return &model{contents: make(map[int]int), recency: []int{}, capacity: capacity}
}

func (m *model) touch(i int) {
	for j, k := range m.recency {
		if k == i {
			m.recency = append(m.recency[:j], m.recency[j+1:]...)
			break
		}
	}
	m.recency = append([]int{i}, m.recency...)
}

func (m *model) put(k, v int) {
	if _, ok := m.contents[k]; ok {
		m.contents[k] = v
		m.touch(k)
		return
	}
	for len(m.recency) >= m.capacity {
		victim := m.recency[len(m.recency)-1]
		m.recency = m.recency[:len(m.recency)-1]
		delete(m.contents, victim)
	}
	m.contents[k] = v
	m.recency = append([]int{k}, m.recency...)
}

func (m *model) get(k int) (int, bool) {
	v, ok := m.contents[k]
	if ok {
		m.touch(k)
	}
	return v, ok
}

func (m *model) delete(k int) bool {
	if _, ok := m.contents[k]; !ok {
		return false
	}
	delete(m.contents, k)
	for j, key := range m.recency {
		if key == k {
			m.recency = append(m.recency[:j], m.recency[j+1:]...)
			break
		}
	}
	return true
}

// TestLruRandomizedModel is the port of the plain package's model test
// (identical seed and step mix, so the two runs walk the same path):
// mixed Get/Put/Delete against the reference model, verifying the
// contents AND the full recency order after every step.
func TestLruRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const capacity = 7
	const keySpace = 40 // larger than capacity, so evictions are constant
	cache := lru_ts.NewLru[int, int](capacity)
	m := newModel(capacity)

	verify := func(step int) {
		if cache.Len() != len(m.contents) {
			t.Fatalf("step %d: Len()=%d, model has %d entries", step, cache.Len(), len(m.contents))
		}
		order := []int{}
		for k, v := range cache.All() {
			order = append(order, k)
			if mv, ok := m.contents[k]; !ok || mv != v {
				t.Fatalf("step %d: cache has %d=%d, model has %d,%v", step, k, v, mv, ok)
			}
		}
		if !reflect.DeepEqual(order, m.recency) {
			t.Fatalf("step %d: recency order %v, model %v", step, order, m.recency)
		}
	}

	for step := 0; step < 10000; step++ {
		k := rng.Intn(keySpace)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Put, including updates
			v := rng.Intn(1000)
			cache.Put(k, v)
			m.put(k, v)
		case 4, 5, 6, 7, 8: // Get
			cv, cok := cache.Get(k)
			mv, mok := m.get(k)
			if cok != mok || (cok && cv != mv) {
				t.Fatalf("step %d: Get(%d) = (%d, %v), model (%d, %v)", step, k, cv, cok, mv, mok)
			}
		default: // Delete
			if cok, mok := cache.Delete(k), m.delete(k); cok != mok {
				t.Fatalf("step %d: Delete(%d) = %v, model %v", step, k, cok, mok)
			}
		}
		verify(step)
	}
}

// TestVetoRandomized ports the plain package's randomized veto test:
// the veto callback must never let eviction drop a protected entry.
func TestVetoRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	protected := map[int]bool{3: true, 17: true, 28: true}
	cache := lru_ts.NewLruFunc(5, func(k int, _ int) bool { return !protected[k] })
	putProtected := map[int]bool{} // protected keys inserted so far
	for step := 0; step < 2000; step++ {
		k := rng.Intn(40)
		if rng.Intn(2) == 0 {
			cache.Put(k, step)
			if protected[k] {
				putProtected[k] = true
			}
		} else {
			cache.Get(k)
		}
		// A protected key that was put must still be present: the veto
		// must never let eviction drop it (this test never deletes).
		for pk := range putProtected {
			if _, ok := cache.Peek(pk); !ok {
				t.Fatalf("step %d: protected key %d was evicted", step, pk)
			}
		}
	}
}

// TestIteratorSnapshotSemantics pins the twin-side iterator contract,
// inverted from the plain package: the snapshot is materialized at
// call time, so mutating the cache from inside the loop is safe (and
// invisible to the running iteration).
func TestIteratorSnapshotSemantics(t *testing.T) {
	c := lru_ts.NewLru[int, int](8)
	for i := 0; i < 4; i++ {
		c.Put(i, i)
	}
	seen := []int{}
	for k, v := range c.All() {
		seen = append(seen, k)
		if v != k {
			t.Fatalf("All yielded %d=%d, want equal", k, v)
		}
		c.Put(k+100, k) // write from inside the loop: safe, not seen
		c.Get(k)        // Get takes the write lock: also safe here
	}
	if !reflect.DeepEqual(seen, []int{3, 2, 1, 0}) {
		t.Errorf("iteration saw %v, want the call-time snapshot [3 2 1 0]", seen)
	}
	if c.Len() != 8 {
		t.Errorf("Len after loop = %d, want 8", c.Len())
	}
}

// TestConcurrentGetPutDeleteIterate is the race-detector hammer (run
// with -race): workers do mixed Get/Put/Delete while a reader drains
// both iterators.  Afterwards the cache must be exactly consistent
// with a final snapshot.
func TestConcurrentGetPutDeleteIterate(t *testing.T) {
	const workers, ops = 8, 3000
	c := lru_ts.NewLru[int, int](50)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			rng := rand.New(rand.NewSource(int64(w)))
			for i := 0; i < ops; i++ {
				k := rng.Intn(80)
				switch rng.Intn(10) {
				case 0, 1, 2, 3:
					c.Put(k, w*ops+i)
				case 4, 5, 6, 7, 8:
					c.Get(k)
				default:
					c.Delete(k)
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 4000; i++ {
			// Each snapshot iteration is internally consistent: every
			// key appears at most once.  Cross-comparing counts or Len
			// would be wrong — writers run between the calls.
			seen := map[int]bool{}
			for k := range c.All() {
				if seen[k] {
					panic("All yielded a key twice in one snapshot")
				}
				seen[k] = true
			}
			seen = map[int]bool{}
			for k := range c.Backward() {
				if seen[k] {
					panic("Backward yielded a key twice in one snapshot")
				}
				seen[k] = true
			}
		}
	}()
	close(start)
	wg.Wait()

	// Final consistency: Len agrees with the snapshot, no duplicates.
	keys := map[int]bool{}
	for k := range c.All() {
		if keys[k] {
			t.Fatalf("key %d yielded twice", k)
		}
		keys[k] = true
	}
	if len(keys) != c.Len() {
		t.Errorf("snapshot has %d keys, Len = %d", len(keys), c.Len())
	}
}

// TestConcurrentEvictToCap is the note's compound gate: goroutines run
// "put a batch, then observe evict-to-capacity" as one Lock + Nl*
// section while others hammer the regular methods — the cache must
// never exceed capacity (nothing is vetoed) and the test must not
// deadlock.
func TestConcurrentEvictToCap(t *testing.T) {
	const capacity, workers, rounds = 20, 8, 500
	c := lru_ts.NewLru[int, int](capacity)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			rng := rand.New(rand.NewSource(1000 + int64(w)))
			for r := 0; r < rounds; r++ {
				c.Lock()
				for b := 0; b < 3; b++ { // the batch: atomic, evicts as it goes
					c.NlPut(rng.Intn(200), r)
				}
				if n := c.NlLen(); n > capacity {
					c.Unlock()
					panic("compound section exceeded capacity")
				}
				c.Unlock()
			}
		}(w)
	}
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < 5000; i++ {
				c.Put(i%200, i)
				c.Get(i % 200)
				if c.Len() > capacity {
					panic("Len exceeded capacity")
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	if c.Len() > capacity {
		t.Errorf("final Len = %d, want <= capacity %d", c.Len(), capacity)
	}
}

// TestLockNlSoftCapEvict shows the soft-cap flush as a compound: with
// every entry vetoed the cache grows past capacity; once the veto
// lifts, a single held-Lock NlPut evicts back down to capacity inside
// one atomic section.
func TestLockNlSoftCapEvict(t *testing.T) {
	vetoAll := true
	c := lru_ts.NewLruFunc(4, func(_, _ int) bool { return !vetoAll })
	for i := 0; i < 7; i++ {
		c.Put(i, i)
	}
	if c.Len() != 7 {
		t.Fatalf("soft-cap Len = %d, want 7", c.Len())
	}
	vetoAll = false

	c.Lock()
	c.NlPut(99, 99) // evicts LRU-first down to capacity, then inserts
	n := c.NlLen()
	v, ok := c.NlGet(99)
	c.Unlock()

	if n != 4 {
		t.Errorf("NlLen after evict-to-cap = %d, want 4", n)
	}
	if !ok || v != 99 {
		t.Errorf("NlGet(99) = (%d,%v)", v, ok)
	}
	if c.Len() != 4 {
		t.Errorf("Len after compound = %d, want 4", c.Len())
	}
}
