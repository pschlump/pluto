/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

// XXH64, the frame content checksum's hash (low 32 bits of the
// digest).  Streaming form: bytes buffer until a 32-byte stripe, the
// four-lane round runs, and Sum finishes the tail.  Reference:
// xxHash specification (XXH64, seed 0).

const (
	xxP1 uint64 = 11400714785074694791
	xxP2 uint64 = 14029467366897019727
	xxP3 uint64 = 1609587929392839161
	xxP4 uint64 = 9650029242287828579
	xxP5 uint64 = 2870177450012600261
)

type xxh64 struct {
	v     [4]uint64
	mem   [32]byte
	n     int
	total uint64
}

func newXXH64() *xxh64 {
	x := &xxh64{}
	x.v[0] = xxP1
	x.v[0] += xxP2 // runtime wrap; constant arithmetic would overflow
	x.v[1] = xxP2
	x.v[2] = 0
	x.v[3] = 0
	x.v[3] -= xxP1
	return x
}

func (x *xxh64) write(p []byte) {
	x.total += uint64(len(p))
	if x.n > 0 {
		k := copy(x.mem[x.n:], p)
		x.n += k
		p = p[k:]
		if x.n < 32 {
			return
		}
		x.round(&x.v, x.mem[:])
		x.n = 0
	}
	for len(p) >= 32 {
		x.round(&x.v, p)
		p = p[32:]
	}
	x.n = copy(x.mem[:], p)
}

func (x *xxh64) round(v *[4]uint64, p []byte) {
	for i := 0; i < 4; i++ {
		k := le64(p[i*8:])
		v[i] = rotl64(v[i]+k*xxP2, 31) * xxP1
	}
}

func (x *xxh64) sum() uint64 {
	var h uint64
	if x.total >= 32 {
		h = rotl64(x.v[0], 1) + rotl64(x.v[1], 7) + rotl64(x.v[2], 12) + rotl64(x.v[3], 18)
		for i := 0; i < 4; i++ {
			h = mergeRound(h, x.v[i])
		}
	} else {
		h = xxP5
	}
	h += x.total
	p := x.mem[:x.n]
	for len(p) >= 8 {
		h ^= rotl64(le64(p)*xxP2, 31) * xxP1
		h = rotl64(h, 27)*xxP1 + xxP4
		p = p[8:]
	}
	if len(p) >= 4 {
		h ^= uint64(le32(p)) * xxP1
		h = rotl64(h, 23)*xxP2 + xxP3
		p = p[4:]
	}
	for _, b := range p {
		h ^= uint64(b) * xxP5
		h = rotl64(h, 11) * xxP1
	}
	h ^= h >> 33
	h *= xxP2
	h ^= h >> 29
	h *= xxP3
	h ^= h >> 32
	return h
}

func mergeRound(h, v uint64) uint64 {
	v *= xxP2
	v = rotl64(v, 31)
	v *= xxP1
	h ^= v
	return h*xxP1 + xxP4
}

func le64(b []byte) uint64 {
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func rotl64(v uint64, n uint) uint64 { return v<<n | v>>(64-n) }
