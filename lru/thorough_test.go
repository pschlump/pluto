/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
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

// Randomized model test: mixed Get/Put/Delete against the reference
// model, verifying the contents AND the full recency order after every
// step.  The seed is fixed so the run is deterministic.
func TestLruRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const capacity = 7
	const keySpace = 40 // larger than capacity, so evictions are constant
	cache := NewLru[int, int](capacity)
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

// The veto callback must never let eviction drop a protected entry,
// even under a random sequence: protected keys always survive.
func TestVetoRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	protected := map[int]bool{3: true, 17: true, 28: true}
	cache := NewLruFunc(5, func(k int, _ int) bool { return !protected[k] })
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

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the cache.
type upperString string

func (u upperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperString(s)
	return nil
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, most recently used first.
	c := NewLru[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `[{"k":"c","v":3},{"k":"b","v":2},{"k":"a","v":1}]` {
		t.Errorf("Expected MRU-first pairs, got %s", b)
	}

	// A Get re-marks, and the encoding follows the new recency order.
	c.Get("a")
	if b, err := json.Marshal(c); err != nil || string(b) != `[{"k":"a","v":1},{"k":"c","v":3},{"k":"b","v":2}]` {
		t.Errorf("Expected a-first after the Get, got (%s, %v)", b, err)
	}

	// An empty cache encodes as [].
	if b, err := json.Marshal(NewLru[int, int](2)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty cache, got (%s, %v)", b, err)
	}

	// A zero-value cache is a tolerated read: [].
	var zero Lru[int, int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value cache, got (%s, %v)", b, err)
	}

	// A direct call on a nil cache encodes as []; json.Marshal on a nil
	// *Lru never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilCache *Lru[int, int]
	if b, err := nilCache.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-cache call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilCache); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil cache, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewLru[string, upperString](2)
	custom.Put("x", upperString("y"))
	if b, err := json.Marshal(custom); err != nil || string(b) != `[{"k":"x","v":"Y"}]` {
		t.Errorf("Expected the value's own marshaler to be used, got (%s, %v)", b, err)
	}

	// A value the json package cannot encode returns its error.
	bad := NewLru[int, chan int](1)
	bad.Put(1, make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a cache of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// The encoded recency order is recreated: pair 0 becomes the most
	// recently used entry.
	c := NewLru[string, int](3)
	if err := json.Unmarshal([]byte(`[{"k":"a","v":1},{"k":"b","v":2}]`), c); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(c); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Expected recency [a b], got %v", got)
	}

	// A round trip rebuilds the cache and keeps the capacity and the
	// veto callback.
	src := NewLruFunc(2, func(k string, _ int) bool { return k != "pinned" })
	src.Put("pinned", 0)
	src.Put("x", 1)
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewLruFunc(2, func(k string, _ int) bool { return k != "pinned" })
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(again); !reflect.DeepEqual(got, mruOrder(src)) {
		t.Errorf("Round trip changed the recency order: %v, want %v", got, mruOrder(src))
	}
	if again.Capacity() != 2 {
		t.Errorf("Capacity after unmarshal = %d, want 2 (kept)", again.Capacity())
	}
	again.Put("y", 2)
	again.Put("z", 3) // "pinned" is LRU but vetoed: "x" and "y" go
	if _, ok := again.Get("pinned"); !ok {
		t.Errorf("Veto callback lost across unmarshal: pinned was evicted")
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte(`[{"k":"only","v":7}]`), c); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(c); !reflect.DeepEqual(got, []string{"only"}) {
		t.Errorf("Expected replacement, got %v", got)
	}

	// An empty array and null clear the cache.
	if err := json.Unmarshal([]byte("[]"), c); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if c.Len() != 0 {
		t.Errorf("Expected [] to clear the cache.")
	}
	c.Put("z", 26)
	if err := json.Unmarshal([]byte("null"), c); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if c.Len() != 0 || c.Capacity() != 3 {
		t.Errorf("Expected null to clear the cache and keep the capacity, got Len=%d Capacity=%d", c.Len(), c.Capacity())
	}

	// An array longer than the capacity is trimmed as it loads, least
	// recently used first — the eviction contract applies.
	small := NewLru[int, int](2)
	if err := json.Unmarshal([]byte(`[{"k":1,"v":10},{"k":2,"v":20},{"k":3,"v":30}]`), small); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(small); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("Expected the two MRU entries to survive, got %v", got)
	}

	// Duplicate keys follow the recency convention: the first (most
	// recently used) pair wins.
	dup := NewLru[string, int](3)
	if err := json.Unmarshal([]byte(`[{"k":"a","v":1},{"k":"a","v":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := dup.Peek("a"); !ok || v != 1 || dup.Len() != 1 {
		t.Errorf("Expected the first pair for a key to win, got (%d, %v), Len=%d", v, ok, dup.Len())
	}

	// Element-level unmarshalers are honored.
	custom := NewLru[string, upperString](2)
	if err := json.Unmarshal([]byte(`[{"k":"x","v":"Y"}]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := custom.Peek("x"); !ok || v != upperString("Y") {
		t.Errorf("Expected the value's own unmarshaler to be used, got (%q, %v)", v, ok)
	}

	// Decode errors are returned and leave the cache untouched.  (A pair
	// with a missing field, like {"k":"x"}, is NOT a decode error — the
	// json package fills in the zero value.)
	keep := NewLru[string, int](3)
	keep.Put("keep", 1)
	for _, badData := range []string{"[1,", `{"k":"a","v":1}`, "7", `[{"k":"a","v":"one"}]`, `[{"k":3,"v":1}]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got := mruOrder(keep); !reflect.DeepEqual(got, []string{"keep"}) {
			t.Errorf("Cache changed after the error on %s: %v", badData, got)
		}
		if v, ok := keep.Peek("keep"); !ok || v != 1 {
			t.Errorf("Cache contents changed after the error on %s.", badData)
		}
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing entries into a nil or zero-value cache panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Lru[string, int]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value cache to be tolerated, got %v", data, err)
		}
	}
	expectPanicMsg(t, "UnmarshalJSON on a zero-value cache",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"k":"a","v":1}]`)) },
		"lru", "UnmarshalJSON", "NewLru")

	var nilCache *Lru[string, int]
	for _, data := range []string{"[]", "null"} {
		if err := nilCache.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil cache to be tolerated, got %v", data, err)
		}
	}
	expectPanicMsg(t, "UnmarshalJSON on a nil cache",
		func() { _ = nilCache.UnmarshalJSON([]byte(`[{"k":"a","v":1}]`)) },
		"lru", "UnmarshalJSON", "nil cache")
}

