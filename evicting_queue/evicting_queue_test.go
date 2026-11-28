/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"testing"
)

func collectQueue[T any](queue *Queue[T]) []T {
	values := []T{}
	for value := range queue.All() {
		values = append(values, value)
	}
	return values
}

func checkQueueModel(t *testing.T, queue *Queue[int], model []int, capacity int) {
	t.Helper()
	if got := queue.Slice(); !slices.Equal(got, model) {
		t.Fatalf("Slice=%v want=%v", got, model)
	}
	if got := collectQueue(queue); !slices.Equal(got, model) {
		t.Fatalf("All=%v want=%v", got, model)
	}
	if queue.Len() != len(model) || queue.Capacity() != capacity || queue.RemainingCapacity() != capacity-len(model) {
		t.Fatalf("size/capacity mismatch: len=%d capacity=%d remaining=%d", queue.Len(), queue.Capacity(), queue.RemainingCapacity())
	}
	oldest, found := queue.Peek()
	if found != (len(model) > 0) || found && oldest != model[0] {
		t.Fatalf("Peek=%d/%v model=%v", oldest, found, model)
	}
}

func pushModel(model []int, values []int, capacity int) []int {
	model = append(model, values...)
	if len(model) > capacity {
		model = append([]int(nil), model[len(model)-capacity:]...)
	}
	return model
}

func TestQueueSeededModel(t *testing.T) {
	for _, capacity := range []int{0, 1, 2, 5} {
		t.Run(string(rune('A'+capacity)), func(t *testing.T) {
			random := rand.New(rand.NewSource(1729 + int64(capacity)))
			queue := NewQueue[int](capacity)
			model := []int{}
			for step := 0; step < 5000; step++ {
				switch random.Intn(5) {
				case 0, 1:
					value := random.Intn(1000)
					queue.Push(value)
					model = pushModel(model, []int{value}, capacity)
				case 2:
					values := make([]int, random.Intn(9))
					for index := range values {
						values[index] = random.Intn(1000)
					}
					queue.PushAll(values)
					model = pushModel(model, values, capacity)
				case 3:
					got, found := queue.Pop()
					wantFound := len(model) > 0
					if found != wantFound || found && got != model[0] {
						t.Fatalf("step %d Pop=%d/%v model=%v", step, got, found, model)
					}
					if wantFound {
						model = model[1:]
					}
				case 4:
					if random.Intn(20) == 0 {
						queue.Clear()
						model = []int{}
					}
				}
				checkQueueModel(t, queue, model, capacity)
			}
		})
	}
}

func TestQueueCloneEqualAndEarlyStop(t *testing.T) {
	queue := NewQueue[int](4)
	queue.PushAll([]int{1, 2, 3})
	clone := queue.Clone()
	if !queue.Equal(clone) || !queue.Equal(newQueueFromTest(1, 2, 3)) {
		t.Fatal("equal queue contents were not equal")
	}
	clone.Pop()
	clone.Push(4)
	if queue.Equal(clone) || !reflect.DeepEqual(queue.Slice(), []int{1, 2, 3}) {
		t.Fatal("clone mutation changed source")
	}
	seen := []int{}
	for value := range queue.All() {
		seen = append(seen, value)
		if len(seen) == 2 {
			break
		}
	}
	if !reflect.DeepEqual(seen, []int{1, 2}) {
		t.Fatal(seen)
	}
}

func newQueueFromTest(values ...int) *Queue[int] {
	queue := NewQueue[int](len(values))
	queue.PushAll(values)
	return queue
}

