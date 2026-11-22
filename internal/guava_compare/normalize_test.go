/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package guava_compare

import (
	"strings"
	"testing"
)

func TestParseGoBenchmark(t *testing.T) {
	input := "goos: darwin\nBenchmarkCounter-8  1000  12.5 ns/op  64 B/op  2 allocs/op\nPASS\n"
	metrics, err := ParseGoBenchmark(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 {
		t.Fatalf("got %d metrics, want 3", len(metrics))
	}
	if metrics[0].Benchmark != "counter" || metrics[0].Metric != "latency" || metrics[0].Value != 12.5 {
		t.Fatalf("unexpected latency metric: %+v", metrics[0])
	}
	if metrics[1].Metric != "allocated_bytes" || metrics[2].Metric != "allocations" {
		t.Fatalf("allocation metrics were not preserved separately: %+v", metrics)
	}
}

func TestParseEmptyJMHResultsFails(t *testing.T) {
	if _, err := ParseJMHResults(strings.NewReader("[]")); err == nil {
		t.Fatal("empty JMH output must be rejected")
	}
}

func TestParseJMHResults(t *testing.T) {
	input := `[{"benchmark":"com.example.CounterBenchmark.counter","mode":"thrpt","threads":1,"primaryMetric":{"score":123.0,"scoreError":4.5,"scoreUnit":"ops/s"}}]`
	metrics, err := ParseJMHResults(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}
	metric := metrics[0]
	if metric.Benchmark != "counter" || metric.Metric != "throughput" || metric.Value != 123 || metric.Error != 4.5 {
		t.Fatalf("unexpected JMH metric: %+v", metric)
	}
}
