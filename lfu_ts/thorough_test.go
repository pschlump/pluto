/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pschlump/pluto/lfu_ts"
)

// tsClock is a read-mostly clock for the race tests: written once
// before the goroutines start, only read inside them.
type tsClock struct{ min int64 }

func (c *tsClock) now() time.Time { return time.Unix(c.min*60, 0).UTC() }

// TestConcurrentTouchRead hammers the table from writers and readers
// at once (run with -race): 8 writers touch disjoint key ranges at
// logFactor 0 (deterministic counters), 4 readers poll Counter, Len
// and IdleMinutes on everything.  Every key's final counter must be
// exactly InitVal+its touch count — the write lock makes each Touch
// atomic, so nothing may be lost.
func TestConcurrentTouchRead(t *testing.T) {
	const writers, perWriter = 8, 500
	clk := &tsClock{min: 1000}
	l := lfu_ts.NewLfuWithClock[int](0, 0, clk.now) // no decay: counts exact

	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				l.Touch(w*perWriter + i)
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < 2000; i++ {
				l.Counter(i % (writers * perWriter))
				l.Len()
				l.IdleMinutes(i % (writers * perWriter))
			}
		}()
	}
	close(start)
	wg.Wait()

	if n := l.Len(); n != writers*perWriter {
		t.Fatalf("Len = %d, want %d", n, writers*perWriter)
	}
	for k := 0; k < writers*perWriter; k++ {
		v, ok := l.Counter(k)
		if !ok || v != lfu_ts.InitVal+1 { // each key touched exactly once
			t.Fatalf("Counter(%d) = (%d,%v), want (%d,true)", k, v, ok, lfu_ts.InitVal+1)
		}
	}
}

// TestConcurrentAddDelete races key creation against deletion; after
// the dust settles, Len must equal the surviving keys exactly.
func TestConcurrentAddDelete(t *testing.T) {
	clk := &tsClock{min: 2000}
	l := lfu_ts.NewLfuWithClock[int](0, 1, clk.now)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < 1000; i++ {
				k := (w*1000 + i) % 400
				l.Add(k)
				if i%3 == 0 {
					l.Delete(k)
				}
			}
		}(w)
	}
	close(start)
	wg.Wait()

	// Survivors are exactly the keys present — the table and Len must
	// agree (each Add/Delete was atomic).
	count := 0
	for k := 0; k < 400; k++ {
		if _, ok := l.Counter(k); ok {
			count++
		}
	}
	if count != l.Len() {
		t.Errorf("present keys %d != Len %d", count, l.Len())
	}
}

