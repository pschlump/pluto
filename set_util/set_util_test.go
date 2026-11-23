/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package set_util

import (
	"reflect"
	"testing"
)

func TestCartesian(t *testing.T) {
	got := [][]int{}
	for tuple := range Cartesian([]int{1, 2}, []int{3, 4}) {
		got = append(got, tuple)
	}
	want := [][]int{{1, 3}, {1, 4}, {2, 3}, {2, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	got[0][0] = 99
	if got[1][0] != 1 {
		t.Fatal("yielded tuples alias")
	}
	count := 0
	for range Cartesian([]int{1, 2}, []int{3}, []int{4}) {
		count++
	}
	if count != 2 {
		t.Fatal(count)
	}
	for range Cartesian([]int{1}, []int{}) {
		t.Fatal("empty input produced tuple")
	}
	for tuple := range Cartesian[int]() {
		if len(tuple) != 0 {
			t.Fatal(tuple)
		}
		return
	}
	t.Fatal("empty product should yield one empty tuple")
}

func TestPowerSetAndEarlyStop(t *testing.T) {
	got := [][]int{}
	for subset := range PowerSet([]int{1, 2}) {
		got = append(got, subset)
	}
	want := [][]int{{}, {1}, {2}, {1, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	count := 0
	for range PowerSet(make([]int, 100)) {
		count++
		break
	}
	if count != 1 {
		t.Fatal("power set did not stop")
	}
}
