/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_ts

import (
	"runtime"
	"testing"
	"time"
)

func TestRingBufferOrderingAndReuse(t *testing.T) {
	ring := newRingBuffer[int](4)
	for i := range 4 {
		if accepted, _ := ring.offer(i); !accepted {
			t.Fatalf("offer(%d) failed before capacity", i)
		}
	}
	if accepted, _ := ring.offer(4); accepted {
		t.Fatal("offer succeeded on a full ring")
	}
	for round := range 3 {
		var got []int
		ring.drain(func(value int) { got = append(got, value) })
		want := []int{round * 4, round*4 + 1, round*4 + 2, round*4 + 3}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("round %d values = %v, want %v", round, got, want)
			}
		}
		for i := range 4 {
			if accepted, _ := ring.offer((round+1)*4 + i); !accepted {
				t.Fatalf("reuse offer failed in round %d", round)
			}
		}
	}
}

func TestWorkerExitsAfterDraining(t *testing.T) {
	c := NewTinyLfu[int, int](16)
	for i := range 1000 {
		c.Put(i, i)
		c.Get(i)
	}
	c.Stats()
	deadline := time.Now().Add(5 * time.Second)
	for c.workerActive.Load() {
		if time.Now().After(deadline) {
			t.Fatal("maintenance worker did not exit after buffers drained")
		}
		runtime.Gosched()
	}
	if c.hasPending() {
		t.Error("worker exited with pending events")
	}
}

func TestReadBuffersRemainBounded(t *testing.T) {
	c := NewTinyLfu[int, int](1)
	c.Put(1, 1)
	for range 10000 {
		c.Get(1)
	}
	for i := range c.readBuffers {
		if c.readBuffers[i].pending() > readBufferSize {
			t.Fatalf("read buffer pending = %d, capacity = %d", c.readBuffers[i].pending(), readBufferSize)
		}
	}
	c.Stats()
	if c.hasPending() {
		t.Error("Stats left pending maintenance")
	}
}
