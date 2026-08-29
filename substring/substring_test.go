package substring

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Table tests: one table exercised against all three searchers
// -------------------------------------------------------------------------------------------------------

// searchers bundles one freshly-constructed searcher of each algorithm
// for the same pattern.
type searchers struct {
	name string
	idx  int // Search(pattern-in-text) result
}

// allSearches runs every algorithm's Search for pattern against text
// and returns the results labeled by algorithm name.
func allSearches(pattern, text string) []searchers {
	k := NewKMP(pattern)
	b := NewBoyerMoore(pattern)
	r := NewRabinKarp(pattern)
	return []searchers{
		{"KMP", k.Search(text)},
		{"BoyerMoore", b.Search(text)},
		{"RabinKarp", r.Search(text)},
	}
}

// TestSearchTable checks the shared semantics — leftmost occurrence or
// -1, empty pattern matches at 0, binary bytes work — against expected
// values verified by hand and by strings.Index.
func TestSearchTable(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		text    string
		want    int
	}{
		{"empty pattern, empty text", "", "", 0},
		{"empty pattern, nonempty text", "", "abc", 0},
		{"empty text", "a", "", -1},
		{"pattern at start", "abacad", "abacadabrabracabracadabrabrabracad", 0},
		{"pattern in middle", "rab", "abacadabrabracabracadabrabrabracad", 8},
		{"repeated pattern, leftmost wins", "bracad", "abacadabrabracabracadabrabrabracad", 15},
		{"long self-overlapping pattern", "rabrabracad", "abacadabrabracabracadabrabrabracad", 23},
		{"pattern absent (algs4 trace miss)", "bcara", "abacadabrabracabracadabrabrabracad", -1},
		{"pattern absent", "zzz", "abacadabrabracabracadabrabrabracad", -1},
		{"pattern equals text", "abracadabra", "abracadabra", 0},
		{"pattern longer than text", "abracadabrax", "abracadabra", -1},
		{"overlapping occurrences", "AA", "AAAAA", 0}, // leftmost, not 1
		{"all same byte, hit", "aaa", "aaaaaaaaaa", 0},
		{"all same byte, miss", "aaa", "bbb", -1},
		{"almost match at end", "aab", "aaaaaaaaaaaaaaab", 13},
		{"self-overlapping pattern", "abab", "abababab", 0},
		{"single byte hit", "c", "abcabc", 2},
		{"single byte miss", "c", "abab", -1},
		{"binary zero bytes", "\x00\x00", "ab\x00\x00cd", 2},
		{"binary 0xff bytes", "\xff\xfe", "a\xff\xfeb", 1},
		{"binary miss", "\xff\xff", "a\xff\xfe\xffb", -1},
		{"utf8 text searched as bytes", "世界", "hello 世界 peace", 6},
	}
	for _, c := range cases {
		// The oracle must agree with the hand-written expectation.
		if oracle := strings.Index(c.text, c.pattern); oracle != c.want {
			t.Fatalf("%s: test itself is wrong: strings.Index=%d, want=%d", c.name, oracle, c.want)
		}
		for _, s := range allSearches(c.pattern, c.text) {
			if s.idx != c.want {
				t.Errorf("%s: %s.Search(%q in %q)=%d, expected %d", c.name, s.name, c.pattern, c.text, s.idx, c.want)
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value tolerance (the package never panics)
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil searcher of each type reports
// -1 from Search and "" from Pattern.
func TestNilTolerated(t *testing.T) {
	var k *KMP
	var b *BoyerMoore
	var r *RabinKarp

	if got := k.Search("abc"); got != -1 {
		t.Errorf("Expected nil KMP Search to return -1, got %d", got)
	}
	if got := b.Search("abc"); got != -1 {
		t.Errorf("Expected nil BoyerMoore Search to return -1, got %d", got)
	}
	if got := r.Search("abc"); got != -1 {
		t.Errorf("Expected nil RabinKarp Search to return -1, got %d", got)
	}
	if got := k.Pattern(); got != "" {
		t.Errorf("Expected nil KMP Pattern to return \"\", got %q", got)
	}
	if got := b.Pattern(); got != "" {
		t.Errorf("Expected nil BoyerMoore Pattern to return \"\", got %q", got)
	}
	if got := r.Pattern(); got != "" {
		t.Errorf("Expected nil RabinKarp Pattern to return \"\", got %q", got)
	}
}

// TestZeroValue verifies that the zero value of each searcher behaves
// as the searcher for the empty pattern: Search returns 0 for every
// text, Pattern returns "".
func TestZeroValue(t *testing.T) {
	var k KMP
	var b BoyerMoore
	var r RabinKarp

	for _, text := range []string{"", "abc", "\x00\xff"} {
		if got := k.Search(text); got != 0 {
			t.Errorf("Expected zero-value KMP Search(%q) to return 0, got %d", text, got)
		}
		if got := b.Search(text); got != 0 {
			t.Errorf("Expected zero-value BoyerMoore Search(%q) to return 0, got %d", text, got)
		}
		if got := r.Search(text); got != 0 {
			t.Errorf("Expected zero-value RabinKarp Search(%q) to return 0, got %d", text, got)
		}
	}
	if k.Pattern() != "" || b.Pattern() != "" || r.Pattern() != "" {
		t.Errorf("Expected zero-value Pattern() to return \"\".")
	}
}

// -------------------------------------------------------------------------------------------------------
// Pattern accessor
// -------------------------------------------------------------------------------------------------------

func TestPattern(t *testing.T) {
	for _, p := range []string{"", "a", "abracadabra", "\x00\xff"} {
		if got := NewKMP(p).Pattern(); got != p {
			t.Errorf("Expected KMP.Pattern()=%q, got %q", p, got)
		}
		if got := NewBoyerMoore(p).Pattern(); got != p {
			t.Errorf("Expected BoyerMoore.Pattern()=%q, got %q", p, got)
		}
		if got := NewRabinKarp(p).Pattern(); got != p {
			t.Errorf("Expected RabinKarp.Pattern()=%q, got %q", p, got)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Search reuses one compiled searcher across many texts
// -------------------------------------------------------------------------------------------------------

// TestReuseSearcher exercises the compile-once-search-many contract:
// one searcher answers queries against several texts with the same
// answer as a freshly built one.
func TestReuseSearcher(t *testing.T) {
	pattern := "needle"
	k := NewKMP(pattern)
	texts := []string{
		"a needle in a haystack",
		"no match here",
		"needl",
		"needleneedle",
		"",
	}
	for _, text := range texts {
		want := strings.Index(text, pattern)
		if got := k.Search(text); got != want {
			t.Errorf("KMP.Search(%q)=%d, expected %d", text, got, want)
		}
	}
	// And again: nothing may have been mutated by the first pass.
	for _, text := range texts {
		want := strings.Index(text, pattern)
		if got := k.Search(text); got != want {
			t.Errorf("second pass: KMP.Search(%q)=%d, expected %d", text, got, want)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks (text of length ~100KB)
// -------------------------------------------------------------------------------------------------------

// benchmarkText returns a deterministic pseudo-random text of length n
// over a 3-letter alphabet — the small alphabet forces many partial
// matches, a hard case for every searcher.
func benchmarkText(n int) string {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic text
	const alphabet = "abc"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}

// benchmarkPattern is sliced out of the middle of the benchmark text,
// so every search is a hit at a known position.
func benchmarkPattern() (text, pattern string) {
	text = benchmarkText(100 * 1024)
	return text, text[50000:50024]
}

func BenchmarkKMPSearch(b *testing.B) {
	text, pattern := benchmarkPattern()
	k := NewKMP(pattern)
	b.ResetTimer()
	for range b.N {
		k.Search(text)
	}
}

func BenchmarkBoyerMooreSearch(b *testing.B) {
	text, pattern := benchmarkPattern()
	bm := NewBoyerMoore(pattern)
	b.ResetTimer()
	for range b.N {
		bm.Search(text)
	}
}

func BenchmarkRabinKarpSearch(b *testing.B) {
	text, pattern := benchmarkPattern()
	rk := NewRabinKarp(pattern)
	b.ResetTimer()
	for range b.N {
		rk.Search(text)
	}
}
