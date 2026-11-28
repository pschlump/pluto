/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package range_set

import (
	"cmp"
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

// The reference oracles below are the original whole-slice algorithms that
// Add/Remove/RangeContaining/Encloses replaced: every operation scans and
// rebuilds the full slice. The binary-search window implementations must
// produce byte-for-byte the same stored ranges (same bounds, same open/closed
// types) and the same query answers. One deliberate deviation from the
// original: a fragment that canonicalizes to empty is dropped (the original
// stored it — see TestRangeSetDiscreteRemoveAtMaximumStoresNoEmptyRange).

func referenceLowerCompare[T any](a, b ranges.Range[T]) int {
	aLower, aBound, aHas := a.LowerBound()
	bLower, bBound, bHas := b.LowerBound()
	if !aHas {
		if bHas {
			return -1
		}
		return 0
	}
	if !bHas {
		return 1
	}
	order := a.Compare()(aLower, bLower)
	if order == 0 && aBound != bBound {
		if aBound == ranges.OpenBound {
			return 1
		}
		return -1
	}
	return order
}

func referenceAdd[T any](s *RangeSet[T], values []ranges.Range[T], value ranges.Range[T]) []ranges.Range[T] {
	value = s.canonical(value)
	if value.IsEmpty() {
		return values
	}
	merged := value
	result := make([]ranges.Range[T], 0, len(values)+1)
	inserted := false
	for _, existing := range values {
		if s.adjacent(merged, existing) {
			merged = merged.Span(existing)
			continue
		}
		if !inserted && referenceLowerCompare(existing, merged) > 0 {
			result = append(result, merged)
			inserted = true
		}
		result = append(result, existing)
	}
	if !inserted {
		result = append(result, merged)
	}
	return result
}

func referenceRemove[T any](s *RangeSet[T], values []ranges.Range[T], value ranges.Range[T]) []ranges.Range[T] {
	value = s.canonical(value)
	if value.IsEmpty() {
		return values
	}
	result := make([]ranges.Range[T], 0, len(values))
	for _, existing := range values {
		if !existing.IsConnected(value) || existing.Intersection(value).IsEmpty() {
			result = append(result, existing)
			continue
		}
		if value.Encloses(existing) {
			continue
		}
		oldLower, oldLowerBound, oldHasLower := existing.LowerBound()
		oldUpper, oldUpperBound, oldHasUpper := existing.UpperBound()
		cutLower, cutLowerBound, cutHasLower := value.LowerBound()
		cutUpper, cutUpperBound, cutHasUpper := value.UpperBound()
		left := false
		if cutHasLower {
			order := s.compare(oldLower, cutLower)
			left = !oldHasLower || order < 0 || order == 0 && oldLowerBound == ranges.ClosedBound && cutLowerBound == ranges.OpenBound
		}
		if left {
			if fragment := s.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true); !fragment.IsEmpty() {
				result = append(result, fragment)
			}
		}
		right := false
		if cutHasUpper {
			order := s.compare(cutUpper, oldUpper)
			right = !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
		}
		if right {
			if fragment := s.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper); !fragment.IsEmpty() {
				result = append(result, fragment)
			}
		}
	}
	return result
}

func referenceContaining[T any](values []ranges.Range[T], point T) (ranges.Range[T], bool) {
	for _, candidate := range values {
		if candidate.Contains(point) {
			return candidate, true
		}
	}
	return ranges.Range[T]{}, false
}

func referenceEncloses[T any](values []ranges.Range[T], value ranges.Range[T]) bool {
	if value.IsEmpty() {
		return true
	}
	for _, candidate := range values {
		if candidate.Encloses(value) {
			return true
		}
	}
	return false
}

// universe maps the integers 0..size-1 onto the element type; ranges span at
// most width consecutive universe points, so a large size with a small width
// keeps many disjoint ranges stored at once.
type universe[T any] struct {
	size, width int
	at          func(int) T
}

func (u universe[T]) point(random *rand.Rand) T { return u.at(random.Intn(u.size)) }

// randomRange draws any range shape. Unbounded shapes are rare so that they do
// not collapse the stored slice to one range.
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

func sameRanges[T any](got, want []ranges.Range[T]) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if !got[index].Equal(want[index]) {
			return false
		}
	}
	return true
}

