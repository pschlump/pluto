/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package guava_compare

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var benchmarkSuffix = regexp.MustCompile(`-[0-9]+$`)

type optionalFloat float64

func (value *optionalFloat) UnmarshalJSON(data []byte) error {
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*value = optionalFloat(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == "NaN" {
		*value = 0
		return nil
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return err
	}
	*value = optionalFloat(number)
	return nil
}

type ResultMetadata struct {
	SchemaVersion     int    `json:"schemaVersion"`
	RecordedAt        string `json:"recordedAt"`
	Machine           string `json:"machine"`
	OS                string `json:"os"`
	Architecture      string `json:"architecture"`
	GoVersion         string `json:"goVersion"`
	JDKVersion        string `json:"jdkVersion"`
	GuavaCommit       string `json:"guavaCommit"`
	JMHSettings       string `json:"jmhSettings"`
	Capacity          int    `json:"capacity"`
	InputDistribution string `json:"inputDistribution"`
	Seed              int64  `json:"seed"`
}

type Metric struct {
	Benchmark         string  `json:"benchmark"`
	Implementation    string  `json:"implementation"`
	Threads           int     `json:"threads"`
	Metric            string  `json:"metric"`
	Value             float64 `json:"value"`
	Unit              string  `json:"unit"`
	Error             float64 `json:"error,omitempty"`
	Capacity          int     `json:"capacity,omitempty"`
	InputDistribution string  `json:"inputDistribution,omitempty"`
	Seed              int64   `json:"seed,omitempty"`
}

type NormalizedResults struct {
	Metadata ResultMetadata `json:"metadata"`
	Metrics  []Metric       `json:"metrics"`
}

func ParseGoBenchmark(reader io.Reader) ([]Metric, error) {
	var metrics []Metric
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		benchmark := strings.ToLower(benchmarkSuffix.ReplaceAllString(strings.TrimPrefix(fields[0], "Benchmark"), ""))
		implementation := "go-map"
		switch benchmark {
		case "hashmultiset":
			implementation = "pluto-hash-multiset"
		case "treemultiset":
			implementation = "pluto-tree-multiset"
		case "listmultimap":
			implementation = "pluto-list-multimap"
		case "setmultimap":
			implementation = "pluto-set-multimap"
		case "sortedsetmultimap":
			implementation = "pluto-sorted-set-multimap"
		case "listmap":
			implementation = "go-list-map"
		case "setmap":
			implementation = "go-set-map"
		case "hashbimap":
			implementation = "pluto-hash-bimap"
		case "mapbimap":
			implementation = "go-two-maps"
		case "bimapinverseiteration":
			implementation = "pluto-hash-bimap"
		case "hashtable":
			implementation = "pluto-hash-table"
		case "treetable":
			implementation = "pluto-tree-table"
		case "nestedmap":
			implementation = "go-nested-maps"
		case "arraytable":
			implementation = "pluto-array-table"
		case "flatmatrix":
			implementation = "go-flat-matrix"
		case "evictingqueue", "evictingqueueiteration":
			implementation = "pluto-evicting-queue"
		case "dqueueeviction", "dqueueiteration":
			implementation = "pluto-dqueue"
		case "minmaxpriorityqueue", "boundedminmaxpriorityqueue", "minmaxoneend":
			implementation = "pluto-min-max-pq"
		case "twoheap":
			implementation = "go-two-heap"
		case "priorityqueue":
			implementation = "pluto-heap"
		case "rangeset", "rangesetiteration":
			implementation = "pluto-range-set"
		case "rangemap", "rangemapiteration":
			implementation = "pluto-range-map"
		case "treesetrangepositioned":
			implementation = "pluto-tree-set-positioned"
		case "treesetrangefiltered":
			implementation = "pluto-tree-set-filtered"
		case "cachehit", "cacheevictingwrite", "cachemissloading", "cacheexpiration", "cacherefresh", "cachehitzipf", "cachehitscan", "cachehitphase", "cacheconcurrenthit8", "cacheconcurrenthit16", "cachecoalescedmiss8":
			implementation = "pluto-cache"
		case "cachelruhit":
			implementation = "pluto-lru-ts"
		case "cachetinylfuhit":
			implementation = "pluto-tiny-lfu-ts"
		}
		if strings.HasPrefix(benchmark, "hashtable") {
			implementation = "pluto-hash-table"
		} else if strings.HasPrefix(benchmark, "treetable") {
			implementation = "pluto-tree-table"
		} else if strings.HasPrefix(benchmark, "arraytable") {
			implementation = "pluto-array-table"
		}
		switch {
		case strings.HasPrefix(benchmark, "immutablelist"):
			implementation = "pluto-immutable-list"
		case strings.HasPrefix(benchmark, "immutableset"):
			implementation = "pluto-immutable-set"
		case strings.HasPrefix(benchmark, "immutablemap"):
			implementation = "pluto-immutable-map"
		case strings.HasPrefix(benchmark, "immutablesortedset"):
			implementation = "pluto-immutable-sorted-set"
		case strings.HasPrefix(benchmark, "immutablesortedmap"):
			implementation = "pluto-immutable-sorted-map"
		case strings.HasPrefix(benchmark, "immutablemultiset"):
			implementation = "pluto-immutable-multiset"
		case strings.HasPrefix(benchmark, "immutablemultimap"):
			implementation = "pluto-immutable-multimap"
		case strings.HasPrefix(benchmark, "immutablebimap"):
			implementation = "pluto-immutable-bimap"
		}
		switch {
		case strings.HasPrefix(benchmark, "genericvaluegraph"):
			implementation = "pluto-generic-value-graph"
		case strings.HasPrefix(benchmark, "genericnetwork"):
			implementation = "pluto-generic-network"
		case strings.HasPrefix(benchmark, "genericgraphdijkstradirect"):
			implementation = "pluto-integer-dijkstra"
		case strings.HasPrefix(benchmark, "genericgraph"):
			implementation = "pluto-generic-graph"
		}
		for index := 2; index+1 < len(fields); index++ {
			var metric, unit string
			switch fields[index+1] {
			case "ns/op":
				metric, unit = "latency", "ns/op"
			case "B/op":
				metric, unit = "allocated_bytes", "B/op"
			case "allocs/op":
				metric, unit = "allocations", "allocs/op"
			default:
				continue
			}
			value, err := strconv.ParseFloat(fields[index], 64)
			if err != nil {
				return nil, fmt.Errorf("parse Go benchmark %q: %w", fields[0], err)
			}
			threads := 1
			if strings.HasSuffix(benchmark, "hit8") || strings.HasSuffix(benchmark, "miss8") {
				threads = 8
			}
			if strings.HasSuffix(benchmark, "hit16") {
				threads = 16
			}
			metrics = append(metrics, Metric{
				Benchmark:      benchmark,
				Implementation: implementation,
				Threads:        threads,
				Metric:         metric,
				Value:          value,
				Unit:           unit,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return metrics, nil
}

func ParseJMHResults(reader io.Reader) ([]Metric, error) {
	var raw []struct {
		Benchmark string `json:"benchmark"`
		Mode      string `json:"mode"`
		Threads   int    `json:"threads"`
		Primary   struct {
			Score      optionalFloat `json:"score"`
			ScoreError optionalFloat `json:"scoreError"`
			ScoreUnit  string        `json:"scoreUnit"`
		} `json:"primaryMetric"`
	}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	if err := ensureEOF(decoder); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("JMH output contains no benchmark results")
	}

	metrics := make([]Metric, 0, len(raw))
	for _, result := range raw {
		metric := result.Mode
		switch result.Mode {
		case "thrpt":
			metric = "throughput"
		case "avgt", "sample":
			metric = "latency"
		}
		parts := strings.Split(result.Benchmark, ".")
		benchmark := parts[len(parts)-1]
		implementation := "guava-hash-multiset"
		switch benchmark {
		case "treeMultiset":
			implementation = "guava-tree-multiset"
		case "mapMultiset":
			implementation = "java-map"
		case "arrayListMultimap":
			implementation = "guava-array-list-multimap"
		case "hashMultimap":
			implementation = "guava-hash-multimap"
		case "treeMultimap":
			implementation = "guava-tree-multimap"
		case "listMap":
			implementation = "java-list-map"
		case "setMap":
			implementation = "java-set-map"
		case "hashBiMap", "biMapInverseIteration":
			implementation = "guava-hash-bimap"
		case "mapBiMap":
			implementation = "java-two-maps"
		case "hashTable":
			implementation = "guava-hash-table"
		case "treeTable":
			implementation = "guava-tree-table"
		case "nestedMap":
			implementation = "java-nested-maps"
		case "arrayTable":
			implementation = "guava-array-table"
		case "flatMatrix":
			implementation = "java-flat-matrix"
		case "evictingQueue", "evictingQueueIteration":
			implementation = "guava-evicting-queue"
		case "dQueueEviction", "dQueueIteration":
			implementation = "java-array-deque"
		case "minMaxPriorityQueue", "boundedMinMaxPriorityQueue", "minMaxOneEnd":
			implementation = "guava-min-max-pq"
		case "twoHeap":
			implementation = "java-two-heap"
		case "priorityQueue":
			implementation = "java-priority-queue"
		case "rangeSet", "rangeSetIteration":
			implementation = "guava-tree-range-set"
		case "rangeMap", "rangeMapIteration":
			implementation = "guava-tree-range-map"
		case "treeSetRangePositioned":
			implementation = "guava-tree-set-positioned"
		case "cacheHit", "cacheEvictingWrite", "cacheMissLoading", "cacheExpiration", "cacheRefresh", "cacheHitZipf", "cacheHitScan", "cacheHitPhase", "cacheConcurrentHit8", "cacheConcurrentHit16", "cacheCoalescedMiss8":
			implementation = "guava-local-cache"
		}
		if strings.HasPrefix(benchmark, "hashTable") {
			implementation = "guava-hash-table"
		} else if strings.HasPrefix(benchmark, "treeTable") {
			implementation = "guava-tree-table"
		} else if strings.HasPrefix(benchmark, "arrayTable") {
			implementation = "guava-array-table"
		}
		switch {
		case strings.HasPrefix(benchmark, "immutableList"):
			implementation = "guava-immutable-list"
		case strings.HasPrefix(benchmark, "immutableSet"):
			implementation = "guava-immutable-set"
		case strings.HasPrefix(benchmark, "immutableMap"):
			implementation = "guava-immutable-map"
		case strings.HasPrefix(benchmark, "immutableSortedSet"):
			implementation = "guava-immutable-sorted-set"
		case strings.HasPrefix(benchmark, "immutableSortedMap"):
			implementation = "guava-immutable-sorted-map"
		case strings.HasPrefix(benchmark, "immutableMultiset"):
			implementation = "guava-immutable-multiset"
		case strings.HasPrefix(benchmark, "immutableMultimap"):
			implementation = "guava-immutable-multimap"
		case strings.HasPrefix(benchmark, "immutableBiMap"):
			implementation = "guava-immutable-bimap"
		}
		if strings.Contains(result.Benchmark, ".GraphBenchmark.") {
			implementation = "guava-graph"
			if strings.HasPrefix(benchmark, "valueGraph") {
				implementation = "guava-value-graph"
			} else if strings.HasPrefix(benchmark, "network") {
				implementation = "guava-network"
			}
		}
		metrics = append(metrics, Metric{
			Benchmark:      strings.ToLower(benchmark),
			Implementation: implementation,
			Threads:        result.Threads,
			Metric:         metric,
			Value:          float64(result.Primary.Score),
			Unit:           result.Primary.ScoreUnit,
			Error:          float64(result.Primary.ScoreError),
		})
	}
	return metrics, nil
}

func WriteNormalizedResults(path string, metadata ResultMetadata, metrics []Metric) error {
	for index := range metrics {
		switch metrics[index].Benchmark {
		case "arraylistmultimap", "hashmultimap", "treemultimap", "listmultimap", "setmultimap", "sortedsetmultimap", "listmap", "setmap":
			metrics[index].Capacity = MultimapBenchmarkCapacity
			metrics[index].InputDistribution = MultimapBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		case "hashbimap", "mapbimap", "bimapinverseiteration":
			metrics[index].Capacity = BimapBenchmarkCapacity
			metrics[index].InputDistribution = BimapBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		case "evictingqueue", "dqueueeviction", "evictingqueueiteration", "dqueueiteration":
			metrics[index].Capacity = EvictingQueueBenchmarkCapacity
			metrics[index].InputDistribution = EvictingQueueBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		case "minmaxpriorityqueue", "boundedminmaxpriorityqueue", "twoheap":
			metrics[index].Capacity = MinMaxBenchmarkMaximumSize
			metrics[index].InputDistribution = MinMaxBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		case "minmaxoneend", "priorityqueue":
			metrics[index].InputDistribution = MinMaxOneEndBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		}
		if strings.Contains(metrics[index].Implementation, "generic-graph") || strings.Contains(metrics[index].Implementation, "generic-value-graph") || strings.Contains(metrics[index].Implementation, "generic-network") || metrics[index].Implementation == "guava-graph" || metrics[index].Implementation == "guava-value-graph" || metrics[index].Implementation == "guava-network" {
			metrics[index].Capacity = GraphBenchmarkNodes
			metrics[index].InputDistribution = GraphBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		}
		if strings.HasPrefix(metrics[index].Benchmark, "cache") {
			metrics[index].Capacity = CacheBenchmarkCapacity
			metrics[index].InputDistribution = CacheBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
			if metrics[index].Benchmark == "cacheevictingwrite" {
				metrics[index].Capacity = CacheBenchmarkEvictionCapacity
			}
		}
		if metrics[index].Benchmark == "rangeset" || metrics[index].Benchmark == "rangesetiteration" || metrics[index].Benchmark == "rangemap" || metrics[index].Benchmark == "rangemapiteration" {
			metrics[index].Capacity = RangeBenchmarkCapacity
			metrics[index].InputDistribution = RangeBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		}
		if metrics[index].Benchmark == "treesetrangepositioned" || metrics[index].Benchmark == "treesetrangefiltered" {
			metrics[index].Capacity = 4096
			metrics[index].InputDistribution = "4096 ordered integers; closed range [1024,1035]"
			metrics[index].Seed = BenchmarkSeed
		}
		if strings.HasPrefix(metrics[index].Benchmark, "immutable") {
			metrics[index].Capacity = ImmutableBenchmarkSize
			metrics[index].InputDistribution = ImmutableBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		}
		if strings.Contains(metrics[index].Benchmark, "table") || metrics[index].Benchmark == "nestedmap" || metrics[index].Benchmark == "flatmatrix" {
			metrics[index].Capacity = TableBenchmarkRows * TableBenchmarkColumns
			metrics[index].InputDistribution = TableBenchmarkInputDistribution
			metrics[index].Seed = BenchmarkSeed
		}
	}
	sort.SliceStable(metrics, func(i, j int) bool {
		if metrics[i].Benchmark != metrics[j].Benchmark {
			return metrics[i].Benchmark < metrics[j].Benchmark
		}
		if metrics[i].Implementation != metrics[j].Implementation {
			return metrics[i].Implementation < metrics[j].Implementation
		}
		return metrics[i].Metric < metrics[j].Metric
	})
	return writeJSON(path, NormalizedResults{Metadata: metadata, Metrics: metrics}, true)
}
