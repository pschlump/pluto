/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// expectPanicMsg runs fx, requires it to panic, and requires the panic
// message to contain every fragment (the method name and the fix).
func expectPanicMsg(t *testing.T, what string, fx func(), fragments ...string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", what)
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("%s panicked with %v, not a message string", what, r)
			return
		}
		for _, frag := range fragments {
			if !strings.Contains(msg, frag) {
				t.Errorf("%s panic message %q should mention %q", what, msg, frag)
			}
		}
	}()
	fx()
}

// TestNewBloomOptimalParams pins the sizing formulas: the classic
// optima m = ceil(-n·ln(p)/(ln 2)²) and k = round(ln(2)·m/n), recomputed
// independently here, with two frozen magic points.
func TestNewBloomOptimalParams(t *testing.T) {
	for _, pt := range []struct {
		n int
		p float64
	}{
		{1, 0.5}, {1, 0.01}, {10, 0.5}, {100, 0.01}, {1_000, 0.001},
		{10_000, 0.01}, {100_000, 0.0001}, {1_000_000, 0.001}, {10_000_000, 0.01},
		{1, 0.999}, // m rounds to 1 bit, k clamps up to 1
	} {
		b := NewBloom(pt.n, pt.p)
		wantM := uint64(math.Ceil(float64(pt.n) * -math.Log(pt.p) / (math.Ln2 * math.Ln2)))
		if got := b.BitCount(); got != wantM {
			t.Errorf("NewBloom(%d, %g): BitCount = %d, want %d", pt.n, pt.p, got, wantM)
		}
		wantK := int(math.Round(float64(wantM) / float64(pt.n) * math.Ln2))
		wantK = max(1, min(maxHashes, wantK))
		if got := b.HashCount(); got != wantK {
			t.Errorf("NewBloom(%d, %g): HashCount = %d, want %d", pt.n, pt.p, got, wantK)
		}
		if len(b.bits) != int((wantM+63)/64) {
			t.Errorf("NewBloom(%d, %g): %d words for %d bits", pt.n, pt.p, len(b.bits), wantM)
		}
		if !b.IsEmpty() || b.Added() != 0 || b.Saturation() != 0 {
			t.Errorf("NewBloom(%d, %g): fresh filter is not empty", pt.n, pt.p)
		}
	}
	// Frozen magic points: the textbook 10k-at-1% filter.
	if b := NewBloom(10_000, 0.01); b.BitCount() != 95851 || b.HashCount() != 7 {
		t.Errorf("NewBloom(10000, 0.01) = m %d, k %d; want m 95851, k 7", b.BitCount(), b.HashCount())
	}
}

// TestNewBloomPanics is the constructor half of the panic contract.
func TestNewBloomPanics(t *testing.T) {
	expectPanicMsg(t, "NewBloom(0, 0.01)", func() { NewBloom(0, 0.01) }, "NewBloom", "n")
	expectPanicMsg(t, "NewBloom(-5, 0.01)", func() { NewBloom(-5, 0.01) }, "NewBloom", "n")
	expectPanicMsg(t, "NewBloom(10, 0)", func() { NewBloom(10, 0) }, "NewBloom", "p")
	expectPanicMsg(t, "NewBloom(10, 1)", func() { NewBloom(10, 1) }, "NewBloom", "p")
	expectPanicMsg(t, "NewBloom(10, -0.1)", func() { NewBloom(10, -0.1) }, "NewBloom", "p")
	expectPanicMsg(t, "NewBloom(10, NaN)", func() { NewBloom(10, math.NaN()) }, "NewBloom", "p")
	expectPanicMsg(t, "NewBloom(1e10, 0.01)", func() { NewBloom(10_000_000_000, 0.01) }, "NewBloom", "maxBits")

	expectPanicMsg(t, "NewBloomBits(0, 7)", func() { NewBloomBits(0, 7) }, "NewBloomBits", "bits")
	expectPanicMsg(t, "NewBloomBits(maxBits+1, 7)", func() { NewBloomBits(maxBits+1, 7) }, "NewBloomBits", "bits")
	expectPanicMsg(t, "NewBloomBits(64, 0)", func() { NewBloomBits(64, 0) }, "NewBloomBits", "hashes")
	expectPanicMsg(t, "NewBloomBits(64, 65)", func() { NewBloomBits(64, 65) }, "NewBloomBits", "hashes")
}

