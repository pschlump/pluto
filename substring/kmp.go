/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package substring

// radix is the alphabet size: the searchers operate on raw bytes.
const radix = 256

// KMP is the Knuth-Morris-Pratt substring searcher for one fixed
// pattern (Sedgwick's KMP, §5.3).  The constructor builds the
// deterministic finite automaton of the pattern — dfa[c][j] is the
// next state when byte c is read in state j — and Search simulates
// that DFA over the text, never backing up in the text.  It is
// immutable after construction; the zero value behaves as the searcher
// for the empty pattern.
type KMP struct {
	pattern string       // the pattern this searcher looks for
	dfa     [radix][]int // dfa[c][j] = state after reading byte c in state j
}

// NewKMP returns the KMP searcher for pattern.  The empty pattern is
// valid input: it matches at index 0 of every text.
// Complexity is O(mR), where m = len(pattern) and R = 256.
func NewKMP(pattern string) *KMP {
	m := len(pattern)
	k := &KMP{pattern: pattern}
	if m == 0 {
		return k
	}
	for c := range radix {
		k.dfa[c] = make([]int, m)
	}
	k.dfa[pattern[0]][0] = 1
	for x, j := 0, 1; j < m; j++ {
		for c := range radix {
			k.dfa[c][j] = k.dfa[c][x] // copy mismatch cases
		}
		k.dfa[pattern[j]][j] = j + 1 // set match case
		x = k.dfa[pattern[j]][x]     // update restart state
	}
	return k
}

// Search returns the index of the first (leftmost) occurrence of the
// pattern in text, or -1 if there is none.  The empty pattern returns
// 0; a pattern longer than text returns -1.  A nil searcher returns
// -1.
// Complexity is O(n), where n = len(text).
func (k *KMP) Search(text string) int {
	if k == nil {
		return -1
	}
	m := len(k.pattern)
	if m == 0 {
		return 0
	}
	j := 0 // DFA state
	for i := 0; i < len(text) && j < m; i++ {
		j = k.dfa[text[i]][j]
		if j == m {
			return i + 1 - m // found
		}
	}
	return -1 // not found
}

// Pattern returns the pattern the searcher was built from.  A nil
// searcher reports "".
// Complexity is O(1).
func (k *KMP) Pattern() string {
	if k == nil {
		return ""
	}
	return k.pattern
}
