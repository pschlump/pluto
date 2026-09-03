/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Model tests: every operation is cross-checked against a structurally
// different reference — the ranges against a bit-per-bit oracle written
// in the bit domain (no masks, no word skipping), the BITOP family
// against a per-bit fold (the port of bitops.tcl's simulate_bit_op),
// and the BITFIELD overflow policies against exact big.Int arithmetic
// (the port of bitfield.tcl's two fuzz loops).  All randomness is
// seeded; every test is deterministic.

package bitmap

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Reference models — independent of the implementation under test.
// ---------------------------------------------------------------------------

// modelBitRange applies the Redis range clamps in the caller's terms and
// returns the inclusive BIT range they select (ok=false when empty).
func modelBitRange(n int, start, end int64, unit Unit, endGiven bool) (lo, hi int64, ok bool) {
	totlen := int64(n)
	if unit == BitUnit {
		totlen *= 8
	}
	if !endGiven {
		end = totlen - 1
	}
	if start < 0 {
		start += totlen
	}
	if end < 0 {
		end += totlen
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end >= totlen {
		end = totlen - 1
	}
	if unit == BitUnit {
		lo, hi = start, end
	} else {
		lo, hi = start*8, end*8+7
	}
	return lo, hi, lo <= hi
}

func modelBitAt(buf []byte, p int64) int {
	return int(buf[p>>3]>>(7-(p&7))) & 1
}

// modelBitPos is the bit-domain BITPOS: find the first matching bit in
// the clamped range, then apply the missing/zero-pad rules.
func modelBitPos(buf []byte, bit int, start, end int64, unit Unit, endGiven bool) (int64, bool) {
	lo, hi, ok := modelBitRange(len(buf), start, end, unit, endGiven)
	if !ok {
		return -1, false
	}
	for p := lo; p <= hi; p++ {
		if modelBitAt(buf, p) == bit {
			return p, true
		}
	}
	if bit == 1 || endGiven {
		return -1, false
	}
	// No explicit end: the right side reads as zero padding, so the
	// first 0 is the bit just past the buffer.
	return int64(len(buf)) * 8, true
}

// modelBitCount is the bit-domain BITCOUNT.  It carries bitcountCommand's
// early both-negative-inverted check — NOT redundant: on a buffer short
// enough that end clamps up to 0, (-1, -4) would otherwise select a
// nonempty range, and Redis answers 0 (bitposCommand has no such check).
func modelBitCount(buf []byte, start, end int64, unit Unit) int64 {
	if buf == nil {
		return 0
	}
	if start < 0 && end < 0 && start > end {
		return 0
	}
	lo, hi, ok := modelBitRange(len(buf), start, end, unit, true)
	if !ok {
		return 0
	}
	n := int64(0)
	for p := lo; p <= hi; p++ {
		n += int64(modelBitAt(buf, p))
	}
	return n
}

// modelBitOp folds the sources bit by bit (simulate_bit_op's shape).
// f receives the accumulated bit and the next source's bit.
func modelBitOp(f func(a, b int) int, srcs ...[]byte) []byte {
	maxlen := 0
	for _, s := range srcs {
		if len(s) > maxlen {
			maxlen = len(s)
		}
	}
	bitAt := func(s []byte, x int64) int {
		if x >= int64(len(s))*8 {
			return 0
		}
		return int(s[x>>3]>>(7-(x&7))) & 1
	}
	out := make([]byte, maxlen)
	for x := int64(0); x < int64(maxlen)*8; x++ {
		v := bitAt(srcs[0], x)
		for _, s := range srcs[1:] {
			v = f(v, bitAt(s, x))
		}
		if v == 1 {
			out[x>>3] |= 1 << (7 - (x & 7))
		}
	}
	return out
}

var (
	modelAnd = func(a, b int) int { return a & b }
	modelOr  = func(a, b int) int { return a | b }
	modelXor = func(a, b int) int { return a ^ b }
)

// randBuf generates a buffer of 0..maxLen bytes with long runs of 0x00
// and 0xFF between random bytes — the shapes that exercise the
// word-skipping scans.
func randBuf(r *rand.Rand, maxLen int) []byte {
	buf := make([]byte, r.IntN(maxLen+1))
	for i := 0; i < len(buf); {
		switch r.IntN(4) {
		case 0, 1: // a run of the skip-value
			b := byte(0)
			if r.IntN(2) == 1 {
				b = 0xFF
			}
			l := 1 + r.IntN(17)
			for ; l > 0 && i < len(buf); l-- {
				buf[i] = b
				i++
			}
		default:
			buf[i] = byte(r.IntN(256))
			i++
		}
	}
	return buf
}

// ---------------------------------------------------------------------------
// BitCount / BitPos against the bit-domain models.
// ---------------------------------------------------------------------------

func TestBitCountModel(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	for iter := 0; iter < 4000; iter++ {
		buf := randBuf(r, 300)
		unit := Unit(r.IntN(2))
		totlen := int64(len(buf))
		if unit == BitUnit {
			totlen *= 8
		}
		start := int64(r.IntN(int(totlen)+4)) - 2 // a little out of range on both ends
		end := int64(r.IntN(int(totlen)+4)) - 2 - int64(r.IntN(3))
		want := modelBitCount(buf, start, end, unit)
		if got := BitCount(buf, start, end, unit); got != want {
			t.Fatalf("iter %d: BitCount(% x, %d, %d, %v) = %d, model %d",
				iter, buf, start, end, unit, got, want)
		}
	}
	// The nil buffer (missing key) is always 0.
	for _, tc := range [][3]int64{{0, -1, 0}, {-6, -7, 0}, {5, 90, 1}} {
		if got := BitCount(nil, tc[0], tc[1], Unit(tc[2])); got != 0 {
			t.Errorf("BitCount(nil, %d, %d, %v) = %d, want 0", tc[0], tc[1], Unit(tc[2]), got)
		}
	}
}

func TestBitPosModel(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	for iter := 0; iter < 4000; iter++ {
		buf := randBuf(r, 300)
		bit := r.IntN(2)
		unit := Unit(r.IntN(2))
		endGiven := r.IntN(2) == 1
		totlen := int64(len(buf))
		if unit == BitUnit {
			totlen *= 8
		}
		start := int64(r.IntN(int(totlen)+4)) - 2
		end := int64(r.IntN(int(totlen)+4)) - 2 - int64(r.IntN(3))
		wantPos, wantFound := modelBitPos(buf, bit, start, end, unit, endGiven)
		pos, found := BitPos(buf, bit, start, end, unit, endGiven)
		if pos != wantPos || found != wantFound {
			t.Fatalf("iter %d: BitPos(% x, %d, %d, %d, %v, %v) = (%d, %v), model (%d, %v)",
				iter, buf, bit, start, end, unit, endGiven, pos, found, wantPos, wantFound)
		}
	}
}

// The port of bitops.tcl's "BITPOS/BITCOUNT fuzzy testing using SETBIT":
// a 10-byte buffer that is all ones except one zero (and the inverse),
// swept over range shapes around the special bit for both units.  The
// expected values are the TCL's closed forms.
func TestBitPosBitCountJointFuzz(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	const max = 80
	buf := make([]byte, max/8) // setbit str 79 0 — the no-op that creates it
	for bit := 0; bit < 2; bit++ {
		buf = BitOpNot(buf)
		for j := int64(0); j < max; j++ {
			buf, _, _ = SetBit(buf, uint64(j), bit)
			for _, pt := range []struct {
				curr, last int64
				unit       Unit
			}{
				{j >> 3, max / 8, ByteUnit},
				{j, max, BitUnit},
			} {
				curr, last, unit := pt.curr, pt.last, pt.unit
				type shape struct{ s1, e1, s2, e2 int64 }
				shapes := []shape{{curr, curr, curr, curr}}
				if curr > 0 {
					shapes = append(shapes, shape{0, curr, 0, curr})
				}
				if curr < last-1 {
					shapes = append(shapes, shape{curr + 1, last, curr + 1, last})
					if curr > 0 {
						shapes = append(shapes, shape{0, curr, curr + 1, last})
					}
				}
				for _, sh := range shapes {
					// randomRange is exclusive of its max in the
					// TCL harness — so end never lands past the
					// buffer and the closed forms below are exact.
					pick := func(lo, hi int64) int64 {
						if hi <= lo {
							return lo
						}
						return lo + r.Int64N(hi-lo)
					}
					start := pick(sh.s1, sh.e1)
					end := pick(sh.s2, sh.e2)
					if start > end {
						start, end = end, start
					}
					var startbit, endbit int64
					if unit == ByteUnit {
						startbit, endbit = start<<3, end<<3+7
					} else {
						startbit, endbit = start, end
					}
					inrange := j >= startbit && j <= endbit

					wantPos := int64(-1)
					if inrange {
						wantPos = j
					}
					pos, found := BitPos(buf, bit, start, end, unit, true)
					if pos != wantPos || found != (wantPos != -1) {
						t.Fatalf("bit=%d j=%d unit=%v range [%d,%d]: BitPos = (%d, %v), want %d",
							bit, j, unit, start, end, pos, found, wantPos)
					}

					delta := int64(-1)
					if bit == 1 {
						delta = 1
					}
					ind := int64(0)
					if inrange {
						ind = 1
					}
					wantCount := (endbit-startbit+1)*int64(1-bit) + ind*delta
					if got := BitCount(buf, start, end, unit); got != wantCount {
						t.Fatalf("bit=%d j=%d unit=%v range [%d,%d]: BitCount = %d, want %d",
							bit, j, unit, start, end, got, wantCount)
					}
				}
			}
			buf, _, _ = SetBit(buf, uint64(j), 1-bit) // restore
		}
	}
}

// ---------------------------------------------------------------------------
// GetBit / SetBit.
// ---------------------------------------------------------------------------

// The bit-order pins: bit 0 is the MSB of byte 0 (the C's worked example
// is a 5-bit field 23 at offset 7 landing 0x01 0x70).
func TestBitOrderPins(t *testing.T) {
	buf, old, err := SetBit(nil, 7, 1)
	if err != nil || old != 0 || !bytes.Equal(buf, []byte{0x01}) {
		t.Fatalf("SetBit(nil, 7, 1) = (% x, %d, %v), want (01, 0, nil)", buf, old, err)
	}
	buf, old, _ = SetBit(buf, 0, 1)
	if !bytes.Equal(buf, []byte{0x81}) || old != 0 {
		t.Fatalf("after SetBit(0, 1): % x old=%d, want 81 old=0", buf, old)
	}
	if got := GetBit(buf, 0); got != 1 {
		t.Errorf("GetBit(buf, 0) = %d, want 1", got)
	}
	if _, old2, _ := SetBit(buf, 7, 0); old2 != 1 {
		t.Errorf("clearing bit 7 reported old=%d, want 1", old2)
	}

	out, _, _, err := ExecuteFieldOps(nil, []FieldOp{{Kind: FieldSet, Bits: 5, Offset: 7, Value: 23}})
	if err != nil || !bytes.Equal(out, []byte{0x01, 0x70}) {
		t.Errorf("5-bit field 23 at offset 7 = % x (%v), want 01 70", out, err)
	}
	_, results, _, err := ExecuteFieldOps(out, []FieldOp{{Kind: FieldGet, Bits: 5, Offset: 7}})
	if err != nil || results[0] != 23 {
		t.Errorf("reading the field back = %d (%v), want 23", results, err)
	}
}

func TestSetBitGetBitModel(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	buf := []byte(nil)
	model := map[uint64]int{}
	for iter := 0; iter < 2000; iter++ {
		off := uint64(r.IntN(600))
		bit := r.IntN(2)
		newBuf, old, err := SetBit(buf, off, bit)
		if err != nil {
			t.Fatalf("iter %d: SetBit(%d, %d): %v", iter, off, bit, err)
		}
		if old != model[off] {
			t.Fatalf("iter %d: SetBit old = %d, want %d", iter, old, model[off])
		}
		buf = newBuf
		model[off] = bit
		if got := GetBit(buf, off); got != bit {
			t.Fatalf("iter %d: GetBit(%d) = %d, want %d", iter, off, got, bit)
		}
	}
	// Every modeled bit reads back; every gap reads zero.
	for off := uint64(0); off < 600; off++ {
		if got := GetBit(buf, off); got != model[off] {
			t.Errorf("final GetBit(%d) = %d, want %d", off, got, model[off])
		}
	}
	if got := GetBit(buf, 100_000); got != 0 {
		t.Errorf("GetBit past the end = %d, want 0", got)
	}
	if got := GetBit(nil, 0); got != 0 {
		t.Errorf("GetBit(nil) = %d, want 0", got)
	}
}

func TestSetBitContracts(t *testing.T) {
	// In place when the byte fits, grown (input untouched) when not.
	buf := []byte{0xFF}
	out, old, err := SetBit(buf, 3, 1)
	if err != nil || old != 1 || &out[0] != &buf[0] || len(out) != 1 {
		t.Errorf("in-place set: out=% x old=%d err=%v", out, old, err)
	}
	out, old, err = SetBit(buf, 8, 1)
	if err != nil || old != 0 || len(out) != 2 || out[0] != 0xFF || out[1] != 0x80 {
		t.Errorf("growing set: out=% x old=%d err=%v", out, old, err)
	}
	if len(buf) != 1 || buf[0] != 0xFF {
		t.Errorf("growing set mutated the input: % x", buf)
	}
	// Zero fill between the old end and the addressed byte.
	out, _, _ = SetBit(out, 40, 1)
	if len(out) != 6 || out[2] != 0 || out[4] != 0 || out[5] == 0 {
		t.Errorf("zero fill: % x", out)
	}

	// Errors leave everything alone.
	for _, tc := range []struct {
		offset uint64
		bit    int
		err    error
	}{
		{0, 2, ErrBitValue},
		{0, -1, ErrBitValue},
		{MaxBitOffset + 1, 1, ErrBitOffset},
		{uint64(1) << 62, 1, ErrBitOffset},
	} {
		before := []byte{0x0A}
		out, old, err := SetBit(before, tc.offset, tc.bit)
		if err != tc.err {
			t.Errorf("SetBit(%d, %d) err = %v, want %v", tc.offset, tc.bit, err, tc.err)
		}
		if old != 0 || !bytes.Equal(out, before) {
			t.Errorf("SetBit(%d, %d) mutated on error: % x", tc.offset, tc.bit, out)
		}
	}

	// The 512 MiB boundary itself is legal (short mode skips the
	// half-gigabyte allocation).
	if !testing.Short() {
		out, old, err := SetBit(nil, MaxBitOffset, 1)
		if err != nil || old != 0 || len(out) != MaxBytes {
			t.Errorf("SetBit at MaxBitOffset: len=%d old=%d err=%v", len(out), old, err)
		}
		if got := GetBit(out, MaxBitOffset); got != 1 {
			t.Errorf("GetBit at MaxBitOffset = %d, want 1", got)
		}
	}
	// One past it is not, on any buffer.
	if _, _, err := SetBit(make([]byte, 8), MaxBitOffset+1, 0); err != ErrBitOffset {
		t.Errorf("SetBit at MaxBitOffset+1 err = %v, want ErrBitOffset", err)
	}
}

// ---------------------------------------------------------------------------
// BitOp against the per-bit fold.
// ---------------------------------------------------------------------------

func TestBitOpModel(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	check := func(got, want []byte, name string, srcs [][]byte) {
		t.Helper()
		if !bytes.Equal(got, want) {
			t.Fatalf("%s(% x) = % x, model % x", name, srcs, got, want)
		}
	}
	for iter := 0; iter < 800; iter++ {
		k := 1 + r.IntN(6)
		srcs := make([][]byte, k)
		for i := range srcs {
			srcs[i] = randBuf(r, 150)
		}
		check(BitOpAnd(srcs...), modelBitOp(modelAnd, srcs...), "AND", srcs)
		check(BitOpOr(srcs...), modelBitOp(modelOr, srcs...), "OR", srcs)
		check(BitOpXor(srcs...), modelBitOp(modelXor, srcs...), "XOR", srcs)
		for _, s := range srcs {
			inv := make([]byte, len(s))
			for i, b := range s {
				inv[i] = ^b
			}
			check(BitOpNot(s), inv, "NOT", [][]byte{s})
		}
	}

	// Word-edge lengths: sources of exactly n and 2n/3 bytes, straddling
	// the 8-byte steps and the zero-padding tail.
	r2 := rand.New(rand.NewPCG(7, 42))
	exact := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(r2.IntN(256))
		}
		return b
	}
	for _, n := range []int{1, 7, 8, 9, 15, 16, 17, 23, 24, 25, 31, 32, 33} {
		srcs := [][]byte{exact(n), exact(n * 2 / 3)}
		check(BitOpAnd(srcs...), modelBitOp(modelAnd, srcs...), "AND", srcs)
		check(BitOpOr(srcs...), modelBitOp(modelOr, srcs...), "OR", srcs)
		check(BitOpXor(srcs...), modelBitOp(modelXor, srcs...), "XOR", srcs)
	}

	// Laws: XOR associativity, AND-over-OR distributivity, NOT
	// self-inverse, and the inputs are never mutated.
	a, b, c := randBuf(r, 200), randBuf(r, 200), randBuf(r, 200)
	a2, b2, c2 := append([]byte(nil), a...), append([]byte(nil), b...), append([]byte(nil), c...)
	if got, want := BitOpXor(BitOpXor(a, b), c), BitOpXor(a, BitOpXor(b, c)); !bytes.Equal(got, want) {
		t.Errorf("XOR associativity broken")
	}
	if got, want := BitOpAnd(a, BitOpOr(b, c)), BitOpOr(BitOpAnd(a, b), BitOpAnd(a, c)); !bytes.Equal(got, want) {
		t.Errorf("AND/OR distributivity broken")
	}
	if got := BitOpNot(BitOpNot(a)); !bytes.Equal(got, a) {
		t.Errorf("NOT self-inverse broken")
	}
	for name, pair := range map[string][2][]byte{"a": {a, a2}, "b": {b, b2}, "c": {c, c2}} {
		if !bytes.Equal(pair[0], pair[1]) {
			t.Errorf("source %q mutated by a BitOp", name)
		}
	}
}

