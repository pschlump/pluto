/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package string_sort

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/quick_sort"
)

// benchDataset is one fixed key set the sorters are measured on.
type benchDataset struct {
	name string
	keys []string
	w    int // LSD key width (LSD is skipped when 0)
}

// benchDatasets builds the standard datasets at n keys, generated with
// a fixed seed so benchmark numbers are comparable across runs:
//
//   - random16:   16 random letter bytes — the generic case
//   - prefixed16: a 12-byte shared prefix + 4 random bytes — the
//     long-common-prefix shape where MSD pays extra buckets and
//     Quick3 peels one byte per pass
//   - long64: a 48-byte shared prefix + 16 random bytes — the
//     URL-like shape
func benchDatasets(n int) []benchDataset {
	rng := rand.New(rand.NewPCG(42, 7))
	letters := func(l int) string {
		b := make([]byte, l)
		for i := range b {
			b[i] = byte('a' + rng.IntN(26))
		}
		return string(b)
	}

	random16 := make([]string, n)
	for i := range random16 {
		random16[i] = letters(16)
	}

	prefix := letters(12)
	prefixed16 := make([]string, n)
	for i := range prefixed16 {
		prefixed16[i] = prefix + letters(4)
	}

	prefix64 := letters(48)
	long64 := make([]string, n)
	for i := range long64 {
		long64[i] = prefix64 + letters(16)
	}

	return []benchDataset{
		{name: "random16", keys: random16, w: 16},
		{name: "prefixed16", keys: prefixed16, w: 16},
		{name: "long64", keys: long64, w: 64},
	}
}

// benchmarkSort measures one sorter over one dataset; cloning the input
// inside the loop keeps every iteration sorting identical data (the
// clone cost is a small fraction of any sort measured here).
func benchmarkSort(b *testing.B, keys []string, sortFn func([]string)) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		data := slices.Clone(keys)
		sortFn(data)
	}
}

func BenchmarkSlicesSort(b *testing.B) {
	for _, ds := range benchDatasets(100_000) {
		b.Run(ds.name, func(b *testing.B) { benchmarkSort(b, ds.keys, slices.Sort) })
	}
}

func BenchmarkQuickSort(b *testing.B) {
	for _, ds := range benchDatasets(100_000) {
		b.Run(ds.name, func(b *testing.B) { benchmarkSort(b, ds.keys, quick_sort.Sort[string]) })
	}
}

func BenchmarkLSDSort(b *testing.B) {
	for _, ds := range benchDatasets(100_000) {
		b.Run(ds.name, func(b *testing.B) {
			benchmarkSort(b, ds.keys, func(a []string) { LSDSort(a, ds.w) })
		})
	}
}

func BenchmarkMSDSort(b *testing.B) {
	for _, ds := range benchDatasets(100_000) {
		b.Run(ds.name, func(b *testing.B) { benchmarkSort(b, ds.keys, MSDSort) })
	}
}

func BenchmarkQuick3String(b *testing.B) {
	for _, ds := range benchDatasets(100_000) {
		b.Run(ds.name, func(b *testing.B) { benchmarkSort(b, ds.keys, Quick3String) })
	}
}
