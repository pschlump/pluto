/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import (
	"strings"
	"testing"
	"time"
)

// fakeClock is a manually advanced clock for deterministic tests.
type fakeClock struct{ min int64 }

func (c *fakeClock) now() time.Time     { return time.Unix(c.min*60, 0).UTC() }
func (c *fakeClock) advance(min int64)  { c.min += min }
func newFakeClock(min int64) *fakeClock { return &fakeClock{min: min} }
func newTestLfu[K comparable](logFactor, decayTime int, min int64) (*Lfu[K], *fakeClock) {
	c := newFakeClock(min)
	return NewLfuWithClock[K](logFactor, decayTime, c.now), c
}

// TestCounterConstruction pins the initial states: NewMorrisCounter at
// InitVal (the Redis fresh-key state), the zero value at 0.
func TestCounterConstruction(t *testing.T) {
	if v := NewMorrisCounter().Value(); v != InitVal {
		t.Errorf("NewMorrisCounter().Value() = %d, want %d", v, InitVal)
	}
	var zero MorrisCounter
	if v := zero.Value(); v != 0 {
		t.Errorf("zero MorrisCounter Value = %d, want 0", v)
	}
	if MaxVal != 255 {
		t.Errorf("MaxVal = %d, want 255", MaxVal)
	}
	if InitVal != 5 {
		t.Errorf("InitVal = %d, want 5 (Redis LFU_INIT_VAL)", InitVal)
	}
}

// TestIncrMath pins LFULogIncr's probability math on exact (counter,
// logFactor, r) triples — the deterministic core, no RNG involved.
func TestIncrMath(t *testing.T) {
	tests := []struct {
		name          string
		counter, want uint8
		logFactor     int
		r             float64
	}{
		// p = 1/((10-5)*10+1) = 1/51 ≈ 0.019608
		{"just below p", 10, 11, 10, 0.0196},
		{"at p", 10, 10, 10, 1.0 / 51.0},
		{"above p", 10, 10, 10, 0.02},
		{"r of zero always increments", 200, 201, 10, 0},
		// at or below InitVal the base value clamps to 0, p = 1: certain
		{"fresh key certain", InitVal, InitVal + 1, 10, 0.999999},
		{"below InitVal certain", 0, 1, 10, 0.999999},
		{"zero logFactor p=1", 100, 101, 0, 0.999999},
		{"negative logFactor clamps to zero", 100, 101, -5, 0.999999},
		// saturation
		{"saturated stays", MaxVal, MaxVal, 10, 0},
		{"last step allowed", MaxVal - 1, MaxVal, 10, 0},
	}
	for _, tt := range tests {
		if got := incrWithR(tt.counter, tt.logFactor, tt.r); got != tt.want {
			t.Errorf("%s: incrWithR(%d, %d, %g) = %d, want %d",
				tt.name, tt.counter, tt.logFactor, tt.r, got, tt.want)
		}
	}
}

// TestIncrDeterministicPath exercises the public Incr at logFactor 0,
// where every draw succeeds — no RNG dependence, exact expectations.
func TestIncrDeterministicPath(t *testing.T) {
	c := NewMorrisCounter() // 5
	want := uint8(6)
	for i := 0; i < int(MaxVal-InitVal); i++ { // 250 certain increments to saturation
		if got := c.Incr(0); got != want {
			t.Fatalf("Incr(0) #%d = %d, want %d", i+1, got, want)
		}
		want++
	}
	if got := c.Incr(0); got != MaxVal {
		t.Errorf("Incr(0) past saturation = %d, want %d", got, MaxVal)
	}
	var z MorrisCounter // 0: increments are certain from below InitVal too
	if got := z.Incr(7); got != 1 {
		t.Errorf("zero-value Incr(7) = %d, want 1", got)
	}
}

// TestDecay pins the clamp arithmetic of LFUDecrAndReturn.
func TestDecay(t *testing.T) {
	tests := []struct {
		counter uint8
		periods int
		want    uint8
	}{
		{10, 3, 7},
		{10, 10, 0},
		{10, 11, 0},  // more periods than counter: saturate at 0
		{10, 300, 0}, // periods far above 255 must not wrap the uint8
		{10, 0, 10},  // no elapsed period: untouched
		{10, -3, 10}, // negative: no-op
		{0, 5, 0},
		{255, 254, 1},
	}
	for _, tt := range tests {
		if got := decayed(tt.counter, tt.periods); got != tt.want {
			t.Errorf("decayed(%d, %d) = %d, want %d", tt.counter, tt.periods, got, tt.want)
		}
	}
	c := NewMorrisCounter() // 5
	c.Incr(0)               // 6
	c.Incr(0)               // 7
	c.Incr(0)               // 8
	if got := c.Decay(2); got != 6 {
		t.Errorf("Decay(2) = %d, want 6", got)
	}
	if got := c.Value(); got != 6 {
		t.Errorf("Value after Decay = %d, want 6", got)
	}
}

