package g_lib

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"testing"
)

func TestMinMax(t *testing.T) {

	x := Min[int](3, 4)
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = Min[int](4, 3)
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = Max[int](3, 4)
	if x != 4 {
		t.Errorf("Min failed")
	}

	x = Max[int](4, 3)
	if x != 4 {
		t.Errorf("Min failed")
	}

}

func TestMinMaxArray(t *testing.T) {

	x := MinArray[int]([]int{3, 4, 5})
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = MinArray[int]([]int{5, 4, 3, 4, 5})
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = MaxArray[int]([]int{1, 2, 4, 2, 3})
	if x != 4 {
		t.Errorf("Min failed")
	}

	x = MaxArray[int]([]int{4, 1, 2, 4, 2, 3})
	if x != 4 {
		t.Errorf("Min failed")
	}

}

func TestInArray(t *testing.T) {
	found := InArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55})
	if !found {
		t.Errorf("Failed to find when shoudl be found in arary")
	}

	found = InArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 43, 55})
	if found {
		t.Errorf("Found in array when not there")
	}
}

func TestLocationInArray(t *testing.T) {
	loc := LocationInArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55})
	if loc != 9 {
		t.Errorf("Incorrect Location, found %d expected 9", loc)
	}

	loc = LocationInArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 43, 55})
	if loc != -1 {
		t.Errorf("Incorrect Location, should not be found")
	}
}

// func KeysForStringMap[T any](aMap map[string]T) (rv []string ) {
func TestKeyForStringMap(t *testing.T) {
	aMap := map[string]int{
		"abc": 1,
		"def": 3,
	}
	k := KeysForStringMap[int](aMap)
	if len(k) != 2 {
		t.Errorf("Incorrect Key Length")
	}
	if !InArray("abc", k) {
		t.Errorf("Incorrect Key")
	}
	if !InArray("def", k) {
		t.Errorf("Incorrect Key")
	}
}

func TestAbs(t *testing.T) {
	x := Abs[int](-4)
	if x != 4 {
		t.Errorf("Abs failed")
	}

	x = Abs[int](4)
	if x != 4 {
		t.Errorf("Abs failed")
	}

	x = Abs[int](0)
	if x != 0 {
		t.Errorf("Abs failed")
	}
}
