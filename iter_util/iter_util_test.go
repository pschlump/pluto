package iter_util_test

import (
	"iter"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/pschlump/pluto/iter_util"
)

func collect[T any](sequence iter.Seq[T]) []T {
	return slices.Collect(sequence)
}

func TestLazyTransforms(t *testing.T) {
	values := slices.Values([]int{1, 2, 3, 4, 5, 6})
	got := collect(iter_util.Map(iter_util.Filter(values, func(value int) bool {
		return value%2 == 0
	}), func(value int) int { return value * 10 }))
	if want := []int{20, 40, 60}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	got = collect(iter_util.Concat(slices.Values([]int{1, 2}), slices.Values([]int{}), slices.Values([]int{3})))
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("concat got %v, want %v", got, want)
	}
	if got = collect(iter_util.Take(values, 3)); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("take got %v", got)
	}
	if got = collect(iter_util.Skip(values, 4)); !reflect.DeepEqual(got, []int{5, 6}) {
		t.Fatalf("skip got %v", got)
	}
	if got = collect(iter_util.Skip(values, 20)); len(got) != 0 {
		t.Fatalf("long skip got %v", got)
	}
}

func TestInfiniteUntilTakeAndEarlyTermination(t *testing.T) {
	calls := 0
	infinite := func(yield func(int) bool) {
		for value := 0; ; value++ {
			calls++
			if !yield(value) {
				return
			}
		}
	}
	if got := collect(iter_util.Take(infinite, 5)); !reflect.DeepEqual(got, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("got %v", got)
	}
	if calls != 5 {
		t.Fatalf("producer called %d times, want 5", calls)
	}
	calls = 0
	if got := collect(iter_util.Take(infinite, 0)); len(got) != 0 || calls != 0 {
		t.Fatalf("zero take got %v with %d calls", got, calls)
	}

	panicAfterTwo := func(yield func(int) bool) {
		if !yield(1) {
			return
		}
		if !yield(2) {
			return
		}
		panic("over-consumed")
	}
	if _, found := iter_util.First(panicAfterTwo); !found {
		t.Fatal("first did not find value")
	}
	if _, ok := iter_util.Only(panicAfterTwo); ok {
		t.Fatal("only accepted two values")
	}
}

func TestPartition(t *testing.T) {
	parts := collect(iter_util.Partition(slices.Values([]int{1, 2, 3, 4, 5}), 2))
	if want := [][]int{{1, 2}, {3, 4}, {5}}; !reflect.DeepEqual(parts, want) {
		t.Fatalf("got %v, want %v", parts, want)
	}
	parts[0][0] = 99
	if parts[1][0] != 3 {
		t.Fatal("partitions alias each other")
	}
	for _, test := range []struct {
		values []int
		size   int
		want   int
	}{{nil, 2, 0}, {[]int{1}, 1, 1}, {[]int{1, 2}, 2, 1}, {[]int{1, 2, 3}, 2, 2}} {
		if got := len(collect(iter_util.Partition(slices.Values(test.values), test.size))); got != test.want {
			t.Errorf("Partition(%v, %d) produced %d parts, want %d", test.values, test.size, got, test.want)
		}
	}
	calls := 0
	producer := func(yield func(int) bool) {
		for _, value := range []int{1, 2, 3} {
			calls++
			if !yield(value) {
				return
			}
		}
	}
	for range iter_util.Take(iter_util.Partition(producer, 2), 1) {
	}
	if calls != 2 {
		t.Fatalf("partition consumed %d values, want 2", calls)
	}
}

