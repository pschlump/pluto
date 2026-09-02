/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hyperloglog_ts implements the HyperLogLog cardinality
// estimator safe for concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/hyperloglog — the dense-register sketch
// behind Redis's PFADD/PFCOUNT/PFMERGE — with the identical API guarded
// by a sync.RWMutex, plus the Lock and Unlock pair and the Nl-prefixed
// (no-lock) methods for compound operations.  The constants and errors
// are aliases of the plain package's, so switching between the twins is
// an import change.
//
// Concurrency model:
//
// Writes (Add, Merge, Reset) take the write lock.  Count, IsEmpty and
// Bytes take the read lock.  Count may look like it mutates — it fills
// the cached estimate — but the plain package's cache fields are
// atomics precisely so this twin can call Count under the read lock:
// concurrent Counts race only on idempotent atomic stores, and an Add's
// invalidation happens-before its write-lock release, so a Count that
// acquires the read lock afterwards always recomputes.
//
// Merge snapshots every operand (a Clone under the operand's own read
// lock, released again) before taking this sketch's write lock and
// folding the snapshots in — the avl_tree_ts set-operation pattern: no
// nested locks, so concurrent merges in opposite directions cannot
// deadlock, and an operand may alias the destination (max(a, a) = a
// makes that a no-op).  An operand mutated concurrently is merged as
// its snapshot at the moment it was taken.
//
// The zero value of Hll is an empty sketch ready to use (including
// Add) — no constructor; the inner plain sketch is allocated lazily on
// the first write (reads delegate to the plain package's nil
// tolerance).  The plain struct is embedded by pointer, not by value:
// its atomics make value copies a go vet copylocks error.  A nil *Hll
// behaves as an empty sketch for every operation except Add, and Merge
// that would fold in a non-empty operand — those panic naming the
// method; the nil guards run before any lock acquisition.  These are
// the package's only panics.
//
// See the hyperloglog package documentation for the data-structure
// contracts (the fixed xxHash64, the register packing, the
// LinearCounting/Ertl estimator and the accuracy profile, the
// serialized form) — this twin changes only the concurrency.
//
// Run the tests with -race.
package hyperloglog_ts

import (
	"sync"

	"github.com/pschlump/pluto/hyperloglog"
)

// The plain package's constants and errors, re-exported for the same
// drop-in reason (an Hll serialized by either twin decodes in both).
const (
	Precision    = hyperloglog.Precision
	RegisterBits = hyperloglog.RegisterBits
	Registers    = hyperloglog.Registers
	DenseSize    = hyperloglog.DenseSize
)

// Errors reported by HllFromBytes on corrupt serialized data — compare
// with errors.Is across either twin.
var (
	ErrBadLength   = hyperloglog.ErrBadLength
	ErrBadRegister = hyperloglog.ErrBadRegister
)

// Hll is a HyperLogLog cardinality sketch guarded by a sync.RWMutex:
// the plain package's sketch behind a pointer plus the one lock (the
// stream_ts composition pattern — no locks inside the borrowed
// structure).  The zero value is an empty sketch ready to use; the
// inner sketch appears on the first write.  Do not copy an Hll (the
// inner atomics and the mutex must not be duplicated) — always use
// *Hll.
type Hll struct {
	h    *hyperloglog.Hll
	lock sync.RWMutex
}

// NewHll returns an empty Hll.  The zero value is identical and fully
// usable; the constructor is convenience and parity with the plain
// twin.
func NewHll() *Hll { return &Hll{h: hyperloglog.NewHll()} }

// ensure returns the inner sketch, allocating it on first use.  The
// caller must hold the write lock (or be inside an Nl* method).
func (h *Hll) ensure() *hyperloglog.Hll {
	if h.h == nil {
		h.h = hyperloglog.NewHll()
	}
	return h.h
}

