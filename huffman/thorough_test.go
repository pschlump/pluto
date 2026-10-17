/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package huffman

import (
	"bytes"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
)

// parseTrie reads just the trie prefix of a blob — the shared helper
// for the invariant checks.
func parseTrie(blob []byte) (*huffNode, bool) {
	r := bitReader{data: blob}
	used := 0
	return readTrie(&r, &used)
}

// symbolSet is the set of distinct bytes in data.
func symbolSet(data []byte) map[byte]bool {
	m := make(map[byte]bool)
	for _, b := range data {
		m[b] = true
	}
	return m
}

// checkTrieInvariants verifies what a serialized trie must hold: a full
// binary tree (every node has 0 or 2 children), at most 256 leaves,
// Kraft's equality Σ 2^-depth == 1 (a complete prefix-free code), and —
// when want is non-nil — exactly the caller's symbol set at the leaves.
func checkTrieInvariants(t *testing.T, root *huffNode, want map[byte]bool) {
	t.Helper()
	leaves := 0
	kraft := 0.0
	seen := make(map[byte]bool)
	var walk func(n *huffNode, depth int)
	walk = func(n *huffNode, depth int) {
		if n.isLeaf() {
			leaves++
			kraft += math.Pow(0.5, float64(depth))
			seen[n.ch] = true
			return
		}
		walk(n.left, depth+1)
		walk(n.right, depth+1)
	}
	walk(root, 0)
	if leaves < 1 || leaves > alphabet {
		t.Errorf("trie has %d leaves, want 1..%d", leaves, alphabet)
	}
	if math.Abs(kraft-1) > 1e-9 {
		t.Errorf("Kraft sum = %g, want 1", kraft)
	}
	if want != nil {
		if len(seen) != len(want) {
			t.Errorf("trie carries %d distinct symbols, input had %d", len(seen), len(want))
		}
		for c := range want {
			if !seen[c] {
				t.Errorf("symbol %q missing from the trie", c)
			}
		}
	}
}

// refOptimalCost computes the optimal weighted path length by a
// structurally different algorithm from buildTrie: no heap, just
// repeated linear scans for the two smallest live weights.  Every
// Huffman trie — whatever its tie-breaking — achieves this cost, so
// equality pins optimality without pinning shape.
func refOptimalCost(freq [alphabet]int64) int64 {
	w := make([]int64, 0, alphabet)
	for _, f := range freq {
		if f > 0 {
			w = append(w, f)
		}
	}
	if len(w) <= 1 {
		return 0
	}
	var total int64
	for len(w) > 1 {
		i0, i1 := 0, 1 // i0 smallest, i1 second smallest
		if w[i1] < w[i0] {
			i0, i1 = i1, i0
		}
		for i := 2; i < len(w); i++ {
			if w[i] < w[i0] {
				i1, i0 = i0, i
			} else if w[i] < w[i1] {
				i1 = i
			}
		}
		total += w[i0] + w[i1]
		w[i0] = w[i0] + w[i1]
		w[i1] = w[len(w)-1]
		w = w[:len(w)-1]
	}
	return total
}

// codeCost is the weighted path length of a built code: Σ freq·len.
func codeCost(freq *[alphabet]int64, codes *[alphabet]string) int64 {
	var cost int64
	for c := range alphabet {
		if freq[c] > 0 {
			cost += freq[c] * int64(len(codes[c]))
		}
	}
	return cost
}

// genData builds a test input in one of six entropy regimes — the
// regimes that stress different trie shapes.
func genData(rng *rand.Rand, iter int) []byte {
	n := rng.IntN(600)
	data := make([]byte, n)
	switch iter % 6 {
	case 0: // uniform over all 256 symbols — flat trie, 8-ish bit codes
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
	case 1: // binary alphabet — one-bit codes
		for i := range data {
			data[i] = byte(rng.IntN(2))
		}
	case 2: // four symbols
		for i := range data {
			data[i] = byte('a' + rng.IntN(4))
		}
	case 3: // heavy skew toward small byte values — a deep, ragged trie
		for i := range data {
			data[i] = byte(255 * math.Pow(rng.Float64(), 8))
		}
	case 4: // long runs over six symbols — Huffman's home turf
		for i := 0; i < len(data); {
			sym := byte(rng.IntN(6))
			run := 1 + rng.IntN(200)
			for j := 0; j < run && i < len(data); j++ {
				data[i] = sym
				i++
			}
		}
	case 5: // a single symbol — the zero-bit codeword
		for i := range data {
			data[i] = 'Z'
		}
	}
	return data
}

