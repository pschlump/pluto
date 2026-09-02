/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lfu_ts implements the LFU frequency counters safe for
// concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/lfu — the Redis LFU eviction counters
// (8-bit Morris counter + 16-bit last-access-minute per key) — with
// the identical API guarded by one sync.RWMutex, plus the Lock and
// Unlock pair and the Nl-prefixed (no-lock) methods for compound
// operations.  MorrisCounter and the constants are aliases of the
// plain package's, so switching between the twins is an import change.
//
// Concurrency model:
//
// Writes (Touch, Add, Delete, Truncate) take the write lock.  Counter,
// IdleMinutes and Len take the read lock.  The underlying plain table
// is borrowed whole (the stream_ts / hyperloglog_ts composition
// pattern — no locks inside the borrowed structure), and Ultima-style
// callers run inside shard owner-goroutines, so the one lock is
// deliberately simple (the request note's guidance).
//
// The eviction scan is the compound the Nl* surface exists for:
// sampling candidate keys and comparing their frequencies must see one
// consistent view, so it runs as Lock + NlCounter... + Unlock.
//
// A nil *Lfu and the zero value behave as an empty table for every
// read (Counter and IdleMinutes report not-found, Delete returns
// false, Len is 0, Truncate does nothing); the nil guards run before
// any lock acquisition.  Touch and Add have no sane answer on a nil or
// zero-value table — those panic naming the method; the zero value
// cannot fabricate the configuration the constructors carry.  These
// are the package's only panics.
//
// See the lfu package documentation for the counter contracts (the
// LFULogIncr math, decay-from-elapsed-time, the 16-bit minute clock
// and its wrap arithmetic) — this twin changes only the concurrency.
//
// Run the tests with -race.
package lfu_ts

import (
	"sync"
	"time"

	"github.com/pschlump/pluto/lfu"
)

// MorrisCounter is the plain package's standalone 8-bit logarithmic
// counter, re-exported so twin-switching needs no second import.  Its
// methods are unchanged — a counter shared across goroutines still
// needs the caller's lock (Incr/Decay mutate the byte).
type MorrisCounter = lfu.MorrisCounter

// The plain package's constants, re-exported for the same drop-in
// reason.
const (
	InitVal          = lfu.InitVal
	MaxVal           = lfu.MaxVal
	DefaultLogFactor = lfu.DefaultLogFactor
	DefaultDecayTime = lfu.DefaultDecayTime
)

// Lfu is a table of per-key LFU frequency counters guarded by one
// sync.RWMutex: the plain package's table behind a pointer plus the
// lock.  Create it with NewLfu or NewLfuWithClock; the zero value
// reads as an empty table but cannot be written into.  Do not copy an
// Lfu (the mutex must not be duplicated) — always use *Lfu.
type Lfu[K comparable] struct {
	inner *lfu.Lfu[K]
	lock  sync.RWMutex
}

// NewLfu creates a table with an explicit lfu-log-factor and
// lfu-decay-time in minutes, using the wall clock — the two
// maxmemory-policy knobs Redis exposes.  Pass DefaultLogFactor and
// DefaultDecayTime for Redis's own settings.
// Complexity is O(1) plus the initial table allocation.
func NewLfu[K comparable](logFactor, decayTimeMinutes int) *Lfu[K] {
	return &Lfu[K]{inner: lfu.NewLfu[K](logFactor, decayTimeMinutes)}
}

// NewLfuWithClock creates a table with an explicit lfu-log-factor,
// lfu-decay-time and injected clock (the plain package's
// NewLfuWithClock — every operation consults the clock, so a fake
// clock makes the table deterministic in tests).
// Complexity is O(1) plus the initial table allocation.
func NewLfuWithClock[K comparable](logFactor, decayTimeMinutes int, clock func() time.Time) *Lfu[K] {
	return &Lfu[K]{inner: lfu.NewLfuWithClock[K](logFactor, decayTimeMinutes, clock)}
}

// Touch records one access of key — decay by elapsed periods, then the
// probabilistic increment, then retimestamp; it returns the new
// counter value (a first-ever Touch returns InitVal+1; see the plain
// package's Touch).  It takes the write lock.
//
// Touch panics on a nil or zero-value table; the message names the
// constructors.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Touch(key K) uint8 {
	if l == nil {
		panic("lfu_ts: Touch on a nil *Lfu — a nil table cannot record an access; create it with NewLfu or NewLfuWithClock")
	}
	if l.inner == nil {
		panic("lfu_ts: Touch on a zero-value Lfu — no configuration; create the table with NewLfu or NewLfuWithClock")
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.inner.Touch(key)
}

