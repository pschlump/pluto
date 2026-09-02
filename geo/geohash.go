/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import "math"

// The bit planes of a score: latitude occupies the even positions
// (0,2,…,50), longitude the odd ones (1,3,…,51) — geohash.c's
// interleave64(lat, long) puts its first argument in the even positions.
const (
	evenBits uint64 = 0x5555555555555555 // latitude plane
	oddBits  uint64 = 0xaaaaaaaaaaaaaaaa // longitude plane
)

// interleave64 spreads the bits of xlo into the even positions of a
// uint64 and ylo into the odd ones (bit 0 of xlo → bit 0, bit 0 of ylo →
// bit 1, …), the classic bit-twiddling interleave geohash.c uses
// (graphics.stanford.edu bithacks, "InterleaveBMN").  x is the latitude
// index, y the longitude index.
func interleave64(xlo, ylo uint32) uint64 {
	x := uint64(xlo)
	y := uint64(ylo)

	x = (x | (x << 16)) & 0x0000FFFF0000FFFF
	y = (y | (y << 16)) & 0x0000FFFF0000FFFF

	x = (x | (x << 8)) & 0x00FF00FF00FF00FF
	y = (y | (y << 8)) & 0x00FF00FF00FF00FF

	x = (x | (x << 4)) & 0x0F0F0F0F0F0F0F0F
	y = (y | (y << 4)) & 0x0F0F0F0F0F0F0F0F

	x = (x | (x << 2)) & 0x3333333333333333
	y = (y | (y << 2)) & 0x3333333333333333

	x = (x | (x << 1)) & evenBits
	y = (y | (y << 1)) & evenBits

	return x | (y << 1)
}

// deinterleave64 reverses interleave64: the even-position bits gather
// into the low 32 bits of the result (the latitude index) and the
// odd-position bits into the high 32 (the longitude index).  The first
// step is a plain mask — the C's `x >> S[0]` shifts by zero, so x|x is
// just x — then the halving shifts run 1, 2, 4, 8, 16.
func deinterleave64(interleaved uint64) uint64 {
	x := interleaved
	y := interleaved >> 1

	x = (x | (x >> 0)) & evenBits
	y = (y | (y >> 0)) & evenBits

	x = (x | (x >> 1)) & 0x3333333333333333
	y = (y | (y >> 1)) & 0x3333333333333333

	x = (x | (x >> 2)) & 0x0F0F0F0F0F0F0F0F
	y = (y | (y >> 2)) & 0x0F0F0F0F0F0F0F0F

	x = (x | (x >> 4)) & 0x00FF00FF00FF00FF
	y = (y | (y >> 4)) & 0x00FF00FF00FF00FF

	x = (x | (x >> 8)) & 0x0000FFFF0000FFFF
	y = (y | (y >> 8)) & 0x0000FFFF0000FFFF

	x = (x | (x >> 16)) & 0x00000000FFFFFFFF
	y = (y | (y >> 16)) & 0x00000000FFFFFFFF

	return x | (y << 32)
}

// encodeStep is geohash.c's geohashEncode with the fixed WGS84 ranges:
// normalize lat/lon to [0,1) offsets, scale to the step's fixed point
// (truncating, like the C uint32_t conversion), interleave lat into the
// even bits and lon into the odd.  The caller has already validated the
// coordinates — with in-range input every intermediate is a non-negative
// float below 2^26, so the uint32 conversions are exact truncations.
// step must be in [1, 26].
func encodeStep(lat, lon float64, step int) uint64 {
	latOffset := (lat - LatMin) / (LatMax - LatMin)
	lonOffset := (lon - LonMin) / (LonMax - LonMin)

	scale := float64(uint64(1) << step)
	latOffset *= scale
	lonOffset *= scale

	return interleave64(uint32(latOffset), uint32(lonOffset))
}

// Encode returns the 52-bit interleaved geohash of (lat, lon) — exactly
// the score Redis's GEOADD stores for the position (geohashEncodeWGS84
// with step 26; the score needs no alignment because 26*2 == 52).  lat
// must be in [-85.05112878, 85.05112878] and lon in [-180, 180] with the
// bounds themselves legal, as in Redis; anything else, including NaN,
// returns ErrOutOfRange.
//
// Complexity is O(1).
func Encode(lat, lon float64) (uint64, error) {
	if err := checkCoord(lat, lon); err != nil {
		return 0, err
	}
	return encodeStep(lat, lon, StepBits), nil
}

