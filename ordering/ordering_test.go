package ordering_test

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/pschlump/pluto/ordering"
)

func TestComposition(t *testing.T) {
	natural := ordering.Natural[int]()
	if natural.Compare(1, 2) >= 0 || natural.Compare(2, 1) <= 0 || natural.Compare(1, 1) != 0 {
		t.Fatal("natural comparison mismatch")
	}
	if got := natural.Reverse().SortedCopy([]int{1, 3, 2}); !reflect.DeepEqual(got, []int{3, 2, 1}) {
		t.Fatalf("reverse got %v", got)
	}
	type record struct {
		Group int
		Name  string
	}
	byGroup := ordering.ByKey(func(value record) int { return value.Group }, natural)
	byName := ordering.ByKey(func(value record) string { return value.Name }, ordering.Natural[string]())
	values := []record{{2, "a"}, {1, "z"}, {1, "a"}}
	want := []record{{1, "a"}, {1, "z"}, {2, "a"}}
	if got := byGroup.Then(byName).SortedCopy(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("composed got %v, want %v", got, want)
	}
	allEqual := ordering.AllEqual[int]()
	if !allEqual.IsOrdered(slices.Values([]int{3, 1, 2})) || allEqual.IsStrictlyOrdered(slices.Values([]int{1, 2})) {
		t.Fatal("all-equal ordering mismatch")
	}
}

func TestLexicographical(t *testing.T) {
	order := ordering.Lexicographical(ordering.Natural[int]())
	values := [][]int{{1, 2}, {1}, {1, 2, 0}, {}, {0, 9}}
	want := [][]int{{}, {0, 9}, {1}, {1, 2}, {1, 2, 0}}
	if got := order.SortedCopy(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtremaAndOrdered(t *testing.T) {
	order := ordering.Natural[int]()
	if value, ok := order.Min(slices.Values([]int{4, 2, 2, 8})); !ok || value != 2 {
		t.Fatalf("Min = %d, %v", value, ok)
	}
	if value, ok := order.Max(slices.Values([]int{4, 8, 2, 8})); !ok || value != 8 {
		t.Fatalf("Max = %d, %v", value, ok)
	}
	if _, ok := order.Min(slices.Values([]int{})); ok {
		t.Fatal("empty Min found value")
	}
	if _, ok := order.Max(slices.Values([]int{})); ok {
		t.Fatal("empty Max found value")
	}
	for _, test := range []struct {
		values  []int
		ordered bool
		strict  bool
	}{{nil, true, true}, {[]int{1}, true, true}, {[]int{1, 1, 2}, true, false}, {[]int{1, 2, 3}, true, true}, {[]int{2, 1}, false, false}} {
		if got := order.IsOrdered(slices.Values(test.values)); got != test.ordered {
			t.Errorf("IsOrdered(%v) = %v", test.values, got)
		}
		if got := order.IsStrictlyOrdered(slices.Values(test.values)); got != test.strict {
			t.Errorf("IsStrictlyOrdered(%v) = %v", test.values, got)
		}
	}
	producer := func(yield func(int) bool) {
		if !yield(2) {
			return
		}
		if !yield(1) {
			return
		}
		panic("ordered check over-consumed")
	}
	if order.IsOrdered(producer) {
		t.Fatal("descending values reported ordered")
	}
}

func TestSortedCopyStable(t *testing.T) {
	type item struct {
		Key int
		ID  int
	}
	order := ordering.ByKey(func(value item) int { return value.Key }, ordering.Natural[int]())
	input := []item{{1, 0}, {0, 1}, {1, 2}, {0, 3}, {1, 4}}
	want := []item{{0, 1}, {0, 3}, {1, 0}, {1, 2}, {1, 4}}
	original := append([]item(nil), input...)
	if got := order.SortedCopy(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("SortedCopy modified input")
	}
}

func TestTopKAgainstSort(t *testing.T) {
	random := rand.New(rand.NewPCG(1729, 91))
	order := ordering.Natural[int]()
	for size := 0; size <= 100; size++ {
		values := make([]int, size)
		for index := range values {
			values[index] = random.IntN(31) - 15
		}
		ascending := order.SortedCopy(values)
		descending := order.Reverse().SortedCopy(values)
		for _, count := range []int{0, 1, 2, 7, size, size + 1} {
			wantLeast := ascending[:min(count, size)]
			wantGreatest := descending[:min(count, size)]
			if got := order.LeastOf(slices.Values(values), count); !slices.Equal(got, wantLeast) {
				t.Fatalf("LeastOf(n=%d, k=%d) = %v, want %v", size, count, got, wantLeast)
			}
			if got := order.GreatestOf(slices.Values(values), count); !slices.Equal(got, wantGreatest) {
				t.Fatalf("GreatestOf(n=%d, k=%d) = %v, want %v", size, count, got, wantGreatest)
			}
		}
	}
}

func TestNegativeTopKPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	ordering.Natural[int]().LeastOf(slices.Values([]int{1}), -1)
}
