/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

import "errors"

// FSE (Finite State Entropy) decoding: the table build from a
// normalized distribution, the normalized-count header reader, and
// the predefined distributions for the sequence codes.  An FSE state
// indexes the table; each step yields a symbol and the bits to add to
// the baseline for the next state.

var errCorrupt = errors.New("zstd: corrupt stream")

const maxFSETable = 512 // 2^9, the largest accuracy log the format allows

type fseTable struct {
	log  uint8
	sym  [maxFSETable]byte
	bits [maxFSETable]uint8
	base [maxFSETable]uint16
}

// rleFSE builds the one-state table of RLE_Mode.
func rleFSE(sym byte) fseTable {
	var t fseTable
	t.sym[0], t.bits[0], t.base[0] = sym, 0, 0
	return t
}

// buildFSE spreads a normalized distribution over the decode table
// the way the format fixes it: "less than 1" symbols (-1 counts) take
// single cells from the end of the table retreating; every other
// symbol takes count cells at stride (size>>1)+(size>>3)+3, skipping
// the reserved tail; then each cell's next-state rule comes from the
// symbol's state counter (Table 21 of RFC 8878).
func buildFSE(norm []int16, log uint8) (fseTable, error) {
	var t fseTable
	t.log = log
	size := 1 << log
	high := size - 1
	for s, c := range norm {
		if c != -1 {
			continue
		}
		t.sym[high] = byte(s)
		high--
	}
	pos, step, m := 0, (size>>1)+(size>>3)+3, size-1
	for s, c := range norm {
		if c <= 0 {
			continue
		}
		if s > 255 {
			return t, errCorrupt
		}
		for i := int16(0); i < c; i++ {
			t.sym[pos] = byte(s)
			pos = (pos + step) & m
			for pos > high {
				pos = (pos + step) & m
			}
		}
	}
	var next [256]uint16
	for s, c := range norm {
		if c == -1 {
			next[s] = 1 // a -1 state is a full reset: tableLog bits, base 0
		} else if s <= 255 {
			next[s] = uint16(c)
		}
	}
	for u := 0; u < size; u++ {
		s := t.sym[u]
		n := next[s]
		next[s]++
		nb := uint8(int(log) - highbit32(uint32(n)))
		t.bits[u] = nb
		t.base[u] = uint16((int(n) << nb) - size)
	}
	return t, nil
}

// readNCount decodes a normalized distribution header: a forward
// LSB-first bitstream holding the accuracy log, then per-symbol
// probabilities with the variable-width encoding of RFC 8878 §4.1.1
// (small values one bit short) and zero runs as chains of 2-bit
// repeat fields.  It returns the counts (indexed from symbol 0), the
// accuracy log, and the bytes consumed.
func readNCount(b []byte, maxLog, maxSym int) ([]int16, uint8, int, error) {
	if len(b) == 0 {
		return nil, 0, 0, errCorrupt
	}
	log := int(b[0]&0xF) + 5
	if log > maxLog {
		return nil, 0, 0, errCorrupt
	}
	var f fwdBits
	f.b = b
	f.acc = uint32(b[0]) >> 4 // low 4 bits were the accuracy log
	f.pos = 1
	f.n = 4

	remaining, threshold := int32(1)<<log+1, int32(1)<<log
	nbBits := log + 1
	norm := make([]int16, 0, maxSym+1)
	sym := 0
	previous0 := false
	for remaining > 1 {
		if !f.load() {
			return nil, 0, 0, errCorrupt
		}
		if previous0 {
			// A zero count is followed by 2-bit repeat fields: each
			// 3 adds three more zeros and continues; the first field
			// below 3 ends the run.
			for {
				r := int(f.peek(2))
				f.skip(2)
				sym += r
				if r < 3 {
					break
				}
			}
			previous0 = false
			for len(norm) < sym {
				norm = append(norm, 0)
			}
		}
		if sym > maxSym {
			return nil, 0, 0, errCorrupt
		}
		max := (2*threshold - 1) - remaining
		v := int32(f.peek(nbBits))
		var value int32
		if v&(threshold-1) < max {
			value = v & (threshold - 1)
			f.skip(nbBits - 1)
		} else {
			value = v
			if value >= threshold {
				value -= max
			}
			f.skip(nbBits)
		}
		count := value - 1 // 0 encodes the -1 "less than 1" probability
		remaining -= abs32(count)
		if remaining < 1 {
			return nil, 0, 0, errCorrupt
		}
		norm = append(norm, int16(count))
		sym++
		if count == 0 {
			previous0 = true
		}
		for remaining < threshold {
			nbBits--
			threshold >>= 1
		}
	}
	if remaining != 1 || len(norm) > maxSym+1 {
		return nil, 0, 0, errCorrupt
	}
	nonzero := 0
	for _, c := range norm {
		if c != 0 {
			nonzero++
		}
	}
	if nonzero < 2 {
		return nil, 0, 0, errCorrupt
	}
	// Bits consumed so far (including the 4-bit accuracy prefix)
	// rounded up to whole bytes.
	consumed := (4 + f.consumed + 7) >> 3
	if consumed <= 0 || consumed > len(b) {
		return nil, 0, 0, errCorrupt
	}
	return norm, uint8(log), consumed, nil
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// The predefined distributions (RFC 8878 §3.1.1.3.2.2), including the
// -1 "less than 1" probabilities the format's default tables carry.
var (
	llNorm = []int16{
		4, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 2, 1, 1, 1, 1, 1,
		-1, -1, -1, -1,
	}
	mlNorm = []int16{
		1, 4, 3, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, -1, -1,
		-1, -1, -1, -1, -1,
	}
	ofNorm = []int16{
		1, 1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, -1, -1, -1, -1, -1,
	}
)

var (
	llPredef, _ = buildFSE(llNorm, 6)
	mlPredef, _ = buildFSE(mlNorm, 6)
	ofPredef, _ = buildFSE(ofNorm, 5)
)

// Sequence code tables: baselines and extra-bit counts for the
// literals-length (0-35) and match-length (0-52) codes.
var (
	llBits = [36]uint8{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		1, 1, 1, 1, 2, 2, 3, 3, 4, 6, 7, 8, 9, 10, 11, 12,
		13, 14, 15, 16,
	}
	llBase = [36]uint32{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
		16, 18, 20, 22, 24, 28, 32, 40, 48, 64, 128, 256, 512, 1024,
		2048, 4096, 8192, 16384, 32768, 65536,
	}
	mlBits = [53]uint8{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		1, 1, 1, 1, 2, 2, 3, 3, 4, 4, 5, 7, 8, 9, 10, 11,
		12, 13, 14, 15, 16,
	}
	mlBase = [53]uint32{
		3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
		19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34,
		35, 37, 39, 41, 43, 47, 51, 59, 67, 83, 99, 131, 259, 515,
		1027, 2051, 4099, 8195, 16387, 32771, 65539,
	}
)
