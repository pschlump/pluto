/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import (
	"math"
	"math/rand/v2"
	"testing"
)

// rng is the fixed-seed PCG source the house randomized tests use.
var rng = rand.New(rand.NewPCG(42, 7))

// randLatLon returns a uniform random encodable position.
func randLatLon() (lat, lon float64) {
	return LatMin + rng.Float64()*(LatMax-LatMin), LonMin + rng.Float64()*(LonMax-LonMin)
}

// randLogMeters returns a log-uniform length in [lo, hi] meters.
func randLogMeters(lo, hi float64) float64 {
	return math.Exp(math.Log(lo) + rng.Float64()*(math.Log(hi)-math.Log(lo)))
}

// destination projects (lat, lon) along bearing θ (radians from north)
// for arc distance d (radians) — the standard great-circle forward
// formula, used to construct points at known distances for the fuzz.
func destination(lat, lon, bearing, dist float64) (float64, float64) {
	sinLat2 := math.Sin(lat*degToRad)*math.Cos(dist) + math.Cos(lat*degToRad)*math.Sin(dist)*math.Cos(bearing)
	lat2 := math.Asin(sinLat2) / degToRad
	lon2 := lon + math.Atan2(math.Sin(bearing)*math.Sin(dist)*math.Cos(lat*degToRad),
		math.Cos(dist)-math.Sin(lat*degToRad)*sinLat2)/degToRad
	for lon2 > LonMax {
		lon2 -= 360
	}
	for lon2 < LonMin {
		lon2 += 360
	}
	return lat2, lon2
}

// TestRoundTrip is the note's round-trip requirement: decoding a score
// and re-encoding the cell center reproduces the score, for random
// points AND for random raw 52-bit scores (the center of cell h is in
// cell h for every h in the score space).
func TestRoundTrip(t *testing.T) {
	for i := 0; i < 20000; i++ {
		lat, lon := randLatLon()
		h, err := Encode(lat, lon)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		clat, clon := Decode(h)
		if again, err := Encode(clat, clon); err != nil || again != h {
			t.Fatalf("iter %d: re-encode of Decode(%d) = %d (%v)", i, h, again, err)
		}
	}
	for i := 0; i < 20000; i++ {
		h := rng.Uint64() % (1 << ScoreBits)
		clat, clon := Decode(h)
		if again, err := Encode(clat, clon); err != nil || again != h {
			t.Fatalf("iter %d: raw score %d center re-encodes to %d (%v)", i, h, again, err)
		}
	}
}

// TestRadiusCompleteness is the note's completeness fuzz: for random
// centers and log-uniform radii (1 m … 2·10⁷ m, so pole-spanning and
// antimeridian-crossing circles are routine), every point constructed
// INSIDE the circle must have its score inside RangesForRadius's
// output.  False positives are fine (the caller post-filters with
// Distance); a single false negative is a bug.
func TestRadiusCompleteness(t *testing.T) {
	fullRanges, samples := 0, 0
	for i := 0; i < 3000; i++ {
		lat, lon := randLatLon()
		radius := randLogMeters(1, 2e7)
		ranges, err := RangesForRadius(lat, lon, radius)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		checkCanonical(t, ranges)
		if len(ranges) == 1 && ranges[0][0] == 0 && ranges[0][1] == scoreSpaceTop {
			fullRanges++
		}
		for k := 0; k < 25; k++ {
			// r ≤ radius by construction (shaved a hair so the point is
			// strictly inside even after float wobble).
			r := radius * math.Sqrt(rng.Float64()) * (1 - 1e-12)
			plat, plon := destination(lat, lon, rng.Float64()*2*math.Pi, r/EarthRadiusInMeters)
			if plat > LatMax || plat < LatMin {
				continue // above the encodable band — not a storable member
			}
			d := Distance(lat, lon, plat, plon)
			if d > radius {
				t.Fatalf("iter %d: constructed point %.3f m but radius %.3f m", i, d, radius)
			}
			score, err := Encode(plat, plon)
			if err != nil {
				t.Fatalf("iter %d: %v", i, err)
			}
			samples++
			if !covered(score, ranges) {
				t.Fatalf("iter %d (lat %.6f lon %.6f r %.1f m): point (%.6f,%.6f) %.1f m away, score %d outside %v",
					i, lat, lon, radius, plat, plon, d, score, ranges)
			}
		}
	}
	t.Logf("%d in-circle samples all covered (%d of %d queries degraded to the full space)", samples, fullRanges, 3000)
}

