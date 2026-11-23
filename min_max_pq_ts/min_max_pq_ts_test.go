/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq_ts

import (
	"encoding/json"
	"slices"
	"sync"
	"testing"
)

func TestMinMaxPriorityQueueConcurrentOperationsAndSnapshots(t *testing.T) {
	queue := NewBoundedMinMaxPriorityQueue[int](128)
	queue.Push(0)
	sequence := queue.All()
	queue.Push(1)
	seen := []int{}
	for value := range sequence {
		seen = append(seen, value)
	}
	if !slices.Equal(seen, []int{0}) {
		t.Fatal("iterator was not a snapshot", seen)
	}

	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				switch iteration % 6 {
				case 0, 1:
					queue.Push(worker*1000 + iteration)
				case 2:
					queue.PushAll([]int{worker, iteration, worker + iteration})
				case 3:
					queue.PopMin()
				case 4:
					queue.PopMax()
				case 5:
					queue.PeekMin()
					queue.PeekMax()
					queue.Len()
					queue.MaximumSize()
					for range queue.All() {
						break
					}
				}
			}
		}(worker)
	}
	wait.Wait()
	if queue.Len() > 128 {
		t.Fatal("bound exceeded")
	}
	clone := queue.Clone()
	if !queue.Equal(clone) {
		t.Fatal("clone differs from source")
	}
	clone.Clear()
	if queue.Len() == 0 {
		t.Fatal("clone clear changed source")
	}
}

func TestMinMaxPriorityQueueOppositeEqualDoesNotDeadlock(t *testing.T) {
	left, right := NewMinMaxPriorityQueue[int](), NewMinMaxPriorityQueue[int]()
	left.PushAll([]int{3, 1, 2})
	right.PushAll([]int{2, 3, 1})
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for range 1000 {
			left.Equal(right)
		}
	}()
	go func() {
		defer wait.Done()
		for range 1000 {
			right.Equal(left)
		}
	}()
	wait.Wait()
}

func TestMinMaxPriorityQueueJSON(t *testing.T) {
	queue := NewBoundedMinMaxPriorityQueue[int](3)
	queue.PushAll([]int{4, 1, 3, 2, 9})
	if err := json.Unmarshal([]byte("[8,7,6,5,2]"), queue); err != nil {
		t.Fatal(err)
	}
	values := queue.Slice()
	slices.Sort(values)
	if !slices.Equal(values, []int{2, 5, 6}) {
		t.Fatal(values)
	}
	encoded, err := json.Marshal(queue)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []int
	if err := json.Unmarshal(encoded, &decoded); err != nil || !slices.Equal(decoded, queue.Slice()) {
		t.Fatal("JSON mismatch", decoded, queue.Slice())
	}
}
