
# The "go" command will need to be changed to go1.18beta1 as soon as that is out.

.PHONY: all
all:
	gotip build

.PHONY: test
test: all
	gotip test

setup:
	go get golang.org/x/exp/constraints