// TestElapsedMinutes pins LFUTimeElapsed including the wrap boundary —
// mirrored exactly from evict.c, boundary quirks included.
func TestElapsedMinutes(t *testing.T) {
	tests := []struct {
		now, lastMin uint16
		want         int
	}{
		{100, 100, 0},
		{110, 100, 10},
		{0, 0, 0},
		{65535, 0, 65535},
		{5, 65530, 10}, // wrapped: 65535-65530+5 — the Redis arithmetic
		{0, 65535, 0},  // the wrap boundary reads as 0, not 1 (do not "fix")
		{1, 65535, 1},
	}
	for _, tt := range tests {
		if got := elapsedMinutes(tt.now, tt.lastMin); got != tt.want {
			t.Errorf("elapsedMinutes(%d, %d) = %d, want %d", tt.now, tt.lastMin, got, tt.want)
		}
	}
}

// TestLfuBasic walks the table contract with an injected clock at
// logFactor 0 (deterministic increments) and decayTime 1.
func TestLfuBasic(t *testing.T) {
	l, clk := newTestLfu[string](0, 1, 1000)

	// Unknown key: Counter/IdleMinutes not-found, Delete false.
	if v, ok := l.Counter("a"); ok || v != 0 {
		t.Errorf("Counter(unknown) = (%d, %v), want (0, false)", v, ok)
	}
	if m, ok := l.IdleMinutes("a"); ok || m != 0 {
		t.Errorf("IdleMinutes(unknown) = (%d, %v), want (0, false)", m, ok)
	}
	if l.Delete("a") {
		t.Errorf("Delete(unknown) = true, want false")
	}

	// Touch of an unknown key is Add-then-Touch: InitVal + 1.
	if got := l.Touch("a"); got != InitVal+1 {
		t.Errorf("first Touch = %d, want %d", got, InitVal+1)
	}
	if v, ok := l.Counter("a"); !ok || v != InitVal+1 {
		t.Errorf("Counter after first Touch = (%d, %v), want (%d, true)", v, ok, InitVal+1)
	}
	if m, ok := l.IdleMinutes("a"); !ok || m != 0 {
		t.Errorf("IdleMinutes = (%d, %v), want (0, true)", m, ok)
	}
	if l.Len() != 1 {
		t.Errorf("Len = %d, want 1", l.Len())
	}

	// At logFactor 0 every touch gains 1 while the clock holds still.
	for want := uint8(InitVal + 2); want <= InitVal+5; want++ {
		if got := l.Touch("a"); got != want {
			t.Fatalf("Touch = %d, want %d", got, want)
		}
	}

	// 3 idle minutes at decayTime 1: Counter (read-only) decays by 3.
	clk.advance(3)
	if v, ok := l.Counter("a"); !ok || v != InitVal+2 {
		t.Errorf("Counter after 3 idle minutes = (%d, %v), want (%d, true)", v, ok, InitVal+2)
	}
	if m, _ := l.IdleMinutes("a"); m != 3 {
		t.Errorf("IdleMinutes = %d, want 3", m)
	}

	// Touch applies the same decay then increments: 10-3+1 = 8.
	if got := l.Touch("a"); got != 8 {
		t.Errorf("Touch after decay = %d, want 8", got)
	}
	// The retimestamp resets the idle clock.
	if m, _ := l.IdleMinutes("a"); m != 0 {
		t.Errorf("IdleMinutes after Touch = %d, want 0", m)
	}

	// decayTime 2: decay steps are full 2-minute periods.
	l2, clk2 := newTestLfu[int](0, 2, 500)
	l2.Touch(1)     // 6
	clk2.advance(5) // 5 minutes = 2 periods
	if v, _ := l2.Counter(1); v != 4 {
		t.Errorf("decayTime 2, 5 minutes: Counter = %d, want 4", v)
	}

	// decayTime 0: no decay ever (Redis semantics).
	l3, clk3 := newTestLfu[int](0, 0, 500)
	l3.Touch(1) // 6
	clk3.advance(10000)
	if v, _ := l3.Counter(1); v != 6 {
		t.Errorf("decayTime 0: Counter = %d, want 6 (no decay)", v)
	}
	if m, _ := l3.IdleMinutes(1); m != 10000 {
		t.Errorf("decayTime 0 IdleMinutes = %d, want 10000", m)
	}

	// Add resets a hot key to InitVal; Delete removes it.
	l.Add("a")
	if v, _ := l.Counter("a"); v != InitVal {
		t.Errorf("Counter after Add reset = %d, want %d", v, InitVal)
	}
	if !l.Delete("a") {
		t.Errorf("Delete(present) = false, want true")
	}
	if _, ok := l.Counter("a"); ok {
		t.Errorf("Counter after Delete found")
	}
	if l.Len() != 0 {
		t.Errorf("Len after Delete = %d, want 0", l.Len())
	}

	// Truncate drops everything, the table stays usable.
	l.Touch("x")
	l.Touch("y")
	l.Truncate()
	if l.Len() != 0 {
		t.Errorf("Len after Truncate = %d, want 0", l.Len())
	}
	if _, ok := l.Counter("x"); ok {
		t.Errorf("Counter after Truncate found")
	}
	if got := l.Touch("z"); got != InitVal+1 {
		t.Errorf("Touch after Truncate = %d, want %d", got, InitVal+1)
	}
}

