package bloom_ts

import "testing"

func TestExpectedFPPAndCompatible(t *testing.T) {
	filter := NewBloomBits(100, 3)
	filter.Add([]byte("a"))
	if filter.ExpectedFPP() <= 0 || filter.NlExpectedFPP() != filter.ExpectedFPP() {
		t.Fatal("expected FPP mismatch")
	}
	if !filter.Compatible(filter.Clone()) || filter.Compatible(NewBloomBits(100, 2)) || !filter.NlCompatible(filter.Clone()) {
		t.Fatal("compatibility mismatch")
	}
	var nilFilter *Bloom
	if nilFilter.ExpectedFPP() != 0 || !nilFilter.Compatible(&Bloom{}) {
		t.Fatal("nil behavior mismatch")
	}
}
