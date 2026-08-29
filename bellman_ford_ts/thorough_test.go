package bellman_ford_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"sync"
	"testing"

	"github.com/pschlump/pluto/dijkstra_ts"
)

// -------------------------------------------------------------------------------------------------------
// Independent reference: repeated-relaxation over plain int slices
// (NO pluto imports — the package under test is checked against a model
// that shares no code with it)
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

// -------------------------------------------------------------------------------------------------------
// Randomized verification against the reference (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestBellmanFordRandomized mirrors the plain package's randomized
// cross-check: random small digraphs (integer weights including
// negatives), every distance compared against the naive O(V·E)
// reference, exactly; negative-cycle detection must agree too.  Fixed
// seed: deterministic run.
func TestBellmanFordRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for trial := range 200 {
		n := 1 + rng.Intn(12)
		m := rng.Intn(4 * n)
		g := dijkstra_ts.NewEdgeWeightedDigraph(n)
		var edges []refEdge
		for range m {
			from, to, wt := rng.Intn(n), rng.Intn(n), rng.Intn(26)-5
			g.AddEdge(dijkstra_ts.DirectedEdge{From: from, To: to, Weight: float64(wt)})
			edges = append(edges, refEdge{from, to, wt})
		}

		s := rng.Intn(n)
		want, refNegCycle := referenceDist(n, edges, s)
		sp := NewBellmanFordSP(g, s)

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
					t.Fatalf("trial %d: NegativeCycle() is not a cycle at edge %d", trial, i)
				}
				sum += e.Weight
			}
			if sum >= 0 {
				t.Fatalf("trial %d: NegativeCycle() weights sum to %v, expected negative", trial, sum)
			}
			continue
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
}

// -------------------------------------------------------------------------------------------------------
// Concurrency smoke test (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestBellmanFordConcurrent hammers a digraph with concurrent AddEdge
// writers, V/E/Adj readers, Bellman-Ford constructors (which snapshot
// under the read lock), and readers of the immutable query objects — all
// at once.  Correctness is verified single-threaded afterwards; under
// -race this fails on any data race.
func TestBellmanFordConcurrent(t *testing.T) {
	const n = 16

	// A weight-1 chain 0->1->...->15, so the initial query object has a
	// known distance to check afterwards.
	g := dijkstra_ts.NewEdgeWeightedDigraph(n)
	for v := range n - 1 {
		g.AddEdge(dijkstra_ts.DirectedEdge{From: v, To: v + 1, Weight: 1})
	}
	sp := NewBellmanFordSP(g, 0)

	var wg sync.WaitGroup

	// Readers: the immutable query object plus the graph read operations.
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				for v := range n {
					sp.DistTo(v)
					sp.HasPathTo(v)
					sp.PathTo(v)
					sp.HasNegativeCycle()
					sp.NegativeCycle()
					g.V()
					g.E()
					g.Adj(v)
				}
			}
		}()
	}

	// Writers: non-negative weights only, so concurrent constructors
	// never find a negative cycle (which would panic the readers'
	// DistTo).
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 500 {
				g.AddEdge(dijkstra_ts.DirectedEdge{From: i % n, To: (i + w + 1) % n, Weight: 1})
			}
		}(w)
	}

	// Constructors running concurrently with the writers — each
	// snapshots the adjacency under the read lock, then searches
	// lock-free.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			NewBellmanFordSP(g, 0)
		}
	}()

	wg.Wait()

	// The pre-concurrency query object is an immutable snapshot: the
	// chain distance must be exactly n-1 regardless of the writes.
	if sp.DistTo(n-1) != float64(n-1) {
		t.Errorf("DistTo(%d) = %v, expected %d (immutable snapshot)", n-1, sp.DistTo(n-1), n-1)
	}
	if sp.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true on a non-negative-weight digraph.")
	}
}

// TestAcyclicConcurrent is the DAG twin of the hammer: writers add only
// edges from lower to higher numbered vertices (every snapshot stays a
// DAG, so the constructors never panic), readers run AcyclicSP/AcyclicLP
// on snapshots concurrently.
func TestAcyclicConcurrent(t *testing.T) {
	const n = 16

	g := dijkstra_ts.NewEdgeWeightedDigraph(n)
	for v := range n - 1 {
		g.AddEdge(dijkstra_ts.DirectedEdge{From: v, To: v + 1, Weight: 1})
	}
	sp := NewAcyclicSP(g, 0)
	lp := NewAcyclicLP(g, 0)

	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				for v := range n {
					sp.DistTo(v)
					sp.PathTo(v)
					lp.DistTo(v)
					lp.HasPathTo(v)
				}
			}
		}()
	}

	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 500 {
				from, to := i%n, (i+w+1)%n
				if from > to {
					from, to = to, from // keep it a DAG: edges go up
				}
				if from != to {
					g.AddEdge(dijkstra_ts.DirectedEdge{From: from, To: to, Weight: 1})
				}
			}
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			NewAcyclicSP(g, 0)
			NewAcyclicLP(g, 0)
		}
	}()

	wg.Wait()

	if sp.DistTo(n-1) != float64(n-1) || lp.DistTo(n-1) != float64(n-1) {
		t.Errorf("DistTo(%d) = %v/%v, expected %d/%d (immutable snapshot)",
			n-1, sp.DistTo(n-1), lp.DistTo(n-1), n-1, n-1)
	}
}

// TestBellmanFordLockUnlock exercises the no-op Lock/Unlock on the
// query objects.
func TestBellmanFordLockUnlock(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedDigraph(2)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 1})
	sp := NewBellmanFordSP(g, 0)
	sp.Lock()   // no-op, must not panic
	sp.Unlock() // no-op, must not panic
	NewAcyclicSP(g, 0).Lock()
	NewAcyclicLP(g, 0).Unlock()
}
