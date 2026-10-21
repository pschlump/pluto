/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package gauss solves systems of linear equations Ax = b by Gaussian
// elimination (forward elimination + back substitution) and by
// Gauss-Jordan elimination (full reduction), both with partial
// pivoting (Sedgewick & Wayne, Algorithms 4th ed., §9.5,
// GaussianElimination.java and GaussJordanElimination.java).  The
// matrix story is minimal: [][]float64 coefficients, []float64
// right-hand side.
//
// Both are constructor-built query objects over an internal copy of
// the augmented matrix — the caller's A and b are never modified — and
// the finished object is immutable, so concurrent reads are safe (the
// maxflow/global_mincut precedent — no _ts twin).
//
// GaussianElimination accepts rectangular m×n systems;
// GaussJordanElimination requires a square n×n system and carries an
// identity block so that an infeasible system yields a certificate y
// with yA = 0, yb ≠ 0 (Dual).
//
// A pivot of magnitude at or below 1e-8 (the algs4 epsilon) is treated
// as zero: it is skipped, its variable becomes a free variable set to
// zero, and a zeroed row with a non-zero right-hand side makes the
// system infeasible.
package gauss

import (
	"fmt"
	"math"
)

// epsilon is the algs4 singular threshold: pivots at or below it in
// absolute value are treated as zero (both algs4 classes use 1e-8).
const epsilon = 1e-8

// GaussianElimination solves a (possibly rectangular) m×n system Ax = b
// by forward elimination with partial pivoting, then back substitution
// (the algs4 GaussianElimination).  The zero value and a nil receiver
// behave as an empty system: solvable, with the empty solution.
type GaussianElimination struct {
	m, n int
	a    [][]float64 // m×(n+1) augmented matrix, in row echelon form
}

// NewGaussianElimination copies A and b into an augmented matrix and
// reduces it to row echelon form with partial pivoting (the row with
// the largest absolute value in the pivot column is swapped into
// place).  It panics if len(b) != len(A), if the rows of A have
// differing lengths, or if any entry is NaN or ±Inf — each panic
// message names the constructor and the offending position.
// Complexity is O(m·n·min(m, n)) time (O(n³) for square systems),
// O(m·n) space.
func NewGaussianElimination(A [][]float64, b []float64) *GaussianElimination {
	m := len(A)
	n := 0
	if m > 0 {
		n = len(A[0])
	}
	if len(b) != m {
		panic(fmt.Sprintf("gauss: NewGaussianElimination called with A of %d rows and b of length %d — b must have one entry per row of A", m, len(b)))
	}
	for i, row := range A {
		if len(row) != n {
			panic(fmt.Sprintf("gauss: NewGaussianElimination called with row %d of length %d but row %d of length %d — all rows of A must have the same length", i, len(row), 0, n))
		}
	}
	requireFinite("NewGaussianElimination", A, b)

	a := make([][]float64, m)
	for i := range a {
		a[i] = make([]float64, n+1)
		copy(a[i], A[i])
		a[i][n] = b[i]
	}
	g := &GaussianElimination{m: m, n: n, a: a}

	// Forward elimination: row reduce a to row echelon form.
	for p := 0; p < min(m, n); p++ {
		// Find the pivot row (partial pivoting) and swap it in.
		max := p
		for i := p + 1; i < m; i++ {
			if math.Abs(a[i][p]) > math.Abs(a[max][p]) {
				max = i
			}
		}
		a[p], a[max] = a[max], a[p]

		// Singular or nearly singular: skip this pivot column.
		if math.Abs(a[p][p]) <= epsilon {
			continue
		}

		// Eliminate the entries below the pivot.
		for i := p + 1; i < m; i++ {
			alpha := a[i][p] / a[p][p]
			for j := p; j <= n; j++ {
				a[i][j] -= alpha * a[p][j]
			}
		}
	}
	return g
}

// Primal returns a solution x of Ax = b: the unique one when the
// pivoted columns are independent, or a particular one with the free
// variables set to zero when the system has infinitely many.  The
// second return is false when no solution exists — a zeroed row with a
// non-zero right-hand side, including an overdetermined row that
// contradicts the solution of the pivoted rows.
//
// Known reference corner, kept faithfully: back substitution assigns
// every variable at its own row index, so for an underdetermined
// system (m < n) the variables at columns m..n-1 never pivot and stay
// zero — a row whose equation needs one of them nonzero to balance is
// reported inconsistent even though a solution exists ([[0, 5]] · x =
// 10 is the minimal example).  Square and overdetermined systems have
// no such gap: every variable's row is visited.
// Complexity is O(n²) for square systems, O(m·n) in general.
func (g *GaussianElimination) Primal() ([]float64, bool) {
	if g == nil {
		return []float64{}, true
	}
	x := make([]float64, g.n)

	// Back substitution, bottom to top; x[i] stays zero where the
	// pivot was skipped (a free variable).
	for i := min(g.n-1, g.m-1); i >= 0; i-- {
		sum := 0.0
		for j := i + 1; j < g.n; j++ {
			sum += g.a[i][j] * x[j]
		}
		if math.Abs(g.a[i][i]) > epsilon {
			x[i] = (g.a[i][g.n] - sum) / g.a[i][i]
		} else if math.Abs(g.a[i][g.n]-sum) > epsilon {
			return nil, false // 0 = nonzero — the system is inconsistent
		}
	}

	// Redundant rows (m > n): check they agree with the computed x.
	for i := g.n; i < g.m; i++ {
		sum := 0.0
		for j := 0; j < g.n; j++ {
			sum += g.a[i][j] * x[j]
		}
		if math.Abs(g.a[i][g.n]-sum) > epsilon {
			return nil, false
		}
	}
	return x, true
}

// IsFeasible reports whether the system has at least one solution.
// Complexity is the same as Primal.
func (g *GaussianElimination) IsFeasible() bool {
	_, ok := g.Primal()
	return ok
}

// requireFinite panics, naming name, if any entry of A or b is NaN or
// ±Inf (pivoting compares absolute values, so a NaN pivot would
// otherwise slip through every comparison and poison the elimination
// silently — the floyd_warshall/global_mincut precedent).
func requireFinite(name string, A [][]float64, b []float64) {
	for i, row := range A {
		for j, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				panic(fmt.Sprintf("gauss: %s found a non-finite entry (NaN or ±Inf) at A[%d][%d] — coefficients and right-hand side must be finite", name, i, j))
			}
		}
	}
	for i, v := range b {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			panic(fmt.Sprintf("gauss: %s found a non-finite entry (NaN or ±Inf) at b[%d] — coefficients and right-hand side must be finite", name, i))
		}
	}
}
