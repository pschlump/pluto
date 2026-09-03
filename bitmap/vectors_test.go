/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Frozen differential vectors ported from the Redis test corpus
// note/redis/tests/unit/bitops.tcl and bitfield.tcl (Redis 8.9.241).
// Every table here is a transcription of a TCL assertion — the expected
// values are Redis's own, not derived from this implementation.

package bitmap

import (
	"bytes"
	"testing"
)

func bitPosOf(t *testing.T, buf []byte, bit int, start, end int64, unit Unit, endGiven bool, want int64) {
	t.Helper()
	pos, found := BitPos(buf, bit, start, end, unit, endGiven)
	if found != (want != -1) || pos != want {
		t.Errorf("BitPos(%q, %d, %d, %d, %v, %v) = (%d, %v), want %d",
			buf, bit, start, end, unit, endGiven, pos, found, want)
	}
}

// countBitsAt is the frozen-value cross-check for the bit-unit ranges:
// an inline MSB-first count over the inclusive bit range [lo, hi],
// independent of the masked word-wise implementation.
func countBitsAt(buf []byte, lo, hi int64) int64 {
	n := int64(0)
	for p := lo; p <= hi; p++ {
		n += int64(buf[p>>3]>>(7-(p&7))) & 1
	}
	return n
}

func TestBitCountVectors(t *testing.T) {
	// bitops.tcl "BITCOUNT against test vector #N": whole-string counts.
	for _, tc := range []struct {
		buf  string
		want int64
	}{
		{"", 0},
		{"\xaa", 4},
		{"\x00\x00\xff", 8},
		{"foobar", 26},
		{"123", 10},
	} {
		buf := []byte(tc.buf)
		if got := BitCount(buf, 0, -1, ByteUnit); got != tc.want {
			t.Errorf("BitCount(%q, 0, -1, byte) = %d, want %d", tc.buf, got, tc.want)
		}
		if got := BitCount(buf, 0, -1, BitUnit); got != tc.want {
			t.Errorf("BitCount(%q, 0, -1, bit) = %d, want %d", tc.buf, got, tc.want)
		}
		// The bit-range helper over the whole buffer must agree.
		if tc.buf != "" {
			if got := countBitsAt(buf, 0, int64(len(buf))*8-1); got != tc.want {
				t.Errorf("countBitsAt(%q) = %d, want %d", tc.buf, got, tc.want)
			}
		}
	}

	// "BITCOUNT returns 0 with out of range indexes".
	if got := BitCount([]byte("xxxx"), 4, 10, ByteUnit); got != 0 {
		t.Errorf("out-of-range byte start: got %d", got)
	}
	if got := BitCount([]byte("xxxx"), 32, 87, BitUnit); got != 0 {
		t.Errorf("out-of-range bit start: got %d", got)
	}

	// "BITCOUNT returns 0 with negative indexes where start > end",
	// including against a non existing key.
	for _, buf := range [][]byte{[]byte("xxxx"), nil} {
		if got := BitCount(buf, -6, -7, ByteUnit); got != 0 {
			t.Errorf("BitCount(%v, -6, -7, byte) = %d, want 0", buf, got)
		}
		if got := BitCount(buf, -6, -15, BitUnit); got != 0 {
			t.Errorf("BitCount(%v, -6, -15, bit) = %d, want 0", buf, got)
		}
	}
	// "BITCOUNT returns 0 against non existing key".
	if got := BitCount(nil, 0, 1000, BitUnit); got != 0 {
		t.Errorf("BitCount(nil, 0, 1000, bit) = %d, want 0", got)
	}

	// "BITCOUNT with start, end" — byte unit, then bit unit.  The
	// bit-range expectations are frozen here and cross-checked against
	// countBitsAt below.
	s := []byte("foobar")
	for _, tc := range []struct {
		start, end int64
		unit       Unit
		want       int64
	}{
		{0, -1, ByteUnit, 26},
		{1, -2, ByteUnit, 18}, // "ooba"
		{-2, 1, ByteUnit, 0},  // empty
		{0, 1000, ByteUnit, 26},
		{0, -1, BitUnit, 26},
		{10, 14, BitUnit, 4},
		{3, 14, BitUnit, 7},
		{3, 29, BitUnit, 16},
		{10, -34, BitUnit, 4},
		{3, -34, BitUnit, 7},
		{3, -19, BitUnit, 16},
		{-2, 1, BitUnit, 0},
		{0, 1000, BitUnit, 26},
	} {
		if got := BitCount(s, tc.start, tc.end, tc.unit); got != tc.want {
			t.Errorf("BitCount(foobar, %d, %d, %v) = %d, want %d",
				tc.start, tc.end, tc.unit, got, tc.want)
		}
		if tc.want > 0 {
			// Resolve the clamps independently and recount bit-wise.
			totlen := int64(len(s))
			if tc.unit == BitUnit {
				totlen *= 8
			}
			lo, hi := tc.start, tc.end
			if hi < 0 {
				hi += totlen
			}
			if hi >= totlen {
				hi = totlen - 1
			}
			if lo < 0 {
				lo += totlen
			}
			if tc.unit == ByteUnit {
				lo, hi = lo*8, hi*8+7
			}
			if got := countBitsAt(s, lo, hi); got != tc.want {
				t.Errorf("countBitsAt(foobar, %d, %d) = %d, want %d", lo, hi, got, tc.want)
			}
		}
	}

	// "BITCOUNT misaligned prefix" and "misaligned prefix + full words +
	// remainder".
	if got := BitCount([]byte("ab"), 1, -1, ByteUnit); got != 3 {
		t.Errorf("BitCount(ab, 1, -1, byte) = %d, want 3", got)
	}
	mis := []byte("__PP" + string(bytes.Repeat([]byte{'x'}, 16)) + "RR__")
	if len(mis) != 24 {
		t.Fatalf("misaligned fixture built %d bytes, want 24", len(mis))
	}
	if got := BitCount(mis, 2, -3, ByteUnit); got != 74 {
		t.Errorf("BitCount(mis, 2, -3, byte) = %d, want 74", got)
	}
}

