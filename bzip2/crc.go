/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2

// bzip2's CRC-32: polynomial 0x04C11DB7 applied MSB-first (the
// bit-reverse of the usual hash/crc32 direction), init and final
// complement all-ones.  The initial value passed to updateCRC is 0
// (the complement wrap makes that the all-ones register).
var crctab [256]uint32

func init() {
	const poly = 0x04C11DB7
	for i := range crctab {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ poly
			} else {
				crc <<= 1
			}
		}
		crctab[i] = crc
	}
}

// updateCRC folds b into val (start from 0).
func updateCRC(val uint32, b []byte) uint32 {
	crc := ^val
	for _, v := range b {
		crc = crctab[byte(crc>>24)^v] ^ crc<<8
	}
	return ^crc
}

// combineCRC folds one block's CRC into the stream CRC:
// rotate-left one bit, then xor.
func combineCRC(file, block uint32) uint32 {
	return (file<<1 | file>>31) ^ block
}
