.PHONY: all
all:
	go build

.PHONY: test
test: all
	go test

.PHONY: race
race:
	go test -race

.PHONY: lint
## lint: run golangci-lint
lint:
	golangci-lint run
