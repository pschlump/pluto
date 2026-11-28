package bloom_filter_ts

import (
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/pluto/bloom_filter"
	"github.com/pschlump/pluto/hashing"
)

// TestTwinMatchesPlainBits pins that the twin's Put/MightContain produce the
// same bit array as the plain package (which owns the allocation-free path).
func TestTwinMatchesPlainBits(t *testing.T) {
	twin := New(2000, 0.01, hashing.StringFunnel)
	plain := bloom_filter.New(2000, 0.01, hashing.StringFunnel)
	for i := range 2000 {
		value := fmt.Sprintf("twin-%d", i)
		if twin.Put(value) != plain.Put(value) {
			t.Fatalf("Put(%q) changed-bit report differs", value)
		}
	}
	twinBytes, plainBytes := twin.Bytes(), plain.Bytes()
	if string(twinBytes) != string(plainBytes) {
		t.Fatal("twin and plain bit arrays differ")
	}
}

func TestCompatibleShapes(t *testing.T) {
	var nilFilter *BloomFilter[string]
	var zero BloomFilter[string]
	a := New(1000, 0.01, hashing.StringFunnel)
	b := New(1000, 0.01, hashing.StringFunnel)
	differentBits := New(2000, 0.01, hashing.StringFunnel)
	differentHashes := NewWithBits(a.BitCount(), a.HashCount()+1, hashing.StringFunnel)
	cases := []struct {
		name        string
		left, right *BloomFilter[string]
		want        bool
	}{
		{"same shape", a, b, true},
		{"self", a, a, true},
		{"different bits", a, differentBits, false},
		{"different hashes", a, differentHashes, false},
		{"nil/nil", nilFilter, nilFilter, true},
		{"nil/zero", nilFilter, &zero, true},
		{"zero/zero", &zero, &zero, true},
		{"nil/real", nilFilter, a, false},
		{"real/zero", a, &zero, false},
	}
	for _, c := range cases {
		if got := c.left.Compatible(c.right); got != c.want {
			t.Errorf("%s: Compatible = %v want %v", c.name, got, c.want)
		}
	}
}

func TestConcurrentCompatibleAndProbes(t *testing.T) {
	a := New(1000, 0.01, hashing.StringFunnel)
	b := New(1000, 0.01, hashing.StringFunnel)
	var wait sync.WaitGroup
	for worker := range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := range 300 {
				a.Put(fmt.Sprintf("%d-%d", worker, i))
				if !a.Compatible(b) || !b.Compatible(a) {
					t.Error("shape compatibility changed under concurrent writes")
				}
				b.MightContain(fmt.Sprintf("%d-%d", worker, i))
			}
		}()
	}
	wait.Wait()
}

func TestProbeAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("sync.Pool deliberately drops items under the race detector")
	}
	filter := New(1000, 0.01, hashing.StringFunnel)
	other := New(1000, 0.01, hashing.StringFunnel)
	filter.Put("value-0000")
	if allocs := testing.AllocsPerRun(1000, func() {
		filter.MightContain("value-0000")
		filter.MightContain("absent-0000")
		filter.Put("value-0001")
	}); allocs != 0 {
		t.Errorf("Put/MightContain allocate %v times per call, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		filter.Compatible(other)
	}); allocs != 0 {
		t.Errorf("Compatible allocates %v times per call, want 0", allocs)
	}
}