func TestQueueJSON(t *testing.T) {
	queue := NewQueue[int](3)
	queue.PushAll([]int{1, 2, 3, 4})
	encoded, err := json.Marshal(queue)
	if err != nil || string(encoded) != "[2,3,4]" {
		t.Fatalf("MarshalJSON=%s, %v", encoded, err)
	}
	if err := json.Unmarshal([]byte("[5,6,7,8]"), queue); err != nil {
		t.Fatal(err)
	}
	if got := queue.Slice(); !reflect.DeepEqual(got, []int{6, 7, 8}) {
		t.Fatal(got)
	}
	if err := json.Unmarshal([]byte("["), queue); err == nil {
		t.Fatal("malformed JSON succeeded")
	}
	if got := queue.Slice(); !reflect.DeepEqual(got, []int{6, 7, 8}) {
		t.Fatal("failed decode mutated queue", got)
	}
	zero := NewQueue[int](0)
	if err := json.Unmarshal([]byte("[1,2]"), zero); err != nil || zero.Len() != 0 {
		t.Fatal("zero-capacity decode", err, zero.Slice())
	}
}

func TestQueueClearsStaleSlots(t *testing.T) {
	queue := NewQueue[*int](3)
	values := []int{1, 2, 3}
	queue.Push(&values[0])
	queue.Push(&values[1])
	queue.Push(&values[2])
	queue.Pop()
	if queue.data[0] != nil {
		t.Fatal("popped slot retained a reference")
	}
	queue.Push(&values[0])
	queue.Clear()
	for index, value := range queue.data {
		if value != nil {
			t.Fatalf("slot %d retained a reference", index)
		}
	}
}

func TestQueueNegativeCapacityPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("negative capacity did not panic")
		}
	}()
	NewQueue[int](-1)
}

// TestQueueWrappedSegments drives every capacity 1..6, every head rotation and
// every length through the two-segment paths (All with early stop at each
// position, Slice, Clone, Equal across different rotations).
func TestQueueWrappedSegments(t *testing.T) {
	for capacity := 1; capacity <= 6; capacity++ {
		for rotation := 0; rotation < capacity; rotation++ {
			for length := 0; length <= capacity; length++ {
				queue := NewQueue[int](capacity)
				for index := 0; index < rotation; index++ { // advance head
					queue.Push(-1)
					queue.Pop()
				}
				model := []int{}
				for value := range length {
					queue.Push(value)
					model = append(model, value)
				}
				name := fmt.Sprintf("cap%d/rot%d/len%d", capacity, rotation, length)
				if got := queue.Slice(); !slices.Equal(got, model) {
					t.Fatalf("%s Slice %v want %v", name, got, model)
				}
				for stop := 0; stop <= length; stop++ {
					seen := []int{}
					for value := range queue.All() {
						if len(seen) == stop {
							break
						}
						seen = append(seen, value)
					}
					if !slices.Equal(seen, model[:stop]) {
						t.Fatalf("%s stop %d saw %v", name, stop, seen)
					}
				}
				clone := queue.Clone()
				if got := clone.Slice(); !slices.Equal(got, model) || clone.Capacity() != capacity {
					t.Fatalf("%s Clone %v cap %d", name, got, clone.Capacity())
				}
				clone.Push(99) // independence
				if got := queue.Slice(); !slices.Equal(got, model) {
					t.Fatalf("%s Clone shares storage: %v", name, got)
				}
				unrotated := NewQueue[int](capacity + 1)
				unrotated.PushAll(model)
				if !queue.Equal(unrotated) || !unrotated.Equal(queue) {
					t.Fatalf("%s Equal false across rotations", name)
				}
				if length > 0 {
					different := NewQueue[int](capacity)
					different.PushAll(model)
					different.Pop()
					different.Push(model[len(model)-1] + 1)
					if queue.Equal(different) {
						t.Fatalf("%s Equal true for different contents", name)
					}
				}
			}
		}
	}
}

// TestQueuePushAllEvictsInPlace covers the short-input PushAll path when it
// wraps and evicts.
func TestQueuePushAllEvictsInPlace(t *testing.T) {
	queue := NewQueue[int](4)
	queue.PushAll([]int{1, 2, 3})
	queue.PushAll([]int{4, 5, 6})
	if got := queue.Slice(); !slices.Equal(got, []int{3, 4, 5, 6}) {
		t.Fatal(got)
	}
	queue.Pop()
	queue.PushAll([]int{7, 8})
	if got := queue.Slice(); !slices.Equal(got, []int{5, 6, 7, 8}) {
		t.Fatal(got)
	}
}
