/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/bits"
	"math/rand/v2"
	"testing"
)

// Shared helpers -------------------------------------------------------------

// newTestRng returns the deterministic PCG stream the suite uses (the
// house seeds: 42/7 for models, others where noted).
func newTestRng(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), 7))
}

// keyOf builds a fresh n-byte key from rng (no allocation beyond the
// slice, deterministic per stream).
func keyOf(rng *rand.Rand, n int) []byte {
	k := make([]byte, n)
	for i := range k {
		k[i] = byte(rng.IntN(256))
	}
	return k
}

func cloneBytes(b []byte) []byte { return append([]byte(nil), b...) }

func putUint64(b []byte, v uint64) { binary.LittleEndian.PutUint64(b, v) }

// checkInvariants verifies the structural invariants of a filter: the
// word count matches m, the maintained set-bit counter equals the
// popcount of the words, no bit at or above m is set, Saturation agrees
// with the counter, and the serialized form round-trips byte-for-byte
// with every counter intact.  Call it after any structural change; it
// reads internals directly, so single-goroutine tests only.
func checkInvariants(t *testing.T, b *Bloom) {
	t.Helper()
	if b == nil || b.m == 0 {
		return
	}
	if want := (b.m + 63) / 64; uint64(len(b.bits)) != want {
		t.Errorf("m=%d: %d words, want %d", b.m, len(b.bits), want)
	}
	var total uint64
	for _, w := range b.bits {
		total += uint64(bits.OnesCount64(w))
	}
	if total != b.set {
		t.Errorf("m=%d: set counter %d, popcount %d", b.m, b.set, total)
	}
	if rem := b.m & 63; rem != 0 && b.bits[len(b.bits)-1]>>rem != 0 {
		t.Errorf("m=%d: a padding bit at or above m is set", b.m)
	}
	if sat, want := b.Saturation(), float64(b.set)/float64(b.m); sat != want {
		t.Errorf("Saturation %g != %g", sat, want)
	}
	if b.set == 0 && b.Count() != 0 {
		t.Errorf("empty filter Count = %d", b.Count())
	}

	data := b.Bytes()
	decoded, err := BloomFromBytes(data)
	if err != nil {
		t.Fatalf("m=%d: FromBytes(Bytes): %v", b.m, err)
	}
	if !bytes.Equal(decoded.Bytes(), data) {
		t.Errorf("m=%d: serialize/decode is not byte-stable", b.m)
	}
	if decoded.added != b.added || decoded.m != b.m || decoded.k != b.k || decoded.set != b.set {
		t.Errorf("m=%d: decode lost a counter", b.m)
	}
}

// The model test ---------------------------------------------------------------

