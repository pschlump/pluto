/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom_ts

import (
	"bytes"
	"errors"
	"math/rand/v2"
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

func keyOf(rng *rand.Rand, n int) []byte {
	k := make([]byte, n)
	for i := range k {
		k[i] = byte(rng.IntN(256))
	}
	return k
}

// TestBasics ports the plain package's behavioral test: adds are always
// found, duplicates never change anything, TestAndSet marks first
// sightings.
func TestBasics(t *testing.T) {
	b := NewBloom(1000, 0.01)
	rng := rand.New(rand.NewPCG(42, 7))
	keys := make([][]byte, 300)
	for i := range keys {
		keys[i] = keyOf(rng, 12)
	}
	for i, k := range keys {
		before := b.MayContain(k)
		if changed := b.Add(k); changed == before {
			t.Errorf("key %d: Add return must be the inverse of prior MayContain", i)
		}
		if again := b.Add(k); again {
			t.Errorf("key %d: duplicate Add reported a change", i)
		}
	}
	for i, k := range keys {
		if !b.MayContain(k) {
			t.Errorf("key %d: false negative", i)
		}
	}
	if b.Added() != 600 {
		t.Errorf("Added = %d, want 600", b.Added())
	}

	b.Reset()
	for i, k := range keys {
		if present := b.TestAndSet(k); present && i < 3 {
			t.Errorf("key %d: TestAndSet reported present on a fresh filter", i)
		}
		if present := b.TestAndSet(k); !present {
			t.Errorf("key %d: second TestAndSet reported absent", i)
		}
	}
	if b.Added() != 600 {
		t.Errorf("Added after reset+TestAndSet = %d, want 600", b.Added())
	}
}

// TestNilAndZeroValue is the tolerance half of the twin's panic
// contract: every read answers, the writes have no sane answer, and the
// nil guards run before any lock acquisition.
func TestNilAndZeroValue(t *testing.T) {
	var nilB *Bloom
	if nilB.MayContain([]byte("x")) || !nilB.IsEmpty() || nilB.Bytes() != nil ||
		nilB.Count() != 0 || nilB.Added() != 0 || nilB.Saturation() != 0 ||
		nilB.BitCount() != 0 || nilB.HashCount() != 0 {
		t.Error("nil does not read as empty")
	}
	if s := nilB.String(); s != "Bloom (empty)" {
		t.Errorf("nil String = %q", s)
	}
	nilB.Reset()                                 // no-op, must not panic
	nilB.Merge(nil, &Bloom{}, NewBloom(10, 0.1)) // only empty operands: tolerated
	if c := nilB.Clone(); c != nil {
		t.Error("nil Clone is not nil")
	}

	var zero Bloom
	if zero.MayContain([]byte("x")) || !zero.IsEmpty() || zero.Bytes() != nil {
		t.Error("zero value does not read as empty")
	}
	if c := zero.Clone(); c != nil {
		t.Error("zero-value Clone is not nil")
	}
	zero.Reset()
	zero.Merge(nil)

	const fix = "NewBloom"
	expectPanicMsg(t, "Add on nil", func() { nilB.Add([]byte("x")) }, "bloom_ts:", "Add", fix)
	expectPanicMsg(t, "TestAndSet on nil", func() { nilB.TestAndSet([]byte("x")) }, "bloom_ts:", "TestAndSet", fix)
	expectPanicMsg(t, "Merge into nil", func() {
		o := NewBloom(10, 0.1)
		o.Add([]byte("e"))
		nilB.Merge(o)
	}, "bloom_ts:", "Merge", fix)
	expectPanicMsg(t, "Add on zero value", func() { zero.Add([]byte("x")) }, "bloom_ts:", "Add", fix)
	expectPanicMsg(t, "TestAndSet on zero value", func() { zero.TestAndSet([]byte("x")) }, "bloom_ts:", "TestAndSet", fix)
	expectPanicMsg(t, "Merge into zero value", func() {
		o := NewBloom(10, 0.1)
		o.Add([]byte("e"))
		zero.Merge(o)
	}, "bloom_ts:", "Merge", fix)
}

// TestMergeShapeContract: contributing operands must share the exact
// shape (the plain core's message fires); empty and nil operands are
// skipped; aliasing operands are skipped.
func TestMergeShapeContract(t *testing.T) {
	a := NewBloom(100, 0.01)
	a.Add([]byte("x"))
	bigger := NewBloom(200, 0.01)
	bigger.Add([]byte("y"))
	expectPanicMsg(t, "Merge different m", func() { a.Merge(bigger) }, "Merge", "same NewBloom")

	differentK := NewBloomBits(a.BitCount(), a.HashCount()+1)
	differentK.Add([]byte("z"))
	expectPanicMsg(t, "Merge different k", func() { a.Merge(differentK) }, "Merge", "same NewBloom")

	before := a.String()
	a.Merge(nil, &Bloom{}, NewBloom(1_000_000, 0.001))
	a.Merge(a) // aliasing operand: skipped, counters unchanged
	if after := a.String(); after != before {
		t.Errorf("no-op merges changed the filter: %q -> %q", before, after)
	}

	// A real merge unions: every operand element is present afterwards.
	b1, b2 := NewBloom(1000, 0.01), NewBloom(1000, 0.01)
	rng := rand.New(rand.NewPCG(42, 7))
	left, right := keyOf(rng, 8), keyOf(rng, 8)
	b1.Add(left)
	b2.Add(right)
	b1.Merge(b2)
	if !b1.MayContain(left) || !b1.MayContain(right) {
		t.Error("merged filter lost an operand element — a false negative")
	}
	if b1.Added() != 2 {
		t.Errorf("merged Added = %d, want 2", b1.Added())
	}
	// b2 is unchanged (its own element is the exact law; reporting left
	// would only be a possible false positive).
	if !b2.MayContain(right) {
		t.Error("merge mutated the operand — lost its own element")
	}
	if b2.Added() != 1 {
		t.Errorf("operand Added = %d, want 1 (merge must not mutate operands)", b2.Added())
	}
}

// TestCloneReset: the clone is an independent snapshot.
func TestCloneReset(t *testing.T) {
	b := NewBloom(100, 0.01)
	rng := rand.New(rand.NewPCG(42, 7))
	for range 50 {
		b.Add(keyOf(rng, 8))
	}
	c := b.Clone()
	for i := range 10 {
		c.Add([]byte{byte(i), 'c'})
	}
	if c.Added() <= b.Added() {
		t.Error("clone mutation visible in the original — not independent")
	}
	for i := range 10 {
		if !c.MayContain([]byte{byte(i), 'c'}) {
			t.Error("clone lost its own adds")
		}
	}
	b.Reset()
	if !b.IsEmpty() || b.Added() != 0 {
		t.Error("Reset left residue")
	}
	if c.IsEmpty() {
		t.Error("Reset reached into the clone")
	}
}

// TestFromBytes round-trips through the twin with the aliased
// sentinels.
func TestFromBytes(t *testing.T) {
	b := NewBloom(100, 0.01)
	b.Add([]byte("v"))
	data := b.Bytes()

	decoded, err := BloomFromBytes(data)
	if err != nil {
		t.Fatalf("BloomFromBytes: %v", err)
	}
	if !decoded.MayContain([]byte("v")) || decoded.Added() != 1 {
		t.Error("decode lost the filter")
	}
	if again := decoded.Bytes(); !bytes.Equal(again, data) {
		t.Error("re-serialize differs")
	}

	if _, err := BloomFromBytes(nil); !errors.Is(err, ErrBadLength) {
		t.Errorf("nil data: err = %v", err)
	}
	if _, err := BloomFromBytes(data[:10]); !errors.Is(err, ErrBadLength) {
		t.Errorf("short data: err = %v", err)
	}
	badK := append([]byte(nil), data...)
	badK[8] = 200 // k = 200 > maxHashes
	if _, err := BloomFromBytes(badK); !errors.Is(err, ErrBadHashes) {
		t.Errorf("k=200: err = %v", err)
	}
}

// TestLockNlCompound is the canonical compound: a batch insert guarded
// by an admission check, atomic under Lock + the Nl* forms.  The Nl*
// write forms carry the zero-value panic; the regular methods would
// deadlock if called under Lock (documented, not tested — it would hang).
func TestLockNlCompound(t *testing.T) {
	b := NewBloom(1000, 0.01)
	keys := make([][]byte, 100)
	rng := rand.New(rand.NewPCG(42, 7))
	for i := range keys {
		keys[i] = keyOf(rng, 8)
	}

	// Admit a batch only while the filter has room (saturation below
	// 0.9): the check and the adds are one consistent view.
	admit := func(batch [][]byte) {
		b.Lock()
		defer b.Unlock()
		if b.NlSaturation() < 0.9 {
			for _, k := range batch {
				b.NlAdd(k)
			}
		}
	}
	admit(keys[:50])
	admit(keys[50:])
	for i, k := range keys {
		if !b.MayContain(k) {
			t.Errorf("key %d: false negative after the compound admit", i)
		}
	}
	if b.Added() != 100 {
		t.Errorf("Added = %d, want 100", b.Added())
	}

	// The Nl* zero-value panics carry the bloom_ts prefix.
	var zero Bloom
	expectPanicMsg(t, "NlAdd on zero value", func() {
		zero.Lock()
		defer zero.Unlock()
		zero.NlAdd([]byte("x"))
	}, "bloom_ts:", "NlAdd", "NewBloom")
}
