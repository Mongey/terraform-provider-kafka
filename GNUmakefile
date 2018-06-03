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
	KAFKA_BOOTSTRAP_SERVER=localhost:9092 \
	KAFKA_CACERT=../ssl-ffs/secrets/snakeoil-ca-1.crt \
	KAFKA_CLIENT_CERT=../ssl-ffs/secrets/kafkacat-ca1-signed.pem \
	KAFKA_CLIENT_KEY=../ssl-ffs/secrets/kafkacat-raw-private-key.pem \
	KAFKA_SKIP_VERIFY=true \
	KAFKA_ENABLE_TLS=true \
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

.PHONY: build test testacc
