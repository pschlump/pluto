/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu

import (
	"math/bits"
	"testing"
)

func makeSketch(maximumSize int) *FrequencySketch[int] {
	return NewFrequencySketchWithHash(maximumSize, func(key int) uint64 {
		return uint64(int64(key))
	})
}

func requirePanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("operation did not panic")
		}
	}()
	fn()
}

func TestSketchZeroValue(t *testing.T) {
	var sketch FrequencySketch[int]
	sketch.Increment(1)
	if got := sketch.Frequency(1); got != 0 {
		t.Errorf("uninitialized Frequency = %d, want 0", got)
	}
	sketch.EnsureCapacity(64)
	sketch.Increment(1)
	if got := sketch.Frequency(1); got != 1 {
		t.Errorf("Frequency after EnsureCapacity = %d, want 1", got)
	}
}

func TestSketchInvalidArguments(t *testing.T) {
	sketch := makeSketch(512)
	requirePanic(t, func() { sketch.EnsureCapacity(-1) })
	requirePanic(t, func() { NewFrequencySketch[int](-1) })
	requirePanic(t, func() { NewFrequencySketchWithHash[int](1, nil) })
}

func TestSketchEnsureCapacity(t *testing.T) {
	t.Run("smaller retains table and retunes aging", func(t *testing.T) {
		sketch := makeSketch(512)
		table := &sketch.table[0]
		sketch.EnsureCapacity(256)
		if len(sketch.table) != 512 || table != &sketch.table[0] {
			t.Fatalf("shrinking replaced the table: len = %d", len(sketch.table))
		}
		if sketch.sampleSize != 2560 {
			t.Errorf("sampleSize = %d, want 2560", sketch.sampleSize)
		}
		wantShift := bits.LeadingZeros32(uint32((len(sketch.table) >> 3) - 1))
		if sketch.blockShift != wantShift {
			t.Errorf("blockShift = %d, want %d", sketch.blockShift, wantShift)
		}
	})

	t.Run("larger forgets counts", func(t *testing.T) {
		sketch := makeSketch(512)
		sketch.Increment(7)
		sketch.EnsureCapacity(1024)
		if len(sketch.table) != 1024 || sketch.sampleSize != 10240 {
			t.Fatalf("table/sampleSize = %d/%d, want 1024/10240", len(sketch.table), sketch.sampleSize)
		}
		if got := sketch.Frequency(7); got != 0 {
			t.Errorf("Frequency after growth = %d, want 0", got)
		}
		wantShift := bits.LeadingZeros32(uint32((len(sketch.table) >> 3) - 1))
		if sketch.blockShift != wantShift {
			t.Errorf("blockShift = %d, want %d", sketch.blockShift, wantShift)
		}
	})

	t.Run("exact match retains counts", func(t *testing.T) {
		sketch := makeSketch(512)
		sketch.Increment(7)
		table := &sketch.table[0]
		sketch.EnsureCapacity(512)
		if table != &sketch.table[0] || sketch.Frequency(7) != 1 {
			t.Error("same-capacity call reset or replaced the sketch")
		}
	})
}

func TestSketchShrinkKeepsResetReachable(t *testing.T) {
	sketch := makeSketch(1024)
	sketch.size = 5000
	sketch.EnsureCapacity(256)
	if sketch.size != 2559 || sketch.sampleSize != 2560 {
		t.Fatalf("size/sampleSize = %d/%d, want 2559/2560", sketch.size, sketch.sampleSize)
	}
	sketch.Increment(42)
	if sketch.size != 1279 {
		t.Errorf("size after scheduled reset = %d, want 1279", sketch.size)
	}
}

func TestSketchShrinkDenseTableAgesOnSchedule(t *testing.T) {
	sketch := makeSketch(1024)
	for i := range sketch.table {
		sketch.table[i] = oneMask
	}
	sketch.size = 5000
	sketch.EnsureCapacity(256)
	sketch.Increment(42)
	if sketch.size != 0 {
		t.Errorf("size after dense reset = %d, want 0", sketch.size)
	}
	if got := sketch.Frequency(42); got != 1 {
		t.Errorf("Frequency after dense reset = %d, want 1", got)
	}
}

func TestSketchRepeatedRetrackKeepsAgingProgress(t *testing.T) {
	sketch := makeSketch(1024)
	for range 15 {
		sketch.Increment(42)
	}
	if got := sketch.Frequency(42); got != 15 {
		t.Fatalf("initial Frequency = %d, want 15", got)
	}
	for i := range 15000 {
		sketch.Increment(i)
		sketch.EnsureCapacity(1024 - (i & 1))
	}
	if got := sketch.Frequency(42); got >= 15 {
		t.Errorf("Frequency after repeated retracking = %d, want less than 15", got)
	}
}

