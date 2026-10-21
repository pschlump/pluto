/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package gauss

import (
	"fmt"
	"math"
)

// GaussJordanElimination solves a square n×n system Ax = b by
// Gauss-Jordan elimination with partial pivoting — full reduction:
// every pivot eliminates both below and above, and its row is scaled
// so the matrix is in reduced form (the algs4 GaussJordanElimination).
// The augmented matrix carries an identity block, so an infeasible
// system yields a certificate y with yA = 0, yb ≠ 0 (Dual, the Farkas
// certificate).  The zero value and a nil receiver behave as an empty
// system: solvable, with the empty solution.
type GaussJordanElimination struct {
	n int
	a [][]float64 // n×(2n+1) augmented: A | identity | b, fully reduced
}

// NewGaussJordanElimination copies A and b into an n×(2n+1) augmented
// matrix (A | identity | b) and fully reduces it with partial
// pivoting.  It panics if A is not square with n = len(b), or if any
// entry is NaN or ±Inf — each panic message names the constructor and
// the offending position.
// Complexity is O(n³) time, O(n²) space.
func NewGaussJordanElimination(A [][]float64, b []float64) *GaussJordanElimination {
	n := len(b)
	if len(A) != n {
		panic(fmt.Sprintf("gauss: NewGaussJordanElimination called with A of %d rows and b of length %d — A must be square, one row per entry of b", len(A), n))
	}
	for i, row := range A {
		if len(row) != n {
			panic(fmt.Sprintf("gauss: NewGaussJordanElimination called with row %d of length %d but %d columns elsewhere — A must be square, one row per entry of b", i, len(row), n))
		}
	}
	requireFinite("NewGaussJordanElimination", A, b)

	a := make([][]float64, n)
	for i := range a {
		a[i] = make([]float64, 2*n+1)
		copy(a[i], A[i])
		a[i][n+i] = 1 // the identity block — only Dual reads it
		a[i][2*n] = b[i]
	}
	g := &GaussJordanElimination{n: n, a: a}

	// Gauss-Jordan elimination.
	for p := 0; p < n; p++ {
		// Find the pivot row (partial pivoting) and swap it in.
		max := p
		for i := p + 1; i < n; i++ {
			if math.Abs(a[i][p]) > math.Abs(a[max][p]) {
				max = i
			}
		}
		a[p], a[max] = a[max], a[p]

		// Singular or nearly singular: skip this pivot column.
		if math.Abs(a[p][p]) <= epsilon {
			continue
		}

		g.pivot(p, p)
	}
	return g
}

// pivot pivots on entry (p, q): every other row is reduced against row
// p, column q is zeroed, and row p is scaled to a unit pivot (the
// algs4 pivot, which runs over all columns "for consistency with the
// simplex pivot").  Complexity is O(n²) per pivot.
func (g *GaussJordanElimination) pivot(p, q int) {
	a := g.a
	n := g.n

	// Everything but row p and column q.
	for i := 0; i < n; i++ {
		alpha := a[i][q] / a[p][q]
		for j := 0; j <= n+n; j++ {
			if i != p && j != q {
				a[i][j] -= alpha * a[p][j]
			}
		}
	}

	// Zero out column q.
	for i := 0; i < n; i++ {
		if i != p {
			a[i][q] = 0
		}
	}

	// Scale row p.
	for j := 0; j <= n+n; j++ {
		if j != q {
			a[p][j] /= a[p][q]
		}
	}
	a[p][q] = 1
}

// Primal returns a solution x of Ax = b: the unique one for a
// nonsingular system, or a particular one with the free variables set
// to zero for a singular system with solutions.  The second return is
// false when no solution exists.
// Complexity is O(n).
func (g *GaussJordanElimination) Primal() ([]float64, bool) {
	if g == nil {
		return []float64{}, true
	}
	n := g.n
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		if math.Abs(g.a[i][i]) > epsilon {
			x[i] = g.a[i][n+n] / g.a[i][i]
		} else if math.Abs(g.a[i][n+n]) > epsilon {
			return nil, false // 0 = nonzero — the system is inconsistent
		}
	}
	return x, true
}

// Dual returns a certificate of infeasibility for a system with no
// solution: a vector y with yA = 0 and yb ≠ 0 (read off the identity
// block of the first inconsistent reduced row — the algs4 dual).  The
// second return is false when the system is feasible, so no
// certificate exists.
// Complexity is O(n).
func (g *GaussJordanElimination) Dual() ([]float64, bool) {
	if g == nil {
		return nil, false
	}
	n := g.n
	for i := 0; i < n; i++ {
		if math.Abs(g.a[i][i]) <= epsilon && math.Abs(g.a[i][n+n]) > epsilon {
			y := make([]float64, n)
			for j := 0; j < n; j++ {
				y[j] = g.a[i][n+j]
			}
			return y, true
		}
	}
	return nil, false
}

// IsFeasible reports whether the system has at least one solution.
// Complexity is O(n).
func (g *GaussJordanElimination) IsFeasible() bool {
	_, ok := g.Primal()
	return ok
}

// IsNonsingular reports whether the reduced matrix has a usable pivot
// in every column — an empty or nil system is vacuously nonsingular.
// (A pluto addition over the algs4 surface, for the backlog's
// singular-detection ask.)
// Complexity is O(n).
func (g *GaussJordanElimination) IsNonsingular() bool {
	if g == nil {
		return true
	}
	for i := 0; i < g.n; i++ {
		if math.Abs(g.a[i][i]) <= epsilon {
			return false
		}
	}
	return true
}