func TestQueriesAndEquality(t *testing.T) {
	values := slices.Values([]int{1, 2, 2, 3})
	if got := iter_util.Frequency(values); !maps.Equal(got, map[int]int{1: 1, 2: 2, 3: 1}) {
		t.Fatalf("frequency %v", got)
	}
	if !iter_util.ElementsEqual(slices.Values([]int{1, 2}), slices.Values([]int{1, 2})) {
		t.Fatal("equal sequences differ")
	}
	for _, test := range [][2][]int{{{1}, {1, 2}}, {{1, 2}, {1}}, {{1, 2}, {1, 3}}, {{}, {1}}} {
		if iter_util.ElementsEqual(slices.Values(test[0]), slices.Values(test[1])) {
			t.Errorf("%v and %v reported equal", test[0], test[1])
		}
	}
	if value, ok := iter_util.Last(values); !ok || value != 3 {
		t.Fatalf("last = %v, %v", value, ok)
	}
	if _, ok := iter_util.Last(slices.Values([]int{})); ok {
		t.Fatal("empty last found a value")
	}
	if value, ok := iter_util.Only(slices.Values([]int{7})); !ok || value != 7 {
		t.Fatalf("only = %v, %v", value, ok)
	}
	if _, ok := iter_util.Only(slices.Values([]int{})); ok {
		t.Fatal("empty only found a value")
	}
	for _, test := range []struct {
		index int
		want  int
		ok    bool
	}{{-1, 0, false}, {0, 1, true}, {2, 2, true}, {3, 3, true}, {4, 0, false}} {
		value, ok := iter_util.At(values, test.index)
		if value != test.want || ok != test.ok {
			t.Errorf("At(%d) = %d, %v; want %d, %v", test.index, value, ok, test.want, test.ok)
		}
	}
	if !iter_util.IsEmpty(slices.Values([]int{})) || iter_util.IsEmpty(values) {
		t.Fatal("IsEmpty mismatch")
	}
	if got := iter_util.Len(values); got != 4 {
		t.Fatalf("Len = %d", got)
	}
}

func TestSliceAndPairAdapters(t *testing.T) {
	input := []int{1, 2, 3}
	sequence := iter_util.FromSlice(input)
	input[0] = 99
	if got := iter_util.ToSlice(sequence); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("snapshot got %v", got)
	}
	pairs := []iter_util.Pair[string, int]{{First: "a", Second: 1}, {First: "b", Second: 2}}
	seq2 := iter_util.Seq2(slices.Values(pairs))
	if got := collect(iter_util.Keys(seq2)); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("keys %v", got)
	}
	seq2 = iter_util.Seq2(slices.Values(pairs))
	if got := collect(iter_util.Values(seq2)); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("values %v", got)
	}
	if got := collect(iter_util.Pairs(maps.All(map[string]int{"a": 1}))); !reflect.DeepEqual(got, pairs[:1]) {
		t.Fatalf("pairs %v", got)
	}
}

func TestPeeking(t *testing.T) {
	calls := 0
	producer := func(yield func(int) bool) {
		for _, value := range []int{4, 5} {
			calls++
			if !yield(value) {
				return
			}
		}
	}
	cursor := iter_util.NewPeeking(producer)
	if calls != 0 {
		t.Fatal("constructor started producer")
	}
	for range 2 {
		if value, ok := cursor.Peek(); !ok || value != 4 {
			t.Fatalf("Peek = %d, %v", value, ok)
		}
	}
	if calls != 1 {
		t.Fatalf("Peek consumed %d values", calls)
	}
	if value, ok := cursor.Next(); !ok || value != 4 {
		t.Fatalf("Next = %d, %v", value, ok)
	}
	if value, ok := cursor.Next(); !ok || value != 5 {
		t.Fatalf("Next = %d, %v", value, ok)
	}
	if _, ok := cursor.Peek(); ok {
		t.Fatal("exhausted Peek found value")
	}
	cursor.Close()

	cursor = iter_util.NewPeeking(producer)
	if _, ok := cursor.Peek(); !ok {
		t.Fatal("new cursor is empty")
	}
	cursor.Close()
	cursor.Close()
	if _, ok := cursor.Next(); ok {
		t.Fatal("closed cursor returned value")
	}
}

func TestInvalidArguments(t *testing.T) {
	for name, function := range map[string]func(){
		"take":      func() { iter_util.Take(slices.Values([]int{1}), -1) },
		"skip":      func() { iter_util.Skip(slices.Values([]int{1}), -1) },
		"partition": func() { iter_util.Partition(slices.Values([]int{1}), 0) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			function()
		})
	}
}
