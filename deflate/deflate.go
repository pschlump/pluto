/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package deflate implements DEFLATE — the LZ77 + canonical-Huffman
// compressed data format of RFC 1951 that gzip (RFC 1952), zlib
// (RFC 1950), zip (method 8) and thus .tar.gz / .tgz archives are
// built on.  Two stateless functions are the slice API; the
// streaming Reader and Writer of stream.go cover the same format
// over io.Reader and io.Writer:
//
//	compressed := deflate.Compress(data)   // a raw RFC 1951 stream
//	back, n := deflate.Expand(compressed)  // == data; n bytes consumed
//
// Expand returns (nil, 0) for malformed input (see the Expand doc for
// the full contract).  This package deals in raw DEFLATE streams; the
// gzip and zlib packages add the member/trailer framing.
//
// The compressor emits all three RFC block types — stored, fixed and
// dynamic — choosing per block whichever is smallest.  LZ77 matching
// is the standard hash-chain search with one-step lazy evaluation
// (match lengths 3..258, window 32768), at a fixed setting roughly
// comparable to zlib's mid levels; it is not tunable.  The decoder is
// a straightforward puff-style canonical decoder.  Neither function
// ever panics, both are stateless, and there is no _ts twin (the lzw
// precedent: pure CPU-bound transforms, nothing shared).
package deflate

// Format and quality constants.
const (
	windowSize   = 32768 // LZ77 window (RFC 1951 §2)
	windowMask   = windowSize - 1
	minMatch     = 3   // shortest encodable match
	maxMatch     = 258 // longest encodable match
	hashBits     = 15  // hash-chain address space
	hashSize     = 1 << hashBits
	maxChain     = 128     // hash-chain walk limit (zlib level-6 ballpark)
	blockTokens  = 1 << 14 // symbols per emitted block
	maxStoredLen = 0xFFFF  // a stored block carries at most 65535 bytes
	maxCodeLen   = 15      // DEFLATE Huffman code length limit (7 for CL)
)

// token is one LZ77 symbol: length == 0 means a literal byte in lit;
// otherwise it is a match of length (3..258) at distance (1..32768).
type token struct {
	lit    uint16
	length uint16
	dist   uint16
}

// Compress returns data as a raw DEFLATE stream.  Any input is
// accepted, including nil and empty — the empty input compresses to
// the 2-byte final-empty-fixed-block stream 03 00.  The result is
// never nil.  Complexity is O(n) expected (hash-chain matching with a
// bounded chain walk); the output bytes are deterministic for a given
// input.
func Compress(data []byte) []byte {
	e := &encoder{
		head: make([]int32, hashSize),
		prev: make([]int32, windowSize),
	}
	for i := range e.head {
		e.head[i] = -1
	}
	for i := range e.prev {
		e.prev[i] = -1
	}
	e.compress(data)
	return e.w.bytes()
}

// encoder carries the per-call state: the output bit stream, the LZ77
// hash chains, and the block under construction.
type encoder struct {
	w    bitWriter
	head []int32 // hash -> most recent position with that hash
	prev []int32 // position&windowMask -> previous position with same hash

	tokens         []token
	insertCursor   int // every position < insertCursor is linked
	blockStartByte int // input offset where the current block begins
}

// hash3 hashes the 3 bytes at b (Knuth multiplicative, shifted to
// hashBits).
func hash3(b []byte) uint32 {
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
	return v * 2654435761 >> (32 - hashBits)
}

// insert links position p into the hash chains.  Positions are linked
// exactly once, in increasing order, so each chain is a decreasing
// position list; entries older than the 32 KiB window are skipped
// during the walk.
func (e *encoder) insert(data []byte, p int) {
	if p+minMatch > len(data) {
		return
	}
	h := hash3(data[p:])
	e.prev[p&windowMask] = e.head[h]
	e.head[h] = int32(p)
}

// insertUpto links every not-yet-linked position below p.
func (e *encoder) insertUpto(data []byte, p int) {
	if p > len(data) {
		p = len(data)
	}
	for ; e.insertCursor < p; e.insertCursor++ {
		e.insert(data, e.insertCursor)
	}
}

// matchLen returns the length of the match between data[j:] and
// data[i:], capped at max.  An overlap of the two (j+k >= i) is legal
// LZ77 — the decoder copies byte by byte — and is measured directly
// against the input, which the encoder already knows in full.
func matchLen(data []byte, j, i, max int) int {
	n := len(data)
	k := 0
	for k < max && i+k < n && data[j+k] == data[i+k] {
		k++
	}
	return k
}

// findMatch walks the hash chain of data[i:], returning the longest
// match within the window (length 0 if none reaches minMatch).  The
// chain head may be i itself, or i+1 after the lazy-match lookahead
// insert — the encoder's own positions — which are skipped first: a
// match at distance 0 is not LZ77.
func (e *encoder) findMatch(data []byte, i int) (length, dist int) {
	n := len(data)
	if i+minMatch > n {
		return 0, 0
	}
	limit := i - windowSize
	if limit < 0 {
		limit = 0
	}
	j := int(e.head[hash3(data[i:])])
	for j >= i {
		j = int(e.prev[j&windowMask])
	}
	best, bestDist := 0, 0
	for chain := 0; j >= limit && chain < maxChain; chain++ {
		if best == 0 || (i+best < n && data[j+best] == data[i+best]) {
			if l := matchLen(data, j, i, maxMatch); l > best {
				best, bestDist = l, i-j
				if best == maxMatch {
					break
				}
			}
		}
		j = int(e.prev[j&windowMask])
	}
	if best < minMatch {
		return 0, 0
	}
	return best, bestDist
}

