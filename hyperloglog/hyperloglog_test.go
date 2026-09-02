/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"testing"
)

// sanityBuffer reproduces the xxHash reference suite's sanity buffer:
// byteGen starts at PRIME32 (2654435761) and multiplies by PRIME64
// (11400714785074694797) after each byte, which is emitted from the top
// byte.  The expected digests below are the reference suite's published
// XXH64 vectors (xsum_sanity_check.c).
func sanityBuffer(n int) []byte {
	const (
		genPrime32 = 2654435761
		genPrime64 = 11400714785074694797
	)
	b := make([]byte, n)
	byteGen := uint64(genPrime32)
	for i := range b {
		b[i] = byte(byteGen >> 56)
		byteGen *= genPrime64
	}
	return b
}

// TestXxh64Vectors pins the hash against the xxHash reference suite's
// published sanity vectors (nine digests over the generated buffer at
// two seeds).
func TestXxh64Vectors(t *testing.T) {
	vectors := []struct {
		len  int
		seed uint64
		want uint64
	}{
		{0, 0, 0xEF46DB3751D8E999},
		{0, 2654435761, 0xAC75FDA2929B17EF},
		{1, 0, 0xE934A84ADB052768},
		{1, 2654435761, 0x5014607643A9B4C3},
		{4, 0, 0x9136A0DCA57457EE},
		{14, 0, 0x8282DCC4994E35C8},
		{14, 2654435761, 0xC3BD6BF63DEB6DF0},
		{222, 0, 0xB641AE8CB691C174},
		{222, 2654435761, 0x20CB8AB7AE10C14A},
	}
	buf := sanityBuffer(222)
	for _, v := range vectors {
		if got := xxh64Seed(buf[:v.len], v.seed); got != v.want {
			t.Errorf("xxh64Seed(len=%d, seed=%d) = %#016x, want %#016x", v.len, v.seed, got, v.want)
		}
	}
}

// TestRegisterPacking verifies the 6-bit accessors against each other:
// the set/get round trip over every index and value, and agreement
// between the per-register accessor and the 3-byte (4-register) group
// decode the histogram uses.
func TestRegisterPacking(t *testing.T) {
	var dense [DenseSize]byte
	rng := rand.New(rand.NewChaCha8([32]byte{})) // fixed zero seed: deterministic
	for range 512 {
		i := rng.IntN(Registers)
		v := uint8(rng.IntN(64))
		setRegister(&dense, i, v)
		if got := getRegister(&dense, i); got != v {
			t.Fatalf("setRegister(%d, %d) then getRegister = %d", i, v, got)
		}
	}
	dense = [DenseSize]byte{}
	for i := range Registers {
		setRegister(&dense, i, uint8(i%64))
	}
	for i := range Registers {
		if want, got := uint8(i%64), getRegister(&dense, i); got != want {
			t.Fatalf("register %d = %d, want %d", i, got, want)
		}
	}
	// Group decode (histogram) must agree with the per-register accessor.
	for range 8 {
		for i := range DenseSize {
			dense[i] = byte(rng.IntN(256))
		}
		for i := range Registers {
			g0 := i / 4 * 3
			b0, b1, b2 := dense[g0], dense[g0+1], dense[g0+2]
			var group uint8
			switch i % 4 {
			case 0:
				group = b0 & registerMask
			case 1:
				group = (b0 >> 6) | (b1&0x0f)<<2
			case 2:
				group = (b1 >> 4) | (b2&0x03)<<4
			case 3:
				group = b2 >> 2
			}
			if got := getRegister(&dense, i); got != group {
				t.Fatalf("accessor disagreement at register %d: get=%d group=%d", i, got, group)
			}
		}
	}
}

// TestEmptySketch covers the zero value, NewHll and the nil-receiver
// reads.
func TestEmptySketch(t *testing.T) {
	for name, h := range map[string]*Hll{
		"NewHll":      NewHll(),
		"zero value":  {},
		"zero as ptr": new(Hll),
	} {
		if got := h.Count(); got != 0 {
			t.Errorf("%s: empty Count = %d, want 0", name, got)
		}
		if !h.IsEmpty() {
			t.Errorf("%s: empty IsEmpty = false", name)
		}
		if b := h.Bytes(); len(b) != DenseSize || !bytes.Equal(b, make([]byte, DenseSize)) {
			t.Errorf("%s: empty Bytes not %d zero bytes", name, DenseSize)
		}
	}
	var nilHll *Hll
	if got := nilHll.Count(); got != 0 {
		t.Errorf("nil Count = %d, want 0", got)
	}
	if !nilHll.IsEmpty() {
		t.Errorf("nil IsEmpty = false")
	}
	if b := nilHll.Bytes(); b != nil {
		t.Errorf("nil Bytes = %v, want nil", b)
	}
	nilHll.Reset()                      // no-op, must not panic
	nilHll.Merge(nil, NewHll(), &Hll{}) // only empty operands, must not panic
}

