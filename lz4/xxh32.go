/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4

// XXH32 — the checksum the LZ4 frame format uses for its header,
// block and content checksums (xxHash32, seed 0, little-endian).
// This is a direct implementation of the reference algorithm; the
// LZ4 CLI's own checksums are the cross-check (a frame with a wrong
// XXH32 here does not decompress under lz4(1), and its frames do
// not decompress here).

const (
	xxp1 = 2654435761
	xxp2 = 2246822519
	xxp3 = 3266489917
	xxp4 = 668265263
	xxp5 = 374761393
)

func rotl32(v uint32, n uint) uint32 { return v<<n | v>>(32-n) }

func xxhRound(acc, val uint32) uint32 {
	acc += val * xxp2
	acc = rotl32(acc, 13)
	acc *= xxp1
	return acc
}

// xxh32 computes XXH32 with seed 0.
func xxh32(b []byte) uint32 {
	var p1, p2, p3, p4, p5 uint32 = xxp1, xxp2, xxp3, xxp4, xxp5
	n := len(b)
	var h uint32
	if n < 16 {
		h = uint32(n) + p5
		i := 0
		for ; i+4 <= n; i += 4 {
			h += readLE32(b[i:]) * p3
			h = rotl32(h, 17) * p4
		}
		for ; i < n; i++ {
			h += uint32(b[i]) * p5
			h = rotl32(h, 11) * p1
		}
	} else {
		acc1, acc2, acc3, acc4 := p1+p2, p2, uint32(0), uint32(0)-p1
		for i := 0; i+16 <= n; i += 16 {
			acc1 = xxhRound(acc1, readLE32(b[i:]))
			acc2 = xxhRound(acc2, readLE32(b[i+4:]))
			acc3 = xxhRound(acc3, readLE32(b[i+8:]))
			acc4 = xxhRound(acc4, readLE32(b[i+12:]))
		}
		h = rotl32(acc1, 1) + rotl32(acc2, 7) + rotl32(acc3, 12) + rotl32(acc4, 18) + uint32(n)
		i := n &^ 15
		for ; i+4 <= n; i += 4 {
			h += readLE32(b[i:]) * p3
			h = rotl32(h, 17) * p4
		}
		for ; i < n; i++ {
			h += uint32(b[i]) * p5
			h = rotl32(h, 11) * p1
		}
	}
	h ^= h >> 15
	h *= p2
	h ^= h >> 13
	h *= p3
	h ^= h >> 16
	return h
}

func readLE32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func appendLE32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