// compress is the tokenize-and-emit loop.  Tokens accumulate into
// e.tokens and flush as a non-final block whenever blockTokens is
// reached; the tail flushes as the final block (an empty tail — an
// empty input, say — emits a final block holding only the
// end-of-block symbol).
func (e *encoder) compress(data []byte) {
	n := len(data)
	i := 0
	for i < n {
		e.insertUpto(data, i+1)
		length, dist := e.findMatch(data, i)
		if length >= minMatch && length < maxMatch && i+1 < n {
			// Lazy evaluation: if the next position matches longer,
			// emit this byte as a literal and take that match.
			e.insertUpto(data, i+2)
			if l2, d2 := e.findMatch(data, i+1); l2 > length {
				e.emit(token{lit: uint16(data[i])})
				i++
				length, dist = l2, d2
			}
		}
		if length >= minMatch {
			e.emit(token{length: uint16(length), dist: uint16(dist)})
			e.insertUpto(data, i+length)
			i += length
		} else {
			e.emit(token{lit: uint16(data[i])})
			i++
		}
		if len(e.tokens) >= blockTokens {
			e.flushBlock(data[e.blockStartByte:i], false)
			e.blockStartByte = i
		}
	}
	e.flushBlock(data[e.blockStartByte:], true)
}

// emit appends one token to the pending block.
func (e *encoder) emit(t token) {
	e.tokens = append(e.tokens, t)
}

// flushBlock writes e.tokens (plus the end-of-block symbol) as one
// DEFLATE block covering exactly the bytes of block, choosing the
// smallest of the stored, fixed and dynamic encodings, and resets the
// token buffer.  final marks BFINAL.
func (e *encoder) flushBlock(block []byte, final bool) {
	litFreq := make([]int, 288)
	distFreq := make([]int, 30)
	extraBits := 0
	for _, t := range e.tokens {
		if t.length == 0 {
			litFreq[t.lit]++
			continue
		}
		lc := lengthCodeOf(int(t.length))
		litFreq[257+lc]++
		extraBits += int(lengthExtra[lc])
		dc := distCodeOf(int(t.dist))
		distFreq[dc]++
		extraBits += int(distExtra[dc])
	}
	litFreq[256]++ // the end-of-block symbol
	litUsed := 0
	for _, f := range litFreq {
		if f > 0 {
			litUsed++
		}
	}

	// Candidate costs, in bits.  The stored candidate pads to a byte
	// boundary after its 3 header bits; the pad depends on the current
	// bit position.
	const hdrBits = 3
	storedBits := -1
	if len(block) <= maxStoredLen {
		pad := (8 - int(e.w.n+hdrBits)%8) % 8
		storedBits = hdrBits + pad + 32 + 8*len(block)
	}
	fixedBits := hdrBits
	for s, f := range litFreq {
		if f > 0 {
			fixedBits += f * int(fixedLitLengths[s])
		}
	}
	for _, f := range distFreq {
		fixedBits += f * 5
	}
	fixedBits += extraBits

	// Build the dynamic block pieces (unless the literal tree would
	// hold a single code, which no decoder accepts — a one-symbol
	// literal code is an incomplete tree).
	var (
		litLengths, distLengths, clLengths []uint8
		ops                                []clOp
		hcCount, nlen, ndist               int
		dynamicBits                        = -1
	)
	if litUsed >= 2 {
		litLengths = buildLengths(litFreq, maxCodeLen)
		distLengths = buildLengths(distFreq, maxCodeLen)
		if distAllZero(distLengths) {
			// A distance tree with no used codes still transmits one
			// 1-bit code (the zlib convention: incomplete trees of a
			// single code are legal only for the distance alphabet).
			distLengths[0] = 1
		}
		nlen = trimTrailing(litLengths[:], 257)
		ndist = trimTrailing(distLengths[:], 1)
		ops = clEncodeLengths(concatLengths(litLengths[:nlen], distLengths[:ndist]))
		clFreq := make([]int, 19)
		for _, op := range ops {
			clFreq[op.sym]++
		}
		clLengths = buildLengths(clFreq, 7)
		hcCount = 19
		for hcCount > 4 && clLengths[clOrder[hcCount-1]] == 0 {
			hcCount--
		}
		dynamicBits = hdrBits + 5 + 5 + 4 + 3*hcCount
		for s, f := range clFreq {
			if f > 0 {
				dynamicBits += f * int(clLengths[s])
			}
		}
		for s, f := range litFreq {
			if f > 0 {
				dynamicBits += f * int(litLengths[s])
			}
		}
		for d, f := range distFreq {
			if f > 0 {
				dynamicBits += f * int(distLengths[d])
			}
		}
		dynamicBits += extraBits + clExtraBits(ops)
	}

	finalBit := uint(0)
	if final {
		finalBit = 1
	}
	switch {
	case storedBits >= 0 && storedBits <= fixedBits &&
		(dynamicBits < 0 || storedBits <= dynamicBits):
		e.w.writeBits(finalBit, 1)
		e.w.writeBits(0, 2) // BTYPE 00: stored
		e.w.alignBytes()
		e.w.writeBits(uint(len(block)), 16)
		e.w.writeBits(^uint(len(block)), 16)
		e.w.buf = append(e.w.buf, block...)

	case dynamicBits >= 0 && dynamicBits < fixedBits:
		e.w.writeBits(finalBit, 1)
		e.w.writeBits(2, 2) // BTYPE 10: dynamic
		e.w.writeBits(uint(nlen-257), 5)
		e.w.writeBits(uint(ndist-1), 5)
		e.w.writeBits(uint(hcCount-4), 4)
		clCodes := canonicalCodes(clLengths[:])
		for i := 0; i < hcCount; i++ {
			e.w.writeBits(uint(clLengths[clOrder[i]]), 3) // the length VALUE, 3 bits
		}
		for _, op := range ops {
			e.w.writeCode(clCodes[op.sym])
			if op.extraBits > 0 {
				e.w.writeBits(uint(op.extraVal), uint(op.extraBits))
			}
		}
		e.writeTokens(canonicalCodes(litLengths[:]), canonicalCodes(distLengths[:]))

	default:
		e.w.writeBits(finalBit, 1)
		e.w.writeBits(1, 2) // BTYPE 01: fixed
		e.writeTokens(fixedLitCodes[:], fixedDistCodes[:])
	}

	e.tokens = e.tokens[:0]
}

