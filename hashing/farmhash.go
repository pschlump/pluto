/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.

FarmHash Fingerprint64 is by Geoff Pike and contributors. This behavioral
port follows Guava's Apache-2.0 implementation at the commit recorded in
docs/guava-to-dev-plan.md.
*/

package hashing

import (
	"encoding/binary"
	"math/bits"
)

const (
	farmK0 = uint64(0xc3a5c85c97cb3127)
	farmK1 = uint64(0xb492b66fbe98f273)
	farmK2 = uint64(0x9ae16a3b2f90404f)
)

// FarmHashFingerprint64 returns Guava-compatible FarmHash Fingerprint64.
func FarmHashFingerprint64() HashFunction {
	return newHashFunction(64, func(data []byte) HashCode {
		return hashFromUint64(farmFingerprint64(data))
	})
}

func farmFingerprint64(data []byte) uint64 {
	if len(data) <= 64 {
		return farmHash0To64(data)
	}
	return farmHash65Plus(data)
}

func farmShiftMix(value uint64) uint64 { return value ^ value>>47 }

func farmHashLength16(u, v, mul uint64) uint64 {
	a := (u ^ v) * mul
	a ^= a >> 47
	b := (v ^ a) * mul
	b ^= b >> 47
	b *= mul
	return b
}

func farmHash0To64(data []byte) uint64 {
	length := len(data)
	if length <= 32 {
		if length <= 16 {
			if length >= 8 {
				mul := farmK2 + uint64(length)*2
				a := binary.LittleEndian.Uint64(data) + farmK2
				b := binary.LittleEndian.Uint64(data[length-8:])
				c := bits.RotateLeft64(b, -37)*mul + a
				d := (bits.RotateLeft64(a, -25) + b) * mul
				return farmHashLength16(c, d, mul)
			}
			if length >= 4 {
				mul := farmK2 + uint64(length)*2
				a := uint64(binary.LittleEndian.Uint32(data))
				b := uint64(binary.LittleEndian.Uint32(data[length-4:]))
				return farmHashLength16(uint64(length)+(a<<3), b, mul)
			}
			if length > 0 {
				a, b, c := data[0], data[length>>1], data[length-1]
				y := uint64(a) + uint64(b)<<8
				z := uint64(length) + uint64(c)<<2
				return farmShiftMix(y*farmK2^z*farmK0) * farmK2
			}
			return farmK2
		}
		mul := farmK2 + uint64(length)*2
		a := binary.LittleEndian.Uint64(data) * farmK1
		b := binary.LittleEndian.Uint64(data[8:])
		c := binary.LittleEndian.Uint64(data[length-8:]) * mul
		d := binary.LittleEndian.Uint64(data[length-16:]) * farmK2
		return farmHashLength16(bits.RotateLeft64(a+b, -43)+bits.RotateLeft64(c, -30)+d,
			a+bits.RotateLeft64(b+farmK2, -18)+c, mul)
	}
	mul := farmK2 + uint64(length)*2
	a := binary.LittleEndian.Uint64(data) * farmK2
	b := binary.LittleEndian.Uint64(data[8:])
	c := binary.LittleEndian.Uint64(data[length-8:]) * mul
	d := binary.LittleEndian.Uint64(data[length-16:]) * farmK2
	y := bits.RotateLeft64(a+b, -43) + bits.RotateLeft64(c, -30) + d
	z := farmHashLength16(y, a+bits.RotateLeft64(b+farmK2, -18)+c, mul)
	e := binary.LittleEndian.Uint64(data[16:]) * mul
	f := binary.LittleEndian.Uint64(data[24:])
	g := (y + binary.LittleEndian.Uint64(data[length-32:])) * mul
	h := (z + binary.LittleEndian.Uint64(data[length-24:])) * mul
	return farmHashLength16(bits.RotateLeft64(e+f, -43)+bits.RotateLeft64(g, -30)+h,
		e+bits.RotateLeft64(f+a, -18)+g, mul)
}

