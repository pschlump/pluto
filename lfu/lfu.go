/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lfu implements the frequency counters behind Redis's LFU
// (least-frequently-used) maxmemory eviction (LFUGetTimeInMinutes/
// LFUTimeElapsed/LFULogIncr in note/redis/src/evict.c; updateLFU and
// LFUDecrAndReturn in note/redis/src/db.c), built for the Ultima
// Redis-clone's allkeys-lfu / volatile-lfu policies and the OBJECT FREQ
// command (request note/05-lfu-counter.md).  LRU is covered by pluto's
// lru; this is its frequency counterpart.
//
// Two layers:
//
//	MorrisCounter — the standalone 8-bit logarithmic counter with
//	    Incr (the Redis LFULogIncr math exactly) and Decay.
//	Lfu[K] — the keyed table mapping each key to a counter plus a
//	    16-bit "last access minute" (the Redis LDT).  Touch is
//	    updateLFU: time-based decay, then the probabilistic
//	    increment, then retimestamp.  The backing store is pluto
//	    hash_grow (intra-pluto composition), so operations are O(1)
//	    average and the table grows with the keyspace.
//
// Core operations:
//
//	Touch — record an access: decay by elapsed periods, then Incr.	O(1) average
//	Add — insert a key at InitVal with the current minute.			O(1) average
//	Counter — the decay-adjusted frequency (OBJECT FREQ), read-only.	O(1) average
//	IdleMinutes — minutes since the last Touch/Add.					O(1) average
//	Delete / Len / Truncate — the table plumbing.					O(1) / O(1) / O(n)
//
// A nil *Lfu and the zero value behave as an empty table for every read
// (Counter and IdleMinutes report not-found, Delete returns false, Len
// is 0, Truncate does nothing).  The package panics in exactly four
// situations, all programmer errors that cannot be handled where they
// occur — each message names the fix:
//
//	NewLfuWithClock with a nil clock — caught at construction.
//	Touch / Add on a nil table — a nil table cannot record an access.
//	Touch / Add on a zero-value table — no configuration; the message names the constructors.
//
// Not safe for concurrent use; the mutex-guarded twin lfu_ts has the
// same interface.
package lfu

import (
	"hash/maphash"
	"math/rand/v2"
	"time"

	"github.com/pschlump/pluto/hash_grow"
)

// DefaultLogFactor and DefaultDecayTime are Redis's lfu-log-factor /
// lfu-decay-time defaults (config.c): a counter at default settings
// reaches ~145 in one million accesses, and loses one count per minute
// of idleness.
const (
	DefaultLogFactor = 10
	DefaultDecayTime = 1 // minutes
)

// lfuEntry is one key's frequency state.  The eq/hash closures below
// read only the key field (the map pattern — the sharded_hash_ts
// precedent): counter and lastMin are satellite data the table never
// compares.
type lfuEntry[K comparable] struct {
	key     K
	counter uint8  // the Morris counter, decays over time
	lastMin uint16 // LDT: the 16-bit minute of the last Touch/Add
}

// Lfu is a table of per-key LFU frequency counters.  Use NewLfu with
// explicit parameters (Redis's defaults are DefaultLogFactor and
// DefaultDecayTime), or NewLfuWithClock to also inject the clock.  The
// zero value reads as an empty table but cannot be written into.
type Lfu[K comparable] struct {
	tab *hash_grow.HashTab[lfuEntry[K]]

	logFactor int // the lfu-log-factor; ≤ 0 makes every Incr certain
	decayTime int // minutes per decay step; 0 (or less) = never decay
	clock     func() time.Time
}

// NewLfu creates a table with an explicit lfu-log-factor and
// lfu-decay-time in minutes, using the wall clock — the two
// maxmemory-policy knobs Redis exposes.  Pass DefaultLogFactor and
// DefaultDecayTime for Redis's own settings.
// Complexity is O(1) plus the initial table allocation.
func NewLfu[K comparable](logFactor, decayTimeMinutes int) *Lfu[K] {
	return NewLfuWithClock[K](logFactor, decayTimeMinutes, time.Now)
}

// NewLfuWithClock creates a table with an explicit lfu-log-factor,
// lfu-decay-time (in minutes; 0 or less disables decay — Redis
// semantics) and clock.  The clock is consulted on every Touch, Add,
// Counter and IdleMinutes call, so an injected fake clock makes all of
// them deterministic in tests.  logFactor ≤ 0 makes every increment
// certain (the counter becomes a plain saturating count) — the Redis
// math at factor 0.  It panics on a nil clock.
// Complexity is O(1) plus the initial table allocation.
func NewLfuWithClock[K comparable](logFactor, decayTimeMinutes int, clock func() time.Time) *Lfu[K] {
	if clock == nil {
		panic("lfu: NewLfuWithClock called with a nil clock function")
	}
	seed := maphash.MakeSeed()
	tab := hash_grow.NewHashTabFunc(
		func(a, b lfuEntry[K]) bool { return a.key == b.key },
		func(e lfuEntry[K]) uint64 { return maphash.Comparable(seed, e.key) },
		16, 0,
	)
	return &Lfu[K]{
		tab:       tab,
		logFactor: logFactor,
		decayTime: decayTimeMinutes,
		clock:     clock,
	}
}