// TestBasics ports the 2016 library's behavioral tests to this API,
// including its tiny 5-bit/2-probe shape.
func TestBasics(t *testing.T) {
	b := NewBloomBits(5, 2) // the 2016 library's NewBloomFilter(5)
	for _, s := range []string{"", "abc", "tahiti", "Mookie", "lala"} {
		b.Add([]byte(s))
		if !b.MayContain([]byte(s)) {
			t.Errorf("MayContain(%q) false after Add — a false negative", s)
		}
	}
	if b.Added() != 5 {
		t.Errorf("Added = %d, want 5", b.Added())
	}
	// The empty element exercises the degenerate-step guard (its
	// SuperFastHash is 0, congruent to 0 modulo every m).
	if !b.MayContain([]byte("")) {
		t.Error("MayContain(\"\") false after Add(\"\")")
	}

	// A 1-bit, 1-probe filter: everything collides with everything.
	tiny := NewBloomBits(1, 1)
	tiny.Add([]byte("x"))
	for _, s := range []string{"a", "b", "the", "rest", "of", "the", "corpus"} {
		if !tiny.MayContain([]byte(s)) {
			t.Errorf("1-bit filter: MayContain(%q) false after an Add — every probe is bit 0", s)
		}
	}
}

// TestAddReturnLaw pins the exact return-value laws: Add reports whether
// any probe bit was previously clear — precisely the inverse of the
// pre-call MayContain — so a duplicate Add always reports false, and
// TestAndSet reports exactly what Add would have.
func TestAddReturnLaw(t *testing.T) {
	b := NewBloom(1000, 0.01)
	rng := newTestRng(42)
	keys := make([][]byte, 500)
	for i := range keys {
		keys[i] = keyOf(rng, 12)
	}
	for i, k := range keys {
		before := b.MayContain(k)
		if changed := b.Add(k); changed == before {
			t.Errorf("key %d: Add returned %v with prior MayContain %v — must be the inverse", i, changed, before)
		}
		if again := b.Add(k); again {
			t.Errorf("key %d: duplicate Add reported a change", i)
		}
	}
	b.Reset()
	for i, k := range keys {
		// The first sight of a key on a fresh filter is a miss unless a
		// false positive lands; the filter is nearly empty for the first
		// handful, so those are exact.
		if present := b.TestAndSet(k); present && i < 3 {
			t.Errorf("key %d: TestAndSet reported present on a fresh filter", i)
		}
		if !b.MayContain(k) {
			t.Errorf("key %d: false negative after TestAndSet", i)
		}
		if present := b.TestAndSet(k); !present {
			t.Errorf("key %d: second TestAndSet reported absent", i)
		}
	}
}

// TestNilAndZeroValue is the tolerance half of the panic contract:
// every read answers, only the writes have no sane answer.
func TestNilAndZeroValue(t *testing.T) {
	var nilB *Bloom
	if nilB.MayContain([]byte("x")) {
		t.Error("nil MayContain reported true")
	}
	if nilB.Count() != 0 || nilB.Added() != 0 || nilB.Saturation() != 0 {
		t.Error("nil counters are nonzero")
	}
	if !nilB.IsEmpty() {
		t.Error("nil IsEmpty is false")
	}
	if nilB.BitCount() != 0 || nilB.HashCount() != 0 {
		t.Error("nil shape is nonzero")
	}
	if nilB.Bytes() != nil {
		t.Error("nil Bytes is not nil")
	}
	if s := nilB.String(); s != "Bloom (empty)" {
		t.Errorf("nil String = %q", s)
	}
	nilB.Reset()              // no-op, must not panic
	nilB.Merge(nil, &Bloom{}) // only empty operands: tolerated no-op
	if c := nilB.Clone(); c != nil {
		t.Error("nil Clone is not nil")
	}

	var zero Bloom // same answers as nil
	if zero.MayContain([]byte("x")) || !zero.IsEmpty() || zero.Bytes() != nil {
		t.Error("zero value does not read as empty")
	}

	expectPanicMsg(t, "Add on nil", func() { nilB.Add([]byte("x")) }, "Add", "NewBloom")
	expectPanicMsg(t, "TestAndSet on nil", func() { nilB.TestAndSet([]byte("x")) }, "TestAndSet", "NewBloom")
	expectPanicMsg(t, "Merge into nil", func() {
		o := NewBloom(10, 0.1)
		o.Add([]byte("e"))
		nilB.Merge(o)
	}, "Merge", "NewBloom")
	expectPanicMsg(t, "Add on zero value", func() { zero.Add([]byte("x")) }, "Add", "NewBloom")
	expectPanicMsg(t, "TestAndSet on zero value", func() { zero.TestAndSet([]byte("x")) }, "TestAndSet", "NewBloom")
}

