/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package range_map

import (
	"cmp"
	"math/rand"
	"sort"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

// The reference oracles below are the original whole-slice algorithms that
// Put/Remove/RangeContaining replaced: every mutation scans and rebuilds the
// full slice, and Put stable-sorts the result. The binary-search window
// implementations must produce exactly the same entries (same bounds, same
// open/closed types, same values) and the same lookups. One deliberate
// deviation from the original: a fragment that canonicalizes to empty is
// dropped (the original stored it — see
// TestRangeMapDiscretePutAtMaximumStoresNoEmptyEntry).

func referenceSubtract[T any, V any](m *RangeMap[T, V], entries []Entry[T, V], value ranges.Range[T]) []Entry[T, V] {
	result := make([]Entry[T, V], 0, len(entries)+1)
	for _, existing := range entries {
		if !existing.Range.IsConnected(value) || existing.Range.Intersection(value).IsEmpty() {
			result = append(result, existing)
			continue
		}
		oldLower, oldLowerBound, oldHasLower := existing.Range.LowerBound()
		oldUpper, oldUpperBound, oldHasUpper := existing.Range.UpperBound()
		cutLower, cutLowerBound, cutHasLower := value.LowerBound()
		cutUpper, cutUpperBound, cutHasUpper := value.UpperBound()
		if cutHasLower {
			order := m.compare(oldLower, cutLower)
			left := !oldHasLower || order < 0 || order == 0 && oldLowerBound == ranges.ClosedBound && cutLowerBound == ranges.OpenBound
			if left {
				if fragment := m.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true); !fragment.IsEmpty() {
					result = append(result, Entry[T, V]{Range: fragment, Value: existing.Value})
				}
			}
		}
		if cutHasUpper {
			order := m.compare(cutUpper, oldUpper)
			right := !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
			if right {
				if fragment := m.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper); !fragment.IsEmpty() {
					result = append(result, Entry[T, V]{Range: fragment, Value: existing.Value})
				}
			}
		}
	}
	return result
}

func referencePut[T any, V any](m *RangeMap[T, V], entries []Entry[T, V], value ranges.Range[T], mapped V) []Entry[T, V] {
	value = m.canonical(value)
	if value.IsEmpty() {
		return entries
	}
	result := referenceSubtract(m, entries, value)
	result = append(result, Entry[T, V]{Range: value, Value: mapped})
	sort.SliceStable(result, func(i, j int) bool { return lowerCompare(result[i], result[j].Range) < 0 })
	return result
}

func referenceRemove[T any, V any](m *RangeMap[T, V], entries []Entry[T, V], value ranges.Range[T]) []Entry[T, V] {
	value = m.canonical(value)
	if value.IsEmpty() {
		return entries
	}
	return referenceSubtract(m, entries, value)
}

func referenceContaining[T any, V any](entries []Entry[T, V], point T) (ranges.Range[T], V, bool) {
	for _, entry := range entries {
		if entry.Range.Contains(point) {
			return entry.Range, entry.Value, true
		}
	}
	return ranges.Range[T]{}, *new(V), false
}

type universe[T any] struct {
	size, width int
	at          func(int) T
}

func (u universe[T]) point(random *rand.Rand) T { return u.at(random.Intn(u.size)) }

// randomRange draws any range shape. Unbounded shapes are rare so that they do
// not collapse the map to a few entries.
func randomRange[T any](random *rand.Rand, compare func(T, T) int, u universe[T]) ranges.Range[T] {
	first := random.Intn(u.size)
	low, high := u.at(first), u.at(min(u.size-1, first+random.Intn(u.width)))
	if compare(low, high) > 0 {
		low, high = high, low
	}
	switch kind := random.Intn(1000); {
	case kind < 200:
		return ranges.ClosedFunc(compare, low, high)
	case kind < 400:
		if compare(low, high) == 0 {
			return ranges.ClosedOpenFunc(compare, low, high)
		}
		return ranges.OpenFunc(compare, low, high)
	case kind < 700:
		return ranges.ClosedOpenFunc(compare, low, high)
	case kind < 980:
		return ranges.OpenClosedFunc(compare, low, high)
	case kind < 984:
		return ranges.GreaterThanFunc(compare, low)
	case kind < 988:
		return ranges.AtLeastFunc(compare, low)
	case kind < 992:
		return ranges.LessThanFunc(compare, high)
	case kind < 996:
		return ranges.AtMostFunc(compare, high)
	case kind < 998:
		return ranges.AllFunc(compare)
	default:
		return ranges.ClosedOpenFunc(compare, low, low) // empty: a no-op
	}
}

func sameEntries[T any](got, want []Entry[T, int]) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if !got[index].Range.Equal(want[index].Range) || got[index].Value != want[index].Value {
			return false
		}
	}
	return true
}

