/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package guava_compare

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
)

type GraphEdge struct {
	Source int `json:"source"`
	Target int `json:"target"`
}

type GraphCorpus struct {
	Version           int         `json:"version"`
	Seed              int64       `json:"seed"`
	Nodes             int         `json:"nodes"`
	InputDistribution string      `json:"inputDistribution"`
	Edges             []GraphEdge `json:"edges"`
	Lookups           []GraphEdge `json:"lookups"`
}

func GenerateGraphCorpus(seed int64, nodes, edgeCount, lookupCount int) GraphCorpus {
	random := rand.New(rand.NewSource(seed))
	edges := make([]GraphEdge, 0, edgeCount)
	seen := make(map[[2]int]struct{}, edgeCount)
	for len(edges) < edgeCount {
		edge := GraphEdge{Source: random.Intn(nodes), Target: random.Intn(nodes)}
		if edge.Source == edge.Target {
			continue
		}
		key := [2]int{edge.Source, edge.Target}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		edges = append(edges, edge)
	}
	lookups := make([]GraphEdge, lookupCount)
	for i := range lookups {
		if i%2 == 0 {
			lookups[i] = edges[random.Intn(len(edges))]
		} else {
			lookups[i] = GraphEdge{Source: random.Intn(nodes), Target: random.Intn(nodes)}
		}
	}
	return GraphCorpus{Version: TraceVersion, Seed: seed, Nodes: nodes, InputDistribution: GraphBenchmarkInputDistribution, Edges: edges, Lookups: lookups}
}

func ReadGraphCorpus(path string) (GraphCorpus, error) {
	file, err := os.Open(path)
	if err != nil {
		return GraphCorpus{}, err
	}
	defer func() { _ = file.Close() }()
	var corpus GraphCorpus
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return GraphCorpus{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return GraphCorpus{}, err
	}
	if corpus.Nodes <= 0 || len(corpus.Edges) == 0 || len(corpus.Lookups) == 0 {
		return GraphCorpus{}, fmt.Errorf("empty graph corpus")
	}
	return corpus, nil
}

func WriteGraphCorpus(path string, corpus GraphCorpus) error { return writeJSON(path, corpus, true) }
