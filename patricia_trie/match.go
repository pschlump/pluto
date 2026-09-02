/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package patricia_trie

import (
	"iter"
	"strings"
)

// KeysThatMatch returns a range-over-func iterator (iter.Seq2) that
// visits every (key, value) pair whose key matches pattern, in ascending
// key order:
//
//	for key, value := range pt.KeysThatMatch("user:*") { ... }
//
// A single-variable range yields the key, not the value (use the
// two-variable form above).  A nil *PatriciaTrie iterates as an empty
// one; breaking out of the loop stops the walk early.
//
// The pattern is a Redis-style glob — the semantics of Redis's
// stringmatchlen (util.c), the matcher behind the KEYS command:
//
//   - any sequence of bytes, including empty
//     ?        any single byte
//     [abc]    any one of the listed bytes
//     [a-z]    any byte in the range (bounds swap if reversed)
//     [^abc]   any byte NOT listed / NOT in range
//     \x       a literal x — the backslash escapes any special byte
//
// Edge behavior mirrors the C exactly: a trailing lone backslash
// matches a literal backslash; an unterminated '[' runs as a class to
// the end of the pattern (a bare trailing '[' is an empty class that
// matches nothing, an unterminated '[^' matches any byte); ']' as the
// first class byte closes immediately; '*' and '?' and classes do not
// match across the end of the key.  The match is on raw bytes — keys
// are binary-safe, any byte value legal.
//
// The iterator walks the live trie: the trie must not be modified while
// the iterator is being consumed.
//
// Cost: the pattern's literal prefix (its bytes before the first
// wildcard, escapes resolved) prunes the descent — only the subtree
// holding keys with that prefix is walked, then each candidate is
// matched in full.  Complexity is O(w + k·m) where w is the literal
// prefix length in bits, k is the number of keys sharing it, and m is
// the pattern/key length — O(n·m) in the worst case for a pattern that
// starts with a wildcard (a full descent).
func (t *PatriciaTrie[T]) KeysThatMatch(pattern string) iter.Seq2[string, T] {
	if t == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	prefix := globLiteralPrefix(pattern)
	return func(yield func(string, T) bool) {
		// Descend to the subtree holding prefix's keys (the same walk
		// as KeysWithPrefix).  Path compression skips bits, so leaves
		// below are re-checked with a literal prefix comparison.
		x := t.root
		for x != nil && x.bit >= 0 && x.bit < symbolBits*len(prefix) {
			x = x.child[bitDir(prefix, x.bit)]
		}
		if x == nil {
			return
		}
		var walk func(x *patriciaNode[T]) bool
		walk = func(x *patriciaNode[T]) bool {
			if x.bit < 0 {
				if strings.HasPrefix(x.key, prefix) && globMatch(pattern, x.key) {
					return yield(x.key, x.value)
				}
				return true
			}
			return walk(x.child[0]) && walk(x.child[1])
		}
		walk(x)
	}
}

// globMatch reports whether s matches the glob pattern in full — the
// behavior of Redis's stringmatchlen (note/redis/src/util.c), ported
// byte for byte, case-sensitive.  See KeysThatMatch for the syntax.
func globMatch(pattern, s string) bool {
	skipLonger := false
	return globMatchImpl(pattern, 0, s, 0, &skipLonger, 0)
}

// globMatchImpl is the recursive core of globMatch — a direct port of
// Redis's stringmatchlen_impl.  pi/si are cursors into pat/s.  The
// skipLonger flag is the C function's early-termination optimization:
// once a '*'s remainder is known to match nowhere in the remaining
// string, enclosing '*' loops stop trying longer prefixes (it never
// changes the outcome).  nesting bounds recursion from pathological
// many-'*' patterns, as in the C.
func globMatchImpl(pat string, pi int, s string, si int, skipLonger *bool, nesting int) bool {
	// Protection against abusive patterns.
	if nesting > 1000 {
		return false
	}
	pEnd, sEnd := len(pat), len(s)
	for pi < pEnd && si < sEnd {
		switch pat[pi] {
		case '*':
			for pi+1 < pEnd && pat[pi+1] == '*' {
				pi++
			}
			if pi == pEnd-1 {
				return true // a lone '*' swallows the rest of the string
			}
			for si < sEnd {
				if globMatchImpl(pat, pi+1, s, si, skipLonger, nesting+1) {
					return true
				}
				if *skipLonger {
					return false
				}
				si++
			}
			// The pattern after '*' matches no suffix of the remaining
			// string; any earlier '*' would need it to match a shorter
			// one, so the enclosing searches can stop too.
			*skipLonger = true
			return false
		case '?':
			si++
		case '[':
			pi++
			negated := pi < pEnd && pat[pi] == '^'
			if negated {
				pi++
			}
			match := false
			for {
				if pi+1 < pEnd && pat[pi] == '\\' {
					pi++
					if pat[pi] == s[si] {
						match = true
					}
				} else if pi == pEnd {
					pi-- // unterminated class: it ran to the end of the pattern
					break
				} else if pat[pi] == ']' {
					break
				} else if pi+2 < pEnd && pat[pi+1] == '-' {
					start, end := pat[pi], pat[pi+2]
					if start > end {
						start, end = end, start
					}
					pi += 2
					if c := s[si]; c >= start && c <= end {
						match = true
					}
				} else if pat[pi] == s[si] {
					match = true
				}
				pi++
			}
			if negated {
				match = !match
			}
			if !match {
				return false
			}
			si++
		case '\\':
			if pi+1 < pEnd {
				pi++ // match the escaped byte that follows
			}
			// A trailing lone backslash matches a literal backslash.
			fallthrough
		default:
			if pat[pi] != s[si] {
				return false
			}
			si++
		}
		pi++
		if si == sEnd {
			// The string is exhausted; only trailing '*'s may remain.
			for pi < pEnd && pat[pi] == '*' {
				pi++
			}
			break
		}
	}
	return pi == pEnd && si == sEnd
}

// globLiteralPrefix returns the longest prefix of pattern that every
// matching string must start with: the bytes before the first wildcard,
// with backslash escapes resolved to their literal bytes.  It is the
// pruning key for KeysThatMatch — "user:\*" yields "user:*", "h?i"
// yields "h", "*x" yields "".
func globLiteralPrefix(pattern string) string {
	var sb strings.Builder
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[':
			return sb.String()
		case '\\':
			if i+1 < len(pattern) {
				i++
				sb.WriteByte(pattern[i])
			} else {
				sb.WriteByte('\\') // a trailing lone backslash is a literal one
			}
		default:
			sb.WriteByte(pattern[i])
		}
	}
	return sb.String()
}
