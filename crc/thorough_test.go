/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package crc

import (
	"hash/crc32"
	"hash/crc64"
	"math/rand"
	"testing"
)

// Cross-check every shared polynomial against the standard library on
// random buffers of varying length.  The seed is fixed so the run is
// deterministic.
func TestCrossCheckStdlib(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	std32 := map[uint32]*crc32.Table{
		IEEE:       crc32.MakeTable(crc32.IEEE),
		Castagnoli: crc32.MakeTable(crc32.Castagnoli),
		Koopman:    crc32.MakeTable(crc32.Koopman),
	}
	std64 := map[uint64]*crc64.Table{
		ECMA: crc64.MakeTable(crc64.ECMA),
		ISO:  crc64.MakeTable(crc64.ISO),
	}

	for length := 0; length <= 5000; length = length*3/2 + 1 {
		data := make([]byte, length)
		rng.Read(data)

		for poly, stdTab := range std32 {
			tab := MakeTable32(poly)
			if got, want := Checksum32(data, tab), crc32.Checksum(data, stdTab); got != want {
				t.Fatalf("length %d, poly 0x%X: Checksum32 = 0x%X, stdlib = 0x%X", length, poly, got, want)
			}
			// Incremental in two random chunks must agree with stdlib too.
			cut := 0
			if length > 0 {
				cut = rng.Intn(length + 1)
			}
			got := Update32(0, tab, data[:cut])
			got = Update32(got, tab, data[cut:])
			if want := crc32.Checksum(data, stdTab); got != want {
				t.Fatalf("length %d cut %d, poly 0x%X: chunked Update32 = 0x%X, stdlib = 0x%X", length, cut, poly, got, want)
			}
		}
		for poly, stdTab := range std64 {
			tab := MakeTable64(poly)
			if got, want := Checksum64(data, tab), crc64.Checksum(data, stdTab); got != want {
				t.Fatalf("length %d, poly 0x%X: Checksum64 = 0x%X, stdlib = 0x%X", length, poly, got, want)
			}
		}
	}
}
