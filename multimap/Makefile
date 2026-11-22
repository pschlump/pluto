.PHONY: all
all:
	go build

.PHONY: test
test: all
	go test

.PHONY: lint
lint:
	golangci-lint run