func TestBitPosVectors(t *testing.T) {
	// "BITPOS bit=0/1 with empty key" — a missing key is an infinite
	// zero array; both the no-end and the explicit-range forms.
	bitPosOf(t, nil, 0, 0, -1, ByteUnit, true, 0)
	bitPosOf(t, nil, 0, 0, -1, BitUnit, true, 0)
	bitPosOf(t, nil, 1, 0, -1, ByteUnit, true, -1)
	bitPosOf(t, nil, 1, 0, -1, BitUnit, true, -1)
	// The no-end form reports the first bit of the infinite zeros.
	bitPosOf(t, nil, 0, 0, 0, ByteUnit, false, 0)
	bitPosOf(t, nil, 1, 0, 0, ByteUnit, false, -1)

	// "with string less than 1 word works".
	bitPosOf(t, []byte("\xff\xf0\x00"), 0, 0, 0, ByteUnit, false, 12)
	bitPosOf(t, []byte("\xff\xf0\x00"), 0, 0, -1, BitUnit, true, 12)
	bitPosOf(t, []byte("\x00\x0f\x00"), 1, 0, 0, ByteUnit, false, 12)
	bitPosOf(t, []byte("\x00\x0f\x00"), 1, 0, -1, BitUnit, true, 12)

	// "starting at unaligned address" (byte 1 / bit 8 as the start).
	bitPosOf(t, []byte("\xff\xf0\x00"), 0, 1, 0, ByteUnit, false, 12)
	bitPosOf(t, []byte("\xff\xf0\x00"), 0, 1, -1, BitUnit, true, 12)
	bitPosOf(t, []byte("\x00\x0f\xff"), 1, 1, 0, ByteUnit, false, 12)
	bitPosOf(t, []byte("\x00\x0f\xff"), 1, 1, -1, BitUnit, true, 12)

	// "unaligned+full word+reminder": 3 prefix bytes, 24 full-word
	// bytes, then the first opposite bit — every start below still
	// lands on bit 216.
	ones := append(bytes.Repeat([]byte{0xFF}, 27), 0x0F)
	zeros := append(bytes.Repeat([]byte{0x00}, 27), 0xF0)
	for start := int64(0); start <= 8; start++ {
		bitPosOf(t, ones, 0, start, 0, ByteUnit, false, 216)
	}
	for start := int64(1); start <= 65; start += 8 {
		bitPosOf(t, ones, 0, start, -1, BitUnit, true, 216)
	}
	for start := int64(0); start <= 8; start++ {
		bitPosOf(t, zeros, 1, start, 0, ByteUnit, false, 216)
	}
	for start := int64(1); start <= 65; start += 8 {
		bitPosOf(t, zeros, 1, start, -1, BitUnit, true, 216)
	}

	// "bit=1 returns -1 if string is all 0 bits", sizes 0..20 (the
	// empty string among them — an existing empty key is not a missing
	// key).
	for n := 0; n <= 20; n++ {
		buf := bytes.Repeat([]byte{0x00}, n)
		bitPosOf(t, buf, 1, 0, 0, ByteUnit, false, -1)
		bitPosOf(t, buf, 1, 0, -1, BitUnit, true, -1)
	}

	// "bit=0 works with intervals" / "bit=1 works with intervals" over
	// "\x00\xff\x00" — all explicit-end ranges.
	i0 := []byte("\x00\xff\x00")
	for _, tc := range []struct {
		bit        int
		start, end int64
		unit       Unit
		want       int64
	}{
		{0, 0, -1, ByteUnit, 0},
		{0, 1, -1, ByteUnit, 16},
		{0, 2, -1, ByteUnit, 16},
		{0, 2, 200, ByteUnit, 16},
		{0, 1, 1, ByteUnit, -1},
		{0, 0, -1, BitUnit, 0},
		{0, 8, -1, BitUnit, 16},
		{0, 16, -1, BitUnit, 16},
		{0, 16, 200, BitUnit, 16},
		{0, 8, 8, BitUnit, -1},
		{1, 0, -1, ByteUnit, 8},
		{1, 1, -1, ByteUnit, 8},
		{1, 2, -1, ByteUnit, -1},
		{1, 2, 200, ByteUnit, -1},
		{1, 1, 1, ByteUnit, 8},
		{1, 0, -1, BitUnit, 8},
		{1, 8, -1, BitUnit, 8},
		{1, 16, -1, BitUnit, -1},
		{1, 16, 200, BitUnit, -1},
		{1, 8, 8, BitUnit, 8},
	} {
		bitPosOf(t, i0, tc.bit, tc.start, tc.end, tc.unit, true, tc.want)
	}

	// "BITPOS bit=0 changes behavior if end is given": without an
	// explicit end the all-ones string answers the first bit past it;
	// with one, not-found.
	allOnes := []byte("\xff\xff\xff")
	bitPosOf(t, allOnes, 0, 0, 0, ByteUnit, false, 24)
	bitPosOf(t, allOnes, 0, 0, -1, ByteUnit, true, -1)
	bitPosOf(t, allOnes, 0, 0, -1, BitUnit, true, -1)
}

