package floyd_warshall_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"strings"
	"testing"

	"github.com/pschlump/pluto/dijkstra_ts"
)

// expectPanic runs fx and fails the test unless it panics; when want is
// non-empty the panic message must contain it.
func expectPanic(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if want != "" {
			if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
				t.Errorf("Unexpected panic message from %s: %v (expected it to contain %q)", name, r, want)
			}
		}
	}()
	fx()
}

// TestBasics pins the query surface on a small negative-weight graph:
// distances, paths, and the snapshot semantics.
func TestBasics(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 3})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 2, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 1, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 1, To: 3, Weight: 2})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 0, Weight: -1})

	fw := NewFloydWarshall(g)
	if fw.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = true, want false")
	}
	if got, want := fw.Dist(0, 1), 2.0; got != want {
		t.Fatalf("Dist(0, 1) = %v, want %v (via 2)", got, want)
	}
	if got, want := fw.Dist(0, 3), 4.0; got != want {
		t.Fatalf("Dist(0, 3) = %v, want %v", got, want)
	}
	path := fw.Path(0, 3)
	if len(path) != 3 || path[0].From != 0 || path[2].To != 3 {
		t.Fatalf("Path(0, 3) = %v — wrong shape", path)
	}
	if p := fw.Path(2, 2); p == nil || len(p) != 0 {
		t.Fatalf("Path(2, 2) = %v, want an empty non-nil slice", p)
	}

	// Immutable snapshot: a shortcut added after construction must not
	// appear in the answers.
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 3, Weight: 0.5})
	if got, want := fw.Dist(0, 3), 4.0; got != want {
		t.Fatalf("Dist(0, 3) = %v, want %v (late AddEdge must not be reflected)", got, want)
	}
}

// TestNegativeCycle covers the detection surface, including a cycle no
// single-source search from vertex 0 could see.
func TestNegativeCycle(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 3, To: 4, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 4, To: 2, Weight: -3}) // cycle 2->3->4->2, weight -1

	fw := NewFloydWarshall(g)
	if !fw.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = false, want true")
	}
	cycle := fw.NegativeCycle()
	if len(cycle) == 0 {
		t.Fatalf("NegativeCycle() returned no edges")
	}
	sum := 0.0
	for i, e := range cycle {
		if next := cycle[(i+1)%len(cycle)]; e.To != next.From {
			t.Fatalf("NegativeCycle() is not a cycle at edge %d", i)
		}
		sum += e.Weight
	}
	if sum >= 0 {
		t.Fatalf("NegativeCycle() weights sum to %v, expected negative", sum)
	}
	expectPanic(t, "Dist", "negative cycle", func() { fw.Dist(0, 4) })
	expectPanic(t, "HasPath", "negative cycle", func() { fw.HasPath(0, 4) })
	expectPanic(t, "Path", "negative cycle", func() { fw.Path(0, 4) })
}

// TestNilAndZeroValue pins the tolerant semantics: a nil or zero-value
// query object answers every question as "no paths".
func TestNilAndZeroValue(t *testing.T) {
	var nilFW *FloydWarshall
	zero := &FloydWarshall{}
	for name, fw := range map[string]*FloydWarshall{"nil": nilFW, "zero": zero} {
		if got := fw.Dist(0, 1); !math.IsInf(got, 1) {
			t.Errorf("%s: Dist = %v, want +Inf", name, got)
		}
		if fw.HasPath(0, 1) {
			t.Errorf("%s: HasPath = true, want false", name)
		}
		if p := fw.Path(0, 1); p != nil {
			t.Errorf("%s: Path = %v, want nil", name, p)
		}
		if fw.HasNegativeCycle() {
			t.Errorf("%s: HasNegativeCycle = true, want false", name)
		}
		if c := fw.NegativeCycle(); c != nil {
			t.Errorf("%s: NegativeCycle = %v, want nil", name, c)
		}
		fw.Lock()   // no-op, must not panic
		fw.Unlock() // no-op, must not panic
	}
}

// TestPanics pins the constructor's panic contract.
func TestPanics(t *testing.T) {
	expectPanic(t, "nil graph", "floyd_warshall_ts: NewFloydWarshall called on a nil graph", func() {
		NewFloydWarshall(nil)
	})
	expectPanic(t, "zero-value graph", "empty graph", func() {
		NewFloydWarshall(&dijkstra_ts.EdgeWeightedDigraph{})
	})
	for name, w := range map[string]float64{
		"NaN":  math.NaN(),
		"+Inf": math.Inf(1),
		"-Inf": math.Inf(-1),
	} {
		g := dijkstra_ts.NewEdgeWeightedDigraph(2)
		g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: w})
		expectPanic(t, name+" weight", "non-finite weight", func() {
			NewFloydWarshall(g)
		})
	}
}
