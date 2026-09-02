/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package geo is the pure geohash math behind the Redis GEO command family
// (note/redis/src/geohash.c, geohash_helper.c and geo.c lineage; request
// note/08-geo.md, built for Ultima's GEOADD/GEODIST/GEOHASH/GEOPOS/
// GEOSEARCH/GEOSEARCHSTORE and the legacy GEORADIUS).  Redis stores each
// member of a GEO key in a sorted set whose score is a 52-bit interleaved
// geohash of its position; everything here is the stateless math that
// produces, decodes and range-queries those scores:
//
//	Encode — (lat, lon) → the 52-bit score Redis's GEOADD stores.	O(1)
//	Decode — score → the center (lat, lon) of its cell (GEOPOS).	O(1)
//	Neighbors — the 8 adjacent cells N, NE, E, SE, S, SW, W, NW.		O(1)
//	RangesForRadius — score ranges covering a circle (radius search).	O(1)
//	RangesForBox — score ranges covering a rectangle (BYBOX search).	O(1)
//	Distance — haversine great-circle distance in meters (GEODIST).	O(1)
//	ToMeters / FromMeters — m, km, ft, mi unit conversion.				O(1)
//
// Every function is bit-compatible with the Redis C code it mirrors.
// Encode/Decode round-trip exactly against both a live Redis server
// (v7.2.7, GEOADD+ZSCORE / GEOPOS capture) and the note's C sources
// compiled as a harness (v8.9.241) — the two agree with each other on
// every shared vector, and both vector sets are baked into the tests.
//
// The score layout, from the C interleave (geohash.c): latitude bits sit
// in the even bit positions 0,2,…,50 and longitude bits in the odd
// positions 1,3,…,51, so read MSB-first the score alternates lon,lat,lon,…
// starting with longitude's most significant bit — the standard geohash
// bit order.  The latitude range is the Mercator projection bound
// [-85.05112878, 85.05112878] (EPSG:900913; the poles are not encodable),
// longitude is [-180, 180].
//
// The package is pure functions over floats and integers — no state, no
// type parameters (nothing is stored, the union_find/suffix_array
// precedent), and therefore no _ts twin (the substring/quick_sort/lzw/
// maxflow precedent): everything is safe for concurrent use as-is.  Data
// errors are sentinel errors compared with errors.Is; no function panics,
// ever.
//
// Search contract.  RangesForRadius and RangesForBox return a
// conservative-complete SUPERSET of the query shape: the union of the
// half-open score ranges [lo, hi) covers every cell the shape can touch
// (the caller looks up [lo, hi) in the sorted set), and the caller then
// post-filters the candidates exactly — with Distance for a circle, or
// the two-Distance rectangle rule documented on RangesForBox.  False
// positives are expected and fine; false negatives cannot happen.
package geo

import (
	"errors"
	"math"
	"strings"
)

// StepBits is the geohash precision Redis's GEO commands use:
// GEO_STEP_MAX, 26 bits per axis, interleaved into a 2*26 = 52-bit score.
const StepBits = 26

// ScoreBits is the total score width in bits (2*StepBits); scores live in
// [0, 1<<ScoreBits).
const ScoreBits = 52

// LatMin and LatMax bound encodable latitude — the Web-Mercator latitude
// range (EPSG:900913 et al.), not ±90: the poles themselves are not
// encodable, matching Redis's GEO_LAT_MIN/GEO_LAT_MAX.
const (
	LatMin = -85.05112878
	LatMax = 85.05112878
)

// LonMin and LonMax bound encodable longitude.
const (
	LonMin = -180.0
	LonMax = 180.0
)

// EarthRadiusInMeters is the radius geohash_helper.c uses for every
// distance and bounding-box computation: Earth's quadratic mean radius
// for WGS-84 (not the 6371000 m sphere of many textbooks).
const EarthRadiusInMeters = 6372797.560856

// ErrOutOfRange reports a latitude outside [-85.05112878, 85.05112878] or
// a longitude outside [-180, 180] (or either being NaN) — what Redis's
// GEOADD rejects with "invalid longitude,latitude pair".  Exactly the
// boundary values remain legal, as in Redis.
var ErrOutOfRange = errors.New("geo: latitude must be in [-85.05112878, 85.05112878] and longitude in [-180, 180]")

// ErrNegativeRadius reports a negative (or NaN) radius, box width or box
// height — Redis rejects these in the command layer ("radius cannot be
// negative", "height or width cannot be negative").  Zero is legal and
// selects the tightest precision.
var ErrNegativeRadius = errors.New("geo: radius, width and height cannot be negative")

// ErrInvalidUnit reports a unit other than m, km, ft or mi — Redis's
// "unsupported unit provided. please use M, KM, FT, MI".
var ErrInvalidUnit = errors.New("geo: unsupported unit, use m, km, ft or mi")

// unitFactor is the meters-per-unit table from geo.c's extractUnitOrReply.
// Note the mile factor is Redis's 1609.34 (not the 1609.344 survey mile);
// ported exactly so distances match Redis bit-for-bit.
func unitFactor(unit string) (float64, bool) {
	switch {
	case strings.EqualFold(unit, "m"):
		return 1, true
	case strings.EqualFold(unit, "km"):
		return 1000, true
	case strings.EqualFold(unit, "ft"):
		return 0.3048, true
	case strings.EqualFold(unit, "mi"):
		return 1609.34, true
	}
	return 0, false
}

// ToMeters converts v in the given unit to meters.  The unit is one of
// m, km, ft, mi, case-insensitive (Redis accepts M, Km, …); any other
// string returns ErrInvalidUnit.
//
// Complexity is O(1).
func ToMeters(v float64, unit string) (float64, error) {
	f, ok := unitFactor(unit)
	if !ok {
		return 0, ErrInvalidUnit
	}
	return v * f, nil
}

// FromMeters converts v in meters to the given unit, the inverse of
// ToMeters.  The unit is one of m, km, ft, mi, case-insensitive; any
// other string returns ErrInvalidUnit.
//
// Complexity is O(1).
func FromMeters(v float64, unit string) (float64, error) {
	f, ok := unitFactor(unit)
	if !ok {
		return 0, ErrInvalidUnit
	}
	return v / f, nil
}

// checkCoord validates a (lat, lon) pair, the shared guard of Encode and
// the range queries.  NaN needs its own test — every comparison against a
// NaN is false, so the range checks alone would let it through (the C
// code has this hole and lands in undefined behavior; pluto rejects it).
func checkCoord(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsNaN(lon) ||
		lat < LatMin || lat > LatMax || lon < LonMin || lon > LonMax {
		return ErrOutOfRange
	}
	return nil
}
