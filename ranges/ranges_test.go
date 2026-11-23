/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package ranges

import (
	"math"
	"reflect"
	"testing"
)

func mustPanic(t *testing.T, operation func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("operation did not panic")
		}
	}()
	operation()
}

func collectRangeValues[T any](set *ContiguousSet[T]) []T {
	values := []T{}
	for value := range set.All() {
		values = append(values, value)
	}
	return values
}

func TestRangeBoundsContainmentAndEmptiness(t *testing.T) {
	tests := []struct {
		value    Range[int]
		contains []int
		excludes []int
		empty    bool
	}{
		{Closed(1, 3), []int{1, 2, 3}, []int{0, 4}, false},
		{Open(1, 3), []int{2}, []int{1, 3}, false},
		{ClosedOpen(1, 3), []int{1, 2}, []int{0, 3}, false},
		{OpenClosed(1, 3), []int{2, 3}, []int{1, 4}, false},
		{Closed(2, 2), []int{2}, nil, false},
		{ClosedOpen(2, 2), nil, []int{2}, true},
		{OpenClosed(2, 2), nil, []int{2}, true},
		{GreaterThan(2), []int{3, 100}, []int{2, 0}, false},
		{AtLeast(2), []int{2, 3}, []int{1}, false},
		{LessThan(2), []int{1, -100}, []int{2, 3}, false},
		{AtMost(2), []int{2, 1}, []int{3}, false},
		{All[int](), []int{math.MinInt, 0, math.MaxInt}, nil, false},
	}
	for _, test := range tests {
		for _, value := range test.contains {
			if !test.value.Contains(value) {
				t.Fatalf("%v should contain %d", test.value, value)
			}
		}
		for _, value := range test.excludes {
			if test.value.Contains(value) {
				t.Fatalf("%v should exclude %d", test.value, value)
			}
		}
		if test.value.IsEmpty() != test.empty {
			t.Fatalf("IsEmpty=%v for %v", test.value.IsEmpty(), test.value)
		}
	}
	mustPanic(t, func() { Open(2, 2) })
	mustPanic(t, func() { Closed(3, 2) })
	mustPanic(t, func() { OpenFunc[int](nil, 1, 2) })
}

func TestRangeConnectionIntersectionSpanAndEncloses(t *testing.T) {
	closed := Closed(1, 3)
	openRight := Closed(3, 5)
	intersection := closed.Intersection(openRight)
	if intersection.IsEmpty() || !intersection.Contains(3) {
		t.Fatal("closed/open touching intersection", intersection)
	}
	empty := Open(1, 3).Intersection(Closed(3, 5))
	if !empty.IsEmpty() || empty.Contains(3) {
		t.Fatal("touching intersection should be empty", empty)
	}
	if Open(1, 3).IsConnected(Open(3, 5)) {
		t.Fatal("open/open ranges are disconnected")
	}
	mustPanic(t, func() { Open(1, 3).Intersection(Open(3, 5)) })
	span := Open(1, 3).Span(Closed(5, 7))
	if !span.Equal(OpenClosed(1, 7)) || !span.Contains(4) {
		t.Fatal("span did not cover the gap", span)
	}
	if !All[int]().Encloses(span) || !ClosedOpen(1, 4).Encloses(Open(1, 4)) || ClosedOpen(1, 4).Encloses(Closed(1, 4)) {
		t.Fatal("enclosure boundary mismatch")
	}
	if !Closed(1, 3).Encloses(ClosedOpen(3, 3)) {
		t.Fatal("range should enclose empty endpoint range")
	}
}

func TestFloatingPointInfinities(t *testing.T) {
	value := ClosedOpen(0.0, math.Inf(1))
	if !value.Contains(math.MaxFloat64) || value.Contains(math.Inf(1)) || value.Contains(math.Inf(-1)) {
		t.Fatal("infinite endpoint containment")
	}
	if !AtMost(math.Inf(1)).Contains(math.Inf(1)) || !GreaterThan(math.Inf(-1)).Contains(0.0) {
		t.Fatal("one-sided infinite range")
	}
}

