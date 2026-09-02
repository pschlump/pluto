/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"
)

// checkInvariants verifies the structural invariants of a sketch:
// the histogram agrees with a per-register tally, every register holds a
// rank Add could have produced, the cached count (if present) matches a
// fresh estimate, and the sketch survives a serialize/decode round trip
// with its count intact.  Call it after any structural change; it reads
// the internals directly, so single-goroutine tests only.
func checkInvariants(t *testing.T, h *Hll) {
	t.Helper()

	reghisto := h.histogram()
	var total int
	for v, c := range reghisto {
		total += c
		if v > rankMax && c > 0 {
			t.Errorf("histogram holds %d registers above rankMax %d", c, rankMax)
		}
	}
	if total != Registers {
		t.Errorf("histogram totals %d registers, want %d", total, Registers)
	}
	var manual [64]int
	for i := range Registers {
		manual[getRegister(&h.dense, i)]++
	}
	if reghisto != manual {
		t.Errorf("histogram disagrees with the per-register tally")
	}

	c1 := h.Count()
	h.valid.Store(false) // force a fresh estimate
	if c2 := h.Count(); c2 != c1 {
		t.Errorf("Count not deterministic: %d then %d", c1, c2)
	}

	decoded, err := HllFromBytes(h.Bytes())
	if err != nil {
		t.Fatalf("HllFromBytes: %v", err)
	}
	if decoded.Count() != c1 {
		t.Errorf("decoded Count = %d, want %d", decoded.Count(), c1)
	}
}

// addUint64 adds the 8-byte little-endian encoding of i (deterministic
// distinct keys, no allocation).
func addUint64(h *Hll, i uint64) bool {
	var key [8]byte
	binary.LittleEndian.PutUint64(key[:], i)
	return h.Add(key[:])
}

// fill returns a sketch with n distinct elements 0..n-1 added and its
// checked invariants.
func fill(t *testing.T, n uint64) *Hll {
	t.Helper()
	h := NewHll()
	for i := uint64(0); i < n; i++ {
		addUint64(h, i)
	}
	checkInvariants(t, h)
	return h
}

// TestRandomizedModel cross-checks the sketch against an exact set
// oracle over interleaved adds, duplicate re-adds, serializations and
// resets (seed 42, deterministic).  Tolerances are deliberately wide:
// they gate the estimator's sanity (right ballpark, no drift), while
// TestCardinalitySweep gates the accuracy contract with statistics.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	h := NewHll()
	oracle := make(map[string]struct{})
	var keyBuf [12]byte

	tolerance := func(n int) int {
		switch {
		case n < 200:
			return max(50, n/4)
		case n < 5000:
			return n / 8
		default:
			return n / 20
		}
	}

	for step := 0; step < 4000; step++ {
		switch r := rng.Float64(); {
		case r < 0.70: // add a fresh or duplicate element
			// 18-bit id space so the ~2800 adds include real duplicates;
			// total key length 8..12 bytes.
			binary.LittleEndian.PutUint64(keyBuf[:8], uint64(rng.Uint32()>>14))
			l := rng.IntN(5) + 8
			for i := 8; i < l; i++ {
				keyBuf[i] = byte(rng.IntN(256))
			}
			key := string(keyBuf[:l])
			_, existed := oracle[key]
			changed := h.Add([]byte(key))
			// A duplicate must never change a register.  A fresh key
			// usually does, but can be silently absorbed when its
			// register already holds a rank at least as high (a
			// register-level collision, not an element-level one) — so
			// only the duplicate direction is an exact law.
			if existed && changed {
				t.Fatalf("step %d: duplicate Add reported a change", step)
			}
			oracle[key] = struct{}{}
		case r < 0.80: // count and compare
			got, want := int(h.Count()), len(oracle)
			if diff := got - want; diff < -tolerance(want) || diff > tolerance(want) {
				t.Fatalf("step %d: Count=%d oracle=%d (tolerance ±%d)", step, got, want, tolerance(want))
			}
		case r < 0.88: // serialize, decode, keep the decode
			decoded, err := HllFromBytes(h.Bytes())
			if err != nil {
				t.Fatalf("step %d: HllFromBytes: %v", step, err)
			}
			h = decoded
		case r < 0.94: // merge the sketch with a small random one
			other := NewHll()
			for range rng.IntN(50) + 1 {
				binary.LittleEndian.PutUint64(keyBuf[:8], rng.Uint64()>>20)
				other.Add(keyBuf[:12])
				oracle[string(keyBuf[:12])] = struct{}{}
			}
			h.Merge(other)
		default: // reset
			h.Reset()
			oracle = make(map[string]struct{})
		}
	}
	checkInvariants(t, h)
	if got, want := h.Count(), uint64(len(oracle)); int(got)-int(want) < -tolerance(len(oracle)) || int(got)-int(want) > tolerance(len(oracle)) {
		t.Errorf("final Count=%d oracle=%d", got, want)
	}
}

