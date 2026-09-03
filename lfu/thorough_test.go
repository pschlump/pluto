/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

// simulate applies n accesses to a fresh counter (InitVal) through the
// deterministic core, drawing from rng — the seeded stand-in for Incr's
// global source.
func simulate(rng *rand.Rand, logFactor, n int) uint8 {
	c := InitVal
	for range n {
		c = incrWithR(c, logFactor, rng.Float64())
	}
	return c
}

// expectedCounter is the fluid-limit expectation of the counter after n
// accesses at logFactor l: with increment probability 1/((c-5)l+1) the
// trajectory obeys dc/dn = 1/((c-5)l+1), which integrates to
// (l/2)x² + x = n for x = c-5.  It tracks the table printed in
// redis.conf (factor 10: 18 at 1000 hits, 142 at 100k) within a few
// percent — the table was one observed run; this is the expectation.
func expectedCounter(logFactor, n int) float64 {
	if logFactor <= 0 {
		return math.Min(float64(InitVal)+float64(n), float64(MaxVal))
	}
	l := float64(logFactor)
	x := (math.Sqrt(1+2*l*float64(n)) - 1) / l
	return math.Min(float64(InitVal)+x, float64(MaxVal))
}

// meanStd returns the mean and standard deviation of xs.
func meanStd(xs []uint8) (mean, sd float64) {
	for _, v := range xs {
		mean += float64(v)
	}
	mean /= float64(len(xs))
	for _, v := range xs {
		sd += (float64(v) - mean) * (float64(v) - mean)
	}
	sd = math.Sqrt(sd / float64(len(xs)))
	return
}

// TestCounterDistribution is the note's distribution gate: over
// thousands of trials with a seeded source, the counter after n
// accesses at factor 10 lands within a band of the fluid-limit
// expectation (mean gate) with a plausible spread, and the mean is
// strictly increasing in n — the counter is monotone in hits, which is
// what eviction ranking needs.
func TestCounterDistribution(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for _, tc := range []struct {
		hits, trials int
	}{
		{100, 10000},
		{1000, 10000},
		{10000, 3000},
		{100000, 300},
	} {
		results := make([]uint8, tc.trials)
		for i := range results {
			results[i] = simulate(rng, DefaultLogFactor, tc.hits)
		}
		mean, sd := meanStd(results)
		want := expectedCounter(DefaultLogFactor, tc.hits)
		tol := math.Max(2.0, 0.12*want) // the redis.conf table itself sits ~3% off the ODE
		if math.Abs(mean-want) > tol {
			t.Errorf("%d hits at factor %d: mean counter %.2f, want %.2f ± %.2f (sd %.2f)",
				tc.hits, DefaultLogFactor, mean, want, tol, sd)
		}
		if sd < 0.1 || sd > 30 {
			t.Errorf("%d hits: sd %.2f outside the plausible band [0.1, 30]", tc.hits, sd)
		}
		t.Logf("hits=%d mean=%.2f sd=%.2f expected=%.2f", tc.hits, mean, sd, want)
	}
	// Strict monotonicity of the means across hit counts.
	for _, pair := range [][2]int{{100, 1000}, {1000, 10000}} {
		sumA := 0.0
		for range 200 {
			sumA += float64(simulate(rng, DefaultLogFactor, pair[0]))
		}
		sumB := 0.0
		for range 200 {
			sumB += float64(simulate(rng, DefaultLogFactor, pair[1]))
		}
		avgA, avgB := sumA/200, sumB/200
		if avgB <= avgA {
			t.Errorf("mean counter not increasing in hits: %d hits %.2f vs %d hits %.2f", pair[0], avgA, pair[1], avgB)
		}
	}
}

// TestCounterDivergence is the note's determinism/divergence gate: two
// counters fed the same access pattern (independent draws from the same
// seeded stream) diverge on individual trials — the estimate is
// probabilistic — but their trial means agree within 4 standard errors.
func TestCounterDivergence(t *testing.T) {
	const (
		hits   = 1000
		trials = 10000
	)
	rng := rand.New(rand.NewPCG(42, 7))
	a, b := make([]uint8, trials), make([]uint8, trials)
	for i := range trials {
		ca, cb := InitVal, InitVal
		for range hits {
			ca = incrWithR(ca, DefaultLogFactor, rng.Float64())
			cb = incrWithR(cb, DefaultLogFactor, rng.Float64())
		}
		a[i], b[i] = ca, cb
	}
	ma, sa := meanStd(a)
	mb, sb := meanStd(b)
	if diff := math.Abs(ma - mb); diff > 4*math.Sqrt((sa*sa+sb*sb)/trials) {
		t.Errorf("paired means differ: %.3f vs %.3f (|Δ|=%.3f)", ma, mb, diff)
	}
	differ := 0
	for i := range trials {
		if a[i] != b[i] {
			differ++
		}
	}
	if differ < trials/20 { // same-pattern counters must diverge on individual runs
		t.Errorf("counters differed in only %d/%d trials, expected the large majority", differ, trials)
	}
	t.Logf("means %.3f/%.3f, differed in %d/%d trials", ma, mb, differ, trials)
}

