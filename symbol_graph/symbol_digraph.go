/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// SymbolDigraph — the directed form, over digraph.Digraph.  See
// symbol_graph.go for the package overview and the line/field
// semantics; only the edge direction differs: field 0 of each line
// points AT every other field of the line.

package symbol_graph

import (
	"io"

	"github.com/pschlump/pluto/digraph"
)

// SymbolDigraph is a directed graph whose vertices are string names,
// over digraph.Digraph.  Build one with NewSymbolDigraph or
// NewSymbolDigraphReader; the name→index mapping is frozen at
// construction.
type SymbolDigraph struct {
	st   map[string]int
	keys []string
	g    *digraph.Digraph
}

// NewSymbolDigraph builds the symbol digraph from lines: every field of
// every line becomes a vertex (indices assigned at first appearance),
// and field 0 of each line gets an edge TO every other field of that
// line.  delimiter splits each line; an empty delimiter would split
// every line into single characters and is rejected.
// Complexity is O(L) over the total input length, plus O(1) per edge.
func NewSymbolDigraph(lines []string, delimiter string) *SymbolDigraph {
	requireDelimiter("NewSymbolDigraph", delimiter)
	st, keys := firstPass(lines, delimiter)
	g := digraph.NewDigraph(len(keys))
	secondPass(lines, delimiter, st, g.AddEdge)
	return &SymbolDigraph{st: st, keys: keys, g: g}
}

// NewSymbolDigraphReader is NewSymbolDigraph over the lines of r.  A
// read error is returned unchanged (wrapped); a line longer than 1 MiB
// is an error, not a silent truncation.  A nil reader is an error.
// Complexity is O(L) over the input length plus one line per entry.
func NewSymbolDigraphReader(r io.Reader, delimiter string) (*SymbolDigraph, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	return NewSymbolDigraph(lines, delimiter), nil
}

// Contains reports whether name is a vertex.
// Complexity is O(1) expected.
func (sd *SymbolDigraph) Contains(name string) bool {
	if sd == nil {
		return false
	}
	_, found := sd.st[name]
	return found
}

// Index returns the vertex index of name.  found is false when name is
// not a vertex.
// Complexity is O(1) expected.
func (sd *SymbolDigraph) Index(name string) (index int, found bool) {
	if sd == nil {
		return 0, false
	}
	i, ok := sd.st[name]
	return i, ok
}

// Name returns the vertex name at index v.  found is false when v is
// outside [0, V).
// Complexity is O(1).
func (sd *SymbolDigraph) Name(v int) (name string, found bool) {
	if sd == nil || v < 0 || v >= len(sd.keys) {
		return "", false
	}
	return sd.keys[v], true
}

// Digraph returns the built digraph on the indices 0..V-1.  It is the
// live internal structure — query objects (Topological, the SCCs, …)
// may be built on it directly, and added edges keep every name/index
// valid.
// Complexity is O(1).
func (sd *SymbolDigraph) Digraph() *digraph.Digraph {
	if sd == nil {
		return nil
	}
	return sd.g
}
