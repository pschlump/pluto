/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

// Compressed-block decode: the literals section (raw / RLE / Huffman
// / treeless), the sequences section (FSE tables for the three code
// types, the interleaved bitstream), and sequence execution with the
// repeat-offset history.

// literals decodes the literals section at the head of a compressed
// block, returning the regenerated literals and the section's size on
// disk.  A Huffman table read here updates d.huff for later Treeless
// blocks.
func (d *decoder) literals(b []byte) ([]byte, int, error) {
	if len(b) == 0 {
		return nil, 0, errCorrupt
	}
	typ := b[0] & 3
	sf := (b[0] >> 2) & 3
	switch typ {
	case 0, 1: // Raw / RLE
		var size, hdr int
		switch sf {
		case 0, 2:
			size, hdr = int(b[0]>>3), 1
		case 1:
			if len(b) < 2 {
				return nil, 0, errCorrupt
			}
			size, hdr = int(b[0])>>4+int(b[1])<<4, 2
		default:
			if len(b) < 3 {
				return nil, 0, errCorrupt
			}
			size, hdr = int(b[0])>>4+int(b[1])<<4+int(b[2])<<12, 3
		}
		if size > maxBlockSize {
			return nil, 0, errCorrupt
		}
		if typ == 0 {
			if hdr+size > len(b) {
				return nil, 0, errCorrupt
			}
			return b[hdr : hdr+size], hdr + size, nil
		}
		if hdr+1 > len(b) {
			return nil, 0, errCorrupt
		}
		lit := make([]byte, size)
		for i := range lit {
			lit[i] = b[hdr]
		}
		return lit, hdr + 1, nil
	}
	// Compressed (2) or Treeless (3): 2-2-(10|14|18) sizes.
	var lhSize int
	var regen, csize int
	switch sf {
	case 0, 1:
		lhSize = 3
	case 2:
		lhSize = 4
	default:
		lhSize = 5
	}
	if len(b) < lhSize {
		return nil, 0, errCorrupt
	}
	v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16
	if lhSize > 3 {
		v |= uint64(b[3]) << 24
	}
	if lhSize > 4 {
		v |= uint64(b[4]) << 32
	}
	switch sf {
	case 0, 1:
		regen = int(v>>4) & 0x3FF
		csize = int(v>>14) & 0x3FF
	case 2:
		regen = int(v>>4) & 0x3FFF
		csize = int(v>>18) & 0x3FFF
	default:
		regen = int(v>>4) & 0x3FFFF
		csize = int(v>>22) & 0x3FFFF
	}
	if regen > maxBlockSize || csize < 1 || lhSize+csize > len(b) {
		return nil, 0, errCorrupt
	}
	body := b[lhSize : lhSize+csize]
	streams := body
	if typ == 2 {
		w, used, err := readHuffWeights(body)
		if err != nil {
			return nil, 0, err
		}
		h, err := buildHuff(w)
		if err != nil {
			return nil, 0, err
		}
		d.huff, d.huffOK = h, true
		streams = body[used:]
	} else if !d.huffOK {
		return nil, 0, errCorrupt // treeless without a table to repeat
	}
	lit := make([]byte, 0, regen)
	if sf == 0 { // single stream
		r, ok := newBitReader(streams)
		if !ok {
			return nil, 0, errCorrupt
		}
		if err := d.huff.decode(r, regen, &lit); err != nil {
			return nil, 0, err
		}
		return lit, lhSize + csize, nil
	}
	// Four streams: a 6-byte jump table, then even splits.
	if len(streams) < 6 {
		return nil, 0, errCorrupt
	}
	s1 := int(streams[0]) | int(streams[1])<<8
	s2 := int(streams[2]) | int(streams[3])<<8
	s3 := int(streams[4]) | int(streams[5])<<8
	if 6+s1+s2+s3 > len(streams) {
		return nil, 0, errCorrupt
	}
	s4 := len(streams) - 6 - s1 - s2 - s3
	seg := (regen + 3) >> 2
	start := 6
	for i, sz := range []int{s1, s2, s3, s4} {
		n := seg
		if i == 3 {
			n = regen - 3*seg
		}
		if n < 0 {
			return nil, 0, errCorrupt
		}
		r, ok := newBitReader(streams[start : start+sz])
		if !ok {
			return nil, 0, errCorrupt
		}
		if err := d.huff.decode(r, n, &lit); err != nil {
			return nil, 0, err
		}
		start += sz
	}
	return lit, lhSize + csize, nil
}

