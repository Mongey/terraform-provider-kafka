GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
KAFKA_BOOTSTRAP_SERVERS ?= localhost:29092
default: build

build:
	go build .

test:
	 go test ./kafka -v $(TESTARGS)

testacc:
	KAFKA_BOOTSTRAP_SERVERS=$(KAFKA_BOOTSTRAP_SERVERS) \
	KAFKA_ENABLE_TLS=false \
	TF_ACC=1 go test ./kafka -v $(TESTARGS) -timeout 9m -count=1

.PHONY: build test testacc