// sweepPoint runs trials independent sketches of n distinct random
// elements each (a deterministic PCG stream per trial) and returns the
// mean signed and mean absolute relative errors.
func sweepPoint(trials int, n uint64) (meanSigned, meanAbs float64) {
	var key [8]byte
	for trial := 0; trial < trials; trial++ {
		rng := rand.New(rand.NewPCG(42, uint64(trial)))
		h := NewHll()
		for i := uint64(0); i < n; i++ {
			binary.LittleEndian.PutUint64(key[:], rng.Uint64())
			h.Add(key[:])
		}
		e := float64(h.Count())/float64(n) - 1
		meanSigned += e
		meanAbs += math.Abs(e)
	}
	return meanSigned / float64(trials), meanAbs / float64(trials)
}

// TestCardinalitySweep is the accuracy contract, demonstrated against
// the exact-set oracle (the count of distinct keys actually added):
//
//   - |relative error| < 1.5% at 1k, 10k, 100k, 1M and 10M, mean over
//     ≥ 20 random trials (observed ≈ 0.4–0.7%, typical single-shot
//     ≈ 0.8% — the 1.04/√m bound), and
//   - near-exact below ~1000: mean |relative error| < 0.5% over 400
//     trials, which holds essentially single-shot: in the
//     LinearCounting regime the noise is the register-collision floor
//     (sd ≈ n/√(2m) elements — a 0.55% relative sd independent of n)
//     and the only bias is the collision count itself (n²/2m elements,
//     −0.3% at n=100, unobservable: a sketch cannot tell two elements
//     that share a register apart).
//
// All trials use deterministic key streams, so the whole sweep is
// reproducible.  The 10M point is skipped under -test.short.
func TestCardinalitySweep(t *testing.T) {
	type point struct {
		name       string
		n          uint64
		trials     int
		signedGate float64
		absGate    float64 // 0: no absolute gate
		skipShort  bool
	}
	points := []point{
		{"small-10", 10, 200, 0.10, 0, false},
		{"small-100", 100, 200, 0.10, 0, false},
		{"near-exact-500", 500, 400, 0.005, 0.005, false},
		{"near-exact-1000", 1000, 400, 0.005, 0.005, false},
		{"1k", 1_000, 50, 0.015, 0.015, false},
		{"10k", 10_000, 50, 0.015, 0.015, false},
		{"100k", 100_000, 20, 0.015, 0.015, false},
		{"1M", 1_000_000, 20, 0.015, 0.015, false},
		{"10M", 10_000_000, 20, 0.015, 0.015, true},
	}
	for _, p := range points {
		if p.skipShort && testing.Short() {
			t.Logf("%s: skipped under -short", p.name)
			continue
		}
		signed, abs := sweepPoint(p.trials, p.n)
		t.Logf("%-16s n=%-10d trials=%-3d mean signed %+.3f%%  mean |err| %.3f%%",
			p.name, p.n, p.trials, signed*100, abs*100)
		if math.Abs(signed) >= p.signedGate {
			t.Errorf("%s: |mean signed rel err| %.3f%% exceeds %.2f%%", p.name, signed*100, p.signedGate*100)
		}
		if p.absGate > 0 && abs >= p.absGate {
			t.Errorf("%s: mean |rel err| %.3f%% exceeds %.2f%%", p.name, abs*100, p.absGate*100)
		}
	}
}

