/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package substring

// prime is the fixed large prime modulus of the Rabin-Karp rolling
// hash: 2³¹-1, a Mersenne prime small enough that radix*h+c never
// overflows int64.  Unlike algs4's RabinKarp, which draws a RANDOM
// 31-bit prime per construction (making runs nondeterministic), this
// package uses one fixed documented prime.
const prime = 2147483647 // 2^31 - 1

// RabinKarp is the Rabin-Karp substring searcher for one fixed pattern
// (Sedgwick's RabinKarp, §5.3): the pattern is hashed once and the
// text is scanned with a rolling hash — remove the leading byte, add
// the trailing byte, all mod prime.  This is the deterministic
// LAS VEGAS variant: on every hash hit the candidate substring is
// verified byte-for-byte, so the result is exact, never a false
// positive.  (algs4's published code is the Monte Carlo variant — no
// verification — with a random prime; see the package README.)
// It is immutable after construction; the zero value behaves as the
// searcher for the empty pattern.
type RabinKarp struct {
	pattern string // the pattern this searcher looks for
	patHash int64  // hash of the pattern
	rm      int64  // radix^(m-1) % prime, for removing the leading byte
}

// NewRabinKarp returns the Rabin-Karp searcher for pattern.  The empty
// pattern is valid input: it matches at index 0 of every text.
// Complexity is O(m), where m = len(pattern).
func NewRabinKarp(pattern string) *RabinKarp {
	m := len(pattern)
	r := &RabinKarp{pattern: pattern, rm: 1}
	// Precompute radix^(m-1) % prime for removing the leading byte.
	for i := 1; i <= m-1; i++ {
		r.rm = (radix * r.rm) % prime
	}
	r.patHash = r.hash(pattern)
	return r
}

// hash computes the rolling hash of key[0..m-1] where m = len(pattern).
func (r *RabinKarp) hash(key string) int64 {
	var h int64
	for j := 0; j < len(r.pattern); j++ {
		h = (radix*h + int64(key[j])) % prime
	}
	return h
}

// Search returns the index of the first (leftmost) occurrence of the
// pattern in text, or -1 if there is none.  The empty pattern returns
// 0; a pattern longer than text returns -1.  A nil searcher returns
// -1.
// Complexity is O(nm) in the worst case, where n = len(text) and
// m = len(pattern); the expected cost is O(n + m).
func (r *RabinKarp) Search(text string) int {
	if r == nil {
		return -1
	}
	m := len(r.pattern)
	if m == 0 {
		return 0
	}
	n := len(text)
	if n < m {
		return -1
	}
	txtHash := r.hash(text)

	// Check for a match at offset 0, then roll the hash forward one
	// byte at a time; verify byte-for-byte on every hash hit.
	if r.patHash == txtHash && r.check(text, 0) {
		return 0
	}
	for i := m; i < n; i++ {
		// Remove the leading byte, add the trailing byte.
		txtHash = (txtHash + prime - r.rm*int64(text[i-m])%prime) % prime
		txtHash = (txtHash*radix + int64(text[i])) % prime
		offset := i - m + 1
		if r.patHash == txtHash && r.check(text, offset) {
			return offset
		}
	}
	return -1 // not found
}

// check verifies that text[i:i+m] equals the pattern byte-for-byte —
// the Las Vegas half of the algorithm, so a hash collision can never
// produce a false match.
func (r *RabinKarp) check(text string, i int) bool {
	for j := 0; j < len(r.pattern); j++ {
		if r.pattern[j] != text[i+j] {
			return false
		}
	}
	return true
}

// Pattern returns the pattern the searcher was built from.  A nil
// searcher reports "".
// Complexity is O(1).
func (r *RabinKarp) Pattern() string {
	if r == nil {
		return ""
	}
	return r.pattern
}
