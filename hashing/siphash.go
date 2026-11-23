/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.

SipHash was designed by Jean-Philippe Aumasson and Daniel J. Bernstein.
This is a behavioral port of the published SipHash-2-4 algorithm.
*/

package hashing

import (
	"encoding/binary"
	"math/bits"
)

// SipHash24 returns SipHash-2-4 with the supplied 128-bit key. SipHash24()
// uses Guava's standard key 000102...0f; SipHash24(k0,k1) accepts each half as
// a little-endian uint64. Other argument counts panic.
func SipHash24(key ...uint64) HashFunction {
	var k0, k1 uint64
	switch len(key) {
	case 0:
		k0, k1 = 0x0706050403020100, 0x0f0e0d0c0b0a0908
	case 2:
		k0, k1 = key[0], key[1]
	default:
		panic("hashing: SipHash24 accepts zero arguments or two key words")
	}
	return newHashFunction(64, func(data []byte) HashCode {
		return hashFromUint64(sipHash24(data, k0, k1))
	})
}

func sipHash24(data []byte, k0, k1 uint64) uint64 {
	v0 := uint64(0x736f6d6570736575) ^ k0
	v1 := uint64(0x646f72616e646f6d) ^ k1
	v2 := uint64(0x6c7967656e657261) ^ k0
	v3 := uint64(0x7465646279746573) ^ k1
	length := len(data)
	round := func() {
		v0 += v1
		v2 += v3
		v1 = bits.RotateLeft64(v1, 13)
		v3 = bits.RotateLeft64(v3, 16)
		v1 ^= v0
		v3 ^= v2
		v0 = bits.RotateLeft64(v0, 32)
		v2 += v1
		v0 += v3
		v1 = bits.RotateLeft64(v1, 17)
		v3 = bits.RotateLeft64(v3, 21)
		v1 ^= v2
		v3 ^= v0
		v2 = bits.RotateLeft64(v2, 32)
	}
	process := func(m uint64) {
		v3 ^= m
		round()
		round()
		v0 ^= m
	}
	for len(data) >= 8 {
		process(binary.LittleEndian.Uint64(data))
		data = data[8:]
	}
	var tail [8]byte
	copy(tail[:], data)
	tail[7] = byte(length)
	process(binary.LittleEndian.Uint64(tail[:]))
	v2 ^= 0xff
	round()
	round()
	round()
	round()
	return v0 ^ v1 ^ v2 ^ v3
}