// TestRandomizedModel cross-checks the filter against an exact set
// oracle over interleaved adds, duplicate re-adds, membership checks on
// present and absent elements, serializations and merges (seed 42,
// deterministic).  The exact laws gated here: an added element is always
// reported present (no false negatives — structural), Add's return is
// exactly the inverse of the pre-call MayContain (so a duplicate Add
// never reports a change), Added counts every call, and the
// false-positive rate stays within a loose band of the design p while
// the oracle is within the design load.
func TestRandomizedModel(t *testing.T) {
	const designN = 2000
	const designP = 0.01
	rng := rand.New(rand.NewPCG(42, 7))
	b := NewBloom(designN, designP)
	oracle := make(map[string]struct{})
	var adds uint64

	key := func() string {
		// 16-bit id space so adds include real duplicates and absent
		// probes include real near-misses.
		k := make([]byte, 4)
		binary.LittleEndian.PutUint16(k, uint16(rng.Uint32()>>16))
		binary.LittleEndian.PutUint16(k[2:], uint16(rng.Uint32()>>16))
		return string(k)
	}

	for step := 0; step < 4000; step++ {
		switch r := rng.Float64(); {
		case r < 0.55: // add a fresh or duplicate element
			k := key()
			_, existed := oracle[k]
			before := b.MayContain([]byte(k))
			changed := b.Add([]byte(k))
			if changed == before {
				t.Fatalf("step %d: Add(%q) returned %v but prior MayContain was %v — must be the inverse", step, k, changed, before)
			}
			if existed && changed {
				t.Fatalf("step %d: duplicate Add reported a change", step)
			}
			if !b.MayContain([]byte(k)) {
				t.Fatalf("step %d: false negative on an added element", step)
			}
			oracle[k] = struct{}{}
			adds++
		case r < 0.72: // every known element reports present — exact
			for k := range oracle { // one random member per step
				if !b.MayContain([]byte(k)) {
					t.Fatalf("step %d: false negative on oracle member %q", step, k)
				}
				break
			}
		case r < 0.87: // absent elements: false positives within budget
			if len(oracle) <= designN { // beyond the design load the rate climbs by design
				const probes = 24
				hits := 0
				for range probes {
					k := key()
					if _, in := oracle[k]; in {
						continue
					}
					if b.MayContain([]byte(k)) {
						hits++
					}
				}
				// Binomial(probes, p=0.01) gate: P(≥4) ≈ 0.3%.  Four
				// is the fixed-seed budget with margin.
				if hits > 3 {
					t.Fatalf("step %d: %d/%d false positives at the design load — rate far above p", step, hits, probes)
				}
			}
		case r < 0.92: // serialize, decode, keep the decode
			decoded, err := BloomFromBytes(b.Bytes())
			if err != nil {
				t.Fatalf("step %d: FromBytes: %v", step, err)
			}
			if decoded.Added() != adds {
				t.Fatalf("step %d: decode lost Added (%d vs %d)", step, decoded.Added(), adds)
			}
			b = decoded
		case r < 0.97: // merge a small same-shape filter
			other := NewBloom(designN, designP)
			for range rng.IntN(20) + 1 {
				k := key()
				other.Add([]byte(k))
				oracle[k] = struct{}{}
				adds++
			}
			b.Merge(other)
		default: // reset
			b.Reset()
			oracle = make(map[string]struct{})
			adds = 0
		}
		if b.Added() != adds {
			t.Fatalf("step %d: Added = %d, want %d", step, b.Added(), adds)
		}
	}
	checkInvariants(t, b)
}

// The accuracy contract --------------------------------------------------------

// TestFalsePositiveRateSweep measures the false-positive rate at the
// design load — n distinct elements added to a NewBloom(n, p) filter,
// then n disjoint absent elements probed — at several (n, p) points.
// Everything is deterministic (fixed seeds, frozen hashes), so the
// observed rates are stable; the 2.5×p gate is loose against a healthy
// filter but fails any real degradation (a broken hash pair collapses
// toward 1.0, a wasted probe count pushes well past 2.5×).  The
// companion exact law — zero false negatives — is asserted at every
// point.
func TestFalsePositiveRateSweep(t *testing.T) {
	points := []struct {
		n int
		p float64
	}{
		{1_000, 0.1},
		{1_000, 0.01},
		{10_000, 0.01},
		{10_000, 0.001},
		{100_000, 0.01},
	}
	for _, pt := range points {
		rng := newTestRng(42)
		b := NewBloom(pt.n, pt.p)
		present := make([]string, pt.n)
		for i := range present {
			k := fmtKey("present-%d-", i, rng)
			present[i] = k
			b.Add([]byte(k))
		}
		for _, k := range present {
			if !b.MayContain([]byte(k)) {
				t.Fatalf("n=%d p=%g: false negative on %q", pt.n, pt.p, k)
			}
		}
		hits := 0
		for i := range pt.n {
			k := fmtKey("absent-%d-", i, rng)
			if b.MayContain([]byte(k)) {
				hits++
			}
		}
		rate := float64(hits) / float64(pt.n)
		t.Logf("n=%-7d p=%-6g m=%-8d k=%-2d saturation=%.3f observed fp=%.4f (%.1f×p)",
			pt.n, pt.p, b.BitCount(), b.HashCount(), b.Saturation(), rate, rate/pt.p)
		if rate > 2.5*pt.p {
			t.Errorf("n=%d p=%g: observed false-positive rate %.4f exceeds 2.5×p", pt.n, pt.p, rate)
		}
		est := b.Count()
		if est < uint64(float64(pt.n)*0.9) || est > uint64(float64(pt.n)*1.1) {
			t.Errorf("n=%d p=%g: Count estimate %d, want ~%d", pt.n, pt.p, est, pt.n)
		}
	}
}

