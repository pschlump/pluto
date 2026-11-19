/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu

import (
	"bufio"
	"compress/gzip"
	"os"
	"strconv"
	"strings"
	"testing"
)

// These traces are from Caffeine's simulator resources, the paper's cited
// open-source implementation. They exercise an underlying loop and a scan.
func TestCaffeineTraceReplays(t *testing.T) {
	for _, name := range []string{"loop", "scan"} {
		t.Run(name, func(t *testing.T) {
			trace := readTrace(t, "testdata/"+name+".trace.gz")
			c := NewTinyLfu[int, int](512)
			c.sketch = NewFrequencySketchWithHash(512, func(key int) uint64 {
				return uint64(int64(key))
			})
			for _, key := range trace {
				request(c, key)
			}
			checkCacheInvariants(t, c)
			stats := c.Stats()
			if stats.Hits+stats.Misses != uint64(len(trace)) {
				t.Errorf("Hits+Misses = %d, trace events = %d", stats.Hits+stats.Misses, len(trace))
			}
			if name == "loop" && stats.Hits == 0 {
				t.Error("looping trace had no hits")
			}
		})
	}
}

func readTrace(t *testing.T, path string) []int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	var trace []int
	scanner := bufio.NewScanner(compressed)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		key, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			t.Fatalf("invalid trace line %q: %v", scanner.Text(), err)
		}
		trace = append(trace, key)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(trace) == 0 {
		t.Fatal("trace is empty")
	}
	return trace
}
