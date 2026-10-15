/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package run_length implements the simple form of run-length
// encoding over byte slices, following Sedgwick & Wayne, Algorithms,
// 4th ed. §5.5 (RunLength.java): bit streams with long runs of
// repeated bits are compressed by replacing each run with a one-byte
// count of its length.
//
// The input is a byte slice; its bits are read MSB-first
// (bit 0 is the most significant bit of data[0] — the same bit order
// as bitmap and lzw).  The output is a slice of count bytes, one per
// run, where runs strictly alternate zero-bits / one-bits and ALWAYS
// begin with a run of zero-bits — so a stream that starts with a 1-bit
// encodes a first count of 0.  A run longer than 255 bits saturates:
// it is emitted as 255, then a 0 count for the opposite polarity
// (which decodes to nothing and merely restores the alternation), then
// the remainder — repeated as needed.  This eager split is exactly
// equivalent to algs4's lazy toggle-at-256 encoder, and it is what the
// algs4 decoder (which strictly alternates polarity per count byte and
// knows nothing of the split) already understands.
//
// Expand inverts Compress.  A blob is malformed — Expand returns nil —
// when its counts do not sum to a whole number of bytes (a bit stream
// that came from a []byte always does); interior or trailing zero
// counts are tolerated (they decode to no bits), though only Compress's
// output is canonical.  Compress(empty) and Expand(empty) are the empty
// result in both directions.
//
// Both functions are pure: the input is never modified, the result is a
// fresh slice.  There are no panics at all, no state, and no shared
// package state, so the functions are safe for concurrent use and no
// _ts twin is needed (the lzw precedent).
package run_length

// Compress run-length encodes data.  The input's bits (MSB-first) are
// replaced by one count byte per alternating run, beginning with a run
// of zero-bits; runs longer than 255 bits split as 255, 0, remainder.
// The input is not modified; the result is a fresh slice.  Empty input
// compresses to an empty result.  The function never panics.
// Complexity is O(n) over the input bytes.
func Compress(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	// Walk the bits MSB-first, tracking the current run length and
	// polarity.  A run longer than 255 emits 255, then a 0 count for
	// the opposite polarity, and keeps counting (the eager form of the
	// algs4 toggle-at-256 split).
	var out []byte
	run := 0
	const maxRun = 255

	flush := func() {
		for run > maxRun {
			out = append(out, maxRun, 0)
			run -= maxRun
		}
		out = append(out, byte(run))
	}

	// The first run is zero-bits; if the stream opens with a 1-bit its
	// count is 0.
	cur := false
	run = 0
	if data[0]&0x80 != 0 {
		out = append(out, 0) // zero-bits run of length 0
		cur = true
	}

	for _, b := range data {
		for bit := 7; bit >= 0; bit-- {
			v := b&(1<<bit) != 0
			if v != cur {
				flush()
				cur = v
				run = 0
			}
			run++
		}
	}
	flush()

	return out
}

// Expand run-length decodes blob.  Each count byte emits that many
// bits of the alternating polarity, beginning with zero-bits; bits are
// packed MSB-first.  It returns nil when the blob is malformed — the
// counts do not sum to a whole number of bytes.  Empty input expands
// to an empty result.  The input is not modified; the result is a
// fresh slice.  The function never panics.
// Complexity is O(m) over the blob bytes.
func Expand(blob []byte) []byte {
	// Count the total bits first: the whole-byte check must happen
	// before anything is allocated, and it also gives the exact output
	// size.
	total := 0
	for _, c := range blob {
		total += int(c)
	}
	if total%8 != 0 {
		return nil
	}
	if total == 0 {
		return nil
	}

	out := make([]byte, total/8)
	cur := false // runs alternate, beginning with zero-bits
	pos := 0     // absolute bit position in out, MSB-first
	for _, c := range blob {
		for range c {
			if cur {
				out[pos/8] |= 1 << (7 - pos%8)
			}
			pos++
		}
		cur = !cur
	}

	return out
}