// Add inserts key with a fresh counter at InitVal and the current
// minute; adding a known key resets it.  It takes the write lock.
//
// Add panics on a nil or zero-value table; the message names the
// constructors.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Add(key K) {
	if l == nil {
		panic("lfu_ts: Add on a nil *Lfu — a nil table cannot record an entry; create it with NewLfu or NewLfuWithClock")
	}
	if l.inner == nil {
		panic("lfu_ts: Add on a zero-value Lfu — no configuration; create the table with NewLfu or NewLfuWithClock")
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	l.inner.Add(key)
}

// Counter returns the decay-adjusted frequency of key — the OBJECT
// FREQ / eviction view, a pure read.  It takes the read lock.  Unknown
// keys and nil or zero-value tables report (0, false).
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Counter(key K) (uint8, bool) {
	if l == nil || l.inner == nil {
		return 0, false
	}
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.inner.Counter(key)
}

// IdleMinutes returns the minutes elapsed since the key's last Touch
// or Add.  It takes the read lock.  Unknown keys and nil or zero-value
// tables report (0, false).
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) IdleMinutes(key K) (int, bool) {
	if l == nil || l.inner == nil {
		return 0, false
	}
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.inner.IdleMinutes(key)
}

// Delete removes the key's entry, reporting whether it was present,
// under the write lock.  A nil or zero-value table reports false.
// Complexity is O(1) average, O(n) worst case (hash_grow).
func (l *Lfu[K]) Delete(key K) bool {
	if l == nil || l.inner == nil {
		return false
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.inner.Delete(key)
}

// Len returns the number of keys with counters, under the read lock.
// A nil or zero-value table has length 0.
// Complexity is O(1).
func (l *Lfu[K]) Len() int {
	if l == nil || l.inner == nil {
		return 0
	}
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.inner.Len()
}

// Truncate drops every key's counter under the write lock, keeping the
// configuration.  A nil or zero-value table does nothing.
// Complexity is O(n).
func (l *Lfu[K]) Truncate() {
	if l == nil || l.inner == nil {
		return
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	l.inner.Truncate()
}

// Lock takes the real write lock, for compound operations — the Nl*
// methods below run unlocked while it is held.  The eviction scan is
// the canonical one: Lock, NlCounter over the sampled candidates,
// Unlock, so the frequencies compared come from one instant.  A nil
// *Lfu no-ops.  Do not call a regular method while the lock is held
// (deadlock) — use the Nl* forms.
func (l *Lfu[K]) Lock() {
	if l == nil {
		return
	}
	l.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A nil *Lfu no-ops.
func (l *Lfu[K]) Unlock() {
	if l == nil {
		return
	}
	l.lock.Unlock()
}

// NlTouch is the no-lock Touch — call it only while holding Lock.
// Complexity is O(1) average.
func (l *Lfu[K]) NlTouch(key K) uint8 { return l.inner.Touch(key) }

// NlAdd is the no-lock Add — call it only while holding Lock.
// Complexity is O(1) average.
func (l *Lfu[K]) NlAdd(key K) { l.inner.Add(key) }

// NlCounter is the no-lock Counter — call it only while holding Lock.
// Complexity is O(1) average.
func (l *Lfu[K]) NlCounter(key K) (uint8, bool) { return l.inner.Counter(key) }

// NlIdleMinutes is the no-lock IdleMinutes — call it only while
// holding Lock.
// Complexity is O(1) average.
func (l *Lfu[K]) NlIdleMinutes(key K) (int, bool) { return l.inner.IdleMinutes(key) }

// NlDelete is the no-lock Delete — call it only while holding Lock.
// Complexity is O(1) average.
func (l *Lfu[K]) NlDelete(key K) bool { return l.inner.Delete(key) }

// NlLen is the no-lock Len — call it only while holding Lock.
// Complexity is O(1).
func (l *Lfu[K]) NlLen() int { return l.inner.Len() }

// NlTruncate is the no-lock Truncate — call it only while holding
// Lock.
// Complexity is O(n).
func (l *Lfu[K]) NlTruncate() { l.inner.Truncate() }
