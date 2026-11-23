/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue_ts

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

func TestQueueConcurrentOperationsAndSnapshots(t *testing.T) {
	queue := NewQueue[int](64)
	queue.Push(0)
	sequence := queue.All()
	queue.Push(1)
	seen := []int{}
	for value := range sequence {
		seen = append(seen, value)
	}
	if !reflect.DeepEqual(seen, []int{0}) {
		t.Fatal("iterator was not a snapshot", seen)
	}

	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				switch iteration % 5 {
				case 0, 1:
					queue.Push(worker*1000 + iteration)
				case 2:
					queue.PushAll([]int{worker, iteration, worker + iteration})
				case 3:
					queue.Pop()
				case 4:
					queue.Peek()
					queue.Len()
					queue.RemainingCapacity()
					for range queue.All() {
						break
					}
				}
			}
		}(worker)
	}
	wait.Wait()
	if queue.Len() > queue.Capacity() {
		t.Fatal("capacity exceeded")
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

func TestQueueOppositeEqualDoesNotDeadlock(t *testing.T) {
	left, right := NewQueue[int](2), NewQueue[int](2)
	left.PushAll([]int{1, 2})
	right.PushAll([]int{1, 2})
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

func TestQueueJSON(t *testing.T) {
	queue := NewQueue[int](3)
	queue.PushAll([]int{1, 2, 3, 4})
	encoded, err := json.Marshal(queue)
	if err != nil || string(encoded) != "[2,3,4]" {
		t.Fatalf("MarshalJSON=%s, %v", encoded, err)
	}
	if err := json.Unmarshal([]byte("[7,8,9,10]"), queue); err != nil {
		t.Fatal(err)
	}
	if got := queue.Slice(); !reflect.DeepEqual(got, []int{8, 9, 10}) {
		t.Fatal(got)
	}
}
