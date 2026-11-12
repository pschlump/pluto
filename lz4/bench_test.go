/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4_test

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/pschlump/pluto/lz4"
)

func benchCorpus() []byte {
	rng := rand.New(rand.NewPCG(42, 42))
	text := []byte(strings.Repeat("Call me Ishmael. Some years ago—never mind how long precisely—", 300))
	random := make([]byte, 8192)
	for i := range random {
		random[i] = byte(rng.IntN(256))
	}
	return append(append([]byte{}, text...), random...) // 25 KiB mixed
}

func BenchmarkCompress(b *testing.B) {
	data := benchCorpus()
	for b.Loop() {
		_ = lz4.Compress(data)
	}
}

func BenchmarkExpand(b *testing.B) {
	data := benchCorpus()
	z := lz4.Compress(data)
	for b.Loop() {
		if back := lz4.Expand(z); !bytes.Equal(back, data) {
			b.Fatal("round trip mismatch")
		}
	}
}
