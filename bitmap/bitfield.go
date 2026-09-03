/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import "math"

// FieldKind selects which BITFIELD subcommand an op performs.
type FieldKind int

const (
	// FieldGet is GET <type> <offset> — read the field, zero-padded past
	// the end of the buffer.  It never fails.
	FieldGet FieldKind = iota
	// FieldSet is SET <type> <offset> <value> — write Value (subject to
	// the overflow policy) and report the PREVIOUS value.
	FieldSet
	// FieldIncrBy is INCRBY <type> <offset> <increment> — add Value to
	// the current field value (subject to the overflow policy) and
	// report the NEW value.
	FieldIncrBy
)

// OverflowPolicy is what a SET or INCRBY does when its result does not
// fit the field — Redis's OVERFLOW subcommand.  It is per-op state here:
// the caller resolves the command's sticky OVERFLOW-WRAP/SAT/FAIL
// prefixes into each FieldOp's Overflow when building the slice (WRAP is
// the zero value, the default policy).
type OverflowPolicy int

const (
	// OverflowWrap wraps modulo 2^bits — two's complement for signed
	// fields (i8 127+1 becomes -128), a plain mask for unsigned.
	OverflowWrap OverflowPolicy = iota
	// OverflowSat saturates at the field's limits (i8: -128/127,
	// u8: 0/255; signed and unsigned limits differ).
	OverflowSat
	// OverflowFail writes nothing and reports the op as failed.
	OverflowFail
)

// FieldOp is one BITFIELD subcommand, ready to execute.  Offset is in
// bits from the MSB of byte 0 (the "#<idx>" form — the index multiplied
// by the type width — is resolved by the caller before building the op).
// Bits is the field width: 1..64 for signed fields, 1..63 for unsigned
// (u64 is not supported but i64 is — an unsigned 64-bit value cannot be
// carried by the int64 results).  Value is the SET value or the INCRBY
// increment.  Overflow is the policy in force for this op.
type FieldOp struct {
	Kind     FieldKind
	Signed   bool
	Bits     int
	Offset   int64 // bit offset; "#n" multiplier resolved by caller
	Value    int64 // Set/IncrBy value
	Overflow OverflowPolicy
}

