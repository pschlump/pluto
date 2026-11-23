/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.

MurmurHash3 was written by Austin Appleby and placed in the public domain.
This behavioral port follows the reference algorithm and Guava's conventions.
*/

package hashing

import (
	"encoding/binary"
	"math/bits"
)

// Murmur3_32 returns Guava-compatible MurmurHash3 x86 32-bit. Omitting seed
// uses zero. Supplying more than one seed panics.
func Murmur3_32(seed ...uint32) HashFunction {
	s := optionalSeed32(seed)
	return newHashFunction(32, func(data []byte) HashCode {
		return hashFromUint32(murmur3Sum32(data, s))
	})
}

// Murmur3_128 returns Guava-compatible MurmurHash3 x64 128-bit. Omitting seed
// uses zero. Supplying more than one seed panics.
func Murmur3_128(seed ...uint32) HashFunction {
	s := optionalSeed32(seed)
	return newHashFunction(128, func(data []byte) HashCode {
		h1, h2 := murmur3Sum128(data, s)
		out := make([]byte, 16)
		binary.LittleEndian.PutUint64(out, h1)
		binary.LittleEndian.PutUint64(out[8:], h2)
		return NewHashCode(out, LittleEndian)
	})
}

func optionalSeed32(seed []uint32) uint32 {
	if len(seed) > 1 {
		panic("hashing: at most one Murmur3 seed may be supplied")
	}
	if len(seed) == 1 {
		return seed[0]
	}
	return 0
}

func murmur3Sum32(data []byte, seed uint32) uint32 {
	h1 := seed
	length := len(data)
	for len(data) >= 4 {
		k1 := binary.LittleEndian.Uint32(data)
		data = data[4:]
		k1 *= 0xcc9e2d51
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= 0x1b873593
		h1 ^= k1
		h1 = bits.RotateLeft32(h1, 13)
		h1 = h1*5 + 0xe6546b64
	}
	var k1 uint32
	for i, b := range data {
		k1 ^= uint32(b) << (8 * i)
	}
	if len(data) != 0 {
		k1 *= 0xcc9e2d51
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= 0x1b873593
		h1 ^= k1
	}
	return murmur3FMix32(h1, length)
}

func murmur3FMix32(h1 uint32, length int) uint32 {
	h1 ^= uint32(length)
	h1 ^= h1 >> 16
	h1 *= 0x85ebca6b
	h1 ^= h1 >> 13
	h1 *= 0xc2b2ae35
	h1 ^= h1 >> 16
	return h1
}

func murmur3Sum128(data []byte, seed uint32) (uint64, uint64) {
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
	h1 = murmur3FMix64(h1)
	h2 = murmur3FMix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func murmur3FMix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}