func TestSketchRetrackDoesNotAdvanceAging(t *testing.T) {
	sketch := makeSketch(1024)
	for range 15 {
		sketch.Increment(42)
	}
	size := sketch.size
	sketch.EnsureCapacity(512)
	if sketch.sampleSize != 5120 || sketch.size != size {
		t.Errorf("sampleSize/size = %d/%d, want 5120/%d", sketch.sampleSize, sketch.size, size)
	}
	if got := sketch.Frequency(42); got != 15 {
		t.Errorf("Frequency after retracking = %d, want 15", got)
	}
}

func TestSketchSpread(t *testing.T) {
	if got := spread(0); got != 0 {
		t.Errorf("spread(0) = %d, want 0", got)
	}
	if got := spread(1); got == 1 {
		t.Error("spread(1) did not mix the input")
	}
	if spread(0x7fffffff) == spread(0x7ffffffe) {
		t.Error("adjacent inputs produced the same hash")
	}
}

func TestSketchIncrementAtSaturates(t *testing.T) {
	sketch := makeSketch(512)
	for range 15 {
		if !sketch.incrementAt(0, 0) {
			t.Fatal("incrementAt returned false before saturation")
		}
	}
	if sketch.incrementAt(0, 0) {
		t.Error("incrementAt modified a saturated counter")
	}
	if got := sketch.table[0] & 0xf; got != 15 {
		t.Errorf("counter = %d, want 15", got)
	}
}

func TestSketchIncrement(t *testing.T) {
	sketch := makeSketch(512)
	sketch.Increment(10)
	if got := sketch.Frequency(10); got != 1 {
		t.Errorf("Frequency after one increment = %d, want 1", got)
	}
	for range 20 {
		sketch.Increment(10)
	}
	if got := sketch.Frequency(10); got != 15 {
		t.Errorf("saturated Frequency = %d, want 15", got)
	}
	sketch.Increment(11)
	if got := sketch.Frequency(11); got != 1 {
		t.Errorf("distinct key Frequency = %d, want 1", got)
	}
	if got := sketch.Frequency(12); got != 0 {
		t.Errorf("unseen key Frequency = %d, want 0", got)
	}
	sketch.Increment(0)
	if got := sketch.Frequency(0); got != 1 {
		t.Errorf("zero key Frequency = %d, want 1", got)
	}
}

func TestSketchReset(t *testing.T) {
	sketch := makeSketch(64)
	reset := false
	for i := 1; i < 20*len(sketch.table); i++ {
		sketch.Increment(i)
		if sketch.size != i {
			reset = true
			break
		}
	}
	if !reset {
		t.Fatal("sketch did not age at its sample boundary")
	}
	if sketch.size > sketch.sampleSize/2 {
		t.Errorf("size after reset = %d, sampleSize = %d", sketch.size, sketch.sampleSize)
	}
}

func TestSketchFullReset(t *testing.T) {
	sketch := makeSketch(512)
	sketch.sampleSize = int(^uint(0) >> 1)
	for i := range 100000 {
		sketch.Increment(i)
	}
	for i, slot := range sketch.table {
		if bits.OnesCount64(slot) != 64 {
			t.Fatalf("table[%d] is not saturated: %064b", i, slot)
		}
	}
	sketch.reset()
	for i, slot := range sketch.table {
		if slot != resetMask {
			t.Fatalf("table[%d] after reset = %x, want %x", i, slot, resetMask)
		}
	}
}

func TestSketchHeavyHitters(t *testing.T) {
	sketch := makeSketch(512)
	for i := 100; i < 100000; i++ {
		sketch.Increment(i)
	}
	for i := 0; i < 10; i += 2 {
		for range i {
			sketch.Increment(i)
		}
	}
	popularity := make([]int, 10)
	for i := range popularity {
		popularity[i] = sketch.Frequency(i)
	}
	for _, i := range []int{0, 1, 3, 5, 7, 9} {
		if popularity[i] > popularity[2] {
			t.Errorf("popularity[%d] = %d exceeds popularity[2] = %d", i, popularity[i], popularity[2])
		}
	}
	for _, i := range []int{2, 4, 6} {
		if popularity[i] > popularity[i+2] {
			t.Errorf("popularity[%d] = %d exceeds popularity[%d] = %d", i, popularity[i], i+2, popularity[i+2])
		}
	}
}
