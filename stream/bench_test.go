/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream

import "testing"

// The benchmark sizes from the request note: one million adds, ranges
// over a one-million-entry stream, and AutoClaim over 100k pending
// entries.  Each Add stores one field pair whose strings are compile-time
// constants, so the measured cost is the structure, not allocation of
// per-entry string data.

// fill adds n entries with auto sequence numbers under one millisecond
// part per thousand, roughly the shape of a real ingest (many entries
// per timestamp tick).
func fillN(n uint64) *Stream {
	s := &Stream{}
	for i := uint64(0); i < n; i++ {
		_, _ = s.Add(ID{Ms: i / 1000, Seq: AutoSeq}, [][2]string{{"f", "v"}})
	}
	return s
}

// BenchmarkAdd1M appends one million entries per iteration.
func BenchmarkAdd1M(b *testing.B) {
	for range b.N {
		s := &Stream{}
		for i := uint64(0); i < 1_000_000; i++ {
			if _, err := s.Add(ID{Ms: i / 1000, Seq: AutoSeq}, [][2]string{{"f", "v"}}); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkRange1M sweeps a one-million-entry stream end to end.
func BenchmarkRange1M(b *testing.B) {
	s := fillN(1_000_000)
	b.ResetTimer()
	for range b.N {
		n := 0
		for range s.Range(MinID, MaxID, 0) {
			n++
		}
		if n != 1_000_000 {
			b.Fatalf("visited %d", n)
		}
	}
}

// BenchmarkRange1MWindow reads a 100-entry window out of a
// one-million-entry stream (the typical XRANGE page).
func BenchmarkRange1MWindow(b *testing.B) {
	s := fillN(1_000_000)
	lo := ID{Ms: 500, Seq: 100}
	b.ResetTimer()
	for range b.N {
		n := 0
		for e := range s.Range(lo, MaxID, 100) {
			_ = e
			n++
		}
		if n != 100 {
			b.Fatalf("visited %d", n)
		}
	}
}

// BenchmarkAutoClaim100kPending claims batches of 100 (the Redis default
// COUNT) from a 100k-entry pending list with minIdle 0, continuing the
// cursor each call.
func BenchmarkAutoClaim100kPending(b *testing.B) {
	s := fillN(100_000)
	if err := s.CreateGroup("g", MinID); err != nil {
		b.Fatal(err)
	}
	if got := s.ReadGroup("g", "c1", MinID, 0); len(got) != 100_000 {
		b.Fatalf("delivered %d", len(got))
	}
	b.ResetTimer()
	cursor := MinID
	for range b.N {
		entries, next, _ := s.AutoClaim("g", "c2", 0, cursor, 100)
		if len(entries) != 100 && next != MinID {
			b.Fatalf("claimed %d, next %v", len(entries), next)
		}
		if next == MinID {
			cursor = MinID // wrapped: start the next sweep
		} else {
			cursor = next
		}
	}
}
