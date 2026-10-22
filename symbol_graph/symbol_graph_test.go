/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package symbol_graph

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pschlump/pluto/graph"
)

// routes is a small routes.txt in the algs4 shape: field 0 connects to
// every other field of the line, and only that.
var routes = []string{
	"JFK MCO",
	"ORD DEN",
	"DEN HOU",
	"DFW ATL",
	"JFK ATL",
	"ORD PHX",
	"PHX LAX",
	"LAX DEN",
	"ATL DEN", // the bridge that keeps JFK and LAX in one component
}

func namesOf(sg *SymbolGraph, vertices []int) []string {
	out := make([]string, 0, len(vertices))
	for _, v := range vertices {
		name, found := sg.Name(v)
		if !found {
			out = append(out, "?")
			continue
		}
		out = append(out, name)
	}
	return out
}

func TestKnownGraph(t *testing.T) {
	sg := NewSymbolGraph(routes, " ")

	if got := sg.Graph().V(); got != 9 {
		t.Errorf("V = %d, want 9 distinct names", got)
	}
	// Indices are assigned in order of first appearance.
	for i, want := range []string{"JFK", "MCO", "ORD", "DEN", "HOU", "DFW", "ATL", "PHX", "LAX"} {
		if name, found := sg.Name(i); !found || name != want {
			t.Errorf("Name(%d) = (%q, %v), want (%q, true)", i, name, found, want)
		}
		if idx, found := sg.Index(want); !found || idx != i {
			t.Errorf("Index(%q) = (%d, %v), want (%d, true)", want, idx, found, i)
		}
	}

	// Field 0 connects to every other field — and NOT the other fields
	// to each other: MCO has exactly the JFK edge.
	if deg, ok := sg.Graph().Degree(sg.st["JFK"]); !ok || deg != 2 {
		t.Errorf("Degree(JFK) = (%d, %v), want 2 (MCO, ATL — one per line)", deg, ok)
	}
	if deg, ok := sg.Graph().Degree(sg.st["MCO"]); !ok || deg != 1 {
		t.Errorf("Degree(MCO) = (%d, %v), want 1 (JFK only — the tail fields do not interconnect)", deg, ok)
	}
	if got := sg.Graph().E(); got != len(routes) {
		t.Errorf("E = %d, want %d (one edge per line tail field)", got, len(routes))
	}
}

func TestDegreesOfSeparation(t *testing.T) {
	// The algs4 client: BFS over the symbol graph, then translate the
	// path back to names.
	sg := NewSymbolGraph(routes, " ")
	jfk, _ := sg.Index("JFK")
	bfs := graph.NewBFSPaths(sg.Graph(), jfk)
	lax, _ := sg.Index("LAX")
	path, ok := bfs.PathTo(lax)
	if !ok {
		t.Fatal("no path JFK to LAX")
	}
	got := namesOf(sg, path)
	if len(got) == 0 || got[0] != "JFK" || got[len(got)-1] != "LAX" {
		t.Fatalf("path = %v, want JFK ... LAX", got)
	}
	// The path must be a shortest path of real edges.
	if want, _ := bfs.DistTo(lax); len(path)-1 != want {
		t.Fatalf("path length %d disagrees with DistTo %d", len(path)-1, want)
	}
	for i := 1; i < len(path); i++ {
		if !sg.Graph().HasEdge(path[i-1], path[i]) {
			t.Fatalf("path %v hops across a non-edge at %d", got, i)
		}
	}
}