// TestLockNlCompound is the eviction-shaped compound under concurrency:
// holding the real Lock, scan the candidates with NlCounter and evict
// (NlDelete) the coldest — one consistent view for the whole decision,
// while concurrent writers keep touching the candidates.  Each evicted
// key must stay gone, and the section must be atomic per key.
func TestLockNlCompound(t *testing.T) {
	clk := &tsClock{min: 3000}
	l := lfu_ts.NewLfuWithClock[string](0, 0, clk.now)
	for i := 0; i < 50; i++ {
		l.Touch(string(rune('a' + i)))
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				l.Touch(string(rune('a' + w)))
			}
		}(w)
	}

	for round := 0; round < 20; round++ {
		l.Lock()
		coldest, coldestVal := "", uint8(255)
		// Candidates are the untouched keys 4..49 — the writers hammer
		// a..d, and an evicted-and-recreated key would blur the count.
		for i := 4; i < 50; i++ {
			k := string(rune('a' + i))
			if v, ok := l.NlCounter(k); ok && v < coldestVal {
				coldest, coldestVal = k, v
			}
		}
		l.NlDelete(coldest)
		if _, ok := l.NlCounter(coldest); ok {
			l.Unlock()
			t.Fatalf("round %d: evicted key %q still present", round, coldest)
		}
		l.Unlock()
	}
	close(stop)
	wg.Wait()

	if n := l.Len(); n != 30 {
		t.Errorf("Len after 20 evictions = %d, want 30", n)
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that key-level marshalers are honored through the table.
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
	// Exact output in first-insert order — the ts twin's enumeration is
	// deterministic, unlike the plain twin's bucket order.
	clk := &tsClock{min: 1000}
	l := lfu_ts.NewLfuWithClock[string](0, 1, clk.now)
	l.Touch("a") // InitVal + 1 at minute 1000
	l.Touch("b")
	l.Touch("b") // b: InitVal + 2
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("json.Marshal(l): %v", err)
	}
	want := `[{"key":"a","counter":6,"lastMin":1000},{"key":"b","counter":7,"lastMin":1000}]`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// An empty table encodes as [].
	if b, err := json.Marshal(lfu_ts.NewLfu[string](lfu_ts.DefaultLogFactor, lfu_ts.DefaultDecayTime)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty table, got (%s, %v)", b, err)
	}

	// A zero-value table is a tolerated read: [].
	var zero lfu_ts.Lfu[string]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value table, got (%s, %v)", b, err)
	}

	// A direct call on a nil table encodes as []; json.Marshal on a nil
	// *Lfu never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTab *lfu_ts.Lfu[string]
	if b, err := nilTab.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-table call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTab); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil table, got (%s, %v)", b, err)
	}

	// Key-level marshalers are honored.
	clk2 := &tsClock{min: 1000}
	custom := lfu_ts.NewLfuWithClock[upperString](0, 1, clk2.now)
	custom.Touch("x")
	if b, err := json.Marshal(custom); err != nil || string(b) != `[{"key":"X","counter":6,"lastMin":1000}]` {
		t.Errorf(`Expected [{"key":"X","counter":6,"lastMin":1000}], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	clk3 := &tsClock{min: 1000}
	bad := lfu_ts.NewLfuWithClock[chan int](0, 1, clk3.now)
	bad.Touch(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded entries restore the exact state.
	clk := &tsClock{min: 1000}
	l := lfu_ts.NewLfuWithClock[string](0, 1, clk.now)
	if err := json.Unmarshal([]byte(`[{"key":"a","counter":42,"lastMin":990}]`), l); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := l.Counter("a"); !ok || v != 32 { // 10 idle minutes at decayTime 1: 42 - 10
		t.Errorf("Counter after unmarshal = (%d, %v), want (32, true)", v, ok)
	}
	if m, _ := l.IdleMinutes("a"); m != 10 {
		t.Errorf("IdleMinutes after unmarshal = %d, want 10", m)
	}

	// A round trip rebuilds the table exactly — counters, idle minutes
	// and first-insert order all agree — and keeps the configuration
	// (Touch still works, and a second marshal is byte-identical).
	srcClk := &tsClock{min: 1000}
	src := lfu_ts.NewLfuWithClock[string](0, 1, srcClk.now)
	for range 3 {
		src.Touch("hot") // 8
	}
	src.Touch("cold") // 6
	src.Add("reset")  // 5
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	againClk := &tsClock{min: 1000}
	again := lfu_ts.NewLfuWithClock[string](0, 1, againClk.now)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for k, want := range map[string]uint8{"hot": 8, "cold": 6, "reset": 5} {
		if v, ok := again.Counter(k); !ok || v != want {
			t.Errorf("Counter(%q) after round trip = (%d,%v), want (%d,true)", k, v, ok, want)
		}
		if m, ok := again.IdleMinutes(k); !ok || m != 0 {
			t.Errorf("IdleMinutes(%q) after round trip = (%d,%v), want (0,true)", k, m, ok)
		}
	}
	if b2, err := json.Marshal(again); err != nil || string(b2) != string(b) {
		t.Errorf("Expected a byte-identical second marshal, got (%s, %v) want %s", b2, err, b)
	}
	if got := again.Touch("hot"); got != 9 {
		t.Errorf("Touch after unmarshal = %d, want 9 (configuration kept)", got)
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`[{"key":"z","counter":6,"lastMin":1000}]`), l); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if l.Len() != 1 {
		t.Errorf("Expected replacement, got length %d, want 1", l.Len())
	}
	if _, ok := l.Counter("a"); ok {
		t.Errorf("Expected the old key to be replaced.")
	}

	// An empty array and null clear the table.
	for _, data := range []string{"[]", "null"} {
		fullClk := &tsClock{min: 1000}
		full := lfu_ts.NewLfuWithClock[string](0, 1, fullClk.now)
		full.Touch("gone")
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if full.Len() != 0 {
			t.Errorf("Expected %s to clear the table.", data)
		}
		// The configuration is kept: the table stays usable.
		if got := full.Touch("back"); got != lfu_ts.InitVal+1 {
			t.Errorf("Touch after %s = %d, want %d", data, got, lfu_ts.InitVal+1)
		}
	}

	// Key-level unmarshalers are honored.
	cuClk := &tsClock{min: 1000}
	custom := lfu_ts.NewLfuWithClock[upperString](0, 1, cuClk.now)
	if err := json.Unmarshal([]byte(`[{"key":"X","counter":6,"lastMin":1000}]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := custom.Counter("X"); !ok || v != 6 {
		t.Errorf("Counter after unmarshal = (%d, %v), want (6, true)", v, ok)
	}

	// Decode errors are returned and leave the table untouched.
	keepClk := &tsClock{min: 1000}
	keep := lfu_ts.NewLfuWithClock[string](0, 1, keepClk.now)
	keep.Touch("keep") // 6
	for _, badData := range []string{
		`[{"key":"a",`,                        // truncated
		`{"key":"a","counter":6,"lastMin":1}`, // not an array
		"7",                                   // not an array
		`[{"key":1,"counter":6,"lastMin":1}]`, // wrong key type
		`[{"key":"a","counter":300,"lastMin":1}]`,   // counter out of uint8 range
		`[{"key":"a","counter":6,"lastMin":70000}]`, // lastMin out of uint16 range
	} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.Len() != 1 {
			t.Errorf("Table changed after the error on %s: Len %d", badData, keep.Len())
		}
		if v, ok := keep.Counter("keep"); !ok || v != 6 {
			t.Errorf("Counter changed after the error on %s: (%d, %v)", badData, v, ok)
		}
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing entries into a nil or zero-value table panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero lfu_ts.Lfu[string]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value table to be tolerated, got %v", data, err)
		}
	}
	expectPanicMessage(t, "UnmarshalJSON on zero-value", "lfu_ts: UnmarshalJSON on a zero-value",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"key":"a","counter":6,"lastMin":1}]`)) })
	expectPanicMessage(t, "UnmarshalJSON on zero-value names the fix", "NewLfu",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"key":"a","counter":6,"lastMin":1}]`)) })

	var nilTab *lfu_ts.Lfu[string]
	if err := nilTab.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON on nil", "lfu_ts: UnmarshalJSON on a nil",
		func() { _ = nilTab.UnmarshalJSON([]byte(`[{"key":"a","counter":6,"lastMin":1}]`)) })
}

