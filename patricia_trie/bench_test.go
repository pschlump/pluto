package patricia_trie

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Comparison benchmarks against the sibling string-keyed tables (tst,
// trie): 1M random 16-byte binary keys, measuring build (Insert),
// Search, and KeysWithPrefix — the numbers behind the comparison table
// in README.adoc.
//
// The R-way trie is measured over the first 32k of the SAME keys, not
// all 1M: with unshared random keys it spends one 2 KiB 256-way node
// per key byte (~16M nodes, ~30 GiB at 1M keys — the documented memory
// trade-off that patricia_trie and tst exist to avoid).  Its per-operation
// costs are depth-bound (16 hops for a 16-byte key, independent of n),
// so the reduced-size numbers are directly comparable.
//
// The keys are generated lazily so plain `go test` runs pay nothing.

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/pschlump/pluto/trie"
	"github.com/pschlump/pluto/tst"
)

const (
	benchKeyLen   = 16      // random 16-byte keys, per the request note
	benchKeyCount = 1 << 20 // 1M keys for patricia_trie and tst
	benchTrieKeys = 1 << 15 // the R-way trie's slice of the same keys
)

var benchKeysOnce sync.Once
var benchKeys []string // benchKeyCount random benchKeyLen-byte keys, fixed seed

// keys returns the shared benchmark keyspace, generating it once.
func keys() []string {
	benchKeysOnce.Do(func() {
		rng := rand.New(rand.NewPCG(42, 7)) // fixed seed: deterministic runs
		benchKeys = make([]string, benchKeyCount)
		for i := range benchKeys {
			b := make([]byte, benchKeyLen)
			for j := range b {
				b[j] = byte(rng.IntN(256))
			}
			benchKeys[i] = string(b)
		}
	})
	return benchKeys
}

// stringTable is the surface the three contenders share; used as a Go
// generic constraint (not an interface value), so each instantiation is
// compiled and called directly — no dynamic dispatch in the measured
// loops (the sharded_hash_ts benchmark precedent).
type stringTable interface {
	Insert(string, int) bool
	Search(string) (int, bool)
	KeysWithPrefix(string) []string
}

// benchBuild measures full builds from scratch, reporting ns/key.
func benchBuild[T stringTable](b *testing.B, newTable func() T, keys []string) {
	b.Helper()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tt := newTable()
		for j, k := range keys {
			tt.Insert(k, j)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(keys)), "ns/key")
}

// prepare builds a contender over keys (untimed).
func prepare[T stringTable](b *testing.B, newTable func() T, keys []string) T {
	b.Helper()
	tt := newTable()
	for j, k := range keys {
		tt.Insert(k, j)
	}
	return tt
}

// benchSearch measures successful searches over a prebuilt table.
func benchSearch[T stringTable](b *testing.B, tt T, keys []string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tt.Search(keys[i&(len(keys)-1)])
	}
}

// benchKeysWithPrefix measures a 2-byte prefix query over a prebuilt
// table (expected match count n/65536: ~16 of 1M keys, ~0.5 of 32k).
func benchKeysWithPrefix[T stringTable](b *testing.B, tt T, keys []string) {
	b.Helper()
	prefix := keys[0][:2]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tt.KeysWithPrefix(prefix)
	}
}

func BenchmarkTableBuild(b *testing.B) {
	b.Run("patricia_trie_1M", func(b *testing.B) {
		benchBuild(b, func() *PatriciaTrie[int] { return &PatriciaTrie[int]{} }, keys())
	})
	b.Run("tst_1M", func(b *testing.B) {
		benchBuild(b, func() *tst.Tst[int] { return &tst.Tst[int]{} }, keys())
	})
	b.Run("trie_32k", func(b *testing.B) {
		benchBuild(b, func() *trie.Trie[int] { return &trie.Trie[int]{} }, keys()[:benchTrieKeys])
	})
}

func BenchmarkTableSearch(b *testing.B) {
	b.Run("patricia_trie_1M", func(b *testing.B) {
		benchSearch(b, prepare(b, func() *PatriciaTrie[int] { return &PatriciaTrie[int]{} }, keys()), keys())
	})
	b.Run("tst_1M", func(b *testing.B) {
		benchSearch(b, prepare(b, func() *tst.Tst[int] { return &tst.Tst[int]{} }, keys()), keys())
	})
	b.Run("trie_32k", func(b *testing.B) {
		benchSearch(b, prepare(b, func() *trie.Trie[int] { return &trie.Trie[int]{} }, keys()[:benchTrieKeys]), keys()[:benchTrieKeys])
	})
}

func BenchmarkTableKeysWithPrefix(b *testing.B) {
	b.Run("patricia_trie_1M", func(b *testing.B) {
		benchKeysWithPrefix(b, prepare(b, func() *PatriciaTrie[int] { return &PatriciaTrie[int]{} }, keys()), keys())
	})
	b.Run("tst_1M", func(b *testing.B) {
		benchKeysWithPrefix(b, prepare(b, func() *tst.Tst[int] { return &tst.Tst[int]{} }, keys()), keys())
	})
	b.Run("trie_32k", func(b *testing.B) {
		benchKeysWithPrefix(b, prepare(b, func() *trie.Trie[int] { return &trie.Trie[int]{} }, keys()[:benchTrieKeys]), keys()[:benchTrieKeys])
	})
}

// BenchmarkTableKeysThatMatch measures the Redis-glob query on a 1M-key
// trie: a 2-byte literal prefix plus '*' (~16 matches) — the literal
// prefix prunes the descent to the matching subtree.
func BenchmarkTableKeysThatMatch(b *testing.B) {
	pt := prepare(b, func() *PatriciaTrie[int] { return &PatriciaTrie[int]{} }, keys())
	pattern := keys()[0][:2] + "*"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := 0
		for range pt.KeysThatMatch(pattern) {
			n++
		}
		_ = n
	}
}

// BenchmarkTableLongestPrefixOf measures the longest-prefix query on a
// 1M-key trie of random keys — the not-found descent (no stored key is
// a prefix of a random 16-byte key), the cost a KEYS-style scan pays
// per lookup.
func BenchmarkTableLongestPrefixOf(b *testing.B) {
	pt := prepare(b, func() *PatriciaTrie[int] { return &PatriciaTrie[int]{} }, keys())
	k := keys()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pt.LongestPrefixOf(k[i&(benchKeyCount-1)])
	}
}
