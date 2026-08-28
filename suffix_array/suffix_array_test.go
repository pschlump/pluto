package suffix_array

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Constructor and full table check on a known string
// -------------------------------------------------------------------------------------------------------

// TestNewSuffixArrayBanana verifies every table of the suffix array of
// "banana" against the textbook trace (algs4 §6.3).
func TestNewSuffixArrayBanana(t *testing.T) {
	sa := NewSuffixArray("banana")
	if sa == nil {
		t.Fatalf("NewSuffixArray returned nil.")
	}
	if sa.Len() != 6 {
		t.Fatalf("Expected Len()=6, got %d", sa.Len())
	}

	wantIndex := []int{5, 3, 1, 0, 4, 2}
	wantSuffix := []string{"a", "ana", "anana", "banana", "na", "nana"}
	wantLCP := []int{0, 1, 3, 0, 0, 2}
	wantRank := []int{3, 2, 5, 1, 4, 0}
	for i := range 6 {
		if off, ok := sa.Index(i); !ok || off != wantIndex[i] {
			t.Errorf("Expected Index(%d)=(%d, true), got (%d, %v)", i, wantIndex[i], off, ok)
		}
		if suf, ok := sa.Select(i); !ok || suf != wantSuffix[i] {
			t.Errorf("Expected Select(%d)=(%q, true), got (%q, %v)", i, wantSuffix[i], suf, ok)
		}
		if lcp, ok := sa.LCP(i); !ok || lcp != wantLCP[i] {
			t.Errorf("Expected LCP(%d)=(%d, true), got (%d, %v)", i, wantLCP[i], lcp, ok)
		}
		if r, ok := sa.Rank(i); !ok || r != wantRank[i] {
			t.Errorf("Expected Rank(%d)=(%d, true), got (%d, %v)", i, wantRank[i], r, ok)
		}
	}
	if got := sa.LongestRepeatedSubstring(); got != "ana" {
		t.Errorf("Expected LongestRepeatedSubstring()=\"ana\", got %q", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Out-of-range positions report, never panic
// -------------------------------------------------------------------------------------------------------

func TestOutOfRange(t *testing.T) {
	sa := NewSuffixArray("banana")

	bad := []int{-1, -100, sa.Len(), sa.Len() + 1, 1000}
	for _, i := range bad {
		if off, ok := sa.Index(i); ok || off != 0 {
			t.Errorf("Expected Index(%d) to return (0, false), got (%d, %v)", i, off, ok)
		}
		if suf, ok := sa.Select(i); ok || suf != "" {
			t.Errorf("Expected Select(%d) to return (\"\", false), got (%q, %v)", i, suf, ok)
		}
		if lcp, ok := sa.LCP(i); ok || lcp != 0 {
			t.Errorf("Expected LCP(%d) to return (0, false), got (%d, %v)", i, lcp, ok)
		}
		if r, ok := sa.Rank(i); ok || r != 0 {
			t.Errorf("Expected Rank(%d) to return (0, false), got (%d, %v)", i, r, ok)
		}
	}

	// LCP(0) is a valid query: the first sorted suffix has no
	// predecessor, so its LCP is defined to be 0.
	if lcp, ok := sa.LCP(0); !ok || lcp != 0 {
		t.Errorf("Expected LCP(0)=(0, true), got (%d, %v)", lcp, ok)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value tolerance (the package never panics)
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil *SuffixArray behaves as an
// empty suffix array for every operation.
func TestNilTolerated(t *testing.T) {
	var nilSA *SuffixArray

	if nilSA.Len() != 0 {
		t.Errorf("Expected Len()=0 on a nil suffix array, got %d", nilSA.Len())
	}
	if off, ok := nilSA.Index(0); ok || off != 0 {
		t.Errorf("Expected Index on a nil suffix array to return (0, false).")
	}
	if suf, ok := nilSA.Select(0); ok || suf != "" {
		t.Errorf("Expected Select on a nil suffix array to return (\"\", false).")
	}
	if lcp, ok := nilSA.LCP(0); ok || lcp != 0 {
		t.Errorf("Expected LCP on a nil suffix array to return (0, false).")
	}
	if r, ok := nilSA.Rank(0); ok || r != 0 {
		t.Errorf("Expected Rank on a nil suffix array to return (0, false).")
	}
	if got := nilSA.LongestRepeatedSubstring(); got != "" {
		t.Errorf("Expected LongestRepeatedSubstring on a nil suffix array to return \"\", got %q", got)
	}
}

// TestZeroValue verifies that the zero value behaves as an empty
// suffix array for every operation.
func TestZeroValue(t *testing.T) {
	var sa SuffixArray

	if sa.Len() != 0 {
		t.Errorf("Expected Len()=0 on the zero value, got %d", sa.Len())
	}
	if _, ok := sa.Index(0); ok {
		t.Errorf("Expected Index on the zero value to return ok=false.")
	}
	if _, ok := sa.Select(0); ok {
		t.Errorf("Expected Select on the zero value to return ok=false.")
	}
	if _, ok := sa.LCP(0); ok {
		t.Errorf("Expected LCP on the zero value to return ok=false.")
	}
	if _, ok := sa.Rank(0); ok {
		t.Errorf("Expected Rank on the zero value to return ok=false.")
	}
	if got := sa.LongestRepeatedSubstring(); got != "" {
		t.Errorf("Expected LongestRepeatedSubstring on the zero value to return \"\", got %q", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Immutability
// -------------------------------------------------------------------------------------------------------

// TestImmutable verifies that no query mutates the tables: the full
// suffix/LCP/rank content is identical before and after exercising
// every method.
func TestImmutable(t *testing.T) {
	sa := NewSuffixArray("mississippi")
	snapshot := func() (idx, lcp, rank []int) {
		idx = append([]int(nil), sa.suffixes...)
		lcp = append([]int(nil), sa.lcp...)
		rank = append([]int(nil), sa.rank...)
		return
	}
	beforeI, beforeL, beforeR := snapshot()

	for i := 0; i < sa.Len(); i++ {
		sa.Index(i)
		sa.Select(i)
		sa.LCP(i)
		sa.Rank(i)
	}
	sa.LongestRepeatedSubstring()

	afterI, afterL, afterR := snapshot()
	for i := range beforeI {
		if beforeI[i] != afterI[i] || beforeL[i] != afterL[i] || beforeR[i] != afterR[i] {
			t.Fatalf("Queries mutated the tables at position %d.", i)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks (text of length ~1000)
// -------------------------------------------------------------------------------------------------------

// benchmarkText returns a deterministic pseudo-random text of length n
// over a 3-letter alphabet — the small alphabet forces long repeats,
// the worst case for the construction.
func benchmarkText(n int) string {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic text
	const alphabet = "abc"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}

func BenchmarkNewSuffixArray(b *testing.B) {
	s := benchmarkText(1000)
	b.ResetTimer()
	for range b.N {
		NewSuffixArray(s)
	}
}

func BenchmarkSelect(b *testing.B) {
	sa := NewSuffixArray(benchmarkText(1000))
	n := sa.Len()
	b.ResetTimer()
	for i := range b.N {
		sa.Select(i % n)
	}
}

func BenchmarkLongestRepeatedSubstring(b *testing.B) {
	sa := NewSuffixArray(benchmarkText(1000))
	b.ResetTimer()
	for range b.N {
		sa.LongestRepeatedSubstring()
	}
}
