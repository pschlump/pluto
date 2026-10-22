/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package symbol_graph

import (
	"fmt"
	"strings"
	"testing"
)

// benchLines builds n lines of k fields each over a vocabulary of
// n/2 distinct names — dense enough to stress both passes.
func benchLines(n, k int) []string {
	rng := newRand()
	lines := make([]string, n)
	for i := range lines {
		fields := make([]string, k)
		for j := range fields {
			fields[j] = fmt.Sprintf("name%04d", rng.Intn(n/2+1))
		}
		lines[i] = strings.Join(fields, " ")
	}
	return lines
}

func BenchmarkNewSymbolGraph1000x4(b *testing.B) {
	lines := benchLines(1000, 4)
	b.ResetTimer()
	for range b.N {
		NewSymbolGraph(lines, " ")
	}
}

func BenchmarkNewSymbolGraphReader1000x4(b *testing.B) {
	text := strings.NewReader(strings.Join(benchLines(1000, 4), "\n"))
	var buf strings.Builder
	if _, err := text.WriteTo(&buf); err != nil {
		b.Fatal(err)
	}
	src := strings.NewReader(buf.String())
	b.ResetTimer()
	for range b.N {
		src.Reset(buf.String())
		if _, err := NewSymbolGraphReader(src, " "); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIndex(b *testing.B) {
	sg := NewSymbolGraph(benchLines(1000, 4), " ")
	b.ResetTimer()
	for i := range b.N {
		_, _ = sg.Index(sg.keys[i%len(sg.keys)])
	}
}

func BenchmarkIndexMiss(b *testing.B) {
	sg := NewSymbolGraph(benchLines(1000, 4), " ")
	b.ResetTimer()
	for range b.N {
		if _, found := sg.Index("nameZZZZ"); found {
			b.Fatal("IndexMiss hit")
		}
	}
}

func BenchmarkContainsMiss(b *testing.B) {
	sg := NewSymbolGraph(benchLines(1000, 4), " ")
	b.ResetTimer()
	for range b.N {
		if sg.Contains("nameZZZZ") {
			b.Fatal("ContainsMiss hit")
		}
	}
}
