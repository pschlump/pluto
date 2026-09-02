/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import (
	"math"
	"slices"
)

// mercatorMax is geohash_helper.c's MERCATOR_MAX, the doubling horizon of
// the step estimator.
const mercatorMax = 20037726.37

// estimateStepsByRadius is geohashEstimateStepsByRadius ported exactly:
// double the radius until it spans the Web-Mercator world, take that
// power-of-two as the step, then shave two off (and one or two more near
// the poles, where cells are geometrically narrower in longitude), and
// frame to [1, StepBits].  rangeMeters must be >= 0 and finite or +Inf —
// negative would never leave the doubling loop (the C has the same hole;
// the public entry points validate first).
func estimateStepsByRadius(rangeMeters, lat float64) int {
	if rangeMeters == 0 {
		return StepBits
	}
	step := 1
	for ; rangeMeters < mercatorMax; step++ {
		rangeMeters *= 2
	}
	step -= 2 // Make sure range is included in most of the base cases.

	// Wider range towards the poles... it is possible to do better by
	// computing the distance between meridians at this latitude, but the
	// C approximation does the trick.
	if lat > 66 || lat < -66 {
		step--
		if lat > 80 || lat < -80 {
			step--
		}
	}

	// Frame to valid range.
	if step < 1 {
		step = 1
	}
	if step > StepBits {
		step = StepBits
	}
	return step
}

// boundingBox is geohashBoundingBox: the lat/lon box conservatively
// containing a shape of the given half-extents (height and width in
// meters) around (lat, lon).  The higher the latitude, the shorter the
// arc, so the shape's left and right edges bend inward — the box uses
// the wider of the two edge deltas (the top edge's in the northern
// hemisphere, the bottom's in the southern), which is always the
// conservative choice.  The result may extend past ±180 or past the
// encodable band; the callers handle both.
func boundingBox(lat, lon, height, width float64) (minLon, minLat, maxLon, maxLat float64) {
	latDelta := radToDeg(height / EarthRadiusInMeters)
	longDeltaTop := radToDeg(width / EarthRadiusInMeters / math.Cos(degToRad*(lat+latDelta)))
	longDeltaBottom := radToDeg(width / EarthRadiusInMeters / math.Cos(degToRad*(lat-latDelta)))
	if lat < 0 { // southern hemisphere: the bottom edge is the wider one
		minLon = lon - longDeltaBottom
		maxLon = lon + longDeltaBottom
	} else {
		minLon = lon - longDeltaTop
		maxLon = lon + longDeltaTop
	}
	minLat = lat - latDelta
	maxLat = lat + latDelta
	return minLon, minLat, maxLon, maxLat
}

// windowCovers reports whether the 3×3 window around the step-precision
// cell containing (lat, lon) covers the (possibly pole- or
// antimeridian-crossing) bounding box.  It is the coverage test of
// geohashCalculateAreasByShapeWGS84 with the C's two blind spots
// repaired:
//
//   - Longitude compares WRAPPED ARCS, not plain coordinates.  The C's
//     `east.longitude.max < max_lon` cannot see that a wrapped east
//     neighbor (the cell just past the antimeridian) covers the box's
//     eastern part, so it decrements the step for every
//     antimeridian-crossing query even when the window already covers
//     the circle.  The arc form measures how far east of the query point
//     the window reaches (adding a full turn when the neighbor wrapped)
//     and asks the same of the box: both arcs are anchored at the query
//     longitude, which sits inside the center cell and inside the box,
//     so fitting both arcs is exactly box-in-window.
//   - Latitude knows the encodable band.  The box is clamped to the band
//     first (nothing storable lives past ±85.05112878), and a direction
//     counts as covered when EITHER the neighbor reaches past the box
//     edge (the C's test) OR the center cell itself already reaches the
//     edge of the band — for a center in the top row every neighbor
//     "north" wraps to the far side of the world, but there is also
//     nothing above the band left to cover.
//
// A box edge more than a full turn away, or a box inverted by the
// bounding-box blowup past a pole (its cos going negative), can never be
// covered at any step; the caller detects this by reaching step 1 and
// degrades to the full score space.
func windowCovers(hash uint64, step int, lon, minLon, minLat, maxLon, maxLat float64) bool {
	latMin, latMax, _, _ := decodeArea(hash, step)
	_, nLatMax, _, _ := decodeArea(moveY(hash, 1, step), step)
	sLatMin, _, _, _ := decodeArea(moveY(hash, -1, step), step)
	_, _, _, eLonMax := decodeArea(moveX(hash, 1, step), step)
	_, _, wLonMin, _ := decodeArea(moveX(hash, -1, step), step)

	// Latitude: clamp the box to the encodable band; a side is covered
	// when the neighbor spans past the clamped edge, or when the center
	// cell itself already spans the whole remaining band beyond it.
	cMaxLat := min(maxLat, LatMax)
	cMinLat := max(minLat, LatMin)
	northOK := nLatMax >= cMaxLat || latMax >= cMaxLat
	southOK := sLatMin <= cMinLat || latMin <= cMinLat

	// Longitude: how far east/west of the query the window reaches (a
	// full turn is added for a wrapped neighbor) vs how far the box
	// reaches.  Needs at or beyond a full turn, or negative (an inverted
	// box), are never satisfiable.
	eastReach := eLonMax - lon
	if eastReach < 0 {
		eastReach += 360
	}
	westReach := lon - wLonMin
	if westReach < 0 {
		westReach += 360
	}
	eastNeed := maxLon - lon
	westNeed := lon - minLon
	eastOK := eastNeed >= 0 && eastNeed < 360 && eastReach >= eastNeed
	westOK := westNeed >= 0 && westNeed < 360 && westReach >= westNeed

	return northOK && southOK && eastOK && westOK
}