// fmtKey builds a deterministic distinct key: prefix + index + random
// tail (the random tail makes same-index keys across suites disjoint).
func fmtKey(prefix string, i int, rng *rand.Rand) string {
	var buf [8]byte
	for j := range buf {
		buf[j] = byte(rng.IntN(256))
	}
	return prefix + itoa(i) + "-" + string(buf[:])
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [24]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

// TestMergeSplitExact verifies the merge contract exactly: splitting n
// distinct elements across k same-shape filters and merging them (in
// any order, including into a non-empty destination) yields a bit array
// identical to adding everything to one filter — the OR of a partition
// is the OR over the whole set, so this is bit-exact, not a tolerance.
func TestMergeSplitExact(t *testing.T) {
	const n = 100_000
	rng := newTestRng(42)
	for _, k := range []int{2, 3, 7} {
		parts := make([]*Bloom, k)
		for i := range parts {
			parts[i] = NewBloom(n, 0.001)
		}
		var key [8]byte
		for i := 0; i < n; i++ {
			putUint64(key[:], uint64(i))
			parts[i%k].Add(key[:])
		}
		rng.Shuffle(k, func(a, b int) { parts[a], parts[b] = parts[b], parts[a] })

		merged := NewBloom(n, 0.001)
		merged.Add([]byte("seed element that the parts do not have")) // non-empty destination
		merged.Merge(parts...)
		want := NewBloom(n, 0.001)
		want.Add([]byte("seed element that the parts do not have"))
		for i := 0; i < n; i++ {
			putUint64(key[:], uint64(i))
			want.Add(key[:])
		}
		if !bytes.Equal(merged.Bytes(), want.Bytes()) {
			t.Errorf("k=%d: merged bits differ from the all-in-one filter", k)
		}
		if merged.Added() != want.Added() {
			t.Errorf("k=%d: merged Added = %d, want %d (added sums across the merge)", k, merged.Added(), want.Added())
		}
		checkInvariants(t, merged)
	}
}

// The serialized form ----------------------------------------------------------

// TestSerializationFuzz round-trips random filters byte-for-byte and
// checks that mutated data reports only the three sentinel errors,
// never a panic.
func TestSerializationFuzz(t *testing.T) {
	rng := rand.New(rand.NewPCG(43, 11))
	for round := 0; round < 100; round++ {
		m := uint64(rng.IntN(5000) + 1)
		k := rng.IntN(maxHashes) + 1
		b := NewBloomBits(m, k)
		for range rng.IntN(200) {
			b.Add(keyOf(rng, 8))
		}
		checkInvariants(t, b)

		data := b.Bytes()
		decoded, err := BloomFromBytes(data)
		if err != nil {
			t.Fatalf("round %d: FromBytes: %v", round, err)
		}
		if !bytes.Equal(decoded.Bytes(), data) || decoded.Added() != b.Added() {
			t.Fatalf("round %d: round trip lost data", round)
		}

		// Random single-byte mutations: decode either succeeds or
		// reports one of the three sentinels.
		for range 8 {
			mut := cloneBytes(data)
			mut[rng.IntN(len(mut))] ^= byte(1) << rng.IntN(8)
			d, err := BloomFromBytes(mut)
			switch {
			case err == nil:
				checkInvariants(t, d)
			case isSerializedErr(err):
			default:
				t.Fatalf("round %d: mutation error %v is none of the sentinels", round, err)
			}
		}
	}
}

func isSerializedErr(err error) bool {
	return errors.Is(err, ErrBadLength) || errors.Is(err, ErrBadHashes) || errors.Is(err, ErrBadBits)
}

// The probe construction ------------------------------------------------------

// TestProbesInRange walks the internal probe construction across shapes
// (m coprime with and aligned to 64, tiny m where probes must wrap and
// repeat) and elements (the empty element exercises the degenerate-step
// guard — its SuperFastHash is 0, so the guarded step is 1 and the
// probes are consecutive).
func TestProbesInRange(t *testing.T) {
	rng := newTestRng(42)
	for _, m := range []uint64{1, 2, 3, 63, 64, 65, 101, 4096} {
		for _, k := range []int{1, 2, 7, maxHashes} {
			b := NewBloomBits(m, k)
			buf := make([]uint64, maxHashes)
			for range 100 {
				v := keyOf(rng, 1+rng.IntN(16))
				b.probes(v, buf[:k])
				for _, p := range buf[:k] {
					if p >= m {
						t.Errorf("m=%d k=%d: probe %d out of range", m, k, p)
					}
				}
			}
		}
	}
	// The empty element: step guard makes the walk consecutive.
	b := NewBloomBits(101, 5)
	var buf [maxHashes]uint64
	b.probes([]byte{}, buf[:5])
	for i := 1; i < 5; i++ {
		next := buf[i-1] + 1
		if next >= 101 {
			next = 0
		}
		if buf[i] != next {
			t.Errorf("empty-element probes are not consecutive (guard broken): %v", buf[:5])
		}
	}
}

// The second-oracle cross-check -------------------------------------------------

// refMurmur2 and refSuperFastHash are structurally different ports of
// the two hashes (index arithmetic instead of re-slicing, explicit byte
// shifts instead of binary.LittleEndian, a sign-extension helper) used
// as a second oracle against the shipping ones — the patricia_trie
// refStringMatchLen precedent.  Together with the frozen C-generated
// vectors (vectors_test.go) a bug would have to survive two independent
// derivations plus the reference itself.

func refMurmur2(v []byte) uint32 {
	const m = 0x5bd1e995
	h := hashSeed ^ uint32(len(v))
	i := 0
	for ; i+4 <= len(v); i += 4 {
		k := uint32(v[i]) | uint32(v[i+1])<<8 | uint32(v[i+2])<<16 | uint32(v[i+3])<<24
		k *= m
		k ^= k >> 24
		k *= m
		h *= m
		h ^= k
	}
	switch len(v) - i {
	case 3:
		h ^= uint32(v[i+2]) << 16
		fallthrough
	case 2:
		h ^= uint32(v[i+1]) << 8
		fallthrough
	case 1:
		h ^= uint32(v[i])
		h *= m
	}
	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}

func refSuperFastHash(v []byte) uint32 {
	if len(v) == 0 {
		return 0
	}
	sext := func(b byte) uint32 { return uint32(int32(int8(b))) } // the C (signed char) casts
	h := uint32(len(v))
	i := 0
	for ; i+4 <= len(v); i += 4 {
		h += uint32(v[i]) | uint32(v[i+1])<<8
		tmp := (uint32(v[i+2])|uint32(v[i+3])<<8)<<11 ^ h
		h = h<<16 ^ tmp
		h += h >> 11
	}
	switch len(v) & 3 {
	case 3:
		h += uint32(v[i]) | uint32(v[i+1])<<8
		h ^= h << 16
		h ^= sext(v[i+2]) << 18
		h += h >> 11
	case 2:
		h += uint32(v[i]) | uint32(v[i+1])<<8
		h ^= h << 11
		h += h >> 17
	case 1:
		h += sext(v[i])
		h ^= h << 10
		h += h >> 1
	}
	h ^= h << 3
	h += h >> 5
	h ^= h << 4
	h += h >> 17
	h ^= h << 25
	h += h >> 6
	return h
}

// TestHashOracleCrossCheck runs both hashes against the second ports
// over 20k random byte strings (every length class and byte value) plus
// the frozen vector corpus.
func TestHashOracleCrossCheck(t *testing.T) {
	rng := newTestRng(42)
	for i := 0; i < 20_000; i++ {
		v := keyOf(rng, rng.IntN(80))
		if got, want := murmur2(v), refMurmur2(v); got != want {
			t.Fatalf("murmur2(%q len %d) = %d, oracle %d", v, len(v), got, want)
		}
		if got, want := superFastHash(v), refSuperFastHash(v); got != want {
			t.Fatalf("superFastHash(%q len %d) = %d, oracle %d", v, len(v), got, want)
		}
	}
	for _, v := range hashVectors {
		if got, want := murmur2([]byte(v.in)), refMurmur2([]byte(v.in)); got != want {
			t.Errorf("vector corpus: murmur2 mismatch (%d vs %d)", got, want)
		}
		if got, want := superFastHash([]byte(v.in)), refSuperFastHash([]byte(v.in)); got != want {
			t.Errorf("vector corpus: superFastHash mismatch (%d vs %d)", got, want)
		}
	}
}

// The estimator formula -------------------------------------------------------

// TestCountFormula pins Count against the closed form recomputed from
// the internals across saturations built by real adds.
func TestCountFormula(t *testing.T) {
	rng := newTestRng(7)
	b := NewBloom(5000, 0.01)
	for range 5000 {
		b.Add(keyOf(rng, 8))
	}
	want := uint64(math.Round(-(float64(b.m) / float64(b.k)) * math.Log(1-float64(b.set)/float64(b.m))))
	if got := b.Count(); got != want {
		t.Errorf("Count = %d, formula %d", got, want)
	}
}
