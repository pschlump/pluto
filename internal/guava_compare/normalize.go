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
	Benchmark      string  `json:"benchmark"`
	Implementation string  `json:"implementation"`
	Threads        int     `json:"threads"`
	Metric         string  `json:"metric"`
	Value          float64 `json:"value"`
	Unit           string  `json:"unit"`
	Error          float64 `json:"error,omitempty"`
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
			metrics = append(metrics, Metric{
				Benchmark:      benchmark,
				Implementation: implementation,
				Threads:        1,
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
			Score      float64 `json:"score"`
			ScoreError float64 `json:"scoreError"`
			ScoreUnit  string  `json:"scoreUnit"`
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
		}
		metrics = append(metrics, Metric{
			Benchmark:      strings.ToLower(benchmark),
			Implementation: implementation,
			Threads:        result.Threads,
			Metric:         metric,
			Value:          result.Primary.Score,
			Unit:           result.Primary.ScoreUnit,
			Error:          result.Primary.ScoreError,
		})
	}
	return metrics, nil
}

func WriteNormalizedResults(path string, metadata ResultMetadata, metrics []Metric) error {
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
