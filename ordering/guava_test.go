package ordering_test

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/pschlump/pluto/ordering"
)

type guavaOrderingVector struct {
	SortedCopy      []int     `json:"sortedCopy"`
	Reverse         []int     `json:"reverse"`
	LeastOf         []int     `json:"leastOf"`
	GreatestOf      []int     `json:"greatestOf"`
	Lexicographical [][]int   `json:"lexicographical"`
	Min             int       `json:"min"`
	Max             int       `json:"max"`
	Ordered         []bool    `json:"ordered"`
	StrictlyOrdered []bool    `json:"strictlyOrdered"`
	StableByKey     []keyItem `json:"stableByKey"`
}

type keyItem struct {
	Key int `json:"key"`
	ID  int `json:"id"`
}

func TestGuavaOrderingVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_ordering.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector guavaOrderingVector
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	values := []int{4, 1, 7, 1, 9, 3, 5}
	natural := ordering.Natural[int]()
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"sorted", natural.SortedCopy(values), vector.SortedCopy},
		{"reverse", natural.Reverse().SortedCopy(values), vector.Reverse},
		{"least", natural.LeastOf(slices.Values(values), 3), vector.LeastOf},
		{"greatest", natural.GreatestOf(slices.Values(values), 3), vector.GreatestOf},
		{"lexicographical", ordering.Lexicographical(natural).SortedCopy([][]int{{1, 2}, {1}, {1, 2, 0}, {}, {0, 9}}), vector.Lexicographical},
	}
	for _, check := range checks {
		if !reflect.DeepEqual(check.got, check.want) {
			t.Errorf("%s = %v, want %v", check.name, check.got, check.want)
		}
	}
	minimum, _ := natural.Min(slices.Values(values))
	maximum, _ := natural.Max(slices.Values(values))
	if minimum != vector.Min || maximum != vector.Max {
		t.Errorf("extrema = %d,%d; want %d,%d", minimum, maximum, vector.Min, vector.Max)
	}
	ordered := []bool{natural.IsOrdered(slices.Values([]int{1, 1, 2})), natural.IsOrdered(slices.Values([]int{2, 1}))}
	strict := []bool{natural.IsStrictlyOrdered(slices.Values([]int{1, 1, 2})), natural.IsStrictlyOrdered(slices.Values([]int{1, 2, 3}))}
	if !reflect.DeepEqual(ordered, vector.Ordered) || !reflect.DeepEqual(strict, vector.StrictlyOrdered) {
		t.Errorf("ordered vectors = %v,%v", ordered, strict)
	}
	items := []keyItem{{1, 0}, {0, 1}, {1, 2}, {0, 3}, {1, 4}}
	byKey := ordering.ByKey(func(item keyItem) int { return item.Key }, natural)
	if got := byKey.SortedCopy(items); !reflect.DeepEqual(got, vector.StableByKey) {
		t.Errorf("stable = %v, want %v", got, vector.StableByKey)
	}
}
