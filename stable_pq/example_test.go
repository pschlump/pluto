/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stable_pq_test

import (
	"fmt"

	"github.com/pschlump/pluto/stable_pq"
)

// Job is a scheduler job: the comparator orders by priority only, so
// jobs with the same priority are ties — the queue hands them back in
// submission order.
type Job struct {
	Priority int
	Name     string
}

func ExampleNewStablePQFunc() {
	pq := stable_pq.NewStablePQFunc(func(a, b Job) int {
		return a.Priority - b.Priority
	})
	for _, j := range []Job{
		{2, "index-build"},
		{1, "billing"},
		{2, "snapshot"},
		{1, "alerting"},
		{3, "backup"},
	} {
		pq.Insert(j)
	}
	for !pq.IsEmpty() {
		j, _ := pq.Pop()
		fmt.Printf("prio %d: %s\n", j.Priority, j.Name)
	}
	// Output:
	// prio 1: billing
	// prio 1: alerting
	// prio 2: index-build
	// prio 2: snapshot
	// prio 3: backup
}

func ExampleNewStablePQ() {
	// Naturally ordered keys (cmp.Ordered) need no comparator.  Equal
	// keys come out in insertion order: both 7s in the order inserted.
	pq := stable_pq.NewStablePQ[int]()
	for _, v := range []int{7, 3, 7, 1} {
		pq.Insert(v)
	}
	for v := range pq.All() {
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 1 3 7 7
}
