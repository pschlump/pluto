/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package substring implements the substring-search algorithms of
// Sedgwick & Wayne, Algorithms, 4th ed. §5.3: Knuth-Morris-Pratt
// (DFA simulation), Boyer-Moore (bad-character rule), and Rabin-Karp
// (rolling hash with verification).
//
// Each algorithm is a constructor-style query object: the pattern is
// compiled once by the constructor (NewKMP, NewBoyerMoore,
// NewRabinKarp) and the resulting immutable searcher then answers
// Search(text) queries against any number of texts.  All three share
// the same semantics:
//
//	Search returns the index of the first (leftmost) occurrence of the
//	pattern in the text, or -1 if there is none (like strings.Index,
//	which returns -1 rather than algs4's n).
//
//	Everything is byte-oriented: texts and patterns are arbitrary
//	binary strings, not necessarily UTF-8.
//
//	An empty pattern matches at index 0 of any text, including the
//	empty text (like strings.Index; algs4's KMP instead crashes on the
//	empty pattern).  A pattern longer than the text never matches (-1).
//
// Like suffix_array the structures are non-generic: there is no
// element type T and no comparison function to supply, because the
// order is inherent in the bytes of the strings.
//
// Basic operations:
//
//	NewKMP(pattern) — build the KMP DFA.			O(mR), R = 256
//	(k) Search(text) — leftmost occurrence or -1.		O(n)
//
//	NewBoyerMoore(pattern) — build the skip table.			O(m + R)
//	(b) Search(text) — leftmost occurrence or -1.		O(nm) worst, sublinear typical
//
//	NewRabinKarp(pattern) — hash the pattern.			O(m)
//	(r) Search(text) — leftmost occurrence or -1.		O(nm) worst, O(n+m) expected
//
// where n = len(text) and m = len(pattern).
//
// Every structure is IMMUTABLE after construction: the constructor
// builds the DFA / skip table / pattern hash and nothing ever mutates
// them afterwards.
//
// The package NEVER panics.  A nil searcher reports -1 from Search and
// "" from Pattern; the zero value of each type behaves as the searcher
// for the empty pattern (Search returns 0).
//
// Because the searchers are immutable, they are SAFE FOR CONCURRENT
// USE by multiple readers, so there are no mutex-guarded _ts twins.
package substring