// TestDecayExactSweep cross-checks decayed against the direct reference
// for every counter value and a spread of period counts, including
// counts far above the uint8 range.
func TestDecayExactSweep(t *testing.T) {
	for c := uint8(0); ; c++ {
		for _, p := range []int{0, -1, 1, 2, 3, 17, 254, 255, 256, 300, 65535, 1 << 20} {
			want := c
			if p > 0 {
				if p > int(c) {
					want = 0
				} else {
					want = c - uint8(p)
				}
			}
			if got := decayed(c, p); got != want {
				t.Fatalf("decayed(%d, %d) = %d, want %d", c, p, got, want)
			}
		}
		if c == MaxVal {
			break
		}
	}
}

// TestHotColdSeparation is the note's eviction-quality gate: after a
// mixed workload, hot keys carry measurably higher counters than cold
// ones.  The deterministic half runs the public table at logFactor 0
// (every increment lands); the statistical half runs the seeded core at
// factor 10 where separation must survive the randomness.
func TestHotColdSeparation(t *testing.T) {
	// Deterministic: counters track hit counts exactly until saturation.
	l, _ := newTestLfu[string](0, 0, 100)
	hits := []int{1, 10, 100, 1000, 10000}
	for i, h := range hits {
		for range h {
			l.Touch(fmtKey("key-", i))
		}
	}
	prev := uint8(0)
	for i := range hits {
		v, ok := l.Counter(fmtKey("key-", i))
		if !ok {
			t.Fatalf("key-%d missing", i)
		}
		want := uint8(math.Min(float64(InitVal)+float64(hits[i]), float64(MaxVal)))
		if v != want {
			t.Errorf("key with %d hits: counter %d, want %d", hits[i], v, want)
		}
		if v < prev {
			t.Errorf("counter not increasing with hits at index %d: %d < %d", i, v, prev)
		}
		prev = v
	}

	// Statistical: at factor 10 a 100x-hotter key must rank higher by a
	// wide margin, not merely on average edge cases.
	rng := rand.New(rand.NewPCG(42, 7))
	cold, hot := make([]uint8, 2000), make([]uint8, 2000)
	for i := range 2000 {
		cold[i] = simulate(rng, DefaultLogFactor, 100)
		hot[i] = simulate(rng, DefaultLogFactor, 10000)
	}
	mc, _ := meanStd(cold)
	mh, sh := meanStd(hot)
	if mh-mc < 10*sh { // ~10σ separation: eviction ranking is unambiguous
		t.Errorf("hot minus cold = %.2f with hot sd %.2f — not separated", mh-mc, sh)
	}
	t.Logf("cold mean %.2f, hot mean %.2f (hot sd %.2f)", mc, mh, sh)
}

// fmtKey builds a test key.
func fmtKey(prefix string, i int) string {
	return prefix + string(rune('a'+i))
}

// modelEntry is the randomized model's mirror of lfuEntry; lastMin is
// an unwrapped int64 so the reference arithmetic stays natural.
type modelEntry struct {
	counter uint8
	lastMin int64
}

// modelTable is an independent implementation of the table semantics:
// natural integer arithmetic, no wrap handling (the model test's clock
// never crosses the 65536-minute wrap — TestLfuClockWrap and
// TestElapsedMinutes cover it), no randomness (logFactor 0).
type modelTable struct {
	decayTime int
	min       int64
	m         map[string]modelEntry
}

func (m *modelTable) counter(k string) (uint8, bool) {
	e, ok := m.m[k]
	if !ok {
		return 0, false
	}
	c := int(e.counter)
	if m.decayTime > 0 {
		p := (m.min - e.lastMin) / int64(m.decayTime)
		c -= int(p)
		if c < 0 {
			c = 0
		}
	}
	if c > int(MaxVal) {
		c = int(MaxVal)
	}
	return uint8(c), true
}

