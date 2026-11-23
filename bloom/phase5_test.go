package bloom

import (
	"math"
	"testing"
)

func TestExpectedFPPAndCompatible(t *testing.T) {
	filter := NewBloomBits(100, 3)
	if got := filter.ExpectedFPP(); got != 0 {
		t.Fatalf("empty FPP = %g", got)
	}
	filter.Add([]byte("a"))
	want := math.Pow(filter.Saturation(), 3)
	if got := filter.ExpectedFPP(); math.Abs(got-want) > 1e-15 {
		t.Fatalf("FPP = %g want %g", got, want)
	}
	if !filter.Compatible(filter.Clone()) || filter.Compatible(NewBloomBits(101, 3)) || filter.Compatible(NewBloomBits(100, 2)) {
		t.Fatal("shape compatibility mismatch")
	}
	var nilFilter *Bloom
	if !nilFilter.Compatible(&Bloom{}) || nilFilter.Compatible(filter) || nilFilter.ExpectedFPP() != 0 {
		t.Fatal("nil compatibility/FPP mismatch")
	}
	full := NewBloomBits(1, 1)
	full.Add(nil)
	if full.ExpectedFPP() != 1 {
		t.Fatalf("full saturation FPP = %g", full.ExpectedFPP())
	}
}
