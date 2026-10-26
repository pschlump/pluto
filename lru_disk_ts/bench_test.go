package lru_disk_ts

import (
	"fmt"
	"math/rand"
	"testing"
)

// The measured workload: 50 cached artifacts of ~300 KB each (15 MB on
// disk), a Zipf(1.1) access stream — some artifacts much hotter than
// the rest — and a memory tier sized to the hot set. All three
// configs run the SAME key stream (fixed seed), so the sub-benchmarks
// differ only in where the hits come from.
const (
	benchKeys    = 50
	benchValSize = 300_000
)

// benchGetZipf preloads benchKeys entries of benchValSize incompressible
// bytes, then drives b.N Gets through a Zipf(1.1) key stream.
func benchGetZipf(b *testing.B, memCapacity int) {
	c, err := NewLruDisk[[]byte](benchKeys, b.TempDir(), benc, bdec, WithMemCapacity(memCapacity))
	if err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewSource(42))
	vals := make([][]byte, benchKeys)
	for i := range vals {
		vals[i] = make([]byte, benchValSize)
		rng.Read(vals[i]) // random payloads: the disk reads are real, not compressed away
		if err := c.Put(fmt.Sprintf("k%02d", i), vals[i]); err != nil {
			b.Fatal(err)
		}
	}
	z := rand.NewZipf(rng, 1.1, 1, benchKeys-1)
	keys := make([]string, benchKeys)
	for i := range keys {
		keys[i] = fmt.Sprintf("k%02d", i)
	}
	b.SetBytes(benchValSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := c.Get(keys[z.Uint64()]); !ok {
			b.Fatal("miss in a full cache")
		}
	}
}

func BenchmarkGetZipf300K(b *testing.B) {
	for _, tc := range []struct {
		name string
		mem  int
	}{
		{"mem=0_disk_only", 0},      // the default: every Get reads the file
		{"mem=20_hot_set", 20},      // the tier sized to the hot set
		{"mem=50_all_resident", 50}, // the ceiling: the tier holds every entry
	} {
		b.Run(tc.name, func(b *testing.B) { benchGetZipf(b, tc.mem) })
	}
}
