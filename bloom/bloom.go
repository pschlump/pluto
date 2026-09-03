/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bloom implements the classic Bloom filter (Burton Bloom,
// "Space/Time Trade-offs in Hash Coding with Allowable Errors", CACM
// 1970): a fixed-size bit array that approximates set membership with
// one-sided error.  Add sets k probe bits derived from the element;
// MayContain reports whether all k are set.  An element that was added
// always reports present — false negatives cannot happen, structurally
// (Add and MayContain derive the exact same probe positions).  An
// element that was never added reports present with probability ≈
// (1 − e^(−kn/m))^k, the false-positive rate p the constructor sizes
// for.  Elements are never stored, compared or iterated — membership is
// the only query, and deletion is not possible (clearing bits would
// break other elements' probes; counting filters are a different
// structure).
//
// The package is the pluto-standard port of the author's 2016
// bloom-filter library: the same hash pair (MurmurHash2 under a fixed
// seed, and SuperFastHash — both now faithful to their C originals; see
// hash.go for the 2016 port's shift bug this fixes), generalized from
// its fixed two probes to the Kirsch–Mitzenmacher double-hashing
// construction (h1 + i·h2) mod m, which derives any probe count from
// the pair.  Constructors size the filter from the intended workload:
// NewBloom(n, p) takes the planned distinct-element count and the
// tolerated false-positive rate and computes the optimal bit count
// m = −n·ln(p)/(ln 2)² and probe count k = ln(2)·m/n; NewBloomBits(m, k)
// takes both directly.  The original's Found/AddTo/TestAndSet surface
// survives as MayContain/Add/TestAndSet (Found also returned the two
// raw hashes; that leak is dropped).
//
// The structure is non-generic (elements are byte strings hashed on the
// way in, the hyperloglog / union_find / suffix_array precedent): a
// membership sketch never stores an element, and the hash pair is a
// frozen constant — two filters merge validly (bitwise OR) only when
// every version and process derives the same probe positions, which
// also makes the hash pair part of the serialized-format contract
// (Bytes).  Elements are []byte; string callers pass []byte(s).
//
// Unlike the constructor-free sketches, a Bloom filter's shape (m, k)
// must exist before the first Add, so the zero value is not writable:
// Add and TestAndSet on a nil or zero-value Bloom panic naming the
// constructors (the lru capacity precedent — no shape, no sane answer).
// Every read tolerates nil and the zero value (MayContain false, Count
// and Added 0, Saturation 0, IsEmpty true, Bytes nil, Merge of only
// empty operands a no-op).  The remaining panics are constructor
// validation (NewBloom: n < 1, p outside (0,1), or a shape beyond
// maxBits; NewBloomBits: bits or hashes out of range) and Merge misuse
// (a non-empty operand whose (m, k) differs from the destination's, or
// a non-empty operand merged into a nil/zero-value destination).  Each
// message names the method or constructor and the fix.
//
// Count estimates the number of *distinct* elements from the fill
// ratio (the maximum-likelihood n̂ = −(m/k)·ln(1 − X/m) where X is the
// set-bit count) — Added by contrast is the exact number of Add and
// TestAndSet calls, duplicates included and indetectable.  At full
// saturation the estimate carries no information and Count reports m.
//
// Core operations:
//
//	Add        — Set the element's k probe bits; true if any was clear.	O(len(v)+k)
//	MayContain — All k probe bits set (never false for an added element).	O(len(v)+k)
//	TestAndSet — MayContain answered and recorded in one pass.			O(len(v)+k)
//	Merge      — Bitwise OR union with same-shape filters.					O(k·m/64)
//	Count      — ML estimate of distinct elements from the fill ratio.	O(1)
//	Saturation — Fraction of bits set.										O(1)
//	Bytes / BloomFromBytes — Serialize / validate-and-decode.				O(m/64)
//	Reset / Clone / IsEmpty / Added / BitCount / HashCount.					O(m/64) / O(1)
//
// Bloom is not safe for concurrent use; the mutex-guarded twin
// bloom_ts has the same interface.
package bloom

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// maxHashes is the largest probe count a filter may use.  Kirsch–
// Mitzenmacher sequences degrade well past it, and the cap keeps k in a
// uint8 with room to spare.  NewBloom clamps its computed optimum here.
const maxHashes = 64

// maxBits is the largest bit count a filter may use: the two 32-bit
// hashes can address at most 2^32 probe positions without biasing the
// (h1 + i·h2) mod m arithmetic, and 2^32 bits is 512 MiB — beyond any
// sane in-memory filter.
const maxBits = 1 << 32

// headerSize is the serialized header: m, k and added, 8 bytes each,
// little-endian.
const headerSize = 24

// ln2 is the natural log of 2, the constant of both optimal-parameter
// formulas.
const ln2 = math.Ln2

// ErrBadLength reports serialized data too short for the header or not
// exactly headerSize plus the bit words its m implies (m itself out of
// range is the same error — no filter could have produced it).
var ErrBadLength = errors.New("bloom: serialized data has the wrong length for its header")

// ErrBadHashes reports a serialized probe count outside [1, maxHashes].
var ErrBadHashes = errors.New("bloom: serialized probe count out of range")

// ErrBadBits reports a serialized set bit at or above m — the final
// word's padding, which no Add can touch.
var ErrBadBits = errors.New("bloom: serialized data has a set bit outside the filter's m bits")

// Bloom is a Bloom filter over byte-string elements: m bits and k
// frozen-hash probe positions per element.  Create it with NewBloom or
// NewBloomBits; the zero value tolerates every read but cannot be
// written.  Do not copy a Bloom by value (the bits slice would alias) —
// always use *Bloom, and Clone for a copy.
type Bloom struct {
	bits  []uint64 // m bits, word i holding bits [64i, 64i+64), little-endian within the word
	m     uint64   // bit count; the zero value is distinguishable by m == 0
	k     uint8    // probe count
	set   uint64   // bits currently set (maintained on 0→1 transitions)
	added uint64   // Add + TestAndSet calls (duplicates counted — indetectable)
}

// NewBloom returns a filter sized for n distinct elements with a
// false-positive rate of at most ~p once n elements have been added:
// m = ceil(−n·ln(p)/(ln 2)²) bits and k = ln(2)·m/n probes, both the
// standard optima (p ≈ (1 − e^(−kn/m))^k is minimized by exactly this
// pair).  k is clamped to [1, maxHashes].  The filter keeps working
// past n elements — the false-positive rate climbs with saturation;
// watch Saturation, or size for the steady-state count.
//
// It panics on n < 1, on p outside (0,1) (NaN included), and when the
// optimal m exceeds maxBits.
func NewBloom(n int, p float64) *Bloom {
	if n < 1 {
		panic(fmt.Sprintf("bloom: NewBloom(n=%d, p=%g) — n must be at least 1", n, p))
	}
	if !(p > 0 && p < 1) { // NaN fails both comparisons and lands here too
		panic(fmt.Sprintf("bloom: NewBloom(n=%d, p=%g) — p must be strictly between 0 and 1", n, p))
	}
	mf := math.Ceil(float64(n) * (-math.Log(p)) / (ln2 * ln2))
	if !(mf <= maxBits) { // NaN cannot occur (p is in (0,1)); only genuinely huge shapes land here
		panic(fmt.Sprintf("bloom: NewBloom(n=%d, p=%g) needs %.4g bits, above the maxBits cap of %d — reduce n or loosen p", n, p, mf, uint64(maxBits)))
	}
	m := uint64(mf)
	k := int(math.Round(float64(m) / float64(n) * ln2))
	if k < 1 {
		k = 1
	} else if k > maxHashes {
		k = maxHashes
	}
	return newBloom(m, k)
}

// NewBloomBits returns a filter with an explicit shape: bits total and
// hashes probes per element.  It panics on bits outside [1, maxBits] or
// hashes outside [1, maxHashes].
func NewBloomBits(bits uint64, hashes int) *Bloom {
	if bits < 1 || bits > maxBits {
		panic(fmt.Sprintf("bloom: NewBloomBits(%d, %d) — bits must be in [1, %d]", bits, hashes, uint64(maxBits)))
	}
	if hashes < 1 || hashes > maxHashes {
		panic(fmt.Sprintf("bloom: NewBloomBits(%d, %d) — hashes must be in [1, %d]", bits, hashes, maxHashes))
	}
	return newBloom(bits, hashes)
}

// newBloom allocates a zeroed filter of the validated shape.
func newBloom(m uint64, k int) *Bloom {
	return &Bloom{
		bits: make([]uint64, (m+63)/64),
		m:    m,
		k:    uint8(k),
	}
}

// Add records v, setting its k probe bits, and reports whether any bit
// was previously clear — exactly the answer MayContain gave before the
// call (false means the element already reported present; re-adding an
// element always reports false).  Complexity is O(len(v)) for the two
// hashes plus O(k).
// Add on a nil or zero-value Bloom panics (there is no shape to probe).
func (b *Bloom) Add(v []byte) bool {
	if b == nil || b.m == 0 {
		panic("bloom: Add on a nil or zero-value Bloom — create it with NewBloom or NewBloomBits")
	}
	var buf [maxHashes]uint64
	b.probes(v, buf[:b.k])
	changed := false
	for _, p := range buf[:b.k] {
		w, mask := p>>6, uint64(1)<<(p&63)
		if b.bits[w]&mask == 0 {
			b.bits[w] |= mask
			b.set++
			changed = true
		}
	}
	b.added++
	return changed
}

// MayContain reports whether v's k probe bits are all set — true for
// every added element (false negatives cannot happen) and, with
// probability ≈ p at the design load, true for absent ones.  A nil or
// zero-value Bloom reports false.  Complexity is O(len(v)) plus O(k)
// with an early exit on the first clear bit.
func (b *Bloom) MayContain(v []byte) bool {
	if b == nil || b.m == 0 {
		return false
	}
	var buf [maxHashes]uint64
	b.probes(v, buf[:b.k])
	for _, p := range buf[:b.k] {
		if b.bits[p>>6]&(uint64(1)<<(p&63)) == 0 {
			return false
		}
	}
	return true
}

// TestAndSet answers MayContain for v and records v in one pass — the
// 2016 library's operation, the atomic insert-if-absent signal: false
// means the element had not been seen (and is now recorded), true means
// it (probably) already was.  The return value is exactly the inverse
// of what Add would have reported.  Complexity is O(len(v)) plus O(k).
// TestAndSet on a nil or zero-value Bloom panics (there is no shape to
// probe).
func (b *Bloom) TestAndSet(v []byte) bool {
	if b == nil || b.m == 0 {
		panic("bloom: TestAndSet on a nil or zero-value Bloom — create it with NewBloom or NewBloomBits")
	}
	var buf [maxHashes]uint64
	b.probes(v, buf[:b.k])
	present := true
	for _, p := range buf[:b.k] {
		w, mask := p>>6, uint64(1)<<(p&63)
		if b.bits[w]&mask == 0 {
			b.bits[w] |= mask
			b.set++
			present = false
		}
	}
	b.added++
	return present
}

// Merge folds others into b with the bitwise OR — the lossless filter
// union: afterwards b reports present for everything any operand would
// have (and only more; the false-positive rates add up).  Nil and empty
// operands are skipped, and an operand that aliases b is a no-op; a nil
// b tolerates a merge of only empty operands.  Every contributing
// operand must share b's exact shape — a non-empty operand whose (m, k)
// differs, or any non-empty operand into a nil or zero-value b, has no
// sane answer and panics naming the mismatch.  added is summed, so the
// operands' Add history is preserved.  Complexity is O(k·m/64) for k
// contributing operands.
func (b *Bloom) Merge(others ...*Bloom) {
	contributing := false
	for _, o := range others {
		if o != nil && o != b && o.set > 0 {
			contributing = true
			break
		}
	}
	if !contributing {
		return
	}
	if b == nil || b.m == 0 {
		panic("bloom: Merge on a nil or zero-value Bloom — create it with NewBloom or NewBloomBits")
	}
	for _, o := range others {
		if o == nil || o == b || o.set == 0 {
			continue
		}
		if o.m != b.m || o.k != b.k {
			panic(fmt.Sprintf("bloom: Merge of a %d-bit/%d-probe filter into a %d-bit/%d-probe filter — merged filters must come from the same NewBloom/NewBloomBits parameters", o.m, o.k, b.m, b.k))
		}
	}
	for _, o := range others {
		if o == nil || o == b || o.set == 0 {
			continue
		}
		for i, w := range o.bits {
			nw := b.bits[i] | w
			if nw != b.bits[i] {
				b.set += uint64(bits.OnesCount64(nw)) - uint64(bits.OnesCount64(b.bits[i]))
				b.bits[i] = nw
			}
		}
		b.added += o.added
	}
}

// Clone returns an independent copy of the filter — bits, shape and
// counters.  A nil or zero-value Bloom clones to nil.
// Complexity is O(m/64).
func (b *Bloom) Clone() *Bloom {
	if b == nil || b.m == 0 {
		return nil
	}
	c := &Bloom{
		bits:  make([]uint64, len(b.bits)),
		m:     b.m,
		k:     b.k,
		set:   b.set,
		added: b.added,
	}
	copy(c.bits, b.bits)
	return c
}

// Reset empties the filter (all counters zero).  Complexity is O(m/64).
// A nil or zero-value Bloom is a no-op.
func (b *Bloom) Reset() {
	if b == nil || b.m == 0 {
		return
	}
	clear(b.bits)
	b.set = 0
	b.added = 0
}

// IsEmpty reports whether no bit is set (true of nil, zero-value and
// freshly Reset filters).  Complexity is O(1) — the set-bit counter is
// maintained incrementally.
func (b *Bloom) IsEmpty() bool {
	if b == nil || b.m == 0 {
		return true
	}
	return b.set == 0
}

// Added returns the exact number of Add and TestAndSet calls — the
// count of insertions, duplicates included.  A filter cannot tell a
// duplicate from a first insert; the *distinct* count is the estimate
// Count reports.  A nil or zero-value Bloom reports 0.
// Complexity is O(1).
func (b *Bloom) Added() uint64 {
	if b == nil || b.m == 0 {
		return 0
	}
	return b.added
}

// Count estimates the number of distinct elements added (or merged in)
// from the fill ratio: the maximum-likelihood n̂ = −(m/k)·ln(1 − X/m)
// for X set bits.  At full saturation (X = m, where the estimate
// diverges and carries no information) it reports m.  A nil or
// zero-value Bloom reports 0.  Complexity is O(1) — the set-bit counter
// is maintained incrementally.
func (b *Bloom) Count() uint64 {
	if b == nil || b.m == 0 || b.set == 0 {
		return 0
	}
	if b.set >= b.m {
		return b.m
	}
	est := -(float64(b.m) / float64(b.k)) * math.Log(1-float64(b.set)/float64(b.m))
	if est < 0 { // defensive: log of exactly 0 above is filtered by set == 0
		return 0
	}
	return uint64(math.Round(est))
}

// Saturation returns the fraction of the m bits that are set, in [0,1]
// — the load signal: at the design load of NewBloom(n, p) it is ≈
// 1 − e^(−kn/m) ≈ 0.5 (k optimal implies half the bits set).  A nil or
// zero-value Bloom reports 0.  Complexity is O(1).
func (b *Bloom) Saturation() float64 {
	if b == nil || b.m == 0 {
		return 0
	}
	return float64(b.set) / float64(b.m)
}

// BitCount returns m, the number of bits in the filter.
// Complexity is O(1).
func (b *Bloom) BitCount() uint64 {
	if b == nil {
		return 0
	}
	return b.m
}

// HashCount returns k, the number of probe positions per element.
// Complexity is O(1).
func (b *Bloom) HashCount() int {
	if b == nil {
		return 0
	}
	return int(b.k)
}

// Bytes returns the serialized form: a 24-byte header (m, k and added,
// 8 bytes each, little-endian) followed by the bit words in array
// order, 8 bytes each, little-endian — headerSize + (m+63)/64·8 bytes
// total, a fresh copy owned by the caller.  The form is versionless —
// no magic — because the hash pair, like hyperloglog's, is a frozen
// constant; changing it breaks compatibility with every previously
// serialized Bloom, which is why the hashes are documented constants.
// A nil or zero-value Bloom returns nil.
// Complexity is O(m/64).
func (b *Bloom) Bytes() []byte {
	if b == nil || b.m == 0 {
		return nil
	}
	out := make([]byte, headerSize+len(b.bits)*8)
	binary.LittleEndian.PutUint64(out[0:], b.m)
	binary.LittleEndian.PutUint64(out[8:], uint64(b.k))
	binary.LittleEndian.PutUint64(out[16:], b.added)
	for i, w := range b.bits {
		binary.LittleEndian.PutUint64(out[headerSize+i*8:], w)
	}
	return out
}

// BloomFromBytes decodes the serialized form produced by Bytes.  It
// validates the length against the header's m (ErrBadLength — too
// short, or not exactly the word count m implies, or m itself outside
// [1, maxBits]), the probe count (ErrBadHashes — k outside
// [1, maxHashes]) and that no bit at or above m is set in the final
// word's padding (ErrBadBits — no Add can touch those).  Corrupt input
// reports an error wrapping one of them, never a panic.
// Complexity is O(m/64).
func BloomFromBytes(data []byte) (*Bloom, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("bloom: %d bytes: %w", len(data), ErrBadLength)
	}
	m := binary.LittleEndian.Uint64(data[0:])
	k := binary.LittleEndian.Uint64(data[8:])
	added := binary.LittleEndian.Uint64(data[16:])
	if m < 1 || m > maxBits {
		return nil, fmt.Errorf("bloom: header m=%d: %w", m, ErrBadLength)
	}
	if want := headerSize + int((m+63)/64)*8; len(data) != want {
		return nil, fmt.Errorf("bloom: %d bytes for m=%d, want %d: %w", len(data), m, want, ErrBadLength)
	}
	if k < 1 || k > maxHashes {
		return nil, fmt.Errorf("bloom: header k=%d: %w", k, ErrBadHashes)
	}
	words := (m + 63) / 64
	b := &Bloom{
		bits:  make([]uint64, words),
		m:     m,
		k:     uint8(k),
		added: added,
	}
	var set uint64
	for i := range words {
		w := binary.LittleEndian.Uint64(data[headerSize+i*8:])
		b.bits[i] = w
		set += uint64(bits.OnesCount64(w))
	}
	if rem := m & 63; rem != 0 && b.bits[words-1]>>(rem) != 0 {
		return nil, fmt.Errorf("bloom: %w", ErrBadBits)
	}
	b.set = set
	return b, nil
}

// String returns a one-line summary of the filter (shape, fill and
// estimates) — the 2016 library's per-bit dump is dropped; a filter can
// hold billions of bits.  Complexity is O(1).
func (b *Bloom) String() string {
	if b == nil || b.m == 0 {
		return "Bloom (empty)"
	}
	return fmt.Sprintf("Bloom m=%d k=%d bits-set=%d saturation=%.4f added=%d distinct~%d",
		b.m, b.k, b.set, b.Saturation(), b.added, b.Count())
}

// Lock and Unlock are no-ops kept so code written against the bloom_ts
// twin compiles unchanged.  The plain package is not safe for
// concurrent use.
func (b *Bloom) Lock() {}

// Unlock is the other half of the no-op lock pair — see Lock.
func (b *Bloom) Unlock() {}