func runReferenceModel[T any](t *testing.T, set *RangeSet[T], seed int64, steps int, u universe[T]) {
	t.Helper()
	random := rand.New(rand.NewSource(seed))
	var model []ranges.Range[T]
	maxLen := 0
	defer func() { t.Logf("largest stored slice: %d ranges", maxLen) }()
	for step := 0; step < steps; step++ {
		value := randomRange(random, set.compare, u)
		if random.Intn(5) < 3 {
			model = referenceAdd(set, model, value)
			set.Add(value)
		} else {
			model = referenceRemove(set, model, value)
			set.Remove(value)
		}
		if random.Intn(1500) == 0 {
			model = nil
			set.Clear()
		}
		maxLen = max(maxLen, len(model))
		if got := set.Slice(); !sameRanges(got, model) {
			t.Fatalf("step %d after %v: got %v want %v", step, value, got, model)
		}
		for probe := 0; probe < 6; probe++ {
			point := u.point(random)
			gotRange, gotFound := set.RangeContaining(point)
			wantRange, wantFound := referenceContaining(model, point)
			if gotFound != wantFound || gotFound && !gotRange.Equal(wantRange) {
				t.Fatalf("step %d RangeContaining(%v) = %v/%v want %v/%v (ranges %v)", step, point, gotRange, gotFound, wantRange, wantFound, model)
			}
			if set.Contains(point) != wantFound {
				t.Fatalf("step %d Contains(%v) disagrees", step, point)
			}
			query := randomRange(random, set.compare, u)
			if got, want := set.Encloses(query), referenceEncloses(model, query); got != want {
				t.Fatalf("step %d Encloses(%v) = %v want %v (ranges %v)", step, query, got, want, model)
			}
		}
	}
}

func identity(index int) int { return index }

func TestRangeSetMatchesReferenceContinuous(t *testing.T) {
	runReferenceModel(t, NewRangeSet[int](), 1729, 6000, universe[int]{size: 41, width: 6, at: identity})
}

func TestRangeSetMatchesReferenceWideUniverse(t *testing.T) {
	// A sparse universe keeps many disjoint ranges stored, so the binary
	// searches run over long slices rather than a handful of entries.
	runReferenceModel(t, NewRangeSet[int](), 42, 6000, universe[int]{size: 1200, width: 4, at: identity})
}

func TestRangeSetMatchesReferenceDiscrete(t *testing.T) {
	runReferenceModel(t, NewDiscreteRangeSet[int](ranges.SignedDomain[int]()), 7, 6000, universe[int]{size: 300, width: 4, at: identity})
}

func TestRangeSetMatchesReferenceDiscreteExtrema(t *testing.T) {
	// int8 endpoints cluster at both extrema, where canonicalization keeps a
	// closed upper bound (no successor) and an exhausted open lower bound
	// becomes empty.
	at := func(index int) int8 {
		if index < 8 {
			return int8(-128 + index)
		}
		return int8(112 + index)
	}
	runReferenceModel(t, NewDiscreteRangeSet[int8](ranges.SignedDomain[int8]()), 99, 6000, universe[int8]{size: 16, width: 3, at: at})
}

func TestRangeSetMatchesReferenceReversedComparator(t *testing.T) {
	reverse := func(a, b int) int { return cmp.Compare(b, a) }
	runReferenceModel(t, NewRangeSetFunc(reverse), 2026, 6000, universe[int]{size: 300, width: 4, at: identity})
}

func TestRangeSetMutationReusesStorage(t *testing.T) {
	set := NewRangeSet[int]()
	for index := 0; index < 64; index += 2 {
		set.Add(ranges.ClosedOpen(index, index+1))
	}
	// Merging into, splitting, and trimming stored ranges at steady size must
	// not reallocate the backing array (the warm-up run absorbs the one
	// amortized growth). Ranges are built outside the measured function.
	a, b, c, d := ranges.ClosedOpen(10, 11), ranges.Closed(20, 21), ranges.Open(20, 21), ranges.ClosedOpen(21, 22)
	e := ranges.ClosedOpen(20, 21)
	found := false
	allocs := testing.AllocsPerRun(100, func() {
		set.Add(a)
		set.Remove(a)
		set.Add(b)
		set.Remove(c)
		set.Remove(d)
		set.Add(e)
		found = set.Contains(30) || found
	})
	if !found {
		t.Fatal("Contains(30) lost the range [30,31)")
	}
	if allocs != 0 {
		t.Fatalf("steady-state mutation allocated %v times per run", allocs)
	}
}

func TestRangeSetDiscreteRemoveAtMaximumStoresNoEmptyRange(t *testing.T) {
	// Removing [120,127] from [100,+inf) over int8 leaves (127,+inf), which
	// canonicalizes to the empty (127,127). It must be dropped: before
	// 2026-11-27 it was stored, breaking Len and the sorted-disjoint invariant
	// the binary searches rely on.
	set := NewDiscreteRangeSet[int8](ranges.SignedDomain[int8]())
	set.Add(ranges.AtLeast[int8](100))
	set.Remove(ranges.Closed[int8](120, 127))
	if got := set.Slice(); len(got) != 1 || !got[0].Equal(ranges.ClosedOpen[int8](100, 120)) {
		t.Fatalf("got %v, want only [100,120)", got)
	}
	if set.Contains(127) || !set.Contains(119) {
		t.Fatal("containment wrong after removal at the domain maximum")
	}
}