func TestLineShapes(t *testing.T) {
	sg := NewSymbolGraph([]string{
		"solo",   // a single-token line: a vertex, no edges
		"",       // a blank line: not a vertex
		"A B A",  // a repeated field on one line: the self loop A-A
		"A B",    // a duplicate edge: parallel edges preserved
		"tail C", // a comma of shapes: field 0 is "tail"
	}, " ")

	if got := sg.Graph().V(); got != 5 {
		t.Errorf("V = %d, want 5 (solo, A, B, tail, C — the blank line is not a vertex)", got)
	}
	if deg, _ := sg.Graph().Degree(sg.st["A"]); deg != 4 {
		t.Errorf("Degree(A) = %d, want 4 (self loop counts twice, plus two parallel B edges)", deg)
	}
	if deg, _ := sg.Graph().Degree(sg.st["solo"]); deg != 0 {
		t.Errorf("Degree(solo) = %d, want 0", deg)
	}

	// Adjacent delimiters produce empty-string names (Java's split does
	// the same) — documented, caller's responsibility.
	sg2 := NewSymbolGraph([]string{"X,,Y"}, ",")
	if got := sg2.Graph().V(); got != 3 {
		t.Errorf("V = %d, want 3 (X, \"\", Y)", got)
	}
	if _, found := sg2.Index(""); !found {
		t.Error("the empty-string name is not a vertex")
	}
}

func TestDelimiterForms(t *testing.T) {
	sg := NewSymbolGraph([]string{"a,b|c", "a|b"}, "|")
	if got := sg.Graph().V(); got != 4 {
		t.Errorf("V = %d, want 4 (a,b and c from line 1; a and b from line 2)", got)
	}
	if !sg.Contains("a,b") {
		t.Error("delimiter , is not special when splitting on |")
	}

	expectPanic(t, "symbol_graph: NewSymbolGraph called with an empty delimiter", func() {
		NewSymbolGraph([]string{"a b"}, "")
	})
	expectPanic(t, "symbol_graph: NewSymbolDigraph called with an empty delimiter", func() {
		NewSymbolDigraph([]string{"a b"}, "")
	})
}

