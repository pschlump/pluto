/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts

import (
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/pluto/lfu"
)

// The twin renders exactly what the plain package renders: the keys as
// sorted formatted strings.  The plain table walks its hash slots, the
// twin walks its first-insert key list, and sorting erases the
// difference — even with the keys added in a different order.
func TestCapabilitiesStringParity(t *testing.T) {
	plain := lfu.NewLfu[string](lfu.DefaultLogFactor, lfu.DefaultDecayTime)
	for _, k := range []string{"pear", "fig", "apple", "date"} {
		plain.Add(k)
	}
	twin := NewLfu[string](lfu.DefaultLogFactor, lfu.DefaultDecayTime)
	for _, k := range []string{"date", "apple", "pear", "fig"} { // different order
		twin.Add(k)
	}
	plain.Touch("fig")
	twin.Touch("pear")
	want := plain.String()
	if got := twin.String(); got != want || got != "[apple date fig pear]" {
		t.Errorf("twin String = %q, plain String = %q, want both %q", got, want, "[apple date fig pear]")
	}
	twin.Delete("fig")
	if got := twin.String(); got != "[apple date pear]" {
		t.Errorf("twin String after Delete = %q, want %q", got, "[apple date pear]")
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
	if got := NewLfu[string](lfu.DefaultLogFactor, lfu.DefaultDecayTime).String(); got != "[]" {
		t.Errorf("empty table String = %q, want %q", got, "[]")
	}
}

// The StringFunc callback runs OUTSIDE the read lock: it may call back
// into the table, including a mutating method — under the lock that
// would deadlock, so completing this test is the proof.
func TestCapabilitiesStringFuncCallbackUnlocked(t *testing.T) {
	l := NewLfu[int](lfu.DefaultLogFactor, lfu.DefaultDecayTime)
	for _, k := range []int{3, 1, 2} {
		l.Add(k)
	}
	saw := 0
	got := l.StringFunc(func(key int) string {
		l.Add(key + 100) // a write from inside the callback
		saw = l.Len()
		return fmt.Sprint(key)
	})
	if got != "[1 2 3]" {
		t.Errorf("StringFunc = %q, want %q", got, "[1 2 3]")
	}
	if saw != 6 {
		t.Errorf("Len seen from callback = %d, want 6", saw)
	}
}

// The race trio for the rendering: concurrent writers, a concurrent
// reader of String, a concurrent StringFunc — run under -race.
func TestCapabilitiesStringRace(t *testing.T) {
	l := NewLfu[int](lfu.DefaultLogFactor, lfu.DefaultDecayTime)
	for i := range 32 {
		l.Add(i)
	}
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 200 {
				l.Add((g*200 + i) % 40)
				l.Touch(i % 32)
				l.Delete((g + i) % 48)
			}
		}(g)
	}
	wg.Add(2)
	stringBytes, funcBytes := 0, 0
	go func() {
		defer wg.Done()
		for range 200 {
			stringBytes += len(l.String()) // always at least "[]" — the point is the race read
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			funcBytes += len(l.StringFunc(func(key int) string { return fmt.Sprint(key) }))
		}
	}()
	wg.Wait()
	if stringBytes < 400 || funcBytes < 400 { // 200 renders x at least "[]" (2 bytes) each
		t.Errorf("render bytes: String %d, StringFunc %d — want at least 400 each", stringBytes, funcBytes)
	}
}