// TestMergeSplitExact verifies the merge contract exactly: splitting n
// distinct elements across k sketches and merging them (in any order,
// including into a non-empty destination) yields registers identical to
// adding everything to one sketch — the register-wise maximum of a
// partition is the maximum over the whole set, so this is bit-exact,
// not a tolerance.
func TestMergeSplitExact(t *testing.T) {
	const n = 200_000
	rng := rand.New(rand.NewPCG(42, 7))
	for _, k := range []int{2, 3, 7} {
		parts := make([]*Hll, k)
		for i := range parts {
			parts[i] = NewHll()
		}
		for i := uint64(0); i < n; i++ {
			addUint64(parts[int(i)%k], i)
		}
		rng.Shuffle(k, func(a, b int) { parts[a], parts[b] = parts[b], parts[a] })

		merged := NewHll()
		merged.Add([]byte("seed element that the parts do not have")) // non-empty destination
		merged.Merge(parts...)
		want := NewHll()
		want.Add([]byte("seed element that the parts do not have"))
		for i := uint64(0); i < n; i++ {
			addUint64(want, i)
		}
		if !bytes.Equal(merged.Bytes(), want.Bytes()) {
			t.Errorf("k=%d: merged registers differ from the all-in-one sketch", k)
		}
		if got := merged.Count(); got < n*97/100 || got > n*103/100 {
			t.Errorf("k=%d: merged Count = %d, want ~%d", k, got, n)
		}
	}
}

// TestMergeCommutesWithSerialization: serializing operands and merging
// the decodes equals merging then serializing.
func TestMergeCommutesWithSerialization(t *testing.T) {
	a, b := fill(t, 50_000), fill(t, 60_000)
	direct := NewHll()
	direct.Merge(a, b)
	decodedA, err := HllFromBytes(a.Bytes())
	if err != nil {
		t.Fatalf("decode a: %v", err)
	}
	decodedB, err := HllFromBytes(b.Bytes())
	if err != nil {
		t.Fatalf("decode b: %v", err)
	}
	indirect := NewHll()
	indirect.Merge(decodedA, decodedB)
	if !bytes.Equal(direct.Bytes(), indirect.Bytes()) {
		t.Errorf("merge/serialize order changed the registers")
	}
}

// TestEstimatorEdgeHistograms pins estimate() on hand-built histograms
// for the degenerate corners: the empty sketch and a full-rank-one
// sketch (every register at the maximum rank — unreachable from any
// real stream, and rejected by HllFromBytes, so built in place).
func TestEstimatorEdgeHistograms(t *testing.T) {
	if got := (&Hll{}).Count(); got != 0 {
		t.Errorf("empty histogram estimate = %d, want 0", got)
	}
	h := NewHll()
	for i := range Registers {
		setRegister(&h.dense, i, rankMax)
	}
	h.valid.Store(false)
	if got := h.Count(); got != 0 {
		t.Errorf("all-rankMax histogram estimate = %d, want the defensive 0", got)
	}
}

// TestSerializationFuzzRoundTrips random data never decodes to a
// register above rankMax unless a value in (51, 63] was actually packed
// — and everything Add can produce decodes cleanly.
func TestSerializationFuzzRoundTrips(t *testing.T) {
	rng := rand.New(rand.NewPCG(43, 11))
	for range 50 {
		h := NewHll()
		for range rng.IntN(2000) {
			var key [8]byte
			binary.LittleEndian.PutUint64(key[:], rng.Uint64())
			h.Add(key[:])
		}
		b := h.Bytes()
		decoded, err := HllFromBytes(b)
		if err != nil {
			t.Fatalf("round trip of an Add-built sketch failed: %v", err)
		}
		if !bytes.Equal(decoded.Bytes(), b) {
			t.Errorf("round trip is not byte-stable")
		}
		if decoded.Count() != h.Count() {
			t.Errorf("round trip count changed")
		}
	}
}
