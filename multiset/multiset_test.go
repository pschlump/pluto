/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"iter"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type intMultiset interface {
	Add(int, int) int
	Remove(int, int) int
	SetCount(int, int) int
	Count(int) int
	Len() int
	DistinctLen() int
	Entries() iter.Seq[Entry[int]]
	All() iter.Seq[int]
}

func TestGuavaGolden(t *testing.T) {
	trace, err := guava_compare.ReadTrace("testdata/multiset.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/multiset.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		new  func() intMultiset
	}{
		{name: "hash", new: func() intMultiset { return NewHashMultiset[int]() }},
		{name: "tree", new: func() intMultiset { return NewTreeMultiset[int]() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			multiset := test.new()
			results := make([]int, 0, len(trace.Operations))
			for _, operation := range trace.Operations {
				value := 0
				if operation.Value != "" {
					if _, err := fmt.Sscanf(operation.Value, "key-%04d", &value); err != nil {
						t.Fatal(err)
					}
				}
				switch operation.Op {
				case "add":
					multiset.Add(value, operation.Count)
					results = append(results, multiset.Count(value))
				case "remove":
					multiset.Remove(value, operation.Count)
					results = append(results, multiset.Count(value))
				case "setCount":
					multiset.SetCount(value, operation.Count)
					results = append(results, multiset.Count(value))
				case "count":
					results = append(results, multiset.Count(value))
				case "size":
					results = append(results, multiset.Len())
				case "distinctSize":
					results = append(results, multiset.DistinctLen())
				}
			}
			entries := make([]Entry[int], 0, multiset.DistinctLen())
			for entry := range multiset.Entries() {
				entries = append(entries, entry)
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Value < entries[j].Value })
			actual := guava_compare.TraceResult{
				Version:     guava_compare.TraceVersion,
				Fixture:     guava_compare.FixtureName,
				GuavaCommit: guava_compare.GuavaCommit,
				Results:     results,
				Final: guava_compare.FinalState{
					Size:         multiset.Len(),
					DistinctSize: multiset.DistinctLen(),
					Entries:      convertEntries(entries),
				},
			}
			encoded, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			encoded = append(encoded, '\n')
			if !reflect.DeepEqual(encoded, expected) {
				t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
			}
		})
	}
}

func convertEntries(entries []Entry[int]) []guava_compare.Entry {
	result := make([]guava_compare.Entry, len(entries))
	for index, entry := range entries {
		result[index] = guava_compare.Entry{Value: "key-" + fourDigits(entry.Value), Count: entry.Count}
	}
	return result
}

func fourDigits(value int) string {
	digits := strconv.Itoa(value)
	return strings.Repeat("0", 4-len(digits)) + digits
}

func TestSeededModels(t *testing.T) {
	for _, test := range []struct {
		name string
		new  func() intMultiset
	}{
		{name: "hash", new: func() intMultiset { return NewHashMultiset[int]() }},
		{name: "tree", new: func() intMultiset { return NewTreeMultiset[int]() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			random := rand.New(rand.NewSource(42))
			multiset := test.new()
			model := make(map[int]int)
			total := 0
			for step := 0; step < 3000; step++ {
				value := random.Intn(24)
				count := random.Intn(7)
				switch random.Intn(4) {
				case 0:
					multiset.Add(value, count)
					model[value] += count
					total += count
				case 1:
					multiset.Remove(value, count)
					removed := min(count, model[value])
					model[value] -= removed
					total -= removed
				case 2:
					old := model[value]
					multiset.SetCount(value, count)
					model[value] = count
					total += count - old
				case 3:
					if got := multiset.Count(value); got != model[value] {
						t.Fatalf("step %d Count(%d)=%d, want %d", step, value, got, model[value])
					}
				}
				if model[value] == 0 {
					delete(model, value)
				}
				if step%25 == 0 {
					verifyModel(t, multiset, model, total)
				}
			}
			verifyModel(t, multiset, model, total)
			if tree, ok := multiset.(*TreeMultiset[int]); ok {
				checkAVL(t, tree.root)
			}
		})
	}
}

func verifyModel(t *testing.T, multiset intMultiset, model map[int]int, total int) {
	t.Helper()
	if multiset.Len() != total || multiset.DistinctLen() != len(model) {
		t.Fatalf("sizes=(%d,%d), want (%d,%d)", multiset.Len(), multiset.DistinctLen(), total, len(model))
	}
	for value := 0; value < 24; value++ {
		if got := multiset.Count(value); got != model[value] {
			t.Fatalf("Count(%d)=%d, want %d", value, got, model[value])
		}
	}
	occurrences := 0
	entries := 0
	for entry := range multiset.Entries() {
		entries++
		if entry.Count <= 0 || model[entry.Value] != entry.Count {
			t.Fatalf("bad entry %+v", entry)
		}
	}
	for range multiset.All() {
		occurrences++
	}
	if entries != len(model) || occurrences != total {
		t.Fatalf("iteration entries=%d occurrences=%d", entries, occurrences)
	}
}

