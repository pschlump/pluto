/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import "encoding/binary"

// BitOpAnd returns the bitwise AND of its sources — BITOP AND dest src...
// The result is a newly allocated slice as long as the longest source;
// sources shorter than that read as zero beyond their end (missing keys
// are an infinite zero array), so a single source yields a copy of
// itself.  A single source shorter than the rest zero-pads the tail.
//
// Complexity is O(k·n) word-wise (k sources, n the longest length —
// 8 bytes per step, the word-at-a-time shape of bitopCommand's fallback
// path; no bit loops anywhere).
func BitOpAnd(srcs ...[]byte) []byte {
	maxlen := 0
	for _, s := range srcs {
		if len(s) > maxlen {
			maxlen = len(s)
		}
	}
	res := make([]byte, maxlen)
	if len(srcs) > 0 {
		copy(res, srcs[0])
	}
	for k := 1; k < len(srcs); k++ {
		s := srcs[k]
		n := len(s)
		if n > maxlen {
			n = maxlen
		}
		for i := 0; i+8 <= n; i += 8 {
			w := binary.LittleEndian.Uint64(res[i:]) & binary.LittleEndian.Uint64(s[i:])
			binary.LittleEndian.PutUint64(res[i:], w)
		}
		for i := n &^ 7; i < n; i++ {
			res[i] &= s[i]
		}
		if n < maxlen {
			// AND with this source's zero padding clears the tail
			// (later, longer sources only AND within their own
			// length; 0 AND anything stays 0).
			clear(res[n:])
		}
	}
	return res
}

// BitOpOr returns the bitwise OR of its sources — BITOP OR dest src...
// The result is a newly allocated slice as long as the longest source;
// shorter sources read as zero beyond their end, so a single source
// yields a copy of itself and a nil source contributes nothing.
//
// Complexity is O(k·n) word-wise, as BitOpAnd.
func BitOpOr(srcs ...[]byte) []byte {
	maxlen := 0
	for _, s := range srcs {
		if len(s) > maxlen {
			maxlen = len(s)
		}
	}
	res := make([]byte, maxlen)
	if len(srcs) > 0 {
		copy(res, srcs[0])
	}
	for k := 1; k < len(srcs); k++ {
		s := srcs[k]
		n := len(s)
		if n > maxlen {
			n = maxlen
		}
		for i := 0; i+8 <= n; i += 8 {
			w := binary.LittleEndian.Uint64(res[i:]) | binary.LittleEndian.Uint64(s[i:])
			binary.LittleEndian.PutUint64(res[i:], w)
		}
		for i := n &^ 7; i < n; i++ {
			res[i] |= s[i]
		}
	}
	return res
}

// BitOpXor returns the bitwise XOR of its sources — BITOP XOR dest src...
// The result is a newly allocated slice as long as the longest source;
// shorter sources read as zero beyond their end, so a single source
// yields a copy of itself.
//
// Complexity is O(k·n) word-wise, as BitOpAnd.
func BitOpXor(srcs ...[]byte) []byte {
	maxlen := 0
	for _, s := range srcs {
		if len(s) > maxlen {
			maxlen = len(s)
		}
	}
	res := make([]byte, maxlen)
	if len(srcs) > 0 {
		copy(res, srcs[0])
	}
	for k := 1; k < len(srcs); k++ {
		s := srcs[k]
		n := len(s)
		if n > maxlen {
			n = maxlen
		}
		for i := 0; i+8 <= n; i += 8 {
			w := binary.LittleEndian.Uint64(res[i:]) ^ binary.LittleEndian.Uint64(s[i:])
			binary.LittleEndian.PutUint64(res[i:], w)
		}
		for i := n &^ 7; i < n; i++ {
			res[i] ^= s[i]
		}
	}
	return res
}

// BitOpNot returns the bitwise complement of src — BITOP NOT dest src,
// which takes exactly one source by construction here.  The result is a
// newly allocated slice of the source's length (the complement of an
// empty string is empty).
//
// Complexity is O(n) word-wise, as BitOpAnd.
func BitOpNot(src []byte) []byte {
	res := make([]byte, len(src))
	for i := 0; i+8 <= len(src); i += 8 {
		binary.LittleEndian.PutUint64(res[i:], ^binary.LittleEndian.Uint64(src[i:]))
	}
	for i := len(src) &^ 7; i < len(src); i++ {
		res[i] = ^src[i]
	}
	return res
}
