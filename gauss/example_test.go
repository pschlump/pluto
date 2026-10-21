package gauss_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/gauss"
)

// ExampleGaussianElimination solves a 2×2 system.
func ExampleGaussianElimination() {
	A := [][]float64{{2, 1}, {1, 3}}
	b := []float64{5, 10}
	x, ok := gauss.NewGaussianElimination(A, b).Primal()
	fmt.Println(ok, x)
	// Output: true [1 3]
}

// ExampleGaussJordanElimination_Dual shows the certificate of
// infeasibility: the inconsistent system [[1,2],[2,4]]x=[1,3] has no
// solution, and the dual y=[1,-0.5] proves it (yA = 0, yb = -0.5).
func ExampleGaussJordanElimination_Dual() {
	A := [][]float64{{1, 2}, {2, 4}}
	b := []float64{1, 3}
	gj := gauss.NewGaussJordanElimination(A, b)
	if _, ok := gj.Primal(); !ok {
		y, _ := gj.Dual()
		fmt.Println("infeasible, certificate y =", y)
	}
	// Output: infeasible, certificate y = [1 -0.5]
}
