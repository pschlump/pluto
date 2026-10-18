/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package assignment_problem solves the n-by-n assignment problem
// (Sedgewick, Algorithms 4th ed. §6.5): choose one entry per row and per
// column of a weight matrix so that the total of the n chosen entries is
// minimum — a minimum-weight perfect matching in the complete bipartite
// graph of rows against columns.  Use NewAssignmentProblem to solve one;
// the result is an immutable query object.
//
// The implementation is the successive shortest paths algorithm: the
// matching starts empty and grows by one edge per round, each round
// running Dijkstra's algorithm over reduced costs (weight minus the row
// and column dual variables, shifted so every cost is non-negative) on
// the residual alternating-path graph, then flipping the shortest
// augmenting path and shifting the duals by the computed distances.
// algs4's AssignmentProblem builds that residual graph as an explicit
// EdgeWeightedDigraph and runs its DijkstraSP n times, O(n³ log n); here
// the Dijkstra scans the matrix directly — the dense-graph form, one
// linear scan per settled vertex, no allocation per round — for O(n³)
// total (the package benchmark measures the difference).
//
// Alongside the matching, the object exposes the optimal dual variables
// DualRow and DualCol, translated back to the caller's weights so the
// textbook optimality certificate holds verbatim on the input matrix:
// Sol is a permutation, DualRow(i)+DualCol(j) <= weight[i][j] for every
// pair with equality on matched pairs, and the duals sum to Weight().
// Those conditions prove the matching optimal (linear-programming
// duality), and the test suite checks exactly that certificate.
//
// Weights may be negative (the solver shifts by the minimum weight
// internally).  NaN and ±Inf are rejected at construction, naming the
// offending cell — encode a forbidden pair as a large finite penalty
// instead.  Results are computed from a copy of the matrix: later
// mutation of the caller's slice is not reflected, and there is no _ts
// twin (the maxflow immutability precedent).
//
// Panic contract: NewAssignmentProblem panics on a ragged matrix or a
// non-finite entry (no sane answer exists); every query on a nil result
// object reports -1 / 0, and out-of-range indices report the same as a
// nil receiver's.  There are no other panics.
package assignment_problem

import (
	"fmt"
	"math"
)

const (
	// unmatched marks an unpaired row or column.
	unmatched = -1

	// floatingPointEpsilon is the relative tolerance at which a reduced
	// cost counts as zero (algs4's value): it keeps the dual arithmetic
	// from drifting on inputs whose optimum is exact in theory.
	floatingPointEpsilon = 1.0e-14
)

// AssignmentProblem is a minimum-weight assignment of an n-by-n weight
// matrix, computed by the successive shortest paths algorithm.  Use
// NewAssignmentProblem to create one; the zero value (and a nil
// receiver) reports the empty assignment.
type AssignmentProblem struct {
	n      int         // the matrix dimension
	weight [][]float64 // the caller's matrix, copied
	minW   float64     // min(0, every weight): the shift making every internal cost non-negative
	px     []float64   // row duals of the shifted instance
	py     []float64   // column duals of the shifted instance
	xy     []int       // xy[i]: the column matched to row i, or unmatched
	yx     []int       // yx[j]: the row matched to column j, or unmatched

	// Per-round Dijkstra scratch, allocated once in prepare: augment
	// allocates nothing.
	distRow    []float64
	distCol    []float64
	settledRow []bool
	settledCol []bool
	prevRow    []int // prevRow[j]: the row whose forward edge relaxed column j
}

// NewAssignmentProblem determines a minimum-weight assignment of the
// n-by-n matrix weight: Sol(i) is the column paired with row i and
// Weight() the total of the n chosen entries.  A nil or empty matrix
// yields the empty assignment.  It panics if some row's length differs
// from the number of rows, or if any entry is NaN or ±Inf, naming the
// offending row or cell.
//
// The solve is successive shortest paths: n rounds of Dijkstra on
// reduced costs, each growing the matching by one edge (algs4's
// AssignmentProblem; see the package comment for the dense-matrix form
// used here).  Complexity is O(n³) time and O(n²) space.
func NewAssignmentProblem(weight [][]float64) *AssignmentProblem {
	a := prepare(weight)
	for range a.n {
		a.augment()
	}
	return a
}

