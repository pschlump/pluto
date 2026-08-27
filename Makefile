# pluto - data structures with pure type-parameter constraints (no interface boxing).

.DEFAULT_GOAL := build

.PHONY: build
build:
	go build ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./... -count=1

.PHONY: race
race:
	go test -race -count=1 ./...

.PHONY: cover
cover:
	go test -coverprofile=coverage.out ./...

.PHONY: bench
bench:
	go test -run='^$$' -bench=. -benchmem ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -f coverage.out
