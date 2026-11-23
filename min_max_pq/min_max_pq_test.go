/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq

import (
	"cmp"
	"encoding/json"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func checkHeapStructure[T any](t *testing.T, queue *MinMaxPriorityQueue[T]) {
	t.Helper()
	for index := 3; index < len(queue.data); index++ {
		grandparent := ((index-1)/2 - 1) / 2
		order := queue.compare(queue.data[index], queue.data[grandparent])
		if isMinLevel(index) && order < 0 || !isMinLevel(index) && order > 0 {
			t.Fatalf("heap violation at index %d (grandparent %d): data=%v", index, grandparent, queue.data)
		}
	}
	if maximum, bounded := queue.MaximumSize(); bounded && queue.Len() > maximum {
		t.Fatalf("Len=%d exceeds maximum=%d", queue.Len(), maximum)
	}
}

func checkHeapModel(t *testing.T, queue *MinMaxPriorityQueue[int], model []int) {
	t.Helper()
	got := queue.Slice()
	slices.Sort(got)
	if !slices.Equal(got, model) {
		t.Fatalf("contents=%v want=%v (heap=%v)", got, model, queue.data)
	}
	if queue.Len() != len(model) || queue.IsEmpty() != (len(model) == 0) {
		t.Fatalf("Len=%d model=%d", queue.Len(), len(model))
	}
	minimum, minFound := queue.PeekMin()
	maximum, maxFound := queue.PeekMax()
	if minFound != (len(model) > 0) || maxFound != minFound {
		t.Fatalf("empty peek mismatch: %v %v", minFound, maxFound)
	}
	if minFound && (minimum != model[0] || maximum != model[len(model)-1]) {
		t.Fatalf("peeks=%d/%d want=%d/%d", minimum, maximum, model[0], model[len(model)-1])
	}
}

func pushHeapModel(model []int, value, maximumSize int) []int {
	model = append(model, value)
	slices.Sort(model)
	if maximumSize > 0 && len(model) > maximumSize {
		model = model[:maximumSize]
	}
	return model
}

func TestMinMaxPriorityQueueSeededModel(t *testing.T) {
	for _, maximumSize := range []int{-1, 1, 2, 7} {
		queue := NewMinMaxPriorityQueue[int]()
		if maximumSize > 0 {
			queue = NewBoundedMinMaxPriorityQueue[int](maximumSize)
		}
		random := rand.New(rand.NewSource(1729 + int64(maximumSize)))
		model := []int{}
		for step := 0; step < 6000; step++ {
			choice := random.Intn(100)
			switch {
			case choice < 45:
				value := random.Intn(41) - 20
				queue.Push(value)
				model = pushHeapModel(model, value, maximumSize)
			case choice < 65:
				got, found := queue.PopMin()
				if found != (len(model) > 0) || found && got != model[0] {
					t.Fatalf("step %d PopMin=%d/%v model=%v", step, got, found, model)
				}
				if found {
					model = model[1:]
				}
			case choice < 85:
				got, found := queue.PopMax()
				if found != (len(model) > 0) || found && got != model[len(model)-1] {
					t.Fatalf("step %d PopMax=%d/%v model=%v", step, got, found, model)
				}
				if found {
					model = model[:len(model)-1]
				}
			case choice < 95:
				values := []int{random.Intn(41) - 20, random.Intn(41) - 20, random.Intn(41) - 20}
				queue.PushAll(values)
				for _, value := range values {
					model = pushHeapModel(model, value, maximumSize)
				}
			default:
				queue.Clear()
				model = []int{}
			}
			checkHeapStructure(t, queue)
			if step%17 == 0 {
				checkHeapModel(t, queue, model)
			}
		}
		checkHeapModel(t, queue, model)
		for len(model) > 0 {
			var got int
			if len(model)%2 == 0 {
				got, _ = queue.PopMin()
				if got != model[0] {
					t.Fatal("minimum drain", got, model)
				}
				model = model[1:]
			} else {
				got, _ = queue.PopMax()
				if got != model[len(model)-1] {
					t.Fatal("maximum drain", got, model)
				}
				model = model[:len(model)-1]
			}
			checkHeapStructure(t, queue)
		}
	}
}

func TestMinMaxPriorityQueueCustomComparatorAndEqual(t *testing.T) {
	type item struct {
		key  string
		data []int
	}
	compare := func(a, b item) int {
		if order := strings.Compare(strings.ToLower(a.key), strings.ToLower(b.key)); order != 0 {
			return order
		}
		return cmp.Compare(len(a.data), len(b.data))
	}
	left := NewMinMaxPriorityQueueFunc(compare)
	right := NewMinMaxPriorityQueueFunc(compare)
	left.PushAll([]item{{key: "B"}, {key: "a"}, {key: "C"}, {key: "a"}})
	right.PushAll([]item{{key: "c"}, {key: "A"}, {key: "b"}, {key: "A"}})
	if !left.Equal(right) {
		t.Fatal("equivalent multisets differ")
	}
	clone := left.Clone()
	if !left.Equal(clone) {
		t.Fatal("clone differs")
	}
	clone.PopMin()
	if left.Len() != 4 || left.Equal(clone) {
		t.Fatal("clone mutation changed source")
	}
	if minimum, _ := left.PeekMin(); minimum.key != "a" {
		t.Fatal("custom comparator minimum", minimum)
	}
	if maximum, _ := left.PeekMax(); maximum.key != "C" {
		t.Fatal("custom comparator maximum", maximum)
	}
}

func TestMinMaxPriorityQueueJSONAndEarlyStop(t *testing.T) {
	queue := NewBoundedMinMaxPriorityQueue[int](3)
	queue.PushAll([]int{4, 1, 3, 2, 9})
	encoded, err := json.Marshal(queue)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	if err := json.Unmarshal(encoded, &values); err != nil || !reflect.DeepEqual(values, queue.Slice()) {
		t.Fatal("JSON layout differs from Slice", values, queue.Slice())
	}
	if err := json.Unmarshal([]byte("[8,7,6,5,2]"), queue); err != nil {
		t.Fatal(err)
	}
	got := queue.Slice()
	slices.Sort(got)
	if !slices.Equal(got, []int{2, 5, 6}) {
		t.Fatal("bounded JSON replacement", got)
	}
	before := queue.Slice()
	if err := json.Unmarshal([]byte("["), queue); err == nil || !reflect.DeepEqual(before, queue.Slice()) {
		t.Fatal("failed decode changed queue")
	}
	seen := 0
	for range queue.All() {
		seen++
		break
	}
	if seen != 1 {
		t.Fatal("early stop failed")
	}
}

func TestMinMaxPriorityQueueReleasesElements(t *testing.T) {
	queue := NewMinMaxPriorityQueueFunc[*int](func(a, b *int) int { return cmp.Compare(*a, *b) })
	values := []int{1, 2, 3, 4}
	for index := range values {
		queue.Push(&values[index])
	}
	queue.Clear()
	if queue.data != nil {
		t.Fatal("Clear retained heap storage")
	}
}

func TestMinMaxPriorityQueueZeroValueMaximumSize(t *testing.T) {
	var queue MinMaxPriorityQueue[int]
	if size, bounded := queue.MaximumSize(); bounded || size != 0 {
		t.Fatalf("zero-value MaximumSize=%d/%v", size, bounded)
	}
}

func TestMinMaxPriorityQueueInvalidConstructors(t *testing.T) {
	for _, test := range []func(){
		func() { NewMinMaxPriorityQueueFunc[int](nil) },
		func() { NewBoundedMinMaxPriorityQueue[int](0) },
		func() { NewBoundedMinMaxPriorityQueueFunc[int](1, nil) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid constructor did not panic")
				}
			}()
			test()
		}()
	}
}