// prepare validates weight, copies it, and lays out the empty matching
// and the solver's scratch arrays — the constructor with the solve loop
// split off, so the white-box tests can check the dual invariants
// between rounds (algs4 asserts them after every augment).
func prepare(weight [][]float64) *AssignmentProblem {
	if len(weight) == 0 {
		return &AssignmentProblem{} // nil or empty: the empty assignment
	}
	n := len(weight)
	a := &AssignmentProblem{
		n:          n,
		weight:     make([][]float64, n),
		px:         make([]float64, n),
		py:         make([]float64, n),
		xy:         make([]int, n),
		yx:         make([]int, n),
		distRow:    make([]float64, n),
		distCol:    make([]float64, n),
		settledRow: make([]bool, n),
		settledCol: make([]bool, n),
		prevRow:    make([]int, n),
	}
	for i := range n {
		if len(weight[i]) != n {
			panic(fmt.Sprintf("assignment_problem: NewAssignmentProblem: row %d has %d entries, want %d — the matrix must be square", i, len(weight[i]), n))
		}
		row := make([]float64, n)
		for j := range n {
			w := weight[i][j]
			if math.IsNaN(w) || math.IsInf(w, 0) {
				panic(fmt.Sprintf("assignment_problem: NewAssignmentProblem: weight[%d][%d] is %v — every weight must be a finite float64 (encode a forbidden pair as a large finite penalty)", i, j, w))
			}
			if w < a.minW {
				a.minW = w
			}
			row[j] = w
		}
		a.weight[i] = row
	}
	for i := range n {
		a.xy[i] = unmatched
		a.yx[i] = unmatched
	}
	return a
}

// augment grows the matching by one edge.  One dense Dijkstra runs over
// the residual alternating-path graph of rows and columns: a virtual
// source feeds every free row at distance 0, a forward edge i -> j costs
// the reduced cost of the pair, a matched pair's edge is reversed into a
// zero-cost backward edge j -> i, and a free column hangs a virtual sink
// edge at the additional cost of its column dual py[j].  The scan
// settles every vertex, the shortest augmenting path is flipped, and the
// duals shift by the computed distances.
//
// The scan runs to completion rather than stopping when the sink comes
// within reach: the dual update adds distTo of EVERY row and column, and
// an early break leaves unsettled vertices with tentative (too large)
// distances — shifting those can push a reduced cost negative and break
// dual feasibility.  At completion every distance is final, and all are
// finite: the bipartite graph is complete, so every vertex is reachable
// from the free rows every round.
func (a *AssignmentProblem) augment() {
	n := a.n
	inf := math.Inf(1)
	clear(a.settledRow)
	clear(a.settledCol)
	for i := range n {
		if a.xy[i] == unmatched {
			a.distRow[i] = 0 // the source's edges: every free row at distance 0
		} else {
			a.distRow[i] = inf // a matched row's only in-edge is its column's backward edge
		}
	}
	for j := range n {
		a.distCol[j] = inf
		a.prevRow[j] = unmatched
	}

	for range 2 * n { // one vertex settled per iteration
		bestRow, bestRowD := unmatched, inf
		for i := range n {
			if !a.settledRow[i] && a.distRow[i] < bestRowD {
				bestRow, bestRowD = i, a.distRow[i]
			}
		}
		bestCol, bestColD := unmatched, inf
		for j := range n {
			if !a.settledCol[j] && a.distCol[j] < bestColD {
				bestCol, bestColD = j, a.distCol[j]
			}
		}
		switch {
		case bestRow != unmatched && (bestCol == unmatched || bestRowD <= bestColD):
			// Settle the closest row; relax its forward edges.
			a.settledRow[bestRow] = true
			di := a.distRow[bestRow]
			for j := range n {
				if j == a.xy[bestRow] {
					continue // that edge is reversed into the backward edge
				}
				if nd := di + a.reducedCost(bestRow, j); nd < a.distCol[j] {
					a.distCol[j] = nd
					a.prevRow[j] = bestRow
				}
			}
		case bestCol != unmatched:
			// Settle the closest column; relax its backward edge.
			a.settledCol[bestCol] = true
			if i := a.yx[bestCol]; i != unmatched && a.distCol[bestCol] < a.distRow[i] {
				a.distRow[i] = a.distCol[bestCol] // backward edges cost 0
			}
		default:
			return // no finite vertex left: unreachable for finite weights
		}
	}

	// The sink sits behind its cheapest free column: distCol[j] + py[j]
	// minimal.  The sink is a dead-end vertex, so settling it would
	// change nothing — at completion the argmin over free columns IS
	// distTo(t), which the flip below walks back from.
	jT, best := unmatched, inf
	for j := range n {
		if a.yx[j] == unmatched {
			if d := a.distCol[j] + a.py[j]; d < best {
				jT, best = j, d
			}
		}
	}

	// Flip the alternating path ending at column jT: walk back through
	// the forward edge prevRow[j] and each intermediate row's current
	// match, re-pairing as we go.  The walk ends at a row that was free
	// when the round began (its old match is the sentinel).
	for j := jT; j != unmatched; {
		i := a.prevRow[j]
		old := a.xy[i]
		a.xy[i] = j
		a.yx[j] = i
		j = old
	}

	// Shift the duals by the final distances.
	for i := range n {
		a.px[i] += a.distRow[i]
	}
	for j := range n {
		a.py[j] += a.distCol[j]
	}
}