func checkAVL(t *testing.T, root *treeNode[int]) {
	t.Helper()
	var check func(*treeNode[int], *int, *int) (int, int, int)
	check = func(node *treeNode[int], minimum, maximum *int) (int, int, int) {
		if node == nil {
			return 0, 0, 0
		}
		if node.count <= 0 || (minimum != nil && node.value <= *minimum) || (maximum != nil && node.value >= *maximum) {
			t.Fatalf("invalid node %+v", node)
		}
		leftHeight, leftSize, leftDistinct := check(node.left, minimum, &node.value)
		rightHeight, rightSize, rightDistinct := check(node.right, &node.value, maximum)
		if abs(leftHeight-rightHeight) > 1 || node.height != 1+max(leftHeight, rightHeight) {
			t.Fatalf("invalid AVL height at %d", node.value)
		}
		return node.height, leftSize + rightSize + node.count, leftDistinct + rightDistinct + 1
	}
	_, _, _ = check(root, nil, nil)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func TestAlgebraAndAliases(t *testing.T) {
	for _, tree := range []bool{false, true} {
		newSet := func(values ...int) intMultiset {
			if tree {
				return FromTreeSlice(values)
			}
			return FromSlice(values)
		}
		a := newSet(1, 1, 1, 2, 2, 4)
		b := newSet(1, 2, 2, 2, 3)
		check := func(name string, got intMultiset, want map[int]int) {
			t.Helper()
			verifyModel(t, got, want, sumCounts(want))
		}
		check("union", unionOf(a, b, tree), map[int]int{1: 3, 2: 3, 3: 1, 4: 1})
		check("sum", sumOf(a, b, tree), map[int]int{1: 4, 2: 5, 3: 1, 4: 1})
		check("intersection", intersectionOf(a, b, tree), map[int]int{1: 1, 2: 2})
		check("minus", minusOf(a, b, tree), map[int]int{1: 2, 4: 1})
		if tree {
			typed := FromTreeSlice([]int{1, 1, 2})
			typed.Sum(typed, typed)
			check("tree alias", typed, map[int]int{1: 4, 2: 2})
		} else {
			typed := FromSlice([]int{1, 1, 2})
			typed.Sum(typed, typed)
			check("hash alias", typed, map[int]int{1: 4, 2: 2})
		}
	}
}

func sumCounts(counts map[int]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func unionOf(a, b intMultiset, tree bool) intMultiset {
	if tree {
		result := &TreeMultiset[int]{}
		result.Union(a.(*TreeMultiset[int]), b.(*TreeMultiset[int]))
		return result
	}
	result := &HashMultiset[int]{}
	result.Union(a.(*HashMultiset[int]), b.(*HashMultiset[int]))
	return result
}

func sumOf(a, b intMultiset, tree bool) intMultiset {
	if tree {
		result := &TreeMultiset[int]{}
		result.Sum(a.(*TreeMultiset[int]), b.(*TreeMultiset[int]))
		return result
	}
	result := &HashMultiset[int]{}
	result.Sum(a.(*HashMultiset[int]), b.(*HashMultiset[int]))
	return result
}

func intersectionOf(a, b intMultiset, tree bool) intMultiset {
	if tree {
		result := &TreeMultiset[int]{}
		result.Intersection(a.(*TreeMultiset[int]), b.(*TreeMultiset[int]))
		return result
	}
	result := &HashMultiset[int]{}
	result.Intersection(a.(*HashMultiset[int]), b.(*HashMultiset[int]))
	return result
}

func minusOf(a, b intMultiset, tree bool) intMultiset {
	if tree {
		result := &TreeMultiset[int]{}
		result.Minus(a.(*TreeMultiset[int]), b.(*TreeMultiset[int]))
		return result
	}
	result := &HashMultiset[int]{}
	result.Minus(a.(*HashMultiset[int]), b.(*HashMultiset[int]))
	return result
}

func TestTreeNavigationAndRanges(t *testing.T) {
	set := FromTreeSlice([]int{1, 1, 2, 3, 3, 3, 5, 8})
	if value, ok := set.FindMin(); !ok || value != 1 {
		t.Fatal(value, ok)
	}
	if value, ok := set.FindMax(); !ok || value != 8 {
		t.Fatal(value, ok)
	}
	if value, ok := set.Floor(4); !ok || value != 3 {
		t.Fatal(value, ok)
	}
	if value, ok := set.Ceil(4); !ok || value != 5 {
		t.Fatal(value, ok)
	}
	if got := set.SubRange(2, true, 5, false).Slice(); !reflect.DeepEqual(got, []int{2, 3, 3, 3}) {
		t.Fatal(got)
	}
	if got := set.SubRange(2, false, 3, true).Slice(); !reflect.DeepEqual(got, []int{3, 3, 3}) {
		t.Fatal(got)
	}
	if got := set.HeadRange(3, false).Slice(); !reflect.DeepEqual(got, []int{1, 1, 2}) {
		t.Fatal(got)
	}
	if got := set.TailRange(3, true).Slice(); !reflect.DeepEqual(got, []int{3, 3, 3, 5, 8}) {
		t.Fatal(got)
	}
	if got := set.SubRange(3, false, 3, true).Len(); got != 0 {
		t.Fatal(got)
	}
	rangeCopy := set.TailRange(2, true)
	rangeCopy.SetCount(3, 20)
	if set.Count(3) != 3 {
		t.Fatal("range snapshot aliases source")
	}
	backward := make([]int, 0, set.Len())
	for value := range set.Backward() {
		backward = append(backward, value)
	}
	if !sort.SliceIsSorted(backward, func(i, j int) bool { return backward[i] > backward[j] }) {
		t.Fatal(backward)
	}
	checkAVL(t, rangeCopy.root)
}

func TestJSONAndHighestCount(t *testing.T) {
	for _, test := range []struct {
		name string
		new  func() intMultiset
	}{
		{name: "hash", new: func() intMultiset { return FromSlice([]int{1, 1, 2}) }},
		{name: "tree", new: func() intMultiset { return FromTreeSlice([]int{1, 1, 2}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := test.new()
			encoded, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			decoded := test.new()
			if err := json.Unmarshal(encoded, decoded); err != nil {
				t.Fatal(err)
			}
			verifyModel(t, decoded, map[int]int{1: 2, 2: 1}, 3)
			before := decoded.Len()
			if err := json.Unmarshal([]byte(`[{"value":1,"count":-2}]`), decoded); err == nil || decoded.Len() != before {
				t.Fatal("negative JSON count did not fail atomically")
			}
			switch value := decoded.(type) {
			case *HashMultiset[int]:
				entries := value.HighestCountFirst()
				if entries[0].Value != 1 || entries[0].Count != 2 {
					t.Fatal(entries)
				}
			case *TreeMultiset[int]:
				entries := value.HighestCountFirst()
				if entries[0].Value != 1 || entries[0].Count != 2 {
					t.Fatal(entries)
				}
			}
		})
	}
}

func TestCustomFunctions(t *testing.T) {
	type item struct {
		key  string
		data []int
	}
	eq := func(a, b item) bool { return strings.EqualFold(a.key, b.key) }
	hash := func(a item) uint64 {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(strings.ToLower(a.key)))
		return hash.Sum64()
	}
	hashSet := FromSliceFunc(eq, hash, []item{{key: "A", data: []int{1}}, {key: "a", data: []int{2}}})
	if hashSet.Len() != 2 || hashSet.DistinctLen() != 1 || hashSet.Count(item{key: "A"}) != 2 {
		t.Fatal("custom hash equality failed")
	}
	for value := range hashSet.Elements() {
		if len(value.data) != 1 || value.data[0] != 2 {
			t.Fatal("duplicate did not replace the stored value")
		}
	}
	compare := func(a, b item) int { return strings.Compare(strings.ToLower(a.key), strings.ToLower(b.key)) }
	treeSet := FromTreeSliceFunc(compare, []item{{key: "b"}, {key: "A"}, {key: "a"}})
	if treeSet.Len() != 3 || treeSet.DistinctLen() != 2 {
		t.Fatal("custom tree comparison failed")
	}
}

func TestOverflowAndNegativeCounts(t *testing.T) {
	hash := NewHashMultiset[int]()
	hash.SetCount(1, maxInt)
	assertPanic(t, func() { hash.Add(1, 1) })
	if hash.Len() != maxInt || hash.Count(1) != maxInt || hash.DistinctLen() != 1 {
		t.Fatal("overflow changed hash multiset")
	}
	assertPanic(t, func() { hash.Add(2, -1) })
	hashOther := FromSlice([]int{2})
	assertPanic(t, func() { hash.Sum(hash, hashOther) })
	if hash.Len() != maxInt || hash.Count(2) != 0 {
		t.Fatal("failed Sum changed hash multiset")
	}
	tree := NewTreeMultiset[int]()
	tree.SetCount(1, maxInt)
	assertPanic(t, func() { tree.Add(1, 1) })
	if tree.Len() != maxInt || tree.Count(1) != maxInt || tree.DistinctLen() != 1 {
		t.Fatal("overflow changed tree multiset")
	}
	assertPanic(t, func() { tree.SetCount(2, -1) })
	treeOther := FromTreeSlice([]int{2})
	assertPanic(t, func() { tree.Sum(tree, treeOther) })
	if tree.Len() != maxInt || tree.Count(2) != 0 {
		t.Fatal("failed Sum changed tree multiset")
	}
}

func assertPanic(t *testing.T, operation func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	operation()
}