// writeTokens emits the pending tokens plus the end-of-block symbol
// through the given code tables.
func (e *encoder) writeTokens(litCodes, distCodes []huffCode) {
	for _, t := range e.tokens {
		if t.length == 0 {
			e.w.writeCode(litCodes[t.lit])
			continue
		}
		lc := lengthCodeOf(int(t.length))
		e.w.writeCode(litCodes[257+lc])
		if x := lengthExtra[lc]; x > 0 {
			e.w.writeBits(uint(int(t.length)-int(lengthBase[lc])), uint(x))
		}
		dc := distCodeOf(int(t.dist))
		e.w.writeCode(distCodes[dc])
		if x := distExtra[dc]; x > 0 {
			e.w.writeBits(uint(int(t.dist)-int(distBase[dc])), uint(x))
		}
	}
	e.w.writeCode(litCodes[256]) // end of block
}

// distAllZero reports whether no distance is used.
func distAllZero(lengths []uint8) bool {
	for _, l := range lengths {
		if l != 0 {
			return false
		}
	}
	return true
}

// trimTrailing returns the effective length with trailing zero code
// lengths removed, never below minimum.
func trimTrailing(lengths []uint8, minimum int) int {
	n := len(lengths)
	for n > minimum && lengths[n-1] == 0 {
		n--
	}
	return n
}

// concatLengths joins the literal/length and distance code-length
// sequences — they are transmitted as one run-length-coded sequence.
func concatLengths(a, b []uint8) []uint8 {
	out := make([]uint8, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}

// clOp is one operation of the code-length (HCLEN) encoding: emit
// symbol sym, then extraVal in extraBits bits (both zero except for
// the repeat codes 16, 17, 18).
type clOp struct {
	sym       uint8
	extraVal  uint8
	extraBits uint8
}

// clEncodeLengths run-length-encodes a code-length sequence with the
// §3.2.7 repeat codes: 16 copies the previous length 3-6 times (2
// extra bits), 17 emits 3-10 zeros (3 extra bits), 18 emits 11-138
// zeros (7 extra bits).
func clEncodeLengths(lengths []uint8) []clOp {
	var ops []clOp
	for i := 0; i < len(lengths); {
		v := lengths[i]
		run := 1
		for i+run < len(lengths) && lengths[i+run] == v {
			run++
		}
		i += run
		if v == 0 {
			for run >= 11 {
				r := min(run, 138)
				ops = append(ops, clOp{18, uint8(r - 11), 7})
				run -= r
			}
			if run >= 3 {
				ops = append(ops, clOp{17, uint8(run - 3), 3})
				run = 0
			}
			for ; run > 0; run-- {
				ops = append(ops, clOp{0, 0, 0})
			}
		} else {
			ops = append(ops, clOp{v, 0, 0})
			run--
			for run >= 3 {
				r := min(run, 6)
				ops = append(ops, clOp{16, uint8(r - 3), 2})
				run -= r
			}
			for ; run > 0; run-- {
				ops = append(ops, clOp{v, 0, 0})
			}
		}
	}
	return ops
}

// clExtraBits totals the extra bits carried by the repeat operations.
func clExtraBits(ops []clOp) int {
	bits := 0
	for _, op := range ops {
		bits += int(op.extraBits)
	}
	return bits
}