// Add hashes v and updates the register it maps to when its rank beats
// the stored one, reporting whether any register changed — the signal
// PFADD replies with.  Complexity is O(len(v)) plus O(1).
// Add on a nil *Hll panics.
func (h *Hll) Add(v []byte) bool {
	if h == nil {
		panic("hyperloglog_ts: Add on a nil *Hll — create one with NewHll() or use a zero Hll value")
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	return h.ensure().Add(v)
}

// Count returns the estimated number of distinct elements added (or
// merged in).  It takes the read lock: the estimate cache inside the
// plain sketch is maintained with atomics, so concurrent Counts are
// race-free while a mutating Add still invalidates it under the write
// lock before this call can observe it.  The first Count after a change
// computes the estimate in O(m); later calls are O(1).  A nil *Hll (or
// a zero value never written) reports 0.
func (h *Hll) Count() uint64 {
	if h == nil {
		return 0
	}
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.h.Count() // nil-tolerant: a never-written inner reports 0
}

// Merge folds others into h with the register-wise maximum — the HLL
// union.  Every operand is snapshotted under its own read lock before
// h's write lock is taken (no nested locks; operands may alias h, which
// makes the fold a no-op for that operand).  Nil elements are treated
// as empty and skipped; merging only empty operands (or none) is a
// no-op, so a nil h tolerates it.  A nil h with a non-empty operand
// panics.  Complexity is O(k·m).
func (h *Hll) Merge(others ...*Hll) {
	// Snapshot the operands first, one read lock at a time.
	var snaps []*hyperloglog.Hll
	nonEmpty := false
	for _, o := range others {
		if o == nil || o == h {
			continue
		}
		o.lock.RLock()
		snap := o.h.Clone() // nil-tolerant: a never-written inner clones to nil
		o.lock.RUnlock()
		if snap == nil {
			continue
		}
		if !snap.IsEmpty() {
			nonEmpty = true
		}
		snaps = append(snaps, snap)
	}
	if !nonEmpty {
		return
	}
	if h == nil {
		panic("hyperloglog_ts: Merge on a nil *Hll — create one with NewHll() or use a zero Hll value")
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	inner := h.ensure()
	changed := false
	for _, snap := range snaps {
		if inner.MergeMax(snap) {
			changed = true
		}
	}
	if changed {
		inner.Invalidate()
	}
}

// Reset empties the sketch.  Complexity is O(m).  A nil *Hll (or a zero
// value never written) is a no-op.
func (h *Hll) Reset() {
	if h == nil {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.h != nil {
		h.h.Reset()
	}
}

// IsEmpty reports whether no element has been recorded.  Complexity is
// O(m) with an early exit.  A nil *Hll reports true.
func (h *Hll) IsEmpty() bool {
	if h == nil {
		return true
	}
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.h.IsEmpty() // nil-tolerant
}

// Bytes returns the dense serialized form (12288 bytes, a fresh copy
// owned by the caller) taken under the read lock.  A nil *Hll returns
// nil; a zero value never written returns the all-zero form of an
// empty sketch, like the plain package's.
func (h *Hll) Bytes() []byte {
	if h == nil {
		return nil
	}
	h.lock.RLock()
	defer h.lock.RUnlock()
	if h.h == nil {
		return make([]byte, DenseSize)
	}
	return h.h.Bytes()
}

// HllFromBytes decodes the dense serialized form produced by Bytes —
// the plain package's decoder with its validation, wrapped in the
// thread-safe type.  Corrupt input reports an error wrapping
// ErrBadLength or ErrBadRegister, never a panic.
func HllFromBytes(b []byte) (*Hll, error) {
	plain, err := hyperloglog.HllFromBytes(b)
	if err != nil {
		return nil, err
	}
	return &Hll{h: plain}, nil
}

// Lock takes the real write lock, for compound operations — the Nl*
// methods below run unlocked while it is held.  A nil *Hll no-ops.  Do
// not call a regular method while the lock is held (deadlock) — use the
// Nl* forms.
func (h *Hll) Lock() {
	if h == nil {
		return
	}
	h.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A nil *Hll no-ops.
func (h *Hll) Unlock() {
	if h == nil {
		return
	}
	h.lock.Unlock()
}

// NlAdd is the no-lock Add — call it only while holding Lock.
func (h *Hll) NlAdd(v []byte) bool { return h.ensure().Add(v) }

// NlCount is the no-lock Count — call it only while holding Lock.
func (h *Hll) NlCount() uint64 { return h.h.Count() }

// NlIsEmpty is the no-lock IsEmpty — call it only while holding Lock.
func (h *Hll) NlIsEmpty() bool { return h.h.IsEmpty() }

// NlReset is the no-lock Reset — call it only while holding Lock.
func (h *Hll) NlReset() {
	if h.h != nil {
		h.h.Reset()
	}
}

// NlBytes is the no-lock Bytes — call it only while holding Lock.
func (h *Hll) NlBytes() []byte { return h.h.Bytes() }

// NlMerge is the no-lock Merge — call it only while holding Lock on
// the destination; the operands must be quiet for the duration (their
// locks are not taken).
func (h *Hll) NlMerge(others ...*Hll) {
	changed := false
	for _, o := range others {
		if o != nil && h.ensure().MergeMax(o.h) {
			changed = true
		}
	}
	if changed {
		h.h.Invalidate()
	}
}