// TestLfuClockWrap drives the stored minute clock across the 65536
// boundary: decay and idle time must stay arithmetic, never corrupting
// a counter (the elapsed wrap computation reads the same key).
func TestLfuClockWrap(t *testing.T) {
	// 65530: five minutes before the wrap.
	l, clk := newTestLfu[string](0, 1, 65530)
	if got := l.Touch("a"); got != InitVal+1 {
		t.Fatalf("Touch = %d, want %d", got, InitVal+1)
	}
	clk.advance(4) // now 65534: still before the wrap
	if m, _ := l.IdleMinutes("a"); m != 4 {
		t.Errorf("IdleMinutes before wrap = %d, want 4", m)
	}
	clk.advance(2) // now 0 (wrapped).  Six real minutes elapsed, but the
	// 16-bit clock reads 65535-65530+0 = 5 — the wrap loses one minute
	// (LFUTimeElapsed's arithmetic; the boundary quirk pinned above).
	if m, _ := l.IdleMinutes("a"); m != 5 {
		t.Errorf("IdleMinutes across wrap = %d, want 5", m)
	}
	// 5 decay periods take the counter from 6 to 1, then this access's
	// certain increment makes 2.  Nothing corrupts; arithmetic continues.
	if got := l.Touch("a"); got != 2 {
		t.Errorf("Touch across wrap = %d, want 2", got)
	}
	if v, _ := l.Counter("a"); v != 2 {
		t.Errorf("Counter after wrap Touch = %d, want 2", v)
	}
}

// TestLfuNilTolerated checks the nil and zero-value reads.
func TestLfuNilTolerated(t *testing.T) {
	var zero Lfu[string]
	for _, l := range []*Lfu[string]{nil, &zero} {
		if v, ok := l.Counter("a"); ok || v != 0 {
			t.Errorf("Counter on %T = (%d, %v), want (0, false)", l, v, ok)
		}
		if m, ok := l.IdleMinutes("a"); ok || m != 0 {
			t.Errorf("IdleMinutes on %T = (%d, %v), want (0, false)", l, m, ok)
		}
		if l.Delete("a") {
			t.Errorf("Delete on %T = true, want false", l)
		}
		if n := l.Len(); n != 0 {
			t.Errorf("Len on %T = %d, want 0", l, n)
		}
		l.Truncate() // must not panic
		l.Lock()     // no-op lock parity
		l.Unlock()
	}
	var mc *MorrisCounter
	if v := mc.Value(); v != 0 {
		t.Errorf("Value on nil *MorrisCounter = %d, want 0", v)
	}
}

// TestLfuPanics checks the exact panic contract — four situations, each
// message naming the method and the fix.
func TestLfuPanics(t *testing.T) {
	var zero Lfu[string]
	expectPanicMessage(t, "NewLfuWithClock(nil clock)", "nil clock",
		func() { NewLfuWithClock[string](10, 1, nil) })
	expectPanicMessage(t, "Touch on nil", "Touch on a nil",
		func() { var l *Lfu[string]; l.Touch("a") })
	expectPanicMessage(t, "Touch on zero-value", "zero-value",
		func() { zero.Touch("a") })
	expectPanicMessage(t, "Add on nil", "Add on a nil",
		func() { var l *Lfu[string]; l.Add("a") })
	expectPanicMessage(t, "Add on zero-value", "zero-value",
		func() { zero.Add("a") })
	var mc *MorrisCounter
	expectPanicMessage(t, "Incr on nil counter", "Incr on a nil",
		func() { mc.Incr(10) })
	expectPanicMessage(t, "Decay on nil counter", "Decay on a nil",
		func() { mc.Decay(1) })
}

// expectPanicMessage checks that fx panics with a message containing
// `want` — the contract says each message names the method and the fix.
func expectPanicMessage(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
			t.Errorf("Expected the %s panic message to contain %q, got %v", name, want, r)
		}
	}()
	fx()
}
