package generic_graph

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"slices"
	"testing"
)

// TestEdgeLogModel drives edgeLog across several blocks against a slice.
func TestEdgeLogModel(t *testing.T) {
	random := rand.New(rand.NewSource(3))
	var log edgeLog
	var model []pairKey
	check := func(step int, l *edgeLog, want []pairKey) {
		t.Helper()
		var got []pairKey
		l.all(func(i int, key pairKey) bool {
			if i != len(got) {
				t.Fatalf("step %d index %d out of sequence", step, i)
			}
			got = append(got, key)
			return true
		})
		if l.Len() != len(want) || !slices.Equal(got, want) {
			t.Fatalf("step %d log len %d (%d entries) want %d", step, l.Len(), len(got), len(want))
		}
		for b, block := range l.blocks {
			if b < len(l.blocks)-1 && len(block) != edgeLogBlock {
				t.Fatalf("step %d block %d holds %d, want full", step, b, len(block))
			}
			if len(block) == 0 {
				t.Fatalf("step %d empty block %d retained", step, b)
			}
		}
	}
	next := 0
	for step := range 30000 {
		switch op := random.Intn(100); {
		case op < 88:
			key := pairKey{next, -next}
			next++
			log.push(key)
			model = append(model, key)
		case op < 97:
			if len(model) > 0 {
				i := random.Intn(len(model))
				log.deleteAt(i)
				model = slices.Delete(model, i, i+1)
			}
		case op < 99:
			if random.Intn(8) != 0 {
				continue
			}
			mod := 2 + random.Intn(5)
			drop := func(key pairKey) bool { return key.source%mod == 0 }
			log.deleteFunc(drop)
			model = slices.DeleteFunc(model, drop)
		default:
			copied := log.clone()
			check(step, &copied, model)
			copied.push(pairKey{-1, -1})
			if len(model) > 0 {
				copied.deleteAt(0)
			}
			check(step, &log, model) // the clone must not alias the original
		}
		if step%97 == 0 {
			check(step, &log, model)
		}
	}
	check(-1, &log, model)
	if len(model) <= 2*edgeLogBlock {
		t.Fatalf("model only reached %d entries; test does not span blocks", len(model))
	}
}

// TestNativeIndexEquality pins that the native (Go map) node index keeps the
// natural == semantics the hash_grow index had: NaN never equals itself, so
// each AddNode(NaN) adds a node that cannot be found again; +0 and -0 are one
// node.
func TestNativeIndexEquality(t *testing.T) {
	graph := NewGraph[float64](true, true)
	nan := math.NaN()
	if err := graph.AddNode(nan); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddNode(nan); err != nil {
		t.Fatal(err)
	}
	if graph.NodeCount() != 2 || graph.ContainsNode(nan) {
		t.Fatalf("NaN nodes: count=%d contains=%v", graph.NodeCount(), graph.ContainsNode(nan))
	}
	if err := graph.AddNode(0.0); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddNode(math.Copysign(0, -1)); err == nil {
		t.Fatal("-0 must equal +0")
	}
	if err := graph.AddEdge(math.Copysign(0, -1), 0.0); err != nil || !graph.HasEdge(0.0, 0.0) {
		t.Fatalf("signed-zero self-loop: %v", err)
	}
}

// networkFingerprint runs a fixed mutation script and hashes every
// observable result. The constants in TestNetworkStorageFingerprint were
// produced by the map-based Network storage this package used before the
// ID-indexed adjacency rewrite, so they pin byte-identical behavior.
func networkFingerprint(directed, parallel bool) uint64 {
	random := rand.New(rand.NewSource(11))
	network := NewNetwork[int, int](directed, true, parallel)
	h := fnv.New64a()
	out := func(format string, args ...any) { _, _ = fmt.Fprintf(h, format, args...) }
	for step := range 3000 {
		u, v, e := random.Intn(20), random.Intn(20), random.Intn(400)
		switch random.Intn(10) {
		case 0, 1:
			out("n%v;", network.AddNode(u))
		case 2, 3, 4, 5:
			out("e%v;", network.AddEdge(u, v, e))
		case 6:
			out("r%v;", network.RemoveEdge(e))
		case 7:
			if step%3 == 0 {
				out("R%v;", network.RemoveNode(u))
			}
		default:
			incident, err := network.IncidentEdges(u)
			connecting, err2 := network.EdgesConnecting(u, v)
			d, _ := network.Degree(u)
			in, _ := network.InDegree(u)
			o, _ := network.OutDegree(u)
			out("q%v%v%v%v%v%v%v%v%v%v;", incident, err, connecting, err2, d, in, o,
				network.SuccessorsSlice(u), network.PredecessorsSlice(u), network.HasEdge(u, v))
		}
		if step%250 == 0 {
			out("N%v E%v C%d;", network.NodesSlice(), network.EdgesSlice(), network.EdgeCount())
			copied := network.Copy()
			_ = copied.AddNode(1000 + step)
			out("c%v;", slices.Equal(copied.EdgesSlice(), network.EdgesSlice()))
		}
	}
	return h.Sum64()
}

func TestNetworkStorageFingerprint(t *testing.T) {
	want := map[[2]bool]uint64{
		{false, false}: 0xe6cc93a6810629e0,
		{false, true}:  0xdcc056e57962082a,
		{true, false}:  0xa3e40e8159b72460,
		{true, true}:   0x1375050704697e82,
	}
	for key, fingerprint := range want {
		if got := networkFingerprint(key[0], key[1]); got != fingerprint {
			t.Errorf("directed=%v parallel=%v fingerprint=%#x want %#x", key[0], key[1], got, fingerprint)
		}
	}
}
