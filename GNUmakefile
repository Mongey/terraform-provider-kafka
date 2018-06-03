TEST?=$$(go list ./... |grep -v 'vendor')
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
default: build

build:
	go install

test:
	go test -i $(TEST) || exit 1
	echo $(TEST) | \
		xargs -t -n4 go test $(TESTARGS) -timeout=30s -parallel=4

testacc:
	KAFKA_BROKER=localhost:9092 TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

.PHONY: build test testacc
