/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package map_util

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pschlump/pluto/immutable"
)

func TestUniqueIndex(t *testing.T) {
	values := []string{"a", "bb"}
	indexed, err := UniqueIndex(values, func(value string) int { return len(value) })
	if err != nil {
		t.Fatal(err)
	}
	if got, found := indexed.Get(2); !found || got != "bb" {
		t.Fatal(got, found)
	}
	values[0] = "changed"
	if got, _ := indexed.Get(1); got != "a" {
		t.Fatal("index retained mutable source")
	}
	if _, err := UniqueIndex([]string{"a", "b"}, func(string) int { return 1 }); !errors.Is(err, immutable.ErrDuplicateKey) {
		t.Fatal(err)
	}
}

func TestUniqueIndexCustomEquality(t *testing.T) {
	equal := func(a, b string) bool { return strings.EqualFold(a, b) }
	hash := func(value string) uint64 { return uint64(len(value)) }
	if _, err := UniqueIndexFunc(equal, hash, []string{"A", "a"}, func(value string) string { return value }); !errors.Is(err, immutable.ErrDuplicateKey) {
		t.Fatal("custom duplicate accepted", err)
	}
}

func TestDifferenceFuncNonComparable(t *testing.T) {
	left := map[string][]int{"common": {1}, "different": {2}, "left": {3}}
	right := map[string][]int{"common": {1}, "different": {9}, "right": {4}}
	difference := DifferenceFunc(left, right, func(a, b []int) bool { return reflect.DeepEqual(a, b) })
	if len(difference.Common) != 1 || len(difference.Differing) != 1 || len(difference.LeftOnly) != 1 || len(difference.RightOnly) != 1 {
		t.Fatal(difference)
	}
	if got := difference.Differing["different"]; !reflect.DeepEqual(got.Left, []int{2}) || !reflect.DeepEqual(got.Right, []int{9}) {
		t.Fatal(got)
	}
}
