
# The "go" command will need to be changed to go1.18beta1 as soon as that is out.

.PHONY: all
all:
	go build

.PHONY: test
test: all
	go test

.PHONY: diff_to_dll
diff_to_dll:
	diff dll.go ../dll/dll.go