// TestMergeShapeContract: contributing operands must share the exact
// shape; empty operands and self-merges never contribute.
func TestMergeShapeContract(t *testing.T) {
	a := NewBloom(100, 0.01)
	a.Add([]byte("x"))
	bigger := NewBloom(200, 0.01)
	bigger.Add([]byte("y"))
	expectPanicMsg(t, "Merge different m", func() { a.Merge(bigger) }, "Merge", "same NewBloom")

	differentK := NewBloomBits(a.BitCount(), a.HashCount()+1)
	differentK.Add([]byte("z")) // an empty operand is skipped regardless of shape
	expectPanicMsg(t, "Merge different k", func() { a.Merge(differentK) }, "Merge", "same NewBloom")

	// Empty and nil operands are skipped regardless of shape.
	empty := NewBloom(1_000_000, 0.01)
	before := a.String()
	a.Merge(nil, &Bloom{}, empty)
	if after := a.String(); after != before {
		t.Errorf("merging nil/empty operands changed the filter: %q -> %q", before, after)
	}

	// Self-merge is a no-op (added must not double).
	added := a.Added()
	set := a.set
	a.Merge(a)
	if a.Added() != added || a.set != set {
		t.Error("self-merge changed counters")
	}
}

// TestCloneReset verifies the copies are independent and the reset is
// total.
func TestCloneReset(t *testing.T) {
	b := NewBloom(100, 0.01)
	for i := range 50 {
		b.Add(keyOf(newTestRng(int64(i)), 8))
	}
	c := b.Clone()
	for i := range 10 { // mutate the clone only
		c.Add([]byte{byte(i), 'c', 'l', 'o', 'n', 'e'})
	}
	if c.set <= b.set || c.added <= b.added {
		t.Error("clone mutation visible in the original — not independent")
	}
	for i := range 10 {
		if !c.MayContain([]byte{byte(i), 'c', 'l', 'o', 'n', 'e'}) {
			t.Error("clone lost its own adds")
		}
	}

	b.Reset()
	if !b.IsEmpty() || b.Added() != 0 || b.Saturation() != 0 || b.Count() != 0 {
		t.Error("Reset left residue")
	}
	if b.MayContain([]byte{0, 'c', 'l', 'o', 'n', 'e'}) {
		t.Error("fresh filter reported present")
	}
}

// TestCountSaturation pins the estimator's degenerate corners: the empty
// filter reports 0, the fully saturated 1-bit filter reports m (no
// information — the documented stand-in), and a design-load filter
// lands near its n.
func TestCountSaturation(t *testing.T) {
	if got := (&Bloom{}).Count(); got != 0 {
		t.Errorf("zero-value Count = %d, want 0", got)
	}
	tiny := NewBloomBits(1, 1)
	tiny.Add([]byte("only"))
	if got := tiny.Saturation(); got != 1 {
		t.Errorf("1-bit filter saturation = %g, want 1", got)
	}
	if got := tiny.Count(); got != 1 {
		t.Errorf("saturated 1-bit Count = %d, want m=1", got)
	}

	b := NewBloom(10_000, 0.01)
	rng := newTestRng(42)
	for i := 0; i < 10_000; i++ {
		b.Add(keyOf(rng, 8))
	}
	est := b.Count()
	if est < 9_000 || est > 11_000 {
		t.Errorf("Count at design load = %d, want ~10000", est)
	}
	if s := b.Saturation(); s < 0.45 || s > 0.55 {
		t.Errorf("saturation at design load = %g, want ~0.5 (optimal k implies half the bits set)", s)
	}
}