// ---------------------------------------------------------------------------
// BITFIELD overflow policies.
// ---------------------------------------------------------------------------

func TestBitfieldOverflowBoundaries(t *testing.T) {
	const (
		i64max = 1<<63 - 1
		i64min = -1 << 63
	)
	// INCRBY at the field limits: the seeded field, the increment, and
	// what each policy answers and stores.  The "exactly" rows pin the
	// checks' strict inequalities — a result landing on a limit is not
	// an overflow.
	for _, tc := range []struct {
		name      string
		old, incr int64
		bits      int
		signed    bool
		wrap, sat int64
		overflows bool
	}{
		{"i8 max+1", 127, 1, 8, true, -128, 127, true},
		{"i8 min-1", -128, -1, 8, true, 127, -128, true},
		{"i8 to max exactly", 126, 1, 8, true, 127, 127, false},
		{"i8 to min exactly", -127, -1, 8, true, -128, -128, false},
		{"i8 +0 at max", 127, 0, 8, true, 127, 127, false},
		{"u8 max+1", 255, 1, 8, false, 0, 255, true},
		{"u8 0-1", 0, -1, 8, false, 255, 0, true},
		{"u8 to max exactly", 254, 1, 8, false, 255, 255, false},
		{"u1 1+1", 1, 1, 1, false, 0, 1, true},
		{"u63 max+1", i64max, 1, 63, false, 0, i64max, true},
		{"u63 to max exactly", i64max - 1, 1, 63, false, i64max, i64max, false},
		{"i64 max+1", i64max, 1, 64, true, i64min, i64max, true},
		{"i64 min-1", i64min, -1, 64, true, i64max, i64min, true},
		{"i64 to max exactly", i64max - 1, 1, 64, true, i64max, i64max, false},
		{"i64 -1+min", -1, i64min, 64, true, i64max, i64min, true},
		{"i64 0+min exactly", 0, i64min, 64, true, i64min, i64min, false},
	} {
		seedOp := FieldOp{Kind: FieldSet, Signed: tc.signed, Bits: tc.bits, Value: tc.old}
		for _, policy := range []OverflowPolicy{OverflowWrap, OverflowSat, OverflowFail} {
			// A fresh seed per policy: the earlier policy's op wrote
			// into the shared buffer.
			seeded, _, _, err := ExecuteFieldOps(nil, []FieldOp{seedOp})
			if err != nil {
				t.Fatalf("%s: seeding: %v", tc.name, err)
			}
			var wantFailed bool
			var wantResult, wantStored int64
			switch policy {
			case OverflowWrap:
				wantFailed, wantResult, wantStored = false, tc.wrap, tc.wrap
			case OverflowSat:
				wantFailed, wantResult, wantStored = false, tc.sat, tc.sat
			case OverflowFail:
				// FAIL refuses only when the result overflows;
				// otherwise it behaves like WRAP (which lands on the
				// exact sum for the non-overflow rows).
				if tc.overflows {
					wantFailed, wantResult, wantStored = true, 0, tc.old
				} else {
					wantFailed, wantResult, wantStored = false, tc.wrap, tc.wrap
				}
			}
			ops := []FieldOp{
				{Kind: FieldIncrBy, Signed: tc.signed, Bits: tc.bits, Value: tc.incr, Overflow: policy},
				{Kind: FieldGet, Signed: tc.signed, Bits: tc.bits},
			}
			_, results, failed, err := ExecuteFieldOps(seeded, ops)
			if err != nil {
				t.Fatalf("%s/%v: %v", tc.name, policy, err)
			}
			if failed[0] != wantFailed {
				t.Errorf("%s/%v: failed=%v, want %v", tc.name, policy, failed[0], wantFailed)
			}
			if results[0] != wantResult {
				t.Errorf("%s/%v: result = %d, want %d", tc.name, policy, results[0], wantResult)
			}
			if results[1] != wantStored {
				t.Errorf("%s/%v: stored %d, want %d", tc.name, policy, results[1], wantStored)
			}
		}
	}

	// SET wrapping/saturating out-of-range values; FAIL refuses and
	// leaves the field untouched (0 on a fresh buffer).
	for _, tc := range []struct {
		name                  string
		value                 int64
		bits                  int
		signed                bool
		wrapStored, satStored int64
		fits                  bool
	}{
		{"SET i8 127", 127, 8, true, 127, 127, true},
		{"SET i8 128", 128, 8, true, -128, 127, false},
		{"SET i8 -129", -129, 8, true, 127, -128, false},
		{"SET u8 255", 255, 8, false, 255, 255, true},
		{"SET u8 256", 256, 8, false, 0, 255, false},
		{"SET u8 -1", -1, 8, false, 255, 255, false},
		{"SET i64 min", i64min, 64, true, i64min, i64min, true},
		{"SET i64 max", i64max, 64, true, i64max, i64max, true},
		{"SET u63 -1", -1, 63, false, i64max, i64max, false},
		{"SET u63 max", i64max, 63, false, i64max, i64max, true},
	} {
		for _, policy := range []OverflowPolicy{OverflowWrap, OverflowSat, OverflowFail} {
			ops := []FieldOp{
				{Kind: FieldSet, Signed: tc.signed, Bits: tc.bits, Value: tc.value, Overflow: policy},
				{Kind: FieldGet, Signed: tc.signed, Bits: tc.bits},
			}
			_, results, failed, err := ExecuteFieldOps(nil, ops)
			if err != nil {
				t.Fatalf("%s/%v: %v", tc.name, policy, err)
			}
			wantFailed := policy == OverflowFail && !tc.fits
			var wantStored int64
			switch {
			case wantFailed:
				wantStored = 0 // the fresh buffer's field, untouched
			case policy == OverflowSat:
				wantStored = tc.satStored
			default: // WRAP, or a fitting value under any policy
				wantStored = tc.wrapStored
			}
			if failed[0] != wantFailed {
				t.Errorf("%s/%v: failed=%v, want %v", tc.name, policy, failed[0], wantFailed)
			}
			if results[1] != wantStored {
				t.Errorf("%s/%v: stored %d, want %d", tc.name, policy, results[1], wantStored)
			}
		}
	}
}