// TestConcurrentJSON races MarshalJSON against concurrent Touch/Add
// writers (run with -race): every marshaled snapshot must decode and
// carry one entry per key present at some instant — never a torn table.
func TestConcurrentJSON(t *testing.T) {
	clk := &tsClock{min: 4000}
	l := lfu_ts.NewLfuWithClock[int](0, 0, clk.now) // no decay: counts exact

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				l.Touch(w*1000 + i%250)
			}
		}(w)
	}
	type entry struct {
		Key     int    `json:"key"`
		Counter uint8  `json:"counter"`
		LastMin uint16 `json:"lastMin"`
	}
	for i := 0; i < 200; i++ {
		b, err := json.Marshal(l)
		if err != nil {
			t.Fatalf("json.Marshal under concurrency: %v", err)
		}
		var entries []entry
		if err := json.Unmarshal(b, &entries); err != nil {
			t.Fatalf("snapshot does not decode: %v (%s)", err, b)
		}
		seen := make(map[int]bool, len(entries))
		for _, e := range entries {
			if seen[e.Key] {
				t.Fatalf("snapshot lists key %d twice", e.Key)
			}
			seen[e.Key] = true
			if e.Counter < lfu_ts.InitVal {
				t.Fatalf("snapshot counter %d below InitVal", e.Counter)
			}
		}
		if len(entries) > l.Len() {
			t.Fatalf("snapshot has %d keys but Len = %d", len(entries), l.Len())
		}
	}
	close(stop)
	wg.Wait()
}
