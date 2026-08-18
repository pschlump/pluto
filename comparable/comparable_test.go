package comparable

import "testing"

type item struct {
	v int
}

var (
	_ Comparable = item{}
	_ Equality   = item{}
)

func (a item) Compare(b Comparable) int {
	return a.v - b.(item).v
}

func (a item) IsEqual(b Equality) bool {
	return a.v == b.(item).v
}

func TestComparable(t *testing.T) {
	if (item{1}).Compare(item{2}) >= 0 {
		t.Errorf("1 should compare less than 2")
	}
	if (item{2}).Compare(item{2}) != 0 {
		t.Errorf("2 should compare equal to 2")
	}
	if !(item{2}).IsEqual(item{2}) {
		t.Errorf("2 should be equal to 2")
	}
	if (item{2}).IsEqual(item{3}) {
		t.Errorf("2 should not be equal to 3")
	}
}

/* vim: set noai ts=4 sw=4: */