func (m *modelTable) touch(k string) uint8 {
	c := int(InitVal)
	if e, ok := m.m[k]; ok {
		c = int(e.counter)
		if m.decayTime > 0 {
			p := (m.min - e.lastMin) / int64(m.decayTime)
			c -= int(p)
			if c < 0 {
				c = 0
			}
		}
	}
	if c < int(MaxVal) {
		c++ // logFactor 0: the increment is certain
	}
	m.m[k] = modelEntry{counter: uint8(c), lastMin: m.min}
	return uint8(c)
}

func (m *modelTable) idle(k string) (int, bool) {
	e, ok := m.m[k]
	if !ok {
		return 0, false
	}
	return int(m.min - e.lastMin), true
}

// TestLfuRandomizedModel cross-checks every operation's return value
// and the full post-state against the independent model at a fixed
// seed.  logFactor 0 keeps Touch deterministic (the increment is
// certain), so the oracle is exact.
func TestLfuRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const keyspace = 40
	l, clk := newTestLfu[string](0, 1, 1000)
	model := &modelTable{decayTime: 1, m: make(map[string]modelEntry)}

	checkAll := func(step int) {
		if l.Len() != len(model.m) {
			t.Fatalf("step %d: Len = %d, model %d", step, l.Len(), len(model.m))
		}
		for i := range keyspace {
			k := "k" + string(rune('a'+i))
			v, ok := l.Counter(k)
			mv, mok := model.counter(k)
			if ok != mok || (ok && v != mv) {
				t.Fatalf("step %d: Counter(%q) = (%d,%v), model (%d,%v)", step, k, v, ok, mv, mok)
			}
			idle, ok := l.IdleMinutes(k)
			midle, mok := model.idle(k)
			if ok != mok || (ok && idle != midle) {
				t.Fatalf("step %d: Idle(%q) = (%d,%v), model (%d,%v)", step, k, idle, ok, midle, mok)
			}
		}
	}

	for step := 1; step <= 4000; step++ {
		k := "k" + string(rune('a'+rng.IntN(keyspace)))
		switch rng.IntN(10) {
		case 0, 1, 2, 3, 4: // Touch (weighted highest — the hot path)
			got := l.Touch(k)
			want := model.touch(k)
			if got != want {
				t.Fatalf("step %d: Touch(%q) = %d, model %d", step, k, got, want)
			}
		case 5: // Add resets
			l.Add(k)
			model.m[k] = modelEntry{counter: InitVal, lastMin: model.min}
		case 6: // Delete
			got := l.Delete(k)
			_, want := model.m[k]
			delete(model.m, k)
			if got != want {
				t.Fatalf("step %d: Delete(%q) = %v, model %v", step, k, got, want)
			}
		case 7: // advance the clock 0..15 minutes (stays inside one wrap)
			d := rng.IntN(16)
			clk.advance(int64(d))
			model.min += int64(d)
		case 8: // Counter is a pure read: repeated calls agree and
			// never advance state — also on an unknown key.
			v1, ok1 := l.Counter(k)
			v2, ok2 := l.Counter(k)
			mv, mok := model.counter(k)
			if ok1 != ok2 || ok1 != mok || (ok1 && (v1 != v2 || v1 != mv)) {
				t.Fatalf("step %d: Counter(%q) impure or wrong: (%d,%v)/(%d,%v) model (%d,%v)",
					step, k, v1, ok1, v2, ok2, mv, mok)
			}
		case 9: // Truncate
			if rng.IntN(8) == 0 {
				l.Truncate()
				model.m = make(map[string]modelEntry)
			} else {
				idle, ok := l.IdleMinutes(k)
				midle, mok := model.idle(k)
				if ok != mok || (ok && idle != midle) {
					t.Fatalf("step %d: Idle(%q) = (%d,%v), model (%d,%v)", step, k, idle, ok, midle, mok)
				}
			}
		}
		if step%500 == 0 {
			checkAll(step)
		}
	}
	checkAll(4000)
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

// checkLfuSame reports a test error unless l reports exactly the model
// counters and idle minutes for every model key and nothing else.  The
// clock must stand still across the check (Counter is decay-adjusted at
// read time).
func checkLfuSame(t *testing.T, l *Lfu[string], model map[string]modelEntry, min int64) {
	t.Helper()
	m := &modelTable{decayTime: 1, min: min, m: model}
	if l.Len() != len(model) {
		t.Fatalf("Len = %d, model %d", l.Len(), len(model))
	}
	for k := range model {
		v, ok := l.Counter(k)
		mv, mok := m.counter(k)
		if ok != mok || (ok && v != mv) {
			t.Errorf("Counter(%q) = (%d,%v), model (%d,%v)", k, v, ok, mv, mok)
		}
		idle, ok := l.IdleMinutes(k)
		midle, mok := m.idle(k)
		if ok != mok || (ok && idle != midle) {
			t.Errorf("IdleMinutes(%q) = (%d,%v), model (%d,%v)", k, idle, ok, midle, mok)
		}
	}
}

