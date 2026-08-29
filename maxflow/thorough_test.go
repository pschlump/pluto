package maxflow

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// The max-flow / min-cut certificate
// -------------------------------------------------------------------------------------------------------

// checkMaxflow verifies the max-flow / min-cut certificate of a result,
// independently of how it was computed:
//
//	(a) capacity constraints: 0 <= flow <= capacity on every edge;
//	(b) flow conservation: net flow is zero at every vertex except s, t;
//	(c) net outflow from s (== net inflow to t) equals Value();
//	(d) the min-cut certificate: the total capacity of the edges
//	    crossing from the InMinCut source side to the sink side equals
//	    Value() exactly — a feasible flow whose value equals a cut's
//	    capacity IS a max flow and the cut IS a min cut.
//
// All capacities in these tests are small integers stored in float64,
// so every sum is exact and comparisons use ==.
func checkMaxflow(t *testing.T, name string, g *FlowNetwork, ff *FordFulkerson) {
	t.Helper()
	n := g.V()
	s, sink := ff.s, ff.t

	edges := ff.edges // the private copy, white-box: same package
	if len(edges) != g.E() {
		t.Fatalf("%s: result has %d edges, network has %d", name, len(edges), g.E())
	}

	// (a) capacity constraints, and every edge stays inside the vertex range.
	net := make([]float64, n)
	for _, e := range edges {
		if e.From < 0 || e.From >= n || e.To < 0 || e.To >= n {
			t.Fatalf("%s: edge %d->%d out of range", name, e.From, e.To)
		}
		if e.Flow < 0 || e.Flow > e.Capacity {
			t.Errorf("%s: edge %d->%d has flow %v, capacity %v (capacity constraint violated)",
				name, e.From, e.To, e.Flow, e.Capacity)
		}
		net[e.From] -= e.Flow
		net[e.To] += e.Flow
	}

	// (b) + (c) conservation and value.
	for v := 0; v < n; v++ {
		switch v {
		case s:
			if net[v] != -ff.Value() {
				t.Errorf("%s: net outflow from s = %v, expected %v (Value)", name, -net[v], ff.Value())
			}
		case sink:
			if net[v] != ff.Value() {
				t.Errorf("%s: net inflow to t = %v, expected %v (Value)", name, net[v], ff.Value())
			}
		default:
			if net[v] != 0 {
				t.Errorf("%s: net flow at %d = %v, expected 0 (conservation)", name, v, net[v])
			}
		}
	}

	// (d) the min-cut certificate.  s must be on the source side and t
	// must not be.
	if !ff.InMinCut(s) {
		t.Errorf("%s: source %d not on the source side of the min cut", name, s)
	}
	if ff.InMinCut(sink) {
		t.Errorf("%s: sink %d on the source side of the min cut", name, sink)
	}
	cutCap := 0.0
	for _, e := range edges {
		if ff.InMinCut(e.From) && !ff.InMinCut(e.To) {
			cutCap += e.Capacity
		}
	}
	if cutCap != ff.Value() {
		t.Errorf("%s: min-cut capacity %v != maxflow value %v (certificate fails)", name, cutCap, ff.Value())
	}
}

// -------------------------------------------------------------------------------------------------------
// Independent reference: brute-force enumeration of all s-t cuts
// -------------------------------------------------------------------------------------------------------

// minCutBruteForce computes the minimum s-t cut capacity by enumerating
// ALL 2^(n-2) vertex subsets containing s and not t.  Exponential,
// obviously correct, and fully independent of the Ford-Fulkerson
// machinery under test (no pluto imports, no residual graphs).
func minCutBruteForce(n int, edges []FlowEdge, s, sink int) float64 {
	mid := make([]int, 0, n-2)
	for v := 0; v < n; v++ {
		if v != s && v != sink {
			mid = append(mid, v)
		}
	}
	best := math.Inf(1)
	for mask := 0; mask < 1<<len(mid); mask++ {
		inS := make([]bool, n)
		inS[s] = true
		for i, v := range mid {
			if mask&(1<<i) != 0 {
				inS[v] = true
			}
		}
		cap := 0.0
		for _, e := range edges {
			if inS[e.From] && !inS[e.To] {
				cap += e.Capacity
			}
		}
		if cap < best {
			best = cap
		}
	}
	return best
}

// randomNetwork builds a random network on n vertices with integer
// capacities 0..20 (zeros included) and returns it with the flat edge
// list for the oracle.  Parallel edges, self-loops and disconnected
// networks all arise naturally.
func randomNetwork(rng *rand.Rand, n, m int) (*FlowNetwork, []FlowEdge) {
	g := NewFlowNetwork(n)
	edges := make([]FlowEdge, 0, m)
	for range m {
		e := FlowEdge{From: rng.Intn(n), To: rng.Intn(n), Capacity: float64(rng.Intn(21))}
		g.AddEdge(e.From, e.To, e.Capacity)
		edges = append(edges, e)
	}
	return g, edges
}

// TestMaxflowRandomizedAgreement runs Ford-Fulkerson on hundreds of
// small random networks (fixed seeds) and cross-checks every result two
// ways: the max-flow / min-cut certificate of checkMaxflow, and the
// brute-force all-cuts oracle — min-cut capacity == Value(), exactly.
func TestMaxflowRandomizedAgreement(t *testing.T) {
	for _, seed := range []int64{42, 43, 44} {
		rng := rand.New(rand.NewSource(seed)) // fixed seed: deterministic run
		for trial := range 150 {
			n := 2 + rng.Intn(11) // 2..12 vertices, so the oracle's 2^(n-2) stays small
			s, sink := rng.Intn(n), rng.Intn(n)
			for sink == s {
				sink = rng.Intn(n)
			}
			m := rng.Intn(3*n + 1)
			g, edges := randomNetwork(rng, n, m)

			ff := NewFordFulkerson(g, s, sink)
			checkMaxflow(t, "random", g, ff)

			if want := minCutBruteForce(n, edges, s, sink); ff.Value() != want {
				t.Fatalf("seed %d trial %d: Value = %v, brute-force min cut = %v (n=%d s=%d t=%d m=%d)",
					seed, trial, ff.Value(), want, n, s, sink, m)
			}
		}
	}
}

// TestMaxflowLargerCertificate runs the certificate check on networks
// too large for the brute-force oracle (fixed seed).
func TestMaxflowLargerCertificate(t *testing.T) {
	rng := rand.New(rand.NewSource(45)) // fixed seed: deterministic run
	for range 100 {
		n := 20 + rng.Intn(41) // 20..60 vertices
		s, sink := rng.Intn(n), rng.Intn(n)
		for sink == s {
			sink = rng.Intn(n)
		}
		m := rng.Intn(4*n + 1)
		g, _ := randomNetwork(rng, n, m)
		ff := NewFordFulkerson(g, s, sink)
		checkMaxflow(t, "larger", g, ff)
	}
}

// benchmarkNetwork builds a random network with a spanning path so the
// source can usually reach the sink, for benchmarks.
func benchmarkNetwork(n, m int, seed int64) *FlowNetwork {
	rng := rand.New(rand.NewSource(seed))
	g := NewFlowNetwork(n)
	for v := 0; v+1 < n; v++ { // a spanning path 0 -> 1 -> ... -> n-1
		g.AddEdge(v, v+1, float64(rng.Intn(100)+1))
	}
	for i := 0; i < m; i++ {
		g.AddEdge(rng.Intn(n), rng.Intn(n), float64(rng.Intn(100)+1))
	}
	return g
}
