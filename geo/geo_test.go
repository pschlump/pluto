/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import (
	"math"
	"slices"
	"testing"
)

// covered reports whether the half-open score value v lies inside one of
// the ranges.
func covered(v uint64, ranges [][2]uint64) bool {
	for _, r := range ranges {
		if v >= r[0] && v < r[1] {
			return true
		}
	}
	return false
}

// checkCanonical asserts the ranges' own contract: sorted ascending,
// disjoint, non-degenerate, inside the score space (which includes the
// spill-bit corner cells up to scoreSpaceTop).
func checkCanonical(t *testing.T, ranges [][2]uint64) {
	t.Helper()
	for i, r := range ranges {
		if r[0] >= r[1] {
			t.Fatalf("range %d degenerate: %v", i, r)
		}
		if r[1] > scoreSpaceTop {
			t.Fatalf("range %d outside score space: %v", i, r)
		}
		if i > 0 && r[0] < ranges[i-1][1] {
			t.Fatalf("ranges %d,%v and %d,%v overlap or are unsorted", i-1, ranges[i-1], i, r)
		}
	}
}

// TestRedisVectors pins Encode and Decode against a live Redis server:
// GEOADD stored each position's score in a real zset, ZSCORE read it
// back, GEOPOS reported the decoded cell center.  The scores compare
// exactly; GEOPOS prints only ~12 significant digits, so its positions
// compare with a tolerance far below one cell (a step-26 cell is
// ~2.5e-6 degrees tall).  Re-encoding the printed center must land back
// in the same cell — the round-trip property the note requires.
func TestRedisVectors(t *testing.T) {
	for i, v := range redisVectors {
		score, err := Encode(v.Lat, v.Lon)
		if err != nil {
			t.Fatalf("row %d (%v,%v): %v", i, v.Lat, v.Lon, err)
		}
		if score != v.Score {
			t.Fatalf("row %d (%v,%v): Encode = %d, live Redis stored %d", i, v.Lat, v.Lon, score, v.Score)
		}

		lat, lon := Decode(v.Score)
		if math.Abs(lat-v.WantLat) > 1e-11 || math.Abs(lon-v.WantLon) > 1e-11 {
			t.Fatalf("row %d: Decode(%d) = (%v,%v), GEOPOS replied (%v,%v)", i, v.Score, lat, lon, v.WantLat, v.WantLon)
		}

		if again, _ := Encode(v.WantLat, v.WantLon); again != v.Score {
			t.Fatalf("row %d: re-encode of GEOPOS reply = %d, want %d", i, again, v.Score)
		}
	}
}

// TestEncodeCVectors pins the step-parameterized encoder for every step
// 1..26 against the compiled Redis sources — the public Encode is the
// step-26 case, RangesForRadius/RangesForBox use the rest.
func TestEncodeCVectors(t *testing.T) {
	for i, v := range cfEncode {
		got := encodeStep(v.Lat, v.Lon, v.Step)
		if got != v.Want {
			t.Fatalf("row %d (%v,%v) step %d: got %d want %d", i, v.Lat, v.Lon, v.Step, got, v.Want)
		}
		if v.Step == StepBits {
			if pub, err := Encode(v.Lat, v.Lon); err != nil || pub != v.Want {
				t.Fatalf("row %d: Encode = %d (%v), want %d", i, pub, err, v.Want)
			}
		}
	}
}

// TestDecodeCVectors pins Decode bit-for-bit (%.17g round-trip) against
// the compiled Redis decode, including hand-picked hashes (0, 1, the
// top-of-band latitude, the all-ones score).
func TestDecodeCVectors(t *testing.T) {
	for i, v := range cfDecode {
		lat, lon := Decode(v.Hash)
		if lat != v.WantLat || lon != v.WantLon {
			t.Fatalf("row %d: Decode(%d) = (%.17g,%.17g), C gives (%.17g,%.17g)", i, v.Hash, lat, lon, v.WantLat, v.WantLon)
		}
	}
}

