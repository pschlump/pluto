package xz

import "github.com/pschlump/pluto/internal/lzmacore"

// The LZMA encoder: the range encoder (the decoder's mirror image —
// same 11-bit probabilities, same adaptation) plus a greedy
// hash-chain match finder with one-position lookahead.  Symbols are
// literals and simple matches (repeated distances are never emitted,
// which every decoder accepts and only costs some ratio); the model
// state transitions mirror the decoder's exactly, since its literal
// decoding keys off them.

// chunkMaxUnpacked is how much data one LZMA chunk carries.  The
// format ceiling is 2 MiB minus one, but a chunk's compressed size is
// a 16-bit field — cutting at 64 KiB keeps a compressed chunk under
// it (incompressible cuts fall back to uncompressed chunks anyway).
const chunkMaxUnpacked = 1 << 16

// encodeLZMA2Chunks appends src as LZMA2 chunks: each cut of at most
// chunkMaxUnpacked bytes becomes one LZMA chunk (falling back to an
// uncompressed chunk when compression does not pay), with the
// dictionary continuing across chunks.  fresh marks the block's first
// chunk (full reset); the returned int is the total chunk bytes.
func encodeLZMA2Chunks(dst []byte, out, src []byte) []byte {
	fresh := true
	var st *lzmaState
	for len(src) > 0 {
		n := len(src)
		if n > chunkMaxUnpacked {
			n = chunkMaxUnpacked
		}
		cut := src[:n]
		var enc []byte
		enc, st = lzmacore.LzmaEncode(nil, out, cut, st, 0)
		if len(enc)+6 < n {
			ctl := byte(0x80)
			if fresh {
				ctl = 0xE0 // state reset + new properties
			}
			unpacked := n - 1
			compField := len(enc) - 1 // the LZMA data length minus one; the properties byte is extra
			dst = append(dst, ctl|byte(unpacked>>16), byte(unpacked>>8), byte(unpacked), byte(compField>>8), byte(compField))
			if fresh {
				dst = append(dst, defaultProps)
			}
			dst = append(dst, enc...)
		} else {
			dst = lzma2Uncompressed(dst, cut, fresh)
		}
		out = append(out, cut...)
		src = src[n:]
		fresh = false
	}
	return dst
}

// lzma2Uncompressed appends src as LZMA2 uncompressed chunks (the
// incompressible-data fallback, and any encoder's legal floor).
func lzma2Uncompressed(dst []byte, src []byte, dictReset bool) []byte {
	for len(src) > 0 {
		n := len(src)
		if n > 1<<16 {
			n = 1 << 16
		}
		ctl := byte(0x02)
		if dictReset {
			ctl = 0x01
			dictReset = false
		}
		dst = append(dst, ctl, byte((n-1)>>8), byte(n-1))
		dst = append(dst, src[:n]...)
		src = src[n:]
	}
	return dst
}