func TestExhaustiveSmallIntersectionAndSpan(t *testing.T) {
	values := []Range[int]{}
	for low := -1; low <= 1; low++ {
		for high := low; high <= 1; high++ {
			values = append(values, Closed(low, high), ClosedOpen(low, high), OpenClosed(low, high))
			if low != high {
				values = append(values, Open(low, high))
			}
		}
	}
	for _, left := range values {
		for _, right := range values {
			if !left.IsConnected(right) {
				continue
			}
			intersection := left.Intersection(right)
			span := left.Span(right)
			for point := -2; point <= 2; point++ {
				if got, want := intersection.Contains(point), left.Contains(point) && right.Contains(point); got != want {
					t.Fatalf("intersection at %d: left=%v right=%v result=%v", point, left, right, intersection)
				}
				if (left.Contains(point) || right.Contains(point)) && !span.Contains(point) {
					t.Fatal("span omitted an input point")
				}
			}
		}
	}
}

func TestCustomComparatorRange(t *testing.T) {
	byLength := func(a, b string) int { return len(a) - len(b) }
	value := ClosedFunc(byLength, "a", "bbb")
	if !value.Contains("xx") || value.Contains("long") {
		t.Fatal("custom comparison failed")
	}
}

func TestCheckedDiscreteDomains(t *testing.T) {
	signed := SignedDomain[int8]()
	if _, ok := signed.Next(math.MaxInt8); ok {
		t.Fatal("signed successor overflow")
	}
	if _, ok := signed.Prev(math.MinInt8); ok {
		t.Fatal("signed predecessor overflow")
	}
	if got, ok := signed.Distance(math.MinInt8, math.MaxInt8); !ok || got != 255 {
		t.Fatal(got, ok)
	}
	if _, ok := signed.Distance(1, 0); ok {
		t.Fatal("reversed distance")
	}
	unsigned := UnsignedDomain[uint8]()
	if _, ok := unsigned.Next(255); ok {
		t.Fatal("unsigned successor overflow")
	}
	if _, ok := unsigned.Prev(0); ok {
		t.Fatal("unsigned predecessor overflow")
	}
	if got, ok := unsigned.Distance(0, 255); !ok || got != 255 {
		t.Fatal(got, ok)
	}
	fullSigned := SignedDomain[int64]()
	if got, ok := fullSigned.Distance(math.MinInt64, math.MaxInt64); !ok || got != math.MaxUint64 {
		t.Fatal(got, ok)
	}
}

func TestCanonicalAndContiguousSet(t *testing.T) {
	domain := SignedDomain[int]()
	canonical := Open(0, 5).Canonical(domain)
	if !canonical.Equal(ClosedOpen(1, 5)) {
		t.Fatal("canonical open range", canonical)
	}
	if !ClosedOpen(0, 5).Canonical(domain).Equal(ClosedOpen(0, 5)) {
		t.Fatal("canonical closed-open")
	}
	if !GreaterThan(math.MaxInt).Canonical(domain).IsEmpty() {
		t.Fatal("exhausted successor should be empty")
	}
	if !NewContiguousSet(LessThan(math.MinInt), domain).IsEmpty() {
		t.Fatal("range below domain minimum should be empty")
	}
	if !AtMost(math.MaxInt).Canonical(domain).Contains(math.MaxInt) {
		t.Fatal("maximum closed upper endpoint was lost")
	}

	set := NewContiguousSet(OpenClosed(1, 5), domain)
	if got := collectRangeValues(set); !reflect.DeepEqual(got, []int{2, 3, 4, 5}) {
		t.Fatal(got)
	}
	if size, ok := set.Len(); !ok || size != 4 {
		t.Fatal(size, ok)
	}
	if rank, ok := set.Rank(4); !ok || rank != 2 {
		t.Fatal(rank, ok)
	}
	if _, ok := set.Rank(1); ok {
		t.Fatal("non-member rank")
	}
	seen := 0
	for range NewContiguousSet(All[int8](), SignedDomain[int8]()).All() {
		seen++
		break
	}
	if seen != 1 {
		t.Fatal("infinite/lazy early stop")
	}
	all8 := NewContiguousSet(All[int8](), SignedDomain[int8]())
	if size, ok := all8.Len(); !ok || size != 256 {
		t.Fatal(size, ok)
	}
	allUint64 := NewContiguousSet(All[uint64](), UnsignedDomain[uint64]())
	if _, ok := allUint64.Len(); ok {
		t.Fatal("2^64 size should overflow uint64")
	}
	if got := collectRangeValues(NewContiguousSet(ClosedOpen(2, 2), domain)); len(got) != 0 {
		t.Fatal(got)
	}
}