// The port of bitfield.tcl's "BITFIELD overflow detection fuzzing": the
// FAIL policy must refuse exactly when exact arithmetic leaves the field
// range (the SET first, then the INCRBY from whatever the SET stored).
func TestBitfieldOverflowFailFuzz(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	i64minB, i64maxB := big.NewInt(-1<<63), big.NewInt(1<<63-1)
	for i := 0; i < 1000; i++ {
		bits := 1 + r.IntN(64)
		signed := r.IntN(2) == 1
		if bits == 64 {
			signed = true // u64 is not supported by BITFIELD
		}
		var min, max *big.Int
		if signed {
			min = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
			max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)), big.NewInt(1))
		} else {
			min = big.NewInt(0)
			max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
		}
		// pick = (min*2) + randomInt(2*range), clamped to int64 — the
		// values live up to a full octave beyond each limit.
		span := new(big.Int).Lsh(new(big.Int).Sub(max, min), 1)
		span.Add(span, big.NewInt(2))
		pick := func() int64 {
			lo := new(big.Int).SetUint64(r.Uint64())
			hi := new(big.Int).SetUint64(r.Uint64())
			v := new(big.Int).Or(new(big.Int).Lsh(hi, 64), lo)
			v.Mod(v, span)
			v.Add(v, new(big.Int).Lsh(min, 1))
			if v.Cmp(i64maxB) > 0 {
				return i64maxB.Int64()
			}
			if v.Cmp(i64minB) < 0 {
				return i64minB.Int64()
			}
			return v.Int64()
		}
		value, incr := pick(), pick()
		inRange := func(v *big.Int) bool { return v.Cmp(min) >= 0 && v.Cmp(max) <= 0 }

		setFails := !inRange(big.NewInt(value))
		fieldAfterSet := big.NewInt(0)
		if !setFails {
			fieldAfterSet = big.NewInt(value)
		}
		sum := new(big.Int).Add(fieldAfterSet, big.NewInt(incr))
		incrFails := !inRange(sum)

		_, results, failed, err := ExecuteFieldOps(nil, []FieldOp{
			{Kind: FieldSet, Signed: signed, Bits: bits, Value: value, Overflow: OverflowFail},
			{Kind: FieldIncrBy, Signed: signed, Bits: bits, Value: incr, Overflow: OverflowFail},
		})
		if err != nil {
			t.Fatalf("i=%d: %v", i, err)
		}
		if failed[0] != setFails {
			t.Fatalf("i=%d %s%d SET %d: failed=%v, want %v", i, sign(signed), bits, value, failed[0], setFails)
		}
		if failed[1] != incrFails {
			t.Fatalf("i=%d %s%d field=%v + %d: failed=%v, want %v", i, sign(signed), bits, fieldAfterSet, incr, failed[1], incrFails)
		}
		if !failed[1] && results[1] != sum.Int64() {
			t.Fatalf("i=%d: INCRBY result %d, want %d", i, results[1], sum.Int64())
		}
	}
}

