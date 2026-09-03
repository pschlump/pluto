/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bitmap implements the Redis bit-operation family over byte
// slices: GETBIT/SETBIT, BITCOUNT, BITPOS, BITOP AND|OR|XOR|NOT and the
// BITFIELD GET/SET/INCRBY subcommands with the WRAP/SAT/FAIL overflow
// policies (request note/09-bitmap-bitfield.md, built for Ultima's SETBIT,
// GETBIT, BITCOUNT, BITPOS, BITOP, BITFIELD and BITFIELD_RO; the mirrored
// C is note/redis/src/bitops.c, Redis 8.9.241 — this Redis keeps all the
// bit commands in bitops.c; the note's bitfield.c pointer predates that
// layout).
//
// Bit order is Redis's: bit 0 is the most significant bit of byte 0, bit 7
// the least significant bit of byte 0, bit 8 the MSB of byte 1, and so on
// — the buffer reads as a big-endian bit stream.  A bitmap is nothing but
// a []byte; nothing is stored between calls.
//
// The package is pure functions over []byte (the geo/substring/quick_sort
// precedent): no state, no type parameters (nothing is stored, the
// hyperloglog/union_find precedent), and no _ts twin — everything is safe
// for concurrent use as-is.  Reads never mutate their buffer.  SetBit and
// ExecuteFieldOps return the buffer to use going forward: they write in
// place when no growth is needed and return a new zero-extended slice when
// growth is needed (the append() idiom — always keep the returned slice).
//
// Semantics are Redis-exact — range units (BYTE|BIT), negative indexes
// from the end in the range's unit, zero-fill growth capped at 512 MiB,
// BITPOS's missing-key vs empty-string distinction, and the OVERFLOW
// policies are ported line-for-line from the C and pinned by the corpus
// ported from note/redis/tests/unit/bitops.tcl and bitfield.tcl.
//
// A nil buffer and an empty non-nil buffer mean different things, because
// they mean different things to Redis: a missing key is an infinite array
// of zero bits (BitPos(nil, 0, ...) reports bit 0), while an empty string
// is a zero-length range (BitPos([]byte{}, 0, ...) reports not found).
package bitmap

import "errors"

// Unit is the indexing unit of a BITCOUNT/BITPOS range — BYTE (the
// default) counts bytes, BIT counts bits.  Both commands accept negative
// start/end indexes measured from the end in the chosen unit.
type Unit int

const (
	// ByteUnit indexes a range in bytes (BITCOUNT key start end BYTE).
	ByteUnit Unit = iota
	// BitUnit indexes a range in bits (BITCOUNT key start end BIT).
	BitUnit
)

// MaxBytes is the largest buffer SetBit and the BITFIELD write ops grow
// to: Redis's proto-max-bulk-len default of 512 MiB.  An offset that
// would address at or past this bound is rejected with ErrBitOffset —
// the GETBIT/SETBIT maximum offset of 2^32-1 follows from it.
const MaxBytes = 1 << 29

// MaxBitOffset is the highest bit offset the write paths accept,
// 2^32 - 1 (offset/8 must stay below MaxBytes).
const MaxBitOffset = MaxBytes*8 - 1

// Errors reported by the write paths and ExecuteFieldOps, compared with
// errors.Is.  They carry the *data* rules (a caller's command layer maps
// them to Redis's reply strings); the only panics in the package are the
// two argument shapes with no sane answer at all, on BitPos and the range
// unit — see the README.
var (
	// ErrBitOffset — bit offset is not an integer or out of range: a
	// write offset addressing at or past MaxBytes, or a FieldOp with a
	// negative or oversized offset.
	ErrBitOffset = errors.New("bitmap: bit offset is not an integer or out of range")

	// ErrBitValue — bit is not an integer or out of range: SetBit's bit
	// argument is neither 0 nor 1.
	ErrBitValue = errors.New("bitmap: bit is not an integer or out of range")

	// ErrBitfieldType — invalid bitfield type: FieldOp.Bits outside
	// 1..64 for signed fields or 1..63 for unsigned (u64 is not
	// supported but i64 is — the results are int64, which cannot carry
	// an unsigned 64-bit value).
	ErrBitfieldType = errors.New("bitmap: invalid bitfield type. Use something like i16 u8. Note that u64 is not supported but i64 is")

	// ErrInvalidFieldOp — a FieldOp whose Kind or OverflowPolicy is not
	// one of the defined constants (a zero FieldOp has valid Kind and
	// policy; this fires only on out-of-range values).
	ErrInvalidFieldOp = errors.New("bitmap: invalid FieldOp Kind or OverflowPolicy")
)

// GetBit returns the bit at offset — bit 0 is the MSB of byte 0 — or 0
// for any offset past the end of buf (the GETBIT answer for a missing or
// short key; Redis's offset validation is the command layer's concern).
//
// Complexity is O(1).
func GetBit(buf []byte, offset uint64) int {
	i := offset >> 3
	if i >= uint64(len(buf)) {
		return 0
	}
	return int(buf[i]>>(7-(offset&7))) & 1
}

// SetBit sets the bit at offset to bit (0 or 1) and returns the buffer to
// use going forward together with the bit's previous value (0 when the
// buffer grew — the growth is zero-filled, the SETBIT contract).
//
// When the addressed byte is already inside buf the write happens in
// place and buf itself is returned; when offset points past the end a new
// zero-extended slice is allocated and buf is left untouched.  Always
// keep the returned buffer.
//
// Errors: ErrBitValue if bit is not 0 or 1; ErrBitOffset if offset is
// greater than MaxBitOffset (it would grow the buffer past MaxBytes).
// On error buf is returned unchanged.
//
// Complexity is O(1) plus the growth allocation.
func SetBit(buf []byte, offset uint64, bit int) ([]byte, int, error) {
	if bit != 0 && bit != 1 {
		return buf, 0, ErrBitValue
	}
	if offset > MaxBitOffset {
		return buf, 0, ErrBitOffset
	}
	i := offset >> 3
	if i >= uint64(len(buf)) {
		grown := make([]byte, i+1)
		copy(grown, buf)
		buf = grown
	}
	sh := 7 - (offset & 7)
	old := int(buf[i]>>sh) & 1
	if bit == 1 {
		buf[i] |= 1 << sh
	} else {
		buf[i] &^= 1 << sh
	}
	return buf, old, nil
}