// ExecuteFieldOps applies ops to buf left-to-right and returns the buffer
// to use going forward, one result per op, and a parallel failed slice —
// BITFIELD (or BITFIELD_RO when every op is a FieldGet).
//
// The whole slice is validated before anything runs (as the command
// parses all subcommands before executing any): on error the buffer is
// returned unchanged with nil results.  Errors: ErrInvalidFieldOp for a
// Kind or Overflow outside the defined constants; ErrBitfieldType for a
// Bits outside the field's sign range; ErrBitOffset for a negative
// offset or one addressing at or past MaxBytes (the limit applies to
// every op, reads included — the getBitOffsetFromArgument rule).
//
// Execution: FieldGet reads the field zero-padded past the buffer end.
// FieldSet reports the previous value; FieldIncrBy reports the new one.
// A SET/INCRBY whose result leaves the field under OverflowWrap wraps,
// under OverflowSat saturates, and under OverflowFail marks the op
// failed (results[i] is then meaningless, 0) and writes nothing.  The
// buffer is grown zero-filled to the furthest bit any write op touches
// before the first op runs (growth happens even if the writes then fail,
// as in the C) — writing in place when it already fits, otherwise
// returning a new slice and leaving buf untouched (the SetBit idiom).
// GET-only slices never grow: a nil buffer stays nil.
//
// Complexity is O(Σ bits) plus the growth allocation.
func ExecuteFieldOps(buf []byte, ops []FieldOp) ([]byte, []int64, []bool, error) {
	// Validate everything first — nothing executes on a bad op.
	var highest uint64
	hasWrite := false
	for i := range ops {
		op := &ops[i]
		if op.Kind != FieldGet && op.Kind != FieldSet && op.Kind != FieldIncrBy {
			return buf, nil, nil, ErrInvalidFieldOp
		}
		if op.Overflow != OverflowWrap && op.Overflow != OverflowSat && op.Overflow != OverflowFail {
			return buf, nil, nil, ErrInvalidFieldOp
		}
		if op.Signed {
			if op.Bits < 1 || op.Bits > 64 {
				return buf, nil, nil, ErrBitfieldType
			}
		} else {
			if op.Bits < 1 || op.Bits > 63 {
				return buf, nil, nil, ErrBitfieldType
			}
		}
		if op.Offset < 0 || uint64(op.Offset) > MaxBitOffset {
			return buf, nil, nil, ErrBitOffset
		}
		if op.Kind != FieldGet {
			hasWrite = true
			if end := uint64(op.Offset) + uint64(op.Bits) - 1; end > highest {
				highest = end
			}
		}
	}
	if hasWrite {
		// Grow to the furthest written bit, zero-filled (the
		// lookupStringForBitCommand step).  An offset that passed
		// validation keeps the growth inside MaxBytes.
		if need := int(highest>>3) + 1; need > len(buf) {
			grown := make([]byte, need)
			copy(grown, buf)
			buf = grown
		}
	}

	results := make([]int64, len(ops))
	failed := make([]bool, len(ops))
	for i, op := range ops {
		off := uint64(op.Offset)
		switch op.Kind {
		case FieldGet:
			if op.Signed {
				results[i] = getFieldSigned(buf, off, op.Bits)
			} else {
				results[i] = int64(getFieldUnsigned(buf, off, op.Bits))
			}

		case FieldSet, FieldIncrBy:
			if op.Signed {
				oldval := getFieldSigned(buf, off, op.Bits)
				var newval, wrapped int64
				var overflow int
				if op.Kind == FieldIncrBy {
					overflow, wrapped = checkSignedOverflow(oldval, op.Value, op.Bits, op.Overflow)
					if overflow != 0 {
						newval = wrapped
					} else {
						newval = oldval + op.Value
					}
					results[i] = newval
				} else {
					newval = op.Value
					overflow, wrapped = checkSignedOverflow(newval, 0, op.Bits, op.Overflow)
					if overflow != 0 {
						newval = wrapped
					}
					results[i] = oldval
				}
				if overflow != 0 && op.Overflow == OverflowFail {
					failed[i] = true
					results[i] = 0
				} else {
					setFieldSigned(buf, off, op.Bits, newval)
				}
			} else {
				oldval := getFieldUnsigned(buf, off, op.Bits)
				var newval, wrapped uint64
				var overflow int
				if op.Kind == FieldIncrBy {
					newval = oldval + uint64(op.Value)
					overflow, wrapped = checkUnsignedOverflow(oldval, op.Value, op.Bits, op.Overflow)
					if overflow != 0 {
						newval = wrapped
					}
					results[i] = int64(newval)
				} else {
					newval = uint64(op.Value)
					overflow, wrapped = checkUnsignedOverflow(newval, 0, op.Bits, op.Overflow)
					if overflow != 0 {
						newval = wrapped
					}
					results[i] = int64(oldval)
				}
				if overflow != 0 && op.Overflow == OverflowFail {
					failed[i] = true
					results[i] = 0
				} else {
					setFieldUnsigned(buf, off, op.Bits, newval)
				}
			}
		}
	}
	return buf, results, failed, nil
}

// getFieldUnsigned reads a bits-wide unsigned field at bit offset — the
// getUnsignedBitfield port, bit-by-bit MSB-first.  Bytes past the end of
// buf read as zero (the C copies up to 9 zero-padded bytes to a local
// buffer; reading zeros directly is the same contract).
func getFieldUnsigned(buf []byte, offset uint64, bits int) uint64 {
	var value uint64
	for j := 0; j < bits; j++ {
		var bitval uint64
		if i := offset >> 3; i < uint64(len(buf)) {
			bitval = uint64(buf[i]>>(7-(offset&7))) & 1
		}
		value = value<<1 | bitval
		offset++
	}
	return value
}

// getFieldSigned reads a bits-wide signed field, sign-extended — the
// getSignedBitfield port (two's complement; the top bit propagates).
func getFieldSigned(buf []byte, offset uint64, bits int) int64 {
	value := getFieldUnsigned(buf, offset, bits)
	if bits < 64 && value&(uint64(1)<<uint(bits-1)) != 0 {
		value |= ^uint64(0) << uint(bits)
	}
	return int64(value)
}

