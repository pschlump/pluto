/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts_test

import (
	"strings"
	"testing"
	"time"

	"github.com/pschlump/pluto/lfu_ts"
)

// fakeClock is a manually advanced clock for deterministic tests, the
// same shape as the plain package's test clock.
type fakeClock struct{ min int64 }

func (c *fakeClock) now() time.Time { return time.Unix(c.min*60, 0).UTC() }

// TestLfuTsMirror checks that the twin's semantics are the plain
// package's: with an injected clock and logFactor 0 every value is
// deterministic — Add-then-Touch on first access, certain increments,
// decay by full periods, retimestamp on touch.
func TestLfuTsMirror(t *testing.T) {
	clk := &fakeClock{min: 1000}
	l := lfu_ts.NewLfuWithClock[string](0, 1, clk.now)

	if got := l.Touch("a"); got != lfu_ts.InitVal+1 {
		t.Errorf("first Touch = %d, want %d", got, lfu_ts.InitVal+1)
	}
	for want := uint8(lfu_ts.InitVal + 2); want <= lfu_ts.InitVal+4; want++ {
		if got := l.Touch("a"); got != want {
			t.Fatalf("Touch = %d, want %d", got, want)
		}
	}
	clk.min += 3 // three idle minutes: one decay period each
	if v, ok := l.Counter("a"); !ok || v != lfu_ts.InitVal+1 {
		t.Errorf("Counter after 3 idle = (%d,%v), want (%d,true)", v, ok, lfu_ts.InitVal+1)
	}
	if m, _ := l.IdleMinutes("a"); m != 3 {
		t.Errorf("IdleMinutes = %d, want 3", m)
	}
	if got := l.Touch("a"); got != lfu_ts.InitVal+2 { // 9-3 decay, +1 increment
		t.Errorf("Touch after decay = %d, want %d", got, lfu_ts.InitVal+2)
	}

	l.Add("b") // reset/new key: InitVal
	if v, _ := l.Counter("b"); v != lfu_ts.InitVal {
		t.Errorf("Counter after Add = %d, want %d", v, lfu_ts.InitVal)
	}
	if l.Len() != 2 {
		t.Errorf("Len = %d, want 2", l.Len())
	}
	if !l.Delete("b") || l.Delete("b") {
		t.Errorf("Delete not idempotent-present-then-absent")
	}
	l.Truncate()
	if l.Len() != 0 {
		t.Errorf("Len after Truncate = %d, want 0", l.Len())
	}
}

// TestLfuTsConstants pins the aliasing: the twin's constants and the
// MorrisCounter type are the plain package's.
func TestLfuTsConstants(t *testing.T) {
	if lfu_ts.InitVal != 5 || lfu_ts.MaxVal != 255 {
		t.Errorf("InitVal/MaxVal = %d/%d, want 5/255", lfu_ts.InitVal, lfu_ts.MaxVal)
	}
	if lfu_ts.DefaultLogFactor != 10 || lfu_ts.DefaultDecayTime != 1 {
		t.Errorf("defaults = %d/%d, want 10/1", lfu_ts.DefaultLogFactor, lfu_ts.DefaultDecayTime)
	}
	c := &lfu_ts.MorrisCounter{}
	if got := c.Incr(0); got != 1 { // zero value, certain increment
		t.Errorf("aliased MorrisCounter Incr(0) = %d, want 1", got)
	}
	if c.Value() != 1 {
		t.Errorf("aliased MorrisCounter Value = %d, want 1", c.Value())
	}
}

// TestLfuTsNilAndZero checks the tolerated reads on nil and the zero
// value, before any lock acquisition.
func TestLfuTsNilAndZero(t *testing.T) {
	var zero lfu_ts.Lfu[string]
	for _, l := range []*lfu_ts.Lfu[string]{nil, &zero} {
		if v, ok := l.Counter("a"); ok || v != 0 {
			t.Errorf("Counter on %T = (%d,%v), want (0,false)", l, v, ok)
		}
		if m, ok := l.IdleMinutes("a"); ok || m != 0 {
			t.Errorf("IdleMinutes on %T = (%d,%v), want (0,false)", l, m, ok)
		}
		if l.Delete("a") {
			t.Errorf("Delete on %T = true, want false", l)
		}
		if n := l.Len(); n != 0 {
			t.Errorf("Len on %T = %d, want 0", l, n)
		}
		l.Truncate()
		l.Lock() // nil no-op form
		l.Unlock()
	}
}

// TestLfuTsPanics pins the twin's own panic contract — Touch/Add on a
// nil or zero-value table, messages prefixed lfu_ts:.
func TestLfuTsPanics(t *testing.T) {
	var zero lfu_ts.Lfu[string]
	expectPanicMessage(t, "Touch on nil", "lfu_ts: Touch on a nil",
		func() { var l *lfu_ts.Lfu[string]; l.Touch("a") })
	expectPanicMessage(t, "Touch on zero-value", "lfu_ts: Touch on a zero-value",
		func() { zero.Touch("a") })
	expectPanicMessage(t, "Add on nil", "lfu_ts: Add on a nil",
		func() { var l *lfu_ts.Lfu[string]; l.Add("a") })
	expectPanicMessage(t, "Add on zero-value", "lfu_ts: Add on a zero-value",
		func() { zero.Add("a") })
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
