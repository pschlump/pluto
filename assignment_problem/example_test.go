/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package assignment_problem_test

import (
	"fmt"

	"github.com/pschlump/pluto/assignment_problem"
)

func ExampleNewAssignmentProblem() {
	// Four tasks, four workers; entry [i][j] is the cost of task i done
	// by worker j.  Pair each task with its own worker, total cost least.
	cost := [][]float64{
		{9, 2, 7, 8},
		{6, 4, 3, 7},
		{5, 8, 1, 8},
		{7, 6, 9, 4},
	}

	ap := assignment_problem.NewAssignmentProblem(cost)
	fmt.Println("total cost:", ap.Weight())
	for i := range cost {
		fmt.Printf("task %d -> worker %d\n", i, ap.Sol(i))
	}

	// The duals certify the optimum: row i plus column j never exceeds
	// cost[i][j], with equality on the chosen pairs, and they sum to the
	// total — so no cheaper assignment can exist.
	var dualSum float64
	for i := range cost {
		dualSum += ap.DualRow(i) + ap.DualCol(i)
	}
	fmt.Println("dual sum:", dualSum)

	// Output:
	// total cost: 13
	// task 0 -> worker 1
	// task 1 -> worker 0
	// task 2 -> worker 2
	// task 3 -> worker 3
	// dual sum: 13
}