// setFieldUnsigned writes a bits-wide field at bit offset — the
// setUnsignedBitfield port.  The buffer must already cover the field (the
// caller's pre-growth guarantees it for the write ops).
func setFieldUnsigned(buf []byte, offset uint64, bits int, value uint64) {
	for j := 0; j < bits; j++ {
		bitval := (value >> uint(bits-1-j)) & 1
		i := offset >> 3
		sh := 7 - (offset & 7)
		buf[i] = buf[i]&^(byte(1)<<sh) | byte(bitval)<<sh
		offset++
	}
}

// setFieldSigned writes a signed value as its two's-complement bits —
// the setSignedBitfield port (the uint64 conversion adds 2^64 for
// negatives, exactly the C's comment).
func setFieldSigned(buf []byte, offset uint64, bits int, value int64) {
	setFieldUnsigned(buf, offset, bits, uint64(value))
}

// checkUnsignedOverflow mirrors checkUnsignedBitfieldOverflow: reports
// whether value+incr fits a bits-wide unsigned field (0), overflows (1)
// or underflows (-1), and the value the policy says to store when it
// does not (WRAP masks, SAT clamps to max or 0, FAIL is signalled only).
func checkUnsignedOverflow(value uint64, incr int64, bits int, policy OverflowPolicy) (int, uint64) {
	max := uint64(1)<<uint(bits) - 1 // bits <= 63, the shift cannot overflow
	maxincr := int64(max - value)    // uint64 wrap, two's-complement reinterpret — as in the C
	minincr := int64(-value)
	if value > max || (incr > 0 && incr > maxincr) {
		if policy == OverflowWrap {
			return 1, (value + uint64(incr)) &^ (^uint64(0) << uint(bits))
		}
		if policy == OverflowSat {
			return 1, max
		}
		return 1, 0
	} else if incr < 0 && incr < minincr {
		if policy == OverflowWrap {
			return -1, (value + uint64(incr)) &^ (^uint64(0) << uint(bits))
		}
		if policy == OverflowSat {
			return -1, 0
		}
		return -1, 0
	}
	return 0, 0
}

// checkSignedOverflow mirrors checkSignedBitfieldOverflow (including its
// strict inequalities — a result landing exactly on a limit is not an
// overflow, which the boundary tests pin).  maxincr/minincr may wrap;
// they are used only after the value range is known, as the C notes.
func checkSignedOverflow(value, incr int64, bits int, policy OverflowPolicy) (int, int64) {
	var max int64
	if bits == 64 {
		max = math.MaxInt64
	} else {
		max = int64(1)<<uint(bits-1) - 1
	}
	min := -max - 1
	maxincr := int64(uint64(max) - uint64(value))
	minincr := int64(uint64(min) - uint64(value))
	if value > max || (bits != 64 && incr > maxincr) || (value >= 0 && incr > 0 && incr > maxincr) {
		if policy == OverflowWrap {
			return 1, signedWrap(value, incr, bits)
		}
		if policy == OverflowSat {
			return 1, max
		}
		return 1, 0
	} else if value < min || (bits != 64 && incr < minincr) || (value < 0 && incr < 0 && incr < minincr) {
		if policy == OverflowWrap {
			return -1, signedWrap(value, incr, bits)
		}
		if policy == OverflowSat {
			return -1, min
		}
		return -1, 0
	}
	return 0, 0
}

// signedWrap is the handle_wrap tail of checkSignedBitfieldOverflow: add
// in uint64 (defined wraparound), then sign-extend or mask to the field
// width so the result lands in [min, max].
func signedWrap(value, incr int64, bits int) int64 {
	c := uint64(value) + uint64(incr)
	if bits < 64 {
		msb := uint64(1) << uint(bits-1)
		mask := ^uint64(0) << uint(bits)
		if c&msb != 0 {
			c |= mask
		} else {
			c &^= mask
		}
	}
	return int64(c)
}
