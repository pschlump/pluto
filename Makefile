# Pluto - generic data structures for Go.
#
# Each subdirectory is a package in the github.com/pschlump/pluto module.

.PHONY: build test race cover bench lint tidy vet clean

## build: compile all packages
build:
	go build ./...

## vet: run go vet across all packages
vet:
	go vet ./...

## test: run all unit tests
test:
	go test ./... -count=1

## race: run all tests with the race detector
race:
	go test -race -count=1 ./...

## cover: run tests and report per-package coverage
cover:
	go test -cover -count=1 ./...

## bench: run all benchmarks
bench:
	go test -run='^$$' -bench=. -benchmem ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## tidy: tidy and verify module dependencies
tidy:
	go mod tidy

## clean: remove build and coverage artefacts
clean:
	rm -f coverage.out
	find . -name '*.test' -delete

.DEFAULT_GOAL := build
