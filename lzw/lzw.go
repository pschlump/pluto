/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lzw implements LZW compression and expansion of byte slices —
// the Lempel-Ziv-Welch algorithm of Sedgwick & Wayne, Algorithms, 4th
// ed. §5.5, in the variable-codeword-width variant.
//
// The alphabet is the 256 byte values; codeword 256 is the EOF sentinel
// that terminates every stream; codewords from 257 up denote multi-byte
// strings learned from the input.  The codebook starts with 512 slots
// and codewords are written W bits wide, starting at W = 9 and widening
// by one whenever the next assignable codeword reaches 2^W, up to
// W = 16 (a maximum codebook of 65536 entries).  The codebook is never
// reset.  Bits are packed big-endian, MSB-first — the algs4
// BinaryStdOut / BinaryStdIn convention (see bits.go).
//
//	compressed := lzw.Compress(data)  // []byte, MSB-first codewords
//	back := lzw.Expand(compressed)    // == data, or nil if malformed
//
// Compress builds its encode table on a ternary search trie
// (pluto/tst, an intra-pluto composition — the TST zero value is
// directly usable, so there is nothing to construct).  Expand rebuilds
// the same table as a slice of strings grown one entry per codeword.
//
// The package NEVER panics.  Compress accepts any input, including nil
// (the empty input yields a stream holding just the EOF codeword).
// Expand returns nil for malformed input — a stream truncated mid-code,
// a stream with no EOF codeword, or an out-of-range codeword — and
// otherwise returns the decoded bytes.  Decoding stops at the EOF
// codeword; any trailing bytes after it are ignored (the algs4
// behavior).
//
// Both functions are stateless — they hold no package-level state — so
// they are safe for concurrent use, and there is no _ts twin.
package lzw

import "github.com/pschlump/pluto/tst"

// The LZW format constants.
const (
	r         = 256           // alphabet size: the input symbols are the bytes 0..255
	eofCode   = r             // the EOF sentinel codeword
	firstCode = r + 1         // 257: the first codeword assigned to a multi-byte string
	minWidth  = 9             // initial codeword width, in bits
	maxWidth  = 16            // maximum codeword width, in bits
	maxCodes  = 1 << maxWidth // 65536: codebook capacity — never reset, never widened past
)

// Compress returns the LZW-compressed form of data: a packed bit stream
// of codewords, MSB-first, terminated by the EOF codeword.  The empty
// (or nil) input yields a stream holding just the EOF codeword.
//
// The encode table is a ternary search trie (pluto/tst) mapping each
// known string to its codeword.  Each step emits the codeword of the
// longest known prefix s of the remaining input, then adds s plus the
// following byte to the table.  The codeword width widens by one bit
// whenever the next assignable codeword reaches 2^W.
// Complexity is O(n) expected, where n = len(data): each table
// operation costs O(L) for a matched prefix of length L, and the
// matched lengths sum to n.
func Compress(data []byte) []byte {
	var st tst.Tst[int] // zero value, ready to use
	for i := range r {
		st.Insert(string([]byte{byte(i)}), i)
	}
	code := firstCode // next codeword available for assignment
	width := minWidth
	var w bitWriter

	input := string(data)
	for len(input) > 0 {
		s := st.LongestPrefixOf(input)
		c, _ := st.Search(s) // s is a table key by construction
		w.writeBits(c, width)
		t := len(s)
		if t < len(input) && code < maxCodes {
			st.Insert(input[:t+1], code)
			code++
			if code == (1<<width) && width < maxWidth {
				width++
			}
		}
		input = input[t:]
	}
	w.writeBits(eofCode, width)
	return w.bytes()
}

// Expand returns the bytes encoded by Compress, or nil if compressed is
// malformed.  Malformed means: the stream ends mid-codeword, the EOF
// codeword never appears, or a codeword is out of range (larger than
// any codeword assigned so far — the encoder is always at most one
// codeword ahead of the decoder, the KwKwK case, which is handled per
// the algorithm).  The empty input is malformed (not even one codeword
// fits).  Decoding stops at the EOF codeword; trailing bytes after it
// are ignored.
//
// The decode table is a []string grown one entry per codeword read,
// widening the codeword width in lockstep with the encoder.
// Complexity is O(n) expected, where n is the length of the decoded
// output.
func Expand(compressed []byte) []byte {
	st := make([]string, firstCode, maxCodes)
	for i := range r {
		st[i] = string([]byte{byte(i)})
	}
	// st[eofCode] stays "": the unused lookahead slot of the EOF
	// codeword (algs4's st[i++] = "").

	rd := bitReader{data: compressed}
	width := minWidth

	codeword, ok := rd.readBits(width)
	if !ok {
		return nil // truncated before even the first codeword
	}
	if codeword == eofCode {
		return []byte{} // the empty message
	}
	if codeword > eofCode {
		return nil // the first codeword must be a literal or EOF
	}
	val := st[codeword]
	out := make([]byte, 0, len(compressed))
	i := firstCode // next table index to assign
	for {
		out = append(out, val...)
		codeword, ok = rd.readBits(width)
		if !ok {
			return nil // truncated stream / missing EOF
		}
		if codeword == eofCode {
			break
		}
		if codeword > i {
			return nil // a codeword no encoder can have emitted yet
		}
		var s string
		if codeword == i {
			// The KwKwK case: the encoder emitted the codeword for
			// val+val[:1] — the entry it added one step ago, which the
			// decoder has not seen the tail of yet.
			s = val + val[:1]
		} else {
			s = st[codeword]
		}
		if i < maxCodes {
			st = append(st, val+s[:1])
			i++
			if i == (1<<width)-1 && width < maxWidth {
				width++
			}
		}
		val = s
	}
	return out
}
