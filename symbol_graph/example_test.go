/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package symbol_graph_test

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/graph"
	"github.com/pschlump/pluto/symbol_graph"
)

func Example() {
	// A tiny routes.txt: field 0 of each line connects to every other
	// field of that line.
	sg := symbol_graph.NewSymbolGraph([]string{
		"JFK MCO",
		"ORD DEN",
		"DEN HOU",
		"JFK ATL",
		"LAX DEN",
	}, " ")

	fmt.Println(sg.Contains("JFK"), sg.Contains("SEA"))
	jfk, _ := sg.Index("JFK")
	name, _ := sg.Name(jfk)
	fmt.Println(name)

	for w := range sg.Graph().Adj(jfk) {
		name, _ := sg.Name(w)
		fmt.Print(name, " ")
	}
	fmt.Println()
	// Output:
	// true false
	// JFK
	// MCO ATL
}

// ExampleDegreesOfSeparation is the algs4 client: a shortest path
// between two names, translated back from indices.
func ExampleSymbolGraph_degreesOfSeparation() {
	sg := symbol_graph.NewSymbolGraph([]string{
		"JFK MCO",
		"ORD DEN",
		"DEN HOU",
		"JFK ATL",
		"ATL DEN",
		"LAX DEN",
	}, " ")
	src, _ := sg.Index("JFK")
	dst, _ := sg.Index("LAX")

	bfs := graph.NewBFSPaths(sg.Graph(), src)
	path, ok := bfs.PathTo(dst)
	if !ok {
		fmt.Println("no path")
		return
	}
	for _, v := range path {
		name, _ := sg.Name(v)
		fmt.Print(name, " ")
	}
	fmt.Println()
	// Output: JFK ATL DEN LAX
}

func ExampleNewSymbolDigraph() {
	// A tiny jobs.txt: the prerequisites point at the course.
	sd := symbol_graph.NewSymbolDigraph([]string{
		"algorithms calculus linear-algebra",
		"calculus",
		"linear-algebra",
	}, " ")

	alg, _ := sd.Index("algorithms")
	out, _ := sd.Digraph().OutDegree(alg)
	fmt.Println(out)
	for _, line := range []string{"calculus", "physics"} {
		if _, found := sd.Index(line); found {
			fmt.Print(line, ":yes ")
		} else {
			fmt.Print(line, ":no ")
		}
	}
	fmt.Println()
	if _, err := symbol_graph.NewSymbolDigraphReader(strings.NewReader("a b\nb c\n"), " "); err != nil {
		fmt.Println("reader failed:", err)
	}
	// Output:
	// 2
	// calculus:yes physics:no
}
