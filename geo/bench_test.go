/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import "testing"

var (
	benchScore  uint64
	benchLatLon [2]float64
	benchRanges [][2]uint64
	benchDist   float64
)

func BenchmarkEncode(b *testing.B) {
	for b.Loop() {
		benchScore, _ = Encode(51.5074, -0.1278)
	}
}

func BenchmarkDecode(b *testing.B) {
	for b.Loop() {
		benchLatLon[0], benchLatLon[1] = Decode(2163557714755072)
	}
}

func BenchmarkNeighbors(b *testing.B) {
	for b.Loop() {
		benchNeighborsArray = Neighbors(2163557714755072)
	}
}

var benchNeighborsArray [8]uint64

func BenchmarkDistance(b *testing.B) {
	for b.Loop() {
		benchDist = Distance(51.5074, -0.1278, 40.7128, -74.0060)
	}
}

func BenchmarkRangesForRadius(b *testing.B) {
	for b.Loop() {
		benchRanges, _ = RangesForRadius(51.5074, -0.1278, 5000)
	}
}

func BenchmarkRangesForBox(b *testing.B) {
	for b.Loop() {
		benchRanges, _ = RangesForBox(51.5074, -0.1278, 5000, 3000)
	}
}
