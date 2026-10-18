/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import (
	"fmt"
	"testing"
	"time"
)

// String renders the KEYS as sorted formatted strings — the hash-family
// rule, because the backing table's slot order is per-table random.
func TestCapabilitiesString(t *testing.T) {
	l := NewLfu[string](DefaultLogFactor, DefaultDecayTime)
	l.Add("pear")
	l.Add("fig")
	l.Add("apple")
	if got := l.String(); got != "[apple fig pear]" {
		t.Errorf("String = %q, want %q", got, "[apple fig pear]")
	}
	// Frequency churn is invisible: the rendering is keys-only.
	l.Touch("pear")
	l.Touch("pear")
	l.Touch("pear")
	if got := l.String(); got != "[apple fig pear]" {
		t.Errorf("String after Touch churn = %q, want %q", got, "[apple fig pear]")
	}
	// Delete removes the key from the rendering.
	l.Delete("fig")
	if got := l.String(); got != "[apple pear]" {
		t.Errorf("String after Delete = %q, want %q", got, "[apple pear]")
	}
}

// Two separately constructed tables holding the same keys render the
// identical string — the determinism pin for the random-seed risk (the
// tables' slot orders differ; the sorted rendering does not).
func TestCapabilitiesStringDeterminism(t *testing.T) {
	build := func() *Lfu[int] {
		l := NewLfu[int](DefaultLogFactor, DefaultDecayTime)
		for _, k := range []int{7, 3, 11, 1, 5} {
			l.Add(k)
		}
		return l
	}
	first, second := build().String(), build().String()
	// Lexicographic on the formatted strings — the house rule that
	// keeps String total for unordered key types ("11" sorts before "3").
	if first != second || first != "[1 11 3 5 7]" {
		t.Errorf("determinism: %q vs %q, want both %q", first, second, "[1 11 3 5 7]")
	}
}

// The rendering is time-invariant: counters decay with the clock, but
// String does not consult it (the stored state renders, not the
// decay-adjusted view).
func TestCapabilitiesStringClockInvariant(t *testing.T) {
	base := time.Unix(0, 0)
	now := base
	l := NewLfuWithClock[string](DefaultLogFactor, 1, func() time.Time { return now })
	l.Add("k")
	l.Touch("k")
	before := l.String()
	now = base.Add(3 * time.Hour) // 180 decay minutes later
	if got := l.String(); got != before || got != "[k]" {
		t.Errorf("String across the clock: before %q, after %q — want equal %q", before, got, "[k]")
	}
}

// nil and empty tables render as "[]"; a nil format function too.
func TestCapabilitiesStringNilEmpty(t *testing.T) {
	var nilTable *Lfu[string]
	if got := nilTable.String(); got != "[]" {
		t.Errorf("nil table String = %q, want %q", got, "[]")
	}
	if got := nilTable.StringFunc(nil); got != "[]" {
		t.Errorf("nil table StringFunc(nil) = %q, want %q", got, "[]")
	}
	if got := NewLfu[string](DefaultLogFactor, DefaultDecayTime).String(); got != "[]" {
		t.Errorf("empty table String = %q, want %q", got, "[]")
	}
	if got := (&Lfu[string]{}).String(); got != "[]" {
		t.Errorf("zero-value table String = %q, want %q", got, "[]")
	}
}

// StringFunc formats each key with the caller's function, still sorted;
// String is the fmt.Sprint specialization; the table is a fmt.Stringer.
func TestCapabilitiesStringFunc(t *testing.T) {
	l := NewLfu[int](DefaultLogFactor, DefaultDecayTime)
	for _, k := range []int{4, 2, 9} {
		l.Add(k)
	}
	got := l.StringFunc(func(key int) string { return fmt.Sprintf("#%d", key) })
	if got != "[#2 #4 #9]" {
		t.Errorf("StringFunc = %q, want %q", got, "[#2 #4 #9]")
	}
	if got := l.String(); got != "[2 4 9]" {
		t.Errorf("String = %q, want %q", got, "[2 4 9]")
	}
	if got := fmt.Sprintf("%v", l); got != "[2 4 9]" {
		t.Errorf("%%v = %q, want %q (fmt.Stringer)", got, "[2 4 9]")
	}
}
