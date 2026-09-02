/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog_ts

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/pschlump/pluto/hyperloglog"
)

// TestBasics covers the zero value, NewHll, Add/Count/IsEmpty/Bytes and
// the serialization pair on the thread-safe type.
func TestBasics(t *testing.T) {
	for name, h := range map[string]*Hll{
		"NewHll":     NewHll(),
		"zero value": {},
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
		if !h.Add([]byte("hello")) {
			t.Errorf("%s: first Add reported no change", name)
		}
		if h.Add([]byte("hello")) {
			t.Errorf("%s: duplicate Add reported a change", name)
		}
		if got := h.Count(); got != 1 {
			t.Errorf("%s: Count of one element = %d, want 1", name, got)
		}
	}

	h := NewHll()
	for i := range 10_000 {
		var key [8]byte
		binary.LittleEndian.PutUint64(key[:], uint64(i))
		h.Add(key[:])
	}
	c := h.Count()
	decoded, err := HllFromBytes(h.Bytes())
	if err != nil {
		t.Fatalf("HllFromBytes: %v", err)
	}
	if decoded.Count() != c {
		t.Errorf("decoded Count = %d, want %d", decoded.Count(), c)
	}
	if _, err := HllFromBytes(make([]byte, 42)); !errors.Is(err, ErrBadLength) {
		t.Errorf("HllFromBytes(short) err = %v, want ErrBadLength", err)
	}
	raw := make([]byte, DenseSize)
	rawDense(raw, 5, 52)
	if _, err := HllFromBytes(raw); !errors.Is(err, ErrBadRegister) {
		t.Errorf("HllFromBytes(register=52) err = %v, want ErrBadRegister", err)
	}
}

// rawDense writes register i (0..16383) with value v into a raw dense
// byte slice (the plain package's 6-bit packing, mirrored for tests).
func rawDense(raw []byte, i int, v uint8) {
	b := i * 6 / 8
	fb := i * 6 & 7
	raw[b] &= ^(byte(0x3f) << fb)
	raw[b] |= v << fb
	if fb > 2 {
		raw[b+1] &= ^(byte(0x3f) >> (8 - fb))
		raw[b+1] |= v >> (8 - fb)
	}
}

// TestNilContract: nil reads are tolerated, the two write panics name
// the method, and Lock/Unlock on nil no-op.
func TestNilContract(t *testing.T) {
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
	nilHll.Reset()
	nilHll.Merge(nil, NewHll(), &Hll{})
	nilHll.Lock()
	nilHll.Unlock()

	func() {
		defer func() {
			r := recover()
			msg, ok := r.(string)
			if !ok || msg != "hyperloglog_ts: Add on a nil *Hll — create one with NewHll() or use a zero Hll value" {
				t.Errorf("Add panic message = %q", r)
			}
		}()
		nilHll.Add([]byte("x"))
	}()
	func() {
		defer func() {
			r := recover()
			msg, ok := r.(string)
			if !ok || msg != "hyperloglog_ts: Merge on a nil *Hll — create one with NewHll() or use a zero Hll value" {
				t.Errorf("Merge panic message = %q", r)
			}
		}()
		nonEmpty := NewHll()
		nonEmpty.Add([]byte("x"))
		nilHll.Merge(nonEmpty)
	}()
}

// TestParityWithPlain: the same key stream through either twin
// produces identical registers and counts (delegation correctness —
// the twin must be the plain structure plus only the lock).
func TestParityWithPlain(t *testing.T) {
	plain := hyperloglog.NewHll()
	ts := NewHll()
	for i := range 50_000 {
		var key [8]byte
		binary.LittleEndian.PutUint64(key[:], uint64(i)*0x9E3779B97F4A7C15)
		pc := plain.Add(key[:])
		tc := ts.Add(key[:])
		if pc != tc {
			t.Fatalf("add %d: changed plain=%v ts=%v", i, pc, tc)
		}
	}
	if plain.Count() != ts.Count() {
		t.Errorf("counts differ: plain=%d ts=%d", plain.Count(), ts.Count())
	}
	if !bytes.Equal(plain.Bytes(), ts.Bytes()) {
		t.Errorf("registers differ between twins")
	}
}

// TestMergeSplitExact: splitting distinct elements across sketches and
// merging yields registers identical to the all-in-one sketch —
// bit-exact, not a tolerance (single-threaded mirror of the plain
// package's test).
func TestMergeSplitExact(t *testing.T) {
	const n = 100_000
	want := NewHll()
	for i := uint64(0); i < n; i++ {
		var key [8]byte
		binary.LittleEndian.PutUint64(key[:], i)
		want.Add(key[:])
	}
	for _, k := range []int{2, 5} {
		parts := make([]*Hll, k)
		for i := range parts {
			parts[i] = NewHll()
		}
		for i := uint64(0); i < n; i++ {
			var key [8]byte
			binary.LittleEndian.PutUint64(key[:], i)
			parts[int(i)%k].Add(key[:])
		}
		merged := NewHll()
		merged.Merge(parts...)
		if !bytes.Equal(merged.Bytes(), want.Bytes()) {
			t.Errorf("k=%d: merged registers differ from the all-in-one sketch", k)
		}
	}
}

// TestCrossTwinCompatibility: a sketch serialized by either twin decodes
// and merges in both — twin-switching is an import change.
func TestCrossTwinCompatibility(t *testing.T) {
	ts := NewHll()
	for i := range 20_000 {
		var key [8]byte
		binary.LittleEndian.PutUint64(key[:], uint64(i))
		ts.Add(key[:])
	}
	plainDecoded, err := hyperloglog.HllFromBytes(ts.Bytes())
	if err != nil {
		t.Fatalf("plain decode of ts bytes: %v", err)
	}
	if plainDecoded.Count() != ts.Count() {
		t.Errorf("plain-decoded count %d != ts count %d", plainDecoded.Count(), ts.Count())
	}
	tsDecoded, err := HllFromBytes(plainDecoded.Bytes())
	if err != nil {
		t.Fatalf("ts decode of plain bytes: %v", err)
	}
	merged := NewHll()
	merged.Merge(tsDecoded, wrapPlain(plainDecoded))
	if got := merged.Count(); got < 19_000 || got > 21_000 {
		t.Errorf("cross-twin merged count = %d, want ~20000", got)
	}
}

// wrapPlain moves a plain sketch into the twin type via serialization.
func wrapPlain(p *hyperloglog.Hll) *Hll {
	h, err := HllFromBytes(p.Bytes())
	if err != nil {
		panic(err)
	}
	return h
}