// TestBytesShape checks the serialized length formula and header
// fields.
func TestBytesShape(t *testing.T) {
	if got := (&Bloom{}).Bytes(); got != nil {
		t.Error("zero-value Bytes is not nil")
	}
	for _, m := range []uint64{1, 63, 64, 65, 101, 95851} {
		b := NewBloomBits(m, 3)
		b.Add([]byte("element"))
		data := b.Bytes()
		want := headerSize + int((m+63)/64)*8
		if len(data) != want {
			t.Errorf("m=%d: len(Bytes) = %d, want %d", m, len(data), want)
		}
		if got := b.BitCount(); got != m {
			t.Errorf("m=%d: BitCount = %d", m, got)
		}
	}
}

// TestFromBytesErrors pins every corrupt-data error, checked with
// errors.Is — corrupt input never panics.
func TestFromBytesErrors(t *testing.T) {
	good := NewBloom(100, 0.01)
	good.Add([]byte("v"))
	data := good.Bytes()

	if _, err := BloomFromBytes(nil); !errors.Is(err, ErrBadLength) {
		t.Errorf("nil data: err = %v", err)
	}
	if _, err := BloomFromBytes(data[:headerSize-1]); !errors.Is(err, ErrBadLength) {
		t.Errorf("short data: err = %v", err)
	}

	badM := cloneBytes(data)
	badM[0] = 0 // m = 0
	if _, err := BloomFromBytes(badM); !errors.Is(err, ErrBadLength) {
		t.Errorf("m=0: err = %v", err)
	}

	badLen := cloneBytes(data)
	badLen = append(badLen, 0) // one byte too long
	if _, err := BloomFromBytes(badLen); !errors.Is(err, ErrBadLength) {
		t.Errorf("trailing byte: err = %v", err)
	}

	badK := cloneBytes(data)
	badK[8] = 0 // k = 0
	if _, err := BloomFromBytes(badK); !errors.Is(err, ErrBadHashes) {
		t.Errorf("k=0: err = %v", err)
	}
	for _, k := range []uint64{1, maxHashes} { // the inclusive bounds decode
		okK := cloneBytes(data)
		putUint64(okK[8:], k)
		b, err := BloomFromBytes(okK)
		if err != nil || b.HashCount() != int(k) {
			t.Errorf("k=%d should decode: %v", k, err)
		}
	}
	overK := cloneBytes(data)
	putUint64(overK[8:], maxHashes+1)
	if _, err := BloomFromBytes(overK); !errors.Is(err, ErrBadHashes) {
		t.Errorf("k=maxHashes+1: err = %v", err)
	}

	// A set bit at or above m: craft a filter with m not a multiple of
	// 64 and flip a padding bit.
	pad := NewBloomBits(70, 2) // 2 words, bits 70..63+64-1 are padding
	pad.Add([]byte("v"))
	padData := pad.Bytes()
	putUint64(padData[headerSize+8:], uint64(1)<<(70&63)) // bit 70, inside word 1's padding
	if _, err := BloomFromBytes(padData); !errors.Is(err, ErrBadBits) {
		t.Errorf("padding bit set: err = %v", err)
	}

	// Random garbage never panics and only reports the three errors.
	rng := newTestRng(7)
	for range 500 {
		junk := make([]byte, rng.IntN(80))
		for i := range junk {
			junk[i] = byte(rng.IntN(256))
		}
		_, err := BloomFromBytes(junk)
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrBadLength) && !errors.Is(err, ErrBadHashes) && !errors.Is(err, ErrBadBits) {
			t.Fatalf("junk %q: err = %v is none of the three sentinels", junk, err)
		}
	}
}

// TestString is a smoke test of the summary line.
func TestString(t *testing.T) {
	b := NewBloomBits(64, 1) // one probe: one Add sets exactly one bit
	b.Add([]byte("k"))
	s := b.String()
	for _, frag := range []string{"m=64", "k=1", "bits-set=1", "added=1"} {
		if !strings.Contains(s, frag) {
			t.Errorf("String %q missing %q", s, frag)
		}
	}
}