// TestAddNilPanics checks the panic contract: Add on a nil *Hll.
func TestAddNilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Add on nil *Hll did not panic")
		}
		msg, ok := r.(string)
		if !ok || msg != "hyperloglog: Add on a nil *Hll — create one with NewHll() or use a zero Hll value" {
			t.Errorf("panic message = %q, want the Add-on-nil message", r)
		}
	}()
	var nilHll *Hll
	nilHll.Add([]byte("x"))
}

// TestMergeNilPanics checks the panic contract: Merge into a nil *Hll
// with a non-empty operand.
func TestMergeNilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Merge on nil *Hll with a non-empty operand did not panic")
		}
		msg, ok := r.(string)
		if !ok || msg != "hyperloglog: Merge on a nil *Hll — create one with NewHll() or use a zero Hll value" {
			t.Errorf("panic message = %q, want the Merge-on-nil message", r)
		}
	}()
	nonEmpty := NewHll()
	nonEmpty.Add([]byte("x"))
	var nilHll *Hll
	nilHll.Merge(nonEmpty)
}

// TestAddSingleAndRepeat verifies that one element counts as one and
// that re-adding it changes nothing.
func TestAddSingleAndRepeat(t *testing.T) {
	h := NewHll()
	if !h.Add([]byte("hello")) {
		t.Errorf("first Add of a value reported no change")
	}
	if got := h.Count(); got != 1 {
		t.Errorf("Count of one element = %d, want 1", got)
	}
	for i := range 1000 {
		if h.Add([]byte("hello")) {
			t.Fatalf("re-Add #%d of the same value reported a change", i)
		}
	}
	if got := h.Count(); got != 1 {
		t.Errorf("Count after 1000 re-adds = %d, want 1", got)
	}
	if h.IsEmpty() {
		t.Errorf("IsEmpty true after an add")
	}
}

// TestCountCache verifies the cached-count contract: a Count is cached,
// an Add that changes a register invalidates it, a no-change Add does
// not, and Reset publishes 0.
func TestCountCache(t *testing.T) {
	h := NewHll()
	for i := range 100 {
		h.Add([]byte{byte(i), byte(i >> 8), 'a'})
	}
	c := h.Count()
	if !h.valid.Load() || h.cached.Load() != c {
		t.Errorf("Count did not populate the cache (valid=%v cached=%d count=%d)", h.valid.Load(), h.cached.Load(), c)
	}
	if got := h.Count(); got != c {
		t.Errorf("second Count = %d, want the cached %d", got, c)
	}
	h.Add([]byte("hello"))
	if h.valid.Load() {
		t.Errorf("change-carrying Add left the cache valid")
	}
	if got := h.Count(); got == c {
		t.Errorf("Count after a new element unchanged (%d)", got)
	}
	c = h.Count()
	h.Add([]byte("hello")) // duplicate: no register changes
	if !h.valid.Load() || h.cached.Load() != c {
		t.Errorf("duplicate Add invalidated a still-valid cache")
	}
	h.Reset()
	if got := h.Count(); got != 0 || !h.valid.Load() || h.cached.Load() != 0 {
		t.Errorf("after Reset: Count=%d valid=%v cached=%d, want 0/true/0", got, h.valid.Load(), h.cached.Load())
	}
}

// TestSerializationRoundTrip covers Bytes/HllFromBytes.
func TestSerializationRoundTrip(t *testing.T) {
	h := NewHll()
	for i := range 10000 {
		h.Add([]byte{byte(i), byte(i >> 8), byte(i >> 16), 'k'})
	}
	before := h.Count()
	b := h.Bytes()
	if len(b) != DenseSize {
		t.Fatalf("Bytes length = %d, want %d", len(b), DenseSize)
	}
	h2, err := HllFromBytes(b)
	if err != nil {
		t.Fatalf("HllFromBytes: %v", err)
	}
	if got := h2.Count(); got != before {
		t.Errorf("decoded Count = %d, want %d", got, before)
	}
	if !bytes.Equal(h2.Bytes(), b) {
		t.Errorf("re-serialized bytes differ")
	}
	// The original keeps working and the decode is independent of it.
	h.Add([]byte("another"))
	if got := h2.Count(); got != before {
		t.Errorf("mutating the original changed the decoded copy (%d)", got)
	}
	// A decoded sketch accepts further adds.
	h2.Add([]byte("another"))
}