// TestJSONStructField marshals and unmarshals an Lru nested in a struct
// through the encoding/json package.  The cache must be created with
// NewLru/NewLruFunc before unmarshaling: for a nil *Lru field the json
// package allocates a zero-value cache itself (no capacity), so
// non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string            `json:"title"`
		Seen  *Lru[string, int] `json:"seen"`
	}

	d := Doc{Title: "pluto", Seen: NewLru[string, int](4)}
	d.Seen.Put("ds", 1)
	d.Seen.Put("go", 2)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","seen":[{"k":"go","v":2},{"k":"ds","v":1}]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created cache field.
	var out Doc
	out.Seen = NewLru[string, int](4)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := mruOrder(out.Seen); !reflect.DeepEqual(got, []string{"go", "ds"}) {
		t.Errorf("Expected [go ds], got %v", got)
	}

	// A nil cache field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created cache and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","seen":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Seen: NewLru[string, int](4)}
	clearDoc.Seen.Put("gone", 9)
	if err := json.Unmarshal([]byte(`{"title":"x","seen":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null seen: %v", err)
	}
	if clearDoc.Seen.Len() != 0 {
		t.Errorf("Expected null seen to clear the cache.")
	}

	// Non-empty data into a nil *Lru field: the json package allocates a
	// zero-value cache, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	expectPanicMsg(t, "unmarshaling into an uncreated cache field", func() {
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","seen":[{"k":"a","v":1}]}`), &bad)
	}, "UnmarshalJSON", "NewLru")
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against the reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const capacity = 7
	const keySpace = 40
	cache := NewLru[int, int](capacity)
	m := newModel(capacity)

	for step := 0; step < 2000; step++ {
		k := rng.Intn(keySpace)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Put, including updates
			v := rng.Intn(1000)
			cache.Put(k, v)
			m.put(k, v)
		case 4, 5, 6, 7, 8: // Get
			cache.Get(k)
			m.get(k)
		default: // Delete
			cache.Delete(k)
			m.delete(k)
		}

		// Marshal must equal the model encoded by hand, MRU first.
		want := "["
		for i, mk := range m.recency {
			if i > 0 {
				want += ","
			}
			want += fmt.Sprintf(`{"k":%d,"v":%d}`, mk, m.contents[mk])
		}
		want += "]"
		got, err := json.Marshal(cache)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if string(got) != want {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh cache must reproduce the model,
		// contents and recency order.
		fresh := NewLru[int, int](capacity)
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if got := mruOrder(fresh); !reflect.DeepEqual(got, m.recency) {
			t.Fatalf("step %d: round trip recency %v, model %v", step, got, m.recency)
		}
		for mk, mv := range m.contents {
			if v, ok := fresh.Peek(mk); !ok || v != mv {
				t.Fatalf("step %d: round trip has %d=(%d, %v), model %d", step, mk, v, ok, mv)
			}
		}
	}
}
