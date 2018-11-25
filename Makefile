lint:
	golangci-lint run

test:
	go test -v -race ./...

test_examples: $(addprefix test_,$(wildcard examples/*))

test_examples/%: examples/$*
	cd examples/$* && go run ../.. build

check_generated:
	go generate -v ./... && git diff --exit-code