// TestBoxCompleteness is TestRadiusCompleteness for the BYBOX rectangle
// under Redis's membership rule (latitude arc ≤ height/2, and the
// longitude great-circle arc at the CANDIDATE's latitude ≤ width/2).
func TestBoxCompleteness(t *testing.T) {
	samples := 0
	for i := 0; i < 2500; i++ {
		lat, lon := randLatLon()
		width := randLogMeters(1, 2e7)
		height := randLogMeters(1, 2e7)
		ranges, err := RangesForBox(lat, lon, width, height)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		checkCanonical(t, ranges)
		for k := 0; k < 25; k++ {
			plat, plon := boxSample(lat, lon, width, height)
			if !inBox(lat, lon, width, height, plat, plon) {
				continue // construction rejected by the rule itself
			}
			score, err := Encode(plat, plon)
			if err != nil {
				continue
			}
			samples++
			if !covered(score, ranges) {
				t.Fatalf("iter %d (lat %.6f lon %.6f w %.1f h %.1f): in-box point (%.6f,%.6f), score %d outside %v",
					i, lat, lon, width, height, plat, plon, score, ranges)
			}
		}
	}
	t.Logf("%d in-box samples all covered", samples)
}

// boxSample constructs one point satisfying the box membership rule.
func boxSample(lat, lon, width, height float64) (float64, float64) {
	latArc := (rng.Float64()*2 - 1) * (height / 2) * (1 - 1e-9) / EarthRadiusInMeters
	plat := lat + latArc/degToRad
	if plat > LatMax {
		plat = LatMax - (plat - LatMax) // fold back (only in-box lats matter)
		if plat < LatMin {
			plat = LatMin
		}
	}
	arc := (rng.Float64()*2 - 1) * (width / 2) * (1 - 1e-9) / EarthRadiusInMeters
	cosLat := math.Cos(plat * degToRad)
	var plon float64
	if cosLat <= 1e-12 || math.Abs(arc) >= math.Pi*cosLat {
		plon = LonMin + rng.Float64()*360 // whole parallel within width/2
	} else {
		plon = lon + arc/degToRad/cosLat
		for plon > LonMax {
			plon -= 360
		}
		for plon < LonMin {
			plon += 360
		}
	}
	return plat, plon
}

// inBox is Redis's geohashGetDistanceIfInRectangle via public Distance
// calls — the documented post-filter recipe.
func inBox(lat, lon, width, height, plat, plon float64) bool {
	if Distance(plat, lon, lat, lon) > height/2 {
		return false
	}
	if Distance(plat, plon, plat, lon) > width/2 {
		return false
	}
	return true
}

// TestDistanceKnownPairs checks the note's accuracy gate: known
// city-pair great-circle distances within ±0.5%, plus exact-value
// identities (antipodal points, pole-to-pole along a meridian, the
// one-degree equatorial arc).
func TestDistanceKnownPairs(t *testing.T) {
	pairs := []struct {
		lat1, lon1, lat2, lon2, km float64
		note                       string
	}{
		{51.5074, -0.1278, 40.7128, -74.0060, 5570, "London–NYC"},
		{40.7128, -74.0060, 35.6762, 139.6503, 10850, "NYC–Tokyo"},
		{35.6762, 139.6503, -33.8688, 151.2093, 7820, "Tokyo–Sydney"},
		{51.5074, -0.1278, 48.8566, 2.3522, 344, "London–Paris"},
		{37.7749, -122.4194, 22.3193, 114.1694, 11140, "SF–Hong Kong"},
	}
	for _, p := range pairs {
		got := Distance(p.lat1, p.lon1, p.lat2, p.lon2) / 1000
		if math.Abs(got-p.km)/p.km > 0.005 {
			t.Errorf("%s: got %.1f km, want %.0f km ±0.5%%", p.note, got, p.km)
		}
	}

	// Antipodal equatorial points: exactly half the circumference,
	// π·R (asin(1) = π/2 exactly on this path).
	if got, want := Distance(0, 0, 0, 180), EarthRadiusInMeters*math.Pi; math.Abs(got-want) > 1e-6*want {
		t.Errorf("antipodal: got %.6f want %.6f", got, want)
	}
	// Pole-to-pole along the Greenwich meridian exercises the
	// equal-longitude fast path: R·(rad span).
	if got, want := Distance(85.05112878, 0, -85.05112878, 0),
		EarthRadiusInMeters*degToRad*(85.05112878*2); math.Abs(got-want) > 1e-9*want {
		t.Errorf("pole-to-pole fast path: got %.9g want %.9g", got, want)
	}
	// One degree on the equator: R·π/180 up to the haversine's rounding.
	if got, want := Distance(0, 0, 0, 1), EarthRadiusInMeters*degToRad; math.Abs(got-want) > 1e-9*want {
		t.Errorf("equatorial degree: got %.9g want %.9g", got, want)
	}
	// Symmetry and identity.
	if Distance(10, 20, 30, 40) != Distance(30, 40, 10, 20) {
		t.Error("Distance not symmetric")
	}
	if got := Distance(10, 20, 10, 20); got != 0 {
		t.Errorf("Distance to self = %v, want 0", got)
	}
}

