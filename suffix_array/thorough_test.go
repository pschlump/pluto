package suffix_array

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// bruteLCP is the obviously correct longest common prefix of a and b.
func bruteLCP(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// checkInvariants verifies the structural invariants of the suffix
// array of s: suffixes is a permutation of 0..n-1, the sorted order is
// correct (each suffix strictly less than the next), lcp matches the
// brute-force LCP of adjacent sorted suffixes (with lcp[0] == 0), and
// Rank and Index are mutual inverses.  Call it after any structural
// change — for this immutable structure that means right after
// construction.
func checkInvariants(t *testing.T, sa *SuffixArray, s string) {
	t.Helper()
	n := len(s)

	if sa.Len() != n {
		t.Fatalf("Len()=%d, expected %d", sa.Len(), n)
	}
	if len(sa.suffixes) != n || len(sa.lcp) != n || len(sa.rank) != n {
		t.Fatalf("table lengths %d/%d/%d, expected %d each",
			len(sa.suffixes), len(sa.lcp), len(sa.rank), n)
	}

	// suffixes must be a permutation of 0..n-1.
	seen := make([]bool, n)
	for i := 0; i < n; i++ {
		off := sa.suffixes[i]
		if off < 0 || off >= n || seen[off] {
			t.Fatalf("suffixes is not a permutation of 0..%d: suffixes[%d]=%d", n-1, i, off)
		}
		seen[off] = true
	}

	// The sorted order must be correct and lcp must match brute force.
	if n > 0 && sa.lcp[0] != 0 {
		t.Fatalf("lcp[0]=%d, must be 0 by definition", sa.lcp[0])
	}
	for i := 1; i < n; i++ {
		prev := s[sa.suffixes[i-1]:]
		cur := s[sa.suffixes[i]:]
		if strings.Compare(prev, cur) >= 0 {
			t.Fatalf("sorted order broken at %d: %q >= %q", i, prev, cur)
		}
		if got, want := sa.lcp[i], bruteLCP(prev, cur); got != want {
			t.Fatalf("lcp[%d]=%d, brute force says %d (%q vs %q)", i, got, want, prev, cur)
		}
	}

	// Rank and Index must be mutual inverses, and the public accessors
	// must agree with the internal tables.
	for i := 0; i < n; i++ {
		r, ok := sa.Rank(i)
		if !ok {
			t.Fatalf("Rank(%d) returned ok=false for an in-range suffix", i)
		}
		if r != sa.rank[i] || sa.suffixes[r] != i {
			t.Fatalf("Rank(%d)=%d but suffixes[%d]=%d", i, r, r, sa.suffixes[r])
		}
		off, ok := sa.Index(r)
		if !ok || off != i {
			t.Fatalf("Index(Rank(%d))=(%d, %v), expected (%d, true)", i, off, ok, i)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// naiveSuffixArray is the reference model: collect every suffix, sort
// them with strings.Compare on the substrings, and compute the LCP of
// each adjacent pair by brute force.  Quadratic and obviously correct —
// the property under test is that the prefix-doubling construction
// computes exactly the same tables.
type naiveSuffixArray struct {
	suffixes []int
	lcp      []int
}

func newNaiveSuffixArray(s string) *naiveSuffixArray {
	n := len(s)
	m := &naiveSuffixArray{suffixes: make([]int, n), lcp: make([]int, n)}
	for i := range n {
		m.suffixes[i] = i
	}
	sort.Slice(m.suffixes, func(x, y int) bool {
		return strings.Compare(s[m.suffixes[x]:], s[m.suffixes[y]:]) < 0
	})
	for i := 1; i < n; i++ {
		m.lcp[i] = bruteLCP(s[m.suffixes[i-1]:], s[m.suffixes[i]:])
	}
	return m
}

// lrs is the reference longest repeated substring: the maximum of the
// LCP table, ties broken by first in sorted order.
func (m *naiveSuffixArray) lrs(s string) string {
	best := ""
	for i := 1; i < len(m.lcp); i++ {
		if m.lcp[i] > len(best) {
			best = s[m.suffixes[i] : m.suffixes[i]+m.lcp[i]]
		}
	}
	return best
}

func TestSuffixArrayRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	// A 3-letter alphabet with lengths up to 200 forces long repeats —
	// the worst case for prefix doubling.
	const alphabet = "abc"
	for step := range 800 {
		n := rng.Intn(201)
		var sb strings.Builder
		for range n {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		s := sb.String()

		sa := NewSuffixArray(s)
		model := newNaiveSuffixArray(s)

		if sa.Len() != n {
			t.Fatalf("step %d: Len()=%d, expected %d", step, sa.Len(), n)
		}
		for i := 0; i < n; i++ {
			if off, ok := sa.Index(i); !ok || off != model.suffixes[i] {
				t.Fatalf("step %d: Index(%d)=(%d, %v), model says %d", step, i, off, ok, model.suffixes[i])
			}
			if suf, ok := sa.Select(i); !ok || suf != s[model.suffixes[i]:] {
				t.Fatalf("step %d: Select(%d)=%q, model says %q", step, i, suf, s[model.suffixes[i]:])
			}
			if lcp, ok := sa.LCP(i); !ok || lcp != model.lcp[i] {
				t.Fatalf("step %d: LCP(%d)=(%d, %v), model says %d", step, i, lcp, ok, model.lcp[i])
			}
			if r, ok := sa.Rank(i); !ok || model.suffixes[r] != i {
				t.Fatalf("step %d: Rank(%d)=(%d, %v), model's sorted position of suffix %d disagrees",
					step, i, r, ok, i)
			}
		}
		if got, want := sa.LongestRepeatedSubstring(), model.lrs(s); got != want {
			t.Fatalf("step %d: LongestRepeatedSubstring()=%q, model says %q", step, got, want)
		}
		if step%37 == 0 {
			checkInvariants(t, sa, s)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Edge cases
// -------------------------------------------------------------------------------------------------------

func TestEmptyString(t *testing.T) {
	sa := NewSuffixArray("")
	if sa.Len() != 0 {
		t.Errorf("Expected Len()=0 for the empty string, got %d", sa.Len())
	}
	if _, ok := sa.Index(0); ok {
		t.Errorf("Expected Index(0) on the empty string to return ok=false.")
	}
	if _, ok := sa.Select(0); ok {
		t.Errorf("Expected Select(0) on the empty string to return ok=false.")
	}
	if _, ok := sa.LCP(0); ok {
		t.Errorf("Expected LCP(0) on the empty string to return ok=false.")
	}
	if _, ok := sa.Rank(0); ok {
		t.Errorf("Expected Rank(0) on the empty string to return ok=false.")
	}
	if got := sa.LongestRepeatedSubstring(); got != "" {
		t.Errorf("Expected LongestRepeatedSubstring()=\"\" for the empty string, got %q", got)
	}
	checkInvariants(t, sa, "")
}

func TestSingleChar(t *testing.T) {
	sa := NewSuffixArray("a")
	if sa.Len() != 1 {
		t.Fatalf("Expected Len()=1, got %d", sa.Len())
	}
	if off, ok := sa.Index(0); !ok || off != 0 {
		t.Errorf("Expected Index(0)=(0, true), got (%d, %v)", off, ok)
	}
	if suf, ok := sa.Select(0); !ok || suf != "a" {
		t.Errorf("Expected Select(0)=(\"a\", true), got (%q, %v)", suf, ok)
	}
	if lcp, ok := sa.LCP(0); !ok || lcp != 0 {
		t.Errorf("Expected LCP(0)=(0, true), got (%d, %v)", lcp, ok)
	}
	if r, ok := sa.Rank(0); !ok || r != 0 {
		t.Errorf("Expected Rank(0)=(0, true), got (%d, %v)", r, ok)
	}
	if got := sa.LongestRepeatedSubstring(); got != "" {
		t.Errorf("Expected LongestRepeatedSubstring()=\"\" for a single char, got %q", got)
	}
	checkInvariants(t, sa, "a")
}

// TestAllSameChar exercises the worst case for the LCP table: "aaaaa"
// sorts its suffixes shortest-first and every adjacent pair shares the
// shorter one entirely.
func TestAllSameChar(t *testing.T) {
	s := "aaaaa"
	sa := NewSuffixArray(s)

	wantIndex := []int{4, 3, 2, 1, 0}
	wantLCP := []int{0, 1, 2, 3, 4}
	wantRank := []int{4, 3, 2, 1, 0}
	for i := range 5 {
		if off, ok := sa.Index(i); !ok || off != wantIndex[i] {
			t.Errorf("Expected Index(%d)=(%d, true), got (%d, %v)", i, wantIndex[i], off, ok)
		}
		if lcp, ok := sa.LCP(i); !ok || lcp != wantLCP[i] {
			t.Errorf("Expected LCP(%d)=(%d, true), got (%d, %v)", i, wantLCP[i], lcp, ok)
		}
		if r, ok := sa.Rank(i); !ok || r != wantRank[i] {
			t.Errorf("Expected Rank(%d)=(%d, true), got (%d, %v)", i, wantRank[i], r, ok)
		}
	}
	if got := sa.LongestRepeatedSubstring(); got != "aaaa" {
		t.Errorf("Expected LongestRepeatedSubstring()=\"aaaa\", got %q", got)
	}
	checkInvariants(t, sa, s)
}

// TestLongestRepeatedSubstringKnown checks the LRS client on inputs
// with verified answers.
func TestLongestRepeatedSubstringKnown(t *testing.T) {
	cases := []struct{ in, want string }{
		{"aacaagtttacaagc", "acaag"}, // the algs4 §6.3 LRS example
		{"mississippi", "issi"},
		{"banana", "ana"},
		{"aaaaa", "aaaa"},
		{"abcd", ""},
		{"a", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := NewSuffixArray(c.in).LongestRepeatedSubstring(); got != c.want {
			t.Errorf("LongestRepeatedSubstring(%q)=%q, expected %q", c.in, got, c.want)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Concurrency smoke test
// -------------------------------------------------------------------------------------------------------

// TestConcurrentReads hammers one immutable SuffixArray from many
// goroutines and checks that every reader sees the same answers the
// single-threaded computation produced.  Run under -race: immutability
// is the package's concurrency guarantee.
func TestConcurrentReads(t *testing.T) {
	s := "aacaagtttacaagc"
	sa := NewSuffixArray(s)
	model := newNaiveSuffixArray(s)
	wantLRS := model.lrs(s)

	const readers = 8
	const rounds = 100
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				if sa.Len() != len(s) {
					t.Errorf("Len()=%d, expected %d", sa.Len(), len(s))
					return
				}
				for i := 0; i < sa.Len(); i++ {
					if off, ok := sa.Index(i); !ok || off != model.suffixes[i] {
						t.Errorf("Index(%d)=(%d, %v), expected %d", i, off, ok, model.suffixes[i])
						return
					}
					if suf, ok := sa.Select(i); !ok || suf != s[model.suffixes[i]:] {
						t.Errorf("Select(%d)=%q, expected %q", i, suf, s[model.suffixes[i]:])
						return
					}
					if lcp, ok := sa.LCP(i); !ok || lcp != model.lcp[i] {
						t.Errorf("LCP(%d)=(%d, %v), expected %d", i, lcp, ok, model.lcp[i])
						return
					}
					if r, ok := sa.Rank(i); !ok || model.suffixes[r] != i {
						t.Errorf("Rank(%d)=(%d, %v), unexpected", i, r, ok)
						return
					}
				}
				if got := sa.LongestRepeatedSubstring(); got != wantLRS {
					t.Errorf("LongestRepeatedSubstring()=%q, expected %q", got, wantLRS)
					return
				}
			}
		}()
	}
	wg.Wait()
}
