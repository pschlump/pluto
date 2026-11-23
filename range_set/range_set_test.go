/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package range_set

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func checkPoints(t *testing.T, set *RangeSet[int], model map[int]bool) {
	t.Helper()
	for point := -12; point <= 12; point++ {
		if got := set.Contains(point); got != model[point] {
			t.Fatalf("Contains(%d)=%v want=%v ranges=%v", point, got, model[point], set.Slice())
		}
	}
	for _, value := range set.Slice() {
		for point := -12; point <= 12; point++ {
			if value.Contains(point) && !model[point] {
				t.Fatalf("range contains unexpected %d: %v", point, value)
			}
		}
	}
}

func TestRangeSetSeededFiniteModel(t *testing.T) {
	random := rand.New(rand.NewSource(1729))
	set := NewRangeSet[int]()
	model := map[int]bool{}
	for step := 0; step < 3000; step++ {
		low, high := random.Intn(21)-10, random.Intn(21)-10
		if low > high {
			low, high = high, low
		}
		var value ranges.Range[int]
		switch random.Intn(4) {
		case 0:
			value = ranges.Closed(low, high)
		case 1:
			if low == high {
				value = ranges.ClosedOpen(low, high)
			} else {
				value = ranges.Open(low, high)
			}
		case 2:
			value = ranges.ClosedOpen(low, high)
		case 3:
			value = ranges.OpenClosed(low, high)
		}
		if random.Intn(2) == 0 {
			set.Add(value)
			for point := low; point <= high; point++ {
				if value.Contains(point) {
					model[point] = true
				}
			}
		} else {
			set.Remove(value)
			for point := low; point <= high; point++ {
				if value.Contains(point) {
					delete(model, point)
				}
			}
		}
		checkPoints(t, set, model)
	}
}

func TestRangeSetMergeSplitComplementAndSubrange(t *testing.T) {
	set := NewRangeSet[int]()
	set.Add(ranges.Closed(1, 3))
	set.Add(ranges.Closed(5, 7))
	set.Add(ranges.Closed(2, 5))
	if got := set.Slice(); len(got) != 1 || !got[0].Equal(ranges.Closed(1, 7)) {
		t.Fatal(got)
	}
	set.Remove(ranges.Closed(2, 5))
	got := set.Slice()
	if len(got) != 2 || !got[0].Equal(ranges.ClosedOpen(1, 2)) || !got[1].Equal(ranges.OpenClosed(5, 7)) {
		t.Fatal(got)
	}
	set.Remove(ranges.ClosedOpen(2, 5))
	if !set.Equal(set.Clone()) {
		t.Fatal("empty removal changed set")
	}

	set.Clear()
	set.Add(ranges.ClosedOpen(1, 3))
	set.Add(ranges.Closed(5, 7))
	complement := set.Complement().Slice()
	want := []ranges.Range[int]{ranges.LessThan(1), ranges.ClosedOpen(3, 5), ranges.GreaterThan(7)}
	if len(complement) != 3 {
		t.Fatal(complement)
	}
	for index := range want {
		if !complement[index].Equal(want[index]) {
			t.Fatal(complement)
		}
	}
	sub := set.SubRangeSet(ranges.Closed(2, 6)).Slice()
	if len(sub) != 2 || !sub[0].Equal(ranges.ClosedOpen(2, 3)) || !sub[1].Equal(ranges.Closed(5, 6)) {
		t.Fatal(sub)
	}
	span, found := set.Span()
	if !found || !span.Equal(ranges.Closed(1, 7)) {
		t.Fatal(span, found)
	}
	if !set.Encloses(ranges.ClosedOpen(3, 3)) {
		t.Fatal("empty range enclosed")
	}
}

func TestRangeSetDiscreteCanonicalMerging(t *testing.T) {
	plain := NewRangeSet[int]()
	plain.Add(ranges.Closed(1, 3))
	plain.Add(ranges.ClosedOpen(4, 6))
	if plain.Len() != 2 {
		t.Fatal("continuous ranges should not merge integer adjacency")
	}
	discrete := NewDiscreteRangeSet[int](ranges.SignedDomain[int]())
	discrete.Add(ranges.Closed(1, 3))
	discrete.Add(ranges.ClosedOpen(4, 6))
	if got := discrete.Slice(); len(got) != 1 || !got[0].Equal(ranges.ClosedOpen(1, 6)) {
		t.Fatal(got)
	}
	discrete.Remove(ranges.Open(1, 4))
	if got := discrete.Slice(); len(got) != 2 || !got[0].Equal(ranges.ClosedOpen(1, 2)) || !got[1].Equal(ranges.ClosedOpen(4, 6)) {
		t.Fatal(got)
	}
}

func TestRangeSetAllEmptyComplement(t *testing.T) {
	set := NewRangeSet[int]()
	if got := set.Complement().Slice(); len(got) != 1 || !got[0].Equal(ranges.All[int]()) {
		t.Fatal(got)
	}
	set.Add(ranges.All[int]())
	if got := set.Complement().Slice(); len(got) != 0 {
		t.Fatal(got)
	}
	if got := collectRanges(set.All()); len(got) != 1 || !got[0].Equal(ranges.All[int]()) {
		t.Fatal(got)
	}
}

func collectRanges(sequence func(func(ranges.Range[int]) bool)) []ranges.Range[int] {
	values := []ranges.Range[int]{}
	for value := range sequence {
		values = append(values, value)
	}
	return values
}
