/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts_test

import (
	"encoding/json"
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

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// Exact output: an array of key/value objects, most recently used first
// (the All order), with the recency restored by a round trip.
func TestMarshalJSON(t *testing.T) {
	c := lru_ts.NewLru[string, int](4)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `[{"key":"c","value":3},{"key":"b","value":2},{"key":"a","value":1}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// Round trip: contents AND the recency order come back.
	back := lru_ts.NewLru[string, int](4)
	if err := json.Unmarshal(b, back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(back); !reflect.DeepEqual(got, []string{"c", "b", "a"}) {
		t.Errorf("recency after round trip = %v, want [c b a]", got)
	}
	for _, k := range []string{"a", "b", "c"} {
		v, ok := back.Peek(k)
		want := map[string]int{"a": 1, "b": 2, "c": 3}[k]
		if !ok || v != want {
			t.Errorf("Peek(%s) after round trip = (%d, %v), want (%d, true)", k, v, ok, want)
		}
	}

	// An empty cache encodes as [].
	empty := lru_ts.NewLru[int, int](2)
	if b, err := empty.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty cache, got (%s, %v)", b, err)
	}

	// A zero-value cache is a tolerated read: [].
	var zero lru_ts.Lru[int, int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value cache, got (%s, %v)", b, err)
	}

	// A direct call on a nil cache encodes as []; json.Marshal on a nil
	// *Lru never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilCache *lru_ts.Lru[int, int]
	if b, err := nilCache.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-cache call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilCache); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil cache, got (%s, %v)", b, err)
	}

	// Values that cannot be encoded return the json package's error.
	bad := lru_ts.NewLru[int, chan int](1)
	bad.Put(1, make(chan int))
	if _, err := bad.MarshalJSON(); err == nil {
		t.Errorf("Expected an error marshaling a cache of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is restored as the recency order: array element 0
	// becomes the most recently used entry.
	c := lru_ts.NewLru[string, int](4)
	if err := json.Unmarshal([]byte(`[{"key":"c","value":3},{"key":"a","value":1}]`), c); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(c); !reflect.DeepEqual(got, []string{"c", "a"}) {
		t.Fatalf("recency after decode = %v, want [c a]", got)
	}

	// The decoded entries REPLACE the previous contents; capacity and
	// the veto callback are kept.
	c.Put("x", 9)
	if err := json.Unmarshal([]byte(`[{"key":"n","value":5}]`), c); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if c.Len() != 1 {
		t.Errorf("Len after replace = %d, want 1", c.Len())
	}
	if _, ok := c.Peek("x"); ok {
		t.Error("old entry x survived a decode that replaces the contents")
	}
	if c.Capacity() != 4 {
		t.Errorf("Capacity after decode = %d, want 4 (kept)", c.Capacity())
	}

	// An array with more entries than the capacity is evicted down as
	// usual, keeping the most recently used entries.
	small := lru_ts.NewLru[int, int](2)
	if err := json.Unmarshal([]byte(`[{"key":1,"value":10},{"key":2,"value":20},{"key":3,"value":30}]`), small); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(small); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("over-capacity decode kept %v, want [1 2] (the MRU entries)", got)
	}

	// [] and null clear the cache and are tolerated everywhere.
	var nilCache *lru_ts.Lru[string, int]
	var zeroCache lru_ts.Lru[string, int]
	for _, data := range []string{"[]", "null"} {
		if err := nilCache.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil cache to be tolerated, got %v", data, err)
		}
		if err := zeroCache.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value cache to be tolerated, got %v", data, err)
		}
		if err := c.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("UnmarshalJSON(%s): %v", data, err)
		}
		if c.Len() != 0 {
			t.Errorf("Len after %s = %d, want 0 (cleared)", data, c.Len())
		}
	}

	// Decode errors leave the cache untouched.
	keep := lru_ts.NewLru[string, int](3)
	keep.Put("k", 1)
	for _, data := range []string{
		`{`,                          // malformed
		`{"key":"k"}`,                // not an array
		`[{"key":"k","value":"v"}]`,  // wrong value type
		`[{"key":1,"value":1}]`,      // wrong key type
	} {
		if err := keep.UnmarshalJSON([]byte(data)); err == nil {
			t.Errorf("Expected an error decoding %s", data)
		}
	}
	if keep.Len() != 1 {
		t.Fatalf("Len after decode errors = %d, want 1 (untouched)", keep.Len())
	}
	if v, ok := keep.Peek("k"); !ok || v != 1 {
		t.Errorf("Peek(k) after decode errors = (%d, %v), want (1, true)", v, ok)
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing entries into a nil or zero-value cache panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilCache *lru_ts.Lru[string, int]
	expectPanicMsg(t, "UnmarshalJSON on a nil cache",
		func() { _ = nilCache.UnmarshalJSON([]byte(`[{"key":"k","value":1}]`)) },
		"lru_ts:", "UnmarshalJSON", "nil cache", "NewLru")

	var zeroCache lru_ts.Lru[string, int]
	expectPanicMsg(t, "UnmarshalJSON on a zero-value cache",
		func() { _ = zeroCache.UnmarshalJSON([]byte(`[{"key":"k","value":1}]`)) },
		"lru_ts:", "UnmarshalJSON", "zero-value", "NewLru")
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with writers and a marshaling reader; every output must be a valid
// JSON array.  Run under -race.
func TestJSONConcurrent(t *testing.T) {
	c := lru_ts.NewLru[int, int](50)

	const workers = 8
	const perWorker = 100
	stop := make(chan struct{})
	var writers sync.WaitGroup
	var readers sync.WaitGroup

	// A marshaling reader: MarshalJSON snapshots under the read lock,
	// so it is safe while the writers replace the contents.
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := c.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON: %v", err)
				return
			}
			var probe []struct {
				Key   int `json:"key"`
				Value int `json:"value"`
			}
			if err := json.Unmarshal(b, &probe); err != nil {
				t.Errorf("MarshalJSON produced invalid JSON %s: %v", b, err)
				return
			}
		}
	}()

	// Writers replace the contents wholesale via UnmarshalJSON.
	for w := 0; w < workers; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := 0; i < perWorker; i++ {
				b, err := json.Marshal(map[string]int{"key": w*perWorker + i, "value": i})
				if err != nil {
					t.Errorf("worker %d: %v", w, err)
					return
				}
				doc := []byte("[" + string(b) + "]")
				if err := c.UnmarshalJSON(doc); err != nil {
					t.Errorf("worker %d: UnmarshalJSON: %v", w, err)
					return
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)
	readers.Wait()
	if c.Len() != 1 {
		t.Errorf("Len after concurrent JSON = %d, want 1 (the last single-entry decode)", c.Len())
	}
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
