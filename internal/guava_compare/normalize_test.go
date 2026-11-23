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

func TestMultisetImplementationLabels(t *testing.T) {
	goMetrics, err := ParseGoBenchmark(strings.NewReader("BenchmarkHashMultiset-8  100  10 ns/op\n"))
	if err != nil || len(goMetrics) != 1 || goMetrics[0].Implementation != "pluto-hash-multiset" {
		t.Fatalf("unexpected Go labels: %+v, %v", goMetrics, err)
	}
	jmhMetrics, err := ParseJMHResults(strings.NewReader(`[{"benchmark":"com.example.MultisetBenchmark.treeMultiset","mode":"thrpt","threads":1,"primaryMetric":{"score":10,"scoreError":1,"scoreUnit":"ops/s"}}]`))
	if err != nil || len(jmhMetrics) != 1 || jmhMetrics[0].Implementation != "guava-tree-multiset" || jmhMetrics[0].Benchmark != "treemultiset" {
		t.Fatalf("unexpected JMH labels: %+v, %v", jmhMetrics, err)
	}
}

func TestSmallStructureImplementationLabels(t *testing.T) {
	goMetrics, err := ParseGoBenchmark(strings.NewReader("BenchmarkEvictingQueue-8  100  10 ns/op\nBenchmarkMinMaxPriorityQueue-8  100  20 ns/op\n"))
	if err != nil || len(goMetrics) != 2 || goMetrics[0].Implementation != "pluto-evicting-queue" || goMetrics[1].Implementation != "pluto-min-max-pq" {
		t.Fatalf("unexpected Go labels: %+v, %v", goMetrics, err)
	}
	jmhMetrics, err := ParseJMHResults(strings.NewReader(`[{"benchmark":"com.example.EvictingQueueBenchmark.evictingQueue","mode":"thrpt","threads":1,"primaryMetric":{"score":10,"scoreError":1,"scoreUnit":"ops/s"}},{"benchmark":"com.example.MinMaxPriorityQueueBenchmark.minMaxPriorityQueue","mode":"thrpt","threads":1,"primaryMetric":{"score":20,"scoreError":1,"scoreUnit":"ops/s"}}]`))
	if err != nil || len(jmhMetrics) != 2 || jmhMetrics[0].Implementation != "guava-evicting-queue" || jmhMetrics[1].Implementation != "guava-min-max-pq" {
		t.Fatalf("unexpected JMH labels: %+v, %v", jmhMetrics, err)
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
