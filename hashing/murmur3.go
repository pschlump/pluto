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

	"github.com/pschlump/pluto/internal/murmur3"
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
	return murmur3.Sum128(data, seed)
}
