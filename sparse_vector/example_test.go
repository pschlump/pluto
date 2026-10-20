/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package sparse_vector_test

import (
	"fmt"

	"github.com/pschlump/pluto/sparse_vector"
)

// The classic client of algs4's SparseVector.java (§3.5): two sparse
// vectors of dimension 10, printed, dotted, and summed.
func ExampleNewSparseVector() {
	a := sparse_vector.NewSparseVector[float64](10)
	a.Put(3, 0.50)
	a.Put(9, 0.75)
	a.Put(6, 0.11)
	a.Put(6, 0.00) // zero removes the entry: 6 is never stored
	b := sparse_vector.NewSparseVector[float64](10)
	b.Put(3, 0.60)
	b.Put(4, 0.90)

	fmt.Println("a =", a)
	fmt.Println("b =", b)
	fmt.Println("a dot b =", a.DotSparse(b))
	fmt.Println("a + b   =", a.Plus(b))
	// Output:
	// a = (3, 0.5) (9, 0.75)
	// b = (3, 0.6) (4, 0.9)
	// a dot b = 0.3
	// a + b   = (3, 1.1) (4, 0.9) (9, 0.75)
}

// Axpy is the row operation of sparse Gaussian elimination:
// y ← y + α·x, here eliminating the x[1] entry of y.
func ExampleSparseVector_Axpy() {
	y := sparse_vector.NewSparseVector[int](5)
	y.Put(0, 1)
	y.Put(1, 2)
	y.Put(2, 3)
	x := sparse_vector.NewSparseVector[int](5)
	x.Put(1, 1)
	x.Put(3, 5)

	y.Axpy(-2, x) // y = y - 2x; the 2 at index 1 cancels to 0 and is pruned
	fmt.Println("y =", y, ", nnz", y.NNz())
	fmt.Println("dense:", y.Dense())
	// Output:
	// y = (0, 1) (2, 3) (3, -10) , nnz 3
	// dense: [1 0 3 -10 0]
}
