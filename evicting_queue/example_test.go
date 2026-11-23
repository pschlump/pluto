/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue_test

import (
	"fmt"

	"github.com/pschlump/pluto/evicting_queue"
)

func ExampleQueue() {
	queue := evicting_queue.NewQueue[int](3)
	queue.PushAll([]int{1, 2, 3, 4, 5})
	fmt.Println(queue.Slice())
	oldest, _ := queue.Pop()
	queue.Push(6)
	fmt.Println(oldest, queue.Slice(), queue.RemainingCapacity())
	// Output:
	// [3 4 5]
	// 3 [4 5 6] 0
}
