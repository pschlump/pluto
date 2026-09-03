/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bloom_ts implements the Bloom filter safe for concurrent
// use.  It is the thread-safe twin of github.com/pschlump/pluto/bloom
// — the fixed-shape probabilistic set membership sketch — with the
// identical API guarded by one sync.RWMutex, plus the Lock and Unlock
// pair and the Nl-prefixed (no-lock) methods for compound operations.
// The serialized-data errors are aliases of the plain package's, so
// switching between the twins is an import change.
//
// Concurrency model:
//
// Writes (Add, TestAndSet, Merge, Reset) take the write lock.  Every
// read — MayContain, Count, Added, Saturation, IsEmpty, BitCount,
// HashCount, Bytes, String — takes the read lock (the plain package
// maintains its counters incrementally, so even Count is O(1) under it).
//
// Merge snapshots every operand (a Clone under the operand's own read
// lock, released again) before taking this filter's write lock and
// folding the snapshots in — the avl_tree_ts set-operation /
// hyperloglog_ts merge pattern: no nested locks, so concurrent merges
// in opposite directions cannot deadlock, and an operand may alias the
// destination (the OR of a set with itself is itself, so an aliasing
// operand is skipped).  An operand mutated concurrently is merged as
// its snapshot at the moment it was taken.
//
// Unlike the constructor-free twins, a Bloom filter's shape (m, k) must
// exist before the first write, so the zero value is not writable: Add,
// TestAndSet and a Merge that would fold in a non-empty operand panic
// naming the constructors (the lru_ts / lfu_ts precedent — no shape, no
// sane answer); the nil guards run before any lock acquisition.  Every
// read tolerates nil and the zero value (MayContain false, counters 0,
// IsEmpty true, Bytes nil), and a Merge of only empty operands is a
// tolerated no-op.  These are the package's only panics of its own; a
// Merge whose operand shape differs from the destination's panics with
// the plain package's message (the fold runs through the plain core).
//
// The compound the Nl* surface exists for: answer-then-record sequences
// that must see one consistent view — batch insert-if-absent across
// several elements, or admission control that consults Saturation and
// adds under one lock hold.  TestAndSet already answers and records
// atomically for a single element; the Nl* forms compose it with
// neighbors.
//
// See the bloom package documentation for the data-structure contracts
// (the frozen MurmurHash2/SuperFastHash pair, the Kirsch–Mitzenmacher
// probe construction, the false-positive math, the serialized form) —
// this twin changes only the concurrency.
//
// Run the tests with -race.
package bloom_ts

import (
	"sync"

	"github.com/pschlump/pluto/bloom"
)

// Errors reported by BloomFromBytes on corrupt serialized data — the
// plain package's sentinels, re-exported so twin-switching needs no
// second import and errors.Is works across either package.
var (
	ErrBadLength = bloom.ErrBadLength
	ErrBadHashes = bloom.ErrBadHashes
	ErrBadBits   = bloom.ErrBadBits
)

// Bloom is a Bloom filter guarded by one sync.RWMutex: the plain
// package's filter behind a pointer plus the lock (the lfu_ts / lru_ts
// composition pattern — no locks inside the borrowed structure).
// Create it with NewBloom, NewBloomBits or BloomFromBytes; the zero
// value tolerates every read but cannot be written into.  Do not copy a
// Bloom (the mutex must not be duplicated) — always use *Bloom, and
// Clone for a copy.
type Bloom struct {
	inner *bloom.Bloom
	lock  sync.RWMutex
}

// NewBloom returns a filter sized for n distinct elements with a
// false-positive rate of at most ~p (the plain package's constructor —
// its parameter validation panics apply unchanged).
// Complexity is O(1) plus the bit-array allocation.
func NewBloom(n int, p float64) *Bloom {
	return &Bloom{inner: bloom.NewBloom(n, p)}
}

// NewBloomBits returns a filter with an explicit shape: bits total and
// hashes probes per element (the plain package's constructor — its
// parameter validation panics apply unchanged).
// Complexity is O(1) plus the bit-array allocation.
func NewBloomBits(bits uint64, hashes int) *Bloom {
	return &Bloom{inner: bloom.NewBloomBits(bits, hashes)}
}

