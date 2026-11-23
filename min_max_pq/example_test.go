/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq_test

import (
	"fmt"

	"github.com/pschlump/pluto/min_max_pq"
)

func ExampleMinMaxPriorityQueue() {
	queue := min_max_pq.NewBoundedMinMaxPriorityQueue[int](3)
	queue.PushAll([]int{8, 2, 5, 1, 9, 4})
	minimum, _ := queue.PopMin()
	maximum, _ := queue.PopMax()
	fmt.Println(minimum, maximum, queue.Len())
	// Output: 1 4 1
}
