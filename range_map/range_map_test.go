/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package range_map

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func checkMapPoints(t *testing.T, actual *RangeMap[int, string], model map[int]string) {
	t.Helper()
	for point := -12; point <= 12; point++ {
		got, found := actual.Get(point)
		want, expected := model[point]
		if got != want || found != expected {
			t.Fatalf("Get(%d)=%q/%v want=%q/%v", point, got, found, want, expected)
		}
	}
	for _, entry := range actual.EntriesSlice() {
		for point := -12; point <= 12; point++ {
			if entry.Range.Contains(point) && model[point] != entry.Value {
				t.Fatalf("entry mismatch at %d: %v", point, entry)
			}
		}
	}
}

func TestRangeMapSeededFiniteModel(t *testing.T) {
	random := rand.New(rand.NewSource(1729))
	actual := NewRangeMap[int, string]()
	model := map[int]string{}
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
			mapped := string(rune('a' + random.Intn(3)))
			actual.Put(value, mapped)
			for point := low; point <= high; point++ {
				if value.Contains(point) {
					model[point] = mapped
				}
			}
		} else {
			actual.Remove(value)
			for point := low; point <= high; point++ {
				if value.Contains(point) {
					delete(model, point)
				}
			}
		}
		checkMapPoints(t, actual, model)
	}
}

func TestRangeMapSplitBoundariesAndNoCoalescing(t *testing.T) {
	actual := NewRangeMap[int, string]()
	actual.Put(ranges.Closed(1, 10), "a")
	actual.Put(ranges.Closed(3, 5), "b")
	entries := actual.EntriesSlice()
	if len(entries) != 3 || !entries[0].Range.Equal(ranges.ClosedOpen(1, 3)) || entries[0].Value != "a" || !entries[1].Range.Equal(ranges.Closed(3, 5)) || entries[1].Value != "b" || !entries[2].Range.Equal(ranges.OpenClosed(5, 10)) || entries[2].Value != "a" {
		t.Fatal(entries)
	}
	actual.Remove(ranges.Open(2, 4))
	entries = actual.EntriesSlice()
	if len(entries) != 3 || !entries[0].Range.Equal(ranges.Closed(1, 2)) || !entries[1].Range.Equal(ranges.Closed(4, 5)) || !entries[2].Range.Equal(ranges.OpenClosed(5, 10)) {
		t.Fatal(entries)
	}

	actual.Clear()
	actual.Put(ranges.Closed(1, 3), "same")
	actual.Put(ranges.Closed(3, 5), "same")
	if actual.Len() != 2 {
		t.Fatal("adjacent equal values coalesced")
	}
	sub := actual.SubRangeMap(ranges.Closed(2, 4)).EntriesSlice()
	if len(sub) != 2 || !sub[0].Range.Equal(ranges.ClosedOpen(2, 3)) || !sub[1].Range.Equal(ranges.Closed(3, 4)) {
		t.Fatal(sub)
	}
	if value, found := actual.Get(3); !found || value != "same" {
		t.Fatal(value, found)
	}
	if !actual.Equal(actual.Clone(), func(a, b string) bool { return a == b }) {
		t.Fatal("clone equality")
	}
}

func TestRangeMapEmptyAndUnbounded(t *testing.T) {
	actual := NewDiscreteRangeMap[int, string](ranges.SignedDomain[int]())
	actual.Put(ranges.ClosedOpen(2, 2), "x")
	if actual.Len() != 0 {
		t.Fatal("empty put")
	}
	actual.Put(ranges.LessThan(0), "negative")
	actual.Put(ranges.AtLeast(0), "nonnegative")
	if value, _ := actual.Get(-1); value != "negative" {
		t.Fatal(value)
	}
	if value, _ := actual.Get(0); value != "nonnegative" {
		t.Fatal(value)
	}
	actual.Remove(ranges.All[int]())
	if !actual.IsEmpty() {
		t.Fatal("remove all")
	}
}
