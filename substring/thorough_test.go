package substring

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Randomized property test against the strings.Index oracle (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestSearchRandomizedModel cross-checks all three searchers against
// strings.Index — an independent, correct reference — on random texts
// over a 3-letter alphabet (which forces many partial matches) with
// random patterns, half of them sliced out of the text itself
// (guaranteed hits at a known planted position) and half fresh
// (mostly misses).  All three algorithms must agree with the oracle,
// and hence with each other, on every single case.
func TestSearchRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const alphabet = "ABC"
	for step := range 4000 {
		n := rng.Intn(501) // text length 0..500
		var sb strings.Builder
		for range n {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		text := sb.String()

		var pattern string
		planted := -1 // index the pattern was sliced from, or -1 for a fresh pattern
		if n > 0 && rng.Intn(2) == 0 {
			// Planted pattern: a slice of the text, length 1..12 clipped
			// to what remains — a guaranteed hit at index `planted`.
			m := 1 + rng.Intn(12)
			planted = rng.Intn(n)
			if planted+m > n {
				m = n - planted
			}
			pattern = text[planted : planted+m]
		} else {
			// Fresh pattern, length 0..12 — usually a miss.
			m := rng.Intn(13)
			var pb strings.Builder
			for range m {
				pb.WriteByte(alphabet[rng.Intn(len(alphabet))])
			}
			pattern = pb.String()
		}

		want := strings.Index(text, pattern)
		for _, s := range allSearches(pattern, text) {
			got := s.idx
			if got != want {
				t.Fatalf("step %d: %s.Search(pattern %q, text %q)=%d, strings.Index says %d",
					step, s.name, pattern, text, got, want)
			}
			if planted >= 0 {
				// A planted hit must be found at or before the planted
				// index, and the bytes there must be the pattern.
				if got > planted {
					t.Fatalf("step %d: %s returned %d but the pattern was planted at %d (pattern %q)",
						step, s.name, got, planted, pattern)
				}
				if got >= 0 && text[got:got+len(pattern)] != pattern {
					t.Fatalf("step %d: %s returned %d but text[%d:%d]!=%q",
						step, s.name, got, got, got+len(pattern), pattern)
				}
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Binary-alphabet stress: worst case for KMP and Rabin-Karp rolling
// -------------------------------------------------------------------------------------------------------

// TestSearchRandomizedBinary runs the same cross-check over a 2-letter
// alphabet with longer texts, where runs of one byte create the
// maximum number of hash collisions and DFA restarts.
func TestSearchRandomizedBinary(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const alphabet = "\x00\xff"
	for step := range 1000 {
		n := rng.Intn(501)
		var sb strings.Builder
		for range n {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		text := sb.String()

		m := rng.Intn(13)
		var pb strings.Builder
		for range m {
			pb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		pattern := pb.String()

		want := strings.Index(text, pattern)
		for _, s := range allSearches(pattern, text) {
			if s.idx != want {
				t.Fatalf("step %d: %s.Search=%d, strings.Index says %d (pattern %q, text %q)",
					step, s.name, s.idx, want, pattern, text)
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Concurrency smoke test
// -------------------------------------------------------------------------------------------------------

// TestConcurrentReads hammers one immutable searcher of each kind from
// many goroutines and checks that every reader sees the same answer
// the single-threaded oracle produced.  Run under -race: immutability
// is the package's concurrency guarantee.
func TestConcurrentReads(t *testing.T) {
	text := benchmarkText(4096)
	pattern := "abcabca"
	k := NewKMP(pattern)
	b := NewBoyerMoore(pattern)
	r := NewRabinKarp(pattern)
	want := strings.Index(text, pattern)

	const readers = 8
	const rounds = 100
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				if got := k.Search(text); got != want {
					t.Errorf("KMP.Search=%d, expected %d", got, want)
					return
				}
				if got := b.Search(text); got != want {
					t.Errorf("BoyerMoore.Search=%d, expected %d", got, want)
					return
				}
				if got := r.Search(text); got != want {
					t.Errorf("RabinKarp.Search=%d, expected %d", got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
}
