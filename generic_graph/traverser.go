/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import (
	"iter"
	"slices"
)

// Traverser lazily walks a graph or a tree-shaped successor function.
type Traverser[N any] struct {
	successors func(N) iter.Seq[N]
	config     Config[N]
	graph      bool
}

// ForGraph returns a deduplicating traverser over graph successors.
func ForGraph[N any](graph *Graph[N]) *Traverser[N] {
	if graph == nil {
		return &Traverser[N]{}
	}
	return &Traverser[N]{successors: graph.Successors, config: graph.config, graph: true}
}

// ForValueGraph returns a deduplicating traverser over graph successors.
func ForValueGraph[N any, V any](graph *ValueGraph[N, V]) *Traverser[N] {
	if graph == nil {
		return ForGraph[N](nil)
	}
	return ForGraph(graph.graph)
}

// ForNetwork returns a deduplicating traverser over network successors.
func ForNetwork[N any, E any](network *Network[N, E]) *Traverser[N] {
	if network == nil {
		return ForGraph[N](nil)
	}
	return &Traverser[N]{successors: network.Successors, config: network.config.Config, graph: true}
}

// ForFunc returns a tree traverser. It deliberately keeps no visited set; every
// node must have a unique path from the starts or cycles will not terminate.
func ForFunc[N any](successors func(N) iter.Seq[N]) *Traverser[N] {
	if successors == nil {
		panic("generic_graph: ForFunc requires a successor function")
	}
	return &Traverser[N]{successors: successors}
}

func (t *Traverser[N]) visitedSet() *indexedSet[N] {
	return newIndexedSet(t.config.Equal, t.config.Hash, 0, t.config.native)
}

// BreadthFirst yields each reachable node once for graphs, in breadth-first
// order. Iteration is lazy and propagates early termination.
func (t *Traverser[N]) BreadthFirst(starts ...N) iter.Seq[N] {
	return func(yield func(N) bool) {
		if t == nil || t.successors == nil {
			return
		}
		queue := append([]N(nil), starts...)
		var visited *indexedSet[N]
		if t.graph {
			visited = t.visitedSet()
			for _, start := range starts {
				visited.add(start)
			}
		}
		for head := 0; head < len(queue); head++ {
			node := queue[head]
			var zero N
			queue[head] = zero
			if !yield(node) {
				return
			}
			for next := range t.successors(node) {
				if t.graph {
					if _, added := visited.add(next); !added {
						continue
					}
				}
				queue = append(queue, next)
			}
		}
	}
}

// DepthFirstPreOrder yields nodes when first visited.
func (t *Traverser[N]) DepthFirstPreOrder(starts ...N) iter.Seq[N] {
	return func(yield func(N) bool) {
		if t == nil || t.successors == nil {
			return
		}
		var visited *indexedSet[N]
		if t.graph {
			visited = t.visitedSet()
		}
		var visit func(N) bool
		visit = func(node N) bool {
			if t.graph {
				if _, added := visited.add(node); !added {
					return true
				}
			}
			if !yield(node) {
				return false
			}
			for next := range t.successors(node) {
				if !visit(next) {
					return false
				}
			}
			return true
		}
		for _, start := range starts {
			if !visit(start) {
				return
			}
		}
	}
}

// DepthFirstPostOrder yields nodes after all reachable descendants.
func (t *Traverser[N]) DepthFirstPostOrder(starts ...N) iter.Seq[N] {
	return func(yield func(N) bool) {
		if t == nil || t.successors == nil {
			return
		}
		var visited *indexedSet[N]
		if t.graph {
			visited = t.visitedSet()
		}
		var visit func(N) bool
		visit = func(node N) bool {
			if t.graph {
				if _, added := visited.add(node); !added {
					return true
				}
			}
			for next := range t.successors(node) {
				if !visit(next) {
					return false
				}
			}
			return yield(node)
		}
		for _, start := range starts {
			if !visit(start) {
				return
			}
		}
	}
}

// ReachableNodes returns a breadth-first snapshot including source itself.
func ReachableNodes[N any](graph *Graph[N], source N) ([]N, error) {
	if !graph.ContainsNode(source) {
		return nil, ErrUnknownNode
	}
	return slices.Collect(ForGraph(graph).BreadthFirst(source)), nil
}