// The port of bitfield.tcl's "BITFIELD overflow wrap fuzzing": after a
// wrapping SET and a wrapping INCRBY the field equals the exact sum
// reduced into the field's range.
func TestBitfieldOverflowWrapFuzz(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	i64maxB := big.NewInt(1<<63 - 1)
	i64minB := new(big.Int).Neg(i64maxB)
	for i := 0; i < 1000; i++ {
		bits := 1 + r.IntN(64)
		signed := r.IntN(2) == 1
		if bits == 64 {
			signed = true
		}
		var min, max *big.Int
		if signed {
			min = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
			max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)), big.NewInt(1))
		} else {
			min = big.NewInt(0)
			max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
		}
		span := new(big.Int).Lsh(new(big.Int).Sub(max, min), 1)
		span.Add(span, big.NewInt(2))
		pick := func() int64 {
			lo := new(big.Int).SetUint64(r.Uint64())
			hi := new(big.Int).SetUint64(r.Uint64())
			v := new(big.Int).Or(new(big.Int).Lsh(hi, 64), lo)
			v.Mod(v, span)
			v.Add(v, new(big.Int).Lsh(min, 1))
			if v.Cmp(i64maxB) > 0 {
				return i64maxB.Int64()
			}
			if v.Cmp(i64minB) < 0 {
				return i64minB.Int64()
			}
			return v.Int64()
		}
		value, incr := pick(), pick()

		// expected = ((value - min) + incr) mod 2^bits + min
		rangeB := new(big.Int).Lsh(big.NewInt(1), uint(bits))
		expected := new(big.Int).Sub(big.NewInt(value), min)
		expected.Add(expected, big.NewInt(incr))
		expected.Mod(expected, rangeB) // big.Int Mod is Euclidean: always in [0, range)
		expected.Add(expected, min)

		_, results, failed, err := ExecuteFieldOps(nil, []FieldOp{
			{Kind: FieldSet, Signed: signed, Bits: bits, Value: value, Overflow: OverflowWrap},
			{Kind: FieldIncrBy, Signed: signed, Bits: bits, Value: incr, Overflow: OverflowWrap},
			{Kind: FieldGet, Signed: signed, Bits: bits},
		})
		if err != nil {
			t.Fatalf("i=%d: %v", i, err)
		}
		for j := range failed {
			if failed[j] {
				t.Fatalf("i=%d: WRAP op %d failed", i, j)
			}
		}
		if results[2] != expected.Int64() {
			t.Fatalf("i=%d %s%d %d+%d: WRAP field = %d, want %d",
				i, sign(signed), bits, value, incr, results[2], expected.Int64())
		}
	}
}

