/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hyperloglog implements the HyperLogLog cardinality estimator
// (Flajolet, Fusy, Gandouet, Meunier) in the dense-register form Redis
// uses for its PFADD/PFCOUNT/PFMERGE commands (note/redis/src/
// hyperloglog.c lineage; built for the Ultima PF* command family,
// note/04-hyperloglog.md): 2^14 = 16384 six-bit registers packed into a
// 12 KiB array, a fixed 64-bit xxHash of each element, LinearCounting
// for small cardinalities and Ertl's corrected estimator above it — the
// estimator current Redis ships.
//
// The structure is non-generic (elements are byte strings hashed on the
// way in, the union_find-over-indices / suffix_array-over-strings
// precedent): a sketch approximates the count of *distinct* elements and
// never stores one.  Elements are never boxed, compared or kept — Add
// hashes, updates one register, and forgets.
//
// The zero value of Hll is an empty sketch ready to use (including Add)
// — no constructor is needed; NewHll exists for note parity and
// readability.  Like the constraint-free containers (queue, stack, the
// tries, stream) there is no comparison or equality function to store:
// the register index and rank come from the fixed hash.  A nil *Hll
// behaves as an empty sketch for every operation with a sane answer
// (Count reports 0, Bytes nil, IsEmpty true, Reset and Merge with only
// empty operands are no-ops).  Exactly two calls have no sane answer and
// panic naming the method: Add on a nil *Hll (nowhere to record the
// element) and Merge into a nil *Hll that would fold in a non-empty
// operand (nowhere to record the union).  HllFromBytes reports corrupt
// serialized data as errors (ErrBadLength, ErrBadRegister), never a
// panic.
//
// Accuracy: standard error ≈ 1.04/√16384 ≈ 0.81% for cardinalities
// above the LinearCounting threshold (~2.5m ≈ 41k); below it the
// LinearCounting estimate is near-exact.  Merge is the register-wise
// maximum, the lossless HLL union.  The serialized form (Bytes) is the
// raw 12288-byte register array; it stays valid across versions because
// the hash, precision and packing are frozen constants.
//
// Core operations:
//
//	Add — Hash v, update one register.  O(len(v)) — true if any register changed.	O(1) registers
//	Count — Estimated number of distinct elements; cached after the first call.	O(m) then O(1)
//	Merge — Register-wise max union with the others (they are unchanged).	O(k·m)
//	Reset — Empty the sketch.																			O(m)
//	Bytes / HllFromBytes — Serialize / validate-and-decode the dense form.								O(m)
//	IsEmpty — Every register zero.																		O(m)
//
// Hll is not safe for concurrent use; the mutex-guarded twin
// hyperloglog_ts has the same interface.
package hyperloglog

import (
	"errors"
	"fmt"
	"math/bits"
	"sync/atomic"
)

// Precision is log₂ of the number of registers: m = 1<<Precision = 16384,
// the Redis HLL_P.  The low Precision bits of each element hash select
// the register; the rest decide the rank.
const Precision = 14

// RegisterBits is the width of one register: 6 bits, so the maximum
// storable rank is 63 (Add can produce at most rankMax).
const RegisterBits = 6

// Registers is the number of registers, m = 1<<Precision.  The standard
// error of the estimate is ≈ 1.04/√Registers ≈ 0.81%.
const Registers = 1 << Precision // 16384

// DenseSize is the size of the dense serialized form in bytes:
// Registers × RegisterBits / 8 = 12288.
const DenseSize = Registers * RegisterBits / 8 // 12288

// rankMax is the largest register value Add can produce: Q+1 where
// Q = 64-Precision is the width of the post-index hash and the +1 covers
// the all-zero remainder.  A serialized register above this cannot have
// come from any 64-bit hash at this precision, so HllFromBytes rejects
// it as corrupt.
const rankMax = 64 - Precision + 1 // 51

// registerMask selects one register within a byte.
const registerMask = 1<<RegisterBits - 1 // 0x3f

// ErrBadLength reports serialized data whose length is not DenseSize.
var ErrBadLength = errors.New("hyperloglog: serialized data must be exactly 12288 bytes")

// ErrBadRegister reports a serialized register value above rankMax.
var ErrBadRegister = errors.New("hyperloglog: register value exceeds the maximum rank of 51")