// TestUnits covers the Redis unit set, case-insensitivity, the round
// trip, and the error.
func TestUnits(t *testing.T) {
	cases := []struct {
		unit     string
		perMeter float64 // meters per unit
	}{
		{"m", 1}, {"M", 1}, {"km", 1000}, {"KM", 1000}, {"Km", 1000},
		{"ft", 0.3048}, {"FT", 0.3048}, {"mi", 1609.34}, {"MI", 1609.34},
	}
	for _, c := range cases {
		m, err := ToMeters(2, c.unit)
		if err != nil || m != 2*c.perMeter {
			t.Errorf("ToMeters(2, %q) = %v, %v", c.unit, m, err)
		}
		u, err := FromMeters(2*c.perMeter*3, c.unit)
		if err != nil || u != 6 {
			t.Errorf("FromMeters round trip on %q = %v, %v", c.unit, u, err)
		}
	}
	for _, bad := range []string{"", "yd", "meter", "miles", "M ", "mc"} {
		if _, err := ToMeters(1, bad); err == nil {
			t.Errorf("ToMeters(1, %q) accepted", bad)
		} else if !isErrInvalidUnit(err) {
			t.Errorf("ToMeters(1, %q) = %v, want ErrInvalidUnit", bad, err)
		}
		if _, err := FromMeters(1, bad); !isErrInvalidUnit(err) {
			t.Errorf("FromMeters(1, %q) = %v, want ErrInvalidUnit", bad, err)
		}
	}
	// The Redis mile factor is 1609.34, not the 1609.344 survey mile.
	if m, _ := ToMeters(1, "mi"); m != 1609.34 {
		t.Errorf("mi factor = %v, want 1609.34 (Redis's, not 1609.344)", m)
	}
}

func isErrInvalidUnit(err error) bool { return err != nil && err.Error() == ErrInvalidUnit.Error() }

// TestEncodeErrors pins the validation contract: band boundaries are
// legal (equality passes, as in Redis), everything beyond — and NaN and
// both infinities — is ErrOutOfRange.
func TestEncodeErrors(t *testing.T) {
	bad := [][2]float64{
		{85.05112879, 0}, {-85.05112879, 0},
		{0, 180.0000001}, {0, -180.0000001},
		{90, 0}, {-90, 0}, {0, 360},
		{math.NaN(), 0}, {0, math.NaN()},
		{math.Inf(1), 0}, {math.Inf(-1), 0}, {0, math.Inf(1)},
	}
	for i, c := range bad {
		if _, err := Encode(c[0], c[1]); err == nil {
			t.Errorf("case %d (%v,%v) accepted", i, c[0], c[1])
		}
	}
	good := [][2]float64{
		{LatMin, LonMin}, {LatMax, LonMax},
		{LatMin, LonMax}, {LatMax, LonMin}, {0, 0},
	}
	for i, c := range good {
		if _, err := Encode(c[0], c[1]); err != nil {
			t.Errorf("case %d (%v,%v) rejected: %v", i, c[0], c[1], err)
		}
	}
}