// Add records v and reports whether any probe bit was previously clear
// — the inverse of the pre-call MayContain.  It takes the write lock.
// Add on a nil or zero-value Bloom panics; the message names the
// constructors.
// Complexity is O(len(v)+k).
func (b *Bloom) Add(v []byte) bool {
	if b == nil {
		panic("bloom_ts: Add on a nil *Bloom — create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	if b.inner == nil {
		panic("bloom_ts: Add on a zero-value Bloom — no shape; create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.inner.Add(v)
}

// MayContain reports whether v's probe bits are all set (never false
// for an added element).  It takes the read lock.  A nil or zero-value
// Bloom reports false.
// Complexity is O(len(v)+k).
func (b *Bloom) MayContain(v []byte) bool {
	if b == nil {
		return false
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.MayContain(v) // nil-tolerant: a never-written inner reports false
}

// TestAndSet answers MayContain for v and records v in one pass — false
// marks the first sighting (and records it).  It takes the write lock.
// TestAndSet on a nil or zero-value Bloom panics; the message names the
// constructors.
// Complexity is O(len(v)+k).
func (b *Bloom) TestAndSet(v []byte) bool {
	if b == nil {
		panic("bloom_ts: TestAndSet on a nil *Bloom — create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	if b.inner == nil {
		panic("bloom_ts: TestAndSet on a zero-value Bloom — no shape; create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.inner.TestAndSet(v)
}

// Merge folds others into b with the bitwise OR — the lossless filter
// union.  Every operand is snapshotted under its own read lock before
// b's write lock is taken (no nested locks; an operand aliasing b is
// skipped — OR with itself is a no-op).  Nil and empty operands are
// skipped, so a nil b tolerates a merge of only empty operands.  A
// non-empty operand into a nil or zero-value b panics naming the
// constructors; a contributing operand whose (m, k) differs from b's
// panics with the plain package's shape message.  added is summed.
// Complexity is O(k·m/64) for k contributing operands.
func (b *Bloom) Merge(others ...*Bloom) {
	// Snapshot the contributing operands first, one read lock at a time.
	var snaps []*bloom.Bloom
	contributing := false
	for _, o := range others {
		if o == nil || o == b {
			continue
		}
		o.lock.RLock()
		snap := o.inner.Clone() // nil-tolerant: a never-written inner clones to nil
		o.lock.RUnlock()
		if snap == nil || snap.IsEmpty() {
			continue
		}
		contributing = true
		snaps = append(snaps, snap)
	}
	if !contributing {
		return
	}
	if b == nil || b.inner == nil {
		panic("bloom_ts: Merge on a nil or zero-value Bloom — create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.inner.Merge(snaps...)
}

// Clone returns an independent copy of the filter taken under the read
// lock — a point-in-time snapshot for handing to another goroutine or
// locking away before a Merge.  A nil or zero-value Bloom clones to
// nil.  Complexity is O(m/64).
func (b *Bloom) Clone() *Bloom {
	if b == nil {
		return nil
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	if b.inner == nil {
		return nil
	}
	return &Bloom{inner: b.inner.Clone()}
}

// Reset empties the filter.  Complexity is O(m/64).  A nil or zero-value
// Bloom is a no-op.
func (b *Bloom) Reset() {
	if b == nil {
		return
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.inner != nil {
		b.inner.Reset()
	}
}

// IsEmpty reports whether no bit is set.  It takes the read lock.  A
// nil or zero-value Bloom reports true.
// Complexity is O(1).
func (b *Bloom) IsEmpty() bool {
	if b == nil {
		return true
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.IsEmpty() // nil-tolerant
}

// Added returns the exact number of Add and TestAndSet calls.  It takes
// the read lock.  A nil or zero-value Bloom reports 0.
// Complexity is O(1).
func (b *Bloom) Added() uint64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Added() // nil-tolerant
}

// Count estimates the number of distinct elements from the fill ratio.
// It takes the read lock.  A nil or zero-value Bloom reports 0.
// Complexity is O(1).
func (b *Bloom) Count() uint64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Count() // nil-tolerant
}

// Saturation returns the fraction of bits set.  It takes the read lock.
// A nil or zero-value Bloom reports 0.
// Complexity is O(1).
func (b *Bloom) Saturation() float64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Saturation() // nil-tolerant
}

// BitCount returns m, the number of bits.  It takes the read lock.  A
// nil or zero-value Bloom reports 0.
// Complexity is O(1).
func (b *Bloom) BitCount() uint64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.BitCount()
}

// HashCount returns k, the probes per element.  It takes the read lock.
// A nil or zero-value Bloom reports 0.
// Complexity is O(1).
func (b *Bloom) HashCount() int {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.HashCount()
}

// Bytes returns the serialized form (the plain package's 24-byte header
// plus bit words) taken under the read lock, a fresh copy owned by the
// caller.  A nil or zero-value Bloom returns nil — unlike the
// constructor-free hyperloglog_ts there is no fixed shape to size an
// empty form.
// Complexity is O(m/64).
func (b *Bloom) Bytes() []byte {
	if b == nil {
		return nil
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Bytes() // nil-tolerant: a never-written inner returns nil
}

// String returns the one-line summary taken under the read lock.
func (b *Bloom) String() string {
	if b == nil {
		return "Bloom (empty)"
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.String()
}

// BloomFromBytes decodes the serialized form produced by either twin's
// Bytes — the plain package's decoder with its validation (errors wrap
// ErrBadLength, ErrBadHashes or ErrBadBits, never a panic), wrapped in
// the thread-safe type.
func BloomFromBytes(data []byte) (*Bloom, error) {
	plain, err := bloom.BloomFromBytes(data)
	if err != nil {
		return nil, err
	}
	return &Bloom{inner: plain}, nil
}

// Lock takes the real write lock, for compound operations — the Nl*
// methods below run unlocked while it is held.  A nil *Bloom no-ops.
// Do not call a regular method while the lock is held (deadlock) — use
// the Nl* forms.
func (b *Bloom) Lock() {
	if b == nil {
		return
	}
	b.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A nil *Bloom no-ops.
func (b *Bloom) Unlock() {
	if b == nil {
		return
	}
	b.lock.Unlock()
}

// NlAdd is the no-lock Add — call it only while holding Lock.
// It panics like Add on a zero-value Bloom (the bloom_ts message).
func (b *Bloom) NlAdd(v []byte) bool {
	if b.inner == nil {
		panic("bloom_ts: NlAdd on a zero-value Bloom — no shape; create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	return b.inner.Add(v)
}

// NlMayContain is the no-lock MayContain — call it only while holding
// Lock.
func (b *Bloom) NlMayContain(v []byte) bool { return b.inner.MayContain(v) }

// NlTestAndSet is the no-lock TestAndSet — call it only while holding
// Lock.  It panics like TestAndSet on a zero-value Bloom.
func (b *Bloom) NlTestAndSet(v []byte) bool {
	if b.inner == nil {
		panic("bloom_ts: NlTestAndSet on a zero-value Bloom — no shape; create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	return b.inner.TestAndSet(v)
}

// NlCount is the no-lock Count — call it only while holding Lock.
func (b *Bloom) NlCount() uint64 { return b.inner.Count() }

// NlAdded is the no-lock Added — call it only while holding Lock.
func (b *Bloom) NlAdded() uint64 { return b.inner.Added() }

// NlSaturation is the no-lock Saturation — call it only while holding
// Lock.
func (b *Bloom) NlSaturation() float64 { return b.inner.Saturation() }

// NlIsEmpty is the no-lock IsEmpty — call it only while holding Lock.
func (b *Bloom) NlIsEmpty() bool { return b.inner.IsEmpty() }

// NlReset is the no-lock Reset — call it only while holding Lock.
func (b *Bloom) NlReset() {
	if b.inner != nil {
		b.inner.Reset()
	}
}

// NlBytes is the no-lock Bytes — call it only while holding Lock.
func (b *Bloom) NlBytes() []byte { return b.inner.Bytes() }

// NlMerge is the no-lock Merge — call it only while holding Lock on the
// destination; the operands must be quiet for the duration (their locks
// are not taken — clone them with Clone first if they are shared).
func (b *Bloom) NlMerge(others ...*Bloom) {
	if b.inner == nil {
		panic("bloom_ts: NlMerge on a zero-value Bloom — no shape; create it with NewBloom, NewBloomBits or BloomFromBytes")
	}
	for _, o := range others {
		if o == nil || o == b {
			continue
		}
		if o.inner != nil && !o.inner.IsEmpty() {
			b.inner.Merge(o.inner)
		}
	}
}