func farmHash65Plus(data []byte) uint64 {
	seed := uint64(81)
	x := seed*farmK2 + binary.LittleEndian.Uint64(data)
	y := seed*farmK1 + 113
	z := farmShiftMix(y*farmK2+113) * farmK2
	var v0, v1, w0, w1 uint64
	tail := (len(data) - 1) & 63
	end := (len(data) - 1) &^ 63
	for offset := 0; ; offset += 64 {
		block := data[offset:]
		part1 := binary.LittleEndian.Uint64(block[8:])
		part5 := binary.LittleEndian.Uint64(block[40:])
		part6 := binary.LittleEndian.Uint64(block[48:])
		x = bits.RotateLeft64(x+y+v0+part1, -37) * farmK1
		y = bits.RotateLeft64(y+v1+part6, -42) * farmK1
		x ^= w1
		y += v0 + part5
		z = bits.RotateLeft64(z+w0, -33) * farmK1
		part0 := binary.LittleEndian.Uint64(block)
		part2 := binary.LittleEndian.Uint64(block[16:])
		part3 := binary.LittleEndian.Uint64(block[24:])
		vSeedA := v1*farmK1 + part0
		vSeedB := bits.RotateLeft64(x+w0+vSeedA+part3, -21)
		vC := vSeedA
		vSeedA += part1 + part2
		vSeedB += bits.RotateLeft64(vSeedA, -44)
		v0 = vSeedA + part3
		v1 = vSeedB + vC
		part4 := binary.LittleEndian.Uint64(block[32:])
		part7 := binary.LittleEndian.Uint64(block[56:])
		wSeedA := z + w1 + part4
		wSeedB := bits.RotateLeft64(y+part2+wSeedA+part7, -21)
		wC := wSeedA
		wSeedA += part5 + part6
		wSeedB += bits.RotateLeft64(wSeedA, -44)
		w0 = wSeedA + part7
		w1 = wSeedB + wC
		x, z = z, x
		if offset+64 == end {
			break
		}
	}
	return farmFinalMix(x, y, z, v0, v1, w0, w1, data[end+tail-63:], tail)
}

func farmFinalMix(x, y, z, v0, v1, w0, w1 uint64, data []byte, remainder int) uint64 {
	part0 := binary.LittleEndian.Uint64(data)
	part1 := binary.LittleEndian.Uint64(data[8:])
	part2 := binary.LittleEndian.Uint64(data[16:])
	part3 := binary.LittleEndian.Uint64(data[24:])
	part4 := binary.LittleEndian.Uint64(data[32:])
	part5 := binary.LittleEndian.Uint64(data[40:])
	part6 := binary.LittleEndian.Uint64(data[48:])
	part7 := binary.LittleEndian.Uint64(data[56:])
	mul := farmK1 + ((z & 0xff) << 1)
	w0 += uint64(remainder)
	v0 += w0
	w0 += v0
	x = bits.RotateLeft64(x+y+v0+part1, -37) * mul
	y = bits.RotateLeft64(y+v1+part6, -42) * mul
	x ^= w1 * 9
	y += v0*9 + part5
	z = bits.RotateLeft64(z+w0, -33) * mul
	vSeedA := v1*mul + part0
	vSeedB := bits.RotateLeft64(x+w0+vSeedA+part3, -21)
	vC := vSeedA
	vSeedA += part1 + part2
	vSeedB += bits.RotateLeft64(vSeedA, -44)
	v0 = vSeedA + part3
	v1 = vSeedB + vC
	wSeedA := z + w1 + part4
	wSeedB := bits.RotateLeft64(y+part2+wSeedA+part7, -21)
	wC := wSeedA
	wSeedA += part5 + part6
	wSeedB += bits.RotateLeft64(wSeedA, -44)
	w0 = wSeedA + part7
	w1 = wSeedB + wC
	return farmHashLength16(farmHashLength16(v0, w0, mul)+farmShiftMix(y)*farmK0+x,
		farmHashLength16(v1, w1, mul)+z, mul)
}
