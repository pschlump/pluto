/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import "fmt"

// Encode a position to the score Redis's GEOADD would store, and decode
// it back to the cell center GEOPOS would report.
func ExampleEncode() {
	score, err := Encode(51.5074, -0.1278) // London
	if err != nil {
		fmt.Println("invalid position:", err)
		return
	}
	fmt.Println("London score:", score)
	lat, lon := Decode(score)
	fmt.Printf("cell center: %.6f, %.6f\n", lat, lon)
	// Output:
	// London score: 2163557714755072
	// cell center: 51.507401, -0.127799
}

// Neighbors returns the 8 adjacent cells N, NE, E, SE, S, SW, W, NW.
func ExampleNeighbors() {
	score, _ := Encode(51.5074, -0.1278)
	for i, name := range []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"} {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Printf("%s=%d", name, Neighbors(score)[i])
	}
	fmt.Println()
	// Output:
	// N=2163557714755073 NE=2163557714755075 E=2163557714755074 SE=2163557714754391 S=2163557714754389 SW=2163557714754047 W=2163557714754730 NW=2163557714754731
}

// Distance between two cities, converted to kilometers.
func ExampleDistance() {
	meters := Distance(51.5074, -0.1278, 40.7128, -74.0060) // London to NYC
	km, _ := FromMeters(meters, "km")
	fmt.Printf("London-NYC: %.0f km\n", km)
	// Output:
	// London-NYC: 5572 km
}

// RangesForRadius returns the score ranges to look up in the sorted set
// for a radius search, then the candidates are post-filtered with
// Distance — the two halves of a GEOSEARCH BYRADIUS.
func ExampleRangesForRadius() {
	ranges, err := RangesForRadius(51.5074, -0.1278, 5000) // 5 km around London
	if err != nil {
		fmt.Println("bad query:", err)
		return
	}
	for _, r := range ranges {
		fmt.Printf("lookup scores in [%d, %d)\n", r[0], r[1])
	}
	// ... then for every member found: keep it when
	// Distance(51.5074, -0.1278, candLat, candLon) <= 5000.
	// Output:
	// lookup scores in [2163543604461568, 2163544678203392)
	// lookup scores in [2163545751945216, 2163546825687040)
	// lookup scores in [2163555415621632, 2163556489363456)
	// lookup scores in [2163557563105280, 2163558636847104)
}

// ToMeters converts a value in any Redis unit to meters.
func ExampleToMeters() {
	marathon, _ := ToMeters(42.195, "km")
	miles, _ := FromMeters(marathon, "mi")
	fmt.Printf("marathon: %.0f m (%.3f mi)\n", marathon, miles)
	// Output:
	// marathon: 42195 m (26.219 mi)
}