// TestNeighborsCVectors pins Neighbors (order N, NE, E, SE, S, SW, W, NW)
// against geohashNeighbors, antimeridian and band-edge points included.
func TestNeighborsCVectors(t *testing.T) {
	for i, v := range cfNeighbors {
		got := Neighbors(v.Hash)
		if got != v.Want {
			t.Fatalf("row %d: Neighbors(%d) = %v, C gives %v", i, v.Hash, got, v.Want)
		}
	}
}

// TestStepsCVectors pins the radius→step estimator across the doubling
// sweep, the polar adjustments (66°, 80°) and the MERCATOR_MAX clamp.
func TestStepsCVectors(t *testing.T) {
	for i, v := range cfSteps {
		if got := estimateStepsByRadius(v.Radius, v.Lat); got != v.Want {
			t.Fatalf("row %d (r=%v, lat=%v): got step %d want %d", i, v.Radius, v.Lat, got, v.Want)
		}
	}
}

// mergedRanges canonicalizes the C harness's per-box range list (printed
// in geo.c's iteration order, duplicates unmerged) into pluto's
// sorted-disjoint form so the two compare as unions.
func mergedRanges(ranges [][2]uint64) [][2]uint64 {
	out := slices.Clone(ranges)
	slices.SortFunc(out, func(a, b [2]uint64) int {
		switch {
		case a[0] < b[0]:
			return -1
		case a[0] > b[0]:
			return 1
		}
		return 0
	})
	merged := out[:0]
	for _, r := range out {
		if n := len(merged); n > 0 && r[0] <= merged[n-1][1] {
			if r[1] > merged[n-1][1] {
				merged[n-1][1] = r[1]
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// unionContains reports whether every interval of b lies inside the
// union of a (both sorted and disjoint).
func unionContains(a, b [][2]uint64) bool {
	j := 0
	for _, iv := range b {
		for j < len(a) && a[j][1] <= iv[0] {
			j++
		}
		if j >= len(a) || a[j][0] > iv[0] || a[j][1] < iv[1] {
			return false
		}
	}
	return true
}

// checkRangesAgainstC runs the shared vector assertions for one query:
// the result is canonical, it contains the query point's own cell, and
// its window relates to the C's window — equal, or one containing the
// other, or (rarely, where pluto's clamped-latitude pruning splits a
// row the C keeps whole) simply incomparable.  pluto and the C share the
// step estimator and the pruning rules but not the coverage test: pluto
// keeps the estimated step where its arc-correct test passes and the
// C's plain comparison does not (finer window), and walks the step down
// where one decrement is not enough or the shape defeats every window
// (coarser window or the full space).  The counts logged by the callers
// show the mix; completeness itself is pinned by the fuzz and the
// live-Redis GEOSEARCH differential.
func checkRangesAgainstC(t *testing.T, got [][2]uint64, cRanges [][2]uint64, lat, lon float64) (classification string) {
	t.Helper()
	checkCanonical(t, got)
	c := mergedRanges(cRanges)

	// The query point's own stored score must be inside both windows
	// (a member at the query position is inside every search of either
	// implementation).
	score := encodeStep(lat, lon, StepBits)
	if !covered(score, got) {
		t.Fatalf("pluto window %v does not cover the query's own score %d", got, score)
	}
	if !covered(score, c) {
		t.Fatalf("C window %v does not cover the query's own score %d", c, score)
	}

	switch {
	case slices.Equal(got, c):
		return "equal"
	case unionContains(got, c):
		return "pluto-wider"
	case unionContains(c, got):
		return "c-wider"
	}
	return "incomparable"
}

// TestRangesRadiusCVectors cross-checks RangesForRadius against the
// compiled geohashCalculateAreasByShapeWGS84 + scoresOfGeoHashBox for
// fixed cities, the four band corners, antimeridian huggers, polar
// latitudes and random centers with log-uniform radii.
func TestRangesRadiusCVectors(t *testing.T) {
	counts := map[string]int{}
	for i, v := range cfRangesRadius {
		got, err := RangesForRadius(v.Lat, v.Lon, v.Radius)
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		counts[checkRangesAgainstC(t, got, v.Ranges, v.Lat, v.Lon)]++
	}
	t.Logf("vs compiled C over %d radius vectors: %v", len(cfRangesRadius), counts)
}

// TestRangesBoxCVectors is TestRangesRadiusCVectors for the GEOSEARCH
// BYBOX rectangle, from 1 m squares up to world-spanning boxes.
func TestRangesBoxCVectors(t *testing.T) {
	counts := map[string]int{}
	for i, v := range cfRangesBox {
		got, err := RangesForBox(v.Lat, v.Lon, v.Width, v.Height)
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		counts[checkRangesAgainstC(t, got, v.Ranges, v.Lat, v.Lon)]++
	}
	t.Logf("vs compiled C over %d box vectors: %v", len(cfRangesBox), counts)
}

// TestDistanceCVectors pins the haversine against the compiled
// geohashGetDistance, city pairs, pole-to-pole and across the
// antimeridian.  The formula and operation order are the exact C port,
// but two effects keep the comparison from being bit-exact: libm
// trigonometry differs by an ulp between implementations (macOS libm vs
// Go's pure-Go math), and the compiler may contract multiplies into
// fused multiply-adds (permitted by the Go spec; see Distance's doc),
// which through the cancellation in lon2r-lon1r shifts results by up to
// ~1e-9 relative.  The tolerance below still exceeds the note's ±0.5%
// accuracy requirement by six orders of magnitude; rows without
// cancellation compare exactly.
func TestDistanceCVectors(t *testing.T) {
	exact := 0
	for i, v := range cfDistance {
		got := Distance(v.Lat1, v.Lon1, v.Lat2, v.Lon2)
		if got == v.Want {
			exact++
			continue
		}
		if math.Abs(got-v.Want) > 1e-9*math.Max(1, math.Abs(v.Want)) {
			t.Fatalf("row %d (%v,%v)-(%v,%v): got %.17g want %.17g", i, v.Lat1, v.Lon1, v.Lat2, v.Lon2, got, v.Want)
		}
	}
	t.Logf("%d/%d distance vectors bit-identical to the C", exact, len(cfDistance))
}

// TestGEOSEARCHDifferential is the end-to-end contract against a live
// Redis server: for every captured GEOSEARCH BYRADIUS query, each member
// Redis actually returned (found through Redis's own 9-box ranges, then
// kept by its distance filter) must have its Encode score inside
// pluto's RangesForRadius output for the same query.  Conversely every
// member within radius by pluto's Distance — including any Redis's
// single-decrement window would have missed — must also be covered.
func TestGEOSEARCHDifferential(t *testing.T) {
	missedByRedis := 0
	for qi, q := range searchQueries {
		ranges, err := RangesForRadius(q.CLat, q.CLon, q.Radius)
		if err != nil {
			t.Fatalf("query %d: %v", qi, err)
		}
		checkCanonical(t, ranges)
		for _, idx := range q.Returned {
			m := searchMembers[idx-1] // member names p1..pN are 1-based
			score, err := Encode(m.Lat, m.Lon)
			if err != nil {
				continue // the grid pushes |lon| past 180 for edge centers; Redis rejected those too
			}
			if !covered(score, ranges) {
				t.Fatalf("query %d (r=%v): member %d (%v,%v) returned by Redis but score %d outside %v",
					qi, q.Radius, idx, m.Lat, m.Lon, score, ranges)
			}
		}
		// The completeness direction, computed over the whole grid — and
		// a census of what Redis's single-decrement window missed.
		returned := make(map[int]bool, len(q.Returned))
		for _, idx := range q.Returned {
			returned[idx-1] = true
		}
		for idx, m := range searchMembers {
			if _, err := Encode(m.Lat, m.Lon); err != nil {
				continue
			}
			if Distance(q.CLat, q.CLon, m.Lat, m.Lon) <= q.Radius {
				score, _ := Encode(m.Lat, m.Lon)
				if !covered(score, ranges) {
					t.Fatalf("query %d (r=%v): member %d (%v,%v) is within radius but score %d outside %v",
						qi, q.Radius, idx, m.Lat, m.Lon, score, ranges)
				}
				if !returned[idx] {
					missedByRedis++
				}
			}
		}
	}
	t.Logf("grids include %d members Redis's window did not return (its known pole/window gaps); pluto covers them", missedByRedis)
}
