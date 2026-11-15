/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

// Huffman-coded literals: the weight-based tree description (direct
// 4-bit weights, or FSE-coded weights with two interleaved states),
// the weights-to-table conversion, and backward stream decode with
// either one stream or four (jump table + even split).

const maxHuffLog = 11

type huffTable struct {
	log  uint8
	sym  [1 << maxHuffLog]byte
	bits [1 << maxHuffLog]uint8
}

// readHuffWeights decodes a Huffman tree description: a header byte
// that selects direct 4-bit weights (header ≥ 128, count = byte−127)
// or an FSE-compressed weight stream (header = its length).  The
// final symbol's weight is implied by completing to the next power
// of two.  It returns the weights of symbols 0..n-1 and the bytes
// the description occupies.
func readHuffWeights(b []byte) ([]byte, int, error) {
	if len(b) == 0 {
		return nil, 0, errCorrupt
	}
	hb := int(b[0])
	var w []byte
	if hb >= 128 {
		n := hb - 127
		if 1+(n+1)/2 > len(b) {
			return nil, 0, errCorrupt
		}
		for i := 0; i < n; i += 2 {
			w = append(w, b[1+i/2]>>4)
			if i+1 < n {
				w = append(w, b[1+i/2]&0xF)
			}
		}
		return w, 1 + (n+1)/2, nil
	}
	// FSE-coded weights: hb bytes total — a table description followed
	// by a backward stream with two interleaved states sharing it.
	if hb > len(b)-1 {
		return nil, 0, errCorrupt
	}
	norm, log, used, err := readNCount(b[1:1+hb], 6, 255)
	if err != nil || used >= hb {
		return nil, 0, errCorrupt
	}
	t, err := buildFSE(norm, log)
	if err != nil {
		return nil, 0, err
	}
	stream := b[1+used : 1+hb]
	r, ok := newBitReader(stream)
	if !ok {
		return nil, 0, errCorrupt
	}
	s1 := uint16(r.read(int(log)))
	s2 := uint16(r.read(int(log)))
	step := func(state uint16) uint16 {
		return t.base[state] + uint16(r.read(int(t.bits[state])))
	}
	for len(w) <= 255 {
		w = append(w, t.sym[s1])
		s1 = step(s1)
		if r.over {
			w = append(w, t.sym[s2])
			break
		}
		w = append(w, t.sym[s2])
		s2 = step(s2)
		if r.over {
			w = append(w, t.sym[s1])
			break
		}
	}
	if len(w) > 255 || r.pos != 0 || r.n != 0 {
		return nil, 0, errCorrupt
	}
	return w, 1 + hb, nil
}

// buildHuff turns weights into a flat decode table indexed by the
// next tableLog bits: canonical codes, symbols sorted by ascending
// weight (longest codes first), natural order within a weight.
func buildHuff(w []byte) (*huffTable, error) {
	// The last symbol's weight is implied: the decoded weights sum
	// (in shares of 2^(weight-1)) to a value completed by the next
	// power of two.
	var shares uint32
	for _, x := range w {
		if x > maxHuffLog {
			return nil, errCorrupt
		}
		if x > 0 {
			shares += 1 << (x - 1)
		}
	}
	if shares == 0 {
		return nil, errCorrupt
	}
	log := uint8(highbit32(shares) + 1)
	if log > maxHuffLog {
		return nil, errCorrupt
	}
	total := uint32(1) << log
	if shares >= total {
		return nil, errCorrupt
	}
	rest := total - shares
	if rest&(rest-1) != 0 {
		return nil, errCorrupt // the completion must be a clean power of two
	}
	last := byte(highbit32(rest) + 1)
	w = append(w, last)

	// A complete tree needs an even count of at least two symbols at
	// the longest code length (weight 1).
	var ones int
	for _, x := range w {
		if x == 1 {
			ones++
		}
	}
	if ones < 2 || ones%2 != 0 {
		return nil, errCorrupt
	}

	// Symbols by ascending weight, natural order within a weight;
	// codes are distributed in that order, counting up through the
	// table-width space (canonical, MSB-first).
	sorted := make([]int, 0, len(w))
	for x := byte(1); x <= log; x++ {
		for s, wx := range w {
			if wx == x {
				sorted = append(sorted, s)
			}
		}
	}
	t := &huffTable{log: log}
	var val uint32
	for _, s := range sorted {
		nb := int(log) + 1 - int(w[s])
		width := uint32(1) << uint(int(log)-nb)
		base := val &^ (width - 1) // code = val >> shift, placed at the top of its run
		for i := uint32(0); i < width; i++ {
			t.sym[base+i] = byte(s)
			t.bits[base+i] = uint8(nb)
		}
		val += width
	}
	if val != total {
		return nil, errCorrupt
	}
	return t, nil
}
