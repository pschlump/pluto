/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom

import "encoding/binary"

// The two element hashes are frozen constants of the package, exactly as
// the single hash of hyperloglog is: two filters can only be merged
// (bitwise OR) or compared in accuracy when every version and every
// process derives the same probe positions from the same element, so the
// hash pair is part of the serialized-format contract (see Bytes).  Both
// come from the author's original 2016 bloom-filter library — MurmurHash2
// (Austin Aubaret) under the fixed seed below and SuperFastHash (Paul
// Hsieh) — re-derived as the Kirsch–Mitzenmacher double-hashing pair
// (h1 + i·h2), which sizes the probe count k freely instead of the 2016
// library's fixed two probes.
//
// The 2016 Go port of both hashes carried a silent bug this port fixes:
// expressions like uint32(data[1] << 8) shift *inside the byte type*, so
// every odd byte contributed zero and each 16-bit read collapsed to its
// low byte (Go discards bits shifted past a type's width; C promotes to
// int first).  superFastHash additionally sign-extends its case-3/case-1
// tail bytes here, as the published C does ((signed char) casts — the
// 2016 port treated them unsigned).  The fixed functions hash the full
// input, matching the C originals bit-for-bit — pinned by vectors
// generated from the C sources — and consequently produce different
// values than the 2016 library, whose test vectors had pinned the
// crippled forms.
//
// Changing hashSeed or either hash body below breaks compatibility with
// every previously serialized Bloom.

// hashSeed is the fixed MurmurHash2 seed (the 2016 library's constant).
const hashSeed uint32 = 552211

// murmur2 is MurmurHash2 (Austin Aubaret, 2008; the public-domain
// reference implementation: 0x5bd1e995 / r=24, little-endian block
// reads, the fall-through tail, the 13/15 final avalanche).
// Complexity is O(len(v)).
func murmur2(v []byte) uint32 {
	const m = 0x5bd1e995
	const r = 24

	h := hashSeed ^ uint32(len(v))
	for len(v) >= 4 {
		k := binary.LittleEndian.Uint32(v)
		v = v[4:]

		k *= m
		k ^= k >> r
		k *= m

		h *= m
		h ^= k
	}

	switch len(v) { // tail falls through like the reference switch
	case 3:
		h ^= uint32(v[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(v[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(v[0])
		h *= m
	}

	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}

// superFastHash is Paul Hsieh's SuperFastHash
// (http://www.azillionmonkeys.com/qed/hash.html, the updated form with
// the revised final avalanche): len-seeded, 16-bit little-endian reads,
// the three tail cases (their bytes sign-extended, per the published
// C's (signed char) casts), then the avalanche.
// Complexity is O(len(v)).
func superFastHash(v []byte) uint32 {
	if len(v) == 0 {
		return 0
	}

	h := uint32(len(v))
	rem := len(v) & 3

	for len(v) >= 4 {
		h += uint32(binary.LittleEndian.Uint16(v))
		tmp := uint32(binary.LittleEndian.Uint16(v[2:]))<<11 ^ h
		h = h<<16 ^ tmp
		v = v[4:]
		h += h >> 11
	}

	switch rem {
	case 3:
		h += uint32(binary.LittleEndian.Uint16(v))
		h ^= h << 16
		h ^= uint32(int32(int8(v[2])) << 18)
		h += h >> 11
	case 2:
		h += uint32(binary.LittleEndian.Uint16(v))
		h ^= h << 11
		h += h >> 17
	case 1:
		h += uint32(int32(int8(v[0])))
		h ^= h << 10
		h += h >> 1
	}

	// Force "avalanching" of final 127 bits.
	h ^= h << 3
	h += h >> 5
	h ^= h << 4
	h += h >> 17
	h ^= h << 25
	h += h >> 6
	return h
}

// probes fills positions with the k probe bit indexes of v — the
// Kirsch–Mitzenmacher construction (h1 + i·h2) mod m, which derives any
// probe count from the two frozen hashes with no extra hashing.  A
// second hash congruent to 0 mod m (always true of the empty element,
// whose SuperFastHash is 0) would collapse every probe onto the first,
// so the step is guarded to 1 — the standard degenerate-step repair.
// positions must have room for the filter's k; every value written is
// < m, so bits at or above m in the final word are never touched.
func (b *Bloom) probes(v []byte, positions []uint64) {
	h1 := uint64(murmur2(v))
	step := uint64(superFastHash(v)) % b.m
	if step == 0 {
		step = 1
	}
	p := h1 % b.m
	for i := range b.k {
		positions[i] = p
		p += step
		if p >= b.m {
			p -= b.m
		}
	}
}