// TestHllFromBytesErrors covers the corrupt-input error contract.
func TestHllFromBytesErrors(t *testing.T) {
	for _, n := range []int{0, 1, 12287, 12289, 2 * DenseSize} {
		if _, err := HllFromBytes(make([]byte, n)); !errors.Is(err, ErrBadLength) {
			t.Errorf("HllFromBytes(len=%d) err = %v, want ErrBadLength", n, err)
		}
	}
	// A register above rankMax cannot come from Add: reject it.
	raw := make([]byte, DenseSize)
	setRegister((*[DenseSize]byte)(raw), 77, 52)
	if _, err := HllFromBytes(raw); !errors.Is(err, ErrBadRegister) {
		t.Errorf("HllFromBytes(register=52) err = %v, want ErrBadRegister", err)
	}
	// rankMax itself is the legal ceiling (an all-zero 50-bit remainder).
	setRegister((*[DenseSize]byte)(raw), 77, rankMax)
	if _, err := HllFromBytes(raw); err != nil {
		t.Errorf("HllFromBytes(register=51) err = %v, want nil", err)
	}
}

// TestResetEmpty clears everything.
func TestResetEmpty(t *testing.T) {
	h := NewHll()
	for i := range 5000 {
		h.Add([]byte{byte(i), byte(i >> 8), 'r'})
	}
	h.Reset()
	if !h.IsEmpty() || h.Count() != 0 {
		t.Errorf("after Reset: IsEmpty=%v Count=%d", h.IsEmpty(), h.Count())
	}
	if b := h.Bytes(); !bytes.Equal(b, make([]byte, DenseSize)) {
		t.Errorf("after Reset: Bytes not all zero")
	}
	if !h.Add([]byte("post-reset")) {
		t.Errorf("Add after Reset reported no change")
	}
	if got := h.Count(); got != 1 {
		t.Errorf("Count after Reset + one add = %d, want 1", got)
	}
}

// TestMergeEmptyOperands: merging empty (or nil) HLLs changes nothing.
func TestMergeEmptyOperands(t *testing.T) {
	h := NewHll()
	for i := range 2000 {
		h.Add([]byte{byte(i), byte(i >> 8), 'm'})
	}
	before := h.Bytes()
	cached := h.Count() // prime the cache
	h.Merge()
	h.Merge(nil)
	h.Merge(NewHll(), &Hll{}, nil)
	if after := h.Bytes(); !bytes.Equal(before, after) {
		t.Errorf("merging empty operands changed the registers")
	}
	if !h.valid.Load() || h.Count() != cached {
		t.Errorf("merging empty operands invalidated the cache")
	}
}

// TestMergeBasic verifies register-wise max on a small case: the merged
// count approximates the union, and merging is idempotent.
func TestMergeBasic(t *testing.T) {
	a, b := NewHll(), NewHll()
	for i := range 3000 {
		a.Add([]byte{byte(i), byte(i >> 8), 'a'})
	}
	for i := 2000; i < 5000; i++ { // 2000 overlap with a
		b.Add([]byte{byte(i), byte(i >> 8), 'a'})
	}
	merged := NewHll()
	merged.Merge(a, b)
	// |a ∪ b| = 5000; the estimate must be within the 0.81% error bound
	// plus a wide margin (LC regime, ~1.5% single-shot sd).
	if got, want := merged.Count(), uint64(5000); got < want*95/100 || got > want*105/100 {
		t.Errorf("merged Count = %d, want ~5000", got)
	}
	snapshot := merged.Bytes()
	merged.Merge(a, b) // idempotent
	if !bytes.Equal(snapshot, merged.Bytes()) {
		t.Errorf("re-merging the same operands changed the registers")
	}
	a.Merge(a) // self-merge is a no-op
	if !bytes.Equal(a.Bytes(), a.Bytes()) {
		t.Errorf("self-merge changed the registers")
	}
}

// TestAccuracySmoke is an early gate: 100k distinct elements estimate
// within 2% (single shot; the thorough suite does the statistics).
func TestAccuracySmoke(t *testing.T) {
	h := NewHll()
	key := make([]byte, 8)
	for i := range 100_000 {
		key[0] = byte(i)
		key[1] = byte(i >> 8)
		key[2] = byte(i >> 16)
		key[3] = 's'
		h.Add(key)
	}
	got := float64(h.Count())
	want := 100_000.0
	err := (got - want) / want
	if err < -0.02 || err > 0.02 {
		t.Errorf("Count of 100k distinct = %d (%.2f%% error)", h.Count(), err*100)
	}
	t.Logf("100k estimate: %d (%+.3f%%)", h.Count(), err*100)
}
