/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.

MurmurHash3 was written by Austin Appleby and placed in the public domain.
This behavioral port follows the reference algorithm and Guava's conventions.
*/

// Package murmur3 holds the single allocation-free MurmurHash3 x64 128-bit
// core shared by hashing.Murmur3_128 and bloom_filter. It is internal so the
// module can reuse it without adding public API; its output is pinned by the
// Guava vector tests in package hashing.
package murmur3

import (
	"encoding/binary"
	"math/bits"
)

// Sum128 returns the two 64-bit halves (h1, h2) of MurmurHash3 x64 128-bit
// over data with seed. Guava's HashCode bytes are h1 then h2, little-endian.
// It never allocates. Complexity is O(len(data)).
func Sum128(data []byte, seed uint32) (uint64, uint64) {
	const c1 = 0x87c37b91114253d5
	const c2 = 0x4cf5ad432745937f
	h1, h2 := uint64(seed), uint64(seed)
	length := len(data)
	for len(data) >= 16 {
		k1 := binary.LittleEndian.Uint64(data)
		k2 := binary.LittleEndian.Uint64(data[8:])
		data = data[16:]
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}
	var tail [16]byte
	copy(tail[:], data)
	k1, k2 := binary.LittleEndian.Uint64(tail[:]), binary.LittleEndian.Uint64(tail[8:])
	if len(data) > 8 {
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
	}
	if len(data)&7 != 0 || len(data) >= 8 {
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint64(length)
	h2 ^= uint64(length)
	h1 += h2
	h2 += h1
	h1 = fmix64(h1)
	h2 = fmix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}