func TestReader(t *testing.T) {
	text := strings.Join(routes, "\n")
	sg, err := NewSymbolGraphReader(strings.NewReader(text), " ")
	if err != nil {
		t.Fatalf("NewSymbolGraphReader: %v", err)
	}
	if got := sg.Graph().V(); got != 9 {
		t.Errorf("V = %d, want 9", got)
	}
	if got := sg.Graph().E(); got != len(routes) {
		t.Errorf("E = %d, want %d", got, len(routes))
	}
	if _, err := NewSymbolGraphReader(nil, " "); err == nil {
		t.Error("nil reader succeeded")
	}
	wantErr := errors.New("boom")
	_, err = NewSymbolGraphReader(errReader{wantErr}, " ")
	if !errors.Is(err, wantErr) {
		t.Errorf("read error = %v, want the wrapped %v", err, wantErr)
	}

	sd, err := NewSymbolDigraphReader(strings.NewReader(text), " ")
	if err != nil {
		t.Fatalf("NewSymbolDigraphReader: %v", err)
	}
	if got := sd.Digraph().E(); got != len(routes) {
		t.Errorf("digraph E = %d, want %d", got, len(routes))
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestDigraphDirection(t *testing.T) {
	sd := NewSymbolDigraph([]string{"A B C", "B C"}, " ")

	// Field 0 points AT the other fields: A→B and A→C, never B→A.
	a, _ := sd.Index("A")
	b, _ := sd.Index("B")
	if out, ok := sd.Digraph().OutDegree(a); !ok || out != 2 {
		t.Errorf("OutDegree(A) = (%d, %v), want 2", out, ok)
	}
	if in, ok := sd.Digraph().InDegree(a); !ok || in != 0 {
		t.Errorf("InDegree(A) = (%d, %v), want 0 (nothing points at the head field)", in, ok)
	}
	if !sd.Digraph().HasEdge(a, b) || sd.Digraph().HasEdge(b, a) {
		t.Error("edge orientation wrong: field 0 must point AT the other fields")
	}
	if _, found := sd.Index("A"); !found {
		t.Error("Index(A) missing")
	}
}

func TestNilTolerated(t *testing.T) {
	var sg *SymbolGraph
	var sd *SymbolDigraph

	if sg.Contains("a") || sd.Contains("a") {
		t.Error("Contains on nil true")
	}
	if idx, found := sg.Index("a"); found || idx != 0 {
		t.Errorf("Index on nil = (%d, %v)", idx, found)
	}
	if name, found := sg.Name(0); found || name != "" {
		t.Errorf("Name on nil = (%q, %v)", name, found)
	}
	if name, found := sd.Name(0); found || name != "" {
		t.Errorf("digraph Name on nil = (%q, %v)", name, found)
	}
	if sg.Graph() != nil || sd.Digraph() != nil {
		t.Error("Graph/Digraph on nil is not nil")
	}
}

// TestRandomizedModel builds random line sets and compares against a
// reference model: a name→index map for the mapping pass and a directly
// built graph for the edge pass.
func TestRandomizedModel(t *testing.T) {
	rng := newRand()
	for round := range 20 {
		nLines := 1 + rng.Intn(30)
		lines := make([]string, nLines)
		for i := range lines {
			n := 1 + rng.Intn(6)
			fields := make([]string, n)
			for j := range fields {
				fields[j] = string(rune('A' + rng.Intn(10)))
			}
			lines[i] = strings.Join(fields, " ")
		}

		sg := NewSymbolGraph(lines, " ")

		// Reference first pass: index at first appearance.
		ref := make(map[string]int)
		var refKeys []string
		for _, line := range lines {
			for _, f := range strings.Split(line, " ") {
				if _, ok := ref[f]; !ok {
					ref[f] = len(refKeys)
					refKeys = append(refKeys, f)
				}
			}
		}
		if got := sg.Graph().V(); got != len(refKeys) {
			t.Fatalf("round %d: V = %d, model %d", round, got, len(refKeys))
		}
		for name, idx := range ref {
			if got, found := sg.Index(name); !found || got != idx {
				t.Fatalf("round %d: Index(%q) = (%d, %v), want %d", round, name, got, found, idx)
			}
		}

		// Reference second pass: a graph built edge by edge.
		model := graph.NewGraph(len(refKeys))
		for _, line := range lines {
			fs := strings.Split(line, " ")
			for _, f := range fs[1:] {
				model.AddEdge(ref[fs[0]], ref[f])
			}
		}
		if sg.Graph().E() != model.E() {
			t.Fatalf("round %d: E = %d, model %d", round, sg.Graph().E(), model.E())
		}
		for v := range sg.Graph().V() {
			gotDeg, _ := sg.Graph().Degree(v)
			wantDeg, _ := model.Degree(v)
			if gotDeg != wantDeg {
				t.Fatalf("round %d: Degree(%d) = %d, model %d", round, v, gotDeg, wantDeg)
			}
			var gotAdj []int
			for w := range sg.Graph().Adj(v) {
				gotAdj = append(gotAdj, w)
			}
			var wantAdj []int
			for w := range model.Adj(v) {
				wantAdj = append(wantAdj, w)
			}
			slices.Sort(gotAdj)
			slices.Sort(wantAdj)
			if !slices.Equal(gotAdj, wantAdj) {
				t.Fatalf("round %d: Adj(%d) = %v, model %v", round, v, gotAdj, wantAdj)
			}
		}
	}
}

func TestMappingFrozenAfterConstruction(t *testing.T) {
	sg := NewSymbolGraph([]string{"A B", "B C"}, " ")
	before := make([]string, sg.Graph().V())
	for v := range before {
		before[v], _ = sg.Name(v)
	}
	// Adding an edge to the live graph must not disturb any mapping.
	a, _ := sg.Index("A")
	c, _ := sg.Index("C")
	sg.Graph().AddEdge(a, c)
	after := make([]string, sg.Graph().V())
	for v := range after {
		after[v], _ = sg.Name(v)
	}
	if !slices.Equal(before, after) {
		t.Errorf("mapping changed after AddEdge: %v -> %v", before, after)
	}
	if got := sg.Graph().E(); got != 3 {
		t.Errorf("E = %d, want 3 (the added edge landed)", got)
	}
}