func TestMarshalJSON(t *testing.T) {
	// Exact single-entry output (multi-entry order is the backing
	// table's bucket order — it varies from process to process).
	l, _ := newTestLfu[string](0, 1, 1000)
	l.Touch("a") // InitVal + 1 at minute 1000
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("json.Marshal(l): %v", err)
	}
	if string(b) != `[{"key":"a","counter":6,"lastMin":1000}]` {
		t.Errorf(`Expected [{"key":"a","counter":6,"lastMin":1000}], got %s`, b)
	}

	// An empty table encodes as [].
	if b, err := json.Marshal(NewLfu[string](DefaultLogFactor, DefaultDecayTime)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty table, got (%s, %v)", b, err)
	}

	// A zero-value table is a tolerated read: [].
	var zero Lfu[string]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value table, got (%s, %v)", b, err)
	}

	// A direct call on a nil table encodes as []; json.Marshal on a nil
	// *Lfu never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTab *Lfu[string]
	if b, err := nilTab.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-table call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTab); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil table, got (%s, %v)", b, err)
	}

	// Key-level marshalers are honored.
	custom, _ := newTestLfu[upperString](0, 1, 1000)
	custom.Touch("x")
	if b, err := json.Marshal(custom); err != nil || string(b) != `[{"key":"X","counter":6,"lastMin":1000}]` {
		t.Errorf(`Expected [{"key":"X","counter":6,"lastMin":1000}], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad, _ := newTestLfu[chan int](0, 1, 1000)
	bad.Touch(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded entries restore the exact state — the model agrees.
	l, _ := newTestLfu[string](0, 1, 1000)
	if err := json.Unmarshal([]byte(`[{"key":"a","counter":42,"lastMin":990}]`), l); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := l.Counter("a"); !ok || v != 32 { // 10 idle minutes at decayTime 1: 42 - 10
		t.Errorf("Counter after unmarshal = (%d, %v), want (32, true)", v, ok)
	}
	if m, _ := l.IdleMinutes("a"); m != 10 {
		t.Errorf("IdleMinutes after unmarshal = %d, want 10", m)
	}

	// A round trip rebuilds the table exactly — every key's counter and
	// idle time agree — and keeps the configuration (Touch still works).
	src, _ := newTestLfu[string](0, 1, 1000)
	for range 3 {
		src.Touch("hot") // 8
	}
	src.Touch("cold") // 6
	src.Add("reset")  // 5
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again, _ := newTestLfu[string](0, 1, 1000)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	model := map[string]modelEntry{
		"hot":   {counter: 8, lastMin: 1000},
		"cold":  {counter: 6, lastMin: 1000},
		"reset": {counter: 5, lastMin: 1000},
	}
	checkLfuSame(t, again, model, 1000)
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
		full, _ := newTestLfu[string](0, 1, 1000)
		full.Touch("gone")
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if full.Len() != 0 {
			t.Errorf("Expected %s to clear the table.", data)
		}
		// The configuration is kept: the table stays usable.
		if got := full.Touch("back"); got != InitVal+1 {
			t.Errorf("Touch after %s = %d, want %d", data, got, InitVal+1)
		}
	}

	// Key-level unmarshalers are honored.
	custom, _ := newTestLfu[upperString](0, 1, 1000)
	if err := json.Unmarshal([]byte(`[{"key":"X","counter":6,"lastMin":1000}]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := custom.Counter("X"); !ok || v != 6 {
		t.Errorf("Counter after unmarshal = (%d, %v), want (6, true)", v, ok)
	}

	// Decode errors are returned and leave the table untouched.
	keep, _ := newTestLfu[string](0, 1, 1000)
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
	var zero Lfu[string]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value table to be tolerated, got %v", data, err)
		}
	}
	expectPanicMessage(t, "UnmarshalJSON on zero-value", "UnmarshalJSON",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"key":"a","counter":6,"lastMin":1}]`)) })
	expectPanicMessage(t, "UnmarshalJSON on zero-value names the fix", "NewLfu",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"key":"a","counter":6,"lastMin":1}]`)) })

	var nilTab *Lfu[string]
	if err := nilTab.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON on nil", "UnmarshalJSON on a nil",
		func() { _ = nilTab.UnmarshalJSON([]byte(`[{"key":"a","counter":6,"lastMin":1}]`)) })
}