// decodeArea returns the (latMin, latMax, lonMin, lonMax) edges of the
// cell hash names at the given step — geohash.c's geohashDecode with the
// fixed WGS84 ranges.  Each edge is written as an explicit fused
// multiply-add: min + frac·scale cancels ~7 digits for cells near the
// band's bottom, so whether the product rounds before the add changes
// the edge far beyond an ulp there.  The reference C is compiled with
// contraction on (clang's C default), i.e. fused — the explicit
// math.FMA pins exactly those semantics deterministically on every
// platform instead of relying on the compiler's optional contraction.
// The +1 wrap of an all-ones latitude index (only reachable from a hash
// that is not Encode's output) mirrors the C's uint32 arithmetic.
func decodeArea(hash uint64, step int) (latMin, latMax, lonMin, lonMax float64) {
	hashSep := deinterleave64(hash) // [lat low 32][lon high 32]

	latScale := LatMax - LatMin
	lonScale := LonMax - LonMin

	ilato := uint32(hashSep)
	ilono := uint32(hashSep >> 32)

	d := float64(uint64(1) << step)
	latMin = math.FMA(float64(ilato)/d, latScale, LatMin)
	latMax = math.FMA(float64(ilato+1)/d, latScale, LatMin)
	lonMin = math.FMA(float64(ilono)/d, lonScale, LonMin)
	lonMax = math.FMA(float64(ilono+1)/d, lonScale, LonMin)
	return latMin, latMax, lonMin, lonMax
}

// Decode returns the center of the cell the 52-bit hash names, the
// position Redis's GEOPOS reports for a stored score.  A hash above the
// legal space (any bit ≥ 52 set) decodes like in Redis: the stray bits
// land in the longitude index and the arithmetic simply proceeds.
//
// Complexity is O(1).
func Decode(hash uint64) (lat, lon float64) {
	latMin, latMax, lonMin, lonMax := decodeArea(hash, StepBits)

	lon = (lonMin + lonMax) / 2
	if lon > LonMax {
		lon = LonMax
	}
	if lon < LonMin {
		lon = LonMin
	}
	lat = (latMin + latMax) / 2
	if lat > LatMax {
		lat = LatMax
	}
	if lat < LatMin {
		lat = LatMin
	}
	return lat, lon
}

// moveX shifts the hash's longitude plane (odd bits) by d cells at the
// given step — geohash.c's geohash_move_x.  Note the C's crossed masks,
// kept exactly: the increment adds the OPPOSITE plane's mask (zz+1) so
// every carry out of one longitude bit lands on the next, and the final
// mask is this plane's own, which drops the carry out of the plane — so
// a move past the eastern edge wraps to the western edge of the world
// (and vice versa).
func moveX(hash uint64, d, step int) uint64 {
	if d == 0 {
		return hash
	}
	x := hash & oddBits
	y := hash & evenBits

	zz := evenBits >> (64 - step*2) // opposite plane: carry ladder
	if d > 0 {
		x += zz + 1
	} else {
		x |= zz
		x -= zz + 1
	}
	x &= oddBits >> (64 - step*2) // own plane: drop the wrap carry
	return x | y
}

// moveY shifts the hash's latitude plane (even bits) by d cells at the
// given step — geohash.c's geohash_move_y, with the same crossed-mask
// carry trick mirrored.  A move north past the top row wraps to the
// bottom row.  RangesForRadius and RangesForBox never scan a wrapped
// latitude neighbor (their coverage test sees to that, or returns the
// full score space), but Neighbors reports it, matching Redis.
func moveY(hash uint64, d, step int) uint64 {
	if d == 0 {
		return hash
	}
	x := hash & oddBits
	y := hash & evenBits

	zz := oddBits >> (64 - step*2) // opposite plane: carry ladder
	if d > 0 {
		y += zz + 1
	} else {
		y |= zz
		y -= zz + 1
	}
	y &= evenBits >> (64 - step*2) // own plane: drop the wrap carry
	return x | y
}

// Neighbors returns the 8 cells adjacent to hash at the same 52-bit
// precision, in the order N, NE, E, SE, S, SW, W, NW.  Both axes wrap
// around the world, matching geohash.c's geohashNeighbors: the western
// neighbor of a cell at the antimeridian is on the far side of +180
// (which is what makes antimeridian-crossing radius searches complete),
// and a northern neighbor of the top row wraps to the bottom row.
//
// Complexity is O(1).
func Neighbors(hash uint64) [8]uint64 {
	n := moveY(hash, 1, StepBits)
	s := moveY(hash, -1, StepBits)
	return [8]uint64{
		n,                         // N
		moveX(n, 1, StepBits),     // NE
		moveX(hash, 1, StepBits),  // E
		moveX(s, 1, StepBits),     // SE
		s,                         // S
		moveX(s, -1, StepBits),    // SW
		moveX(hash, -1, StepBits), // W
		moveX(n, -1, StepBits),    // NW
	}
}
