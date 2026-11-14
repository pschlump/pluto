/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz

import "github.com/pschlump/pluto/internal/lzmacore"

// The LZMA model lives in pluto/internal/lzmacore (shared with the
// lzma-alone codec); these aliases keep the xz package's names.
type (
	lzmaState    = lzmacore.LzmaState
	rangeDecoder = lzmacore.RangeDecoder
)

func newLzmaState(props byte) *lzmaState      { return lzmacore.NewLzmaState(props) }
func newRangeDecoder(in []byte) *rangeDecoder { return lzmacore.NewRangeDecoder(in) }
func lzmaDecode(rc *rangeDecoder, st *lzmaState, out *[]byte, want, dictBase int) bool {
	return lzmacore.LzmaDecode(rc, st, out, want, dictBase)
}
