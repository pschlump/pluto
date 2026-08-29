/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package substring

// BoyerMoore is the Boyer-Moore substring searcher for one fixed
// pattern (Sedgwick's BoyerMoore, §5.3): the mismatched-character
// (bad-character) heuristic only — right[c] is the position of the
// rightmost occurrence of byte c in the pattern, or -1 — WITHOUT the
// strong good-suffix rule, matching the algs4 reference.  It is
// immutable after construction; the zero value behaves as the searcher
// for the empty pattern.
type BoyerMoore struct {
	pattern string     // the pattern this searcher looks for
	right   [radix]int // right[c] = rightmost index of c in pattern, or -1
}

// NewBoyerMoore returns the Boyer-Moore searcher for pattern.  The
// empty pattern is valid input: it matches at index 0 of every text.
// Complexity is O(m + R), where m = len(pattern) and R = 256.
func NewBoyerMoore(pattern string) *BoyerMoore {
	b := &BoyerMoore{pattern: pattern}
	for c := range radix {
		b.right[c] = -1
	}
	for j := 0; j < len(pattern); j++ {
		b.right[pattern[j]] = j
	}
	return b
}

// Search returns the index of the first (leftmost) occurrence of the
// pattern in text, or -1 if there is none.  The empty pattern returns
// 0; a pattern longer than text returns -1.  A nil searcher returns
// -1.
// Complexity is O(nm) in the worst case, where n = len(text) and
// m = len(pattern); on typical inputs it is sublinear, about n/m.
func (b *BoyerMoore) Search(text string) int {
	if b == nil {
		return -1
	}
	m := len(b.pattern)
	if m == 0 {
		return 0
	}
	n := len(text)
	for i := 0; i <= n-m; {
		// Compare right to left; on a mismatch skip by the bad-character rule.
		skip := 0
		for j := m - 1; j >= 0; j-- {
			if b.pattern[j] != text[i+j] {
				skip = j - b.right[text[i+j]]
				if skip < 1 {
					skip = 1 // never move backwards
				}
				break
			}
		}
		if skip == 0 {
			return i // found
		}
		i += skip
	}
	return -1 // not found
}

// Pattern returns the pattern the searcher was built from.  A nil
// searcher reports "".
// Complexity is O(1).
func (b *BoyerMoore) Pattern() string {
	if b == nil {
		return ""
	}
	return b.pattern
}
