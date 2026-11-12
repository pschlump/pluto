/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2_test

import (
	"bytes"
	stdbz "compress/bzip2"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/bzip2"
)

var benchData = func() []byte {
	rng := rand.New(rand.NewPCG(1234, 1234))
	data := make([]byte, 0, 1<<18)
	seg := make([]byte, 4096)
	for len(data) < 1<<18 {
		switch len(data) % 3 {
		case 0:
			for i := range seg {
				seg[i] = byte(rng.IntN(256))
			}
			data = append(data, seg...)
		case 1:
			data = append(data, bytes.Repeat([]byte{'x'}, 4000)...)
		default:
			data = append(data, []byte("the quick brown fox jumps over the lazy dog 0123456789 ")...)
		}
	}
	return data
}()

func BenchmarkCompress(b *testing.B) {
	b.SetBytes(int64(len(benchData)))
	for b.Loop() {
		_ = bzip2.Compress(benchData)
	}
}

func BenchmarkExpand(b *testing.B) {
	compressed := bzip2.Compress(benchData)
	b.SetBytes(int64(len(benchData)))
	for b.Loop() {
		_ = bzip2.Expand(compressed)
	}
}

// BenchmarkStdlibExpand gives the reference speed point for the
// decoder (the stdlib has no bzip2 writer to compare encoders).
func BenchmarkStdlibExpand(b *testing.B) {
	compressed := bzip2.Compress(benchData)
	b.SetBytes(int64(len(benchData)))
	for b.Loop() {
		r := stdbz.NewReader(bytes.NewReader(compressed))
		_, _ = io.Copy(io.Discard, r)
	}
}