// cellRange is geo.c's scoresOfGeoHashBox: the half-open score interval
// [lo, hi) covering every 52-bit score inside the cell — lo included,
// hi excluded, the exact zset range Redis queries (minex 0, maxex 1).
func cellRange(bits uint64, step int) [2]uint64 {
	shift := uint(ScoreBits - step*2)
	return [2]uint64{bits << shift, (bits + 1) << shift}
}

// windowRanges collects the score ranges of the 3×3 window's cells,
// pruning the rows and columns that cannot touch the bounding box (the
// GZERO block of the C: when the center cell itself already crosses a
// box edge, the neighbor row beyond it lies entirely outside the box —
// the C prunes only at steps >= 2).  Latitude bounds are clamped to the
// encodable band first so a center cell in the top/bottom row prunes its
// WRAPPED latitude neighbor (without the clamp a pole-adjacent query
// would scan the tropics as its "north").  The result is canonical:
// sorted ascending, duplicate cells collapsed, touching ranges
// coalesced — the same score space the C queries with its nine per-box
// lookups, as one disjoint list (the C only skips consecutive
// duplicates, so at coarse steps it can query the same box twice; the
// canonical form cannot).
func windowRanges(hash uint64, step int, minLon, minLat, maxLon, maxLat float64) [][2]uint64 {
	latMin, latMax, lonMin, lonMax := decodeArea(hash, step)
	dropSouth := latMin < max(minLat, LatMin)
	dropNorth := latMax > min(maxLat, LatMax)
	dropWest := lonMin < minLon
	dropEast := lonMax > maxLon
	if step < 2 {
		dropSouth, dropNorth, dropWest, dropEast = false, false, false, false
	}

	var ranges [][2]uint64
	for dy := -1; dy <= 1; dy++ {
		if (dy == -1 && dropSouth) || (dy == 1 && dropNorth) {
			continue
		}
		row := moveY(hash, dy, step)
		for dx := -1; dx <= 1; dx++ {
			if (dx == -1 && dropWest) || (dx == 1 && dropEast) {
				continue
			}
			ranges = append(ranges, cellRange(moveX(row, dx, step), step))
		}
	}
	return canonicalize(ranges)
}