// TestRangesValidation pins the range queries' data-error contract.
func TestRangesValidation(t *testing.T) {
	if _, err := RangesForRadius(0, 0, -1); !isErrNegative(err) {
		t.Errorf("negative radius: %v", err)
	}
	if _, err := RangesForRadius(0, 0, math.NaN()); !isErrNegative(err) {
		t.Errorf("NaN radius: %v", err)
	}
	if _, err := RangesForRadius(91, 0, 1); !isOutOfRange(err) {
		t.Errorf("out-of-range center: %v", err)
	}
	if _, err := RangesForBox(0, 0, -1, 1); !isErrNegative(err) {
		t.Errorf("negative width: %v", err)
	}
	if _, err := RangesForBox(0, 0, 1, math.NaN()); !isErrNegative(err) {
		t.Errorf("NaN height: %v", err)
	}
	if _, err := RangesForBox(0, 181, 1, 1); !isOutOfRange(err) {
		t.Errorf("out-of-range center: %v", err)
	}
	// Zero dimensions are legal (the tightest precision).
	if r, err := RangesForRadius(51.5074, -0.1278, 0); err != nil || len(r) == 0 {
		t.Errorf("zero radius: %v %v", r, err)
	}
	if r, err := RangesForBox(51.5074, -0.1278, 0, 0); err != nil || len(r) == 0 {
		t.Errorf("zero box: %v %v", r, err)
	}
	// An infinite radius covers everything.
	if r, err := RangesForRadius(0, 0, math.Inf(1)); err != nil ||
		len(r) != 1 || r[0][0] != 0 || r[0][1] != scoreSpaceTop {
		t.Errorf("infinite radius: %v %v", r, err)
	}
}

func isErrNegative(err error) bool { return err != nil && err == ErrNegativeRadius }
func isOutOfRange(err error) bool  { return err != nil && err == ErrOutOfRange }

// TestNeighborsWrap pins the wraparound contract: the western neighbor
// of an antimeridian-hugging cell is on the far side of +180, and the
// northern neighbor of a top-row cell wraps to the bottom of the band.
func TestNeighborsWrap(t *testing.T) {
	// lon just inside +180: the E neighbor must decode to ≈ -180.
	h, _ := Encode(10, 179.99999999)
	n := Neighbors(h)
	_, eLon := Decode(n[2]) // E
	if eLon > 0 {
		t.Errorf("east neighbor of 179.99999999 decoded to lon %v, want wrapped negative", eLon)
	}
	_, wLon := Decode(n[6]) // W
	if wLon < 179 {
		t.Errorf("west neighbor decoded to lon %v, want still near +180", wLon)
	}

	// Top row: the N neighbor wraps to the bottom of the band.
	top, _ := Encode(LatMax, 0)
	tn := Neighbors(top)
	nLat, _ := Decode(tn[0]) // N
	if nLat > 0 {
		t.Errorf("north neighbor of the top row decoded to lat %v, want wrapped southern", nLat)
	}
	// Every neighbor is a valid score and re-encodes its own center.
	for i, nb := range tn {
		if _, err := Encode(Decode(nb)); err != nil {
			t.Errorf("neighbor %d (%d) does not round-trip: %v", i, nb, err)
		}
	}
}

// modelEncode is an oracle independent of the production bit twiddling:
// interval halving of the lat/lon ranges, most significant score bit
// first — the score's MSB is longitude's top bit, then latitude's, and
// so on down (the interleave order documented in the package comment).
func modelEncode(lat, lon float64) uint64 {
	var hash uint64
	latLo, latHi := LatMin, LatMax
	lonLo, lonHi := LonMin, LonMax
	for i := 0; i < StepBits; i++ {
		lonMid := (lonLo + lonHi) / 2
		var lonBit uint64
		if lon >= lonMid {
			lonBit, lonLo = 1, lonMid
		} else {
			lonHi = lonMid
		}
		latMid := (latLo + latHi) / 2
		var latBit uint64
		if lat >= latMid {
			latBit, latLo = 1, latMid
		} else {
			latHi = latMid
		}
		hash = hash<<2 | lonBit<<1 | latBit
	}
	return hash
}