// Hll is a HyperLogLog cardinality sketch over byte-string elements:
// the count of distinct values Add has seen, estimated from 16384
// six-bit registers.  The zero value is an empty sketch ready to use.
// Count caches its result; the cache fields are atomics so that the
// hyperloglog_ts twin can call Count under its read lock (see the twin
// docs) — do not copy an Hll (always use *Hll).
type Hll struct {
	dense  [DenseSize]byte
	cached atomic.Uint64
	valid  atomic.Bool
}

// NewHll returns an empty Hll.  The zero value is identical and fully
// usable; the constructor is convenience and note parity.
func NewHll() *Hll { return &Hll{} }

// Add hashes v and updates the register it maps to when its rank beats
// the stored one.  It reports whether any register changed — the signal
// PFADD replies with and the cache invalidation key.  Adding the same
// value repeatedly keeps returning false after the first add.
// Complexity is O(len(v)) for the hash plus O(1) for the register.
// Add on a nil *Hll panics (there is nowhere to record the element).
func (h *Hll) Add(v []byte) bool {
	if h == nil {
		panic("hyperloglog: Add on a nil *Hll — create one with NewHll() or use a zero Hll value")
	}
	hash := xxh64(v)
	idx := hash & (Registers - 1)
	// Rank = 1 + the run of zero bits after the index bits, with a
	// sentinel one bit at position Q so an all-zero remainder yields Q+1.
	rank := uint8(bits.TrailingZeros64((hash>>Precision)|1<<(64-Precision)) + 1)
	if rank > getRegister(&h.dense, int(idx)) {
		setRegister(&h.dense, int(idx), rank)
		h.valid.Store(false) // after the write, so a cached read never pairs new registers with a stale count
		return true
	}
	return false
}

// Count returns the estimated number of distinct elements added (or
// merged in).  The first call after a mutation that changed a register
// computes the estimate in O(m); the result is cached and every later
// call is O(1) until the next change.  Standard error ≈ 0.81%; near
// exact below the LinearCounting threshold.  A nil *Hll reports 0.
func (h *Hll) Count() uint64 {
	if h == nil {
		return 0
	}
	if h.valid.Load() {
		return h.cached.Load()
	}
	reghisto := h.histogram()
	c := estimate(&reghisto)
	h.cached.Store(c)
	h.valid.Store(true)
	return c
}

// Merge folds others into h with the register-wise maximum — the HLL
// union, so the merged estimate approximates the cardinality of the
// union of all input sets.  The others are unchanged unless one aliases
// h (harmless: max(a, a) = a).  Nil elements are treated as empty and
// skipped; merging only empty operands (or none) is a no-op, so a nil h
// tolerates it.  A nil h with a non-empty operand panics (there is
// nowhere to record the union).  Complexity is O(k·m) for k others.
func (h *Hll) Merge(others ...*Hll) {
	nonEmpty := false
	for _, o := range others {
		if o != nil && !o.isEmpty() {
			nonEmpty = true
			break
		}
	}
	if !nonEmpty {
		return
	}
	if h == nil {
		panic("hyperloglog: Merge on a nil *Hll — create one with NewHll() or use a zero Hll value")
	}
	changed := false
	for _, o := range others {
		if h.MergeMax(o) {
			changed = true
		}
	}
	if changed {
		h.valid.Store(false)
	}
}

// MergeMax folds o's registers into h with the register-wise maximum,
// reporting whether any register changed.  It is the lock-free core of
// Merge and the hook the hyperloglog_ts twin folds its operand
// snapshots through: it never consults or touches the count cache —
// the caller invalidates on change.  A nil o, or one that aliases h,
// is a no-op returning false.  Complexity is O(m).
func (h *Hll) MergeMax(o *Hll) bool {
	if o == nil || o == h {
		return false
	}
	changed := false
	for i := 0; i < Registers; i++ {
		if v := getRegister(&o.dense, i); v > getRegister(&h.dense, i) {
			setRegister(&h.dense, i, v)
			changed = true
		}
	}
	return changed
}

// Clone returns an independent copy of the sketch — registers copied,
// count cache cleared (the clone recomputes on its next Count).  The
// copy is a point-in-time snapshot for handing to another goroutine or
// locking away before a Merge; a nil *Hll clones to nil.
// Complexity is O(m).
func (h *Hll) Clone() *Hll {
	if h == nil {
		return nil
	}
	c := new(Hll)
	copy(c.dense[:], h.dense[:])
	return c
}