// minutesNow is LFUGetTimeInMinutes: the current time as minutes since
// the epoch, truncated to the low 16 bits (wrapping every 65536 minutes
// ≈ 45.5 days).
func (l *Lfu[K]) minutesNow() uint16 {
	return uint16(l.clock().Unix() / 60)
}

// Touch records one access of key — Redis updateLFU: the stored counter
// is first decayed by the number of full decay periods elapsed since
// the stored last-access minute (the decay is recomputed from elapsed
// time each access, never accumulated), then incremented with the
// Morris probability, and the entry is stored back with the current
// minute.  Touch of an unknown key is Add-then-Touch: the counter
// starts at InitVal and this access's increment lands on top of it
// (certain while the counter is at or below InitVal), so a first-ever
// Touch returns InitVal+1.  It returns the new counter value.
//
// Touch panics on a nil or zero-value table; the message names the
// constructors.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Touch(key K) uint8 {
	if l == nil {
		panic("lfu: Touch on a nil *Lfu — a nil table cannot record an access; create it with NewLfu or NewLfuWithClock")
	}
	if l.tab == nil {
		panic("lfu: Touch on a zero-value Lfu — no configuration; create the table with NewLfu or NewLfuWithClock")
	}
	now := l.minutesNow()
	counter := InitVal
	if e, ok := l.tab.Search(lfuEntry[K]{key: key}); ok {
		counter = e.counter
		if l.decayTime > 0 {
			counter = decayed(counter, elapsedMinutes(now, e.lastMin)/l.decayTime)
		}
	}
	counter = incrWithR(counter, l.logFactor, rand.Float64())
	l.tab.Insert(lfuEntry[K]{key: key, counter: counter, lastMin: now})
	return counter
}

// Add inserts key with a fresh counter at InitVal and the current
// minute — the state a newly created Redis key starts in.  Adding a
// known key resets it (a Redis SET replaces the object, resetting its
// LFU counter).
//
// Add panics on a nil or zero-value table; the message names the
// constructors.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Add(key K) {
	if l == nil {
		panic("lfu: Add on a nil *Lfu — a nil table cannot record an entry; create it with NewLfu or NewLfuWithClock")
	}
	if l.tab == nil {
		panic("lfu: Add on a zero-value Lfu — no configuration; create the table with NewLfu or NewLfuWithClock")
	}
	l.tab.Insert(lfuEntry[K]{key: key, counter: InitVal, lastMin: l.minutesNow()})
}

// Counter returns the decay-adjusted frequency of key — exactly what
// Redis's OBJECT FREQ reports and what eviction should compare
// (LFUDecrAndReturn): the stored counter minus the decay periods
// elapsed since the last Touch, computed for this read and not stored
// back.  Unknown keys report (0, false).  A nil or zero-value table
// reports (0, false).
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Counter(key K) (uint8, bool) {
	if l == nil || l.tab == nil || l.clock == nil {
		return 0, false
	}
	e, ok := l.tab.Search(lfuEntry[K]{key: key})
	if !ok {
		return 0, false
	}
	counter := e.counter
	if l.decayTime > 0 {
		counter = decayed(counter, elapsedMinutes(l.minutesNow(), e.lastMin)/l.decayTime)
	}
	return counter, true
}

// IdleMinutes returns the minutes elapsed since the key's last Touch or
// Add, from the stored 16-bit minute clock — raw elapsed minutes, not
// divided by the decay time.  Unknown keys report (0, false).  A nil or
// zero-value table reports (0, false).  A key idle across the 65536
// minute (~45.5 day) clock wrap is read by the wraparound arithmetic,
// which cannot see more than 65535 minutes — see the README.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) IdleMinutes(key K) (int, bool) {
	if l == nil || l.tab == nil || l.clock == nil {
		return 0, false
	}
	e, ok := l.tab.Search(lfuEntry[K]{key: key})
	if !ok {
		return 0, false
	}
	return elapsedMinutes(l.minutesNow(), e.lastMin), true
}

// Delete removes the key's entry, reporting whether it was present.  A
// nil or zero-value table reports false.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Delete(key K) bool {
	if l == nil || l.tab == nil {
		return false
	}
	return l.tab.Delete(lfuEntry[K]{key: key})
}

// Len returns the number of keys with counters.  A nil or zero-value
// table has length 0.
// Complexity is O(1).
func (l *Lfu[K]) Len() int {
	if l == nil || l.tab == nil {
		return 0
	}
	return l.tab.Len()
}

// Truncate drops every key's counter, keeping the configuration.  A nil
// or zero-value table does nothing.
// Complexity is O(n).
func (l *Lfu[K]) Truncate() {
	if l == nil || l.tab == nil {
		return
	}
	l.tab.Truncate()
}

// Lock and Unlock are no-ops kept so code written against the lfu_ts
// twin compiles unchanged.  The plain package is not safe for
// concurrent use.
func (l *Lfu[K]) Lock() {}

// Unlock is the other half of the no-op lock pair — see Lock.
func (l *Lfu[K]) Unlock() {}
