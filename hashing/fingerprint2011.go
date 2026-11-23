/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.

Fingerprint2011 is by Geoff Pike and contributors. This behavioral port
follows Guava's Apache-2.0 implementation at the pinned reference commit.
*/

package hashing

import (
	"encoding/binary"
	"math/bits"
)

const (
	fingerprintK0 = uint64(0xa5b85c5e198ed849)
	fingerprintK1 = uint64(0x8d58ac26afe12e47)
	fingerprintK2 = uint64(0xc47b6e9e3a970ed3)
	fingerprintK3 = uint64(0xc6a4a7935bd1e995)
)

// Fingerprint2011 returns Guava-compatible Fingerprint2011.
func Fingerprint2011() HashFunction {
	return newHashFunction(64, func(data []byte) HashCode {
		return hashFromUint64(fingerprint2011(data))
	})
}

func fingerprint2011(data []byte) uint64 {
	var result uint64
	switch length := len(data); {
	case length <= 32:
		result = fingerprintMurmur64(data, fingerprintK0^fingerprintK1^fingerprintK2)
	case length <= 64:
		result = fingerprintHash33To64(data)
	default:
		result = fingerprintFull(data)
	}
	u, v := fingerprintK0, fingerprintK0
	if len(data) >= 8 {
		u = binary.LittleEndian.Uint64(data)
	}
	if len(data) >= 9 {
		v = binary.LittleEndian.Uint64(data[len(data)-8:])
	}
	result = fingerprintHash128To64(result+v, u)
	if result == 0 || result == 1 {
		return result + ^uint64(1)
	}
	return result
}

func fingerprintHash128To64(high, low uint64) uint64 {
	a := (low ^ high) * fingerprintK3
	a ^= a >> 47
	b := (high ^ a) * fingerprintK3
	b ^= b >> 47
	return b * fingerprintK3
}

func fingerprintWeak32(data []byte, seedA, seedB uint64) (uint64, uint64) {
	part1 := binary.LittleEndian.Uint64(data)
	part2 := binary.LittleEndian.Uint64(data[8:])
	part3 := binary.LittleEndian.Uint64(data[16:])
	part4 := binary.LittleEndian.Uint64(data[24:])
	seedA += part1
	seedB = bits.RotateLeft64(seedB+seedA+part4, -51)
	c := seedA
	seedA += part2 + part3
	seedB += bits.RotateLeft64(seedA, -23)
	return seedA + part4, seedB + c
}

func fingerprintFull(data []byte) uint64 {
	length := len(data)
	x := binary.LittleEndian.Uint64(data)
	y := binary.LittleEndian.Uint64(data[length-16:]) ^ fingerprintK1
	z := binary.LittleEndian.Uint64(data[length-56:]) ^ fingerprintK0
	v0, v1 := fingerprintWeak32(data[length-64:], uint64(length), y)
	w0, w1 := fingerprintWeak32(data[length-32:], uint64(length)*fingerprintK1, fingerprintK0)
	z += farmShiftMix(v1) * fingerprintK1
	x = bits.RotateLeft64(z+x, -39) * fingerprintK1
	y = bits.RotateLeft64(y, -33) * fingerprintK1
	remaining := (length - 1) &^ 63
	for offset := 0; remaining != 0; offset, remaining = offset+64, remaining-64 {
		block := data[offset:]
		x = bits.RotateLeft64(x+y+v0+binary.LittleEndian.Uint64(block[16:]), -37) * fingerprintK1
		y = bits.RotateLeft64(y+v1+binary.LittleEndian.Uint64(block[48:]), -42) * fingerprintK1
		x ^= w1
		y ^= v0
		z = bits.RotateLeft64(z^w0, -33)
		v0, v1 = fingerprintWeak32(block, v1*fingerprintK1, x+w0)
		w0, w1 = fingerprintWeak32(block[32:], z+w1, y)
		z, x = x, z
	}
	return fingerprintHash128To64(fingerprintHash128To64(v0, w0)+farmShiftMix(y)*fingerprintK1+z,
		fingerprintHash128To64(v1, w1)+x)
}

func fingerprintHash33To64(data []byte) uint64 {
	length := len(data)
	z := binary.LittleEndian.Uint64(data[24:])
	a := binary.LittleEndian.Uint64(data) + (uint64(length)+binary.LittleEndian.Uint64(data[length-16:]))*fingerprintK0
	b := bits.RotateLeft64(a+z, -52)
	c := bits.RotateLeft64(a, -37)
	a += binary.LittleEndian.Uint64(data[8:])
	c += bits.RotateLeft64(a, -7)
	a += binary.LittleEndian.Uint64(data[16:])
	vf := a + z
	vs := b + bits.RotateLeft64(a, -31) + c
	a = binary.LittleEndian.Uint64(data[16:]) + binary.LittleEndian.Uint64(data[length-32:])
	z = binary.LittleEndian.Uint64(data[length-8:])
	b = bits.RotateLeft64(a+z, -52)
	c = bits.RotateLeft64(a, -37)
	a += binary.LittleEndian.Uint64(data[length-24:])
	c += bits.RotateLeft64(a, -7)
	a += binary.LittleEndian.Uint64(data[length-16:])
	wf := a + z
	ws := b + bits.RotateLeft64(a, -31) + c
	r := farmShiftMix((vf+ws)*fingerprintK2 + (wf+vs)*fingerprintK0)
	return farmShiftMix(r*fingerprintK0+vs) * fingerprintK2
}

func fingerprintMurmur64(data []byte, seed uint64) uint64 {
	length := len(data)
	hash := seed ^ uint64(length)*fingerprintK3
	aligned := length &^ 7
	for offset := 0; offset < aligned; offset += 8 {
		loaded := binary.LittleEndian.Uint64(data[offset:])
		hash ^= farmShiftMix(loaded*fingerprintK3) * fingerprintK3
		hash *= fingerprintK3
	}
	if length&7 != 0 {
		var tail [8]byte
		copy(tail[:], data[aligned:])
		hash ^= binary.LittleEndian.Uint64(tail[:])
		hash *= fingerprintK3
	}
	hash = farmShiftMix(hash) * fingerprintK3
	return farmShiftMix(hash)
}