// reducedCost of pairing row i with column j on the internally shifted
// matrix: (weight[i][j] - minW) + px[i] - py[j], non-negative whenever
// the duals are feasible (the invariant every round maintains).  A value
// within a relative epsilon of zero snaps to zero — algs4's guard
// against floating-point drift, load-bearing for the near-zero
// comparisons the certificates make.
func (a *AssignmentProblem) reducedCost(i, j int) float64 {
	rc := (a.weight[i][j] - a.minW) + a.px[i] - a.py[j]
	magnitude := math.Abs(a.weight[i][j]) + math.Abs(a.px[i]) + math.Abs(a.py[j])
	if math.Abs(rc) <= floatingPointEpsilon*magnitude {
		return 0
	}
	return rc
}

// Sol returns the column matched to row i in the optimal assignment, or
// -1 if i is out of range (or the receiver is nil).
// Complexity is O(1).
func (a *AssignmentProblem) Sol(i int) int {
	if a == nil || i < 0 || i >= a.n {
		return unmatched
	}
	return a.xy[i]
}

// RowOf returns the row matched to column j — the inverse of Sol — or
// -1 if j is out of range (or the receiver is nil).  An n-by-n
// assignment matches every row and every column, so RowOf(Sol(i)) == i.
// Complexity is O(1).
func (a *AssignmentProblem) RowOf(j int) int {
	if a == nil || j < 0 || j >= a.n {
		return unmatched
	}
	return a.yx[j]
}

// Weight returns the total of the n matched entries of the caller's
// matrix (0 for the empty assignment or a nil receiver).
// Complexity is O(n).
func (a *AssignmentProblem) Weight() float64 {
	if a == nil {
		return 0
	}
	var total float64
	for i := range a.n {
		if j := a.xy[i]; j != unmatched {
			total += a.weight[i][j]
		}
	}
	return total
}

// DualRow returns the optimal dual variable of row i (0 if i is out of
// range or the receiver is nil).  Together with DualCol it carries the
// optimality certificate over the caller's matrix: DualRow(i) +
// DualCol(j) <= weight[i][j] for every pair, equality on every matched
// pair, and the dual sum equals Weight().
//
// Internally the solver keeps duals px/py of the minW-shifted instance
// it runs on (algs4's dualRow/dualCol return those, and its column dual
// enters the residual graph with the opposite sign); DualRow translates
// back to the caller's weights as minW - px[i].
// Complexity is O(1).
func (a *AssignmentProblem) DualRow(i int) float64 {
	if a == nil || i < 0 || i >= a.n {
		return 0
	}
	return a.minW - a.px[i]
}

// DualCol returns the optimal dual variable of column j (0 if j is out
// of range or the receiver is nil) — py[j] in the solver's terms; see
// DualRow for the certificate the pair carries over the caller's
// matrix.
// Complexity is O(1).
func (a *AssignmentProblem) DualCol(j int) float64 {
	if a == nil || j < 0 || j >= a.n {
		return 0
	}
	return a.py[j]
}