// Invalidate drops the cached estimate so the next Count recomputes.
// Exposed for the hyperloglog_ts twin (which folds raw MergeMax
// snapshots under its write lock) and for callers that mutate through
// MergeMax directly; ordinary use never needs it — Add, Merge and
// Reset invalidate themselves.  A nil *Hll is a no-op.
func (h *Hll) Invalidate() {
	if h == nil {
		return
	}
	h.valid.Store(false)
}

// Reset empties the sketch.  Complexity is O(m).  A nil *Hll is a no-op
// (an empty sketch needs no resetting).
func (h *Hll) Reset() {
	if h == nil {
		return
	}
	clear(h.dense[:])
	h.cached.Store(0)
	h.valid.Store(true)
}

// IsEmpty reports whether no element has been recorded (every register
// zero — true of nil, zero-value and freshly Reset sketches).
// Complexity is O(m) with an early exit on the first nonzero register.
func (h *Hll) IsEmpty() bool {
	if h == nil {
		return true
	}
	return h.isEmpty()
}

// isEmpty is the lock-free core of IsEmpty.
func (h *Hll) isEmpty() bool {
	for _, b := range &h.dense { // range over the pointer: no 12 KiB copy
		if b != 0 {
			return false
		}
	}
	return true
}

// Bytes returns the dense serialized form: exactly DenseSize (12288)
// bytes with the registers packed RegisterBits each, least-significant
// first — register i occupies bits [6i, 6i+6) of the array.  The slice
// is a fresh copy owned by the caller.  The form is fixed for the life
// of the package (the hash, precision and packing are frozen constants),
// so a serialized Hll decodes and merges across versions.  A nil *Hll
// returns nil.  Complexity is O(m).
func (h *Hll) Bytes() []byte {
	if h == nil {
		return nil
	}
	out := make([]byte, DenseSize)
	copy(out, h.dense[:])
	return out
}

// HllFromBytes decodes the dense serialized form produced by Bytes.  It
// validates the length (ErrBadLength) and that no register holds a rank
// Add could not have produced at this precision (ErrBadRegister, the
// max is rankMax = 51); corrupt input reports an error wrapping one of
// them, never a panic.  Complexity is O(m).
func HllFromBytes(b []byte) (*Hll, error) {
	if len(b) != DenseSize {
		return nil, fmt.Errorf("hyperloglog: got %d bytes: %w", len(b), ErrBadLength)
	}
	h := &Hll{}
	copy(h.dense[:], b)
	for i := 0; i < Registers; i++ {
		if getRegister(&h.dense, i) > rankMax {
			return nil, fmt.Errorf("hyperloglog: register %d: %w", i, ErrBadRegister)
		}
	}
	return h, nil
}

// Lock and Unlock are no-ops kept so code written against the
// hyperloglog_ts twin compiles unchanged.  The plain package is not
// safe for concurrent use.
func (h *Hll) Lock() {}

// Unlock is the other half of the no-op lock pair — see Lock.
func (h *Hll) Unlock() {}

// getRegister reads register i from the packed array.  The pair fb ≤ 2
// keeps a register inside byte b; fb = 4 or 6 spills the top bits into
// the next byte, and the last register (fb = 2) never touches past the
// end of the array.
func getRegister(p *[DenseSize]byte, i int) uint8 {
	b := i * RegisterBits / 8
	fb := i * RegisterBits & 7
	v := p[b] >> fb
	if fb > 8-RegisterBits { // register spills into the next byte
		v |= p[b+1] << (8 - fb)
	}
	return v & registerMask
}

// setRegister writes register i (value v, already masked to 6 bits) into
// the packed array — see getRegister for the bit layout.
func setRegister(p *[DenseSize]byte, i int, v uint8) {
	b := i * RegisterBits / 8
	fb := i * RegisterBits & 7
	p[b] &= ^(registerMask << fb)
	p[b] |= v << fb
	if fb > 8-RegisterBits { // register spills into the next byte
		p[b+1] &= ^(registerMask >> (8 - fb))
		p[b+1] |= v >> (8 - fb)
	}
}