// TestRandomizedRoundTrip is the centerpiece: across the six regimes,
// every input round-trips, the format is deterministic (recompressing
// an expansion reproduces the blob byte for byte), and every blob's
// trie is structurally sound.
func TestRandomizedRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for iter := 0; iter < 400; iter++ {
		data := genData(rng, iter)
		blob := Compress(data)
		if len(data) == 0 {
			if len(blob) != 0 {
				t.Fatalf("iter %d: Compress(empty) = % x, want empty", iter, blob)
			}
			continue
		}
		if back := Expand(blob); !bytes.Equal(back, data) {
			t.Fatalf("iter %d: round trip failed (len %d)", iter, len(data))
		}
		if again := Compress(Expand(blob)); !bytes.Equal(again, blob) {
			t.Fatalf("iter %d: recompression is not byte-identical", iter)
		}
		root, ok := parseTrie(blob)
		if !ok {
			t.Fatalf("iter %d: trie prefix of blob does not parse", iter)
		}
		checkTrieInvariants(t, root, symbolSet(data))
	}
}

// TestOptimalityVsReference cross-checks the built code's weighted path
// length against the scan-based reference for random frequency tables —
// including ties and equal-frequency tables, where tie-breaking shapes
// differ but the cost must not.
func TestOptimalityVsReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for iter := 0; iter < 300; iter++ {
		var freq [alphabet]int64
		syms := 1 + rng.IntN(alphabet)
		switch iter % 3 {
		case 0: // wide-range counts, ties rare
			for c := 0; c < syms; c++ {
				freq[c] = 1 + int64(rng.IntN(1_000_000))
			}
		case 1: // small counts, ties everywhere
			for c := 0; c < syms; c++ {
				freq[c] = 1 + int64(rng.IntN(4))
			}
		case 2: // all equal — every tie-break story at once
			for c := 0; c < syms; c++ {
				freq[c] = 7
			}
		}
		root := buildTrie(&freq)
		var codes [alphabet]string
		buildCode(&codes, root, "")
		if got, want := codeCost(&freq, &codes), refOptimalCost(freq); got != want {
			t.Errorf("iter %d: weighted path length %d, reference optimum %d", iter, got, want)
		}
		checkTrieInvariants(t, root, symbolSetOfFreq(&freq))
	}
}

func symbolSetOfFreq(freq *[alphabet]int64) map[byte]bool {
	m := make(map[byte]bool)
	for c := range alphabet {
		if freq[c] > 0 {
			m[byte(c)] = true
		}
	}
	return m
}

// Fibonacci-weighted frequencies are the canonical pathological skew:
// every merge lands exactly on the next existing weight, and the trie
// degenerates toward a chain.  Eighty-five symbols make codewords far
// longer than 64 bits — which is why buildCode stores strings — while
// the largest frequency (F(87) ≈ 6.8e17) stays comfortably inside int64
// (the sequence must NOT run past ~90 symbols: Fibonacci overflows
// int64 there, and the wrapped values stop being frequencies at all).
func TestFibonacciDeepCodes(t *testing.T) {
	var freq [alphabet]int64
	a, b := int64(1), int64(2)
	for c := 0; c < 85; c++ {
		freq[c] = a
		a, b = b, a+b
	}
	root := buildTrie(&freq)
	var codes [alphabet]string
	buildCode(&codes, root, "")
	longest := 0
	for _, c := range codes {
		if len(c) > longest {
			longest = len(c)
		}
	}
	// The deterministic tie-break makes the chain exact: 85 symbols,
	// deepest codeword 84 bits (observed and pinned; any Huffman trie
	// over these weights puts some symbol at depth ≥ 65 regardless).
	if longest != 84 {
		t.Errorf("longest Fibonacci codeword = %d bits, want 84", longest)
	}
	if got, want := codeCost(&freq, &codes), refOptimalCost(freq); got != want {
		t.Errorf("weighted path length %d, reference optimum %d", got, want)
	}
	checkTrieInvariants(t, root, symbolSetOfFreq(&freq))
}