// fieldCall runs one BITFIELD command's worth of ops against buf and
// checks the reply array: want entries parallel the ops, with nil for
// the FAILED (null) replies.
func fieldCall(t *testing.T, name string, buf []byte, ops []FieldOp, want []*int64) []byte {
	t.Helper()
	newBuf, results, failed, err := ExecuteFieldOps(buf, ops)
	if err != nil {
		t.Fatalf("%s: ExecuteFieldOps error: %v", name, err)
	}
	if len(results) != len(ops) || len(failed) != len(ops) {
		t.Fatalf("%s: got %d/%d results/failed for %d ops", name, len(results), len(failed), len(ops))
	}
	for i, w := range want {
		if w == nil {
			if !failed[i] {
				t.Errorf("%s: op %d = (%d, failed=false), want failed", name, i, results[i])
			}
			continue
		}
		if failed[i] {
			t.Errorf("%s: op %d failed, want %d", name, i, *w)
			continue
		}
		if results[i] != *w {
			t.Errorf("%s: op %d = %d, want %d", name, i, results[i], *w)
		}
	}
	return newBuf
}

func i64p(v int64) *int64 { return &v }

func TestBitfieldVectors(t *testing.T) {
	// "BITFIELD signed SET and GET basics": fresh key per command.
	fieldCall(t, "signed basics 1", nil, []FieldOp{{Kind: FieldSet, Signed: true, Bits: 8, Value: -100}}, []*int64{i64p(0)})
	fieldCall(t, "signed basics 2", nil, []FieldOp{{Kind: FieldSet, Signed: true, Bits: 8, Value: 101}}, []*int64{i64p(0)})
	fieldCall(t, "signed basics 3", nil, []FieldOp{
		{Kind: FieldSet, Signed: true, Bits: 8, Value: -100},
		{Kind: FieldSet, Signed: true, Bits: 8, Value: 101},
		{Kind: FieldGet, Signed: true, Bits: 8},
	}, []*int64{i64p(0), i64p(-100), i64p(101)})

	// "BITFIELD signed i64 SET handles positive values".
	fieldCall(t, "i64 positive", nil, []FieldOp{
		{Kind: FieldSet, Signed: true, Bits: 64, Value: 32},
		{Kind: FieldGet, Signed: true, Bits: 64},
	}, []*int64{i64p(0), i64p(32)})

	// "BITFIELD unsigned SET and GET basics".
	fieldCall(t, "unsigned basics", nil, []FieldOp{
		{Kind: FieldSet, Signed: false, Bits: 8, Value: 255},
		{Kind: FieldSet, Signed: false, Bits: 8, Value: 100},
		{Kind: FieldGet, Signed: false, Bits: 8},
	}, []*int64{i64p(0), i64p(255), i64p(100)})

	// "BITFIELD signed SET and GET together": SET i8 255 wraps to -1.
	fieldCall(t, "signed together", nil, []FieldOp{
		{Kind: FieldSet, Signed: true, Bits: 8, Value: 255},
		{Kind: FieldSet, Signed: true, Bits: 8, Value: 100},
		{Kind: FieldGet, Signed: true, Bits: 8},
	}, []*int64{i64p(0), i64p(-1), i64p(100)})

	// "BITFIELD unsigned with SET, GET and INCRBY arguments".
	fieldCall(t, "unsigned incrby", nil, []FieldOp{
		{Kind: FieldSet, Bits: 8, Value: 255},
		{Kind: FieldIncrBy, Bits: 8, Value: 100},
		{Kind: FieldGet, Bits: 8},
	}, []*int64{i64p(0), i64p(99), i64p(99)})

	// "BITFIELD with only key as argument" — an empty op list.
	if _, results, failed, err := ExecuteFieldOps(nil, nil); err != nil || len(results) != 0 || len(failed) != 0 {
		t.Errorf("empty ops: results=%v failed=%v err=%v, want all empty", results, failed, err)
	}

	// "BITFIELD #<idx> form": u8 #0/#1/#2 is bit offsets 0/8/16 — the
	// caller resolves the multiplier; the result spells ABC.
	buf, _, _, _ := ExecuteFieldOps(nil, []FieldOp{
		{Kind: FieldSet, Bits: 8, Offset: 0, Value: 'A'},
		{Kind: FieldSet, Bits: 8, Offset: 8, Value: 'B'},
		{Kind: FieldSet, Bits: 8, Offset: 16, Value: 'C'},
	})
	if string(buf) != "ABC" {
		t.Errorf("# form built %q, want ABC", buf)
	}

	// "BITFIELD basic INCRBY form" and "chaining of multiple commands".
	buf, _, _, _ = ExecuteFieldOps(nil, []FieldOp{{Kind: FieldSet, Bits: 8, Value: 10}})
	fieldCall(t, "incrby chain", buf, []FieldOp{
		{Kind: FieldIncrBy, Bits: 8, Value: 100},
		{Kind: FieldIncrBy, Bits: 8, Value: 100},
	}, []*int64{i64p(110), i64p(210)})

	// "BITFIELD unsigned overflow wrap".
	buf, _, _, _ = ExecuteFieldOps(nil, []FieldOp{{Kind: FieldSet, Bits: 8, Value: 100}})
	fieldCall(t, "unsigned wrap", buf, []FieldOp{
		{Kind: FieldIncrBy, Bits: 8, Value: 257, Overflow: OverflowWrap},
		{Kind: FieldGet, Bits: 8},
		{Kind: FieldIncrBy, Bits: 8, Value: 255, Overflow: OverflowWrap},
		{Kind: FieldGet, Bits: 8},
	}, []*int64{i64p(101), i64p(101), i64p(100), i64p(100)})

	// "BITFIELD unsigned overflow sat".
	fieldCall(t, "unsigned sat", buf, []FieldOp{
		{Kind: FieldIncrBy, Bits: 8, Value: 257, Overflow: OverflowSat},
		{Kind: FieldGet, Bits: 8},
		{Kind: FieldIncrBy, Bits: 8, Value: -255, Overflow: OverflowSat},
		{Kind: FieldGet, Bits: 8},
	}, []*int64{i64p(255), i64p(255), i64p(0), i64p(0)})

	// "BITFIELD signed overflow wrap".
	buf, _, _, _ = ExecuteFieldOps(nil, []FieldOp{{Kind: FieldSet, Signed: true, Bits: 8, Value: 100}})
	fieldCall(t, "signed wrap", buf, []FieldOp{
		{Kind: FieldIncrBy, Signed: true, Bits: 8, Value: 257, Overflow: OverflowWrap},
		{Kind: FieldGet, Signed: true, Bits: 8},
		{Kind: FieldIncrBy, Signed: true, Bits: 8, Value: 255, Overflow: OverflowWrap},
		{Kind: FieldGet, Signed: true, Bits: 8},
	}, []*int64{i64p(101), i64p(101), i64p(100), i64p(100)})

	// "BITFIELD signed overflow sat" — the seed value is stored by an
	// unsigned op, exactly as the TCL does.
	buf, _, _, _ = ExecuteFieldOps(nil, []FieldOp{{Kind: FieldSet, Bits: 8, Value: 100}})
	fieldCall(t, "signed sat", buf, []FieldOp{
		{Kind: FieldIncrBy, Signed: true, Bits: 8, Value: 257, Overflow: OverflowSat},
		{Kind: FieldGet, Signed: true, Bits: 8},
		{Kind: FieldIncrBy, Signed: true, Bits: 8, Value: -255, Overflow: OverflowSat},
		{Kind: FieldGet, Signed: true, Bits: 8},
	}, []*int64{i64p(127), i64p(127), i64p(-128), i64p(-128)})

	// "BITFIELD regression for #3221": reading a u1 out of "1".
	buf = []byte("1")
	fieldCall(t, "#3221", buf, []FieldOp{{Kind: FieldGet, Bits: 1}}, []*int64{i64p(0)})

	// "BITFIELD regression for #3564": the same op sequence, ten times
	// on a fresh key each, reports {0 0 60}.
	for range 10 {
		fieldCall(t, "#3564", nil, []FieldOp{
			{Kind: FieldSet, Signed: true, Bits: 8, Offset: 0, Value: 10},
			{Kind: FieldSet, Signed: true, Bits: 8, Offset: 64, Value: 10},
			{Kind: FieldIncrBy, Signed: true, Bits: 8, Offset: 10, Value: 99900},
		}, []*int64{i64p(0), i64p(0), i64p(60)})
	}

	// "BITFIELD #<idx> form rejects offsets that overflow when scaled":
	// the caller's scaled offset lands past the limit and is rejected
	// as ErrBitOffset (2^57 * 64 would itself overflow int64 — the
	// caller's check, mirroring the C's LLONG_MAX/bits guard).
	_, _, _, err := ExecuteFieldOps(nil, []FieldOp{{Kind: FieldGet, Signed: true, Bits: 64, Offset: MaxBitOffset + 1}})
	if err != ErrBitOffset {
		t.Errorf("scaled-offset overflow: err = %v, want ErrBitOffset", err)
	}
}

