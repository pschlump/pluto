/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog

import "encoding/binary"

// The element hash is xxHash64 (Yann Collet) under the fixed seed below.
// Redis hashes with MurmurHash64A instead — matching its hash exactly was
// optional per the request note — but whichever hash is chosen it must be
// frozen: two sketches can only be merged (register-wise max) or compared
// in accuracy if every version and every process derives the same register
// index and rank from the same element.  Changing hashSeed or the xxh64
// body below breaks compatibility with every previously serialized Hll,
// so both are part of the serialized-format contract (see Bytes).
const hashSeed uint64 = 0

// xxHash64 primes.
const (
	prime64_1 = 0x9E3779B185EBCA87
	prime64_2 = 0xC2B2AE3D27D4EB4F
	prime64_3 = 0x165667B19E3779F9
	prime64_4 = 0x85EBCA77C2B2AE63
	prime64_5 = 0x27D4EB2F165667C5
)

// rotl64 rotates x left by r bits.
func rotl64(x uint64, r uint) uint64 { return x<<r | x>>(64-r) }

// xxhRound is one accumulator round of xxHash64.
func xxhRound(acc, input uint64) uint64 {
	acc += input * prime64_2
	acc = rotl64(acc, 31)
	acc *= prime64_1
	return acc
}

// xxhMergeRound merges one stripe accumulator into the hash.
func xxhMergeRound(acc, val uint64) uint64 {
	val = xxhRound(0, val)
	acc ^= val
	return acc*prime64_1 + prime64_4
}

// xxh64 computes the xxHash64 checksum of b under hashSeed; see
// xxh64Seed.
func xxh64(b []byte) uint64 { return xxh64Seed(b, hashSeed) }

// xxh64Seed computes the xxHash64 checksum of b under the given seed.
// Complexity is O(len(b)).  Bit-compatible with the xxHash
// specification (little-endian lanes, 8/4/1-byte tail steps, final
// avalanche); the test suite pins it against the reference suite's
// published vectors.
func xxh64Seed(b []byte, seed uint64) uint64 {
	n := uint64(len(b))
	var h uint64

	if len(b) >= 32 {
		v1 := seed + prime64_1 + prime64_2
		v2 := seed + prime64_2
		v3 := seed
		v4 := seed - prime64_1

		for len(b) >= 32 {
			v1 = xxhRound(v1, binary.LittleEndian.Uint64(b))
			v2 = xxhRound(v2, binary.LittleEndian.Uint64(b[8:]))
			v3 = xxhRound(v3, binary.LittleEndian.Uint64(b[16:]))
			v4 = xxhRound(v4, binary.LittleEndian.Uint64(b[24:]))
			b = b[32:]
		}

		h = rotl64(v1, 1) + rotl64(v2, 7) + rotl64(v3, 12) + rotl64(v4, 18)
		h = xxhMergeRound(h, v1)
		h = xxhMergeRound(h, v2)
		h = xxhMergeRound(h, v3)
		h = xxhMergeRound(h, v4)
	} else {
		h = seed + prime64_5
	}

	h += n

	for len(b) >= 8 {
		h ^= xxhRound(0, binary.LittleEndian.Uint64(b))
		h = rotl64(h, 27)*prime64_1 + prime64_4
		b = b[8:]
	}
	for len(b) >= 4 {
		h ^= uint64(binary.LittleEndian.Uint32(b)) * prime64_1
		h = rotl64(h, 23)*prime64_2 + prime64_3
		b = b[4:]
	}
	for len(b) >= 1 {
		h ^= uint64(b[0]) * prime64_5
		h = rotl64(h, 11) * prime64_1
		b = b[1:]
	}

	h ^= h >> 33
	h *= prime64_2
	h ^= h >> 29
	h *= prime64_3
	h ^= h >> 32
	return h
}