func sign(signed bool) string {
	if signed {
		return "i"
	}
	return "u"
}

// ---------------------------------------------------------------------------
// ExecuteFieldOps contracts.
// ---------------------------------------------------------------------------

func TestExecuteFieldOpsContracts(t *testing.T) {
	// In place when no growth is needed.
	buf := make([]byte, 4)
	out, _, _, err := ExecuteFieldOps(buf, []FieldOp{{Kind: FieldSet, Bits: 8, Offset: 24, Value: 5}})
	if err != nil || len(out) != 4 || &out[0] != &buf[0] {
		t.Errorf("no-growth write: len=%d err=%v (want in place)", len(out), err)
	}
	snapshot := append([]byte(nil), buf...)

	// Growth to the exact byte the furthest write touches: bit 107 is
	// the last bit of byte 13.
	out, _, _, err = ExecuteFieldOps(buf, []FieldOp{{Kind: FieldSet, Bits: 8, Offset: 100, Value: 5}})
	if err != nil || len(out) != 14 {
		t.Errorf("growth write: len=%d err=%v, want 14", len(out), err)
	}
	if !bytes.Equal(buf, snapshot) {
		t.Errorf("growth write mutated the input: % x", buf)
	}

	// Growth happens even when every write then fails — the buffer is
	// the C's strGrowSize side effect.
	out, _, failed, err := ExecuteFieldOps(nil, []FieldOp{
		{Kind: FieldSet, Bits: 8, Offset: 800, Value: 300, Overflow: OverflowFail},
	})
	if err != nil || !failed[0] || len(out) != 101 {
		t.Errorf("failed growth: len=%d failed=%v err=%v, want len 101 failed", len(out), failed[0], err)
	}

	// A failed op writes nothing: the field a later GET sees is intact.
	buf2 := []byte{0xFF}
	_, results, failed, err := ExecuteFieldOps(buf2, []FieldOp{
		{Kind: FieldIncrBy, Bits: 8, Value: 256, Overflow: OverflowFail},
		{Kind: FieldGet, Bits: 8},
	})
	if err != nil || !failed[0] {
		t.Errorf("incrby 256 on 255 must fail: failed=%v err=%v", failed[0], err)
	}
	if results[0] != 0 || results[1] != 255 {
		t.Errorf("failed incrby: results=%v, want [0 255] (nothing written)", results)
	}

	// GET-only slices never grow; a nil buffer reads zeros.
	out, results, failed, err = ExecuteFieldOps(nil, []FieldOp{
		{Kind: FieldGet, Bits: 8, Offset: 900},
		{Kind: FieldGet, Signed: true, Bits: 16, Offset: 10_000},
	})
	if err != nil || len(out) != 0 || results[0] != 0 || results[1] != 0 || failed[0] || failed[1] {
		t.Errorf("GET-only on nil: out=%d results=%v failed=%v err=%v", len(out), results, failed, err)
	}

	// Validation: nothing runs when any op is malformed, and the
	// buffer comes back unchanged with nil results.
	good := []FieldOp{{Kind: FieldSet, Bits: 8, Value: 1}}
	for _, bad := range []FieldOp{
		{Kind: FieldKind(99), Bits: 8},
		{Kind: FieldSet, Bits: 8, Overflow: OverflowPolicy(99)},
		{Kind: FieldSet, Bits: 0},                // i0
		{Kind: FieldSet, Bits: 65, Signed: true}, // i65
		{Kind: FieldSet, Bits: 64},               // u64
		{Kind: FieldSet, Bits: -3},
		{Kind: FieldGet, Bits: 8, Offset: -1},
		{Kind: FieldSet, Bits: 8, Offset: MaxBitOffset + 1},
	} {
		buf3 := []byte{1, 2, 3}
		ops := append(append([]FieldOp{}, good...), bad)
		out, results, failed, err := ExecuteFieldOps(buf3, ops)
		if err == nil {
			t.Errorf("bad op %+v accepted", bad)
		}
		if results != nil || failed != nil {
			t.Errorf("bad op %+v returned results/failed", bad)
		}
		if !bytes.Equal(out, buf3) || &out[0] != &buf3[0] {
			t.Errorf("bad op %+v changed the buffer: % x", bad, out)
		}
	}

	// The sentinel classes.
	if _, _, _, err := ExecuteFieldOps(nil, []FieldOp{{Kind: FieldGet, Bits: 64}}); err != ErrBitfieldType {
		t.Errorf("u64 err = %v, want ErrBitfieldType", err)
	}
	if _, _, _, err := ExecuteFieldOps(nil, []FieldOp{{Kind: FieldGet, Bits: 8, Offset: -8}}); err != ErrBitOffset {
		t.Errorf("negative offset err = %v, want ErrBitOffset", err)
	}
	if _, _, _, err := ExecuteFieldOps(nil, []FieldOp{{Kind: FieldKind(-1), Bits: 8}}); err != ErrInvalidFieldOp {
		t.Errorf("bad kind err = %v, want ErrInvalidFieldOp", err)
	}
}