// modelDecode inverts modelEncode by the same halving.
func modelDecode(hash uint64) (lat, lon float64) {
	latLo, latHi := LatMin, LatMax
	lonLo, lonHi := LonMin, LonMax
	for i := StepBits - 1; i >= 0; i-- {
		lonBit := (hash >> uint(2*i+1)) & 1
		latBit := (hash >> uint(2*i)) & 1
		lonMid := (lonLo + lonHi) / 2
		if lonBit == 1 {
			lonLo = lonMid
		} else {
			lonHi = lonMid
		}
		latMid := (latLo + latHi) / 2
		if latBit == 1 {
			latLo = latMid
		} else {
			latHi = latMid
		}
	}
	return (latLo + latHi) / 2, (lonLo + lonHi) / 2
}

// TestRandomizedModel is the house-style property test: an independent
// interval-halving oracle cross-checks Encode and Decode, a
// coordinate-arithmetic oracle cross-checks Neighbors (where the move
// stays inside the world — the wrap cases are pinned by the C vectors),
// and a spherical-law-of-cosines oracle cross-checks Distance.  Every
// 37th step also spot-checks radius completeness.  Seed 42/7.
func TestRandomizedModel(t *testing.T) {
	for step := 0; step < 4000; step++ {
		lat, lon := randLatLon()

		h, err := Encode(lat, lon)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if m := modelEncode(lat, lon); m != h {
			t.Fatalf("step %d: Encode = %d, halving oracle %d", step, h, m)
		}

		// Random raw score: decode against the oracle (tolerance a few
		// ulps of a degree — the two arithmetics round differently).
		rh := rng.Uint64() % (1 << ScoreBits)
		dLat, dLon := Decode(rh)
		mLat, mLon := modelDecode(rh)
		if math.Abs(dLat-mLat) > 1e-12 || math.Abs(dLon-mLon) > 1e-12 {
			t.Fatalf("step %d: Decode(%d) = (%.17g,%.17g), oracle (%.17g,%.17g)", step, rh, dLat, dLon, mLat, mLon)
		}

		// Neighbors via coordinate arithmetic where no wrap is involved.
		clat, clon := Decode(h)
		latCell := (LatMax - LatMin) / (1 << StepBits)
		lonCell := 360.0 / (1 << StepBits)
		n := Neighbors(h)
		for di, d := range [][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}} {
			plat := clat + float64(d[0])*latCell
			plon := clon + float64(d[1])*lonCell
			if plat > LatMax || plat < LatMin || plon > LonMax || plon < LonMin {
				continue // wrapped: pinned by TestNeighborsCVectors instead
			}
			if want := modelEncode(plat, plon); n[di] != want {
				t.Fatalf("step %d: neighbor %d = %d, coordinate oracle %d", step, di, n[di], want)
			}
		}

		// Distance vs the spherical law of cosines (a structurally
		// different formula; agreement to 0.1% at worst-case conditioning).
		lat2, lon2 := randLatLon()
		hav := Distance(lat, lon, lat2, lon2)
		cosD := math.Sin(lat*degToRad)*math.Sin(lat2*degToRad) +
			math.Cos(lat*degToRad)*math.Cos(lat2*degToRad)*math.Cos((lon-lon2)*degToRad)
		los := EarthRadiusInMeters * math.Acos(math.Min(1, math.Max(-1, cosD)))
		if math.Abs(hav-los) > 2e-3*math.Max(1, los) {
			t.Fatalf("step %d: haversine %.3f vs law-of-cosines %.3f", step, hav, los)
		}

		// Spot-check radius completeness.
		if step%37 == 0 {
			radius := randLogMeters(10, 1e7)
			ranges, err := RangesForRadius(lat, lon, radius)
			if err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
			for k := 0; k < 5; k++ {
				r := radius * math.Sqrt(rng.Float64()) * (1 - 1e-12)
				plat, plon := destination(lat, lon, rng.Float64()*2*math.Pi, r/EarthRadiusInMeters)
				if plat > LatMax || plat < LatMin {
					continue
				}
				score, _ := Encode(plat, plon)
				if !covered(score, ranges) {
					t.Fatalf("step %d: in-circle point (%.6f,%.6f) score %d outside %v", step, plat, plon, score, ranges)
				}
			}
		}
	}
}
