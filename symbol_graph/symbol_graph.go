/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package symbol_graph implements SymbolGraph and SymbolDigraph —
// string vertex names over the integer-vertex graph and digraph
// packages (Sedgewick & Wayne, Algorithms 4th ed., §4.1
// SymbolGraph.java and §4.2 SymbolDigraph.java).  Every graph client
// wants the mapping: a name→index pass over the input lines, then a
// graph built on the indices, with Index/Name translating both ways and
// Graph/Digraph handing the built structure to the query objects
// (NewBFSPaths, NewCC, Topological, the SCCs, …).
//
// The input is algs4's: a list of lines, each split on a single
// delimiter into fields.  Field 0 of a line is connected to every other
// field of the line — "JFK MCO ORD" connects JFK–MCO and JFK–ORD, not
// MCO–ORD.  Every field of every line is a vertex; names are assigned
// indices 0, 1, 2, … in order of first appearance.  A blank line is not
// a vertex; a single-token line is a vertex with no edges; adjacent
// delimiters yield empty-string names (Java's split behaves the same
// way — the delimiter is the caller's responsibility).  Duplicate
// lines, parallel edges and self-loops are all legal and preserved
// exactly as the underlying graph/digraph packages store them.
//
// NewSymbolGraph builds from a []string; NewSymbolGraphReader reads the
// lines from any io.Reader (returning read errors, the only failure
// mode) with a 1 MiB per-line limit.  NewSymbolDigraph and
// NewSymbolDigraphReader are the directed forms.
//
// The name→index mapping is frozen at construction; the returned
// graph/digraph is the live internal structure, as in algs4, so callers
// may add edges — the indices stay valid — or run any query on it.
// Concurrent reads are safe exactly as far as the caller does not
// mutate (the underlying package's own contract); there is no _ts twin
// (the gauss/maxflow precedent — constructor-built, mapping immutable
// afterwards).
package symbol_graph

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/pschlump/pluto/graph"
)

// SymbolGraph is an undirected graph whose vertices are string names,
// over graph.Graph.  Build one with NewSymbolGraph or
// NewSymbolGraphReader; the name→index mapping is frozen at
// construction.
type SymbolGraph struct {
	st   map[string]int // name → vertex index (first-appearance order)
	keys []string       // vertex index → name (the inverse of st)
	g    *graph.Graph
}

// NewSymbolGraph builds the symbol graph from lines: every field of
// every line becomes a vertex (indices assigned at first appearance),
// and field 0 of each line is connected to every other field of that
// line.  delimiter splits each line; an empty delimiter would split
// every line into single characters and is rejected.
// Complexity is O(L) over the total input length, plus O(1) per edge.
func NewSymbolGraph(lines []string, delimiter string) *SymbolGraph {
	requireDelimiter("NewSymbolGraph", delimiter)
	st, keys := firstPass(lines, delimiter)
	g := graph.NewGraph(len(keys))
	secondPass(lines, delimiter, st, g.AddEdge)
	return &SymbolGraph{st: st, keys: keys, g: g}
}

// NewSymbolGraphReader is NewSymbolGraph over the lines of r.  A read
// error is returned unchanged (wrapped); a line longer than 1 MiB is an
// error, not a silent truncation.  A nil reader is an error.
// Complexity is O(L) over the input length plus one line per entry.
func NewSymbolGraphReader(r io.Reader, delimiter string) (*SymbolGraph, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	return NewSymbolGraph(lines, delimiter), nil
}

// Contains reports whether name is a vertex.
// Complexity is O(1) expected.
func (sg *SymbolGraph) Contains(name string) bool {
	if sg == nil {
		return false
	}
	_, found := sg.st[name]
	return found
}

// Index returns the vertex index of name.  found is false when name is
// not a vertex.
// Complexity is O(1) expected.
func (sg *SymbolGraph) Index(name string) (index int, found bool) {
	if sg == nil {
		return 0, false
	}
	i, ok := sg.st[name]
	return i, ok
}

// Name returns the vertex name at index v.  found is false when v is
// outside [0, V).
// Complexity is O(1).
func (sg *SymbolGraph) Name(v int) (name string, found bool) {
	if sg == nil || v < 0 || v >= len(sg.keys) {
		return "", false
	}
	return sg.keys[v], true
}

// Graph returns the built undirected graph on the indices 0..V-1.  It
// is the live internal structure — query objects may be built on it
// directly, and added edges keep every name/index valid.
// Complexity is O(1).
func (sg *SymbolGraph) Graph() *graph.Graph {
	if sg == nil {
		return nil
	}
	return sg.g
}

// requireDelimiter rejects the empty delimiter, which strings.Split
// would otherwise silently treat as "split into single characters".
func requireDelimiter(caller, delimiter string) {
	if delimiter == "" {
		panic(fmt.Sprintf("symbol_graph: %s called with an empty delimiter (pass the field separator, for example \" \")", caller))
	}
}

// fields splits one input line on delimiter, mapping a blank line to no
// fields — a blank line names no vertex.
func fields(line, delimiter string) []string {
	parts := strings.Split(line, delimiter)
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}

// firstPass is the algs4 symbol-table pass: assign an index to every
// field of every line at first appearance.
func firstPass(lines []string, delimiter string) (st map[string]int, keys []string) {
	st = make(map[string]int)
	for _, line := range lines {
		for _, f := range fields(line, delimiter) {
			if _, ok := st[f]; !ok {
				st[f] = len(keys)
				keys = append(keys, f)
			}
		}
	}
	return st, keys
}

// secondPass is the algs4 edge pass: connect field 0 of every line to
// every other field of that line.  addEdge is the underlying graph's
// AddEdge (its out-of-range false is unreachable — indices come from st).
func secondPass(lines []string, delimiter string, st map[string]int, addEdge func(v, w int) bool) {
	for _, line := range lines {
		fs := fields(line, delimiter)
		if len(fs) < 2 {
			continue
		}
		v := st[fs[0]]
		for _, f := range fs[1:] {
			addEdge(v, st[f])
		}
	}
}

// readLines collects the lines of r for the two passes.  The default
// 64 KiB scanner limit is raised to 1 MiB; beyond that is an error.
func readLines(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("symbol_graph: called with a nil reader")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("symbol_graph: reading input: %w", err)
	}
	return lines, nil
}