// decode appends exactly n symbols from one Huffman stream, which
// must then be exactly consumed.
func (t *huffTable) decode(r *bitReader, n int, out *[]byte) error {
	for i := 0; i < n; i++ {
		u := r.peek(int(t.log))
		*out = append(*out, t.sym[u])
		r.skip(int(t.bits[u]))
	}
	if r.over || !r.done() {
		return errCorrupt
	}
	return nil
}

// readFSE reads one sequence table description for the given mode
// (0 predefined, 1 RLE, 2 FSE, 3 repeat), updating d's table.
func (d *decoder) readFSE(b []byte, mode int, pre *fseTable, maxLog, maxSym int, ok *bool, t *fseTable) (int, error) {
	switch mode {
	case 0:
		*t = *pre
		*ok = true
		return 0, nil
	case 1:
		if len(b) == 0 || int(b[0]) > maxSym {
			return 0, errCorrupt
		}
		*t = rleFSE(b[0])
		*ok = true
		return 1, nil
	case 2:
		norm, log, used, err := readNCount(b, maxLog, maxSym)
		if err != nil {
			return 0, err
		}
		nt, err := buildFSE(norm, log)
		if err != nil {
			return 0, err
		}
		*t = nt
		*ok = true
		return used, nil
	default:
		if !*ok {
			return 0, errCorrupt // repeat without a previous table
		}
		return 0, nil
	}
}

// block decodes one compressed block body (b), appending at most
// budget bytes (the block maximum) to d.out.  A compressed block's
// regenerated size is not stated anywhere: it is whatever the
// sequences produce.
func (d *decoder) block(b []byte, budget int) error {
	end := len(d.out) + budget
	lit, used, err := d.literals(b)
	if err != nil {
		return err
	}
	seq := b[used:]
	if len(seq) == 0 {
		return errCorrupt
	}
	// Number of sequences.
	nbSeq := int(seq[0])
	sp := 1
	switch {
	case nbSeq == 0:
		if len(lit) > budget {
			return errCorrupt
		}
		d.out = append(d.out, lit...)
		return nil
	case nbSeq < 128:
	default:
		if nbSeq < 255 {
			if len(seq) < 2 {
				return errCorrupt
			}
			nbSeq = ((nbSeq - 128) << 8) + int(seq[1])
			sp = 2
		} else {
			if len(seq) < 3 {
				return errCorrupt
			}
			nbSeq = int(seq[1]) + int(seq[2])<<8 + 0x7F00
			sp = 3
		}
	}
	seq = seq[sp:]
	if len(seq) == 0 {
		return errCorrupt
	}
	modes := seq[0]
	if modes&3 != 0 {
		return errCorrupt // reserved bits must be zero
	}
	seq = seq[1:]
	if seq, err = d.readFSESeqTables(seq, int(modes>>6&3), int(modes>>4&3), int(modes>>2&3)); err != nil {
		return err
	}
	return d.sequences(seq, nbSeq, lit, end)
}

