/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import (
	"encoding/binary"
	"testing"
)

// benchFill returns a deterministic pseudo-random buffer of n bytes
// (xorshift64* — no allocation-order dependence on the test RNG).
func benchFill(n int) []byte {
	buf := make([]byte, n)
	var s uint64 = 0x9E3779B97F4A7C15
	for i := range buf {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		buf[i] = byte(s >> 32)
	}
	return buf
}

func benchSize(b *testing.B, n int) {
	if testing.Short() {
		b.Skip("skipping large benchmark in short mode")
	}
}

func BenchmarkBitCount1MB(b *testing.B) {
	buf := benchFill(1 << 20)
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for range b.N {
		BitCount(buf, 0, -1, ByteUnit)
	}
}

func BenchmarkBitCount100MB(b *testing.B) {
	benchSize(b, 1)
	buf := benchFill(100 << 20)
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for range b.N {
		BitCount(buf, 0, -1, ByteUnit)
	}
}

func BenchmarkBitOpOr1MB(b *testing.B) {
	a, c := benchFill(1<<20), benchFill(1<<20-7) // ragged lengths cross the word steps
	b.SetBytes(int64(len(a)))
	b.ResetTimer()
	for range b.N {
		BitOpOr(a, c)
	}
}

func BenchmarkBitOpOr100MB(b *testing.B) {
	benchSize(b, 1)
	a, c := benchFill(100<<20), benchFill(100<<20-7)
	b.SetBytes(int64(len(a)))
	b.ResetTimer()
	for range b.N {
		BitOpOr(a, c)
	}
}

func BenchmarkBitOpAnd1MB(b *testing.B) {
	a, c, d := benchFill(1<<20), benchFill(1<<20-7), benchFill(1<<20-64)
	b.SetBytes(int64(len(a)))
	b.ResetTimer()
	for range b.N {
		BitOpAnd(a, c, d)
	}
}

func BenchmarkBitOpNot1MB(b *testing.B) {
	a := benchFill(1 << 20)
	b.SetBytes(int64(len(a)))
	b.ResetTimer()
	for range b.N {
		BitOpNot(a)
	}
}

func BenchmarkBitPos1MB(b *testing.B) {
	// No matching bit until the last word: the worst case for the scan.
	buf := benchFill(1 << 20)
	for i := range buf {
		buf[i] = 0xFF
	}
	buf[len(buf)-1] = 0x7F
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for range b.N {
		BitPos(buf, 0, 0, 0, ByteUnit, false)
	}
}

func BenchmarkGetBit(b *testing.B) {
	buf := benchFill(1024)
	var offsets [64]uint64
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint64(buf[i*8:]) % (1024 * 8)
	}
	b.ResetTimer()
	for i := range b.N {
		GetBit(buf, offsets[i&63])
	}
}

func BenchmarkSetBitInPlace(b *testing.B) {
	buf := benchFill(4096)
	b.ResetTimer()
	for i := uint64(0); i < uint64(b.N); i++ {
		// The offsets stay inside the buffer, so this is the no-growth
		// in-place path (errors impossible: bit is 0 or 1, offset small).
		_, _, _ = SetBit(buf, (i*101)%32768, int(i&1))
	}
}

func BenchmarkExecuteFieldOps(b *testing.B) {
	// Eight mixed ops on an 8-byte buffer, all in place.
	buf := benchFill(8)
	ops := []FieldOp{
		{Kind: FieldGet, Bits: 8},
		{Kind: FieldGet, Signed: true, Bits: 16, Offset: 8},
		{Kind: FieldSet, Bits: 8, Offset: 16, Value: 42},
		{Kind: FieldIncrBy, Bits: 32, Offset: 24, Value: 7},
		{Kind: FieldGet, Signed: true, Bits: 64, Offset: 0},
		{Kind: FieldSet, Signed: true, Bits: 8, Offset: 56, Value: -1},
		{Kind: FieldIncrBy, Bits: 8, Offset: 32, Value: -100, Overflow: OverflowWrap},
		{Kind: FieldGet, Bits: 8, Offset: 8},
	}
	b.ResetTimer()
	for range b.N {
		_, _, _, _ = ExecuteFieldOps(buf, ops)
	}
}