func TestBitOpVectors(t *testing.T) {
	// "BITOP NOT (empty string)" and "(known string)".
	if got := BitOpNot([]byte("")); len(got) != 0 {
		t.Errorf("NOT(empty) = %q, want empty", got)
	}
	if got := BitOpNot([]byte("\xaa\x00\xff\x55")); !bytes.Equal(got, []byte("\x55\xff\x00\xaa")) {
		t.Errorf("NOT(known) = % x, want 55 ff 00 aa", got)
	}

	// "BITOP AND|OR|XOR don't change the string with single input key".
	solo := []byte("\x01\x02\xff")
	for _, got := range [][]byte{BitOpAnd(solo), BitOpOr(solo), BitOpXor(solo)} {
		if !bytes.Equal(got, solo) {
			t.Errorf("single-source op = % x, want % x", got, solo)
		}
	}

	// "BITOP missing key is considered a stream of zero".
	a := []byte("\x01\x02\xff")
	if got := BitOpAnd(nil, a); !bytes.Equal(got, []byte("\x00\x00\x00")) {
		t.Errorf("AND(missing, a) = % x, want 00 00 00", got)
	}
	if got := BitOpOr(nil, a, nil); !bytes.Equal(got, a) {
		t.Errorf("OR(missing, a, missing) = % x, want a", got)
	}
	if got := BitOpXor(nil, a); !bytes.Equal(got, a) {
		t.Errorf("XOR(missing, a) = % x, want a", got)
	}

	// "BITOP shorter keys are zero-padded to the key with max length".
	long := []byte("\x01\x02\xff\xff")
	short := []byte("\x01\x02\xff")
	if got := BitOpAnd(long, short); !bytes.Equal(got, []byte("\x01\x02\xff\x00")) {
		t.Errorf("AND(long, short) = % x, want 01 02 ff 00", got)
	}
	if got := BitOpOr(long, short); !bytes.Equal(got, []byte("\x01\x02\xff\xff")) {
		t.Errorf("OR(long, short) = % x, want 01 02 ff ff", got)
	}
	if got := BitOpXor(long, short); !bytes.Equal(got, []byte("\x00\x00\x00\xff")) {
		t.Errorf("XOR(long, short) = % x, want 00 00 00 ff", got)
	}

	// "BITOP with empty string after non empty string (issue #529)":
	// the empty source pads to nothing — the result is 32 zero bytes.
	big := bytes.Repeat([]byte{0x00}, 32)
	if got := BitOpOr(big, nil); len(got) != 32 || got[0] != 0 {
		t.Errorf("OR(32 zeros, empty) = %d bytes % x, want 32 zeros", len(got), got)
	}

	// All-empty inputs yield an empty result (Redis deletes the target).
	if got := BitOpAnd(nil, nil); len(got) != 0 {
		t.Errorf("AND(empty, empty) = % x, want empty", got)
	}
	if got := BitOpOr(); len(got) != 0 {
		t.Errorf("OR() = % x, want empty", got)
	}
}
