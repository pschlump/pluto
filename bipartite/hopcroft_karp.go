/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bipartite

import (
	"math"

	"github.com/pschlump/pluto/graph"
)

// unreachable is the infinite distance sentinel of the BFS layering.
const unreachable = math.MaxInt32

// NewHopcroftKarp computes a maximum matching of g with the
// Hopcroft-Karp algorithm: each phase runs a BFS from all free white
// vertices that layers the alternating graph by distance (dist is
// assigned to WHITE vertices), finds the length of the shortest
// augmenting path, then a DFS from every free white vertex augments
// along a maximal set of vertex-disjoint shortest paths.  Shortest
// path length grows every phase, so O(√V) phases suffice.  It panics
// if g is not bipartite; a nil or empty g yields the empty matching.
// Complexity is O(E·√V) time, O(V+E) space.
func NewHopcroftKarp(g *graph.Graph) *HopcroftKarp {
	m := newMatching(g, "NewHopcroftKarp")
	if len(m.mate) == 0 {
		return &HopcroftKarp{m: m}
	}
	adj := adjacency(g)
	n := len(m.mate)

	dist := make([]int, n)  // BFS layer of each WHITE vertex
	used := make([]bool, n) // per-phase: this black vertex was explored
	queue := make([]int, 0, n)

	for {
		// --- BFS phase: layer the white vertices from every free one,
		// and find the shortest length at which a free black is
		// adjacent (the shortest augmenting path length).
		for v := range dist {
			dist[v] = unreachable
		}
		queue = queue[:0]
		for v := range n {
			if m.white[v] && m.mate[v] == unmatched {
				dist[v] = 0
				queue = append(queue, v)
			}
		}
		distFree := unreachable
		for i := 0; i < len(queue); i++ {
			v := queue[i]
			for _, w := range adj[v] { // w is black
				u := m.mate[w]
				if u == unmatched {
					if d := dist[v] + 1; d < distFree {
						distFree = d
					}
				} else if dist[u] == unreachable {
					dist[u] = dist[v] + 1
					queue = append(queue, u)
				}
			}
		}
		if distFree == unreachable {
			break // no augmenting path of any length: the matching is maximum
		}

		// --- DFS phase: augment along vertex-disjoint shortest paths.
		// used marks a black vertex as consumed by a SUCCESSFUL
		// augmentation this phase, which is what keeps the paths
		// disjoint.  Failed explorations must NOT consume the black:
		// they prune the white instead (dist set to the sentinel in
		// augmentLayered) — marking on failure can destroy the last
		// shortest path and wedge the phase loop at zero augmentations.
		clear(used)
		for v := range n {
			if m.white[v] && m.mate[v] == unmatched {
				if m.augmentLayered(v, distFree, adj, dist, used) {
					m.cardinality++
				}
			}
		}
	}

	m.computeCover(adj)
	return &HopcroftKarp{m: m}
}

// augmentLayered is the phase DFS from the free white vertex v: it
// walks only layered edges — to the matching partner's white at the
// next layer, strictly below the boundary, or to a free black exactly
// at the boundary — so every path found is a shortest augmenting path.
// A black vertex is marked used only when an augmentation consumes it;
// on failure the WHITE vertex's layer is destroyed (dist set to the
// sentinel), pruning every later attempt through it this phase without
// blocking the failed black for other paths.  This is also what bounds
// the phase at O(E): a failed white is never re-entered, and a
// succeeded white is unreachable — its new black is used and its old
// black was just re-paired to its parent.
func (m *matching) augmentLayered(v int, distFree int, adj [][]int, dist []int, used []bool) bool {
	for _, w := range adj[v] { // w is black
		if used[w] {
			continue // consumed by an earlier augmentation this phase
		}
		if m.mate[w] == unmatched {
			if dist[v]+1 == distFree {
				m.mate[v] = w
				m.mate[w] = v
				used[w] = true
				return true
			}
			continue
		}
		u := m.mate[w]
		if dist[u] == dist[v]+1 && dist[v]+1 < distFree && m.augmentLayered(u, distFree, adj, dist, used) {
			m.mate[v] = w
			m.mate[w] = v
			used[w] = true
			return true
		}
	}
	dist[v] = unreachable
	return false
}

// Cardinality returns the number of edges in the matching (which is
// also the size of the minimum vertex cover).
// Complexity is O(1).
func (hk *HopcroftKarp) Cardinality() int {
	if hk == nil {
		return 0
	}
	return hk.m.cardinality
}

// Mate returns the vertex matched to v, or -1 if v is unmatched (also
// for an out-of-range v or a nil receiver).  Hopcroft-Karp and the
// single-path algorithm may return different partners — both are
// maximum matchings; the cardinality and the certificate agree.
// Complexity is O(1).
func (hk *HopcroftKarp) Mate(v int) int {
	if hk == nil || v < 0 || v >= len(hk.m.mate) {
		return unmatched
	}
	return hk.m.mate[v]
}

// InMatching reports whether v is an endpoint of a matched edge.
// Complexity is O(1).
func (hk *HopcroftKarp) InMatching(v int) bool {
	return hk.Mate(v) != unmatched
}

// Edges returns the matched edges as [white, black] vertex pairs —
// the first element of each pair is on side A of the bipartition
// (TwoColor's color false; which side that is, is an implementation
// detail).  The slice is fresh; the caller may keep or mutate it.  A
// nil receiver reports nil.
// Complexity is O(V).
func (hk *HopcroftKarp) Edges() [][2]int {
	if hk == nil {
		return nil
	}
	return hk.m.edges()
}

// InMinVertexCover reports whether v is in the minimum vertex cover
// derived from the matching (König's theorem).
// Complexity is O(1).
func (hk *HopcroftKarp) InMinVertexCover(v int) bool {
	if hk == nil || v < 0 || v >= len(hk.m.inCover) {
		return false
	}
	return hk.m.inCover[v]
}

// MinVertexCover returns the minimum vertex cover as a slice of
// vertices (ascending order).  The slice is fresh.  A nil receiver
// reports nil.
// Complexity is O(V).
func (hk *HopcroftKarp) MinVertexCover() []int {
	if hk == nil {
		return nil
	}
	return hk.m.minVertexCover()
}