// No input Compress can accept produces codewords longer than its
// feasible frequency sums allow, so deep decode is pinned from the
// expand side: a hand-built left chain of 100 internal nodes (201 trie
// nodes, inside the 511 cap) with codewords of length 100 and 1..100.
func TestDeepCodesExpand(t *testing.T) {
	head := &huffNode{ch: 'A'}
	for range 100 {
		head = &huffNode{left: head, right: &huffNode{ch: 'B'}}
	}
	var w bitWriter
	writeTrie(&w, head)
	w.writeBits(3, 64)
	w.writeCode(strings.Repeat("0", 100))      // A, 100 bits deep
	w.writeCode("1")                           // a B at depth 1
	w.writeCode(strings.Repeat("0", 99) + "1") // a B at depth 100

	got := Expand(w.bytes())
	if string(got) != "ABB" {
		t.Errorf("Expand(deep-chain blob) = %q, want ABB", got)
	}
}

// Every strict prefix of a valid blob is malformed: the message's bits
// run to the end of the blob (the writer never pads a whole byte), so
// cutting anywhere starves the trie, the count, or a codeword.
func TestMalformedTruncation(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	big := make([]byte, 1024)
	for i := range big {
		big[i] = byte(rng.IntN(256))
	}
	inputs := [][]byte{
		[]byte("A"),
		[]byte("AB"),
		[]byte("AAB"),
		bytes.Repeat([]byte("abracadabra"), 30),
		big,
	}
	for _, in := range inputs {
		blob := Compress(in)
		for k := 0; k < len(blob); k++ {
			if got := Expand(blob[:k]); got != nil {
				t.Fatalf("Expand(blob[:%d]) decoded %d bytes, want nil", k, len(got))
			}
		}
	}
}

// An all-zero blob is an endless chain of internal-node bits and an
// all-ones blob an endless chain of leaves; both must trip the 511-node
// cap and report malformed, not loop or recurse without bound.
func TestMalformedTrieCap(t *testing.T) {
	if got := Expand(make([]byte, 600)); got != nil {
		t.Errorf("Expand(600 zero bytes) = %d bytes, want nil", len(got))
	}
	if got := Expand(bytes.Repeat([]byte{0xFF}, 600)); got != nil {
		t.Errorf("Expand(600 0xFF bytes) = %d bytes, want nil", len(got))
	}
}

// Random garbage never panics.  With the first bit forced to 0 the root
// is internal, so every symbol costs at least one bit and a decode that
// does succeed is bounded by the blob's own bits — the provable half of
// the bomb story (the single-leaf half is the next test).
func TestMalformedGarbage(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for i := 0; i < 300; i++ {
		blob := make([]byte, 1+rng.IntN(64))
		for j := range blob {
			blob[j] = byte(rng.IntN(256))
		}
		blob[0] &^= 0x80 // internal root: bounded output
		if got := Expand(blob); got != nil && len(got) > 8*len(blob) {
			t.Fatalf("garbage blob decoded to %d bytes from %d-byte input", len(got), len(blob))
		}
	}
}

// A crafted single-leaf blob with a large count inflates — every symbol
// costs zero bits, so the claim is unfalsifiable by the blob's own
// length.  This is legal by the format (it is what Compress produces
// for constant data) and is the documented decompression-bomb exposure.
func TestCountInflationIsLegal(t *testing.T) {
	var w bitWriter
	w.writeBit(1)
	w.writeBits('Q', 8)
	w.writeBits(0, 32)
	w.writeBits(1_000_000, 32)
	got := Expand(w.bytes())
	if len(got) != 1_000_000 {
		t.Fatalf("inflated blob decoded to %d bytes, want 1_000_000", len(got))
	}
	for _, b := range got {
		if b != 'Q' {
			t.Fatalf("inflated blob decoded a non-'Q' byte")
		}
	}
}

func TestConcurrentRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("abracadabra"), 50)
	blob := Compress(data)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !bytes.Equal(Expand(blob), data) {
				t.Error("concurrent round trip failed")
			}
			if !bytes.Equal(Compress(data), blob) {
				t.Error("concurrent compression is not deterministic")
			}
		}()
	}
	wg.Wait()
}
