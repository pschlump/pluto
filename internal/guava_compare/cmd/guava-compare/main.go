/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pschlump/pluto/internal/guava_compare"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = generate(os.Args[2:])
	case "oracle":
		err = oracle(os.Args[2:])
	case "normalize":
		err = normalize(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "guava-compare:", err)
		os.Exit(1)
	}
}

func generate(arguments []string) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	tracePath := flags.String("trace", "internal/guava_compare/testdata/counter.trace.json", "trace output")
	corpusPath := flags.String("corpus", "internal/guava_compare/testdata/counter.corpus.json", "benchmark corpus output")
	multimapCorpusPath := flags.String("multimap-corpus", "internal/guava_compare/testdata/multimap.corpus.json", "multimap benchmark corpus output")
	bimapCorpusPath := flags.String("bimap-corpus", "internal/guava_compare/testdata/bimap.corpus.json", "bimap benchmark corpus output")
	tableCorpusPath := flags.String("table-corpus", "internal/guava_compare/testdata/table.corpus.json", "table benchmark corpus output")
	evictingQueueCorpusPath := flags.String("evicting-queue-corpus", "internal/guava_compare/testdata/evicting_queue.corpus.json", "evicting queue benchmark corpus output")
	minMaxCorpusPath := flags.String("min-max-corpus", "internal/guava_compare/testdata/min_max_pq.corpus.json", "min-max queue benchmark corpus output")
	immutableCorpusPath := flags.String("immutable-corpus", "internal/guava_compare/testdata/immutable.corpus.json", "immutable benchmark corpus output")
	rangeCorpusPath := flags.String("range-corpus", "internal/guava_compare/testdata/range.corpus.json", "range benchmark corpus output")
	seed := flags.Int64("seed", guava_compare.BenchmarkSeed, "random seed")
	operations := flags.Int("operations", 68, "trace operation count")
	corpusOperations := flags.Int("corpus-operations", guava_compare.BenchmarkCorpusSize, "benchmark operation count")
	capacity := flags.Int("capacity", guava_compare.BenchmarkCapacity, "distinct key capacity")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *operations <= 0 || *corpusOperations <= 0 || *capacity <= 0 {
		return fmt.Errorf("operation counts and capacity must be positive")
	}
	if err := guava_compare.WriteTrace(*tracePath, guava_compare.GenerateTrace(*seed, *operations)); err != nil {
		return err
	}
	if err := guava_compare.WriteCorpus(*corpusPath, guava_compare.GenerateCorpus(*seed, *corpusOperations, *capacity)); err != nil {
		return err
	}
	if err := guava_compare.WriteMultimapCorpus(*multimapCorpusPath, guava_compare.GenerateMultimapCorpus(*seed, *corpusOperations, guava_compare.MultimapBenchmarkCapacity, guava_compare.MultimapBenchmarkValuesPerKey)); err != nil {
		return err
	}
	if err := guava_compare.WriteMultimapCorpus(*bimapCorpusPath, guava_compare.GenerateBimapCorpus(*seed, *corpusOperations, guava_compare.BimapBenchmarkCapacity)); err != nil {
		return err
	}
	if err := guava_compare.WriteTableCorpus(*tableCorpusPath, guava_compare.GenerateTableCorpus(*seed, *corpusOperations, guava_compare.TableBenchmarkRows, guava_compare.TableBenchmarkColumns)); err != nil {
		return err
	}
	if err := guava_compare.WriteEvictingQueueCorpus(*evictingQueueCorpusPath, guava_compare.GenerateEvictingQueueCorpus(*seed, *corpusOperations, guava_compare.EvictingQueueBenchmarkCapacity)); err != nil {
		return err
	}
	if err := guava_compare.WriteMinMaxCorpus(*minMaxCorpusPath, guava_compare.GenerateMinMaxCorpus(*seed, *corpusOperations, guava_compare.MinMaxBenchmarkMaximumSize)); err != nil {
		return err
	}
	if err := guava_compare.WriteImmutableCorpus(*immutableCorpusPath, guava_compare.GenerateImmutableCorpus(*seed, guava_compare.ImmutableBenchmarkSize)); err != nil {
		return err
	}
	return guava_compare.WriteRangeCorpus(*rangeCorpusPath, guava_compare.GenerateRangeCorpus(*seed, *corpusOperations, guava_compare.RangeBenchmarkCapacity))
}

func oracle(arguments []string) error {
	flags := flag.NewFlagSet("oracle", flag.ContinueOnError)
	tracePath := flags.String("trace", "", "input trace")
	outputPath := flags.String("output", "", "canonical result output")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *tracePath == "" || *outputPath == "" {
		return fmt.Errorf("-trace and -output are required")
	}
	trace, err := guava_compare.ReadTrace(*tracePath)
	if err != nil {
		return err
	}
	result, err := guava_compare.ExecuteTrace(trace)
	if err != nil {
		return err
	}
	return guava_compare.WriteTraceResult(*outputPath, result)
}

func normalize(arguments []string) error {
	flags := flag.NewFlagSet("normalize", flag.ContinueOnError)
	goPath := flags.String("go", "", "Go benchmark text output")
	jmhPath := flags.String("jmh", "", "JMH JSON output")
	outputPath := flags.String("output", "", "normalized JSON output")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *goPath == "" || *jmhPath == "" || *outputPath == "" {
		return fmt.Errorf("-go, -jmh, and -output are required")
	}
	goFile, err := os.Open(*goPath)
	if err != nil {
		return err
	}
	goMetrics, parseErr := guava_compare.ParseGoBenchmark(goFile)
	closeErr := goFile.Close()
	if parseErr != nil {
		return parseErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(goMetrics) == 0 {
		return fmt.Errorf("go output contains no benchmark results")
	}
	jmhFile, err := os.Open(*jmhPath)
	if err != nil {
		return err
	}
	jmhMetrics, parseErr := guava_compare.ParseJMHResults(jmhFile)
	closeErr = jmhFile.Close()
	if parseErr != nil {
		return parseErr
	}
	if closeErr != nil {
		return closeErr
	}
	metadata, err := collectMetadata()
	if err != nil {
		return err
	}
	return guava_compare.WriteNormalizedResults(*outputPath, metadata, append(goMetrics, jmhMetrics...))
}

func collectMetadata() (guava_compare.ResultMetadata, error) {
	machine, err := os.Hostname()
	if err != nil {
		return guava_compare.ResultMetadata{}, err
	}
	goOutput, err := exec.Command("go", "version").Output()
	if err != nil {
		return guava_compare.ResultMetadata{}, fmt.Errorf("get Go version: %w", err)
	}
	jdkOutput, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		return guava_compare.ResultMetadata{}, fmt.Errorf("get JDK version: %w", err)
	}
	jdkVersion := strings.TrimSpace(strings.SplitN(string(jdkOutput), "\n", 2)[0])
	return guava_compare.ResultMetadata{
		SchemaVersion:     guava_compare.TraceVersion,
		RecordedAt:        time.Now().UTC().Format(time.RFC3339),
		Machine:           machine,
		OS:                runtime.GOOS,
		Architecture:      runtime.GOARCH,
		GoVersion:         strings.TrimSpace(string(goOutput)),
		JDKVersion:        jdkVersion,
		GuavaCommit:       guava_compare.GuavaCommit,
		JMHSettings:       guava_compare.BenchmarkJMHSettings,
		Capacity:          guava_compare.BenchmarkCapacity,
		InputDistribution: guava_compare.BenchmarkInputDistribution,
		Seed:              guava_compare.BenchmarkSeed,
	}, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: guava-compare generate|oracle|normalize [options]")
}
