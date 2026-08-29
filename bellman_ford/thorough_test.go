package bellman_ford

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// -------------------------------------------------------------------------------------------------------
// Independent reference: repeated-relaxation over plain int slices
// (NO pluto imports — the package under test is checked against a model
// that shares no code with it, not even pluto/dijkstra's edge type)
// -------------------------------------------------------------------------------------------------------

// refEdge is the reference model's edge type: integer weights, stored
// and summed exactly, so distances compare with ==.
type refEdge struct {
	from, to, wt int
}

// referenceDist computes shortest distances from s by n-1 rounds of
// relaxing every edge (the slow-and-sure Bellman-Ford), then does one
// extra round: any improvement in round n means a negative cycle is
// reachable from s.  Distances are +Inf when unreachable.
func referenceDist(n int, edges []refEdge, s int) (dist []float64, hasNegCycle bool) {
	dist = make([]float64, n)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[s] = 0
	for range n - 1 {
		for _, e := range edges {
			if nd := dist[e.from] + float64(e.wt); nd < dist[e.to] {
				dist[e.to] = nd
			}
		}
	}
	for _, e := range edges {
		if nd := dist[e.from] + float64(e.wt); nd < dist[e.to] {
			hasNegCycle = true
		}
	}
	return dist, hasNegCycle
}

// referenceLongestDist computes longest distances from s in a DAG by
// negating the weights and running the shortest-path reference — valid
// because a DAG has no positive-weight cycles to unbound the result.
// Unreachable vertices report -Inf.
func referenceLongestDist(n int, edges []refEdge, s int) []float64 {
	neg := make([]refEdge, len(edges))
	for i, e := range edges {
		neg[i] = refEdge{e.from, e.to, -e.wt}
	}
	dist, hasNegCycle := referenceDist(n, neg, s)
	if hasNegCycle {
		panic("referenceLongestDist called on a cyclic digraph")
	}
	for i, d := range dist {
		if math.IsInf(d, 1) {
			dist[i] = math.Inf(-1)
		} else {
			dist[i] = -d
		}
	}
	return dist
}

// -------------------------------------------------------------------------------------------------------
// Randomized verification against the reference (fixed seeds)
// -------------------------------------------------------------------------------------------------------