func runReferenceModel[T any](t *testing.T, mapped *RangeMap[T, int], seed int64, steps int, u universe[T]) {
	t.Helper()
	random := rand.New(rand.NewSource(seed))
	var model []Entry[T, int]
	maxLen := 0
	defer func() { t.Logf("largest stored slice: %d entries", maxLen) }()
	for step := 0; step < steps; step++ {
		value := randomRange(random, mapped.compare, u)
		if random.Intn(5) < 3 {
			label := random.Intn(1000)
			model = referencePut(mapped, model, value, label)
			mapped.Put(value, label)
		} else {
			model = referenceRemove(mapped, model, value)
			mapped.Remove(value)
		}
		if random.Intn(1500) == 0 {
			model = nil
			mapped.Clear()
		}
		maxLen = max(maxLen, len(model))
		if got := mapped.EntriesSlice(); !sameEntries(got, model) {
			t.Fatalf("step %d after %v: got %v want %v", step, value, got, model)
		}
		for probe := 0; probe < 6; probe++ {
			point := u.point(random)
			gotRange, gotValue, gotFound := mapped.RangeContaining(point)
			wantRange, wantValue, wantFound := referenceContaining(model, point)
			if gotFound != wantFound || gotFound && (!gotRange.Equal(wantRange) || gotValue != wantValue) {
				t.Fatalf("step %d RangeContaining(%v) = %v/%v/%v want %v/%v/%v (entries %v)", step, point, gotRange, gotValue, gotFound, wantRange, wantValue, wantFound, model)
			}
			if value, found := mapped.Get(point); found != wantFound || value != wantValue {
				t.Fatalf("step %d Get(%v) = %v/%v want %v/%v", step, point, value, found, wantValue, wantFound)
			}
		}
	}
}

func identity(index int) int { return index }

func TestRangeMapMatchesReferenceContinuous(t *testing.T) {
	runReferenceModel(t, NewRangeMap[int, int](), 1729, 6000, universe[int]{size: 41, width: 6, at: identity})
}

func TestRangeMapMatchesReferenceWideUniverse(t *testing.T) {
	// A sparse universe keeps many entries stored, so the binary searches run
	// over long slices rather than a handful of entries.
	runReferenceModel(t, NewRangeMap[int, int](), 42, 6000, universe[int]{size: 1200, width: 4, at: identity})
}

func TestRangeMapMatchesReferenceDiscrete(t *testing.T) {
	runReferenceModel(t, NewDiscreteRangeMap[int, int](ranges.SignedDomain[int]()), 7, 6000, universe[int]{size: 300, width: 4, at: identity})
}

func TestRangeMapMatchesReferenceDiscreteExtrema(t *testing.T) {
	at := func(index int) int8 {
		if index < 8 {
			return int8(-128 + index)
		}
		return int8(112 + index)
	}
	runReferenceModel(t, NewDiscreteRangeMap[int8, int](ranges.SignedDomain[int8]()), 99, 6000, universe[int8]{size: 16, width: 3, at: at})
}

func TestRangeMapMatchesReferenceReversedComparator(t *testing.T) {
	reverse := func(a, b int) int { return cmp.Compare(b, a) }
	runReferenceModel(t, NewRangeMapFunc[int, int](reverse), 2026, 6000, universe[int]{size: 300, width: 4, at: identity})
}

func TestRangeMapMutationReusesStorage(t *testing.T) {
	mapped := NewRangeMap[int, int]()
	for index := 0; index < 64; index += 2 {
		mapped.Put(ranges.ClosedOpen(index, index+1), index)
	}
	// Overwriting, splitting, and trimming entries at steady size must not
	// reallocate the backing array (the warm-up run absorbs the one amortized
	// growth). Ranges are built outside the measured function.
	a, b, c := ranges.ClosedOpen(10, 11), ranges.Open(20, 21), ranges.Closed(20, 21)
	d := ranges.ClosedOpen(20, 21)
	found := false
	allocs := testing.AllocsPerRun(100, func() {
		mapped.Put(a, 1)
		mapped.Put(c, 2)
		mapped.Put(b, 3)
		mapped.Remove(c)
		mapped.Put(d, 20)
		_, ok := mapped.Get(30)
		found = ok || found
	})
	if !found {
		t.Fatal("Get(30) lost the entry [30,31)")
	}
	if allocs != 0 {
		t.Fatalf("steady-state mutation allocated %v times per run", allocs)
	}
}

func TestRangeMapDiscretePutAtMaximumStoresNoEmptyEntry(t *testing.T) {
	// Overwriting [120,127] inside [100,+inf) over int8 leaves (127,+inf),
	// which canonicalizes to the empty (127,127). It must be dropped: before
	// 2026-11-27 it was stored, breaking Len and the sorted-disjoint invariant
	// the binary searches rely on.
	mapped := NewDiscreteRangeMap[int8, int](ranges.SignedDomain[int8]())
	mapped.Put(ranges.AtLeast[int8](100), 1)
	mapped.Put(ranges.Closed[int8](120, 127), 2)
	got := mapped.EntriesSlice()
	if len(got) != 2 || !got[0].Range.Equal(ranges.ClosedOpen[int8](100, 120)) || got[0].Value != 1 ||
		!got[1].Range.Equal(ranges.Closed[int8](120, 127)) || got[1].Value != 2 {
		t.Fatalf("got %v, want [100,120)=1 and [120,127]=2", got)
	}
	if value, found := mapped.Get(127); !found || value != 2 {
		t.Fatalf("Get(127) = %v/%v, want 2/true", value, found)
	}
	mapped.Remove(ranges.AtLeast[int8](110))
	if got := mapped.EntriesSlice(); len(got) != 1 || !got[0].Range.Equal(ranges.ClosedOpen[int8](100, 110)) {
		t.Fatalf("after Remove got %v, want only [100,110)", got)
	}
}
