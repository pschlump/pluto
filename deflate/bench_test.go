/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate_test

import (
	"bytes"
	"compress/flate"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/deflate"
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
	for b.Loop() {
		_ = deflate.Compress(benchData)
	}
}

func BenchmarkExpand(b *testing.B) {
	compressed := deflate.Compress(benchData)
	b.SetBytes(int64(len(benchData)))
	for b.Loop() {
		_, _ = deflate.Expand(compressed)
	}
}

// BenchmarkStdlibCompress/Expand give the reference ratio/speed points
// (stdlib level 6) for comparison.
func BenchmarkStdlibCompress(b *testing.B) {
	b.SetBytes(int64(len(benchData)))
	for b.Loop() {
		w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression)
		_, _ = w.Write(benchData)
		_ = w.Close()
	}
}

func BenchmarkStdlibExpand(b *testing.B) {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	_, _ = w.Write(benchData)
	_ = w.Close()
	compressed := buf.Bytes()
	b.SetBytes(int64(len(benchData)))
	for b.Loop() {
		r := flate.NewReader(bytes.NewReader(compressed))
		_, _ = io.Copy(io.Discard, r)
	}
}