// checkBellmanFordAgainstRef asserts that sp agrees with the reference
// on every vertex: negative-cycle detection (and the DistTo/PathTo panic
// contract when a cycle exists), exact distances, HasPathTo parity, and
// connected paths whose weights sum exactly to DistTo.
func checkBellmanFordAgainstRef(t *testing.T, trial int, sp *BellmanFordSP, edges []refEdge, want []float64, refNegCycle bool, s, n int) {
	t.Helper()
	if sp.HasNegativeCycle() != refNegCycle {
		t.Fatalf("trial %d: HasNegativeCycle() = %v, reference says %v", trial, sp.HasNegativeCycle(), refNegCycle)
	}
	if refNegCycle {
		cycle := sp.NegativeCycle()
		if len(cycle) == 0 {
			t.Fatalf("trial %d: NegativeCycle() returned no edges", trial)
		}
		sum := 0.0
		for i, e := range cycle {
			if next := cycle[(i+1)%len(cycle)]; e.To != next.From {
				t.Fatalf("trial %d: NegativeCycle() is not a cycle at edge %d (%v then %v)", trial, i, e, next)
			}
			// Every reported cycle edge is an edge of the graph.
			found := false
			for _, r := range edges {
				if r.from == e.From && r.to == e.To && float64(r.wt) == e.Weight {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("trial %d: NegativeCycle() edge %v is not in the graph", trial, e)
			}
			sum += e.Weight
		}
		if sum >= 0 {
			t.Fatalf("trial %d: NegativeCycle() weights sum to %v, expected negative", trial, sum)
		}
		expectPanic(t, "DistTo with a negative cycle", "negative cycle", func() { sp.DistTo(s) })
		expectPanic(t, "PathTo with a negative cycle", "negative cycle", func() { sp.PathTo(s) })
		return
	}
	if sp.NegativeCycle() != nil {
		t.Fatalf("trial %d: NegativeCycle() = %v, expected nil", trial, sp.NegativeCycle())
	}
	for v := range n {
		if sp.DistTo(v) != want[v] {
			t.Fatalf("trial %d: DistTo(%d) = %v, reference says %v", trial, v, sp.DistTo(v), want[v])
		}
		if sp.HasPathTo(v) != !math.IsInf(want[v], 1) {
			t.Fatalf("trial %d: HasPathTo(%d) = %v, reference distance is %v",
				trial, v, sp.HasPathTo(v), want[v])
		}
		if math.IsInf(want[v], 1) || v == s {
			continue
		}
		path := sp.PathTo(v)
		if len(path) == 0 || path[0].From != s || path[len(path)-1].To != v {
			t.Fatalf("trial %d: PathTo(%d) = %v — wrong endpoints", trial, v, path)
		}
		sum := 0.0
		for i, e := range path {
			if i > 0 && path[i-1].To != e.From {
				t.Fatalf("trial %d: PathTo(%d) is not connected at edge %d", trial, v, i)
			}
			sum += e.Weight
		}
		if sum != sp.DistTo(v) {
			t.Fatalf("trial %d: PathTo(%d) sums to %v, DistTo = %v", trial, v, sum, sp.DistTo(v))
		}
	}
}

// TestBellmanFordRandomized runs the queue-based Bellman-Ford on many
// random small digraphs (integer weights including negatives, self-loops
// and parallel edges included) and compares every distance — reachable,
// +Inf, and negative-cycle cases alike — against the naive O(V·E)
// reference, exactly.  Fixed seeds: deterministic runs.
func TestBellmanFordRandomized(t *testing.T) {
	for _, seed := range []int64{42, 43, 44} {
		rng := rand.New(rand.NewSource(seed))
		for trial := range 200 {
			n := 1 + rng.Intn(12)
			m := rng.Intn(4 * n)
			g := dijkstra.NewEdgeWeightedDigraph(n)
			var edges []refEdge
			for range m {
				from, to, wt := rng.Intn(n), rng.Intn(n), rng.Intn(26)-5
				g.AddEdge(dijkstra.DirectedEdge{From: from, To: to, Weight: float64(wt)})
				edges = append(edges, refEdge{from, to, wt})
			}

			s := rng.Intn(n)
			want, refNegCycle := referenceDist(n, edges, s)
			sp := NewBellmanFordSP(g, s)
			checkBellmanFordAgainstRef(t, trial, sp, edges, want, refNegCycle, s, n)
		}
	}
}

// TestAcyclicSPRandomized runs AcyclicSP and AcyclicLP on many random
// small DAGs (edges only from lower to higher numbered vertices, integer
// weights including negatives) and cross-checks against the reference
// and against BellmanFordSP itself.  Fixed seed: deterministic run.
func TestAcyclicSPRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for trial := range 200 {
		n := 1 + rng.Intn(12)
		m := rng.Intn(4 * n)
		g := dijkstra.NewEdgeWeightedDigraph(n)
		var edges []refEdge
		for range m {
			from := rng.Intn(n)
			to := rng.Intn(n)
			if from >= to {
				from, to = to, from // keep it a DAG: edges go up
			}
			if from == to {
				continue // a self-loop would be a cycle
			}
			wt := rng.Intn(26) - 5
			g.AddEdge(dijkstra.DirectedEdge{From: from, To: to, Weight: float64(wt)})
			edges = append(edges, refEdge{from, to, wt})
		}

		s := rng.Intn(n)
		want, refNegCycle := referenceDist(n, edges, s)
		if refNegCycle {
			t.Fatalf("trial %d: the reference found a negative cycle in a DAG — generator bug", trial)
		}
		wantLP := referenceLongestDist(n, edges, s)

		sp := NewAcyclicSP(g, s)
		lp := NewAcyclicLP(g, s)
		bf := NewBellmanFordSP(g, s) // a DAG has no cycle at all: Bellman-Ford must agree
		if bf.HasNegativeCycle() {
			t.Fatalf("trial %d: BellmanFordSP found a negative cycle in a DAG", trial)
		}

		for v := range n {
			if sp.DistTo(v) != want[v] {
				t.Fatalf("trial %d: AcyclicSP DistTo(%d) = %v, reference says %v", trial, v, sp.DistTo(v), want[v])
			}
			if sp.DistTo(v) != bf.DistTo(v) {
				t.Fatalf("trial %d: AcyclicSP DistTo(%d) = %v, BellmanFordSP says %v", trial, v, sp.DistTo(v), bf.DistTo(v))
			}
			if lp.DistTo(v) != wantLP[v] {
				t.Fatalf("trial %d: AcyclicLP DistTo(%d) = %v, reference says %v", trial, v, lp.DistTo(v), wantLP[v])
			}
			if sp.HasPathTo(v) != lp.HasPathTo(v) {
				t.Fatalf("trial %d: HasPathTo(%d) disagrees between AcyclicSP and AcyclicLP", trial, v)
			}
			if math.IsInf(want[v], 1) || v == s {
				continue
			}
			// Paths are connected, run s -> v, and sum to the distance.
			sum := 0.0
			at := s
			for _, e := range sp.PathTo(v) {
				if e.From != at {
					t.Fatalf("trial %d: AcyclicSP PathTo(%d) is not connected at %v", trial, v, e)
				}
				at = e.To
				sum += e.Weight
			}
			if at != v || sum != sp.DistTo(v) {
				t.Fatalf("trial %d: AcyclicSP PathTo(%d) ends at %d summing to %v, DistTo = %v", trial, v, at, sum, sp.DistTo(v))
			}
			sum = 0.0
			at = s
			for _, e := range lp.PathTo(v) {
				if e.From != at {
					t.Fatalf("trial %d: AcyclicLP PathTo(%d) is not connected at %v", trial, v, e)
				}
				at = e.To
				sum += e.Weight
			}
			if at != v || sum != lp.DistTo(v) {
				t.Fatalf("trial %d: AcyclicLP PathTo(%d) ends at %d summing to %v, DistTo = %v", trial, v, at, sum, lp.DistTo(v))
			}
		}
	}
}
