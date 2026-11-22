#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"

for tool in go java git cmp tee; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    printf 'guava-bench: required tool %q was not found in PATH\n' "$tool" >&2
    exit 1
  fi
done

if command -v mvn >/dev/null 2>&1; then
  MAVEN=(mvn)
elif [[ -x note/guava/mvnw ]]; then
  MAVEN=(note/guava/mvnw)
else
  printf '%s\n' 'guava-bench: install Maven or provide the executable note/guava/mvnw wrapper' >&2
  exit 1
fi

EXPECTED_COMMIT=36e3995499d6c3b74b1207d362fa9e19915d2589
if [[ ! -d note/guava/.git ]]; then
  printf '%s\n' 'guava-bench: clone https://github.com/google/guava.git into note/guava' >&2
  exit 1
fi
ACTUAL_COMMIT=$(git -C note/guava rev-parse HEAD)
if [[ "$ACTUAL_COMMIT" != "$EXPECTED_COMMIT" ]]; then
  printf 'guava-bench: note/guava is at %s; expected pinned commit %s\n' "$ACTUAL_COMMIT" "$EXPECTED_COMMIT" >&2
  exit 1
fi
if ! git -C note/guava diff --quiet || ! git -C note/guava diff --cached --quiet; then
  printf '%s\n' 'guava-bench: note/guava has tracked modifications; use a clean pinned checkout' >&2
  exit 1
fi

RESULTS=internal/guava_compare/results
TRACE=internal/guava_compare/testdata/counter.trace.json
CORPUS=internal/guava_compare/testdata/counter.corpus.json
POM=internal/guava_compare/java/pom.xml
JAR=internal/guava_compare/java/target/guava-compare-1.0.0-SNAPSHOT.jar
mkdir -p "$RESULTS"

printf '%s\n' 'Compiling the pinned Guava source and comparison drivers...'
"${MAVEN[@]}" -q -f "$POM" clean package

go run ./internal/guava_compare/cmd/guava-compare oracle \
  -trace "$TRACE" -output "$RESULTS/go-oracle.json"
java -cp "$JAR" com.pschlump.pluto.guava.CounterOracle \
  "$TRACE" "$RESULTS/java-oracle.json"
cmp "$RESULTS/go-oracle.json" "$RESULTS/java-oracle.json"
cmp "$RESULTS/java-oracle.json" internal/guava_compare/testdata/counter.golden.json
printf '%s\n' 'Go and Java oracle results match the committed golden.'

go test ./internal/guava_compare -run '^$' -bench '^BenchmarkCounter$' \
  -benchmem -benchtime=1s -count=3 | tee "$RESULTS/go-bench.txt"
java -Dcorpus="$ROOT/$CORPUS" -jar "$JAR" \
  CounterBenchmark -rf json -rff "$ROOT/$RESULTS/jmh.json"

go run ./internal/guava_compare/cmd/guava-compare normalize \
  -go "$RESULTS/go-bench.txt" -jmh "$RESULTS/jmh.json" \
  -output "$RESULTS/normalized.json"
printf 'Raw and normalized results are in %s\n' "$RESULTS"
