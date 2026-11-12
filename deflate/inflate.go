/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate

// The decoder side (RFC 1951 §3.2).  It is deliberately a
// straightforward puff-style canonical decoder: per-length symbol
// tables and bit-at-a-time code walks.  It validates as strictly as
// zlib's inflate does — over-subscribed and incomplete Huffman trees
// are rejected (the single-code distance tree excepted), reserved
// symbols and impossible distances are rejected, and every rejected
// condition reports the package's one failure value, (nil, 0).

// fixedLitTree and fixedDistTree are the §3.2.6 fixed decoding trees,
// built once at package init and read-only afterwards.
var (
	fixedLitTree  = buildFixedTree(fixedLitLengths[:])
	fixedDistTree = buildFixedTree(fixedDistLengths[:])
)

// Expand decodes a raw DEFLATE stream.  It returns the decompressed
// bytes and n, the number of bytes of data the stream occupies: a
// byte holding only the final block's padding bits counts as consumed,
// and data[n:] is untouched trailing input (the gzip and zlib wrappers
// use n to locate their trailers).  Decoding stops at the end of the
// first final (BFINAL=1) block.
//
// A malformed stream returns (nil, 0).  Malformed means: input
// exhausted mid-block, BTYPE 11, a stored block with LEN and the
// one's-complement of NLEN disagreeing, a stored block truncated,
// invalid or over-/under-subscribed Huffman code lengths, a code-length
// repeat with no previous length or past the end of the length
// sequence, a literal/length code for symbols 286-287, a distance
// code for symbols 30-31, a distance reaching before the start of the
// output, or a bit pattern that matches no code in the current tree.
//
// The empty input is malformed.  Complexity is O(n) in the output
// size; each Huffman symbol costs at most 15 bit reads.
func Expand(data []byte) ([]byte, int) {
	r := &bitReader{buf: data}
	out := make([]byte, 0, len(data)+64)
	final := false
	for !final {
		b, ok := r.readBit()
		if !ok {
			return nil, 0
		}
		final = b == 1
		t, ok := r.readBits(2)
		if !ok {
			return nil, 0
		}
		switch t {
		case 0:
			if !expandStored(r, &out) {
				return nil, 0
			}
		case 1:
			if !expandBlock(r, &out, fixedLitTree, fixedDistTree) {
				return nil, 0
			}
		case 2:
			lit, dist, ok := readDynamicHeader(r)
			if !ok || !expandBlock(r, &out, lit, dist) {
				return nil, 0
			}
		default:
			return nil, 0 // BTYPE 11 is reserved
		}
	}
	return out, r.consumed()
}

// expandStored reads one stored (BTYPE 00) block: skip to a byte
// boundary, LEN and its one's complement NLEN as little-endian 16-bit
// values, then LEN raw bytes.
func expandStored(r *bitReader, out *[]byte) bool {
	r.align()
	length, ok := r.readBits(16)
	if !ok {
		return false
	}
	nlen, ok := r.readBits(16)
	if !ok || length != ^nlen&0xFFFF {
		return false
	}
	if r.pos+int(length) > len(r.buf) {
		return false // truncated block
	}
	*out = append(*out, r.buf[r.pos:r.pos+int(length)]...)
	r.pos += int(length)
	r.n = 0
	r.acc = 0
	return true
}

// readDynamicHeader reads the §3.2.7 dynamic-block header and returns
// the two decoding trees.  HLIT > 286 or HDIST > 30 is malformed
// (those symbols do not exist), and the literal tree must carry a code
// for the end-of-block symbol 256 — a block that cannot end is not a
// block.
func readDynamicHeader(r *bitReader) (lit, dist *decTree, ok bool) {
	hlit, ok := r.readBits(5)
	if !ok {
		return nil, nil, false
	}
	hdist, ok := r.readBits(5)
	if !ok {
		return nil, nil, false
	}
	hclen, ok := r.readBits(4)
	if !ok {
		return nil, nil, false
	}
	nlen := int(hlit) + 257
	ndist := int(hdist) + 1
	if nlen > 286 || ndist > 30 {
		return nil, nil, false
	}
	ncl := int(hclen) + 4

	var clLengths [19]uint8
	for i := 0; i < ncl; i++ {
		v, ok := r.readBits(3)
		if !ok {
			return nil, nil, false
		}
		clLengths[clOrder[i]] = uint8(v)
	}
	clTree, ok := buildDecTree(clLengths[:], false)
	if !ok {
		return nil, nil, false // the code-length tree must be complete
	}

	lengths := make([]uint8, 0, nlen+ndist)
	prev := byte(0)
	for len(lengths) < nlen+ndist {
		sym, ok := clTree.decode(r)
		if !ok {
			return nil, nil, false
		}
		if sym <= 15 {
			lengths = append(lengths, byte(sym))
			prev = byte(sym)
			continue
		}
		var run int
		switch sym {
		case 16:
			if len(lengths) == 0 {
				return nil, nil, false // no previous length to copy
			}
			x, ok := r.readBits(2)
			if !ok {
				return nil, nil, false
			}
			run = 3 + int(x)
			for ; run > 0; run-- {
				lengths = append(lengths, prev)
			}
		case 17:
			x, ok := r.readBits(3)
			if !ok {
				return nil, nil, false
			}
			run = 3 + int(x)
			for ; run > 0; run-- {
				lengths = append(lengths, 0)
			}
		case 18:
			x, ok := r.readBits(7)
			if !ok {
				return nil, nil, false
			}
			run = 11 + int(x)
			for ; run > 0; run-- {
				lengths = append(lengths, 0)
			}
		}
		if len(lengths) > nlen+ndist {
			return nil, nil, false // repeat ran past the sequence end
		}
	}
	if lengths[256] == 0 {
		return nil, nil, false // no end-of-block code: block can never end
	}
	lit, ok = buildDecTree(lengths[:nlen], false)
	if !ok {
		return nil, nil, false
	}
	dist, ok = buildDecTree(lengths[nlen:], true)
	if !ok {
		return nil, nil, false
	}
	return lit, dist, true
}

// expandBlock reads literals and matches until the end-of-block symbol
// 256.  Matches may overlap the output (distance < length); each is
// copied byte by byte from dist behind the current end.
func expandBlock(r *bitReader, out *[]byte, lit, dist *decTree) bool {
	for {
		sym, ok := lit.decode(r)
		if !ok {
			return false
		}
		if sym < 256 {
			*out = append(*out, byte(sym))
			continue
		}
		if sym == 256 {
			return true
		}
		if sym > 285 {
			return false // 286-287 never appear in a valid stream
		}
		lc := sym - 257
		length := int(lengthBase[lc])
		if x := lengthExtra[lc]; x > 0 {
			v, ok := r.readBits(uint(x))
			if !ok {
				return false
			}
			length += int(v)
		}
		dsym, ok := dist.decode(r)
		if !ok {
			return false
		}
		if dsym > 29 {
			return false // 30-31 never appear in a valid stream
		}
		d := int(distBase[dsym])
		if x := distExtra[dsym]; x > 0 {
			v, ok := r.readBits(uint(x))
			if !ok {
				return false
			}
			d += int(v)
		}
		if d > len(*out) {
			return false // distance reaches before the start of output
		}
		start := len(*out) - d
		for k := 0; k < length; k++ {
			*out = append(*out, (*out)[start+k])
		}
	}
}