// readFSESeqTables consumes the three table descriptions, in
// literals-length, offset, match-length order.
func (d *decoder) readFSESeqTables(seq []byte, llMode, ofMode, mlMode int) ([]byte, error) {
	n, err := d.readFSE(seq, llMode, &llPredef, 9, 35, &d.llOK, &d.fseLL)
	if err != nil {
		return nil, err
	}
	seq = seq[n:]
	n, err = d.readFSE(seq, ofMode, &ofPredef, 8, 31, &d.ofOK, &d.fseOF)
	if err != nil {
		return nil, err
	}
	seq = seq[n:]
	n, err = d.readFSE(seq, mlMode, &mlPredef, 9, 52, &d.mlOK, &d.fseML)
	if err != nil {
		return nil, err
	}
	return seq[n:], nil
}

// sequences decodes and executes nbSeq sequences from the interleaved
// FSE bitstream, appending literals and matches to d.out until the
// block reaches its end offset.
func (d *decoder) sequences(b []byte, nbSeq int, lit []byte, end int) error {
	if len(b) == 0 {
		return errCorrupt
	}
	r, ok := newBitReader(b)
	if !ok {
		return errCorrupt
	}
	next := func(t *fseTable, state uint16) uint16 {
		return t.base[state] + uint16(r.read(int(t.bits[state])))
	}
	ll := uint16(r.read(int(d.fseLL.log)))
	of := uint16(r.read(int(d.fseOF.log)))
	ml := uint16(r.read(int(d.fseML.log)))
	litPos := 0
	for i := 0; i < nbSeq; i++ {
		llCode := d.fseLL.sym[ll]
		ofCode := d.fseOF.sym[of]
		mlCode := d.fseML.sym[ml]
		// Extra bits: offset first, then match length, then literals
		// length (RFC 8878 §3.1.1.3.2.1.2).
		offValue := uint64(1)<<uint(ofCode) + uint64(r.read(int(ofCode)))
		matchLen := uint64(mlBase[mlCode]) + uint64(r.read(int(mlBits[mlCode])))
		litLen := uint64(llBase[llCode]) + uint64(r.read(int(llBits[llCode])))
		if i < nbSeq-1 {
			// State updates — literals length, match length, offset —
			// are skipped for the last sequence.
			ll = next(&d.fseLL, ll)
			ml = next(&d.fseML, ml)
			of = next(&d.fseOF, of)
		}
		if litPos+int(litLen) > len(lit) || matchLen > uint64(end) {
			return errCorrupt
		}
		d.out = append(d.out, lit[litPos:litPos+int(litLen)]...)
		litPos += int(litLen)
		if len(d.out)+int(matchLen) > end {
			return errCorrupt
		}
		offset, ok := d.resolveOffset(offValue, litLen > 0)
		if !ok || offset == 0 || offset > uint64(len(d.out)) {
			return errCorrupt
		}
		src := len(d.out) - int(offset)
		for j := 0; j < int(matchLen); j++ {
			d.out = append(d.out, d.out[src+j])
		}
	}
	if r.over || !r.done() {
		return errCorrupt
	}
	d.out = append(d.out, lit[litPos:]...) // leftover literals
	if len(d.out) > end {
		return errCorrupt
	}
	return nil
}

// resolveOffset maps an offset_value (1-3 are repeat codes, with the
// zero-literals shift) to the match offset, maintaining the repeat
// history per RFC 8878 §3.1.1.5.
func (d *decoder) resolveOffset(v uint64, hasLits bool) (uint64, bool) {
	switch {
	case v > 3:
		off := v - 3
		d.rep[2], d.rep[1], d.rep[0] = d.rep[1], d.rep[0], off
		return off, true
	case v == 3 && !hasLits:
		off := d.rep[0] - 1
		if off == 0 {
			return 0, false
		}
		d.rep[2], d.rep[1], d.rep[0] = d.rep[1], d.rep[0], off
		return off, true
	}
	idx := int(v - 1)
	if !hasLits {
		idx++ // no literals: repeat codes shift by one
	}
	off := d.rep[idx]
	switch idx {
	case 1:
		d.rep[1], d.rep[0] = d.rep[0], off
	case 2:
		d.rep[2], d.rep[1], d.rep[0] = d.rep[1], d.rep[0], off
	}
	return off, true
}