// ---------------------------------------------------------------------------
// Panic contract and nil-vs-empty distinction.
// ---------------------------------------------------------------------------

func TestPanics(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
		want string
	}{
		{"BitPos bit 2", func() { BitPos([]byte{0}, 2, 0, 0, ByteUnit, true) }, "bitmap.BitPos"},
		{"BitPos bit -1", func() { BitPos([]byte{0}, -1, 0, 0, ByteUnit, true) }, "bitmap.BitPos"},
		{"BitCount unit 5", func() { BitCount([]byte{0}, 0, 0, Unit(5)) }, "bitmap.BitCount"},
		{"BitPos unit -1", func() { BitPos([]byte{0}, 0, 0, 0, Unit(-1), true) }, "bitmap.BitPos"},
	}
	for _, tc := range cases {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("%s: no panic", tc.name)
					return
				}
				if msg := fmt.Sprint(r); !strings.Contains(msg, tc.want) {
					t.Errorf("%s: panic %q does not name %q", tc.name, msg, tc.want)
				}
			}()
			tc.fn()
		}()
	}
}

func TestNilVsEmpty(t *testing.T) {
	// Missing key: an infinite zero array.
	if pos, found := BitPos(nil, 0, 0, -1, ByteUnit, true); !found || pos != 0 {
		t.Errorf("BitPos(nil, 0) = (%d, %v), want (0, true)", pos, found)
	}
	// Empty string: a zero-length range.
	if pos, found := BitPos([]byte{}, 0, 0, -1, ByteUnit, true); found || pos != -1 {
		t.Errorf("BitPos(empty, 0) = (%d, %v), want (-1, false)", pos, found)
	}
	if pos, found := BitPos([]byte{}, 0, 0, 0, ByteUnit, false); found || pos != -1 {
		t.Errorf("BitPos(empty, 0, no end) = (%d, %v), want (-1, false)", pos, found)
	}
	if pos, found := BitPos([]byte{}, 1, 0, -1, BitUnit, true); found || pos != -1 {
		t.Errorf("BitPos(empty, 1) = (%d, %v), want (-1, false)", pos, found)
	}
	// BitCount cannot tell them apart — both count 0.
	if BitCount(nil, 0, -1, ByteUnit) != 0 || BitCount([]byte{}, 0, -1, ByteUnit) != 0 {
		t.Errorf("BitCount nil vs empty mismatch")
	}
}