// canonicalize sorts the ranges ascending, drops duplicates and coalesces
// touching or overlapping ones — a minimal disjoint list of the same
// union.  (Cells of one window can only touch, never overlap; the
// overlap merge is defensive.)
func canonicalize(ranges [][2]uint64) [][2]uint64 {
	slices.SortFunc(ranges, func(a, b [2]uint64) int {
		switch {
		case a[0] < b[0]:
			return -1
		case a[0] > b[0]:
			return 1
		}
		return 0
	})
	out := ranges[:0]
	for _, r := range ranges {
		if n := len(out); n > 0 && r[0] <= out[n-1][1] {
			if r[1] > out[n-1][1] {
				out[n-1][1] = r[1]
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// maxScore is the largest score Encode can produce: encoding the exact
// north-east band corner (85.05112878, 180) yields latitude and
// longitude indices of exactly 2^26, whose interleave sets bits 52 and
// 53 — one bit above the nominal score space.  Redis does the same (a
// live server stores 13510798882111488 for that corner), so the corner
// cells genuinely live at scores >= 1<<ScoreBits and fullRange must
// include them.
const maxScore = 1<<ScoreBits + 1<<(ScoreBits+1) // 2^52 + 2^53 = 13510798882111488

// scoreSpaceTop is the exclusive top of everything the range queries can
// emit: the step-1 cell beyond the corner spill is maxScore's neighbor
// (bits+1), aligned — 13·2^50.  fullRange covers [0, scoreSpaceTop).
const scoreSpaceTop = maxScore + 1<<(ScoreBits-2)

// fullRange is the degraded area of interest for shapes no geohash
// window can cover (those reaching a pole or otherwise wrapping the
// world in longitude): every score Encode can produce.  Complete by
// construction; the caller's post-filter restores exactness.
func fullRange() [][2]uint64 {
	return [][2]uint64{{0, scoreSpaceTop}}
}

// rangesForShape is geohashCalculateAreasByShapeWGS84 plus geo.c's
// box-to-score-range translation: estimate the step for the shape's
// radius, take the 3×3 window at that step, and lower the step until the
// window covers the bounding box.
//
// One deliberate divergence from the C, which checks coverage once and
// decreases the step at most a single time: near the poles one decrement
// is not enough (a circle at latitude 84 with a 446 km radius spans
// ±115° of longitude while its step-4 window spans ~±34°) and Redis
// silently misses members.  pluto loops the decrease until the
// arc-correct windowCovers passes, or — for a shape whose box defeats
// even the coarsest window (it reaches a pole, wrapping the world in
// longitude) — degrades to fullRange, the whole score space.  The
// divergences only ever change which candidate cells are scanned; the
// ranges stay a conservative-complete superset of the shape, and the
// caller's exact post-filter removes the extras.
func rangesForShape(lat, lon float64, circle bool, a, b float64) [][2]uint64 {
	var height, width, radius float64
	if circle {
		// The C feeds a circle its radius as both half-extents.
		height, width, radius = a, a, a
	} else {
		width, height = a, b
		// Center-to-corner distance; the squares go through prod so no
		// compiler contraction drifts the sqrt input an ulp.
		radius = math.Sqrt(prod(a/2, a/2) + prod(b/2, b/2))
	}
	minLon, minLat, maxLon, maxLat := boundingBox(lat, lon, height, width)

	steps := estimateStepsByRadius(radius, lat)
	for {
		hash := encodeStep(lat, lon, steps)
		if windowCovers(hash, steps, lon, minLon, minLat, maxLon, maxLat) {
			return windowRanges(hash, steps, minLon, minLat, maxLon, maxLat)
		}
		if steps == 1 {
			// The coarsest window (3 of the 4 step-1 longitude columns)
			// still does not cover the box — a shape reaching a pole or
			// otherwise wrapping the world in longitude.
			return fullRange()
		}
		steps--
	}
}

// RangesForRadius returns the score ranges to look up in a sorted set for
// a radius query around (lat, lon): a sorted list of disjoint half-open
// ranges [lo, hi) whose union covers every 52-bit geohash score the
// circle of radiusMeters can touch — plus, necessarily, some it cannot.
// The contract is conservative-complete: NO member inside the circle is
// outside the ranges (no false negatives); members outside the circle
// regularly fall inside them (false positives) — the caller post-filters
// every candidate with
//
//	Distance(lat, lon, candLat, candLon) <= radiusMeters
//
// for exactness, exactly as Redis's geoWithinShape does.
//
// The circle may cross the antimeridian (the window's cells wrap with
// the hash, so the ranges stay complete) but may not reach a pole: a
// circle whose bounding box leaves the encodable band ±85.05112878
// returns the single full range [0, 1<<ScoreBits) — scan everything,
// post-filter, still exact.
//
// radiusMeters must be >= 0 (zero is legal — the tightest precision);
// negative or NaN returns ErrNegativeRadius.  Coordinates follow
// Encode's range rules; violation returns ErrOutOfRange.
//
// Complexity is O(1) — at most 26 coverage checks of O(1) each.
func RangesForRadius(lat, lon, radiusMeters float64) ([][2]uint64, error) {
	if err := checkCoord(lat, lon); err != nil {
		return nil, err
	}
	if math.IsNaN(radiusMeters) || radiusMeters < 0 {
		return nil, ErrNegativeRadius
	}
	return rangesForShape(lat, lon, true, radiusMeters, 0), nil
}

// RangesForBox is RangesForRadius for the GEOSEARCH BYBOX rectangle: the
// half-open score ranges [lo, hi) covering every cell the
// widthMeters × heightMeters box centered on (lat, lon) can touch, with
// the same conservative-complete contract.  Redis's rectangle membership
// rule for the post-filter is two Distance calls:
//
//	latDist := Distance(candLat, lon, lat, lon)  // same-longitude, the fast path
//	lonDist := Distance(candLat, candLon, candLat, lon)
//	inBox := latDist <= heightMeters/2 && lonDist <= widthMeters/2
//
// (the box's longitude test runs at the CANDIDATE's latitude — its left
// and right edges bend towards the poles, which is also why the ranges
// for a near-pole box can be wide).
//
// A box reaching past the encodable band ±85.05112878 returns the single
// full range [0, 1<<ScoreBits).  widthMeters and heightMeters must be
// >= 0; negative or NaN returns ErrNegativeRadius.  Coordinates follow
// Encode's range rules; violation returns ErrOutOfRange.
//
// Complexity is O(1).
func RangesForBox(lat, lon, widthMeters, heightMeters float64) ([][2]uint64, error) {
	if err := checkCoord(lat, lon); err != nil {
		return nil, err
	}
	if math.IsNaN(widthMeters) || widthMeters < 0 ||
		math.IsNaN(heightMeters) || heightMeters < 0 {
		return nil, ErrNegativeRadius
	}
	return rangesForShape(lat, lon, false, widthMeters, heightMeters), nil
}
